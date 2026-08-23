package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSRPLargeTypeRequiresFourSignalsOrExtremeSurface(t *testing.T) {
	cfg := DefaultConfig()
	fset := token.NewFileSet()
	methods := make([]*ast.FuncDecl, 18)
	for i := range methods {
		methods[i] = &ast.FuncDecl{Name: ast.NewIdent("Exported")}
	}
	profile := &srpTypeProfile{
		name:    "WideButInconclusive",
		fields:  make([]string, 9),
		methods: methods,
		lines:   100,
		metrics: srpMethodMetrics{wmc: cfg.MaxTypeComplexity},
	}
	if issue := srpProfileLargeTypeIssue(profile, fset, cfg, true); issue != nil {
		t.Fatalf("three size signals should be inconclusive, got %+v", issue)
	}

	profile.lines = cfg.MaxTypeLines + 1
	if issue := srpProfileLargeTypeIssue(profile, fset, cfg, true); issue == nil {
		t.Fatal("four independent size signals should report large-type")
	}

	profile.lines = 100
	profile.metrics.wmc = 29
	profile.fields = make([]string, 13)
	profile.methods = make([]*ast.FuncDecl, 29)
	for i := range profile.methods {
		profile.methods[i] = &ast.FuncDecl{Name: ast.NewIdent("Exported")}
	}
	if issue := srpProfileLargeTypeIssue(profile, fset, cfg, true); issue == nil {
		t.Fatal("an extreme method and field surface should report even with three signals")
	}
}

