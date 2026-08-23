package analyzer

import (
	"go/ast"
	"go/token"
)

func positionRange(fset *token.FileSet, node ast.Node) (start, end token.Position) {
	if node == nil || fset == nil {
		return start, end
	}
	start = fset.Position(node.Pos())
	end = fset.Position(node.End())
	return start, end
}

func issueAt(fset *token.FileSet, node ast.Node, issue Issue) Issue {
	start, end := positionRange(fset, node)
	issue.Pos = start
	issue.End = end
	return issue
}

func issueSpan(fset *token.FileSet, start, end token.Pos, issue Issue) Issue {
	if fset == nil {
		return issue
	}
	if start != token.NoPos {
		issue.Pos = fset.Position(start)
	}
	if end != token.NoPos {
		issue.End = fset.Position(end)
	}
	return issue
}
