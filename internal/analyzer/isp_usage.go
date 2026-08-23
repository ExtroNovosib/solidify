package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

func checkISPUsageRatio(fset *token.FileSet, files []*ast.File, info *types.Info, cfg Config, pkg *packageFiles) []Issue {
	if !shouldCheckISPUsageRatio(info, cfg, pkg) {
		return nil
	}
	issues := checkISPParameterUsageRatio(fset, files, info, cfg, pkg)
	issues = append(issues, checkISPFieldUsageRatio(fset, files, info, cfg, pkg)...)
	return issues
}

func shouldCheckISPUsageRatio(info *types.Info, cfg Config, pkg *packageFiles) bool {
	if info == nil {
		return false
	}
	// Composition roots are responsible for wiring broad interfaces, so the
	// usage-ratio check does not apply to those packages. Keep the package
	// path check here rather than in the public ISP entry point so fat-interface
	// and stub findings remain active in composition roots.
	if pkg != nil && pkg.pkgPath != "" && matchesAnyPackagePattern(pkg.pkgPath, cfg.OCPCompositionRoots) {
		return false
	}
	// When architecture logic packages are configured, use them as an include
	// filter. This keeps usage-ratio focused on business consumers while
	// excluding adapter and persistence wiring without disabling the check.
	if pkg != nil && pkg.pkgPath != "" && len(cfg.OCPLogicPackages) > 0 &&
		!matchesAnyPackagePattern(pkg.pkgPath, cfg.OCPLogicPackages) {
		return false
	}
	return true
}

func checkISPParameterUsageRatio(
	fset *token.FileSet,
	files []*ast.File,
	info *types.Info,
	cfg Config,
	pkg *packageFiles,
) []Issue {
	var issues []Issue
	for _, f := range files {
		if skipGenerated(pkg, f) {
			continue
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || !fn.Name.IsExported() {
				continue
			}
			issues = append(issues, checkISPFunctionUsageRatio(fset, fn, info, cfg, pkg)...)
		}
	}
	return issues
}

func checkISPFunctionUsageRatio(
	fset *token.FileSet,
	fn *ast.FuncDecl,
	info *types.Info,
	cfg Config,
	pkg *packageFiles,
) []Issue {
	var issues []Issue
	for _, field := range fn.Type.Params.List {
		iface, ok := eligibleUsageInterface(info.TypeOf(field.Type), info, cfg, pkg)
		if !ok {
			continue
		}
		for _, name := range field.Names {
			if issue := parameterUsageRatioIssue(fset, fn, name, iface, info, cfg); issue != nil {
				issues = append(issues, *issue)
			}
		}
	}
	return issues
}

func parameterUsageRatioIssue(
	fset *token.FileSet,
	fn *ast.FuncDecl,
	name *ast.Ident,
	iface *types.Interface,
	info *types.Info,
	cfg Config,
) *Issue {
	paramObject, ok := info.Defs[name].(*types.Var)
	if !ok {
		return nil
	}
	used, indirect := interfaceMethodsUsed(fn.Body, paramObject, info)
	if indirect || len(used) == 0 {
		return nil
	}
	ratioPercent := 100 * len(used) / iface.NumMethods()
	if ratioPercent >= cfg.ISPUsageRatioPercent {
		return nil
	}
	usedList := SortedSymbols(used)
	issue := issueAt(fset, name, Issue{
		Rule:     RuleISP,
		Check:    CheckISPUsageRatio,
		Severity: SeverityWarning,
		Message: fmt.Sprintf(
			"parameter %q accepts interface with %d methods but only uses %d (%d%%): consider a narrower interface with just %s",
			name.Name, iface.NumMethods(), len(used), ratioPercent, strings.Join(usedList, ", "),
		),
		Evidence: fmt.Sprintf(
			"usage-ratio:function=%s;parameter=%s;used=%d;total=%d;methods=%s",
			fn.Name.Name, name.Name, len(used), iface.NumMethods(), strings.Join(usedList, ","),
		),
	})
	return &issue
}

func checkISPFieldUsageRatio(fset *token.FileSet, files []*ast.File, info *types.Info, cfg Config, pkg *packageFiles) []Issue {
	var issues []Issue
	for _, file := range files {
		if skipGenerated(pkg, file) {
			continue
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				issues = append(issues, checkISPStructUsageRatio(fset, spec, files, info, cfg, pkg)...)
			}
		}
	}
	return issues
}

