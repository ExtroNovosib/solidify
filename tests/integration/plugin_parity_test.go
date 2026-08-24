package integration_test

import (
	"go/types"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"

	"github.com/ExtroNovosib/solidify/internal/analysisapi"
	"github.com/ExtroNovosib/solidify/internal/analyzer"
)

func TestPluginAndCLIParityForExternalAndIncompleteTypes(t *testing.T) {
	tests := []struct {
		fixture  string
		analyzer string
		group    analyzer.ExecutionGroup
	}{
		{fixture: "clean", analyzer: "solidisp", group: ispPackageGroup()},
		{fixture: "fat", analyzer: "solidisp", group: ispPackageGroup()},
		{fixture: "concrete", analyzer: "soliddip", group: dipPackageGroup()},
		{fixture: "incomplete", analyzer: "solidisp", group: ispPackageGroup()},
		{fixture: "incomplete-dip", analyzer: "soliddip", group: dipPackageGroup()},
		{fixture: "externaliface", analyzer: "solidisp", group: ispPackageGroup()},
	}
	for _, test := range tests {
		t.Run(test.fixture, func(t *testing.T) {
			pluginAnalyzer := selectedFactoryAnalyzer(t, test.analyzer)
			for _, pkg := range loadParityFixture(t, test.fixture) {
				cliCategories := cliIssueCategories(analyzer.SnapshotFromPackages(pkg).RunGroup(test.group, analyzer.DefaultConfig()))
				pluginCategories := pluginDiagnosticCategories(t, pluginAnalyzer, pkg)
				if !slices.Equal(pluginCategories, cliCategories) {
					t.Fatalf("package %s: plugin categories %v != CLI categories %v", pkg.PkgPath, pluginCategories, cliCategories)
				}
			}
		})
	}
}

func loadParityFixture(t *testing.T, fixture string) []*packages.Package {
	t.Helper()
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(filename), "..", "..", "internal", "analysisapi", "testdata", "src", fixture)
	loaded, err := packages.Load(&packages.Config{Dir: dir, Mode: packages.LoadAllSyntax | packages.NeedModule}, "./...")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) == 0 {
		t.Fatalf("fixture %q loaded no packages", fixture)
	}
	return loaded
}

func selectedFactoryAnalyzer(t *testing.T, name string) *analysis.Analyzer {
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
	t.Fatalf("factory did not build analyzer %q", name)
	return nil
}

func pluginDiagnosticCategories(t *testing.T, item *analysis.Analyzer, pkg *packages.Package) []string {
	t.Helper()
	var categories []string
	var typeErrors []types.Error
	if pkg.IllTyped {
		typeErrors = []types.Error{{Msg: "package is ill-typed"}}
	}
	var module *analysis.Module
	if pkg.Module != nil {
		module = &analysis.Module{Path: pkg.Module.Path}
	}
	pass := &analysis.Pass{
		Analyzer: item, Fset: pkg.Fset, Files: pkg.Syntax, Pkg: pkg.Types,
		TypesInfo: pkg.TypesInfo, TypeErrors: typeErrors, Module: module,
		Report: func(diagnostic analysis.Diagnostic) {
			categories = append(categories, diagnostic.Category)
		},
	}
	if _, err := item.Run(pass); err != nil {
		t.Fatal(err)
	}
	sort.Strings(categories)
	return categories
}

func cliIssueCategories(issues []analyzer.Issue) []string {
	categories := make([]string, len(issues))
	for index, issue := range issues {
		categories[index] = issue.ID()
	}
	sort.Strings(categories)
	return categories
}

func ispPackageGroup() analyzer.ExecutionGroup {
	return analyzer.ExecutionGroup{Name: "isp-package", Scope: analyzer.ScopePackage, Checks: []analyzer.CheckID{
		analyzer.CheckISPFatInterface, analyzer.CheckISPUsageRatio, analyzer.CheckISPStubImplementation,
	}}
}

func dipPackageGroup() analyzer.ExecutionGroup {
	return analyzer.ExecutionGroup{Name: "dip-package", Scope: analyzer.ScopePackage, Checks: []analyzer.CheckID{
		analyzer.CheckDIPConcreteDependency,
	}}
}
