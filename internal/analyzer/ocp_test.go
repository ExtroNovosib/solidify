package analyzer

import (
	"go/ast"
	"path/filepath"
	"strings"
	"testing"
)

func TestCountCaseClauses(t *testing.T) {
	fset, files := parseSource(t, `package p

func f(x any) {
	switch x.(type) {
	case int:
	case string:
	case bool:
	default:
	}
}
`)
	fn := files[0].Decls[0].(*ast.FuncDecl)
	sw := fn.Body.List[0].(*ast.TypeSwitchStmt)
	if n := countCaseClauses(sw.Body); n != 3 {
		t.Errorf("countCaseClauses = %d, want 3", n)
	}
	_ = fset
}

func TestCountCaseClauses_MultipleTypesInOneClause(t *testing.T) {
	fset, files := parseSource(t, `package p

func f(x any) {
	switch x.(type) {
	case int, string:
	case bool:
	}
}
`)
	fn := files[0].Decls[0].(*ast.FuncDecl)
	sw := fn.Body.List[0].(*ast.TypeSwitchStmt)
	if n := countCaseClauses(sw.Body); n != 3 {
		t.Errorf("countCaseClauses = %d, want 3", n)
	}
	_ = fset
}

func TestCheckOCP_TypeSwitch(t *testing.T) {
	src := `package p

func f(x any) {
	switch x.(type) {
	case int:
	case string:
	case bool:
	case float64:
	case complex128:
	}
}
`
	fset, files := parseSource(t, src)
	cfg := DefaultConfig()
	cfg.MaxTypeSwitchCases = 3
	issues := CheckOCP(fset, files, cfg)
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1", len(issues))
	}
	if issues[0].Rule != RuleOCP || !strings.Contains(issues[0].Message, "type switch has 5 cases") {
		t.Errorf("unexpected issue: %+v", issues[0])
	}
}

func TestCheckOCP_TypeAssertChain(t *testing.T) {
	src := `package p

type A struct{}
type B struct{}
type C struct{}
type D struct{}
type E struct{}

func f(x any) {
	if _, ok := x.(*A); ok {
	} else if _, ok := x.(*B); ok {
	} else if _, ok := x.(*C); ok {
	} else if _, ok := x.(*D); ok {
	} else if _, ok := x.(*E); ok {
	}
}

`
	fset, files := parseSource(t, src)
	cfg := DefaultConfig()
	cfg.MaxTypeSwitchCases = 3
	issues := CheckOCP(fset, files, cfg)
	if len(issues) != 1 || !strings.Contains(issues[0].Message, "if/else-if chain has 5 type assertions") {
		t.Fatalf("unexpected issues: %v", issues)
	}
}

func TestCheckOCP_StandaloneAssertions(t *testing.T) {
	src := `package p
type Event interface{}
type A struct{}
type B struct{}
type C struct{}
type D struct{}
type E struct{}
func a(x Event) { _ = x.(A) }
func b(x Event) { _ = x.(B) }
func c(x Event) { _ = x.(C) }
func d(x Event) { _ = x.(D) }
func e(x Event) { _ = x.(E) }
`
	fset, files := parseSource(t, src)
	cfg := DefaultConfig()
	cfg.MaxTypeSwitchCases = 3
	issues := CheckOCP(fset, files, cfg)
	if len(issues) != 1 || !strings.Contains(issues[0].Message, "dispatches on") {
		t.Fatalf("unexpected issues: %v", issues)
	}
}

func TestCheckOCP_DynamicJSONShapeAssertionsAreNotCorrelated(t *testing.T) {
	src := `package p

func Verify(data map[string]any) {
	_, _ = data["role"].(string)
	_, _ = data["profile"].(map[string]any)
	_, _ = data["intakes"].([]map[string]any)
	_, _ = data["watchlist"].([]map[string]any)
	_, _ = data["comments"].([]map[string]any)
}
`
	fset, files := parseSource(t, src)
	info := typeCheckSource(t, fset, files)
	pkg := &packageFiles{
		fset: fset, files: files, info: info, typeComplete: true,
		pkgPath: "example.com/p", generated: map[*ast.File]bool{},
	}
	for _, issue := range CheckOCPProgram([]*packageFiles{pkg}, DefaultConfig()) {
		if issue.Check == CheckOCPTypeDispatch {
			t.Fatalf("unrelated JSON shape assertions were correlated: %v", issue)
		}
	}
}

