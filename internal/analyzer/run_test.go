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

func TestRun_CacheInvalidatesWhenImportedMethodSetChanges(t *testing.T) {
	dir := t.TempDir()
	initTempModule(t, dir)
	depDir := filepath.Join(dir, "domain")
	consumerDir := filepath.Join(dir, "consumer")
	for _, path := range []string{depDir, consumerDir} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	depPath := filepath.Join(depDir, "dep.go")
	if err := os.WriteFile(depPath, []byte("package domain\n\ntype Dependency struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(consumerDir, "consumer.go"), []byte(`package consumer

import "tempmod/domain"

type Service struct { dependency *domain.Dependency }
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
	if issues := Run(pkgs, cfg, enabled); len(issues) != 0 {
		t.Fatalf("method-free imported value object = %v, want no findings", issues)
	}

	if writeErr := os.WriteFile(depPath, []byte("package domain\n\ntype Dependency struct{}\n\nfunc (*Dependency) Run() {}\n"), 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}
	pkgs, _, err = LoadWorkspace([]string{consumerDir}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	if issues := Run(pkgs, cfg, enabled); len(issues) != 1 {
		t.Fatalf("dependency-only method change reused stale cache: %v", issues)
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

func writeModuleFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()
	initTempModule(t, root)
	for name, source := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		writePolicyFixture(t, path, source)
	}
}

func runAllChecks(t *testing.T, root string, cfg Config) ([]Issue, ExecutionStats, []*packageFiles) {
	t.Helper()
	pkgs, _, err := LoadWorkspace([]string{root}, false, "auto")
	if err != nil {
		t.Fatal(err)
	}
	cfg.CacheEnabled = false
	plan, err := NewExecutionPlan(cfg, allRulesEnabled(), SurfaceCLI)
	if err != nil {
		t.Fatal(err)
	}
	issues, stats := RunPlan(pkgs, cfg, plan)
	return issues, stats, pkgs
}

// Handwritten code that uses generated declarations (sqlc, protobuf, stringer)
// must keep complete type information, and so must every importer.
func TestGeneratedDeclarationsKeepTypeInformation(t *testing.T) {
	root := t.TempDir()
	writeModuleFixture(t, root, map[string]string{
		"db/models.go": "// Code generated by sqlc. DO NOT EDIT.\n\npackage db\n\ntype Order struct{ ID int }\n\ntype Queries struct{}\n\nfunc (q *Queries) Get(id int) Order { return Order{ID: id} }\n",
		"db/store.go":  "package db\n\ntype Store struct{ queries *Queries }\n\nfunc NewStore(queries *Queries) *Store { return &Store{queries: queries} }\n\nfunc (s *Store) Get(id int) Order { return s.queries.Get(id) }\n",
		"svc/svc.go":   "package svc\n\nimport \"tempmod/db\"\n\ntype Port interface {\n\tA()\n\tB()\n\tC()\n\tD()\n}\n\ntype Service struct {\n\tPort Port\n\tlast db.Order\n}\n\nfunc (s *Service) Run() { s.Port.A() }\n",
	})
	issues, stats, _ := runAllChecks(t, root, DefaultConfig())
	for _, pkg := range stats.Packages {
		if !pkg.TypeComplete {
			t.Fatalf("%s lost type information: %+v", pkg.Package, stats.Packages)
		}
	}
	usageRatio := false
	for _, issue := range issues {
		switch {
		case issue.Check == CheckISPUsageRatio && strings.HasSuffix(filepath.ToSlash(issue.Pos.Filename), "svc/svc.go"):
			usageRatio = true
		case issue.Check == CheckDIPConcreteDependency:
			t.Fatalf("same-package generated type reported as a concrete dependency: %v", issue)
		}
	}
	if !usageRatio {
		t.Fatalf("typed usage-ratio finding missing for an importer of generated code: %v", issues)
	}
}

func TestExcludedFileRebuildFallsBackWhenIncludedCodeNeedsIt(t *testing.T) {
	root := t.TempDir()
	writeModuleFixture(t, root, map[string]string{
		"a/excluded.go": "package a\n\ntype Helper struct{}\n",
		"a/service.go":  "package a\n\ntype Port interface {\n\tA()\n\tB()\n\tC()\n\tD()\n}\n\ntype Service struct {\n\tPort   Port\n\thelper Helper\n}\n\nfunc (s *Service) Run() { s.Port.A() }\n",
	})
	cfg := DefaultConfig()
	cfg.ExcludedFiles = []string{"a/excluded.go"}
	issues, stats, _ := runAllChecks(t, root, cfg)
	if len(stats.Packages) != 1 || !stats.Packages[0].TypeComplete {
		t.Fatalf("exclusion silently disabled typed checks: %+v", stats.Packages)
	}
	if len(stats.Warnings) != 1 || !strings.Contains(stats.Warnings[0], "tempmod/a") || !strings.Contains(stats.Warnings[0], "Helper") {
		t.Fatalf("warnings = %v, want one explaining the kept declarations", stats.Warnings)
	}
	found := false
	for _, issue := range issues {
		found = found || issue.Check == CheckISPUsageRatio
	}
	if !found {
		t.Fatalf("typed findings missing after the exclusion fallback: %v", issues)
	}
}

func TestExcludedFileRebuildIsLimitedToImporters(t *testing.T) {
	root := t.TempDir()
	writeModuleFixture(t, root, map[string]string{
		"a/a.go":        "package a\n\ntype Value struct{}\n",
		"a/excluded.go": "package a\n\ntype Unused struct{}\n",
		"b/b.go":        "package b\n\nimport \"tempmod/a\"\n\nvar Current a.Value\n",
		"c/c.go":        "package c\n\ntype Independent struct{}\n",
	})
	pkgs, _, err := LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	pkgs = ApplyWorkspaceFilePolicy(pkgs, []string{"a/excluded.go"})
	byPath := map[string]*packageFiles{}
	for _, pkg := range pkgs {
		byPath[pkg.pkgPath] = pkg
	}
	a, b, c := byPath["tempmod/a"], byPath["tempmod/b"], byPath["tempmod/c"]
	if a.filteredRebuilds != 1 || b.filteredRebuilds != 1 || c.filteredRebuilds != 0 {
		t.Fatalf("rebuilds a=%d b=%d c=%d, want only the changed package and its importer", a.filteredRebuilds, b.filteredRebuilds, c.filteredRebuilds)
	}
	if a.typePkg.Scope().Lookup("Unused") != nil {
		t.Fatal("excluded declaration remained in the rebuilt snapshot")
	}
	for _, imported := range b.typePkg.Imports() {
		if imported.Path() == "tempmod/a" && imported != a.typePkg {
			t.Fatal("importer was not rebuilt against the filtered package")
		}
	}
	for _, pkg := range []*packageFiles{a, b, c} {
		if !pkg.typeComplete {
			t.Fatalf("%s lost type information", pkg.pkgPath)
		}
	}
}

// Two read-only stores in one file used to abort the whole run with an
// identity collision. Only the colliding findings gain a qualifier.
func TestRun_SameNamedMethodFindingsGetDistinctIdentities(t *testing.T) {
	root := t.TempDir()
	writeModuleFixture(t, root, map[string]string{
		"p/store.go": `package p

import "errors"

type Store interface {
	Get(key string) (string, error)
	Put(key, value string) error
	Delete(key string) error
}

type MemoryStore struct{ data map[string]string }

func (m *MemoryStore) Get(key string) (string, error) { return m.data[key], nil }
func (m *MemoryStore) Put(key, value string) error    { return errors.ErrUnsupported }
func (m *MemoryStore) Delete(key string) error        { return errors.ErrUnsupported }

type FileStore struct{ path string }

func (f *FileStore) Get(key string) (string, error) { return f.path + key, nil }
func (f *FileStore) Put(key, value string) error    { return errors.ErrUnsupported }
func (f *FileStore) Delete(key string) error        { return errors.ErrUnsupported }
`,
		"p/cache.go": `package p

import "errors"

type Cache struct{}

func (Cache) Get(key string) (string, error) { return "", nil }
func (Cache) Put(key, value string) error    { return nil }
func (Cache) Delete(key string) error        { return errors.ErrUnsupported }
`,
	})
	issues, _, _ := runAllChecks(t, root, DefaultConfig())
	if err := FinalizeIssues(issues, "workspace"); err != nil {
		t.Fatal(err)
	}
	var identities []string
	for _, issue := range issues {
		if issue.Check == CheckISPStubImplementation {
			identities = append(identities, filepath.Base(issue.Pos.Filename)+" "+issue.Identity)
		}
	}
	sort.Strings(identities)
	want := []string{
		"cache.go stub-implementation;method=Delete;interface=Store;kind=returns errors.ErrUnsupported",
		"store.go stub-implementation;method=Delete;interface=Store;kind=returns errors.ErrUnsupported;receiver=FileStore",
		"store.go stub-implementation;method=Delete;interface=Store;kind=returns errors.ErrUnsupported;receiver=MemoryStore",
		"store.go stub-implementation;method=Put;interface=Store;kind=returns errors.ErrUnsupported;receiver=FileStore",
		"store.go stub-implementation;method=Put;interface=Store;kind=returns errors.ErrUnsupported;receiver=MemoryStore",
	}
	if strings.Join(identities, "\n") != strings.Join(want, "\n") {
		t.Fatalf("stub identities:\n%s\nwant:\n%s", strings.Join(identities, "\n"), strings.Join(want, "\n"))
	}
}

func TestRun_RepeatedInitFindingsGetOccurrenceIdentities(t *testing.T) {
	root := t.TempDir()
	writeModuleFixture(t, root, map[string]string{
		"p/setup.go": "package p\n\nvar ready bool\n\nfunc init() {\n\tif ready {\n\t\tready = false\n\t}\n}\n\nfunc init() {\n\tif !ready {\n\t\tready = true\n\t}\n}\n",
	})
	cfg := DefaultConfig()
	cfg.MaxFuncLines = 1
	cfg.MaxFuncComplexity = 1
	issues, _, _ := runAllChecks(t, root, cfg)
	var identities []string
	for _, issue := range issues {
		if issue.Check == CheckSRPComplexFunction {
			identities = append(identities, issue.Identity)
		}
	}
	if strings.Join(identities, ",") != "complex-function;function=init,complex-function;function=init;occurrence=2" {
		t.Fatalf("init identities = %v", identities)
	}
}
