package analyzer

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func parseSource(t *testing.T, src string) (*token.FileSet, []*ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse source: %v", err)
	}
	return fset, []*ast.File{f}
}

func typeCheckSource(t *testing.T, fset *token.FileSet, files []*ast.File) *types.Info {
	t.Helper()
	info := &types.Info{
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
		Types:      map[ast.Expr]types.TypeAndValue{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	if _, err := (&types.Config{Importer: importer.Default()}).Check("p", fset, files, info); err != nil {
		t.Fatalf("type check: %v", err)
	}
	return info
}

func testdataDir(tb testing.TB, name string) string {
	tb.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		tb.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", name)
}

func initTempModule(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module tempmod\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func loadWorkspaceDir(t *testing.T, dir string, includeTests bool, mode string) []*packageFiles {
	t.Helper()
	initTempModule(t, dir)
	pkgs, _, err := LoadWorkspace([]string{dir}, includeTests, mode)
	if err != nil {
		t.Fatal(err)
	}
	return pkgs
}

func allRulesEnabled() map[Rule]bool {
	enabled := make(map[Rule]bool, len(All))
	for _, r := range All {
		enabled[r] = true
	}
	return enabled
}

func issuesByRule(issues []Issue) map[Rule]int {
	counts := make(map[Rule]int)
	for _, is := range issues {
		counts[is.Rule]++
	}
	return counts
}
