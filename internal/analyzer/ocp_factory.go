package analyzer

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"
)

func emitOCPConcreteParameters(pkgs []*packageFiles, cfg Config) []Issue {
	var issues []Issue
	for _, pkg := range pkgs {
		if pkg.info == nil || !pkg.typeComplete {
			continue
		}
		for _, file := range pkg.files {
			if !ocpFileEnabled(pkg, file, cfg) {
				continue
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || fn.Type.Params == nil {
					continue
				}
				for _, field := range fn.Type.Params.List {
					paramType := pkg.info.TypeOf(field.Type)
					if !concreteTypeCandidate(paramType) || !foreignPointerParameter(paramType, pkg.typePkg) || allowedDependency(canonicalTypeKey(paramType), cfg) {
						continue
					}
					for _, name := range field.Names {
						obj, ok := pkg.info.Defs[name].(*types.Var)
						if !ok {
							continue
						}
						methods, safe := concreteParameterMethods(fn.Body, obj, pkg.info)
						if !safe || len(methods) < cfg.OCPMinConcreteParameterMethods || domainMapperMethods(paramType, methods, cfg) {
							continue
						}
						interfaceName := matchingInterface(pkg, methods)
						methodNameList := methodNames(methods)
						message := fmt.Sprintf("parameter %q has concrete type %s but is only used through methods %s; consider a consumer-defined interface", name.Name, canonicalTypeKey(paramType), strings.Join(methodNameList, ", "))
						if interfaceName != "" {
							message += fmt.Sprintf(" (matching interface: %s)", interfaceName)
						}
						issues = append(issues, issueAt(pkg.fset, name, Issue{Rule: RuleOCP, Check: CheckOCPConcreteParameter, Severity: SeverityNote, Message: message,
							Evidence: fmt.Sprintf("concrete-parameter:function=%s;parameter=%s;type=%s;methods=%s", fn.Name.Name, name.Name, canonicalTypeKey(paramType), strings.Join(methodNameList, ","))}))
					}
				}
			}
		}
	}
	return issues
}

// foreignPointerParameter reports whether typ points to a named type declared
// outside current. Like constructor concrete-dependency findings, a package's
// own types and by-value copies are its vocabulary rather than an extension
// seam a caller could substitute.
func foreignPointerParameter(typ types.Type, current *types.Package) bool {
	pointer, ok := types.Unalias(typ).(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := types.Unalias(pointer.Elem()).(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	return current == nil || named.Obj().Pkg().Path() != current.Path()
}

func emitOCPFactories(pkgs []*packageFiles, cfg Config) ([]Issue, map[string]bool) {
	issues := []Issue{}
	positions := map[string]bool{}
	for _, pkg := range pkgs {
		if pkg.info == nil || !pkg.typeComplete {
			continue
		}
		for _, file := range pkg.files {
			if !ocpFileEnabled(pkg, file, cfg) {
				continue
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || !isFactoryName(fn.Name.Name) || !functionReturnsInterface(fn, pkg.info) {
					continue
				}
				for _, stmt := range fn.Body.List {
					sw, ok := stmt.(*ast.SwitchStmt)
					if !ok {
						continue
					}
					cases := countCaseClauses(sw.Body)
					if cases <= cfg.MaxTypeSwitchCases || !factoryBranchProducts(sw.Body, functionSignature(fn, pkg.info), pkg.info, localFunctionDeclarations(pkg.files, pkg.info)) {
						continue
					}
					pos := pkg.fset.Position(sw.Pos())
					positions[positionKey(pos)] = true
					issues = append(issues, issueAt(pkg.fset, sw, Issue{Rule: RuleOCP, Check: CheckOCPClosedFactory, Severity: SeverityWarning,
						Message:  fmt.Sprintf("factory %q has %d hardcoded branches for an interface result (max %d); register constructors or inject a registry", fn.Name.Name, cases, cfg.MaxTypeSwitchCases),
						Evidence: fmt.Sprintf("closed-factory:function=%s;cases=%d;max=%d", fn.Name.Name, cases, cfg.MaxTypeSwitchCases), Metrics: []Metric{{Name: "cases", Value: float64(cases), Threshold: float64(cfg.MaxTypeSwitchCases)}}}))
				}
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					literal, ok := node.(*ast.CompositeLit)
					if !ok || pkg.info.TypeOf(literal.Type) == nil {
						return true
					}
					mapType, ok := pkg.info.TypeOf(literal.Type).Underlying().(*types.Map)
					if !ok || len(literal.Elts) <= cfg.MaxTypeSwitchCases || (!factoryMapValue(mapType) || !factoryTableProducts(literal, mapType, pkg)) {
						return true
					}
					pos := pkg.fset.Position(literal.Pos())
					if positions[positionKey(pos)] {
						return true
					}
					positions[positionKey(pos)] = true
					issues = append(issues, issueAt(pkg.fset, literal, Issue{Rule: RuleOCP, Check: CheckOCPClosedFactory, Severity: SeverityWarning,
						Message:  fmt.Sprintf("factory %q contains a static constructor table with %d entries (max %d); expose registration or inject the registry", fn.Name.Name, len(literal.Elts), cfg.MaxTypeSwitchCases),
						Evidence: fmt.Sprintf("closed-factory:function=%s;map_entries=%d;max=%d", fn.Name.Name, len(literal.Elts), cfg.MaxTypeSwitchCases), Metrics: []Metric{{Name: "map_entries", Value: float64(len(literal.Elts)), Threshold: float64(cfg.MaxTypeSwitchCases)}}}))
					return true
				})
			}
		}
	}
	return issues, positions
}

