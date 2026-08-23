package integration_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ExtroNovosib/solidify/internal/analyzer"
	"github.com/ExtroNovosib/solidify/internal/baseline"
	"github.com/ExtroNovosib/solidify/internal/report"
)

func TestAnalyzerReportBaselineFlow(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(filename), "..", "..", "testdata", "violations")
	packages, _, err := analyzer.LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	cfg := analyzer.DefaultConfig()
	cfg.Profile = analyzer.ProfileStable
	issues := analyzer.Run(packages, cfg, map[analyzer.Rule]bool{analyzer.RuleSRP: true, analyzer.RuleOCP: true, analyzer.RuleISP: true, analyzer.RuleDIP: true})
	if len(issues) == 0 {
		t.Fatal("expected stable findings")
	}
	if _, err := report.EncodeJSON(issues); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := baseline.Write(path, issues); err != nil {
		t.Fatal(err)
	}
	accepted, err := baseline.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if remaining := baseline.Filter(append([]analyzer.Issue(nil), issues...), accepted); len(remaining) != 0 {
		t.Fatalf("remaining = %v", remaining)
	}
}
