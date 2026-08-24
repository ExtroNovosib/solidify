package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

var dipBuiltinDetailImports = []string{
	"database/sql",
	"database/sql/driver",
	"net/http",
	"os/exec",
}

var defaultDIPInfraErrorPackages = []string{
	"database/sql",
}

var defaultDIPTransportTypes = []string{
	"net/http.Request",
	"net/http.ResponseWriter",
	"database/sql.Tx",
	"github.com/gin-gonic/gin.Context",
	"github.com/labstack/echo/v4.Context",
}

// CheckDIPProgram runs architecture-aware DIP checks that need package
// imports, module paths, or cross-file type information.
func CheckDIPProgram(pkgs []*packageFiles, cfg Config) []Issue {
	var issues []Issue
	if checkEnabled(cfg, CheckDIPLayerImport) {
		issues = append(issues, emitDIPLayerImport(pkgs, cfg)...)
	}
	var hidden []Issue
	hiddenSites := map[string]bool{}
	if checkEnabled(cfg, CheckDIPHiddenConstruction) || checkEnabled(cfg, CheckDIPWiringOutsideRoot) {
		hidden, hiddenSites = emitDIPHiddenConstruction(pkgs, cfg)
	}
	if checkEnabled(cfg, CheckDIPHiddenConstruction) {
		issues = append(issues, hidden...)
	}
	if checkEnabled(cfg, CheckDIPWiringOutsideRoot) {
		issues = append(issues, emitDIPWiringOutsideRoot(pkgs, cfg, hiddenSites)...)
	}
	if checkEnabled(cfg, CheckDIPInfraErrorLeak) {
		issues = append(issues, emitDIPInfraErrorLeak(pkgs, cfg)...)
	}
	if checkEnabled(cfg, CheckDIPTransportLeak) {
		issues = append(issues, emitDIPTransportLeak(pkgs, cfg)...)
	}
	return issues
}

func dipArchitectureEnabled(cfg Config) bool {
	return len(cfg.OCPLogicPackages) > 0 && len(cfg.OCPImplementationPackages) > 0
}

func dipInfraErrorPackages(cfg Config) []string {
	if len(cfg.DIPInfraErrorPackages) > 0 {
		return cfg.DIPInfraErrorPackages
	}
	return defaultDIPInfraErrorPackages
}

func dipTransportTypes(cfg Config) []string {
	if len(cfg.DIPTransportTypes) > 0 {
		return cfg.DIPTransportTypes
	}
	return defaultDIPTransportTypes
}

func emitDIPLayerImport(pkgs []*packageFiles, cfg Config) []Issue {
	if !dipArchitectureEnabled(cfg) {
		return nil
	}
	var issues []Issue
	for _, pkg := range pkgs {
		if pkg.pkgPath == "" || !matchesAnyPackagePattern(pkg.pkgPath, cfg.OCPLogicPackages) {
			continue
		}
		if matchesAnyPackagePattern(pkg.pkgPath, cfg.OCPCompositionRoots) {
			continue
		}
		for _, imported := range pkg.imports {
			if !dipForbiddenLogicImport(imported, cfg) {
				continue
			}
			locations := importRelatedLocations(pkg, []string{imported})
			start, end := importSpecSpan(pkg, imported)
			issues = append(issues, issueSpan(pkg.fset, start, end, Issue{
				Rule:     RuleDIP,
				Check:    CheckDIPLayerImport,
				Severity: SeverityWarning,
				Message: fmt.Sprintf(
					"logic package %s must not import implementation detail %q; depend on a boundary abstraction and wire %q at the composition root",
					pkg.pkgPath, imported, imported,
				),
				Evidence: fmt.Sprintf("layer-import:from=%s;to=%s", pkg.pkgPath, imported),
				Related:  locations,
			}))
		}
	}
	return issues
}

func dipForbiddenLogicImport(importPath string, cfg Config) bool {
	for _, builtin := range dipBuiltinDetailImports {
		if importPath == builtin {
			return true
		}
	}
	return matchesAnyPackagePattern(importPath, cfg.OCPImplementationPackages)
}

