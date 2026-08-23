package analyzer

import (
	"path/filepath"
	"strings"
	"testing"
)

func verdictDir(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(testdataDir(t, "verdict"), name)
}

func runVerdictPackage(t *testing.T, name string) []Issue {
	t.Helper()
	pkgs, _, err := LoadWorkspace([]string{verdictDir(t, name)}, false, "types")
	if err != nil {
		t.Fatalf("LoadWorkspace(%s): %v", name, err)
	}
	return Run(pkgs, DefaultConfig(), allRulesEnabled())
}

func runDIPVerdictModule(t *testing.T, cfg Config) []Issue {
	t.Helper()
	root := testdataDir(t, "dip_verdict")
	pkgs, _, err := LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		t.Fatalf("LoadWorkspace(dip_verdict): %v", err)
	}
	return Run(pkgs, cfg, allRulesEnabled())
}

func dipVerdictConfig() Config {
	cfg := DefaultConfig()
	cfg.OCPLogicPackages = []string{"example.com/dipverdict/dip_logic_import"}
	cfg.OCPImplementationPackages = []string{"example.com/dipverdict/adapters/**"}
	cfg.OCPCompositionRoots = []string{"example.com/dipverdict/dip_composition_root"}
	return cfg
}

func issuesWithCheck(issues []Issue, check CheckID) []Issue {
	var out []Issue
	for _, issue := range issues {
		if issue.Check == check {
			out = append(out, issue)
		}
	}
	return out
}

func hasCheck(issues []Issue, check CheckID) bool {
	return len(issuesWithCheck(issues, check)) > 0
}

func TestVerdictCorpus(t *testing.T) {
	t.Run("god_hub", func(t *testing.T) {
		issues := runVerdictPackage(t, "god_hub")
		if !hasCheck(issues, CheckSRPLargeType) {
			t.Fatalf("want SOLID-S/large-type on HubRuntime, got: %v", issues)
		}
		for _, issue := range issues {
			if issue.Check == CheckSRPLargeType && !strings.Contains(issue.Evidence, "type=HubRuntime") {
				t.Fatalf("large-type should target HubRuntime, got %q", issue.Evidence)
			}
		}
	})

	t.Run("god_console_package_scan", func(t *testing.T) {
		issues := runVerdictPackage(t, "god_console")
		large := issuesWithCheck(issues, CheckSRPLargeType)
		if len(large) == 0 {
			t.Fatalf("package scan should report large-type on ConsoleRuntime, got: %v", issues)
		}
	})

	t.Run("god_console_single_file_undercounts", func(t *testing.T) {
		path := filepath.Join(verdictDir(t, "god_console"), "console.go")
		pkg, err := parsePackageFiles(filepath.Dir(path), []string{path}, true)
		if err != nil {
			t.Fatal(err)
		}
		issues := Run([]*packageFiles{pkg}, DefaultConfig(), allRulesEnabled())
		if hasCheck(issues, CheckSRPLargeType) {
			t.Fatalf("single definition file should not surface large-type alone; scan the package directory")
		}
	})

	t.Run("edge_proxy", func(t *testing.T) {
		issues := runVerdictPackage(t, "edge_proxy")
		if !hasCheck(issues, CheckSRPMixedInputSurface) {
			t.Fatalf("want mixed-input-surface on callRelayStatus, got: %v", issues)
		}
		if hasCheck(issues, CheckSRPLargeType) {
			t.Fatalf("semantic proxy coupling is not a large-type signal: %v", issues)
		}
	})

	t.Run("thin_repo", func(t *testing.T) {
		issues := runVerdictPackage(t, "thin_repo")
		if hasCheck(issues, CheckSRPLargeType) {
			t.Fatalf("thin wrapper should not trigger large-type: %v", issues)
		}
	})

	t.Run("quota_guard", func(t *testing.T) {
		issues := runVerdictPackage(t, "quota_guard")
		if hasCheck(issues, CheckSRPLargeType) {
			t.Fatalf("cohesive quota guard should not trigger large-type: %v", issues)
		}
	})

	t.Run("domain_blob", func(t *testing.T) {
		issues := runVerdictPackage(t, "domain_blob")
		if hasCheck(issues, CheckSRPLargeType) {
			t.Fatalf("DTO should not trigger large-type: %v", issues)
		}
	})

	t.Run("wiring_root", func(t *testing.T) {
		issues := runVerdictPackage(t, "wiring_root")
		for _, issue := range issues {
			if issue.Rule != RuleDIP {
				continue
			}
			if strings.Contains(issue.Message, "WireRoot.") {
				t.Fatalf("composition root fields should not trigger DIP: %v", issues)
			}
		}
	})

	t.Run("bridge_port", func(t *testing.T) {
		issues := runVerdictPackage(t, "bridge_port")
		for _, issue := range issues {
			if issue.Rule != RuleDIP {
				continue
			}
			if strings.Contains(issue.Message, "RuntimePort.") {
				t.Fatalf("bridge adapter fields should not trigger DIP: %v", issues)
			}
		}
	})

	t.Run("mixed_concerns", func(t *testing.T) {
		issues := runVerdictPackage(t, "mixed_concerns")
		if !hasCheck(issues, CheckSRPMixedImportClusters) {
			t.Fatalf("want mixed-import-clusters on Gateway, got: %v", issues)
		}
		for _, issue := range issues {
			if issue.Check == CheckSRPMixedImportClusters && !strings.Contains(issue.Evidence, "type=Gateway") {
				t.Fatalf("mixed-import-clusters should target Gateway, got %q", issue.Evidence)
			}
		}
	})

	t.Run("dip_logic_import", func(t *testing.T) {
		issues := runDIPVerdictModule(t, dipVerdictConfig())
		layer := issuesWithCheck(issues, CheckDIPLayerImport)
		if len(layer) == 0 {
			t.Fatalf("want layer-import on logic package, got: %v", issues)
		}
		for _, issue := range layer {
			if !strings.Contains(issue.Evidence, "from=example.com/dipverdict/dip_logic_import") {
				t.Fatalf("unexpected layer-import evidence: %q", issue.Evidence)
			}
		}
	})

	t.Run("dip_composition_root", func(t *testing.T) {
		issues := runDIPVerdictModule(t, dipVerdictConfig())
		for _, issue := range issues {
			if issue.Check == CheckDIPWiringOutsideRoot && strings.Contains(issue.Evidence, "package=example.com/dipverdict/dip_composition_root") {
				t.Fatalf("composition root wiring should be exempt: %v", issue)
			}
		}
	})
}
