package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ExtroNovosib/solidify/internal/analyzer"
)

func run(args []string) int {
	return Run(args, BuildInfo{Version: "dev", Commit: "test", BuildDate: "test"})
}

func TestMain(m *testing.M) {
	if err := os.Chdir(filepath.Join("..", "..")); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestBaselineMatchingChangedAndStale(t *testing.T) {
	issue := analyzer.Issue{Rule: analyzer.RuleISP, Check: analyzer.CheckISPFatInterface, Message: "fat", Evidence: "six", Subject: "p.Wide", Identity: "interface=Wide"}
	issue.Pos.Filename = "a.go"
	accepted := map[string]bool{issue.Fingerprint(): true, "stale": true}
	if got := filterBaseline([]analyzer.Issue{issue}, accepted); len(got) != 0 {
		t.Fatalf("matching finding was not accepted: %v", got)
	}
	changed := issue
	changed.Identity = "interface=Wider"
	if got := filterBaseline([]analyzer.Issue{changed}, accepted); len(got) != 1 {
		t.Fatalf("changed finding was accepted: %v", got)
	}
	if got := staleBaseline(accepted, []analyzer.Issue{issue}); len(got) != 1 || got[0] != "stale" {
		t.Fatalf("stale = %v", got)
	}
}

func TestPortableBaselineMatchesAcrossCheckoutDirectories(t *testing.T) {
	first := createPortableBaselineCheckout(t, filepath.Join(t.TempDir(), "checkout-a"))
	second := createPortableBaselineCheckout(t, filepath.Join(t.TempDir(), "checkout-b"))
	cfg := analyzer.DefaultConfig()
	cfg.CacheEnabled = false
	enabled := map[analyzer.Rule]bool{analyzer.RuleDIP: true}

	firstPackages, _, err := analyzer.LoadWorkspace([]string{first}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	firstIssues := analyzer.Run(firstPackages, cfg, enabled)
	if len(firstIssues) != 1 {
		t.Fatalf("checkout A findings = %v, want one", firstIssues)
	}
	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	if err := writeBaseline(baselinePath, firstIssues, "reviewed portability contract"); err != nil {
		t.Fatal(err)
	}
	accepted, version, err := readBaselineInfo(baselinePath)
	if err != nil {
		t.Fatal(err)
	}
	if version != 5 {
		t.Fatalf("baseline version = %d, want 5", version)
	}

	secondPackages, _, err := analyzer.LoadWorkspace([]string{second}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	secondIssues := analyzer.Run(secondPackages, cfg, enabled)
	if len(secondIssues) != 1 {
		t.Fatalf("checkout B findings = %v, want one", secondIssues)
	}
	if remaining := filterBaseline(secondIssues, accepted); len(remaining) != 0 {
		t.Fatalf("portable baseline did not suppress checkout B: %v", remaining)
	}
}

func createPortableBaselineCheckout(t *testing.T, dir string) string {
	t.Helper()
	for _, subdir := range []string{dir, filepath.Join(dir, "dep"), filepath.Join(dir, "service")} {
		if err := os.MkdirAll(subdir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"go.mod":         "module portable\n\ngo 1.22\n",
		"dep/dep.go":     "package dep\n\ntype Driver struct{}\n",
		"service/app.go": "package service\n\nimport \"portable/dep\"\n\ntype Service struct { driver *dep.Driver }\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestParseRules_All(t *testing.T) {
	enabled, err := parseRules("S,O,L,I,D")
	if err != nil {
		t.Fatalf("parseRules: %v", err)
	}
	for _, r := range analyzer.All {
		if !enabled[r] {
			t.Errorf("rule %s not enabled", r)
		}
	}
}

func TestParseRules_Subset(t *testing.T) {
	enabled, err := parseRules("S, I")
	if err != nil {
		t.Fatalf("parseRules: %v", err)
	}
	if !enabled[analyzer.RuleSRP] || !enabled[analyzer.RuleISP] {
		t.Fatal("expected S and I enabled")
	}
	if enabled[analyzer.RuleOCP] || enabled[analyzer.RuleLSP] || enabled[analyzer.RuleDIP] {
		t.Fatal("expected only S and I enabled")
	}
}

func TestParseRules_Unknown(t *testing.T) {
	_, err := parseRules("S,X")
	if err == nil {
		t.Fatal("expected error for unknown rule")
	}
}

func TestParseRules_Empty(t *testing.T) {
	_, err := parseRules(",,")
	if err == nil {
		t.Fatal("expected error for empty rules")
	}
}

func TestRun_CleanTestdata(t *testing.T) {
	if code := run([]string{"-fail=false", "testdata/clean"}); code != 0 {
		t.Fatalf("run exit code = %d, want 0", code)
	}
}

func TestRun_DotDotDotPath(t *testing.T) {
	if code := run([]string{"-analysis=types", "-fail=false", "testdata/clean/..."}); code != 0 {
		t.Fatalf("run ./... exit = %d", code)
	}
}

func TestRun_ViolationsTestdata(t *testing.T) {
	if code := run([]string{"-fail=false", "testdata/violations"}); code != 0 {
		t.Fatalf("run exit code = %d, want 0 with -fail=false", code)
	}
}

func TestRun_FailOnFindings(t *testing.T) {
	if code := run([]string{"testdata/violations"}); code != 1 {
		t.Fatalf("run exit code = %d, want 1", code)
	}
}

func TestRun_JSONFormat(t *testing.T) {
	if code := run([]string{"-format=json", "-fail=false", "testdata/clean"}); code != 0 {
		t.Fatalf("run exit code = %d, want 0", code)
	}
}

func TestRun_UnknownFormat(t *testing.T) {
	if code := run([]string{"-format=yaml", "-fail=false", "testdata/clean"}); code != 2 {
		t.Fatalf("run exit code = %d, want 2", code)
	}
}

func TestRun_UnknownAnalysis(t *testing.T) {
	if code := run([]string{"-analysis=semantic", "-fail=false", "testdata/clean"}); code != 2 {
		t.Fatalf("run exit code = %d, want 2", code)
	}
}

func TestRun_Version(t *testing.T) {
	if code := run([]string{"-version"}); code != 0 {
		t.Fatalf("run exit code = %d, want 0", code)
	}
}

func TestSingleGoFileUsesContainingPackageWithoutObsoleteTip(t *testing.T) {
	stderr := captureStderr(t, func() {
		code := run([]string{"-fail=false", "testdata/verdict/god_console/console.go"})
		if code != 0 {
			t.Fatalf("run exit code = %d, want 0", code)
		}
	})
	if strings.Contains(stderr, "scan the package directory") {
		t.Fatalf("stderr contains obsolete package-directory tip: %q", stderr)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	defer func() { os.Stderr = old }()
	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestPrintConfigExitsBeforeAnalysis(t *testing.T) {
	stdout := captureStdout(t, func() {
		if code := run([]string{"-print-config", "-fail=false", "testdata/clean"}); code != 0 {
			t.Fatalf("run exit code = %d, want 0", code)
		}
	})
	var output struct {
		SchemaVersion int                `json:"schemaVersion"`
		Profile       analyzer.Profile   `json:"profile"`
		EnabledChecks []analyzer.CheckID `json:"enabledChecks"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if output.SchemaVersion != 1 {
		t.Fatalf("schemaVersion = %d, want 1", output.SchemaVersion)
	}
	if output.Profile != analyzer.ProfileStable || len(output.EnabledChecks) != 7 {
		t.Fatalf("resolved profile/checks = %s / %v", output.Profile, output.EnabledChecks)
	}
}

func TestBaselineStalePolicies(t *testing.T) {
	dir := t.TempDir()
	baselinePath := filepath.Join(dir, "baseline.json")
	if err := os.WriteFile(baselinePath, []byte(`{"version":4,"fingerprints":["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"-baseline", baselinePath, "-baseline-stale=error", "-fail=false", "testdata/clean"}); code != 1 {
		t.Fatalf("baseline-stale=error exit = %d, want 1", code)
	}
	stderr := captureStderr(t, func() {
		if code := run([]string{"-baseline", baselinePath, "-baseline-stale=ignore", "-fail=false", "testdata/clean"}); code != 0 {
			t.Fatalf("baseline-stale=ignore exit = %d, want 0", code)
		}
	})
	if strings.Contains(stderr, "stale fingerprint") {
		t.Fatalf("baseline-stale=ignore should suppress stale notice: %q", stderr)
	}
}

func TestWriteBaselineRejectsIdentityCollision(t *testing.T) {
	issue := analyzer.Issue{Rule: analyzer.RuleISP, Check: analyzer.CheckISPFatInterface, Message: "fat", Evidence: "six", Subject: "p.Wide", Identity: "interface=Wide"}
	issue.Pos.Filename = "a.go"
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := writeBaseline(path, []analyzer.Issue{issue, issue}, "reviewed duplicate identity"); err == nil || !strings.Contains(err.Error(), "identity collision") {
		t.Fatalf("collision error = %v", err)
	}
}

func TestRun_DoesNotWriteArtifactsIntoScannedTree(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module scanned\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"-cache-dir", filepath.Join(root, "external-cache"), "-fail=false", root}); code != 0 {
		t.Fatalf("run exit code = %d, want 0", code)
	}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if strings.HasPrefix(base, ".solidlint") || strings.Contains(base, ".solidlint-cache") {
			t.Errorf("unexpected artifact under scanned tree: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
