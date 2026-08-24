package analyzer

import (
	"reflect"
	"testing"
)

func TestExecutionPlanSchedulesSelectedRunnerGroups(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Profile = ProfileStable
	cfg.EnabledChecks = []CheckID{CheckLSPNonExactEOF}
	plan, err := NewExecutionPlan(cfg, map[Rule]bool{
		RuleSRP: true, RuleLSP: true, RuleISP: true, RuleDIP: true,
	}, SurfaceCLI)
	if err != nil {
		t.Fatal(err)
	}
	wantChecks := []CheckID{
		CheckSRPLargeType, CheckSRPDataClump, CheckLSPNonExactEOF,
		CheckISPFatInterface, CheckISPUsageRatio, CheckISPStubImplementation,
		CheckDIPConcreteDependency,
	}
	if got := plan.SelectedCheckIDs(); !reflect.DeepEqual(got, wantChecks) {
		t.Fatalf("selected checks = %v, want %v", got, wantChecks)
	}
	wantGroups := []string{"srp-package", "lsp-package", "isp-package", "dip-package"}
	groups := plan.Groups()
	gotGroups := make([]string, len(groups))
	for index, group := range groups {
		gotGroups[index] = group.Name
	}
	if !reflect.DeepEqual(gotGroups, wantGroups) {
		t.Fatalf("groups = %v, want %v", gotGroups, wantGroups)
	}

	groups[0].Checks[0] = CheckSRPGodType
	if !plan.Includes(CheckSRPLargeType) || plan.Includes(CheckSRPGodType) {
		t.Fatal("Groups exposed mutable plan selection")
	}
}

func TestExecutionPlanFiltersUnsupportedPluginChecks(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Profile = ProfileAll
	plan, err := NewExecutionPlan(cfg, map[Rule]bool{
		RuleSRP: true, RuleOCP: true, RuleLSP: true, RuleISP: true, RuleDIP: true,
	}, SurfaceModulePlugin)
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range plan.Groups() {
		if group.Scope == ScopeProgram {
			t.Fatalf("plugin plan contains program group %q", group.Name)
		}
		for _, id := range group.Checks {
			metadata, _ := CheckMetadata(id)
			if !metadata.Surfaces.Supports(SurfaceModulePlugin) {
				t.Fatalf("plugin plan contains unsupported check %q", id)
			}
		}
	}
}

func TestRunPlanFiltersInsideRunnerGroup(t *testing.T) {
	pkgs, _, err := LoadWorkspace([]string{"../../testdata/violations"}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.Profile = ProfileStable
	cfg.CacheEnabled = false
	cfg.DisabledChecks = []CheckID{CheckISPUsageRatio, CheckISPStubImplementation}
	plan, err := NewExecutionPlan(cfg, map[Rule]bool{RuleISP: true}, SurfaceCLI)
	if err != nil {
		t.Fatal(err)
	}
	issues, stats := RunPlan(pkgs, cfg, plan)
	for _, issue := range issues {
		if issue.Check != CheckISPFatInterface {
			t.Fatalf("unexpected check %q from selected runner group", issue.Check)
		}
	}
	if len(stats.Groups) != 1 || stats.Groups[0].Name != "isp-package" || stats.Groups[0].Executions != len(pkgs) {
		t.Fatalf("stats = %+v, want one ISP execution per package (%d)", stats.Groups, len(pkgs))
	}
}
