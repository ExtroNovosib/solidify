package analyzer

import "fmt"

// IgnoreSuppressionFix returns a mechanical suggested fix that inserts a
// justified suppression comment on the line above a finding.
func IgnoreSuppressionFix(issue Issue, reason string) SuggestedFix {
	if reason == "" {
		reason = "document why this finding is accepted"
	}
	start := issue.Pos
	start.Column = 1
	return SuggestedFix{
		Message: fmt.Sprintf("add //solidify:ignore %s %s", issue.ID(), reason),
		Edits: []TextEdit{{
			Filename: issue.Pos.Filename,
			Start:    start,
			End:      start,
			NewText:  fmt.Sprintf("//solidify:ignore %s %s\n", issue.ID(), reason),
		}},
	}
}
