package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestIssueAtSetsEndRange(t *testing.T) {
	const src = `package p

type Widget struct{}

func (w *Widget) Work() {}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "widget.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var typeSpec *ast.TypeSpec
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			if ts, ok := spec.(*ast.TypeSpec); ok {
				typeSpec = ts
			}
		}
	}
	if typeSpec == nil {
		t.Fatal("missing type spec")
	}

	issue := issueAt(fset, typeSpec, Issue{Check: CheckSRPLargeType, Message: "large"})
	if issue.Pos.Line == 0 || issue.End.Line == 0 {
		t.Fatalf("expected start and end lines, got %+v %+v", issue.Pos, issue.End)
	}
	if issue.End.Offset <= issue.Pos.Offset {
		t.Fatalf("expected end after start: %+v %+v", issue.Pos, issue.End)
	}
}

func TestIssueSpanSetsEndRange(t *testing.T) {
	fset := token.NewFileSet()
	file := fset.AddFile("a.go", fset.Base(), 20)
	start := file.Pos(1)
	end := file.Pos(10)
	issue := issueSpan(fset, start, end, Issue{Check: CheckSRPDataClump, Message: "clump"})
	if issue.Pos.Line != 1 || issue.Pos.Column != 2 || issue.End.Line != 1 || issue.End.Column != 11 {
		t.Fatalf("unexpected span: %+v %+v", issue.Pos, issue.End)
	}
}
