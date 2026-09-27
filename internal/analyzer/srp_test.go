package analyzer

import (
	"fmt"
	"go/ast"
	"strings"
	"testing"
)

func TestReceiverTypeName(t *testing.T) {
	fset, files := parseSource(t, `package p

type Foo struct{}
func (f Foo) A() {}
func (f *Foo) B() {}
`)
	recvFoo := files[0].Decls[1].(*ast.FuncDecl).Recv.List[0].Type
	recvPtrFoo := files[0].Decls[2].(*ast.FuncDecl).Recv.List[0].Type

	if got := receiverTypeName(recvFoo); got != "Foo" {
		t.Errorf("receiverTypeName(Foo) = %q, want Foo", got)
	}
	if got := receiverTypeName(recvPtrFoo); got != "Foo" {
		t.Errorf("receiverTypeName(*Foo) = %q, want Foo", got)
	}
	_ = fset
}

func TestReceiverTypeName_Generic(t *testing.T) {
	fset, files := parseSource(t, `package p

type Box[T any] struct{}
func (b *Box[T]) Put(T) {}
`)
	receiver := files[0].Decls[1].(*ast.FuncDecl).Recv.List[0].Type
	if got := receiverTypeName(receiver); got != "Box" {
		t.Errorf("receiverTypeName(*Box[T]) = %q, want Box", got)
	}
	_ = fset
}

// largeTypeFixture declares Widget with eleven heterogeneous fields and eleven
// exported methods, each with four branches and its own field, so methods,
// exported methods, fields, and WMC (11*5 = 55) exceed the defaults while TCC
// stays at zero.
func largeTypeFixture(pkgName string) string {
	fieldTypes := []string{"int", "string", "bool", "float64", "[]byte", "map[string]int", "chan int", "error", "rune", "uint", "[]string"}
	var source strings.Builder
	source.WriteString("package " + pkgName + "\n\ntype Widget struct {\n")
	for index, fieldType := range fieldTypes {
		fmt.Fprintf(&source, "\tfield%c %s\n", 'A'+index, fieldType)
	}
	source.WriteString("}\n")
	for index := range fieldTypes {
		fmt.Fprintf(&source, `
func (w *Widget) Method%c(value int) int {
	if value > 1 {
		value++
	}
	if value > 2 {
		value--
	}
	if value > 3 {
		value *= 2
	}
	if value > 4 {
		value /= 2
	}
	_ = w.field%c
	return value
}
`, 'A'+index, 'A'+index)
	}
	return source.String()
}

func TestCheckSRP_TooManyMethods(t *testing.T) {
	fset, files := parseSource(t, largeTypeFixture("p"))
	var large []Issue
	for _, issue := range CheckSRP(fset, files, DefaultConfig()) {
		if issue.Check == CheckSRPLargeType {
			large = append(large, issue)
		}
	}
	if len(large) != 1 {
		t.Fatalf("got %d large-type issues, want 1: %v", len(large), large)
	}
	issue := large[0]
	if issue.Rule != RuleSRP || issue.Severity != SeverityWarning {
		t.Errorf("rule/severity = %s/%s, want SOLID-S/warning", issue.Rule, issue.Severity)
	}
	if !strings.HasPrefix(issue.Evidence, "large-type:type=Widget;methods=11;exported_methods=11;fields=11;") || !strings.HasSuffix(issue.Evidence, ";signals=4") {
		t.Errorf("unexpected evidence: %s", issue.Evidence)
	}
}

func TestCheckSRPSyntaxLargeTypeUsesMultiSignalRule(t *testing.T) {
	source := "package p\n\ntype Widget struct{}\n"
	for index := 0; index < 11; index++ {
		source += fmt.Sprintf("func (w *Widget) Method%c() {}\n", 'A'+index)
	}
	fset, files := parseSource(t, source)
	for _, issue := range CheckSRP(fset, files, DefaultConfig()) {
		if issue.Check == CheckSRPLargeType {
			t.Fatalf("method count alone produced a large-type finding: %v", issue)
		}
	}
}

func TestSyntaxLargeTypeFingerprintMatchesTyped(t *testing.T) {
	root := t.TempDir()
	writeModuleFixture(t, root, map[string]string{"widget/widget.go": largeTypeFixture("widget")})
	byMode := map[string]Issue{}
	for _, mode := range []string{"syntax", "types"} {
		pkgs, _, err := LoadWorkspace([]string{root}, false, mode)
		if err != nil {
			t.Fatal(err)
		}
		cfg := DefaultConfig()
		cfg.CacheEnabled = false
		cfg.AnalysisMode = mode
		plan, err := NewExecutionPlan(cfg, map[Rule]bool{RuleSRP: true}, SurfaceCLI)
		if err != nil {
			t.Fatal(err)
		}
		issues, _ := RunPlan(pkgs, cfg, plan)
		var large []Issue
		for _, issue := range issues {
			if issue.Check == CheckSRPLargeType {
				large = append(large, issue)
			}
		}
		if len(large) != 1 {
			t.Fatalf("%s mode large-type findings = %v, want one", mode, large)
		}
		byMode[mode] = large[0]
	}
	syntax, typed := byMode["syntax"], byMode["types"]
	if syntax.Identity != typed.Identity || syntax.Subject != typed.Subject || syntax.Fingerprint() != typed.Fingerprint() {
		t.Fatalf("syntax and typed large-type differ:\nsyntax: %s %s %s\ntyped:  %s %s %s", syntax.Subject, syntax.Identity, syntax.Fingerprint(), typed.Subject, typed.Identity, typed.Fingerprint())
	}
	if syntax.Identity != "large-type;type=Widget" || syntax.Severity != typed.Severity || syntax.Evidence != typed.Evidence || syntax.Pos != typed.Pos {
		t.Fatalf("syntax = %+v\ntyped = %+v", syntax, typed)
	}
}

