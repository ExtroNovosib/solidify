package solidlint

import (
	"testing"

	"github.com/ExtroNovosib/solidify/internal/analysisapi"
)

func TestModulePluginContract(t *testing.T) {
	settings := map[string]any{"profile": "stable"}
	plugin, err := New(settings)
	if err != nil {
		t.Fatal(err)
	}
	settings["profile"] = "all"
	analyzers, err := plugin.BuildAnalyzers()
	if err != nil {
		t.Fatal(err)
	}
	if len(analyzers) != 3 {
		t.Fatalf("analyzers = %d, want selected stable SRP/ISP/DIP groups", len(analyzers))
	}
	if analyzers[0].Name != "solidsrp" || analyzers[1].Name != "solidisp" || analyzers[2].Name != "soliddip" {
		t.Fatalf("analyzers = %v, want deterministic selected groups", []string{analyzers[0].Name, analyzers[1].Name, analyzers[2].Name})
	}
}

func TestPluginSettingsAreStrictAndRejectProgramChecks(t *testing.T) {
	for name, settings := range map[string]any{
		"unknown": map[string]any{"surprise": true},
		"program": map[string]any{"enabled_checks": []string{"SOLID-O/type-dispatch"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := analysisapi.NewAnalyzers(settings); err == nil {
				t.Fatal("settings unexpectedly accepted")
			}
		})
	}
}
