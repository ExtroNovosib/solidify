package analyzer

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
)

// CheckDIP flags struct fields that depend directly on another *concrete*
// type declared in the same source set, instead of on an interface/
// abstraction. High-level types wiring themselves directly to low-level
// concrete implementations is exactly what "depend on abstractions, not
// concretions" warns against — it makes the high-level type impossible to
// reuse or test without dragging the concrete dependency along.
//
// This is necessarily a local, syntax-only heuristic (no go/packages, no
// type-checking): it only "knows about" types declared in the files being
// linted, so it can't see through interfaces or types from other packages.
// That keeps the tool dependency-free; it trades recall for zero setup.
func CheckDIP(fset *token.FileSet, files []*ast.File, cfg Config) []Issue {
	return CheckDIPWithTypes(fset, files, nil, cfg, nil)
}

func CheckDIPWithTypes(fset *token.FileSet, files []*ast.File, info *types.Info, cfg Config, pkg *packageFiles) []Issue {
	if pkg != nil && pkg.pkgPath != "" && matchesAnyPackagePattern(pkg.pkgPath, cfg.OCPCompositionRoots) {
		return nil
	}
	kind := indexLocalTypeKinds(files)
	var issues []Issue
	issues = append(issues, fieldConcreteIssues(fset, files, info, kind, cfg, pkg)...)
	issues = append(issues, constructorConcreteIssues(fset, files, info, kind, cfg, pkg)...)
	return issues
}

func indexLocalTypeKinds(files []*ast.File) map[string]string {
	kind := map[string]string{}
	for _, f := range files {
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				switch ts.Type.(type) {
				case *ast.InterfaceType:
					kind[ts.Name.Name] = "interface"
				case *ast.StructType:
					kind[ts.Name.Name] = localStructKind
				default:
					kind[ts.Name.Name] = "other"
				}
			}
		}
	}
	return kind
}

func fieldConcreteIssues(fset *token.FileSet, files []*ast.File, info *types.Info, kind map[string]string, cfg Config, pkg *packageFiles) []Issue {
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
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok || st.Fields == nil {
					continue
				}
				if serializedDataCarrierStruct(files, ts.Name.Name, st) {
					continue
				}
				env := dipFieldEnv{fset: fset, files: files, typeName: ts.Name.Name, info: info, kind: kind, cfg: cfg}
				rootThreshold := env.cfg.DIPCompositionRootFields
				if rootThreshold <= 0 {
					rootThreshold = DefaultConfig().DIPCompositionRootFields
				}
				compositionRoot := countConcreteStructFieldDeps(env, st) >= rootThreshold
				forwardedFields := dipForwardedConcreteFields(env, files, ts.Name.Name, st)
				for _, field := range st.Fields.List {
					if compositionRoot || dipBridgeFieldForwarded(field, forwardedFields) {
						continue
					}
					if issue, ok := structFieldConcreteIssue(env, field); ok {
						issues = append(issues, issue)
					}
				}
			}
		}
	}
	return issues
}

type dipFieldEnv struct {
	fset     *token.FileSet
	files    []*ast.File
	typeName string
	info     *types.Info
	kind     map[string]string
	cfg      Config
}

func structFieldConcreteIssue(env dipFieldEnv, field *ast.Field) (Issue, bool) {
	dep, ok := concreteFieldDependency(env, field)
	if !ok {
		return Issue{}, false
	}
	if allowedDependency(dep, env.cfg) {
		return Issue{}, false
	}
	fieldName := "field"
	if len(field.Names) > 0 {
		fieldName = field.Names[0].Name
	}
	if passiveTestDataField(env, field) {
		return Issue{}, false
	}
	if concreteFieldIsExposed(env.files, env.info, env.typeName, fieldName, dep) {
		return Issue{}, false
	}
	return issueAt(env.fset, field, Issue{
		Rule:     RuleDIP,
		Check:    CheckDIPConcreteDependency,
		Severity: SeverityWarning,
		Message: fmt.Sprintf(
			"%s.%s depends on the concrete type *%s instead of an interface: "+
				"define an interface for what %s needs from %s and depend on that "+
				"(constructor-inject it) so %s can be tested/reused independently of %s's implementation",
			env.typeName, fieldName, dep, env.typeName, dep, env.typeName, dep,
		),
		Evidence: fmt.Sprintf("concrete-dependency:type=%s;field=%s;dependency=%s", env.typeName, fieldName, dep),
	}), true
}

