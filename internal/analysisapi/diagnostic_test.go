package analysisapi

import (
	"go/token"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"

	"github.com/ExtroNovosib/solidify/internal/analyzer"
)

func TestReportIssuesPreservesDiagnosticMetadata(t *testing.T) {
	fset := token.NewFileSet()
	file := fset.AddFile("service.go", -1, 80)
	file.SetLines([]int{0, 20, 40, 60})
	issue := analyzer.Issue{
		Rule: analyzer.RuleISP, Check: analyzer.CheckISPFatInterface,
		Severity: analyzer.SeverityWarning, Message: "wide", Evidence: "fat-interface:interface=Wide;methods=9;max=8",
		Pos:     token.Position{Filename: "service.go", Line: 2, Column: 2},
		End:     token.Position{Filename: "service.go", Line: 2, Column: 8},
		Related: []analyzer.RelatedLocation{{Pos: token.Position{Filename: "service.go", Line: 3, Column: 1}, Message: "consumer"}},
	}
	var diagnostics []analysis.Diagnostic
	pass := &analysis.Pass{Fset: fset, Report: func(diagnostic analysis.Diagnostic) { diagnostics = append(diagnostics, diagnostic) }}
	reportIssues(pass, []analyzer.Issue{issue})
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
	diagnostic := diagnostics[0]
	if diagnostic.Category != string(analyzer.CheckISPFatInterface) || diagnostic.URL != analyzer.CheckHelpURI(analyzer.CheckISPFatInterface) {
		t.Fatalf("category/url lost: %+v", diagnostic)
	}
	if diagnostic.End <= diagnostic.Pos || len(diagnostic.Related) != 1 || !strings.Contains(diagnostic.Message, "[warning]") || !strings.Contains(diagnostic.Message, issue.Evidence) {
		t.Fatalf("diagnostic metadata lost: %+v", diagnostic)
	}
}
