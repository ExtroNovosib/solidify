package integration_test

import (
	"os"
	"path/filepath"
	"runtime"
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
