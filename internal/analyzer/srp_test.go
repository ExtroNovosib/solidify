package analyzer

import (
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

func TestCheckSRP_TooManyMethods(t *testing.T) {
	src := `package p

type Widget struct{}
`
	for i := 0; i < 4; i++ {
		src += "func (w *Widget) M" + string(rune('A'+i)) + "() {}\n"
	}

	fset, files := parseSource(t, src)
	cfg := DefaultConfig()
	cfg.MaxMethodsPerType = 3

	issues := CheckSRP(fset, files, cfg)
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1", len(issues))
	}
	if issues[0].Rule != RuleSRP {
		t.Errorf("rule = %q, want SOLID-S", issues[0].Rule)
	}
	if !strings.Contains(issues[0].Message, `type "Widget" has 4 methods`) {
		t.Errorf("unexpected message: %s", issues[0].Message)
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
