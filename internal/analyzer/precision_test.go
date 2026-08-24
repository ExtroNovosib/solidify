package analyzer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestPrecisionCorpus is the reproducible precision gate for the bundled
// positive and negative corpus. The fixtures are deliberately small and
// reviewable: violations covers both OCP dispatch patterns, embedded ISP
// interfaces, ISP usage-ratio and stub checks, constructor DIP dependencies, and the threshold-based SRP
// smells, while clean includes both threshold boundaries and a deliberately
// long but cohesive parameter list.
func TestPrecisionCorpus(t *testing.T) {
	positive, _, err := LoadWorkspace([]string{testdataDir(t, "violations")}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	negative, _, err := LoadWorkspace([]string{testdataDir(t, "clean")}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	found := Run(positive, DefaultConfig(), allRulesEnabled())
	wantCounts := map[CheckID]int{
		CheckSRPLargeType: 1, CheckSRPDataClump: 1, CheckOCPTypeDispatch: 2,
		CheckISPFatInterface: 2, CheckISPUsageRatio: 1,
		CheckISPStubImplementation: 2, CheckDIPConcreteDependency: 1,
	}
	gotCounts := map[CheckID]int{}
	for _, issue := range found {
		gotCounts[issue.Check]++
		if issue.Subject == "" || issue.Identity == "" {
			t.Fatalf("%s lacks stable subject identity: %+v", issue.Check, issue)
		}
	}
	if !reflect.DeepEqual(gotCounts, wantCounts) {
		t.Fatalf("positive corpus exact IDs = %v, want %v", gotCounts, wantCounts)
	}
	for check, count := range wantCounts {
		metadata, _ := CheckMetadata(check)
		independent := Run(positive, DefaultConfig(), map[Rule]bool{metadata.Rule: true})
		if got := len(issuesWithCheck(independent, check)); got != count {
			t.Fatalf("independent %s findings = %d, want %d", check, got, count)
		}
	}
	if clean := Run(negative, DefaultConfig(), allRulesEnabled()); len(clean) != 0 {
		t.Fatalf("negative corpus: got %d false positives: %v", len(clean), clean)
	}
}

type stableEvaluationManifest struct {
	Revision string                 `json:"revision"`
	Cases    []stableEvaluationCase `json:"cases"`
}

type stableEvaluationCase struct {
	Key           string              `json:"key"`
	CheckID       CheckID             `json:"checkId"`
	Positive      stableEvaluationRef `json:"positive"`
	Negative      stableEvaluationRef `json:"negative"`
	Rationale     string              `json:"rationale"`
	Documentation string              `json:"documentation"`
}

type stableEvaluationRef struct {
	Root    string `json:"root"`
	Path    string `json:"path"`
	Subject string `json:"subject"`
}

func TestStableEvaluationManifestCoverageAndVerdicts(t *testing.T) {
	repositoryRoot := filepath.Clean(testdataDir(t, ".."))
	manifestPath := filepath.Join(repositoryRoot, "testdata", "evaluation", "stable-v0.2.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest stableEvaluationManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Revision != "stable-v0.2-r1" {
		t.Fatalf("manifest revision = %q", manifest.Revision)
	}
	stable := map[CheckID]bool{}
	for _, id := range RegisteredCheckIDs() {
		metadata, _ := CheckMetadata(id)
		if metadata.Maturity == MaturityStable {
			stable[id] = true
		}
	}
	seen := map[CheckID]bool{}
	loaded := map[string][]Issue{}
	for _, testCase := range manifest.Cases {
		if seen[testCase.CheckID] {
			t.Fatalf("duplicate manifest check %s", testCase.CheckID)
		}
		seen[testCase.CheckID] = true
		if !stable[testCase.CheckID] {
			t.Fatalf("orphan/non-stable manifest check %s", testCase.CheckID)
		}
		if strings.TrimSpace(testCase.Key) == "" || strings.TrimSpace(testCase.Rationale) == "" || testCase.Positive.Path == "" || testCase.Positive.Subject == "" || testCase.Negative.Path == "" || testCase.Negative.Subject == "" {
			t.Fatalf("incomplete manifest case: %+v", testCase)
		}
		document, err := os.ReadFile(filepath.Join(repositoryRoot, testCase.Documentation))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(document), testCase.Key) {
			t.Fatalf("%s does not link evaluation case %s", testCase.Documentation, testCase.Key)
		}
		positive := loadManifestIssues(t, repositoryRoot, testCase.Positive.Root, loaded)
		matched := false
		for _, issue := range positive {
			if issue.Check == testCase.CheckID && issue.PortablePath() == testCase.Positive.Path && issue.Subject == testCase.Positive.Subject {
				matched = true
				break
			}
		}
		if !matched {
			t.Fatalf("positive case %s missing %s at %s subject %s: %v", testCase.Key, testCase.CheckID, testCase.Positive.Path, testCase.Positive.Subject, positive)
		}
		negative := loadManifestIssues(t, repositoryRoot, testCase.Negative.Root, loaded)
		if got := len(issuesWithCheck(negative, testCase.CheckID)); got != 0 {
			t.Fatalf("negative case %s emitted %d %s findings: %v", testCase.Key, got, testCase.CheckID, negative)
		}
	}
	if !reflect.DeepEqual(seen, stable) {
		t.Fatalf("manifest checks = %v, stable registry = %v", seen, stable)
	}
}

func loadManifestIssues(t *testing.T, repositoryRoot, relativeRoot string, loaded map[string][]Issue) []Issue {
	t.Helper()
	if issues, ok := loaded[relativeRoot]; ok {
		return issues
	}
	root := filepath.Join(repositoryRoot, filepath.FromSlash(relativeRoot))
	packages, _, err := LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.CacheEnabled = false
	issues := Run(packages, cfg, allRulesEnabled())
	loaded[relativeRoot] = issues
	return issues
}