func TestCheckOCPSyntaxDoesNotCorrelateUnrelatedLocalNames(t *testing.T) {
	src := `package p
type A struct{}
type B struct{}
type C struct{}
type D struct{}
type E struct{}
func first() { x := any(A{}); _ = x.(A); _ = x.(B); _ = x.(C) }
func second() { x := any(D{}); _ = x.(D); _ = x.(E) }
`
	fset, files := parseSource(t, src)
	cfg := DefaultConfig()
	cfg.MaxTypeSwitchCases = 3
	if issues := CheckOCP(fset, files, cfg); len(issues) != 0 {
		t.Fatalf("unrelated lexical bindings were correlated: %v", issues)
	}
}

func TestCheckOCPSyntaxCorrelatesExplicitNamedType(t *testing.T) {
	src := `package p
type Event interface{}
type A struct{}
type B struct{}
type C struct{}
type D struct{}
type E struct{}
func first(x Event) { _ = x.(A); _ = x.(B); _ = x.(C) }
func second(x Event) { _ = x.(D); _ = x.(E) }
`
	fset, files := parseSource(t, src)
	cfg := DefaultConfig()
	cfg.MaxTypeSwitchCases = 3
	if issues := CheckOCP(fset, files, cfg); len(issues) != 1 || issues[0].Check != CheckOCPTypeDispatch {
		t.Fatalf("explicit named source did not correlate: %v", issues)
	}
}

func TestIsTypeAssertionIf(t *testing.T) {
	fset, files := parseSource(t, `package p

func f(x any) { if v, ok := x.(*int); ok { _ = v } }
`)
	fn := files[0].Decls[0].(*ast.FuncDecl)
	if !isTypeAssertionIf(fn.Body.List[0].(*ast.IfStmt)) {
		t.Error("isTypeAssertionIf = false, want true")
	}
	_ = fset
}

func TestCheckOCP_StdlibConcreteParameterIgnored(t *testing.T) {
	src := `package p

import "net/http"

func Serve(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_ = r.Method
}
`
	fset, files := parseSource(t, src)
	info := typeCheckSource(t, fset, files)
	pkgs := []*packageFiles{{
		fset:         fset,
		files:        files,
		info:         info,
		typeComplete: true,
	}}
	issues := emitOCPConcreteParameters(pkgs, DefaultConfig())
	for _, issue := range issues {
		if issue.Check == CheckOCPConcreteParameter {
			t.Fatalf("stdlib request/response should not trigger concrete-parameter: %+v", issue)
		}
	}
}

func TestCheckOCP_AllowlistedConcreteParameterIgnored(t *testing.T) {
	fset, files := parseSource(t, `package p

type Client struct{}
func (*Client) Read() {}
func (*Client) Write() {}

func Use(client *Client) {
	client.Read()
	client.Write()
}
`)
	info := typeCheckSource(t, fset, files)
	pkgs := []*packageFiles{{
		fset:         fset,
		files:        files,
		info:         info,
		typeComplete: true,
	}}
	cfg := DefaultConfig()
	cfg.DIPAllowDependencies = []string{"Client"}
	issues := emitOCPConcreteParameters(pkgs, cfg)
	for _, issue := range issues {
		if issue.Check == CheckOCPConcreteParameter {
			t.Fatalf("allowlisted concrete parameter should not be reported: %+v", issue)
		}
	}
}

func TestMatchingInterfaceRejectsIncompatibleMethodSignature(t *testing.T) {
	issues := concreteParameterFindings(t, map[string]string{
		"dep/dep.go": `package dep
type Client struct{}
func (*Client) Read() string { return "" }
func (*Client) Write(string) error { return nil }
`,
		"p/p.go": `package p
import "tempmod/dep"
type Wrong interface { Read() int; Write(string) error }
func Use(client *dep.Client) { _ = client.Read(); _ = client.Write("") }
`,
	})
	if len(issues) != 1 {
		t.Fatalf("issues = %v", issues)
	}
	if strings.Contains(issues[0].Message, "matching interface") {
		t.Fatalf("incompatible interface suggested: %s", issues[0].Message)
	}
}

