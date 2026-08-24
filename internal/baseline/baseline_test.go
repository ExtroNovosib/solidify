package baseline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"

	"github.com/ExtroNovosib/solidify/internal/analyzer"
)

func TestBaselineVersionFiveRoundTripAndDistinctIdentities(t *testing.T) {
	issues := baselineTestIssues()
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := Write(path, issues, "reviewed compatibility debt"); err != nil {
		t.Fatal(err)
	}
	document, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if document.Version != Version || len(document.Entries) != 2 {
		t.Fatalf("document = %+v", document)
	}
	for _, entry := range document.Entries {
		if entry.CheckID == "" || entry.Path == "" || entry.Subject == "" || entry.Reason != "reviewed compatibility debt" {
			t.Fatalf("incomplete annotated entry: %+v", entry)
		}
	}
	accepted, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(Filter(append([]analyzer.Issue(nil), issues...), accepted)) != 0 {
		t.Fatal("round-trip baseline did not accept findings")
	}
}

func TestBaselineReadsVersionFourCompatibilityDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.json")
	fingerprint := strings.Repeat("a", 64)
	if err := os.WriteFile(path, []byte(`{"version":4,"fingerprints":["`+fingerprint+`"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	document, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if document.Version != LegacyVersion || len(document.Entries) != 1 || document.Entries[0].Fingerprint != fingerprint {
		t.Fatalf("legacy document = %+v", document)
	}
}

func TestBaselineRejectsMalformedEntriesAndPlaceholderReasons(t *testing.T) {
	validFingerprint := strings.Repeat("a", 64)
	for _, body := range []string{
		`{"version":3,"fingerprints":[]}`,
		`{"version":4,"fingerprints":["ABC"]}`,
		`{"version":5,"entries":[{"fingerprint":"` + validFingerprint + `","checkId":"SOLID-I/fat-interface","path":"a.go","subject":"p.Wide","reason":"todo"}]}`,
		`{"version":5,"entries":[{"fingerprint":"` + validFingerprint + `","checkId":"SOLID-X/nope","path":"a.go","subject":"p.Wide","reason":"reviewed compatibility debt"}]}`,
	} {
		path := filepath.Join(t.TempDir(), "baseline.json")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatalf("Load(%s) unexpectedly succeeded", body)
		}
	}
}

func TestBaselineUpdatePreservesAnnotationsAndPruneIsExplicit(t *testing.T) {
	issues := baselineTestIssues()
	initial, _, err := Update(Document{Version: Version}, issues[:1], Annotation{
		Reason: "reviewed compatibility debt", Owner: "architecture", Expires: "2027-01-01",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	updated, diff, err := Update(initial, issues[1:], Annotation{Reason: "newly reviewed migration debt"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Added) != 1 || len(diff.Stale) != 1 || len(updated.Entries) != 2 {
		t.Fatalf("update diff=%+v document=%+v", diff, updated)
	}
	var stale Entry
	for _, entry := range updated.Entries {
		if entry.Fingerprint == initial.Entries[0].Fingerprint {
			stale = entry
		}
	}
	if stale.Owner != "architecture" || stale.Expires != "2027-01-01" || stale.Reason != "reviewed compatibility debt" {
		t.Fatalf("stale annotation changed: %+v", stale)
	}
	pruned, pruneDiff, err := Prune(updated, issues[1:])
	if err != nil {
		t.Fatal(err)
	}
	if len(pruneDiff.Stale) != 1 || len(pruned.Entries) != 1 {
		t.Fatalf("prune diff=%+v document=%+v", pruneDiff, pruned)
	}
}

func TestBaselineMutationUsesSameDirectoryAtomicReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "baseline.json")
	if err := Write(path, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".solidlint-baseline-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain: %v", matches)
	}
}

func TestBaselineVersionFiveMatchesSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := Write(path, baselineTestIssues(), "reviewed schema compatibility debt"); err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile(filepath.Join("..", "..", "schemas", "solidlint-baseline-v5.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(document); err != nil {
		t.Fatalf("baseline does not match schema: %v\n%s", err, data)
	}
}

func baselineTestIssues() []analyzer.Issue {
	issues := []analyzer.Issue{
		{Rule: analyzer.RuleSRP, Check: analyzer.CheckSRPDataClump, Severity: analyzer.SeverityWarning, Subject: "p.First", Identity: "parameters=a,b", Message: "clump", Evidence: "data-clump:first"},
		{Rule: analyzer.RuleSRP, Check: analyzer.CheckSRPDataClump, Severity: analyzer.SeverityWarning, Subject: "p.Second", Identity: "parameters=a,b", Message: "clump", Evidence: "data-clump:second"},
	}
	for index := range issues {
		issues[index].Pos.Filename = "p.go"
		issues[index].Pos.Line = index + 1
		issues[index].Pos.Column = 1
	}
	return issues
}