func emitDIPHiddenConstruction(pkgs []*packageFiles, cfg Config) ([]Issue, map[string]bool) {
	sites := map[string]bool{}
	if !dipArchitectureEnabled(cfg) {
		return nil, sites
	}
	var issues []Issue
	for _, pkg := range pkgs {
		if pkg.info == nil || pkg.pkgPath == "" ||
			!matchesAnyPackagePattern(pkg.pkgPath, cfg.OCPLogicPackages) ||
			matchesAnyPackagePattern(pkg.pkgPath, cfg.OCPCompositionRoots) {
			continue
		}
		for _, file := range pkg.files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || !isDIPFactoryFunction(fn.Name.Name) {
					continue
				}
				for _, call := range dipFactoryConstructionCalls(fn, pkg, cfg) {
					pos := pkg.fset.Position(call.Pos())
					key := positionKey(pos)
					sites[key] = true
					callee := dipCallDescription(call, pkg.info)
					issues = append(issues, issueAt(pkg.fset, call, Issue{
						Rule:     RuleDIP,
						Check:    CheckDIPHiddenConstruction,
						Severity: SeverityWarning,
						Message: fmt.Sprintf(
							"factory %q constructs %s inside its body instead of receiving it via a parameter; inject the implementation at the composition root",
							fn.Name.Name, callee,
						),
						Evidence: fmt.Sprintf("hidden-construction:factory=%s;callee=%s", fn.Name.Name, callee),
					}))
				}
			}
		}
	}
	return issues, sites
}

func emitDIPWiringOutsideRoot(pkgs []*packageFiles, cfg Config, hiddenSites map[string]bool) []Issue {
	if !dipArchitectureEnabled(cfg) {
		return nil
	}
	var issues []Issue
	for _, pkg := range pkgs {
		if pkg.info == nil || pkg.pkgPath == "" ||
			matchesAnyPackagePattern(pkg.pkgPath, cfg.OCPImplementationPackages) ||
			matchesAnyPackagePattern(pkg.pkgPath, cfg.OCPCompositionRoots) {
			continue
		}
		for _, file := range pkg.files {
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || !dipIsImplementationConstruction(call, pkg, cfg) {
					return true
				}
				pos := pkg.fset.Position(call.Pos())
				if hiddenSites[positionKey(pos)] {
					return true
				}
				callee := dipCallDescription(call, pkg.info)
				issues = append(issues, issueAt(pkg.fset, call, Issue{
					Rule:     RuleDIP,
					Check:    CheckDIPWiringOutsideRoot,
					Severity: SeverityWarning,
					Message: fmt.Sprintf(
						"package %s wires implementation %s outside a composition root; move construction to cmd/ or app/ wiring",
						pkg.pkgPath, callee,
					),
					Evidence: fmt.Sprintf("wiring-outside-root:package=%s;callee=%s", pkg.pkgPath, callee),
				}))
				return true
			})
		}
	}
	return issues
}

func emitDIPInfraErrorLeak(pkgs []*packageFiles, cfg Config) []Issue {
	if len(cfg.OCPLogicPackages) == 0 {
		return nil
	}
	infraPackages := dipInfraErrorPackages(cfg)
	var issues []Issue
	for _, pkg := range pkgs {
		if pkg.info == nil || pkg.pkgPath == "" || !matchesAnyPackagePattern(pkg.pkgPath, cfg.OCPLogicPackages) {
			continue
		}
		if matchesAnyPackagePattern(pkg.pkgPath, cfg.OCPCompositionRoots) {
			continue
		}
		for _, file := range pkg.files {
			ast.Inspect(file, func(node ast.Node) bool {
				switch expr := node.(type) {
				case *ast.CallExpr:
					if issue, ok := dipInfraErrorCallIssue(expr, pkg, infraPackages); ok {
						issues = append(issues, issue)
					}
				case *ast.BinaryExpr:
					if expr.Op != token.EQL && expr.Op != token.NEQ {
						return true
					}
					if issue, ok := dipInfraErrorCompareIssue(expr, pkg, infraPackages); ok {
						issues = append(issues, issue)
					}
				}
				return true
			})
		}
	}
	return issues
}

