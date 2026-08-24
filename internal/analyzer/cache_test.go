package analyzer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCacheDigestIOIsLazyAndCompact(t *testing.T) {
	root := testdataDir(t, "violations")
	pkgs, _, err := LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range pkgs {
		if pkg.dependencyFacts != "" {
			t.Fatalf("package %s eagerly retained dependency source/export data", pkg.pkgPath)
		}
	}
	cfg := DefaultConfig()
	cfg.CacheDir = filepath.Join(t.TempDir(), "cache")
	plan, err := NewExecutionPlan(cfg, allRulesEnabled(), SurfaceCLI)
	if err != nil {
		t.Fatal(err)
	}
	cache := newPackageCache(cfg.CacheDir, cfg, plan)
	if reads := cache.sourceReads.Load(); reads != 0 {
		t.Fatalf("source reads before cache key demand = %d", reads)
	}
	for _, pkg := range pkgs {
		digest := cache.packageHash(pkg)
		if len(digest) != 64 || strings.Contains(digest, "package ") {
			t.Fatalf("package digest is not compact SHA-256: %q", digest)
		}
	}
	if reads := cache.sourceReads.Load(); reads == 0 {
		t.Fatal("cache key demand did not hash local sources")
	}
}

func TestProgramCacheWarmHitPreservesFindings(t *testing.T) {
	root := testdataDir(t, "violations")
	cfg := DefaultConfig()
	cfg.CacheDir = filepath.Join(t.TempDir(), "cache")
	enabled := map[Rule]bool{RuleOCP: true}
	plan, err := NewExecutionPlan(cfg, enabled, SurfaceCLI)
	if err != nil {
		t.Fatal(err)
	}
	pkgs, _, err := LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	cold, coldStats := RunPlan(pkgs, cfg, plan)
	pkgs, _, err = LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	warm, warmStats := RunPlan(pkgs, cfg, plan)
	if issueSignatures(cold) != issueSignatures(warm) {
		t.Fatalf("program cache changed findings\ncold=%v\nwarm=%v", cold, warm)
	}
	if len(coldStats.Groups) != 1 || coldStats.Groups[0].Executions != 1 || coldStats.Groups[0].CacheMisses != 1 {
		t.Fatalf("cold program stats = %+v", coldStats.Groups)
	}
	if len(warmStats.Groups) != 1 || warmStats.Groups[0].Executions != 0 || warmStats.Groups[0].CacheHits != 1 {
		t.Fatalf("warm program stats = %+v", warmStats.Groups)
	}
}

func TestPackageCacheCorruptEntryCountsAsMiss(t *testing.T) {
	dir := t.TempDir()
	initTempModule(t, dir)
	pkgDir := filepath.Join(dir, "service")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "service.go"), []byte(`package service

import "tempmod/other"

type Service struct { driver *other.Driver }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	otherDir := filepath.Join(dir, "other")
	if err := os.MkdirAll(otherDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherDir, "driver.go"), []byte("package other\n\ntype Driver struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	cfg.CacheDir = filepath.Join(dir, "cache")
	enabled := map[Rule]bool{RuleDIP: true}
	pkgs, _, err := LoadWorkspace([]string{pkgDir}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := NewExecutionPlan(cfg, enabled, SurfaceCLI)
	if err != nil {
		t.Fatal(err)
	}
	cache := newPackageCache(cfg.CacheDir, cfg, plan)
	issues := Run(pkgs, cfg, enabled)
	if len(issues) != 1 {
		t.Fatalf("initial findings = %v, want one", issues)
	}

	cacheID := groupCacheID(plan.Groups()[0])
	path := cache.entryPath(pkgs[0], cacheID)
	if err := os.WriteFile(path, []byte(`{"version":"solidlint-cache-v6","hash":"truncated`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := cache.load(pkgs[0], cacheID); ok {
		t.Fatal("truncated cache entry should miss")
	}
	if cache.corrupt.Load() != 1 {
		t.Fatalf("corrupt count = %d, want 1", cache.corrupt.Load())
	}
}

func TestPackageCacheWarmHitPreservesAbsenceOfUnownedFixes(t *testing.T) {
	root := testdataDir(t, "violations")
	cfg := DefaultConfig()
	cfg.CacheDir = filepath.Join(t.TempDir(), "cache")
	enabled := map[Rule]bool{RuleISP: true}
	pkgs, _, err := LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	cold := Run(pkgs, cfg, enabled)
	for _, issue := range cold {
		if len(issue.SuggestedFixes) != 0 {
			t.Fatalf("cold finding has unowned fix: %+v", issue)
		}
	}

	pkgs, _, err = LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	warm := Run(pkgs, cfg, enabled)
	for _, issue := range warm {
		if len(issue.SuggestedFixes) != 0 {
			t.Fatalf("warm finding has unowned fix: %+v", issue)
		}
	}
}
