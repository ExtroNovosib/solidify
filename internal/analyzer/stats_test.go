package analyzer

import "testing"

func TestAnalysisCoverageReasons(t *testing.T) {
	allRules := map[Rule]bool{RuleSRP: true, RuleOCP: true, RuleLSP: true, RuleISP: true, RuleDIP: true}
	packageWithTypes := []*packageFiles{{pkgPath: "example.com/complete", typeComplete: true}}
	packageWithoutTypes := []*packageFiles{{pkgPath: "example.com/incomplete", typeComplete: false}}

	t.Run("selection", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Profile = ProfileStable
		plan, err := NewExecutionPlan(cfg, allRules, SurfaceCLI)
		if err != nil {
			t.Fatal(err)
		}
		coverage := coverageByCheck(t, newRunStats(plan, cfg).snapshot(packageWithTypes))
		assertCoverage(t, coverage, CheckSRPLargeType, "complete", "complete go/types information was available for every analyzed package")
		assertCoverage(t, coverage, CheckLSPNonExactEOF, "not-selected", "not selected by the resolved profile, rule, and check configuration")
	})

	t.Run("syntax contracts and architecture configuration", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Profile = ProfileAll
		cfg.AnalysisMode = syntaxAnalysisMode
		plan, err := NewExecutionPlan(cfg, allRules, SurfaceCLI)
		if err != nil {
			t.Fatal(err)
		}
		coverage := coverageByCheck(t, newRunStats(plan, cfg).snapshot(packageWithoutTypes))
		assertCoverage(t, coverage, CheckISPFatInterface, "syntax-equivalent", "this check is equivalent without complete go/types information")
		assertCoverage(t, coverage, CheckSRPLargeType, "conservative", "one or more packages lack complete go/types information; this check ran its conservative syntax fallback")
		assertCoverage(t, coverage, CheckLSPNonExactEOF, "unavailable", "complete go/types information was unavailable for every analyzed package")
		assertCoverage(t, coverage, CheckOCPImplementationCoupling, "configuration-unavailable", "architecture.logic_packages and architecture.implementation_packages must both be configured")
	})

	t.Run("partial and unsupported surface", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Profile = ProfileAll
		cfg.AnalysisMode = analysisModeAuto
		plan, err := NewExecutionPlan(cfg, allRules, SurfaceCLI)
		if err != nil {
			t.Fatal(err)
		}
		coverage := coverageByCheck(t, newRunStats(plan, cfg).snapshot(append(packageWithTypes, packageWithoutTypes...)))
		assertCoverage(t, coverage, CheckLSPNonExactEOF, "partial", "complete go/types information was unavailable for one or more packages; findings from those packages were omitted")

		pluginPlan, err := NewExecutionPlan(cfg, allRules, SurfaceModulePlugin)
		if err != nil {
			t.Fatal(err)
		}
		pluginCoverage := coverageByCheck(t, newRunStats(pluginPlan, cfg).snapshot(packageWithTypes))
		assertCoverage(t, pluginCoverage, CheckOCPTypeDispatch, "unsupported-surface", "the module-plugin integration does not support this check")
	})
}

func coverageByCheck(t *testing.T, stats ExecutionStats) map[CheckID]CheckCoverageStats {
	t.Helper()
	if len(stats.CheckCoverage) != len(RegisteredCheckIDs()) {
		t.Fatalf("coverage entries = %d, want %d: %+v", len(stats.CheckCoverage), len(RegisteredCheckIDs()), stats.CheckCoverage)
	}
	coverage := make(map[CheckID]CheckCoverageStats, len(stats.CheckCoverage))
	for index, entry := range stats.CheckCoverage {
		if entry.Check != RegisteredCheckIDs()[index] {
			t.Fatalf("coverage[%d] = %s, want registry ID %s", index, entry.Check, RegisteredCheckIDs()[index])
		}
		if entry.Status == "" || entry.Reason == "" {
			t.Fatalf("coverage for %s is incomplete: %+v", entry.Check, entry)
		}
		coverage[entry.Check] = entry
	}
	return coverage
}

func assertCoverage(t *testing.T, coverage map[CheckID]CheckCoverageStats, check CheckID, status, reason string) {
	t.Helper()
	entry, ok := coverage[check]
	if !ok || entry.Status != status || entry.Reason != reason {
		t.Fatalf("coverage[%s] = %+v, want status=%q reason=%q", check, entry, status, reason)
	}
}
