package analyzer

import (
	"go/ast"
)

func functionComplexity(fn *ast.FuncDecl) int {
	if fn.Body == nil {
		return 1
	}
	complexity := 1
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if addsBranch(node) {
			complexity++
		}
		return true
	})
	return complexity
}

func addsBranch(node ast.Node) bool {
	if _, ok := node.(*ast.IfStmt); ok {
		return true
	}
	if _, ok := node.(*ast.ForStmt); ok {
		return true
	}
	if _, ok := node.(*ast.RangeStmt); ok {
		return true
	}
	if clause, ok := node.(*ast.CaseClause); ok && clause.List != nil {
		return true
	}
	if clause, ok := node.(*ast.CommClause); ok && clause.Comm != nil {
		return true
	}
	if expr, ok := node.(*ast.BinaryExpr); ok {
		return expr.Op.String() == "&&" || expr.Op.String() == "||"
	}
	return false
}

func allMethodsTrivialAccessors(methods []*ast.FuncDecl) bool {
	if len(methods) == 0 {
		return false
	}
	for _, method := range methods {
		if !isTrivialAccessorMethod(method) {
			return false
		}
	}
	return true
}

func isTrivialAccessorMethod(fn *ast.FuncDecl) bool {
	if fn.Body == nil {
		return true
	}
	if len(fn.Body.List) > 3 {
		return false
	}
	complex := false
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if complex {
			return false
		}
		var stop bool
		complex, stop = accessorInspectMarksComplex(node)
		return !stop
	})
	return !complex
}

func accessorInspectMarksComplex(node ast.Node) (complex bool, stop bool) {
	switch node := node.(type) {
	case *ast.CallExpr:
		return true, true
	case *ast.ForStmt, *ast.RangeStmt:
		return true, true
	default:
		return accessorInspectMarksComplexLoop(node)
	}
}

func accessorInspectMarksComplexLoop(node ast.Node) (complex bool, stop bool) {
	switch node := node.(type) {
	case *ast.SwitchStmt, *ast.TypeSwitchStmt:
		return true, true
	default:
		return accessorInspectMarksComplexControl(node)
	}
}

func accessorInspectMarksComplexControl(node ast.Node) (complex bool, stop bool) {
	switch node := node.(type) {
	case *ast.SelectStmt, *ast.GoStmt, *ast.DeferStmt:
		return true, true
	case *ast.IfStmt:
		if !ifStmtIsEarlyReturnGuard(node) {
			return true, true
		}
	}
	return false, false
}

func ifStmtIsEarlyReturnGuard(stmt *ast.IfStmt) bool {
	if stmt.Init != nil || stmt.Else != nil {
		return false
	}
	if len(stmt.Body.List) != 1 {
		return false
	}
	_, condReturn := stmt.Body.List[0].(*ast.ReturnStmt)
	return condReturn
}
