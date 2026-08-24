package analysisapi

import (
	"fmt"
	"go/token"

	"golang.org/x/tools/go/analysis"

	"github.com/ExtroNovosib/solidify/internal/analyzer"
)

func reportIssues(pass *analysis.Pass, issues []analyzer.Issue) {
	for _, issue := range issues {
		start := diagnosticPos(pass.Fset, issue.Pos)
		diagnostic := analysis.Diagnostic{
			Pos:            start,
			Category:       issue.ID(),
			URL:            analyzer.CheckHelpURI(issue.Check),
			Message:        fmt.Sprintf("[%s] [%s] %s; evidence: %s", issue.ID(), issue.Severity, issue.Message, issue.Evidence),
			SuggestedFixes: suggestedFixes(pass.Fset, issue),
		}
		if end := diagnosticPos(pass.Fset, issue.End); end != token.NoPos && end > start {
			diagnostic.End = end
		}
		for _, related := range issue.Related {
			pos := diagnosticPos(pass.Fset, related.Pos)
			if pos == token.NoPos {
				continue
			}
			diagnostic.Related = append(diagnostic.Related, analysis.RelatedInformation{Pos: pos, Message: related.Message})
		}
		pass.Report(diagnostic)
	}
}

func diagnosticPos(fset *token.FileSet, pos token.Position) token.Pos {
	if pos.Filename == "" || pos.Line <= 0 {
		return token.NoPos
	}
	var file *token.File
	fset.Iterate(func(candidate *token.File) bool {
		if candidate.Name() == pos.Filename {
			file = candidate
			return false
		}
		return true
	})
	if file == nil {
		return token.NoPos
	}
	column := pos.Column
	if column <= 0 {
		column = 1
	}
	return file.LineStart(pos.Line) + token.Pos(column-1)
}

func suggestedFixes(fset *token.FileSet, issue analyzer.Issue) []analysis.SuggestedFix {
	if len(issue.SuggestedFixes) == 0 {
		return nil
	}
	out := make([]analysis.SuggestedFix, 0, len(issue.SuggestedFixes))
	for _, fix := range issue.SuggestedFixes {
		var textEdits []analysis.TextEdit
		for _, edit := range fix.Edits {
			start := diagnosticPos(fset, edit.Start)
			end := diagnosticPos(fset, edit.End)
			if end == token.NoPos {
				end = start
			}
			textEdits = append(textEdits, analysis.TextEdit{Pos: start, End: end, NewText: []byte(edit.NewText)})
		}
		out = append(out, analysis.SuggestedFix{Message: fix.Message, TextEdits: textEdits})
	}
	return out
}

// DiagnosticMessage formats an issue for tests.
func DiagnosticMessage(issue analyzer.Issue) string {
	return fmt.Sprintf("[%s] %s", issue.ID(), issue.Message)
}
