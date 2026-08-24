package report

import (
	"bytes"
	"encoding/json"
	"go/token"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"

	"github.com/ExtroNovosib/solidify/internal/analyzer"
)

func TestSARIFContainsRuleLocationRelatedAndBuildMetadata(t *testing.T) {
	issue := analyzer.Issue{
		Rule: analyzer.RuleOCP, Check: analyzer.CheckOCPTypeDispatch,
		Severity: analyzer.SeverityWarning, Message: "repeated dispatch", Evidence: "sites=2",
		Subject: "dispatch-family", Identity: "variants=A,B",
		Related: []analyzer.RelatedLocation{{Pos: token.Position{Filename: "b.go", Line: 20, Column: 3}, Message: "same family"}},
	}
	issue.Pos = token.Position{Filename: "a.go", Line: 10, Column: 2}
	issue.End = token.Position{Filename: "a.go", Line: 10, Column: 8}
	var output bytes.Buffer
	if err := EncodeSARIF(&output, []analyzer.Issue{issue}, SARIFMetadata{ToolName: "solidlint", ToolVersion: "v0.2.0-test"}); err != nil {
		t.Fatal(err)
	}
	validateSARIFSchema(t, output.Bytes())
	for _, expected := range [][]byte{[]byte(`"version":"v0.2.0-test"`), []byte(`"ruleId":"SOLID-O/type-dispatch"`), []byte(`"same family"`), []byte(`"endColumn":8`)} {
		if !bytes.Contains(output.Bytes(), expected) {
			t.Fatalf("SARIF missing %s: %s", expected, output.String())
		}
	}
}

func TestSARIFDeclaresURIBaseAndStableFingerprints(t *testing.T) {
	root := filepath.Join(repositoryRoot(t), "testdata", "violations")
	packages, _, err := analyzer.LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	issues := analyzer.Run(packages, analyzer.DefaultConfig(), map[analyzer.Rule]bool{analyzer.RuleISP: true})
	if len(issues) == 0 {
		t.Fatal("expected ISP findings")
	}
	var output bytes.Buffer
	if err := EncodeSARIF(&output, issues, SARIFMetadata{ToolName: "solidlint", ToolVersion: "dev"}); err != nil {
		t.Fatal(err)
	}
	validateSARIFSchema(t, output.Bytes())
	for _, expected := range [][]byte{[]byte(`"originalUriBaseIds"`), []byte(`"uriBaseId":"ROOT"`), []byte(`"primaryLocationLineHash"`), []byte(`"solidlint/v4"`)} {
		if !bytes.Contains(output.Bytes(), expected) {
			t.Fatalf("SARIF missing %s: %s", expected, output.String())
		}
	}
}

func TestSARIFPreservesExplicitSafeFix(t *testing.T) {
	issue := analyzer.Issue{
		Rule: analyzer.RuleISP, Check: analyzer.CheckISPStubImplementation,
		Severity: analyzer.SeverityWarning, Message: "stub", Evidence: "stub-method:method=A",
		Subject: "implementation.A", Identity: "method=A",
		SuggestedFixes: []analyzer.SuggestedFix{{Message: "replace body", Edits: []analyzer.TextEdit{{
			Filename: "fixture.go", Start: token.Position{Filename: "fixture.go", Line: 3, Column: 1},
			End: token.Position{Filename: "fixture.go", Line: 3, Column: 5}, NewText: "return",
		}}}},
	}
	issue.Pos = token.Position{Filename: "fixture.go", Line: 3, Column: 1}
	var output bytes.Buffer
	if err := EncodeSARIF(&output, []analyzer.Issue{issue}, SARIFMetadata{ToolName: "solidlint", ToolVersion: "dev"}); err != nil {
		t.Fatal(err)
	}
	validateSARIFSchema(t, output.Bytes())
	for _, expected := range [][]byte{[]byte(`"artifactChanges"`), []byte(`"replacements"`), []byte(`"insertedContent"`)} {
		if !bytes.Contains(output.Bytes(), expected) {
			t.Fatalf("SARIF missing safe fix metadata: %s", output.String())
		}
	}
}

func TestSARIFOmitUnownedSuppressionFixes(t *testing.T) {
	issue := analyzer.Issue{
		Rule: analyzer.RuleISP, Check: analyzer.CheckISPStubImplementation,
		Severity: analyzer.SeverityWarning, Message: "stub", Evidence: "stub-method:method=A",
		Subject: "implementation.A", Identity: "method=A",
	}
	issue.Pos = token.Position{Filename: "fixture.go", Line: 3, Column: 1}
	var output bytes.Buffer
	if err := EncodeSARIF(&output, []analyzer.Issue{issue}, SARIFMetadata{ToolName: "solidlint", ToolVersion: "dev"}); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(output.Bytes(), []byte(`"fixes"`)) || bytes.Contains(output.Bytes(), []byte("accepted for now")) {
		t.Fatalf("SARIF advertises an unowned suppression fix: %s", output.String())
	}
}

func TestSARIFEmptyReportValidatesAgainstOfficialSchema(t *testing.T) {
	var output bytes.Buffer
	if err := EncodeSARIF(&output, nil, SARIFMetadata{ToolName: "solidlint", ToolVersion: "dev"}); err != nil {
		t.Fatal(err)
	}
	validateSARIFSchema(t, output.Bytes())
}

func validateSARIFSchema(t *testing.T, data []byte) {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft4
	schema, err := compiler.Compile(filepath.Join(repositoryRoot(t), "schemas", "sarif-schema-2.1.0.json"))
	if err != nil {
		t.Fatalf("compile official SARIF schema: %v", err)
	}
	var document any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("decode SARIF document: %v", err)
	}
	if err := schema.Validate(document); err != nil {
		t.Fatalf("SARIF does not conform to official 2.1.0 schema: %v\n%s", err, data)
	}
}
