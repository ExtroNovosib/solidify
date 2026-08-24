package e2e_test

import (
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
	stdout, err := command.Output()
	result := processResult{stdout: string(stdout)}
	if exitError, ok := err.(*exec.ExitError); ok {
		result.exitCode = exitError.ExitCode()
		result.stderr = string(exitError.Stderr)
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
