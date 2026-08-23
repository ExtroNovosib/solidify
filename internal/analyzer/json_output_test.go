package analyzer

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

func schemaPath(t *testing.T, name string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "schemas", name)
}

func TestIssuesJSONValidatesAgainstSchema(t *testing.T) {
	root := testdataDir(t, "violations")
	pkgs, _, err := LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	issues := Run(pkgs, DefaultConfig(), allRulesEnabled())
	data, err := EncodeIssuesJSON(issues)
	if err != nil {
		t.Fatal(err)
	}
	validateJSONSchema(t, schemaPath(t, "solidlint-result-v3.schema.json"), data)
}

func TestIssuesJSONValidatesAgainstSchemaWhenEmpty(t *testing.T) {
	data, err := EncodeIssuesJSON(nil)
	if err != nil {
		t.Fatal(err)
	}
	validateJSONSchema(t, schemaPath(t, "solidlint-result-v3.schema.json"), data)
}

func TestIssuesJSONSchemaRejectsMalformedOwnedFields(t *testing.T) {
	schema := schemaPath(t, "solidlint-result-v3.schema.json")
	for _, document := range []string{
		`[{"schemaVersion":3,"fingerprintVersion":4,"id":"SOLID-S/large-type","fingerprint":"bad"}]`,
		`[{"schemaVersion":3,"fingerprintVersion":4,"id":"SOLID-S/large-type","unknown":true}]`,
	} {
		compiler := jsonschema.NewCompiler()
		compiled, err := compiler.Compile(schema)
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

func TestEncodeIssuesJSONRejectsInvalidRange(t *testing.T) {
	issue := Issue{Check: CheckSRPLargeType, Rule: RuleSRP, Severity: SeverityWarning, Subject: "p.Service", Identity: "type=Service", Evidence: "large-type:type=Service", Message: "large"}
	issue.Pos.Filename, issue.Pos.Line, issue.Pos.Column = "service.go", 10, 2
	issue.End.Line, issue.End.Column = 9, 1
	if _, err := EncodeIssuesJSON([]Issue{issue}); err == nil {
		t.Fatal("invalid source range was accepted")
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
