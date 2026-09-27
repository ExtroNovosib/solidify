package cli

import (
	"encoding/json"
	"fmt"
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

func TestChecksExplainExamplesAreCheckSpecific(t *testing.T) {
	registered := map[analyzer.CheckID]bool{}
	seenBefore := map[string]analyzer.CheckID{}
	for _, id := range analyzer.RegisteredCheckIDs() {
		registered[id] = true
		example, ok := checkExamples[id]
		if !ok {
			t.Errorf("%s has no checks explain example", id)
			continue
		}
		if example.before == "" || example.after == "" || example.exception == "" || example.before == example.after {
			t.Errorf("%s example is incomplete: %+v", id, example)
		}
		if other, dup := seenBefore[example.before]; dup {
			t.Errorf("%s and %s share the same example before text", other, id)
		}
		seenBefore[example.before] = id
		description := describeCheck(mustCheckMetadata(t, id))
		if description.ExampleBefore != example.before || description.ExampleAfter != example.after || description.Exception != example.exception {
			t.Errorf("%s explain output does not use its example", id)
		}
	}
	for id := range checkExamples {
		if !registered[id] {
			t.Errorf("example for unregistered check %s", id)
		}
	}
}

func mustCheckMetadata(t *testing.T, id analyzer.CheckID) analyzer.Check {
	t.Helper()
	metadata, ok := analyzer.CheckMetadata(id)
	if !ok {
		t.Fatalf("unknown check %s", id)
	}
	return metadata
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

func TestConfigFailLevelAffectsExitCode(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".solidify.yml")
	if err := os.WriteFile(path, []byte("fail_level: error\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	configured := captureInvocation(t, []string{"check", "-cache=false", "-config", path, "testdata/violations"})
	if configured.code != 0 {
		t.Fatalf("config fail_level=error exit = %d, want 0; stderr=%q", configured.code, configured.stderr)
	}
	if !strings.Contains(configured.stdout, "issue(s) found") || strings.Contains(configured.stdout, "\n0 issue(s) found") {
		t.Fatalf("violations fixture produced no findings:\n%s", configured.stdout)
	}
	overridden := captureInvocation(t, []string{"check", "-cache=false", "-config", path, "-fail-level=warning", "testdata/violations"})
	if overridden.code != 1 {
		t.Fatalf("-fail-level=warning exit = %d, want 1; stderr=%q", overridden.code, overridden.stderr)
	}
}

func TestTextOutputUsesWorkingDirectoryRelativePaths(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"testdata/violations", filepath.Join(cwd, "testdata", "violations")} {
		result := captureInvocation(t, []string{"check", "-cache=false", "-fail=false", target})
		if result.code != 0 {
			t.Fatalf("check %s = %+v", target, result)
		}
		lines := strings.Split(strings.TrimSpace(result.stdout), "\n")
		if len(lines) < 3 || !strings.HasPrefix(lines[0], filepath.Join("testdata", "violations")+string(filepath.Separator)) || strings.Contains(result.stdout, cwd) {
			t.Fatalf("text output for %s is not working-directory relative:\n%s", target, result.stdout)
		}
	}

	outside := t.TempDir()
	writeCLIModule(t, outside, "")
	result := captureInvocation(t, []string{"check", "-cache=false", "-fail=false", outside})
	if result.code != 0 || !strings.Contains(result.stdout, filepath.Join(outside, "service.go")+":") {
		t.Fatalf("path outside the working directory was not left absolute:\n%+v", result)
	}

	relativeJSON := captureInvocation(t, []string{"check", "-cache=false", "-fail=false", "-format=json", "testdata/violations"})
	absoluteJSON := captureInvocation(t, []string{"check", "-cache=false", "-fail=false", "-format=json", filepath.Join(cwd, "testdata", "violations")})
	if relativeJSON.code != 0 || relativeJSON.stdout != absoluteJSON.stdout || strings.Contains(relativeJSON.stdout, cwd) {
		t.Fatalf("JSON output depends on the target spelling or working directory")
	}
}

// writeCLIModule writes a small module whose Service type has a concrete
// cross-package dependency, plus an optional discovered configuration.
func writeCLIModule(t *testing.T, dir, config string) {
	t.Helper()
	files := map[string]string{
		"go.mod":        "module example.com/clifixture\n\ngo 1.25.0\n",
		"dep/dep.go":    "package dep\n\ntype Driver struct{}\n\nfunc (*Driver) Run() {}\n",
		"service.go":    "package clifixture\n\nimport \"example.com/clifixture/dep\"\n\ntype Service struct{ driver *dep.Driver }\n\nfunc NewService(driver *dep.Driver) *Service { return &Service{driver: driver} }\n",
		".solidify.yml": config,
	}
	if config == "" {
		delete(files, ".solidify.yml")
	}
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDiscoveredConfigNoticeNamesEffectiveProfile(t *testing.T) {
	dir := t.TempDir()
	writeCLIModule(t, dir, "profile: all\n")
	result := captureInvocation(t, []string{"check", "-cache=false", "-fail=false", dir})
	want := "solidlint: using config " + filepath.Join(dir, ".solidify.yml") + " (discovered from scan target); profile=all checks=" + fmt.Sprint(len(analyzer.RegisteredCheckIDs())) + "\n"
	if result.code != 0 || !strings.Contains(result.stderr, want) {
		t.Fatalf("discovered notice = %q, want %q", result.stderr, want)
	}
	stable := t.TempDir()
	writeCLIModule(t, stable, "fail_level: error\n")
	if result := captureInvocation(t, []string{"check", "-cache=false", "-fail=false", stable}); !strings.Contains(result.stderr, "; profile=stable checks=7\n") {
		t.Fatalf("stable discovered notice = %q", result.stderr)
	}
}

func TestQuietSuppressesInformationalNotices(t *testing.T) {
	dir := t.TempDir()
	writeCLIModule(t, dir, "profile: all\n")
	quiet := captureInvocation(t, []string{"check", "-quiet", "-cache=false", "-fail=false", dir})
	if quiet.code != 0 || strings.Contains(quiet.stderr, "using config") {
		t.Fatalf("-quiet kept the discovered-config notice: %+v", quiet)
	}
	if !strings.Contains(quiet.stdout, "SOLID-D/concrete-dependency") {
		t.Fatalf("-quiet changed findings:\n%s", quiet.stdout)
	}

	illTyped := t.TempDir()
	writeCLIModule(t, illTyped, "profile: all\n")
	if err := os.WriteFile(filepath.Join(illTyped, "broken.go"), []byte("package clifixture\n\nvar _ = missingIdentifier\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	warned := captureInvocation(t, []string{"check", "-quiet", "-analysis=auto", "-cache=false", "-fail=false", illTyped})
	if warned.code != 0 || strings.Contains(warned.stderr, "using config") || !strings.Contains(warned.stderr, "solidlint: warning: type resolution incomplete") {
		t.Fatalf("-quiet must keep analysis warnings and drop only the notice: %+v", warned)
	}
}

func TestThresholdFlagAppliesRegistryKeys(t *testing.T) {
	result := captureInvocation(t, []string{"check", "-threshold", "max_interface_methods=3", "-threshold=min_tcc_percent=40", "-print-config", "testdata/clean"})
	if result.code != 0 {
		t.Fatalf("-threshold -print-config = %+v", result)
	}
	var output struct {
		Config analyzer.Config `json:"config"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &output); err != nil {
		t.Fatal(err)
	}
	if output.Config.MaxInterfaceMethods != 3 || output.Config.MinTCCPercent != 40 {
		t.Fatalf("thresholds = max_interface_methods %d, min_tcc_percent %d", output.Config.MaxInterfaceMethods, output.Config.MinTCCPercent)
	}

	config := filepath.Join(t.TempDir(), ".solidify.yml")
	if err := os.WriteFile(config, []byte("thresholds:\n  max_interface_methods: 5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	overridden := captureInvocation(t, []string{"check", "-config", config, "-threshold", "max_interface_methods=4", "-print-config", "testdata/clean"})
	if overridden.code != 0 || !strings.Contains(overridden.stdout, `"MaxInterfaceMethods": 4`) {
		t.Fatalf("-threshold did not override the config file: %+v", overridden)
	}

	for _, value := range []string{"max_interface_methods", "=3", "max_interface_methods=three", "max_interfaces=3", "isp_usage_ratio_percent=101"} {
		if result := captureInvocation(t, []string{"check", "-threshold", value, "-print-config", "testdata/clean"}); result.code != 2 {
			t.Fatalf("-threshold %s exit = %d, want 2 (%s)", value, result.code, result.stderr)
		}
	}
}

func TestThresholdFlagRejectsConflictingLegacyFlag(t *testing.T) {
	result := captureInvocation(t, []string{"check", "-max-interface-methods=4", "-threshold", "max_interface_methods=3", "-print-config", "testdata/clean"})
	if result.code != 2 || !strings.Contains(result.stderr, "-max-interface-methods and -threshold max_interface_methods") {
		t.Fatalf("conflicting threshold flags = %+v, want exit 2", result)
	}
	distinct := captureInvocation(t, []string{"check", "-max-interface-methods=4", "-threshold", "max_methods=12", "-print-config", "testdata/clean"})
	if distinct.code != 0 {
		t.Fatalf("different keys must combine: %+v", distinct)
	}
}

func TestConfigValidateDefaultsToSolidlintFilename(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".solidlint.yml"), []byte("fail_level: error\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	if result := captureInvocation(t, []string{"config", "validate"}); result.code != 0 || result.stdout != ".solidlint.yml: valid\n" {
		t.Fatalf("config validate = %+v, want .solidlint.yml validated", result)
	}
	if err := os.WriteFile(filepath.Join(dir, ".solidify.yml"), []byte("fail_level: error\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	both := captureInvocation(t, []string{"config", "validate"})
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	namesDir := strings.Contains(both.stderr, dir) || strings.Contains(both.stderr, resolved)
	if both.code != 2 || !namesDir || !strings.Contains(both.stderr, "contains both .solidlint.yml and .solidify.yml") {
		t.Fatalf("config validate with both names = %+v, want exit 2 naming %s", both, dir)
	}
	writeCLIModule(t, dir, "")
	if check := captureInvocation(t, []string{"check", "-cache=false", "-fail=false", "."}); check.code != 2 || !strings.Contains(check.stderr, "contains both") {
		t.Fatalf("check with both config names = %+v, want exit 2", check)
	}
	if err := os.Remove(filepath.Join(dir, ".solidlint.yml")); err != nil {
		t.Fatal(err)
	}
	if result := captureInvocation(t, []string{"config", "validate"}); result.code != 0 || result.stdout != ".solidify.yml: valid\n" {
		t.Fatalf("config validate fallback = %+v, want .solidify.yml validated", result)
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
	code := Run(isolateCache(t, args), testBuild)
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
	code := Run(isolateCache(t, args), testBuild)
	os.Stdout, os.Stderr = oldStdout, oldStderr
	_ = writer.Close()
	_ = stderr.Close()
	data, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatal(err)
	}
	return code, string(data)
}
