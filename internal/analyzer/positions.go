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

// sourceOrderLess orders positions by filename and then offset. Raw token.Pos
// values also encode the order in which go/packages happened to parse a
// package's files, which varies between runs.
func sourceOrderLess(fset *token.FileSet, left, right token.Pos) bool {
	if fset == nil {
		return left < right
	}
	leftPosition, rightPosition := fset.Position(left), fset.Position(right)
	if leftPosition.Filename != rightPosition.Filename {
		return leftPosition.Filename < rightPosition.Filename
	}
	return leftPosition.Offset < rightPosition.Offset
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
