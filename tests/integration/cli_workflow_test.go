package integration_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ExtroNovosib/solidify/internal/baseline"
	"github.com/ExtroNovosib/solidify/internal/cli"
)

func TestBaselineUpdatePreservesAnnotationsAndPruneIsExplicit(t *testing.T) {
	root := integrationRepositoryRoot(t)
	path := filepath.Join(t.TempDir(), "baseline.json")
	build := cli.BuildInfo{Version: "dev", Commit: "test", BuildDate: "test"}
	initArgs := []string{
		"baseline", "init", "-baseline", path, "-baseline-reason", "reviewed integration compatibility debt",
		"-baseline-owner", "architecture", "-baseline-expires", "2027-01-01", "-cache=false",
		filepath.Join(root, "testdata", "violations"),
	}
	if code := runCLIQuietly(t, build, initArgs); code != 0 {
		t.Fatalf("baseline init exit = %d", code)
	}
	initial, err := baseline.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(initial.Entries) == 0 {
		t.Fatal("baseline init created no entries")
	}
	beforeDiff, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	diffArgs := []string{"baseline", "diff", "-baseline", path, "-cache=false", filepath.Join(root, "testdata", "clean")}
	if code := runCLIQuietly(t, build, diffArgs); code != 1 {
		t.Fatalf("baseline diff exit = %d, want 1", code)
	}
	afterDiff, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(beforeDiff) != string(afterDiff) {
		t.Fatal("baseline diff mutated the document")
	}
	updateArgs := []string{"baseline", "update", "-baseline", path, "-cache=false", filepath.Join(root, "testdata", "clean")}
	if code := runCLIQuietly(t, build, updateArgs); code != 0 {
		t.Fatalf("baseline update exit = %d", code)
	}
	updated, err := baseline.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Entries) != len(initial.Entries) {
		t.Fatalf("update pruned stale entries: before=%d after=%d", len(initial.Entries), len(updated.Entries))
	}
	for _, entry := range updated.Entries {
		if entry.Reason != "reviewed integration compatibility debt" || entry.Owner != "architecture" || entry.Expires != "2027-01-01" {
			t.Fatalf("update changed annotation: %+v", entry)
		}
	}
	pruneArgs := []string{"baseline", "prune", "-baseline", path, "-cache=false", filepath.Join(root, "testdata", "clean")}
	if code := runCLIQuietly(t, build, pruneArgs); code != 0 {
		t.Fatalf("baseline prune exit = %d", code)
	}
	pruned, err := baseline.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(pruned.Entries) != 0 {
		t.Fatalf("pruned entries = %d, want 0", len(pruned.Entries))
	}
}

func TestExpiredBaselineWorkflow(t *testing.T) {
	root := integrationRepositoryRoot(t)
	path := filepath.Join(t.TempDir(), "baseline.json")
	build := cli.BuildInfo{Version: "dev", Commit: "test", BuildDate: "test"}
	fixture := filepath.Join(root, "testdata", "violations")
	init := runCLICaptured(t, build, []string{
		"baseline", "init", "-baseline", path, "-baseline-reason", "reviewed time-bounded integration debt",
		"-baseline-expires", "2000-01-01", "-cache=false", fixture,
	})
	if init.code != 0 {
		t.Fatalf("baseline init = %+v", init)
	}
	warn := runCLICaptured(t, build, []string{
		"check", "-baseline", path, "-baseline-expired=warn", "-cache=false", "-fail=false", fixture,
	})
	if warn.code != 0 || !strings.Contains(warn.stderr, "expired entry") || !strings.Contains(warn.stdout, "SOLID-") {
		t.Fatalf("expired baseline warning workflow = %+v", warn)
	}
	errorPolicy := runCLICaptured(t, build, []string{
		"check", "-baseline", path, "-baseline-expired=error", "-cache=false", "-fail=false", fixture,
	})
	if errorPolicy.code != 1 || !strings.Contains(errorPolicy.stderr, "expired entry") {
		t.Fatalf("expired baseline error workflow = %+v", errorPolicy)
	}
}

type capturedCLIResult struct {
	code           int
	stdout, stderr string
}

func runCLICaptured(t *testing.T, build cli.BuildInfo, args []string) capturedCLIResult {
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
	code := cli.Run(args, build)
	os.Stdout, os.Stderr = oldStdout, oldStderr
	if err := stdout.Close(); err != nil {
		t.Fatal(err)
	}
	if err := stderr.Close(); err != nil {
		t.Fatal(err)
	}
	stdoutData, err := os.ReadFile(stdout.Name())
	if err != nil {
		t.Fatal(err)
	}
	stderrData, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatal(err)
	}
	return capturedCLIResult{code: code, stdout: string(stdoutData), stderr: string(stderrData)}
}

func runCLIQuietly(t *testing.T, build cli.BuildInfo, args []string) int {
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
	code := cli.Run(args, build)
	os.Stdout, os.Stderr = oldStdout, oldStderr
	_ = stdout.Close()
	_ = stderr.Close()
	return code
}

func integrationRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..")
}