// passiveTestDataField excludes test-fixture state that stores a domain value
// or DTO for later return/assertion. A direct method call or use as a
// constructor collaborator remains a concrete dependency signal.
func passiveTestDataField(env dipFieldEnv, field *ast.Field) bool {
	if env.info == nil || len(field.Names) == 0 || !strings.HasSuffix(env.fset.Position(field.Pos()).Filename, "_test.go") {
		return false
	}
	fieldType := env.info.TypeOf(field.Type)
	if !isDomainStructType(fieldType) && !isSerializedTestDataType(fieldType) {
		return false
	}
	fieldObject, ok := env.info.Defs[field.Names[0]].(*types.Var)
	return ok && !testFieldHasBehavioralEvidence(env.files, env.info, env.typeName, fieldObject)
}

func testFieldHasBehavioralEvidence(files []*ast.File, info *types.Info, owner string, fieldObject *types.Var) bool {
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !isReceiverMethodOf(fn, owner) {
				continue
			}
			parents := astParentIndex(fn.Body)
			behavioral := false
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if !ok || selectionObject(info, selector) != fieldObject {
					return true
				}
				if member, ok := parents[selector].(*ast.SelectorExpr); ok && member.X == selector {
					selection := info.Selections[member]
					if selection != nil && selection.Kind() == types.MethodVal {
						behavioral = true
						return false
					}
				}
				if call, ok := parents[selector].(*ast.CallExpr); ok && constructorCallUses(call, selector) {
					behavioral = true
					return false
				}
				return true
			})
			if behavioral {
				return true
			}
		}
	}
	return false
}

func constructorCallUses(call *ast.CallExpr, argument ast.Expr) bool {
	used := false
	for _, candidate := range call.Args {
		if candidate == argument {
			used = true
			break
		}
	}
	if !used {
		return false
	}
	switch callee := call.Fun.(type) {
	case *ast.Ident:
		return strings.HasPrefix(callee.Name, "New")
	case *ast.SelectorExpr:
		return strings.HasPrefix(callee.Sel.Name, "New")
	default:
		return false
	}
}

func concreteFieldIsExposed(files []*ast.File, info *types.Info, owner, fieldName, dependency string) bool {
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Body == nil || receiverTypeName(fn.Recv.List[0].Type) != owner {
				continue
			}
			if !functionReturnsConcreteDependency(fn, info, dependency) {
				continue
			}
			receivers := receiverNames(fn)
			exposes := false
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				ret, ok := node.(*ast.ReturnStmt)
				if !ok {
					return true
				}
				for _, result := range ret.Results {
					ast.Inspect(result, func(node ast.Node) bool {
						sel, ok := node.(*ast.SelectorExpr)
						if ok && sel.Sel.Name == fieldName && containsString(receivers, selectorReceiverName(sel.X)) {
							exposes = true
							return false
						}
						return true
					})
				}
				return !exposes
			})
			if exposes {
				return true
			}
		}
	}
	return false
}

func functionReturnsConcreteDependency(fn *ast.FuncDecl, info *types.Info, dependency string) bool {
	if fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
		return false
	}
	if info != nil {
		resultType := info.TypeOf(fn.Type.Results.List[0].Type)
		if _, pointer := types.Unalias(resultType).(*types.Pointer); pointer {
			return typeString(resultType) == dependency
		}
	}
	name, pointer := pointerToIdent(fn.Type.Results.List[0].Type)
	return pointer && name == dependency
}

func countConcreteStructFieldDeps(env dipFieldEnv, st *ast.StructType) int {
	count := 0
	for _, field := range st.Fields.List {
		if _, ok := concreteFieldDependency(env, field); ok {
			count++
		}
	}
	return count
}