func emitDIPTransportLeak(pkgs []*packageFiles, cfg Config) []Issue {
	if len(cfg.OCPLogicPackages) == 0 {
		return nil
	}
	transportTypes := dipTransportTypes(cfg)
	var issues []Issue
	for _, pkg := range pkgs {
		if pkg.info == nil || pkg.pkgPath == "" || !matchesAnyPackagePattern(pkg.pkgPath, cfg.OCPLogicPackages) {
			continue
		}
		if matchesAnyPackagePattern(pkg.pkgPath, cfg.OCPCompositionRoots) {
			continue
		}
		for _, file := range pkg.files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				if issue, ok := dipTransportFieldListIssue(fn.Type.Params, "parameter", fn, pkg, transportTypes); ok {
					issues = append(issues, issue)
				}
				if fn.Type.Results != nil {
					if issue, ok := dipTransportFieldListIssue(fn.Type.Results, "result", fn, pkg, transportTypes); ok {
						issues = append(issues, issue)
					}
				}
			}
		}
	}
	return issues
}

func isDIPFactoryFunction(name string) bool {
	return strings.HasPrefix(name, "New") || strings.HasPrefix(name, "Provide")
}

func isDIPWiringCallee(name string) bool {
	return strings.HasPrefix(name, "New") || strings.HasPrefix(name, "Provide") ||
		strings.HasPrefix(name, "Open") || strings.HasPrefix(name, "Must")
}

func dipFactoryConstructionCalls(fn *ast.FuncDecl, pkg *packageFiles, cfg Config) []*ast.CallExpr {
	var calls []*ast.CallExpr
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !dipIsImplementationConstruction(call, pkg, cfg) {
			return true
		}
		if dipCallInStructFieldInit(fn.Body, call) {
			calls = append(calls, call)
		}
		return true
	})
	return calls
}

func dipIsImplementationConstruction(call *ast.CallExpr, pkg *packageFiles, cfg Config) bool {
	if pkg.info == nil {
		return false
	}
	calleeName := dipCalleeName(call.Fun)
	if calleeName == "" || !isDIPWiringCallee(calleeName) {
		return false
	}
	pkgPath := dipCallPackagePath(call, pkg.info)
	return pkgPath != "" && matchesAnyPackagePattern(pkgPath, cfg.OCPImplementationPackages)
}

func dipCallInStructFieldInit(body *ast.BlockStmt, call *ast.CallExpr) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.KeyValueExpr:
			if dipExprContains(n.Value, call) {
				found = true
				return false
			}
		case *ast.AssignStmt:
			for _, rhs := range n.Rhs {
				if !dipExprContains(rhs, call) {
					continue
				}
				for _, lhs := range n.Lhs {
					if _, ok := lhs.(*ast.SelectorExpr); ok {
						found = true
						return false
					}
				}
			}
		}
		return true
	})
	return found
}

func dipExprContains(root ast.Expr, target ast.Node) bool {
	contains := false
	ast.Inspect(root, func(node ast.Node) bool {
		if node == target {
			contains = true
			return false
		}
		return true
	})
	return contains
}

func dipInfraErrorCallIssue(call *ast.CallExpr, pkg *packageFiles, infraPackages []string) (Issue, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || pkg.info == nil {
		return Issue{}, false
	}
	pkgPath, name := dipSelectorPackage(sel, pkg.info)
	if pkgPath == "" || name == "" {
		return Issue{}, false
	}
	isErrorsHelper := pkgPath == "errors" && (name == "Is" || name == "As")
	if !isErrorsHelper {
		return Issue{}, false
	}
	for _, arg := range call.Args {
		if issue, ok := dipInfraSelectorIssue(arg, pkg, infraPackages, "errors."+name); ok {
			return issue, true
		}
	}
	return Issue{}, false
}

func dipInfraErrorCompareIssue(expr *ast.BinaryExpr, pkg *packageFiles, infraPackages []string) (Issue, bool) {
	if issue, ok := dipInfraSelectorIssue(expr.X, pkg, infraPackages, "=="); ok {
		return issue, true
	}
	return dipInfraSelectorIssue(expr.Y, pkg, infraPackages, "==")
}

func dipInfraSelectorIssue(expr ast.Expr, pkg *packageFiles, infraPackages []string, context string) (Issue, bool) {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || pkg.info == nil {
		return Issue{}, false
	}
	pkgPath, name := dipSelectorPackage(sel, pkg.info)
	if pkgPath == "" || !matchesAnyPackagePattern(pkgPath, infraPackages) {
		return Issue{}, false
	}
	symbol := pkgPath + "." + name
	return issueAt(pkg.fset, sel, Issue{
		Rule:     RuleDIP,
		Check:    CheckDIPInfraErrorLeak,
		Severity: SeverityWarning,
		Message: fmt.Sprintf(
			"logic package %s compares against infrastructure sentinel %s; map infra errors to domain errors at the adapter boundary",
			pkg.pkgPath, symbol,
		),
		Evidence: fmt.Sprintf("infra-error-leak:package=%s;symbol=%s;context=%s", pkg.pkgPath, symbol, context),
	}), true
}

