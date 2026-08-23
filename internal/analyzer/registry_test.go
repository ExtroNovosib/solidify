package analyzer

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestCheckRegistryCompleteness(t *testing.T) {
	expected := []CheckID{
		CheckSRPGodType, CheckSRPLowCohesionType, CheckSRPLargeType,
		CheckSRPHighFanOutType, CheckSRPComplexFunction, CheckSRPMixedInputSurface,
		CheckSRPDataClump, CheckSRPFlagArgument, CheckSRPMixedImportClusters,
		CheckOCPTypeDispatch, CheckOCPDiscriminatorDispatch, CheckOCPRuntimeExhaustiveness,
		CheckOCPConcreteParameter, CheckOCPClosedFactory, CheckOCPImplementationCoupling,
		CheckOCPParallelImplementations, CheckLSPNonExactEOF, CheckLSPNilEmbeddedInterface,
		CheckISPFatInterface, CheckISPUsageRatio, CheckISPStubImplementation,
		CheckDIPConcreteDependency, CheckDIPLayerImport, CheckDIPWiringOutsideRoot,
		CheckDIPHiddenConstruction, CheckDIPInfraErrorLeak, CheckDIPTransportLeak,
	}
	got := RegisteredCheckIDs()
	sort.Slice(expected, func(i, j int) bool { return expected[i] < expected[j] })
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	if strings.Join(checkIDStrings(got), "\n") != strings.Join(checkIDStrings(expected), "\n") {
		t.Fatalf("registered checks:\n%v\nwant:\n%v", got, expected)
	}

	seen := map[CheckID]bool{}
	for _, check := range checkRegistry {
		if seen[check.ID] {
			t.Errorf("duplicate check ID %q", check.ID)
		}
		seen[check.ID] = true
		if check.Name == "" || check.Doc == "" || check.RunnerGroup == "" {
			t.Errorf("%s has incomplete metadata: %+v", check.ID, check)
		}
		if check.Maturity != MaturityStable && check.Maturity != MaturityExperimental {
			t.Errorf("%s has invalid maturity %q", check.ID, check.Maturity)
		}
		if check.Syntax != SyntaxEquivalent && check.Syntax != SyntaxConservative && check.Syntax != SyntaxUnavailable {
			t.Errorf("%s has invalid syntax capability %q", check.ID, check.Syntax)
		}
		if !check.Surfaces.Supports(SurfaceCLI) {
			t.Errorf("%s is not available through the standalone CLI", check.ID)
		}
		if !validSeverity(check.DefaultSev) {
			t.Errorf("%s has invalid default severity %q", check.ID, check.DefaultSev)
		}
		expectedHelp := checkDocsBaseURI + "/" + string(check.ID) + ".md"
		if check.HelpURI != expectedHelp {
			t.Errorf("%s help URI = %q, want %q", check.ID, check.HelpURI, expectedHelp)
		}
		docPath := filepath.Join("..", "..", "docs", "checks", filepath.FromSlash(string(check.ID))+".md")
		document, err := os.ReadFile(docPath)
		if err != nil {
			t.Errorf("%s documentation: %v", check.ID, err)
		} else {
			text := string(document)
			for _, required := range []string{"## Product contract", "Maturity: **" + string(check.Maturity) + "**", "Analysis modes:", "Surfaces:", "## Examples", "Positive:", "Clean:", "## Limitations and remediation"} {
				if !strings.Contains(text, required) {
					t.Errorf("%s documentation lacks %q", check.ID, required)
				}
			}
		}
	}
}

