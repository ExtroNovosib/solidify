package e2e_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTunnelRuntimeContractProbes(t *testing.T) {
	root := repositoryRoot(t)
	cmd := exec.Command("go", "test", "./positive/case000", "./positive/case001", "./positive/case002", "./positive/case003", "./positive/case004", "./positive/case005", "./negative/case000", "./negative/case001", "./negative/case002", "./negative/case003", "./negative/case004", "./negative/case005", "-count=1")
	cmd.Dir = filepath.Join(root, "testdata/tunnel_calibration/corpus")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("runtime contract probes: %v\n%s", err, output)
	}
}

func TestTunnelCLIWorkflow(t *testing.T) {
	root := repositoryRoot(t)
	binary := buildCLI(t, root)
	fixture := filepath.Join(root, "testdata/tunnel_calibration/corpus/positive/case000")
	check := "SOLID-L/discarded-read"
	base := []string{"check", "-cache=false", "-format=json", "-fail=false", "-enable-checks=" + check}
	var typed string
	for _, mode := range []string{"syntax", "types", "auto"} {
		result := runCLI(t, binary, root, append(append([]string{}, base...), "-analysis="+mode, fixture)...)
		if result.exitCode != 0 {
			t.Fatalf("mode%s: %+v", mode, result)
		}
		if mode == "syntax" {
			if strings.Contains(result.stdout, check) {
				t.Fatal("typed check ran in syntax mode")
			}
			continue
		}
		if !strings.Contains(result.stdout, check) {
			t.Fatalf("mode%s lost contract: %s", mode, result.stdout)
		}
		if typed == "" {
			typed = result.stdout
		} else if result.stdout != typed {
			t.Fatal("auto and types report differ")
		}
	}
	var findings []struct {
		ID, File, Fingerprint, Identity   string
		SchemaVersion, FingerprintVersion int
	}
	if err := json.Unmarshal([]byte(typed), &findings); err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].SchemaVersion != 3 || findings[0].FingerprintVersion != 4 || len(findings[0].Fingerprint) != 64 || filepath.IsAbs(findings[0].File) || findings[0].Identity == "" {
		t.Fatalf("report contracts: %+v", findings)
	}
	stable := runCLI(t, binary, root, "check", "-cache=false", "-format=json", "-fail=false", fixture)
	if strings.Contains(stable.stdout, check) {
		t.Fatal("experimental check leaked into stable")
	}
	baseline := filepath.Join(t.TempDir(), "baseline.json")
	init := runCLI(t, binary, root, "baseline", "init", "-cache=false", "-enable-checks="+check, "-baseline="+baseline, "-baseline-reason=reviewed stream contract", "-analysis=types", fixture)
	if init.exitCode != 0 {
		t.Fatalf("baseline init: %+v", init)
	}
	filtered := runCLI(t, binary, root, append(append([]string{}, base...), "-baseline="+baseline, fixture)...)
	if filtered.exitCode != 0 || strings.Contains(filtered.stdout, check) {
		t.Fatalf("baseline filter: %+v", filtered)
	}
	sarif := runCLI(t, binary, root, "check", "-cache=false", "-format=sarif", "-fail=false", "-enable-checks="+check, fixture)
	if sarif.exitCode != 0 || !json.Valid([]byte(sarif.stdout)) || !strings.Contains(sarif.stdout, check) {
		t.Fatalf("SARIF: %+v", sarif)
	}
	work := t.TempDir()
	data, err := os.ReadFile(filepath.Join(fixture, "fixture.go"))
	if err != nil {
		t.Fatal(err)
	}
	source := strings.Replace(string(data), "func (c *Stream) Read", "//solidlint:ignore "+check+" reviewed framed API\nfunc (c *Stream) Read", 1)
	writeE2EFile(t, filepath.Join(work, "go.mod"), "module example.com/tunnel-suppression\n\ngo 1.25\n")
	writeE2EFile(t, filepath.Join(work, "stream.go"), source)
	suppressed := runCLI(t, binary, root, append(append([]string{}, base...), work)...)
	if suppressed.exitCode != 0 || strings.Contains(suppressed.stdout, check) {
		t.Fatalf("suppression: %+v", suppressed)
	}
}
