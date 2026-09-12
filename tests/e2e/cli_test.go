package e2e_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLegacyExplicitCheckProcessParity(t *testing.T) {
	root := repositoryRoot(t)
	binary := buildCLI(t, root)
	args := []string{"-cache=false", "-format=json", "-fail=false", filepath.Join(root, "testdata", "clean")}
	legacy := runCLI(t, binary, root, args...)
	explicit := runCLI(t, binary, root, append([]string{"check"}, args...)...)
	if legacy.exitCode != explicit.exitCode || legacy.stdout != explicit.stdout || legacy.stderr != explicit.stderr {
		t.Fatalf("legacy=%+v explicit=%+v", legacy, explicit)
	}
}

func TestAllProfileSelfScanHasNoCoordinatorComplexFunctionFindings(t *testing.T) {
	root := repositoryRoot(t)
	binary := buildCLI(t, root)
	result := runCLI(t, binary, root, "check", "-cache=false", "-profile=all", "-format=json", "-fail=false", filepath.Join(root, "internal", "cli"))
	if result.exitCode != 0 {
		t.Fatalf("solidlint exited %d: %s", result.exitCode, result.stderr)
	}
	if strings.Contains(result.stdout, `"id":"SOLID-S/complex-function"`) {
		t.Fatalf("CLI coordinator regressed into a complex function finding:\n%s", result.stdout)
	}
}

func TestE2EArtifactsStayOutsideScannedWorkspace(t *testing.T) {
	root := repositoryRoot(t)
	binary := buildCLI(t, root)
	relative, err := filepath.Rel(root, binary)
	if err != nil {
		t.Fatal(err)
	}
	if relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		t.Fatalf("E2E binary %q is inside scanned workspace %q", binary, root)
	}
	result := runCLI(t, binary, root, "check", "-cache=false", "-fail=false", filepath.Join(root, "testdata", "clean"))
	if result.exitCode != 0 {
		t.Fatalf("clean subprocess exited %d: %s", result.exitCode, result.stderr)
	}
}

func TestDocumentedCLIExamples(t *testing.T) {
	root := repositoryRoot(t)
	binary := buildCLI(t, root)
	tests := []struct {
		name string
		args []string
	}{
		{"checks-list", []string{"checks", "list", "-format=json"}},
		{"checks-explain", []string{"checks", "explain", "SOLID-I/fat-interface", "-format=json"}},
		{"config-schema", []string{"config", "schema", "-format=json"}},
		{"stats", []string{"stats", "-cache=false", "-format=json", filepath.Join(root, "testdata", "clean")}},
		{"single-go-file", []string{"check", "-cache=false", "-fail=false", filepath.Join(root, "internal", "cli", "run.go")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := runCLI(t, binary, root, test.args...)
			if result.exitCode != 0 {
				t.Fatalf("solidlint %v exited %d: %s", test.args, result.exitCode, result.stderr)
			}
			if strings.Contains(test.name, "json") || test.name == "checks-list" || test.name == "checks-explain" || test.name == "config-schema" || test.name == "stats" {
				var value any
				if err := json.Unmarshal([]byte(result.stdout), &value); err != nil {
					t.Fatalf("solidlint %v returned invalid JSON: %v\n%s", test.args, err, result.stdout)
				}
			}
		})
	}
}

func TestHelpAndChecksExplainProcess(t *testing.T) {
	root := repositoryRoot(t)
	binary := buildCLI(t, root)
	help := runCLI(t, binary, root, "--help")
	if help.exitCode != 0 || !strings.Contains(help.stderr, "Usage: solidlint") {
		t.Fatalf("help = %+v", help)
	}
	explain := runCLI(t, binary, root, "checks", "explain", "SOLID-I/fat-interface")
	if explain.exitCode != 0 || !strings.Contains(explain.stdout, "thresholds.max_interface_methods") || !strings.Contains(explain.stdout, "remediation:") {
		t.Fatalf("checks explain = %+v", explain)
	}
}

