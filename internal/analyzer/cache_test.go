package analyzer

import (
	"os"
	"path/filepath"
	"testing"
)

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
	cache := newPackageCache(cfg.CacheDir, cfg, enabled)
	issues := Run(pkgs, cfg, enabled)
	if len(issues) != 1 {
		t.Fatalf("initial findings = %v, want one", issues)
	}

	path := cache.entryPath(pkgs[0], CheckDIPConcreteDependency)
	if err := os.WriteFile(path, []byte(`{"version":"solidlint-cache-v6","hash":"truncated`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := cache.load(pkgs[0], CheckDIPConcreteDependency); ok {
		t.Fatal("truncated cache entry should miss")
	}
	if cache.corrupt.Load() != 1 {
		t.Fatalf("corrupt count = %d, want 1", cache.corrupt.Load())
	}
}

func TestPackageCacheWarmHitPreservesSuggestedFixes(t *testing.T) {
	root := testdataDir(t, "violations")
	cfg := DefaultConfig()
	cfg.CacheDir = filepath.Join(t.TempDir(), "cache")
	enabled := map[Rule]bool{RuleISP: true}
	pkgs, _, err := LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	cold := Run(pkgs, cfg, enabled)
	var stub *Issue
	for index := range cold {
		if cold[index].Check == CheckISPStubImplementation && len(cold[index].SuggestedFixes) > 0 {
			stub = &cold[index]
			break
		}
	}
	if stub == nil {
		t.Fatal("expected ISP stub finding with suggested fixes")
	}

	pkgs, _, err = LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	warm := Run(pkgs, cfg, enabled)
	var warmStub *Issue
	for index := range warm {
		if warm[index].Check == CheckISPStubImplementation {
			warmStub = &warm[index]
			break
		}
	}
	if warmStub == nil {
		t.Fatal("expected warm ISP stub finding")
	}
	if len(warmStub.SuggestedFixes) != len(stub.SuggestedFixes) {
		t.Fatalf("warm suggested fixes = %d, want %d", len(warmStub.SuggestedFixes), len(stub.SuggestedFixes))
	}
	if warmStub.SuggestedFixes[0].Message != stub.SuggestedFixes[0].Message {
		t.Fatalf("warm fix message = %q, want %q", warmStub.SuggestedFixes[0].Message, stub.SuggestedFixes[0].Message)
	}
	if len(warmStub.SuggestedFixes[0].Edits) != len(stub.SuggestedFixes[0].Edits) {
		t.Fatalf("warm fix edits = %d, want %d", len(warmStub.SuggestedFixes[0].Edits), len(stub.SuggestedFixes[0].Edits))
	}
}
