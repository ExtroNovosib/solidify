package analysisapi_test

import (
	"reflect"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/ExtroNovosib/solidify/internal/analysisapi"
)

func TestISPAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), factoryAnalyzer(t, "solidisp"), "fat")
}

func TestISPAnalyzerClean(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), factoryAnalyzer(t, "solidisp"), "clean")
}

func TestISPAnalyzerStub(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), factoryAnalyzer(t, "solidisp"), "stub")
}

func TestISPAnalyzerGenerated(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), factoryAnalyzer(t, "solidisp"), "generated")
}

func TestISPAnalyzerIncompleteTypes(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), factoryAnalyzer(t, "solidisp"), "incomplete")
}

func TestDIPAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), factoryAnalyzer(t, "soliddip"), "concrete")
}

func TestDIPAnalyzerClean(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), factoryAnalyzer(t, "soliddip"), "clean")
}

func TestDIPAnalyzerGenerated(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), factoryAnalyzer(t, "soliddip"), "generated")
}

func TestDIPAnalyzerIncompleteTypes(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), factoryAnalyzer(t, "soliddip"), "incomplete-dip")
}

func TestFactoryBuildsSelectedAnalyzerGroups(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]any
		want     []string
	}{
		{name: "stable", settings: map[string]any{"profile": "stable"}, want: []string{"solidsrp", "solidisp", "soliddip"}},
		{name: "lsp only", settings: map[string]any{"profile": "all", "enabled_rules": []string{"L"}}, want: []string{"solidlsp"}},
		{name: "dip only", settings: map[string]any{"profile": "all", "enabled_rules": []string{"D"}}, want: []string{"soliddip"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			built, err := analysisapi.NewAnalyzers(test.settings)
			if err != nil {
				t.Fatal(err)
			}
			names := make([]string, len(built))
			for index, item := range built {
				names[index] = item.Name
			}
			if !reflect.DeepEqual(names, test.want) {
				t.Fatalf("analyzers = %v, want %v", names, test.want)
			}
		})
	}
}

func TestFactoryRespectsSyntaxSupportOnTypeErrors(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), factoryAnalyzer(t, "solidisp"), "incomplete")
	analysistest.Run(t, analysistest.TestData(), factoryAnalyzer(t, "soliddip"), "incomplete-dip")
}

func factoryAnalyzer(t *testing.T, name string) *analysis.Analyzer {
	t.Helper()
	built, err := analysisapi.NewAnalyzers(map[string]any{"profile": "stable"})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range built {
		if item.Name == name {
			return item
		}
	}
	t.Fatalf("factory did not build %q", name)
	return nil
}
