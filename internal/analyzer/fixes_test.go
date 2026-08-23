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
