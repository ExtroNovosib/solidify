package solidlint

import (
	"testing"

	"github.com/ExtroNovosib/solidify/internal/analysisapi"
)

func TestModulePluginContract(t *testing.T) {
	plugin, err := New(map[string]any{"profile": "stable"})
	if err != nil {
		t.Fatal(err)
	}
	analyzers, err := plugin.BuildAnalyzers()
	if err != nil {
		t.Fatal(err)
	}
	if len(analyzers) != 4 {
		t.Fatalf("analyzers = %d, want SRP/LSP/ISP/DIP", len(analyzers))
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
