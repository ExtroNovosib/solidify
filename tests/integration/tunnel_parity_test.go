package integration_test

import (
	"path/filepath"
	"slices"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/ExtroNovosib/solidify/internal/analysisapi"
	"github.com/ExtroNovosib/solidify/internal/analyzer"
)

func TestTunnelPackagePluginParity(t *testing.T) {
	root := integrationRepositoryRoot(t)
	checks := []analyzer.CheckID{analyzer.CheckLSPDiscardedRead, analyzer.CheckLSPNoopDeadline, analyzer.CheckISPConstructorRole, analyzer.CheckSRPTransportWorkflow}
	cfg := analyzer.DefaultConfig()
	cfg.Profile = analyzer.ProfileStable
	cfg.EnabledChecks = checks
	plan, err := analyzer.NewExecutionPlan(cfg, nil, analyzer.SurfaceModulePlugin)
	if err != nil {
		t.Fatal(err)
	}
	built, err := analysisapi.NewAnalyzers(map[string]any{"enabled_checks": checks})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := packages.Load(&packages.Config{Dir: filepath.Join(root, "testdata/tunnel_calibration/corpus"), Mode: packages.LoadAllSyntax | packages.NeedModule}, "./positive/case000", "./positive/case003", "./positive/case008", "./positive/case091", "./negative/case000", "./negative/case003", "./negative/case008", "./negative/case091")
	if err != nil || packages.PrintErrors(loaded) > 0 {
		t.Fatalf("load corpus: %v", err)
	}
	seen := map[string]bool{}
	for _, pkg := range loaded {
		for _, group := range plan.Groups() {
			var itemName string
			switch group.Name {
			case "lsp-package":
				itemName = "solidlsp"
			case "isp-package":
				itemName = "solidisp"
			case "srp-package":
				itemName = "solidsrp"
			}
			for _, item := range built {
				if item.Name != itemName {
					continue
				}
				cli := cliIssueCategories(analyzer.SnapshotFromPackages(pkg).RunGroup(group, cfg))
				plugin := pluginDiagnosticCategories(t, item, pkg)
				if !slices.Equal(cli, plugin) {
					t.Fatalf("%s %s: CLI=%v plugin=%v", pkg.PkgPath, group.Name, cli, plugin)
				}
				for _, id := range plugin {
					seen[id] = true
				}
			}
		}
	}
	for _, check := range checks {
		if !seen[string(check)] {
			t.Errorf("missing plugin proof for %s", check)
		}
	}
}
