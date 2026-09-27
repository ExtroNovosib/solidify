package e2e_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestCIWorkflowContract pins the repository side of the CI and release
// workflows: tool versions stay reproducible, redundant runs are cancelled,
// every job is time-bounded, and self-scan SARIF reaches code scanning.
func TestCIWorkflowContract(t *testing.T) {
	root := repositoryRoot(t)
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, ".github", "workflows", name))
		if err != nil {
			t.Fatal(err)
		}
		return strings.ReplaceAll(string(data), "\r\n", "\n")
	}
	ci, release := read("ci.yml"), read("release.yml")

	for name, workflow := range map[string]string{"ci.yml": ci, "release.yml": release} {
		if strings.Contains(workflow, "@latest") || regexp.MustCompile(`version:\s*latest\b`).MatchString(workflow) {
			t.Errorf("%s uses an unpinned latest version", name)
		}
		jobs := strings.Count(workflow, "runs-on:")
		if timeouts := strings.Count(workflow, "timeout-minutes:"); jobs == 0 || timeouts != jobs {
			t.Errorf("%s has %d timeout-minutes for %d jobs", name, timeouts, jobs)
		}
		if !regexp.MustCompile(`goreleaser/goreleaser-action@v7\n\s+with: \{[^}]*version: v2\.17\.1`).MatchString(workflow) {
			t.Errorf("%s does not pin goreleaser-action@v7 to GoReleaser v2.17.1", name)
		}
		if !strings.Contains(workflow, "concurrency:\n  group: ${{ github.workflow }}-${{ github.ref }}") {
			t.Errorf("%s has no per-ref concurrency group", name)
		}
	}

	for _, required := range []string{
		"on:\n  push:\n    branches: [main]\n  pull_request: {}",
		"  cancel-in-progress: true",
		"golangci/golangci-lint-action@v8\n        with: {version: v2.12.2, install-only: true}",
		"make check-fast GOLANGCI_LINT=golangci-lint",
		"golang.org/x/vuln/cmd/govulncheck@v1.7.0",
		"github/codeql-action/upload-sarif@v3",
		"-format=sarif",
	} {
		if !strings.Contains(ci, required) {
			t.Errorf("ci.yml is missing %q", required)
		}
	}
	sarifJob := ci[strings.Index(ci, "  sarif:"):]
	if next := strings.Index(sarifJob[len("  sarif:"):], "\n  release-snapshot:"); next >= 0 {
		sarifJob = sarifJob[:len("  sarif:")+next]
	}
	if !strings.Contains(sarifJob, "security-events: write") || !strings.Contains(sarifJob, "upload-sarif@v3") {
		t.Errorf("sarif job lacks security-events: write or the SARIF upload:\n%s", sarifJob)
	}
	if strings.Contains(release, "cancel-in-progress: true") {
		t.Error("release.yml must not cancel an in-progress release")
	}
}
