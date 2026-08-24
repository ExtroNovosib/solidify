package analyzer

import (
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestRun_CacheInvalidatesWhenSourceChanges(t *testing.T) {
	dir := t.TempDir()
	initTempModule(t, dir)
	otherDir := filepath.Join(dir, "other")
	if err := os.MkdirAll(otherDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherDir, "driver.go"), []byte("package other\n\ntype Driver struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	serviceDir := filepath.Join(dir, "service")
	if err := os.MkdirAll(serviceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(serviceDir, "service.go")
	if err := os.WriteFile(path, []byte(`package service

import "tempmod/other"

type Service struct { driver *other.Driver }
`), 0o644); err != nil {
		t.Fatal(err)
	}

	enabled := map[Rule]bool{RuleDIP: true}
	pkgs, _, err := LoadWorkspace([]string{serviceDir}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	if issues := Run(pkgs, DefaultConfig(), enabled); len(issues) != 1 {
		t.Fatalf("initial source = %v, want one DIP finding", issues)
	}

	if writeErr := os.WriteFile(path, []byte(`package service

type Driver interface{ Drive() }
type Service struct { driver Driver }
`), 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}
	pkgs, _, err = LoadWorkspace([]string{serviceDir}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	if issues := Run(pkgs, DefaultConfig(), enabled); len(issues) != 0 {
		t.Fatalf("changed source must not reuse stale DIP findings: %v", issues)
	}
}

func TestRun_CacheInvalidatesWhenImportedTypeChanges(t *testing.T) {
	dir := t.TempDir()
	initTempModule(t, dir)
	depDir := filepath.Join(dir, "dep")
	consumerDir := filepath.Join(dir, "consumer")
	for _, path := range []string{depDir, consumerDir} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	depPath := filepath.Join(depDir, "dep.go")
	if err := os.WriteFile(depPath, []byte("package dep\n\ntype Dependency struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(consumerDir, "consumer.go"), []byte(`package consumer

import "tempmod/dep"

type Service struct { dependency *dep.Dependency }
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	cfg.CacheDir = filepath.Join(dir, "cache")
	enabled := map[Rule]bool{RuleDIP: true}
	pkgs, _, err := LoadWorkspace([]string{consumerDir}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	if issues := Run(pkgs, cfg, enabled); len(issues) != 1 {
		t.Fatalf("concrete imported dependency = %v, want one finding", issues)
	}

	if writeErr := os.WriteFile(depPath, []byte("package dep\n\ntype Dependency interface { Run() }\n"), 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}
	pkgs, _, err = LoadWorkspace([]string{consumerDir}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	if issues := Run(pkgs, cfg, enabled); len(issues) != 0 {
		t.Fatalf("dependency-only change reused stale cache: %v", issues)
	}
}

func TestFilterExcludedFilesRebuildsTypedSnapshot(t *testing.T) {
	dir := t.TempDir()
	initTempModule(t, dir)
	pkgDir := filepath.Join(dir, "service")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "included.go"), []byte(`package service

type Service struct{}
func (*Service) Included() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "excluded.go"), []byte(`package service

func (*Service) Excluded() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgs, _, err := LoadWorkspace([]string{pkgDir}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	pkgs = FilterExcludedFiles(pkgs, []string{"service/excluded.go"})
	if len(pkgs) != 1 || !pkgs[0].typeComplete {
		t.Fatalf("filtered package is not type complete: %+v", pkgs)
	}
	object := pkgs[0].typePkg.Scope().Lookup("Service")
	typeName, ok := object.(*types.TypeName)
	if !ok {
		t.Fatalf("Service type missing from filtered snapshot: %v", object)
	}
	methods := types.NewMethodSet(types.NewPointer(typeName.Type()))
	names := make([]string, 0, methods.Len())
	for i := 0; i < methods.Len(); i++ {
		names = append(names, methods.At(i).Obj().Name())
	}
	if strings.Join(names, ",") != "Included" {
		t.Fatalf("filtered method set = %v, want only Included", names)
	}
	profiles := buildSRPTypeProfiles(pkgs[0].fset, pkgs[0].files, pkgs[0].info, pkgs[0].typePkg, pkgs[0])
	if len(profiles) != 1 || len(profiles[0].methods) != 1 || profiles[0].methods[0].Name.Name != "Included" {
		t.Fatalf("filtered SRP profile retained excluded input: %+v", profiles)
	}
}

func TestRun_ExcludedFilesOmittedFromRelatedLocations(t *testing.T) {
	dir := t.TempDir()
	initTempModule(t, dir)
	pkgDir := filepath.Join(dir, "service")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	included := `package service

type Service struct{}
func (s *Service) A() {}
func (s *Service) B() {}
func (s *Service) C() {}
func (s *Service) D() {}
func (s *Service) E() {}
func (s *Service) F() {}
func (s *Service) G() {}
func (s *Service) H() {}
func (s *Service) I() {}
func (s *Service) J() {}
`
	excluded := `package service

func (s *Service) K() {}
func (s *Service) L() {}
func (s *Service) M() {}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "included.go"), []byte(included), 0o644); err != nil {
		t.Fatal(err)
	}
	excludedPath := filepath.Join(pkgDir, "excluded.go")
	if err := os.WriteFile(excludedPath, []byte(excluded), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	cfg.ExcludedFiles = []string{"service/excluded.go"}
	pkgs, _, err := LoadWorkspace([]string{pkgDir}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	pkgs = FilterExcludedFiles(pkgs, cfg.ExcludedFiles)
	issues := Run(pkgs, cfg, map[Rule]bool{RuleSRP: true})
	for _, issue := range issues {
		for _, related := range issue.Related {
			if strings.HasSuffix(filepath.ToSlash(related.Pos.Filename), "service/excluded.go") {
				t.Fatalf("excluded file appeared in related locations: %+v", related)
			}
		}
	}
}

func TestLoad_SkipsTestFilesByDefault(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a_test.go"), []byte("package p\nfunc TestX(t *testing.T) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pkgs := loadWorkspaceDir(t, dir, false, "syntax")
	if len(pkgs) != 1 {
		t.Fatalf("got %d packages, want 1", len(pkgs))
	}
	if len(pkgs[0].files) != 1 {
		t.Fatalf("got %d files, want 1 (test file skipped)", len(pkgs[0].files))
	}
}

func TestLoad_IncludesTestFilesWhenRequested(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a_test.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pkgs := loadWorkspaceDir(t, dir, true, "syntax")
	if len(pkgs[0].files) != 2 {
		t.Fatalf("got %d files, want 2", len(pkgs[0].files))
	}
}

func TestLoad_RespectsBuildTagsAndPackageVariants(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"normal.go": "package p\n", "generated.go": "// Code generated; DO NOT EDIT.\npackage p\n",
		"tagged.go": "//go:build never\n\npackage p\n", "external_test.go": "package p_test\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pkgs := loadWorkspaceDir(t, dir, true, "syntax")
	if len(pkgs) != 2 {
		t.Fatalf("package variants = %d, want 2", len(pkgs))
	}
	var regular int
	for _, pkg := range pkgs {
		if len(pkg.files) > regular {
			regular = len(pkg.files)
		}
	}
	if regular != 2 {
		t.Fatalf("regular files = %d, want generated + normal", regular)
	}
}

func TestLoadWithTypes_PartialResolutionFallsBackToSyntax(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "broken.go"), []byte("package p\nimport _ \"example.invalid/missing\"\ntype S struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgs := loadWorkspaceDir(t, dir, false, "auto")
	if len(pkgs) != 1 || pkgs[0].info == nil {
		t.Fatalf("partial resolution should retain syntax package: pkgs=%d", len(pkgs))
	}
}

func TestRun_ViolationsTestdata(t *testing.T) {
	root := testdataDir(t, "violations")
	pkgs, _, err := LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}

	issues := Run(pkgs, DefaultConfig(), allRulesEnabled())
	if len(issues) != 10 {
		t.Fatalf("got %d issues, want 10", len(issues))
	}

	byRule := issuesByRule(issues)
	want := map[Rule]int{
		RuleSRP: 2, // large type + repeated parameter data clump
		RuleOCP: 2, // type switch + type-assertion chain
		RuleLSP: 0, // stub detection moved to ISP
		RuleISP: 5, // fat interfaces + stub implementations + usage ratio
		RuleDIP: 1, // constructor dependency
	}
	for rule, count := range want {
		if byRule[rule] != count {
			t.Errorf("rule %s: got %d issues, want %d", rule, byRule[rule], count)
		}
	}
}

func TestRun_CleanTestdata(t *testing.T) {
	root := testdataDir(t, "clean")
	pkgs, _, err := LoadWorkspace([]string{root}, false, "syntax")
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}

	issues := Run(pkgs, DefaultConfig(), allRulesEnabled())
	if len(issues) != 0 {
		t.Fatalf("got %d issues, want 0: %v", len(issues), issues)
	}
}

func TestRun_RespectsEnabledRules(t *testing.T) {
	root := testdataDir(t, "violations")
	pkgs, _, err := LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}

	enabled := map[Rule]bool{RuleDIP: true}
	issues := Run(pkgs, DefaultConfig(), enabled)

	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1", len(issues))
	}
	if issues[0].Rule != RuleDIP {
		t.Errorf("rule = %q, want SOLID-D", issues[0].Rule)
	}
}

func TestWorkspaceFilePolicyRebuildsTypedSnapshotOnce(t *testing.T) {
	root := t.TempDir()
	initTempModule(t, root)
	writePolicyFixture(t, filepath.Join(root, "included.go"), "package tempmod\ntype Included struct{}\n")
	writePolicyFixture(t, filepath.Join(root, "excluded.go"), "package tempmod\ntype Excluded struct{}\n")
	writePolicyFixture(t, filepath.Join(root, "generated.go"), "// Code generated by test. DO NOT EDIT.\npackage tempmod\ntype Generated struct{}\n")
	pkgs, _, err := LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.ExcludedFiles = []string{"excluded.go"}
	Run(pkgs, cfg, allRulesEnabled())
	if len(pkgs) != 1 || pkgs[0].filteredRebuilds != 1 {
		t.Fatalf("filtered rebuilds = %d, want one combined rebuild", pkgs[0].filteredRebuilds)
	}
	Run(pkgs, cfg, allRulesEnabled())
	if pkgs[0].filteredRebuilds != 1 {
		t.Fatalf("unchanged policy rebuilt again: %d", pkgs[0].filteredRebuilds)
	}
}

func writePolicyFixture(t *testing.T, path, source string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRun_SortedByPosition(t *testing.T) {
	root := testdataDir(t, "violations")
	pkgs, _, err := LoadWorkspace([]string{root}, false, "syntax")
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}

	issues := Run(pkgs, DefaultConfig(), allRulesEnabled())
	for i := 1; i < len(issues); i++ {
		prev, cur := issues[i-1].Pos, issues[i].Pos
		if prev.Filename > cur.Filename ||
			(prev.Filename == cur.Filename && prev.Line > cur.Line) ||
			(prev.Filename == cur.Filename && prev.Line == cur.Line && prev.Column > cur.Column) {
			t.Errorf("issues not sorted at index %d: %+v before %+v", i, prev, cur)
		}
	}
}

func TestRun_ColdWarmCacheEquivalencePrecisionCorpus(t *testing.T) {
	for _, corpus := range []string{"violations", "clean"} {
		t.Run(corpus, func(t *testing.T) {
			root := testdataDir(t, corpus)
			cfg := DefaultConfig()
			cfg.CacheDir = filepath.Join(t.TempDir(), corpus)
			enabled := allRulesEnabled()

			pkgs, _, err := LoadWorkspace([]string{root}, false, "types")
			if err != nil {
				t.Fatal(err)
			}
			cold := Run(pkgs, cfg, enabled)

			pkgs, _, err = LoadWorkspace([]string{root}, false, "types")
			if err != nil {
				t.Fatal(err)
			}
			warm := Run(pkgs, cfg, enabled)

			disabledConfig := cfg
			disabledConfig.CacheEnabled = false
			pkgs, _, err = LoadWorkspace([]string{root}, false, "types")
			if err != nil {
				t.Fatal(err)
			}
			disabled := Run(pkgs, disabledConfig, enabled)

			if issueSignatures(cold) != issueSignatures(warm) {
				t.Fatalf("cold/warm mismatch\ncold=%v\nwarm=%v", cold, warm)
			}
			if issueSignatures(cold) != issueSignatures(disabled) {
				t.Fatalf("cold/cache-disabled mismatch\ncold=%v\ndisabled=%v", cold, disabled)
			}
		})
	}
}

func TestRun_LegacyBaselineMatchingIdenticalAfterCacheHit(t *testing.T) {
	root := testdataDir(t, "violations")
	cfg := DefaultConfig()
	cfg.CacheDir = filepath.Join(t.TempDir(), "cache")
	enabled := allRulesEnabled()

	pkgs, _, err := LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	cold := Run(pkgs, cfg, enabled)

	pkgs, _, err = LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	warm := Run(pkgs, cfg, enabled)

	if len(cold) != len(warm) {
		t.Fatalf("cold=%d warm=%d findings", len(cold), len(warm))
	}

	coldByFingerprint := map[string]Issue{}
	for _, issue := range cold {
		coldByFingerprint[issue.Fingerprint()] = issue
	}
	for _, warmIssue := range warm {
		if _, ok := coldByFingerprint[warmIssue.Fingerprint()]; !ok {
			t.Fatalf("warm finding missing from cold run: %v", warmIssue)
		}
	}
}

func issueSignatures(issues []Issue) string {
	signatures := make([]string, 0, len(issues))
	for _, issue := range issues {
		signatures = append(signatures, fmt.Sprintf(
			"%s|%s|%s|%s|%d|%d|%s",
			issue.ID(), issue.Fingerprint(), issue.Evidence, issue.PortablePath(),
			issue.Pos.Line, issue.Pos.Column, issue.Message,
		))
	}
	sort.Strings(signatures)
	return strings.Join(signatures, "\n")
}