func TestSRPStrictGodTypeUsesPackageMetrics(t *testing.T) {
	dir := t.TempDir()
	source := `package p
import "database/sql"
type Service struct { db *sql.DB; left int; right int }
func (s *Service) LeftOne() { _ = s.left; _ = s.db.Stats() }
func (s *Service) LeftTwo() { _ = s.left; _ = s.db.Stats() }
func (s *Service) RightOne() { _ = s.right }
func (s *Service) RightTwo() { _ = s.right }
`
	if err := os.WriteFile(filepath.Join(dir, "service.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgs := loadWorkspaceDir(t, dir, false, "auto")
	cfg := DefaultConfig()
	cfg.MaxTypeComplexity = 4
	cfg.MinTCCPercent = 60
	cfg.MaxFanOut = 0
	issues := Run(pkgs, cfg, map[Rule]bool{RuleSRP: true})
	var god *Issue
	for i := range issues {
		if issues[i].Check == CheckSRPGodType {
			god = &issues[i]
		}
	}
	if god == nil {
		t.Fatalf("expected god-type finding, got %v", issues)
		return
	}
	if len(god.Metrics) < 4 || len(god.Groups) < 2 {
		t.Fatalf("god-type evidence is incomplete: %+v", god)
	}
}

func TestSRPLowCohesionSkipsSerializedDataCarrier(t *testing.T) {
	dir := t.TempDir()
	source := `package p
type Config struct {
    Server int ` + "`json:\"server\"`" + `
    Auth int ` + "`json:\"auth\"`" + `
    Tunnel int ` + "`json:\"tunnel\"`" + `
}
func (c *Config) ServerOne() int { return c.Server }
func (c *Config) ServerTwo() int { return c.Server }
func (c *Config) AuthOne() int { return c.Auth }
func (c *Config) AuthTwo() int { return c.Auth }
func (c *Config) TunnelOne() int { return c.Tunnel }
func (c *Config) TunnelTwo() int { return c.Tunnel }
func (c *Config) Save() { persist(c) }
func persist(any) {}
`
	if err := os.WriteFile(filepath.Join(dir, "config.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgs := loadWorkspaceDir(t, dir, false, "auto")
	for _, issue := range Run(pkgs, DefaultConfig(), map[Rule]bool{RuleSRP: true}) {
		if issue.Check == CheckSRPLowCohesionType {
			t.Fatalf("serialized data carrier should not report low cohesion: %+v", issue)
		}
	}
}

func TestSRPLowCohesionKeepsBehavioralSerializedType(t *testing.T) {
	dir := t.TempDir()
	source := `package p
type Workflow struct {
    Create int ` + "`json:\"create\"`" + `
    Update int ` + "`json:\"update\"`" + `
    Delete int ` + "`json:\"delete\"`" + `
}
func (w *Workflow) CreateOne() { work(w.Create) }
func (w *Workflow) CreateTwo() { work(w.Create) }
func (w *Workflow) UpdateOne() { work(w.Update) }
func (w *Workflow) UpdateTwo() { work(w.Update) }
func (w *Workflow) DeleteOne() { work(w.Delete) }
func (w *Workflow) DeleteTwo() { work(w.Delete) }
func work(int) {}
`
	if err := os.WriteFile(filepath.Join(dir, "workflow.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgs := loadWorkspaceDir(t, dir, false, "auto")
	issues := Run(pkgs, DefaultConfig(), map[Rule]bool{RuleSRP: true})
	if !hasCheck(issues, CheckSRPLowCohesionType) {
		t.Fatalf("behavioral serialized type should retain low-cohesion finding: %v", issues)
	}
}

func TestSRPLowCohesionRequiresTwoMultiMethodComponents(t *testing.T) {
	dir := t.TempDir()
	source := `package p
type Service struct { left int; right int }
func (s *Service) LeftOne() { _ = s.left }
func (s *Service) LeftTwo() { _ = s.left }
func (s *Service) RightOne() { _ = s.right }
func (s *Service) RightTwo() { _ = s.right }
`
	if err := os.WriteFile(filepath.Join(dir, "service.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgs := loadWorkspaceDir(t, dir, false, "auto")
	cfg := DefaultConfig()
	cfg.MaxTypeComplexity = 999
	issues := Run(pkgs, cfg, map[Rule]bool{RuleSRP: true})
	if len(issues) != 1 || issues[0].Check != CheckSRPLowCohesionType {
		t.Fatalf("expected one low-cohesion finding, got %v", issues)
	}
}

func TestSRPLowCohesionSkipsHandlerSuffix(t *testing.T) {
	dir := t.TempDir()
	source := `package p
type CreateTunnelHandler struct { left int; right int }
func (h *CreateTunnelHandler) LeftOne() { _ = h.left }
func (h *CreateTunnelHandler) LeftTwo() { _ = h.left }
func (h *CreateTunnelHandler) RightOne() { _ = h.right }
func (h *CreateTunnelHandler) RightTwo() { _ = h.right }
`
	if err := os.WriteFile(filepath.Join(dir, "handler.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgs := loadWorkspaceDir(t, dir, false, "auto")
	cfg := DefaultConfig()
	cfg.MaxTypeComplexity = 999
	issues := Run(pkgs, cfg, map[Rule]bool{RuleSRP: true})
	if hasCheck(issues, CheckSRPLowCohesionType) {
		t.Fatalf("Handler suffix should skip low-cohesion: %v", issues)
	}
}

func TestSRPStrictChecksSkipIncompleteTypeInformation(t *testing.T) {
	dir := t.TempDir()
	source := `package p
import "example.invalid/missing"
type Service struct { left int; right int }
func (s *Service) LeftOne() { _ = s.left }
func (s *Service) LeftTwo() { _ = s.left }
func (s *Service) RightOne() { _ = s.right }
func (s *Service) RightTwo() { _ = s.right }
var _ = missing.Value
`
	if err := os.WriteFile(filepath.Join(dir, "service.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgs := loadWorkspaceDir(t, dir, false, "auto")
	cfg := DefaultConfig()
	cfg.MaxTypeComplexity = 1
	for _, issue := range Run(pkgs, cfg, map[Rule]bool{RuleSRP: true}) {
		if issue.Check == CheckSRPGodType || issue.Check == CheckSRPLowCohesionType {
			t.Fatalf("strict finding emitted with incomplete types: %+v", issue)
		}
	}
}

func TestSRPTypedParameterRulesRespectContextAndShadowing(t *testing.T) {
	dir := t.TempDir()
	source := `package p
import "context"
type Switch bool
func Cohesive(ctx context.Context, a int, b string, c float64, d byte, e rune) {}
func Render(mode Switch) string { if mode { return "a" } else { return "b" } }
func Shadow(mode bool) string {
    if true { mode := false; if mode { return "inner-a" } else { return "inner-b" } }
    return "outer"
}
`
	if err := os.WriteFile(filepath.Join(dir, "params.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgs := loadWorkspaceDir(t, dir, false, "auto")
	issues := Run(pkgs, DefaultConfig(), map[Rule]bool{RuleSRP: true})
	var flagFound bool
	for _, issue := range issues {
		if issue.Check == CheckSRPFlagArgument {
			if issue.Evidence == "flag-argument:function=Render;parameters=mode" {
				flagFound = true
			}
			if issue.Evidence == "flag-argument:function=Shadow;parameters=mode" {
				t.Fatalf("shadowed bool was treated as a flag argument: %+v", issue)
			}
		}
		if issue.Check == CheckSRPMixedInputSurface && issue.Evidence == "mixed-parameters:function=Cohesive;count=5;types=5;max=5" {
			t.Fatalf("context.Context was not ignored: %+v", issue)
		}
	}
	if !flagFound {
		t.Fatalf("bool alias flag argument was not reported: %v", issues)
	}
}

func TestComputeSRPProfileCohesionCountsMethodPairOnce(t *testing.T) {
	profile := &srpTypeProfile{methods: []*ast.FuncDecl{{Name: ast.NewIdent("A")}, {Name: ast.NewIdent("B")}}}
	profile.metrics.fieldUsers = map[string]map[int]bool{
		"first":  {0: true, 1: true},
		"second": {0: true, 1: true},
	}
	profile.metrics.callEdges = map[[2]int]bool{}
	computeSRPProfileCohesion(profile)
	if profile.metrics.tcc != 100 {
		t.Fatalf("TCC = %v, want 100", profile.metrics.tcc)
	}
}

func TestSRPProfileTypeLinesSumsIndependentFileIntervals(t *testing.T) {
	fset := token.NewFileSet()
	typeFile, err := parser.ParseFile(fset, "type.go", "package p\ntype Service struct{}\n", 0)
	if err != nil {
		t.Fatal(err)
	}
	methodFile, err := parser.ParseFile(fset, "method.go", "package p\n"+strings.Repeat("\n", 100)+"func (*Service) Run() {}\n", 0)
	if err != nil {
		t.Fatal(err)
	}
	typeSpec := typeFile.Decls[0].(*ast.GenDecl).Specs[0].(*ast.TypeSpec)
	method := methodFile.Decls[0].(*ast.FuncDecl)
	profile := &srpTypeProfile{pos: typeSpec.Pos(), end: typeSpec.End(), methods: []*ast.FuncDecl{method}}
	if got := srpProfileTypeLines(profile, fset); got != 2 {
		t.Fatalf("type LOC = %d, want 2 independent source lines", got)
	}
}