func checkISPStructUsageRatio(
	fset *token.FileSet,
	spec ast.Spec,
	files []*ast.File,
	info *types.Info,
	cfg Config,
	pkg *packageFiles,
) []Issue {
	typeSpec, ok := spec.(*ast.TypeSpec)
	if !ok || !typeSpec.Name.IsExported() {
		return nil
	}
	structType, ok := typeSpec.Type.(*ast.StructType)
	if !ok || structType.Fields == nil {
		return nil
	}
	var issues []Issue
	for _, field := range structType.Fields.List {
		iface, ok := eligibleUsageInterface(info.TypeOf(field.Type), info, cfg, pkg)
		if !ok {
			continue
		}
		for _, name := range field.Names {
			if issue := fieldUsageRatioIssue(fset, typeSpec.Name.Name, name, iface, files, info, cfg); issue != nil {
				issues = append(issues, *issue)
			}
		}
	}
	return issues
}

func fieldUsageRatioIssue(
	fset *token.FileSet,
	owner string,
	name *ast.Ident,
	iface *types.Interface,
	files []*ast.File,
	info *types.Info,
	cfg Config,
) *Issue {
	fieldObject, ok := info.Defs[name].(*types.Var)
	if !ok {
		return nil
	}
	used, indirect := interfaceFieldMethodsUsed(owner, fieldObject, files, info)
	if indirect || len(used) == 0 {
		return nil
	}
	ratioPercent := 100 * len(used) / iface.NumMethods()
	if ratioPercent >= cfg.ISPUsageRatioPercent {
		return nil
	}
	usedList := SortedSymbols(used)
	issue := issueAt(fset, name, Issue{
		Rule:     RuleISP,
		Check:    CheckISPUsageRatio,
		Severity: SeverityWarning,
		Message: fmt.Sprintf(
			"field %s.%s accepts interface with %d methods but receiver methods only use %d (%d%%): consider a narrower interface with just %s",
			owner, name.Name, iface.NumMethods(), len(used), ratioPercent, strings.Join(usedList, ", "),
		),
		Evidence: fmt.Sprintf(
			"usage-ratio:type=%s;field=%s;used=%d;total=%d;methods=%s",
			owner, name.Name, len(used), iface.NumMethods(), strings.Join(usedList, ","),
		),
	})
	return &issue
}

func eligibleUsageInterface(t types.Type, info *types.Info, cfg Config, pkg *packageFiles) (*types.Interface, bool) {
	iface, ok := underlyingInterface(t)
	if !ok || isExternalInterface(t, pkg) || isWellKnownWideInterface(t) {
		return nil, false
	}
	iface.Complete()
	return iface, iface.NumMethods() >= cfg.ISPMinMethods && info != nil
}

func interfaceFieldMethodsUsed(owner string, fieldObject *types.Var, files []*ast.File, info *types.Info) (used []string, indirect bool) {
	collector := fieldUsageCollector{
		fieldObject:    fieldObject,
		info:           info,
		localFunctions: localFunctionDeclarations(files, info),
		seen:           map[string]bool{},
	}
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !isReceiverMethodOf(fn, owner) {
				continue
			}
			ast.Inspect(fn.Body, collector.inspect)
			if collector.indirect {
				break
			}
		}
		if collector.indirect {
			break
		}
	}
	for name := range collector.seen {
		used = append(used, name)
	}
	return used, collector.indirect
}

type fieldUsageCollector struct {
	fieldObject    *types.Var
	info           *types.Info
	localFunctions map[*types.Func]*ast.FuncDecl
	seen           map[string]bool
	indirect       bool
}

func (collector *fieldUsageCollector) inspect(node ast.Node) bool {
	if collector.indirect {
		return false
	}
	switch current := node.(type) {
	case *ast.CallExpr:
		return collector.inspectCall(current)
	case *ast.SelectorExpr:
		return collector.inspectSelector(current)
	default:
		return true
	}
}

func (collector *fieldUsageCollector) inspectCall(call *ast.CallExpr) bool {
	if method := fieldMethodCall(call, collector.fieldObject, collector.info); method != "" {
		collector.seen[method] = true
		return false
	}
	fieldArgument := false
	for index, argument := range call.Args {
		selector, ok := argument.(*ast.SelectorExpr)
		if !ok || selectionObject(collector.info, selector) != collector.fieldObject {
			if expressionUsesField(argument, collector.fieldObject, collector.info) {
				collector.indirect = true
				return false
			}
			continue
		}
		fieldArgument = true
		if !collector.collectLocalCallUsage(call, index) {
			collector.indirect = true
			return false
		}
	}
	return !fieldArgument
}