func dipForwardedConcreteFields(env dipFieldEnv, files []*ast.File, typeName string, st *ast.StructType) map[string]bool {
	concreteDeps := countConcreteStructFieldDeps(env, st)
	if concreteDeps == 0 || concreteDeps > 2 {
		return nil
	}
	if countDIPRelevantFields(env, st) > 3 {
		return nil
	}
	concreteFields := map[string]bool{}
	for _, field := range st.Fields.List {
		if _, ok := concreteFieldDependency(env, field); !ok {
			continue
		}
		for _, name := range field.Names {
			concreteFields[name.Name] = true
		}
	}
	if len(concreteFields) == 0 {
		return nil
	}
	forwarded := map[string]bool{}
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Body == nil {
				continue
			}
			if receiverTypeName(fn.Recv.List[0].Type) != typeName {
				continue
			}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				sel, ok := node.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if concreteFields[sel.Sel.Name] {
					forwarded[sel.Sel.Name] = true
				}
				return true
			})
		}
	}
	if len(forwarded) == 0 {
		return nil
	}
	return forwarded
}

func dipBridgeFieldForwarded(field *ast.Field, forwarded map[string]bool) bool {
	if len(forwarded) == 0 {
		return false
	}
	for _, name := range field.Names {
		if forwarded[name.Name] {
			return true
		}
	}
	return false
}

func countDIPRelevantFields(env dipFieldEnv, st *ast.StructType) int {
	count := 0
	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			continue
		}
		if env.info != nil && isStdlibConcreteType(env.info.TypeOf(field.Type)) {
			continue
		}
		count += len(field.Names)
	}
	return count
}

func concreteFieldDependency(env dipFieldEnv, field *ast.Field) (dep string, ok bool) {
	if env.info != nil {
		fieldType := env.info.TypeOf(field.Type)
		if isStdlibConcreteType(fieldType) || isPassiveDomainDataType(fieldType) {
			return "", false
		}
	}
	dep, isPtr := pointerToIdent(field.Type)
	if dep != "" && (isSamePackageLocalStruct(env.kind, dep) || isConfigDataBagType(dep)) {
		return "", false
	}
	if dep == "" && (env.info == nil || !isConcreteType(env.info.TypeOf(field.Type))) {
		return "", false
	}
	if dep == "" {
		dep = typeString(env.info.TypeOf(field.Type))
		_, isPtr = field.Type.(*ast.StarExpr)
	}
	if isSamePackageLocalStruct(env.kind, dep) || isConfigDataBagType(dep) {
		return "", false
	}
	if env.kind[dep] != localStructKind {
		if env.info == nil || !isConcreteType(env.info.TypeOf(field.Type)) {
			return "", false
		}
		if !isPtr {
			return "", false
		}
		dep = typeString(env.info.TypeOf(field.Type))
	} else if !isPtr {
		return "", false
	}
	if !isPtr && env.info == nil {
		return "", false
	}
	return dep, true
}

func constructorConcreteIssues(fset *token.FileSet, files []*ast.File, info *types.Info, kind map[string]string, cfg Config, pkg *packageFiles) []Issue {
	var issues []Issue
	for _, f := range files {
		if skipGenerated(pkg, f) {
			continue
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !strings.HasPrefix(fn.Name.Name, "New") || fn.Type.Params == nil {
				continue
			}
			for _, field := range fn.Type.Params.List {
				dep, ptr := pointerToIdent(field.Type)
				if info != nil {
					fieldType := info.TypeOf(field.Type)
					if !isConcreteType(fieldType) {
						continue
					}
					if isStdlibConcreteType(fieldType) || isPassiveDomainDataType(fieldType) {
						continue
					}
					if dep == "" {
						dep = typeString(fieldType)
						_, ptr = field.Type.(*ast.StarExpr)
					}
					if isSamePackageLocalStruct(kind, dep) || isConfigDataBagType(dep) {
						continue
					}
					if !ptr || allowedDependency(dep, cfg) {
						continue
					}
					issues = append(issues, issueAt(fset, field, Issue{Rule: RuleDIP, Check: CheckDIPConcreteDependency, Severity: SeverityWarning, Message: fmt.Sprintf("constructor %q depends on the concrete type *%s instead of an interface", fn.Name.Name, dep), Evidence: fmt.Sprintf("concrete-dependency:function=%s;dependency=%s", fn.Name.Name, dep)}))
				}
			}
		}
	}
	return issues
}