func TestCheckSRP_MixedParameterTypes(t *testing.T) {
	src := `package p

func Coordinate(ctx Context, request Request, store Store, logger Logger, retries int, notify func()) {}
`
	fset, files := parseSource(t, src)
	cfg := DefaultConfig()
	cfg.MaxFuncParams = 5

	issues := CheckSRP(fset, files, cfg)
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1", len(issues))
	}
	if !strings.Contains(issues[0].Message, `function "Coordinate" takes 6 parameters spanning 6 distinct types`) {
		t.Errorf("unexpected message: %s", issues[0].Message)
	}
	if !strings.Contains(issues[0].Evidence, "mixed-parameters") {
		t.Errorf("unexpected evidence: %s", issues[0].Evidence)
	}
}

func TestCheckSRP_HomogeneousParametersCanBeCohesive(t *testing.T) {
	fset, files := parseSource(t, `package p

func Join(a, b, c, d, e, f int) int { return a + b + c + d + e + f }
`)
	cfg := DefaultConfig()
	cfg.MaxFuncParams = 5

	if issues := CheckSRP(fset, files, cfg); len(issues) != 0 {
		t.Fatalf("got %d issues for a cohesive parameter list, want 0: %v", len(issues), issues)
	}
}

func TestCheckSRP_RepeatedParametersRevealDataClump(t *testing.T) {
	fset, files := parseSource(t, `package p

func Register(name, email, phone, address, city, country string) {}
func UpdateContact(name, email, phone, address, city, country string) {}
`)
	cfg := DefaultConfig()
	cfg.MaxFuncParams = 5

	issues := CheckSRP(fset, files, cfg)
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1", len(issues))
	}
	if !strings.Contains(issues[0].Message, `function "UpdateContact" repeats 6 parameters also used by "Register"`) {
		t.Errorf("unexpected message: %s", issues[0].Message)
	}
	if !strings.Contains(issues[0].Evidence, "data-clump") {
		t.Errorf("unexpected evidence: %s", issues[0].Evidence)
	}
}

func TestCheckSRP_DataClumpIsReportedOnceForThreeFunctions(t *testing.T) {
	fset, files := parseSource(t, `package p

func First(a, b, c, d, e, f, g, h, i string) {}
func Second(a, b, c, d, e, f, g, h, i string) {}
func Third(a, b, c, d, e, f, g, h, i string) {}
`)
	issues := CheckSRP(fset, files, DefaultConfig())
	var clumps []Issue
	for _, issue := range issues {
		if issue.Check == CheckSRPDataClump {
			clumps = append(clumps, issue)
		}
	}
	if len(clumps) != 1 || len(clumps[0].Groups) != 1 || len(clumps[0].Groups[0].Symbols) != 3 {
		t.Fatalf("expected one maximal clump with three functions, got %v", clumps)
	}
}

func TestCheckSRP_DataClumpSkipsFunctionsAtParameterLimit(t *testing.T) {
	fset, files := parseSource(t, `package p

func Short(name, email string) {}
func First(name, email, phone string) {}
func Between(name, email string) {}
func Second(name, email, phone string) {}
`)
	cfg := DefaultConfig()
	cfg.MaxFuncParams = 2

	var clumps []Issue
	for _, issue := range CheckSRP(fset, files, cfg) {
		if issue.Check == CheckSRPDataClump {
			clumps = append(clumps, issue)
		}
	}
	if len(clumps) != 1 || len(clumps[0].Groups) != 1 || strings.Join(clumps[0].Groups[0].Symbols, ",") != "First,Second" {
		t.Fatalf("expected one clump of First and Second, got %v", clumps)
	}
}

func TestCheckSRP_BooleanFlagSelectsBehavior(t *testing.T) {
	fset, files := parseSource(t, `package p

func Render(document string, compact bool) string {
	if compact && document != "" {
		return "compact"
	} else {
		return document
	}
}
`)

	issues := CheckSRP(fset, files, DefaultConfig())
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1", len(issues))
	}
	if !strings.Contains(issues[0].Message, `uses boolean parameter(s) "compact" to select between behaviors`) {
		t.Errorf("unexpected message: %s", issues[0].Message)
	}
	if !strings.Contains(issues[0].Evidence, "flag-argument") {
		t.Errorf("unexpected evidence: %s", issues[0].Evidence)
	}
}

func TestCheckSRP_BooleanValueWithoutAlternateBehaviorIsNotFlagged(t *testing.T) {
	fset, files := parseSource(t, `package p

func Persist(enabled bool) { record(enabled) }
func Warm(enabled bool) { if enabled { record(true) } }
`)

	if issues := CheckSRP(fset, files, DefaultConfig()); len(issues) != 0 {
		t.Fatalf("got %d issues for non-selecting boolean values, want 0: %v", len(issues), issues)
	}
}

func TestCheckSRP_TooManyLines(t *testing.T) {
	src := "package p\n\nfunc Long() {\n"
	for i := 0; i < 8; i++ {
		src += "\t_ = 1\n"
	}
	src += "}\n"

	fset, files := parseSource(t, src)
	cfg := DefaultConfig()
	cfg.MaxFuncLines = 5

	issues := CheckSRP(fset, files, cfg)
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1", len(issues))
	}
	if !strings.Contains(issues[0].Message, `function "Long" is`) {
		t.Errorf("unexpected message: %s", issues[0].Message)
	}
}
