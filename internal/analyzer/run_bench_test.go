package analyzer

import (
	"path/filepath"
	"strconv"
	"testing"
)

func BenchmarkRunSelectedCold(b *testing.B) {
	root := testdataDir(b, "violations")
	cacheRoot := b.TempDir()
	enabled := map[Rule]bool{RuleISP: true}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		cfg := DefaultConfig()
		cfg.CacheDir = filepath.Join(cacheRoot, strconv.Itoa(index))
		benchmarkSelectedRun(b, root, cfg, enabled)
	}
}

func BenchmarkRunSelectedWarm(b *testing.B) {
	root := testdataDir(b, "violations")
	cfg := DefaultConfig()
	cfg.CacheDir = filepath.Join(b.TempDir(), "warm-selected")
	enabled := map[Rule]bool{RuleISP: true}
	benchmarkSelectedRun(b, root, cfg, enabled)
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		benchmarkSelectedRun(b, root, cfg, enabled)
	}
}

func BenchmarkRunSelectedDisabled(b *testing.B) {
	root := testdataDir(b, "violations")
	cfg := DefaultConfig()
	cfg.CacheEnabled = false
	enabled := map[Rule]bool{RuleISP: true}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		benchmarkSelectedRun(b, root, cfg, enabled)
	}
}

func benchmarkSelectedRun(b *testing.B, root string, cfg Config, enabled map[Rule]bool) {
	b.Helper()
	pkgs, _, err := LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		b.Fatal(err)
	}
	if issues := Run(pkgs, cfg, enabled); len(issues) == 0 {
		b.Fatal("expected selected ISP findings")
	}
}

// BenchmarkRunViolationsCold measures analysis with caching disabled.
func BenchmarkRunViolationsCold(b *testing.B) {
	root := testdataDir(b, "violations")
	cfg := DefaultConfig()
	cfg.CacheEnabled = false
	enabled := allRulesEnabled()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pkgs, _, err := LoadWorkspace([]string{root}, false, "types")
		if err != nil {
			b.Fatal(err)
		}
		if issues := Run(pkgs, cfg, enabled); len(issues) == 0 {
			b.Fatal("expected violations corpus findings")
		}
	}
}

// BenchmarkRunViolationsWarm measures repeated analysis against populated cache entries.
func BenchmarkRunViolationsWarm(b *testing.B) {
	root := testdataDir(b, "violations")
	cfg := DefaultConfig()
	cfg.CacheDir = filepath.Join(b.TempDir(), "warm")
	enabled := allRulesEnabled()

	pkgs, _, err := LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		b.Fatal(err)
	}
	if issues := Run(pkgs, cfg, enabled); len(issues) == 0 {
		b.Fatal("expected violations corpus findings")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pkgs, _, err := LoadWorkspace([]string{root}, false, "types")
		if err != nil {
			b.Fatal(err)
		}
		if issues := Run(pkgs, cfg, enabled); len(issues) == 0 {
			b.Fatal("expected violations corpus findings")
		}
	}
}
