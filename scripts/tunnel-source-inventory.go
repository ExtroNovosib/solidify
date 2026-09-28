//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type D struct {
	Path    string `json:"path"`
	Package string `json:"package"`
	Kind    string `json:"kind"`
	Symbol  string `json:"symbol"`
	Owner   string `json:"owner,omitempty"`
	Type    string `json:"type,omitempty"`
	Line    int    `json:"line"`
	EndLine int    `json:"endLine"`
}

func main() {
	root := os.Args[1]
	fset := token.NewFileSet()
	var records []D
	expr := func(x ast.Expr) string {
		if x == nil {
			return ""
		}
		var b bytes.Buffer
		_ = format.Node(&b, fset, x)
		return b.String()
	}
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		pkg := "github.com/fortunnels/tunnel/" + filepath.ToSlash(filepath.Dir(rel))
		add := func(kind, sym, own, typ string, n ast.Node) {
			records = append(records, D{filepath.ToSlash(rel), pkg, kind, sym, own, typ, fset.Position(n.Pos()).Line, fset.Position(n.End()).Line})
		}
		for _, decl := range f.Decls {
			switch n := decl.(type) {
			case *ast.FuncDecl:
				owner := ""
				if n.Recv != nil {
					owner = expr(n.Recv.List[0].Type)
					owner = strings.TrimPrefix(owner, "*")
				}
				add("function", n.Name.Name, owner, "", n)
				ast.Inspect(n.Body, func(node ast.Node) bool {
					assertion, ok := node.(*ast.TypeAssertExpr)
					if ok && assertion.Type != nil {
						add("assertion", n.Name.Name, owner, expr(assertion.Type), assertion)
					}
					return true
				})
				if n.Type.Params != nil {
					for _, field := range n.Type.Params.List {
						for _, name := range field.Names {
							add("parameter", name.Name, n.Name.Name, expr(field.Type), field)
						}
					}
				}
			case *ast.GenDecl:
				for _, raw := range n.Specs {
					switch s := raw.(type) {
					case *ast.TypeSpec:
						add("type", s.Name.Name, "", expr(s.Type), s)
						if st, ok := s.Type.(*ast.StructType); ok {
							for _, field := range st.Fields.List {
								if len(field.Names) == 0 {
									add("field", expr(field.Type), s.Name.Name, expr(field.Type), field)
								}
								for _, name := range field.Names {
									add("field", name.Name, s.Name.Name, expr(field.Type), field)
								}
							}
						}
					case *ast.ImportSpec:
						add("import", strings.Trim(s.Path.Value, "\""), "", "", s)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	_ = json.NewEncoder(os.Stdout).Encode(records)
}
