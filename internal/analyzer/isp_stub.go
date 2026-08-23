package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
)

func checkISPStubImplementation(fset *token.FileSet, files []*ast.File, info *types.Info, cfg Config, pkg *packageFiles) []Issue {
	if info == nil {
		return nil
	}
	var issues []Issue
	for _, f := range files {
		if skipGenerated(pkg, f) {
			continue
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Body == nil {
				continue
			}
			ifaceName, ok := qualifyingInterfaceForMethod(fn, info, cfg.ISPMinMethods)
			if !ok {
				continue
			}
			if stmt, kind := stubStatement(fn.Body, info); stmt != nil {
				issue := issueAt(fset, fn, Issue{
					Rule:     RuleISP,
					Check:    CheckISPStubImplementation,
					Severity: SeverityWarning,
					Message: fmt.Sprintf(
						"method %q %s on interface %q: the type is forced to satisfy an operation it does not meaningfully support; split the interface so implementers only declare what they support",
						fn.Name.Name, kind, ifaceName,
					),
					Evidence: fmt.Sprintf("stub-implementation:method=%s;interface=%s;kind=%s", fn.Name.Name, ifaceName, kind),
				})
				issue.SuggestedFixes = []SuggestedFix{IgnoreSuppressionFix(issue, "accepted stub pending interface split")}
				issues = append(issues, issue)
			}
		}
	}
	return issues
}

func qualifyingInterfaceForMethod(fn *ast.FuncDecl, info *types.Info, minMethods int) (string, bool) {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return "", false
	}
	recv := info.TypeOf(fn.Recv.List[0].Type)
	if recv == nil {
		return "", false
	}
	elem := recv
	if p, ok := recv.(*types.Pointer); ok {
		elem = p.Elem()
	}
	bestName := ""
	bestMethods := 0
	for _, obj := range info.Defs {
		tn, ok := obj.(*types.TypeName)
		if !ok {
			continue
		}
		iface, ok := tn.Type().Underlying().(*types.Interface)
		if !ok {
			continue
		}
		iface.Complete()
		if iface.NumMethods() < minMethods {
			continue
		}
		for n := 0; n < iface.NumMethods(); n++ {
			if iface.Method(n).Name() != fn.Name.Name {
				continue
			}
			if !types.Implements(elem, iface) && !types.Implements(types.NewPointer(elem), iface) {
				continue
			}
			name := tn.Name()
			numMethods := iface.NumMethods()
			if bestName == "" || numMethods > bestMethods || (numMethods == bestMethods && name < bestName) {
				bestName = name
				bestMethods = numMethods
			}
		}
	}
	if bestName == "" {
		return "", false
	}
	return bestName, true
}

func stubStatement(body *ast.BlockStmt, info *types.Info) (ast.Stmt, string) {
	stmts := body.List
	if len(stmts) != 1 {
		return nil, ""
	}
	switch st := stmts[0].(type) {
	case *ast.ExprStmt:
		if call, ok := st.X.(*ast.CallExpr); ok {
			if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "panic" {
				if len(call.Args) == 1 {
					if _, ok := call.Args[0].(*ast.BasicLit); ok {
						return st, "unconditionally panics with a string literal"
					}
				}
				return st, "unconditionally panics"
			}
		}
	case *ast.ReturnStmt:
		for _, r := range st.Results {
			if returnsErrUnsupported(r, info) {
				return st, "returns errors.ErrUnsupported"
			}
		}
	}
	return nil, ""
}

func returnsErrUnsupported(expr ast.Expr, info *types.Info) bool {
	found := false
	var walk func(ast.Expr)
	walk = func(e ast.Expr) {
		if found || e == nil {
			return
		}
		switch node := e.(type) {
		case *ast.SelectorExpr:
			if selectorIsErrUnsupported(node, info) {
				found = true
				return
			}
			walk(node.X)
		case *ast.CallExpr:
			for _, arg := range node.Args {
				walk(arg)
			}
		default:
			walkErrUnsupportedChildren(node, walk)
		}
	}
	walk(expr)
	return found
}

func selectorIsErrUnsupported(node *ast.SelectorExpr, info *types.Info) bool {
	if node.Sel.Name != "ErrUnsupported" {
		return false
	}
	ident, ok := node.X.(*ast.Ident)
	if !ok {
		return false
	}
	pkg, ok := info.Uses[ident].(*types.PkgName)
	return ok && pkg.Imported().Path() == errorsPackagePath
}

func walkErrUnsupportedChildren(e ast.Expr, walk func(ast.Expr)) {
	switch node := e.(type) {
	case *ast.UnaryExpr:
		walk(node.X)
	case *ast.BinaryExpr:
		walk(node.X)
		walk(node.Y)
	case *ast.ParenExpr:
		walk(node.X)
	}
}
