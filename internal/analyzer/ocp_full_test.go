package analyzer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckOCPProgramFullSignals(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/ocptest\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := `package p

import "fmt"

type Shape interface { Area() float64; Perimeter() float64 }
type Circle struct{ R float64 }
func (Circle) Area() float64 { return 1 }
func (Circle) Perimeter() float64 { return 2 }
type Square struct{ S float64 }
func (Square) Area() float64 { return 1 }
func (Square) Perimeter() float64 { return 2 }
type Triangle struct{ B float64 }
func (Triangle) Area() float64 { return 1 }
func (Triangle) Perimeter() float64 { return 2 }

func Render(s Shape) float64 {
	switch v := s.(type) {
	case Circle: return v.Area()
	case Square: return v.Area()
	case Triangle: return v.Area()
	case *Circle: return v.Area()
	case *Square: return v.Area()
	default: panic(fmt.Sprintf("unknown type %T", s))
	}
}

func Export(s Shape) float64 {
	switch v := s.(type) {
	case Circle: return v.Area()
	case Square: return v.Area()
	case Triangle: return v.Area()
	case *Circle: return v.Area()
	case *Square: return v.Area()
	default: panic(fmt.Sprintf("unknown type %T", s))
	}
}

type Kind int
const ( KindA Kind = iota; KindB; KindC )
type Item struct{ Kind Kind }
func Validate(i Item) bool { switch i.Kind { case KindA: return true; case KindB: return false; case KindC: return true }; return false }
func ExportItem(i Item) bool { switch i.Kind { case KindA: return true; case KindB: return false; case KindC: return true }; return false }

func ProcessOrder(o *Order) float64 { return o.Validate() + o.Total() }
type Order struct{}
func (*Order) Validate() float64 { return 1 }
func (*Order) Total() float64 { return 2 }

func MakeShape(kind string) Shape {
	switch kind { case "circle": return Circle{}; case "square": return Square{}; case "triangle": return Triangle{}; case "circle2": return Circle{}; case "square2": return Square{} }
	return nil
}
func MakeShapeFromRegistry(kind string) Shape {
	table := map[string]func() Shape{"a": func() Shape { return Circle{} }, "b": func() Shape { return Square{} }, "c": func() Shape { return Triangle{} }, "d": func() Shape { return Circle{} }, "e": func() Shape { return Square{} }}
	return table[kind]()
}

func ProcessCircle(v *Circle) float64 { total := v.Area(); if total > 0 { total += v.Perimeter() }; return total }
func ProcessSquare(v *Square) float64 { total := v.Area(); if total > 0 { total += v.Perimeter() }; return total }
func ProcessTriangle(v *Triangle) float64 { total := v.Area(); if total > 0 { total += v.Perimeter() }; return total }
`
	if err := os.WriteFile(filepath.Join(dir, "p.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgs, warnings, err := LoadWorkspace([]string{dir}, false, "types")
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected load warnings: %v", warnings)
	}
	issues := CheckOCPProgram(pkgs, DefaultConfig())
	seen := map[CheckID]bool{}
	for _, issue := range issues {
		seen[issue.Check] = true
	}
	for _, check := range []CheckID{CheckOCPTypeDispatch, CheckOCPDiscriminatorDispatch, CheckOCPRuntimeExhaustiveness, CheckOCPConcreteParameter, CheckOCPClosedFactory, CheckOCPParallelImplementations} {
		if !seen[check] {
			t.Errorf("missing OCP check %s in %v", check, issues)
		}
	}
}

func TestLoadWorkspaceSyntaxAndTypedModes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/modes\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "p.go"), []byte("package p\nfunc F() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if pkgs, _, err := LoadWorkspace([]string{dir}, false, "syntax"); err != nil || len(pkgs) != 1 || pkgs[0].typeComplete {
		t.Fatalf("syntax load = pkgs=%d complete=%v err=%v", len(pkgs), len(pkgs) == 1 && pkgs[0].typeComplete, err)
	}
	if pkgs, _, err := LoadWorkspace([]string{dir}, false, "types"); err != nil || len(pkgs) != 1 || !pkgs[0].typeComplete {
		t.Fatalf("typed load = pkgs=%d complete=%v err=%v", len(pkgs), len(pkgs) == 1 && pkgs[0].typeComplete, err)
	}
}

func TestCheckOCPImplementationCoupling(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/arch\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, provider := range []string{"stripe", "paypal"} {
		providerDir := filepath.Join(dir, "providers", provider)
		if err := os.MkdirAll(providerDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(providerDir, "provider.go"), []byte("package "+provider+"\ntype Client struct{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	serviceDir := filepath.Join(dir, "service")
	if err := os.MkdirAll(serviceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	service := `package service
import (
  "example.com/arch/providers/paypal"
  "example.com/arch/providers/stripe"
)
type Service struct { stripe stripe.Client; paypal paypal.Client }
`
	if err := os.WriteFile(filepath.Join(serviceDir, "service.go"), []byte(service), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgs, _, err := LoadWorkspace([]string{dir}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.OCPLogicPackages = []string{"example.com/arch/service"}
	cfg.OCPImplementationPackages = []string{"example.com/arch/providers/**"}
	issues := CheckOCPProgram(pkgs, cfg)
	for _, issue := range issues {
		if issue.Check == CheckOCPImplementationCoupling {
			return
		}
	}
	t.Fatalf("implementation coupling was not reported: %v", issues)
}
