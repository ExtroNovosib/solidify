package report

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"

	"github.com/ExtroNovosib/solidify/internal/analyzer"
)

func TestEncodeEmptyJSON(t *testing.T) {
	data, err := EncodeJSON(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(data), []byte("[]")) {
		t.Fatalf("JSON = %s", data)
	}
}

func TestIssuesJSONValidatesAgainstSchema(t *testing.T) {
	root := filepath.Join(repositoryRoot(t), "testdata", "violations")
	packages, _, err := analyzer.LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	issues := analyzer.Run(packages, analyzer.DefaultConfig(), map[analyzer.Rule]bool{
		analyzer.RuleSRP: true, analyzer.RuleOCP: true, analyzer.RuleLSP: true,
		analyzer.RuleISP: true, analyzer.RuleDIP: true,
	})
	data, err := EncodeJSON(issues)
	if err != nil {
		t.Fatal(err)
	}
	validateJSONSchema(t, filepath.Join(repositoryRoot(t), "schemas", "solidlint-result-v3.schema.json"), data)
}

func TestEncodeJSONRejectsInvalidRange(t *testing.T) {
	issue := analyzer.Issue{Check: analyzer.CheckSRPLargeType, Rule: analyzer.RuleSRP, Severity: analyzer.SeverityWarning, Subject: "p.Service", Identity: "type=Service", Evidence: "large-type:type=Service", Message: "large"}
	issue.Pos.Filename, issue.Pos.Line, issue.Pos.Column = "service.go", 10, 2
	issue.End.Line, issue.End.Column = 9, 1
	if _, err := EncodeJSON([]analyzer.Issue{issue}); err == nil {
		t.Fatal("invalid source range was accepted")
	}
}

func TestIssuesJSONSchemaRejectsMalformedOwnedFields(t *testing.T) {
	schemaPath := filepath.Join(repositoryRoot(t), "schemas", "solidlint-result-v3.schema.json")
	for _, document := range []string{
		`[{"schemaVersion":3,"fingerprintVersion":4,"id":"SOLID-S/large-type","fingerprint":"bad"}]`,
		`[{"schemaVersion":3,"fingerprintVersion":4,"id":"SOLID-S/large-type","unknown":true}]`,
	} {
		compiler := jsonschema.NewCompiler()
		compiled, err := compiler.Compile(schemaPath)
		if err != nil {
			t.Fatal(err)
		}
		var value any
		if err := json.Unmarshal([]byte(document), &value); err != nil {
			t.Fatal(err)
		}
		if err := compiled.Validate(value); err == nil {
			t.Fatalf("schema accepted malformed document: %s", document)
		}
	}
}

func validateJSONSchema(t *testing.T, schemaPath string, data []byte) {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile(schemaPath)
	if err != nil {
		t.Fatalf("compile JSON schema: %v", err)
	}
	var document any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("decode JSON document: %v", err)
	}
	if err := schema.Validate(document); err != nil {
		t.Fatalf("JSON does not conform to %s: %v\n%s", schemaPath, err, data)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..")
}