func TestCheckRegistryProfilesModesAndSurfaces(t *testing.T) {
	stable := map[CheckID]bool{
		CheckSRPLargeType: true, CheckSRPDataClump: true, CheckOCPTypeDispatch: true,
		CheckISPFatInterface: true, CheckISPUsageRatio: true,
		CheckISPStubImplementation: true, CheckDIPConcreteDependency: true,
	}
	plugin := map[CheckID]bool{
		CheckSRPGodType: true, CheckSRPLowCohesionType: true, CheckSRPLargeType: true,
		CheckSRPHighFanOutType: true, CheckSRPComplexFunction: true,
		CheckSRPMixedInputSurface: true, CheckSRPDataClump: true,
		CheckSRPFlagArgument: true, CheckSRPMixedImportClusters: true,
		CheckLSPNonExactEOF: true, CheckISPFatInterface: true,
		CheckISPUsageRatio: true, CheckISPStubImplementation: true,
		CheckDIPConcreteDependency: true,
	}
	stableCount, pluginCount := 0, 0
	modeCounts := map[SyntaxSupport]int{}
	for _, check := range checkRegistry {
		if check.Maturity == MaturityStable {
			stableCount++
			if !stable[check.ID] {
				t.Errorf("unexpected stable check %s", check.ID)
			}
		}
		modeCounts[check.Syntax]++
		module := check.Surfaces.Supports(SurfaceModulePlugin)
		shared := check.Surfaces.Supports(SurfaceGoPlugin)
		if module != shared {
			t.Errorf("%s diverges between plugin surfaces", check.ID)
		}
		if module {
			pluginCount++
			if !plugin[check.ID] {
				t.Errorf("unexpected plugin check %s", check.ID)
			}
		}
	}
	if stableCount != 7 {
		t.Fatalf("stable checks = %d, want 7", stableCount)
	}
	if pluginCount != 14 {
		t.Fatalf("plugin checks = %d, want 14", pluginCount)
	}
	if modeCounts[SyntaxEquivalent] != 4 || modeCounts[SyntaxConservative] != 9 || modeCounts[SyntaxUnavailable] != 14 {
		t.Fatalf("syntax capability counts = %v, want equivalent=4 conservative=9 unavailable=14", modeCounts)
	}
}

func TestResolveCheckSelection(t *testing.T) {
	stable, err := ResolveCheckSelection(ProfileStable, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(SelectedCheckIDs(stable)); got != 7 {
		t.Fatalf("stable selection = %d checks, want 7", got)
	}
	selected, err := ResolveCheckSelection(ProfileStable, map[Rule]bool{RuleSRP: true}, []CheckID{CheckSRPComplexFunction}, []CheckID{CheckSRPLargeType})
	if err != nil {
		t.Fatal(err)
	}
	if selected[CheckSRPLargeType] || !selected[CheckSRPDataClump] || !selected[CheckSRPComplexFunction] || selected[CheckISPFatInterface] {
		t.Fatalf("unexpected effective selection: %v", SelectedCheckIDs(selected))
	}
	if _, err := ResolveCheckSelection(ProfileStable, nil, []CheckID{CheckSRPLargeType}, []CheckID{CheckSRPLargeType}); err == nil {
		t.Fatal("contradictory selection must fail")
	}
}

func TestCheckRegistryKnownIDs(t *testing.T) {
	if !IsKnownCheckID(string(CheckISPFatInterface)) {
		t.Fatal("registered check ID should be known")
	}
	if IsKnownCheckID("SOLID-X/not-real") {
		t.Fatal("unknown check ID should not be accepted")
	}
	if IsKnownCheckID("SOLID-S/design-smell") || IsKnownCheckID("SOLID-L/unsupported-contract") {
		t.Fatal("legacy check aliases must not be accepted")
	}
	if !IsKnownSeverityTarget(string(RuleISP)) {
		t.Fatal("rule family should be a valid severity target")
	}
}

func checkIDStrings(ids []CheckID) []string {
	values := make([]string, len(ids))
	for i, id := range ids {
		values[i] = string(id)
	}
	return values
}

func TestCheckRegistryRejectsOrphanDocumentation(t *testing.T) {
	docsRoot := filepath.Join("..", "..", "docs", "checks")
	registered := map[string]bool{}
	for _, check := range checkRegistry {
		registered[string(check.ID)] = true
	}
	err := filepath.Walk(docsRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		relative, err := filepath.Rel(docsRoot, path)
		if err != nil {
			return err
		}
		checkID := strings.TrimSuffix(filepath.ToSlash(relative), ".md")
		if !registered[checkID] {
			t.Errorf("documentation file %q has no registered check", checkID)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