func TestTopLevelHelpProcess(t *testing.T) {
	root := repositoryRoot(t)
	binary := buildCLI(t, root)
	for _, args := range [][]string{{"--help"}, {"help"}} {
		result := runCLI(t, binary, root, args...)
		if result.exitCode != 0 {
			t.Fatalf("solidlint %v exited %d: %s", args, result.exitCode, result.stderr)
		}
		for _, required := range []string{"Usage: solidlint", "check", "checks", "config", "baseline", "stats"} {
			if !strings.Contains(result.stderr, required) {
				t.Fatalf("solidlint %v omitted %q:\n%s", args, required, result.stderr)
			}
		}
	}
}

func TestAnalysisModesHandleIllTypedSource(t *testing.T) {
	root := repositoryRoot(t)
	binary := buildCLI(t, root)
	fixture := t.TempDir()
	writeE2EFile(t, filepath.Join(fixture, "go.mod"), "module example.com/illtyped\n\ngo 1.25.0\n")
	writeE2EFile(t, filepath.Join(fixture, "illtyped.go"), `package illtyped

var _ = missingIdentifier

type Oversized struct{}

func (Oversized) One() {}
func (Oversized) Two() {}
func (Oversized) Three() {}
func (Oversized) Four() {}
func (Oversized) Five() {}
func (Oversized) Six() {}
func (Oversized) Seven() {}
func (Oversized) Eight() {}
func (Oversized) Nine() {}
func (Oversized) Ten() {}
func (Oversized) Eleven() {}
`)

	for _, mode := range []string{"syntax", "auto"} {
		t.Run(mode, func(t *testing.T) {
			result := runCLI(t, binary, root, "check", "-analysis="+mode, "-cache=false", "-fail=false", fixture)
			if result.exitCode != 0 {
				t.Fatalf("%s mode exited %d: %s", mode, result.exitCode, result.stderr)
			}
			if !strings.Contains(result.stdout, "SOLID-S/large-type") {
				t.Fatalf("%s mode lost syntax-capable finding:\n%s", mode, result.stdout)
			}
			if mode == "auto" && !strings.Contains(result.stderr, "type resolution incomplete") {
				t.Fatalf("auto mode did not report incomplete type coverage: %s", result.stderr)
			}
		})
	}

	types := runCLI(t, binary, root, "check", "-analysis=types", "-cache=false", "-fail=false", fixture)
	if types.exitCode != 2 || !strings.Contains(types.stderr, "type analysis failed") {
		t.Fatalf("types mode = %+v, want type-analysis exit 2", types)
	}

	statsResult := runCLI(t, binary, root, "stats", "-analysis=auto", "-cache=false", "-format=json", fixture)
	if statsResult.exitCode != 0 {
		t.Fatalf("auto stats exited %d: %s", statsResult.exitCode, statsResult.stderr)
	}
	var stats struct {
		Packages []struct {
			Package      string `json:"package"`
			TypeComplete bool   `json:"typeComplete"`
		} `json:"packages"`
	}
	if err := json.Unmarshal([]byte(statsResult.stdout), &stats); err != nil {
		t.Fatalf("auto stats JSON: %v\n%s", err, statsResult.stdout)
	}
	if len(stats.Packages) != 1 || stats.Packages[0].Package != "example.com/illtyped" || stats.Packages[0].TypeComplete {
		t.Fatalf("auto stats did not disclose incomplete package coverage: %+v", stats.Packages)
	}
}

type processResult struct {
	exitCode       int
	stdout, stderr string
}

func buildCLI(t *testing.T, root string) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "solidlint")
	command := exec.Command("go", "build", "-o", binary, "./cmd/solidlint")
	command.Dir = root
	command.Env = append(os.Environ(), "GOCACHE="+filepath.Join(t.TempDir(), "go-build"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	return binary
}

func runCLI(t *testing.T, binary, root string, args ...string) processResult {
	t.Helper()
	command := exec.Command(binary, args...)
	command.Dir = root
	var stderr bytes.Buffer
	command.Stderr = &stderr
	stdout, err := command.Output()
	result := processResult{stdout: string(stdout), stderr: stderr.String()}
	if exitError, ok := err.(*exec.ExitError); ok {
		result.exitCode = exitError.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return result
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..")
}
