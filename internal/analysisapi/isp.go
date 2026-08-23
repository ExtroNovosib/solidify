package analysisapi

import (
	"fmt"
	"go/token"

	"golang.org/x/tools/go/analysis"

	"github.com/ExtroNovosib/solidify/internal/analyzer"
)

// ISPAnalyzer wraps package-scoped ISP checks for go/analysis consumers.
var ISPAnalyzer = &analysis.Analyzer{
	Name:             "solidisp",
	Doc:              "reports fat interfaces and related ISP smells",
	Run:              runISP,
	RunDespiteErrors: true,
}

// DIPAnalyzer wraps package-scoped DIP checks for go/analysis consumers.
var DIPAnalyzer = &analysis.Analyzer{
	Name:             "soliddip",
	Doc:              "reports concrete dependencies at package boundaries",
	Run:              runDIP,
	RunDespiteErrors: true,
}

// Analyzers lists package-scoped analyzers exposed to golangci-lint modules.
var Analyzers = []*analysis.Analyzer{ISPAnalyzer, DIPAnalyzer}

func runISP(pass *analysis.Pass) (any, error) {
	reportIssues(pass, analyzer.SnapshotFromSyntax(pass.Fset, pass.Files, pass.TypesInfo, pass.TypesInfo != nil).RunISP(analyzer.DefaultConfig()))
	return nil, nil
}

func runDIP(pass *analysis.Pass) (any, error) {
	reportIssues(pass, analyzer.SnapshotFromSyntax(pass.Fset, pass.Files, pass.TypesInfo, pass.TypesInfo != nil).RunDIP(analyzer.DefaultConfig()))
	return nil, nil
}

func reportIssues(pass *analysis.Pass, issues []analyzer.Issue) {
	for _, issue := range issues {
		start := diagnosticPos(pass.Fset, issue.Pos)
		diag := analysis.Diagnostic{
			Pos:            start,
			Category:       issue.ID(),
			URL:            analyzer.CheckHelpURI(issue.Check),
			Message:        fmt.Sprintf("[%s] [%s] %s; evidence: %s", issue.ID(), issue.Severity, issue.Message, issue.Evidence),
			SuggestedFixes: suggestedFixes(pass.Fset, issue),
		}
		if end := diagnosticPos(pass.Fset, issue.End); end != token.NoPos && end > start {
			diag.End = end
		}
		for _, related := range issue.Related {
			pos := diagnosticPos(pass.Fset, related.Pos)
			if pos == token.NoPos {
				continue
			}
			diag.Related = append(diag.Related, analysis.RelatedInformation{Pos: pos, Message: related.Message})
		}
		pass.Report(diag)
	}
}

func diagnosticPos(fset *token.FileSet, pos token.Position) token.Pos {
	if pos.Filename == "" || pos.Line <= 0 {
		return token.NoPos
	}
	var file *token.File
	fset.Iterate(func(f *token.File) bool {
		if f.Name() == pos.Filename {
			file = f
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
