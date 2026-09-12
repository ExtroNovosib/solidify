package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ExtroNovosib/solidify/internal/analyzer"
	configpkg "github.com/ExtroNovosib/solidify/internal/config"
)

func TestLegacyAndExplicitCheckEquivalent(t *testing.T) {
	invalidConfig := filepath.Join(t.TempDir(), ".solidify.yml")
	if err := os.WriteFile(invalidConfig, []byte("thresholds:\n  max_methodz: 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := [][]string{
		{"-cache=false", "-fail=false", "testdata/clean"},
		{"-cache=false", "-fail=false", "testdata/violations"},
		{"-config", invalidConfig, "testdata/clean"},
		{"-version"},
		{"-cache=false", "-print-config", "testdata/clean"},
	}
	for _, args := range cases {
		name := strings.ReplaceAll(strings.Join(args, "_"), "/", "-")
		t.Run(name, func(t *testing.T) {
			legacy := captureInvocation(t, args)
			explicit := captureInvocation(t, append([]string{"check"}, args...))
			if legacy != explicit {
				t.Fatalf("legacy = %+v\nexplicit = %+v", legacy, explicit)
			}
		})
	}
}

func TestLegacyAndExplicitCheckBrokenPipeEquivalent(t *testing.T) {
	legacyCode, legacyStderr := captureBrokenPipeInvocation(t, []string{"-cache=false", "-format=json", "-fail=false", "testdata/clean"})
	explicitCode, explicitStderr := captureBrokenPipeInvocation(t, []string{"check", "-cache=false", "-format=json", "-fail=false", "testdata/clean"})
	if legacyCode != explicitCode || legacyStderr != explicitStderr || legacyCode != 0 {
		t.Fatalf("legacy=(%d,%q) explicit=(%d,%q)", legacyCode, legacyStderr, explicitCode, explicitStderr)
	}
}

func TestChecksListAndExplainUseRegistryOrder(t *testing.T) {
	list := captureInvocation(t, []string{"checks", "list", "-format=json"})
	if list.code != 0 || list.stderr != "" {
		t.Fatalf("checks list = %+v", list)
	}
	var descriptions []checkDescription
	if err := json.Unmarshal([]byte(list.stdout), &descriptions); err != nil {
		t.Fatal(err)
	}
	wantIDs := analyzer.RegisteredCheckIDs()
	gotIDs := make([]analyzer.CheckID, len(descriptions))
	for index, item := range descriptions {
		gotIDs[index] = item.ID
	}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("checks order = %v, want %v", gotIDs, wantIDs)
	}
	explain := captureInvocation(t, []string{"checks", "explain", string(analyzer.CheckISPFatInterface), "-format=json"})
	if explain.code != 0 || !strings.Contains(explain.stdout, analyzer.CheckDoc(analyzer.CheckISPFatInterface)) {
		t.Fatalf("checks explain = %+v", explain)
	}
}

func TestChecksExplainIncludesConfigurationAndRemediation(t *testing.T) {
	text := captureInvocation(t, []string{"checks", "explain", string(analyzer.CheckISPFatInterface)})
	if text.code != 0 || !strings.Contains(text.stdout, "thresholds.max_interface_methods") || !strings.Contains(text.stdout, "remediation: Split the interface") {
		t.Fatalf("text explain = %+v", text)
	}
	jsonResult := captureInvocation(t, []string{"checks", "explain", string(analyzer.CheckISPFatInterface), "-format=json"})
	if jsonResult.code != 0 {
		t.Fatalf("json explain = %+v", jsonResult)
	}
	var explanation checkDescription
	if err := json.Unmarshal([]byte(jsonResult.stdout), &explanation); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(explanation.Configuration, "thresholds.max_interface_methods") || explanation.Remediation == "" {
		t.Fatalf("explanation guidance = %+v", explanation)
	}
}

func TestHelpExitsSuccessfully(t *testing.T) {
	result := captureInvocation(t, []string{"--help"})
	if result.code != 0 || !strings.Contains(result.stderr, "Usage: solidlint") {
		t.Fatalf("help = %+v", result)
	}
}

func TestTopLevelHelpListsCommands(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"help"}} {
		result := captureInvocation(t, args)
		if result.code != 0 {
			t.Fatalf("help %v = %+v", args, result)
		}
		for _, required := range []string{"Usage: solidlint", "check", "checks", "config", "baseline", "stats", "solidlint checks explain"} {
			if !strings.Contains(result.stderr, required) {
				t.Fatalf("help %v omitted %q:\n%s", args, required, result.stderr)
			}
		}
	}
}

func TestChecksExplainExamples(t *testing.T) {
	representative := []analyzer.CheckID{
		analyzer.CheckSRPLargeType,
		analyzer.CheckOCPTypeDispatch,
		analyzer.CheckLSPNonExactEOF,
		analyzer.CheckISPFatInterface,
		analyzer.CheckDIPConcreteDependency,
	}
	for _, id := range representative {
		t.Run(string(id), func(t *testing.T) {
			text := captureInvocation(t, []string{"checks", "explain", string(id)})
			if text.code != 0 {
				t.Fatalf("text explain = %+v", text)
			}
			for _, required := range []string{"configuration:", "remediation:", "example before:", "example after:", "legitimate exception:"} {
				if !strings.Contains(text.stdout, required) {
					t.Fatalf("text explanation for %s omitted %q:\n%s", id, required, text.stdout)
				}
			}

			jsonResult := captureInvocation(t, []string{"checks", "explain", string(id), "-format=json"})
			if jsonResult.code != 0 {
				t.Fatalf("json explain = %+v", jsonResult)
			}
			var description checkDescription
			if err := json.Unmarshal([]byte(jsonResult.stdout), &description); err != nil {
				t.Fatal(err)
			}
			assertCompleteCheckGuidance(t, description)
		})
	}

	all := allCheckDescriptions()
	if len(all) != len(analyzer.RegisteredCheckIDs()) {
		t.Fatalf("description count = %d, want %d", len(all), len(analyzer.RegisteredCheckIDs()))
	}
	for index, description := range all {
		if description.ID != analyzer.RegisteredCheckIDs()[index] {
			t.Fatalf("description[%d] = %s, want %s", index, description.ID, analyzer.RegisteredCheckIDs()[index])
		}
		assertCompleteCheckGuidance(t, description)
	}
}