func (collector *fieldUsageCollector) inspectSelector(selector *ast.SelectorExpr) bool {
	if fieldSelector, ok := selector.X.(*ast.SelectorExpr); ok &&
		selectionObject(collector.info, fieldSelector) == collector.fieldObject {
		collector.seen[selector.Sel.Name] = true
		return false
	}
	if selectionObject(collector.info, selector) == collector.fieldObject {
		collector.indirect = true
		return false
	}
	return true
}

func (collector *fieldUsageCollector) collectLocalCallUsage(call *ast.CallExpr, argumentIndex int) bool {
	parameter, callee, ok := localCallParameter(call, argumentIndex, collector.localFunctions, collector.info)
	if !ok {
		return false
	}
	used, indirect := interfaceMethodsUsed(callee.Body, parameter, collector.info)
	if indirect {
		return false
	}
	for _, method := range used {
		collector.seen[method] = true
	}
	return true
}

func fieldMethodCall(call *ast.CallExpr, fieldObject *types.Var, info *types.Info) string {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	fieldSelector, ok := selector.X.(*ast.SelectorExpr)
	if !ok || selectionObject(info, fieldSelector) != fieldObject {
		return ""
	}
	return selector.Sel.Name
}

func localFunctionDeclarations(files []*ast.File, info *types.Info) map[*types.Func]*ast.FuncDecl {
	functions := map[*types.Func]*ast.FuncDecl{}
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if object, ok := info.Defs[fn.Name].(*types.Func); ok {
				functions[object] = fn
			}
		}
	}
	return functions
}

func isReceiverMethodOf(fn *ast.FuncDecl, owner string) bool {
	return fn.Recv != nil && fn.Body != nil && receiverTypeName(fn.Recv.List[0].Type) == owner
}

func localCallParameter(call *ast.CallExpr, argumentIndex int, functions map[*types.Func]*ast.FuncDecl, info *types.Info) (*types.Var, *ast.FuncDecl, bool) {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return nil, nil, false
	}
	object, ok := info.Uses[ident].(*types.Func)
	if !ok {
		return nil, nil, false
	}
	fn := functions[object]
	if fn == nil || fn.Type.Params == nil {
		return nil, nil, false
	}
	index := 0
	for _, field := range fn.Type.Params.List {
		if len(field.Names) == 0 {
			if index == argumentIndex {
				return nil, nil, false
			}
			index++
			continue
		}
		for _, name := range field.Names {
			if index == argumentIndex {
				parameter, ok := info.Defs[name].(*types.Var)
				return parameter, fn, ok
			}
			index++
		}
	}
	return nil, nil, false
}

func expressionUsesField(expr ast.Expr, fieldObject *types.Var, info *types.Info) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if ok && selectionObject(info, selector) == fieldObject {
			found = true
			return false
		}
		return !found
	})
	return found
}

func selectionObject(info *types.Info, selector *ast.SelectorExpr) types.Object {
	if info == nil || selector == nil {
		return nil
	}
	selection := info.Selections[selector]
	if selection == nil {
		return nil
	}
	return selection.Obj()
}

func underlyingInterface(t types.Type) (*types.Interface, bool) {
	if t == nil {
		return nil, false
	}
	iface, ok := t.Underlying().(*types.Interface)
	return iface, ok
}

// isExternalInterface reports whether a named interface belongs to a dependency
// outside the scanned module. Clients cannot change those interfaces directly;
// they should introduce a local port before usage-ratio can provide an
// actionable ISP suggestion. Anonymous interfaces and package snapshots without
// module metadata remain eligible for analysis.
func isExternalInterface(t types.Type, pkg *packageFiles) bool {
	if pkg == nil || pkg.modulePath == "" {
		return false
	}
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	interfacePackage := named.Obj().Pkg().Path()
	return interfacePackage != pkg.modulePath && !strings.HasPrefix(interfacePackage, pkg.modulePath+"/")
}

func interfaceMethodsUsed(body *ast.BlockStmt, param *types.Var, info *types.Info) (used []string, indirect bool) {
	seen := map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		if indirect {
			return false
		}
		switch node := n.(type) {
		case *ast.SelectorExpr:
			if ident, ok := node.X.(*ast.Ident); ok {
				if obj, ok := info.Uses[ident]; ok && obj == param {
					seen[node.Sel.Name] = true
					return false
				}
			}
		case *ast.Ident:
			if obj, ok := info.Uses[node]; ok && obj == param {
				indirect = true
				return false
			}
		}
		return true
	})
	for name := range seen {
		used = append(used, name)
	}
	return used, indirect
}
