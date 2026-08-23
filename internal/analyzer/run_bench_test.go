package analyzer

import (
	"path/filepath"
	"testing"
)

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
