package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

// CheckISP flags interfaces that are too big. A "fat" interface forces
// every implementer to provide methods it may not need, and forces every
// consumer to depend on methods it never calls — the opposite of what ISP
// asks for ("many client-specific interfaces are better than one general
// purpose interface").
func CheckISP(fset *token.FileSet, files []*ast.File, cfg Config) []Issue {
	return CheckISPWithTypes(fset, files, nil, cfg, nil)
}

// CheckISPWithTypes includes complete embedded method sets when type
// information is available and retains a local-AST fallback otherwise.
func CheckISPWithTypes(fset *token.FileSet, files []*ast.File, info *types.Info, cfg Config, pkg *packageFiles) []Issue {
	interfaces := localInterfaces(files)
	var issues []Issue
	if checkEnabled(cfg, CheckISPConstructorRole) {
		issues = append(issues, checkISPConstructorRoles(fset, files, info, pkg)...)
	}
	if checkEnabled(cfg, CheckISPFatInterface) {
		issues = append(issues, checkISPFatInterfaces(fset, files, info, cfg, pkg, interfaces)...)
	}
	if checkEnabled(cfg, CheckISPUsageRatio) {
		issues = append(issues, checkISPUsageRatio(fset, files, info, cfg, pkg)...)
	}
	if checkEnabled(cfg, CheckISPConsumerRole) {
		issues = append(issues, checkISPConsumerRoles(fset, files, info, cfg, pkg)...)
	}
	if checkEnabled(cfg, CheckISPUnusedDependency) {
		issues = append(issues, checkISPUnusedDependencies(fset, files, info, cfg, pkg)...)
	}
	if checkEnabled(cfg, CheckISPStubImplementation) {
		issues = append(issues, checkISPStubImplementation(fset, files, info, cfg, pkg)...)
	}
	return issues
}

func checkISPFatInterfaces(
	fset *token.FileSet,
	files []*ast.File,
	info *types.Info,
	cfg Config,
	pkg *packageFiles,
	interfaces map[string]*ast.InterfaceType,
) []Issue {
	var issues []Issue
	for _, f := range files {
		if skipGenerated(pkg, f) {
			continue
		}
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				if issue := fatInterfaceIssue(fset, gen, spec, info, cfg, interfaces); issue != nil && !fullyConsumedComposedInterface(spec, files, info) {
					issues = append(issues, *issue)
				}
			}
		}
	}
	return issues
}

func fatInterfaceIssue(
	fset *token.FileSet,
	gen *ast.GenDecl,
	spec ast.Spec,
	info *types.Info,
	cfg Config,
	interfaces map[string]*ast.InterfaceType,
) *Issue {
	ts, ok := spec.(*ast.TypeSpec)
	if !ok {
		return nil
	}
	iface, ok := ts.Type.(*ast.InterfaceType)
	if !ok || iface.Methods == nil {
		return nil
	}
	methodCount := completeInterfaceMethodCount(ts, iface, interfaces, info)
	if methodCount <= cfg.MaxInterfaceMethods {
		return nil
	}
	aggregate := interfaceAggregatesFatInterface(iface, interfaces, info, cfg.MaxInterfaceMethods)
	severity := SeverityWarning
	if interfaceDeclDeprecated(gen, ts) || aggregate {
		severity = SeverityNote
	}
	message, evidence := fatInterfaceMessage(ts.Name.Name, methodCount, cfg.MaxInterfaceMethods, aggregate)
	issue := issueAt(fset, ts, Issue{
		Rule:     RuleISP,
		Check:    CheckISPFatInterface,
		Severity: severity,
		Message:  message,
		Evidence: evidence,
	})
	return &issue
}

func completeInterfaceMethodCount(
	ts *ast.TypeSpec,
	iface *ast.InterfaceType,
	interfaces map[string]*ast.InterfaceType,
	info *types.Info,
) int {
	methodCount := countInterfaceMethods(iface, interfaces, map[string]bool{})
	if info == nil {
		return methodCount
	}
	object, ok := info.Defs[ts.Name].(*types.TypeName)
	if !ok {
		return methodCount
	}
	typed, ok := object.Type().Underlying().(*types.Interface)
	if !ok {
		return methodCount
	}
	typed.Complete()
	return typed.NumMethods()
}

func fatInterfaceMessage(name string, methodCount, maxMethods int, aggregate bool) (string, string) {
	evidence := fmt.Sprintf("fat-interface:interface=%s;methods=%d;max=%d", name, methodCount, maxMethods)
	if aggregate {
		return fmt.Sprintf(
			"interface %q aggregates %d methods (max %d), including an already-wide embedded role: keep the aggregate at wiring boundaries and inject narrower roles into business consumers",
			name, methodCount, maxMethods,
		), evidence + ";aggregate=true"
	}
	return fmt.Sprintf(
		"interface %q declares %d methods (max %d): implementers are forced to satisfy all of them; consider splitting it into smaller, role-specific interfaces",
		name, methodCount, maxMethods,
	), evidence
}

