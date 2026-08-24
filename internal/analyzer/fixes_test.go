package analyzer

import (
	"go/token"
	"testing"
)

func TestIgnoreSuppressionFixTemplate(t *testing.T) {
	issue := Issue{Rule: RuleISP, Check: CheckISPFatInterface, Pos: token.Position{Filename: "a.go", Line: 4, Column: 1}}
	fix := IgnoreSuppressionFix(issue, "legacy API")
	if len(fix.Edits) != 1 {
		t.Fatalf("edits = %d", len(fix.Edits))
	}
	want := "//solidify:ignore SOLID-I/fat-interface legacy API\n"
	if fix.Edits[0].NewText != want {
		t.Fatalf("got %q, want %q", fix.Edits[0].NewText, want)
	}
}

func TestFindingsDoNotReceiveGenericSuppressionFix(t *testing.T) {
	packages, _, err := LoadWorkspace([]string{testdataDir(t, "violations")}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	issues := Run(packages, DefaultConfig(), allRulesEnabled())
	if len(issues) == 0 {
		t.Fatal("expected findings")
	}
	for _, issue := range issues {
		if len(issue.SuggestedFixes) != 0 {
			t.Fatalf("%s received an unowned generic fix: %+v", issue.ID(), issue.SuggestedFixes)
		}
		metadata, _ := CheckMetadata(issue.Check)
		if metadata.HasSafeFix {
			t.Fatalf("%s advertises a safe fix without owning one", issue.ID())
		}
	}
}