// concreteParameterFindings type-checks a temporary module and returns its
// SOLID-O/concrete-parameter findings.
func concreteParameterFindings(t *testing.T, files map[string]string) []Issue {
	t.Helper()
	root := t.TempDir()
	writeModuleFixture(t, root, files)
	pkgs, _, err := LoadWorkspace([]string{root + "/..."}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	var issues []Issue
	for _, issue := range emitOCPConcreteParameters(pkgs, DefaultConfig()) {
		if issue.Check == CheckOCPConcreteParameter {
			issues = append(issues, issue)
		}
	}
	return issues
}

const concreteParameterClientSource = `package dep
type Client struct{}
func (*Client) Read() {}
func (*Client) Write() {}
type Value struct{}
func (Value) Read() {}
func (Value) Write() {}
`

func TestCheckOCP_SamePackageConcreteParameterIgnored(t *testing.T) {
	issues := concreteParameterFindings(t, map[string]string{
		"dep/dep.go": concreteParameterClientSource + `
func Use(client *Client) {
	client.Read()
	client.Write()
}
`,
		"p/p.go": `package p
import "tempmod/dep"
func Use(client *dep.Client) {
	client.Read()
	client.Write()
}
`,
	})
	if len(issues) != 1 || !strings.Contains(issues[0].Pos.Filename, filepath.Join("p", "p.go")) {
		t.Fatalf("findings = %v, want only the cross-package pointer parameter in p/p.go", issues)
	}
}

func TestCheckOCP_ValueConcreteParameterIgnored(t *testing.T) {
	issues := concreteParameterFindings(t, map[string]string{
		"dep/dep.go": concreteParameterClientSource,
		"p/p.go": `package p
import "tempmod/dep"
func UseValue(value dep.Value) {
	value.Read()
	value.Write()
}
func UsePointer(client *dep.Client) {
	client.Read()
	client.Write()
}
`,
	})
	if len(issues) != 1 || !strings.HasPrefix(issues[0].Evidence, "concrete-parameter:function=UsePointer;parameter=client;type=*tempmod/dep.Client;") {
		t.Fatalf("findings = %v, want only the pointer parameter", issues)
	}
}

func TestTypeAssertChain(t *testing.T) {
	fset, files := parseSource(t, `package p

type A struct{}
type B struct{}
func f(x any) { if _, ok := x.(*A); ok {} else if _, ok := x.(*B); ok {} }
`)
	fn := files[0].Decls[2].(*ast.FuncDecl)
	count, links := typeAssertChain(fn.Body.List[0].(*ast.IfStmt))
	if count != 2 || len(links) != 2 {
		t.Errorf("typeAssertChain = (%d, %d links), want (2, 2)", count, len(links))
	}
	_ = fset
}

func TestTypeAssertChain_DifferentOperandsNotCombined(t *testing.T) {
	fset, files := parseSource(t, `package p

type A struct{}
type B struct{}
func f(first, second any) { if _, ok := first.(*A); ok {} else if _, ok := second.(*B); ok {} }
`)
	fn := files[0].Decls[2].(*ast.FuncDecl)
	count, links := typeAssertChain(fn.Body.List[0].(*ast.IfStmt))
	if count != 1 || len(links) != 1 {
		t.Errorf("typeAssertChain = (%d, %d links), want (1, 1)", count, len(links))
	}
	_ = fset
}

func TestMatchPackagePattern_BasePackage(t *testing.T) {
	pattern := "example.com/diparch/service/**"
	if !matchPackagePattern("example.com/diparch/service", pattern) {
		t.Fatal("base package should match /** pattern")
	}
	if !matchPackagePattern("example.com/diparch/service/sub", pattern) {
		t.Fatal("subpackage should match /** pattern")
	}
	if matchPackagePattern("example.com/diparch/services", pattern) {
		t.Fatal("similar prefix should not match /** pattern")
	}
	wildcardPattern := "example.com/diparch/internal/*/application/**"
	if !matchPackagePattern("example.com/diparch/internal/auth/application", wildcardPattern) {
		t.Fatal("wildcard /** pattern should match its base package")
	}
	if !matchPackagePattern("example.com/diparch/internal/auth/application/sub", wildcardPattern) {
		t.Fatal("wildcard /** pattern should match a child package")
	}
}

func TestTypeAssertChainTargets_ConditionAssertion(t *testing.T) {
	fset, files := parseSource(t, `package p

type A struct{}
type B struct{}
type C struct{}
type D struct{}
type E struct{}

func f(x any) {
	if x.(*A) != nil {
	} else if x.(*B) != nil {
	} else if x.(*C) != nil {
	} else if x.(*D) != nil {
	} else if x.(*E) != nil {
	}
}
`)
	fn := files[0].Decls[5].(*ast.FuncDecl)
	stmt := fn.Body.List[0].(*ast.IfStmt)
	info := typeCheckSource(t, fset, files)
	targets := typeAssertChainTargets(stmt, info)
	if len(targets) != 1 || !strings.Contains(targets[0], "A") {
		t.Fatalf("typeAssertChainTargets = %v, want asserted type A", targets)
	}

	cfg := DefaultConfig()
	cfg.MaxTypeSwitchCases = 3
	issues := CheckOCP(fset, files, cfg)
	if len(issues) != 1 || !strings.Contains(issues[0].Message, "if/else-if chain has 5 type assertions") {
		t.Fatalf("unexpected issues: %v", issues)
	}
}

const ocpStdlibVisitorSource = `package p

import "go/ast"

func visit(node ast.Node) string {
	switch node.(type) {
	case *ast.File:
		return "file"
	case *ast.FuncDecl:
		return "func"
	case *ast.GenDecl:
		return "gen"
	case *ast.BlockStmt:
		return "block"
	case *ast.ReturnStmt:
		return "return"
	case *ast.Ident:
		return "ident"
	}
	return ""
}
`

const ocpLocalShapeSource = `package p

type Shape interface{ Area() float64 }
type Circle struct{}
type Square struct{}
type Triangle struct{}
type Hexagon struct{}
type Ellipse struct{}

func (Circle) Area() float64   { return 0 }
func (Square) Area() float64   { return 0 }
func (Triangle) Area() float64 { return 0 }
func (Hexagon) Area() float64  { return 0 }
func (Ellipse) Area() float64  { return 0 }

func describe(shape Shape) string {
	switch shape.(type) {
	case Circle:
		return "circle"
	case Square:
		return "square"
	case Triangle:
		return "triangle"
	case Hexagon:
		return "hexagon"
	case Ellipse:
		return "ellipse"
	}
	return ""
}
`

func TestCheckOCPSyntaxAllowsStdlibVisitorDispatch(t *testing.T) {
	fset, files := parseSource(t, ocpStdlibVisitorSource)
	for _, issue := range CheckOCP(fset, files, DefaultConfig()) {
		if issue.Check == CheckOCPTypeDispatch {
			t.Fatalf("syntax-only go/ast visitor dispatch reported: %v", issue)
		}
	}
	cfg := DefaultConfig()
	cfg.OCPAllowDispatchTypes = []string{"Shape"}
	fset, files = parseSource(t, ocpLocalShapeSource)
	if issues := CheckOCP(fset, files, cfg); len(issues) != 0 {
		t.Fatalf("ocp.allow_dispatch_types did not apply to a syntax source: %v", issues)
	}
	if issues := CheckOCP(fset, files, DefaultConfig()); len(issues) != 1 || issues[0].Check != CheckOCPTypeDispatch {
		t.Fatalf("local five-case dispatch = %v, want one type-dispatch finding", issues)
	}
}

func TestCheckOCPSyntaxAndTypedDispatchParity(t *testing.T) {
	illTyped := "\nvar _ = undefinedSymbol\n"
	root := t.TempDir()
	writeModuleFixture(t, root, map[string]string{
		"visitor/visitor.go": ocpStdlibVisitorSource + illTyped,
		"shapes/shapes.go":   strings.Replace(ocpLocalShapeSource, "package p", "package shapes", 1) + illTyped,
	})
	for _, mode := range []string{"auto", "syntax"} {
		t.Run(mode, func(t *testing.T) {
			pkgs, _, err := LoadWorkspace([]string{root}, false, mode)
			if err != nil {
				t.Fatal(err)
			}
			for _, pkg := range pkgs {
				if pkg.typeComplete {
					t.Fatalf("fixture package %s unexpectedly type-checked", pkg.pkgPath)
				}
			}
			cfg := DefaultConfig()
			cfg.CacheEnabled = false
			cfg.AnalysisMode = mode
			plan, err := NewExecutionPlan(cfg, allRulesEnabled(), SurfaceCLI)
			if err != nil {
				t.Fatal(err)
			}
			issues, _ := RunPlan(pkgs, cfg, plan)
			var dispatch []Issue
			for _, issue := range issues {
				if issue.Check == CheckOCPTypeDispatch {
					dispatch = append(dispatch, issue)
				}
			}
			if len(dispatch) != 1 || !strings.Contains(filepath.ToSlash(dispatch[0].Pos.Filename), "shapes/shapes.go") {
				t.Fatalf("%s dispatch findings = %v, want exactly one on the local Shape switch", mode, dispatch)
			}
		})
	}
}