func localInterfaces(files []*ast.File) map[string]*ast.InterfaceType {
	interfaces := map[string]*ast.InterfaceType{}
	for _, f := range files {
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if ok {
					if iface, ok := ts.Type.(*ast.InterfaceType); ok {
						interfaces[ts.Name.Name] = iface
					}
				}
			}
		}
	}
	return interfaces
}

func countInterfaceMethods(iface *ast.InterfaceType, interfaces map[string]*ast.InterfaceType, seen map[string]bool) int {
	methods := map[string]bool{}
	var collect func(*ast.InterfaceType)
	collect = func(current *ast.InterfaceType) {
		for _, field := range current.Methods.List {
			if len(field.Names) > 0 {
				for _, name := range field.Names {
					methods[name.Name] = true
				}
				continue
			}
			if embedded, ok := field.Type.(*ast.Ident); ok && !seen[embedded.Name] {
				if embeddedInterface, ok := interfaces[embedded.Name]; ok {
					seen[embedded.Name] = true
					collect(embeddedInterface)
				}
			}
		}
	}
	collect(iface)
	return len(methods)
}

func interfaceAggregatesFatInterface(iface *ast.InterfaceType, interfaces map[string]*ast.InterfaceType, info *types.Info, maxMethods int) bool {
	for _, field := range iface.Methods.List {
		if len(field.Names) > 0 {
			continue
		}
		if info != nil {
			if embedded, ok := underlyingInterface(info.TypeOf(field.Type)); ok {
				embedded.Complete()
				if embedded.NumMethods() > maxMethods {
					return true
				}
			}
		}
		if ident, ok := field.Type.(*ast.Ident); ok {
			if embedded := interfaces[ident.Name]; embedded != nil && countInterfaceMethods(embedded, interfaces, map[string]bool{}) > maxMethods {
				return true
			}
		}
	}
	return false
}

func interfaceDeclDeprecated(gen *ast.GenDecl, ts *ast.TypeSpec) bool {
	for _, doc := range []*ast.CommentGroup{gen.Doc, ts.Doc} {
		if doc == nil {
			continue
		}
		for _, comment := range doc.List {
			text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
			lower := strings.ToLower(text)
			if strings.Contains(lower, "deprecated:") {
				return true
			}
			if strings.HasPrefix(lower, "deprecated ") {
				return true
			}
		}
	}
	return false
}

// A composed port has already declared its roles. A complete consumer is not
// forced to implement unrelated methods; partial consumers remain diagnosed.
func fullyConsumedComposedInterface(spec ast.Spec, files []*ast.File, info *types.Info) bool {
	if info == nil {
		return false
	}
	ts, ok := spec.(*ast.TypeSpec)
	if !ok {
		return false
	}
	syntax, ok := ts.Type.(*ast.InterfaceType)
	if !ok {
		return false
	}
	embedded := 0
	for _, field := range syntax.Methods.List {
		if len(field.Names) == 0 {
			embedded++
		}
	}
	if embedded < 2 {
		return false
	}
	object, ok := info.Defs[ts.Name].(*types.TypeName)
	if !ok {
		return false
	}
	target := object.Type()
	iface, ok := target.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	iface.Complete()
	locals := localFunctionDeclarations(files, info)
	for _, file := range files {
		for _, declaration := range file.Decls {
			gen, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, raw := range gen.Specs {
				owner, ok := raw.(*ast.TypeSpec)
				if !ok {
					continue
				}
				structure, ok := owner.Type.(*ast.StructType)
				if !ok {
					continue
				}
				for _, field := range structure.Fields.List {
					if !types.Identical(info.TypeOf(field.Type), target) {
						continue
					}
					for _, name := range field.Names {
						obj, ok := info.Defs[name].(*types.Var)
						if !ok {
							continue
						}
						usage := receiverFieldUsage(owner.Name.Name, obj, files, info, locals)
						for method := range directReceiverFieldMethods(owner.Name.Name, obj, files, info) {
							usage.methods[method] = true
						}
						if len(unusedInterfaceMethods(iface, usage.methods)) == 0 {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

func directReceiverFieldMethods(owner string, field *types.Var, files []*ast.File, info *types.Info) map[string]bool {
	methods := map[string]bool{}
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !isReceiverMethodOf(fn, owner) {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				member, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				base, ok := member.X.(*ast.SelectorExpr)
				if !ok || selectionObject(info, base) != field {
					return true
				}
				selection := info.Selections[member]
				if selection != nil && selection.Kind() == types.MethodVal {
					methods[member.Sel.Name] = true
				}
				return true
			})
		}
	}
	return methods
}