func assertCompleteCheckGuidance(t *testing.T, description checkDescription) {
	t.Helper()
	if len(description.Configuration) == 0 || description.Remediation == "" || description.ExampleBefore == "" || description.ExampleAfter == "" || description.Exception == "" {
		t.Fatalf("incomplete guidance for %s: %+v", description.ID, description)
	}
}

func TestStatsUsesExecutionPlanCounters(t *testing.T) {
	result := captureInvocation(t, []string{"stats", "-cache=false", "-format=json", "testdata/clean"})
	if result.code != 0 {
		t.Fatalf("stats = %+v", result)
	}
	var stats analyzer.ExecutionStats
	if err := json.Unmarshal([]byte(result.stdout), &stats); err != nil {
		t.Fatal(err)
	}
	if stats.PlanIdentity == "" || len(stats.SelectedChecks) != 7 || len(stats.Groups) == 0 {
		t.Fatalf("stats = %+v", stats)
	}
	for _, group := range stats.Groups {
		if group.CacheHits != 0 || group.CacheMisses != 0 {
			t.Fatalf("cache-disabled stats = %+v", stats.Groups)
		}
	}
}

func TestAnalysisCoverageStats(t *testing.T) {
	result := captureInvocation(t, []string{"stats", "-cache=false", "-format=json", "testdata/clean"})
	if result.code != 0 {
		t.Fatalf("stats = %+v", result)
	}
	var stats analyzer.ExecutionStats
	if err := json.Unmarshal([]byte(result.stdout), &stats); err != nil {
		t.Fatal(err)
	}
	if len(stats.Packages) == 0 {
		t.Fatalf("stats omitted package coverage: %+v", stats)
	}
	for _, pkg := range stats.Packages {
		if !pkg.TypeComplete {
			t.Fatalf("clean fixture is unexpectedly incomplete: %+v", stats.Packages)
		}
	}
}

func TestConfigCommandsShareGeneratedArtifacts(t *testing.T) {
	initialized := captureInvocation(t, []string{"config", "init"})
	if initialized.code != 0 {
		t.Fatalf("config init = %+v", initialized)
	}
	path := filepath.Join(t.TempDir(), ".solidify.yml")
	if err := os.WriteFile(path, []byte(initialized.stdout), 0o644); err != nil {
		t.Fatal(err)
	}
	validated := captureInvocation(t, []string{"config", "validate", path})
	if validated.code != 0 || !strings.Contains(validated.stdout, "valid") {
		t.Fatalf("config validate = %+v", validated)
	}
	schema := captureInvocation(t, []string{"config", "schema", "-format=json"})
	want, err := configpkg.SchemaJSON()
	if err != nil {
		t.Fatal(err)
	}
	if schema.code != 0 || schema.stdout != string(want) {
		t.Fatalf("config schema mismatch: %+v", schema)
	}
}

func TestLegacyWriteBaselineRequiresReasonAndMalformedBaselineIsUsageError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.json")
	withoutReason := captureInvocation(t, []string{"-cache=false", "-write-baseline", path, "-fail=false", "testdata/violations"})
	if withoutReason.code != 2 || !strings.Contains(withoutReason.stderr, "at least 12 characters") {
		t.Fatalf("legacy write without reason = %+v", withoutReason)
	}
	withReason := captureInvocation(t, []string{"-cache=false", "-write-baseline", path, "-baseline-reason", "reviewed legacy compatibility debt", "-fail=false", "testdata/violations"})
	if withReason.code != 0 {
		t.Fatalf("legacy write with reason = %+v", withReason)
	}
	malformed := filepath.Join(t.TempDir(), "malformed.json")
	if err := os.WriteFile(malformed, []byte(`{"version":5,"entries":[{"reason":"todo"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	result := captureInvocation(t, []string{"baseline", "diff", "-baseline", malformed, "-cache=false", "testdata/clean"})
	if result.code != 2 {
		t.Fatalf("malformed baseline exit = %+v", result)
	}
}

type invocationResult struct {
	code           int
	stdout, stderr string
}

func captureInvocation(t *testing.T, args []string) invocationResult {
	t.Helper()
	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	oldStdout, oldStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdout, stderr
	code := Run(args, BuildInfo{Version: "dev", Commit: "test", BuildDate: "test"})
	os.Stdout, os.Stderr = oldStdout, oldStderr
	if closeErr := stdout.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if closeErr := stderr.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	stdoutData, err := os.ReadFile(stdout.Name())
	if err != nil {
		t.Fatal(err)
	}
	stderrData, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatal(err)
	}
	return invocationResult{code: code, stdout: string(stdoutData), stderr: string(stderrData)}
}

func captureBrokenPipeInvocation(t *testing.T, args []string) (int, string) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr := reader.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	oldStdout, oldStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = writer, stderr
	code := Run(args, BuildInfo{Version: "dev", Commit: "test", BuildDate: "test"})
	os.Stdout, os.Stderr = oldStdout, oldStderr
	_ = writer.Close()
	_ = stderr.Close()
	data, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatal(err)
	}
	return code, string(data)
}