func allowedDependency(dep string, cfg Config) bool {
	dep = strings.TrimPrefix(dep, "*")
	terminal := dep
	if slash := strings.LastIndex(terminal, "/"); slash >= 0 {
		terminal = terminal[slash+1:]
	}
	unqualified := terminal
	if dot := strings.LastIndex(unqualified, "."); dot >= 0 {
		unqualified = unqualified[dot+1:]
	}
	for _, allowed := range cfg.DIPAllowDependencies {
		allowed = strings.TrimPrefix(strings.TrimSpace(allowed), "*")
		switch {
		case strings.Contains(allowed, "/") && allowed == dep:
			return true
		case !strings.Contains(allowed, "/") && strings.Contains(allowed, ".") && allowed == terminal:
			return true
		case !strings.Contains(allowed, ".") && allowed == unqualified:
			return true
		}
	}
	return false
}

// isPassiveDomainDataType identifies domain structs that carry state but expose
// no behavior. Depending on such values directly is idiomatic Go data flow, not
// a dependency on replaceable behavior.
func isPassiveDomainDataType(t types.Type) bool {
	named, ok := namedConcreteStructType(t)
	if !ok || !isDomainStructType(t) {
		return false
	}
	return types.NewMethodSet(named).Len() == 0 && types.NewMethodSet(types.NewPointer(named)).Len() == 0
}

func isDomainStructType(t types.Type) bool {
	named, ok := namedConcreteStructType(t)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	pkg := named.Obj().Pkg()
	pkgPath := strings.TrimSuffix(pkg.Path(), "/")
	pathBase := pkgPath
	if slash := strings.LastIndex(pathBase, "/"); slash >= 0 {
		pathBase = pathBase[slash+1:]
	}
	if pkg.Name() != "domain" && pathBase != "domain" {
		return false
	}
	return true
}

func isSerializedTestDataType(t types.Type) bool {
	named, ok := namedConcreteStructType(t)
	if !ok || named.Obj() == nil {
		return false
	}
	name := named.Obj().Name()
	for _, suffix := range []string{"DTO", "Data", "Input", "Output", "Payload", "Record", "Request", "Response"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	structure, ok := named.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	for index := 0; index < structure.NumFields(); index++ {
		if strings.Contains(structure.Tag(index), "json:") {
			return true
		}
	}
	return false
}

func namedConcreteStructType(t types.Type) (*types.Named, bool) {
	if t == nil {
		return nil, false
	}
	t = types.Unalias(t)
	if pointer, ok := t.(*types.Pointer); ok {
		t = types.Unalias(pointer.Elem())
	}
	named, ok := t.(*types.Named)
	if !ok {
		return nil, false
	}
	_, ok = named.Underlying().(*types.Struct)
	return named, ok
}

func isStdlibConcreteType(t types.Type) bool {
	if t == nil || !isConcreteType(t) {
		return false
	}
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	n, ok := t.(*types.Named)
	if !ok {
		return false
	}
	return isStdlibPackage(n.Obj().Pkg())
}

func isStdlibPackage(pkg *types.Package) bool {
	if pkg == nil {
		return false
	}
	path := pkg.Path()
	if path == "" || strings.Contains(path, ".") {
		return false
	}
	info, err := os.Stat(filepath.Join(build.Default.GOROOT, "src", path))
	return err == nil && info.IsDir()
}

func isConcreteType(t types.Type) bool {
	if t == nil {
		return false
	}
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	_, ok := t.Underlying().(*types.Struct)
	return ok
}

func typeString(t types.Type) string {
	t = types.Unalias(t)
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	t = types.Unalias(t)
	return types.TypeString(t, func(pkg *types.Package) string {
		if pkg == nil {
			return ""
		}
		return pkg.Path()
	})
}

// pointerToIdent reports the identifier name if expr is `*Ident` or bare
// `Ident`, along with whether it was a pointer.
func pointerToIdent(expr ast.Expr) (name string, isPtr bool) {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return typeExprIdent(t.X), true
	case *ast.Ident:
		return t.Name, false
	case *ast.IndexExpr:
		return typeExprIdent(t.X), false
	case *ast.IndexListExpr:
		return typeExprIdent(t.X), false
	}
	return "", false
}

func typeExprIdent(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return typeExprIdent(t.X)
	case *ast.IndexListExpr:
		return typeExprIdent(t.X)
	}
	return ""
}
