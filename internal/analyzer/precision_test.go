package analyzer

import (
	"reflect"
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