func dipTransportFieldListIssue(fields *ast.FieldList, role string, fn *ast.FuncDecl, pkg *packageFiles, transportTypes []string) (Issue, bool) {
	if fields == nil || pkg.info == nil {
		return Issue{}, false
	}
	for _, field := range fields.List {
		typeName := dipQualifiedTypeName(pkg.info.TypeOf(field.Type))
		if typeName == "" {
			continue
		}
		if !dipMatchesTransportType(typeName, transportTypes) {
			continue
		}
		fnName := fn.Name.Name
		if fn.Recv != nil && len(fn.Recv.List) > 0 {
			fnName = receiverTypeName(fn.Recv.List[0].Type) + "." + fnName
		}
		return issueAt(pkg.fset, field, Issue{
			Rule:     RuleDIP,
			Check:    CheckDIPTransportLeak,
			Severity: SeverityWarning,
			Message: fmt.Sprintf(
				"logic function %s exposes transport type %s in a %s; keep HTTP/SQL/gin/echo types at the adapter edge",
				fnName, typeName, role,
			),
			Evidence: fmt.Sprintf("transport-leak:package=%s;function=%s;type=%s;role=%s", pkg.pkgPath, fnName, typeName, role),
		}), true
	}
	return Issue{}, false
}

func dipMatchesTransportType(typeName string, patterns []string) bool {
	normalized := strings.TrimPrefix(typeName, "*")
	for _, pattern := range patterns {
		if typeName == pattern || normalized == strings.TrimPrefix(pattern, "*") {
			return true
		}
		if strings.HasSuffix(pattern, "."+normalized) {
			return true
		}
	}
	return false
}

func dipQualifiedTypeName(t types.Type) string {
	if t == nil {
		return ""
	}
	if ptr, ok := t.(*types.Pointer); ok {
		if inner := dipQualifiedTypeName(ptr.Elem()); inner != "" {
			return "*" + inner
		}
	}
	if named, ok := t.(*types.Named); ok && named.Obj() != nil && named.Obj().Pkg() != nil {
		return named.Obj().Pkg().Path() + "." + named.Obj().Name()
	}
	return ""
}

func dipCallDescription(call *ast.CallExpr, info *types.Info) string {
	name := dipCalleeName(call.Fun)
	pkgPath := dipCallPackagePath(call, info)
	if pkgPath != "" {
		return pkgPath + "." + name
	}
	return name
}

func dipCalleeName(fun ast.Expr) string {
	switch expr := fun.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.SelectorExpr:
		return expr.Sel.Name
	default:
		return ""
	}
}

func dipCallPackagePath(call *ast.CallExpr, info *types.Info) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		if obj, ok := info.Uses[fun]; ok && obj != nil && obj.Pkg() != nil {
			return obj.Pkg().Path()
		}
	case *ast.SelectorExpr:
		return dipSelectorPackagePath(fun, info)
	}
	return ""
}

func dipSelectorPackage(sel *ast.SelectorExpr, info *types.Info) (pkgPath, name string) {
	if sel == nil || info == nil {
		return "", ""
	}
	if selection, ok := info.Selections[sel]; ok && selection != nil && selection.Obj() != nil && selection.Obj().Pkg() != nil {
		return selection.Obj().Pkg().Path(), selection.Obj().Name()
	}
	if obj, ok := info.Uses[sel.Sel]; ok && obj != nil && obj.Pkg() != nil {
		return obj.Pkg().Path(), obj.Name()
	}
	return "", ""
}

func dipSelectorPackagePath(sel *ast.SelectorExpr, info *types.Info) string {
	pkgPath, _ := dipSelectorPackage(sel, info)
	if pkgPath != "" {
		return pkgPath
	}
	if id, ok := sel.X.(*ast.Ident); ok {
		if obj, ok := info.Uses[id]; ok {
			if pkgName, ok := obj.(*types.PkgName); ok && pkgName.Imported() != nil {
				return pkgName.Imported().Path()
			}
		}
	}
	return ""
}