func factoryMapValue(mapType *types.Map) bool {
	value := mapType.Elem()
	if signature, ok := value.(*types.Signature); ok {
		if signature.Results() == nil {
			return false
		}
		for index := 0; index < signature.Results().Len(); index++ {
			if factoryProductInterface(signature.Results().At(index).Type()) {
				return true
			}
		}
	}
	return factoryProductInterface(value)
}

func isFactoryName(name string) bool {
	for _, prefix := range []string{"New", "Make", "Create", "Build", "Parse"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// Domain data mapping uses observational accessors, not injected operations.
// The package role alone cannot exempt a behavioral collaborator.
func domainMapperMethods(typ types.Type, methods []*types.Func, cfg Config) bool {
	if !isDomainStructType(typ, cfg) || len(methods) == 0 {
		return false
	}
	for _, method := range methods {
		sig, ok := method.Type().(*types.Signature)
		if !ok || sig.Params().Len() != 0 || sig.Results().Len() == 0 || domainMutationName(method.Name()) {
			return false
		}
		for i := 0; i < sig.Results().Len(); i++ {
			if types.Identical(sig.Results().At(i).Type(), types.Universe.Lookup("error").Type()) {
				return false
			}
		}
	}
	return true
}

func factoryProductInterface(typ types.Type) bool {
	return isInterface(typ) && !types.Identical(typ, types.Universe.Lookup("error").Type())
}
func functionSignature(fn *ast.FuncDecl, info *types.Info) *types.Signature {
	object, ok := info.Defs[fn.Name].(*types.Func)
	if !ok {
		return nil
	}
	sig, _ := object.Type().(*types.Signature)
	return sig
}
func factoryBranchProducts(body ast.Node, sig *types.Signature, info *types.Info, locals map[*types.Func]*ast.FuncDecl) bool {
	if sig == nil {
		return false
	}
	for i := 0; i < sig.Results().Len(); i++ {
		abstraction := sig.Results().At(i).Type()
		if !factoryProductInterface(abstraction) {
			continue
		}
		products := map[string]bool{}
		collectFactoryReturns(body, i, abstraction, info, locals, products, map[*ast.FuncDecl]bool{})
		if len(products) > 1 {
			return true
		}
	}
	return false
}
func collectFactoryReturns(body ast.Node, index int, abstraction types.Type, info *types.Info, locals map[*types.Func]*ast.FuncDecl, products map[string]bool, visiting map[*ast.FuncDecl]bool) {
	if body == nil {
		return
	}
	ast.Inspect(body, func(node ast.Node) bool {
		if _, closure := node.(*ast.FuncLit); closure {
			return false
		}
		ret, ok := node.(*ast.ReturnStmt)
		if !ok || index >= len(ret.Results) {
			return true
		}
		collectFactoryProduct(ret.Results[index], index, abstraction, info, locals, products, visiting)
		return true
	})
}
func collectFactoryProduct(expr ast.Expr, index int, abstraction types.Type, info *types.Info, locals map[*types.Func]*ast.FuncDecl, products map[string]bool, visiting map[*ast.FuncDecl]bool) {
	typ := info.TypeOf(expr)
	if typ != nil && !isInterface(typ) && concreteTypeCandidate(typ) && types.AssignableTo(typ, abstraction) {
		products[canonicalTypeKey(typ)] = true
		return
	}
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return
	}
	helper := locals[calledFunction(call.Fun, info)]
	if helper == nil || helper.Body == nil || visiting[helper] {
		return
	}
	visiting[helper] = true
	defer delete(visiting, helper)
	collectFactoryReturns(helper.Body, index, abstraction, info, locals, products, visiting)
}
func factoryTableProducts(literal *ast.CompositeLit, mapType *types.Map, pkg *packageFiles) bool {
	locals := localFunctionDeclarations(pkg.files, pkg.info)
	value := mapType.Elem()
	if sig, ok := value.(*types.Signature); ok {
		for i := 0; i < sig.Results().Len(); i++ {
			abstraction := sig.Results().At(i).Type()
			if !factoryProductInterface(abstraction) {
				continue
			}
			products := map[string]bool{}
			for _, entry := range literal.Elts {
				pair, ok := entry.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if closure, ok := pair.Value.(*ast.FuncLit); ok {
					collectFactoryReturns(closure.Body, i, abstraction, pkg.info, locals, products, map[*ast.FuncDecl]bool{})
					continue
				}
				if helper := locals[calledFunction(pair.Value, pkg.info)]; helper != nil && helper.Body != nil {
					collectFactoryReturns(helper.Body, i, abstraction, pkg.info, locals, products, map[*ast.FuncDecl]bool{})
				}
			}
			if len(products) > 1 {
				return true
			}
		}
		return false
	}
	products := map[string]bool{}
	for _, entry := range literal.Elts {
		pair, ok := entry.(*ast.KeyValueExpr)
		if ok {
			collectFactoryProduct(pair.Value, 0, value, pkg.info, locals, products, map[*ast.FuncDecl]bool{})
		}
	}
	return len(products) > 1
}
