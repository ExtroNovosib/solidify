package cli

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"

	"github.com/ExtroNovosib/solidify/internal/analyzer"
)

func TestSARIFContainsRuleMetadataAndLocation(t *testing.T) {
	issue := analyzer.Issue{Rule: analyzer.RuleISP, Check: analyzer.CheckISPFatInterface, Severity: analyzer.SeverityWarning, Message: "fat interface", Evidence: "6 methods"}
	issue.Pos.Filename, issue.Pos.Line, issue.Pos.Column = "fixture.go", 4, 2
	issue.End.Filename, issue.End.Line, issue.End.Column = "fixture.go", 4, 20
	var output bytes.Buffer
	if err := writeSARIF(&output, []analyzer.Issue{issue}); err != nil {
		t.Fatal(err)
	}
	validateSARIFSchema(t, output.Bytes())
	var report struct {
		Version string `json:"version"`
		Runs    []struct {
			Tool struct {
				Driver struct {
					Rules []struct {
						ID      string `json:"id"`
						HelpURI string `json:"helpUri"`
					} `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
			Results []struct {
				RuleID    string `json:"ruleId"`
				Locations []struct {
					PhysicalLocation struct {
						Region struct {
							StartLine int `json:"startLine"`
							EndLine   int `json:"endLine"`
							EndColumn int `json:"endColumn"`
						} `json:"region"`
					} `json:"physicalLocation"`
				} `json:"locations"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Version != "2.1.0" || len(report.Runs) != 1 || len(report.Runs[0].Tool.Driver.Rules) != 1 || report.Runs[0].Tool.Driver.Rules[0].ID != "SOLID-I/fat-interface" {
		t.Fatalf("invalid SARIF metadata: %s", output.String())
	}
	if report.Runs[0].Tool.Driver.Rules[0].HelpURI == "" {
		t.Fatalf("missing helpUri: %s", output.String())
	}
	if len(report.Runs[0].Results) != 1 || len(report.Runs[0].Results[0].Locations) != 1 {
		t.Fatalf("missing result location: %s", output.String())
	}
	region := report.Runs[0].Results[0].Locations[0].PhysicalLocation.Region
	if region.EndLine != 4 || region.EndColumn != 20 {
		t.Fatalf("missing end region: %+v", region)
	}
}

func TestSARIFContainsOCPRuleMetadataAndRelatedLocation(t *testing.T) {
	issue := analyzer.Issue{Rule: analyzer.RuleOCP, Check: analyzer.CheckOCPTypeDispatch, Severity: analyzer.SeverityWarning, Message: "repeated dispatch", Evidence: "sites=2"}
	issue.Pos.Filename, issue.Pos.Line, issue.Pos.Column = "a.go", 10, 2
	issue.Related = []analyzer.RelatedLocation{{Pos: token.Position{Filename: "b.go", Line: 20, Column: 3}, Message: "same family"}}
	var output bytes.Buffer
	if err := writeSARIF(&output, []analyzer.Issue{issue}); err != nil {
		t.Fatal(err)
	}
	validateSARIFSchema(t, output.Bytes())
	if !bytes.Contains(output.Bytes(), []byte(string(analyzer.CheckOCPTypeDispatch))) || !bytes.Contains(output.Bytes(), []byte("same family")) {
		t.Fatalf("missing OCP SARIF metadata: %s", output.String())
	}
}

func TestSARIFDeclaresUriBaseForRepositoryArtifacts(t *testing.T) {
	pkgs, _, err := analyzer.LoadWorkspace([]string{"testdata/violations"}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	issues := analyzer.Run(pkgs, analyzer.DefaultConfig(), map[analyzer.Rule]bool{analyzer.RuleISP: true})
	if len(issues) == 0 {
		t.Fatal("expected ISP findings")
	}
	var output bytes.Buffer
	if err := writeSARIF(&output, issues); err != nil {
		t.Fatal(err)
	}
	validateSARIFSchema(t, output.Bytes())
	if !bytes.Contains(output.Bytes(), []byte(`"originalUriBaseIds"`)) || !bytes.Contains(output.Bytes(), []byte(`"uriBaseId":"ROOT"`)) {
		t.Fatalf("missing SARIF URI base mapping: %s", output.String())
	}
}

func TestSARIFSuggestedFixContainsArtifactChange(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", `package fixture

type Wide interface { A(); B(); C(); D(); E(); F() }
type implementation struct{}
func (implementation) A() { panic("unsupported") }
func (implementation) B() {}
func (implementation) C() {}
func (implementation) D() {}
func (implementation) E() {}
func (implementation) F() {}
`, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{},
		Uses: map[*ast.Ident]types.Object{}, Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	if _, err := (&types.Config{}).Check("fixture", fset, []*ast.File{file}, info); err != nil {
		t.Fatal(err)
	}
	issues := analyzer.SnapshotFromSyntax(fset, []*ast.File{file}, info, true).RunISP(analyzer.DefaultConfig())
	var issue *analyzer.Issue
	for i := range issues {
		if issues[i].Check == analyzer.CheckISPStubImplementation {
			issue = &issues[i]
			break
		}
	}
	if issue == nil {
		t.Fatalf("analyzer did not produce ISP stub finding: %v", issues)
	}
	var output bytes.Buffer
	if err := writeSARIF(&output, []analyzer.Issue{*issue}); err != nil {
		t.Fatal(err)
	}
	validateSARIFSchema(t, output.Bytes())
	var report map[string]any
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(report)
	if !bytes.Contains(data, []byte(`"artifactChanges"`)) || !bytes.Contains(data, []byte(`"replacements"`)) || !bytes.Contains(data, []byte(`"insertedContent"`)) {
		t.Fatalf("SARIF fix is missing replacement content: %s", output.String())
	}
}

func TestSARIFEmptyReportValidatesAgainstOfficialSchema(t *testing.T) {
	var output bytes.Buffer
	if err := writeSARIF(&output, nil); err != nil {
		t.Fatal(err)
	}
	validateSARIFSchema(t, output.Bytes())
}

func TestSARIFContainsGitHubPrimaryLocationLineHash(t *testing.T) {
	pkgs, _, err := analyzer.LoadWorkspace([]string{"testdata/violations"}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	issues := analyzer.Run(pkgs, analyzer.DefaultConfig(), map[analyzer.Rule]bool{analyzer.RuleISP: true})
	if len(issues) == 0 {
		t.Fatal("expected ISP findings")
	}
	var output bytes.Buffer
	if err := writeSARIF(&output, issues); err != nil {
		t.Fatal(err)
	}
	validateSARIFSchema(t, output.Bytes())
	var report struct {
		Runs []struct {
			Results []struct {
				PartialFingerprints map[string]string `json:"partialFingerprints"`
				Properties          map[string]any    `json:"properties"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Runs) != 1 || len(report.Runs[0].Results) == 0 {
		t.Fatalf("expected SARIF results: %s", output.String())
	}
	for index, result := range report.Runs[0].Results {
		hash, ok := result.PartialFingerprints["primaryLocationLineHash"]
		if !ok || hash == "" {
			t.Fatalf("result %d missing primaryLocationLineHash: %+v", index, result.PartialFingerprints)
		}
		if result.PartialFingerprints["solidlint/v4"] == "" {
			t.Fatalf("result %d missing solidlint/v4 fingerprint: %+v", index, result.PartialFingerprints)
		}
		for _, key := range []string{"subject", "identity", "evidence", "maturity", "metrics", "groups"} {
			if _, ok := result.Properties[key]; !ok {
				t.Fatalf("result %d missing property %q: %+v", index, key, result.Properties)
			}
		}
	}
}

func validateSARIFSchema(t *testing.T, data []byte) {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft4
	schema, err := compiler.Compile("schemas/sarif-schema-2.1.0.json")
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
