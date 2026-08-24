package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomGolangCIModulePluginHonorsSelectedChecks(t *testing.T) {
	root := repositoryRoot(t)
	work := t.TempDir()
	binDir := filepath.Join(work, "bin")
	consumer := filepath.Join(work, "consumer")
	if err := os.MkdirAll(consumer, 0o755); err != nil {
		t.Fatal(err)
	}
	writeE2EFile(t, filepath.Join(work, ".custom-gcl.yml"), fmt.Sprintf(`version: v2.12.2
name: solidlint-golangci-e2e
destination: %s
plugins:
  - module: github.com/ExtroNovosib/solidify
    import: github.com/ExtroNovosib/solidify/plugin/solidlint
    path: %s
`, binDir, root))
	writeE2EFile(t, filepath.Join(consumer, "go.mod"), "module example.com/solidlint-e2e\n\ngo 1.25.0\n")
	writeE2EFile(t, filepath.Join(consumer, "fat.go"), `package consumer

type Wide interface {
	A()
	B()
	C()
	D()
	E()
	F()
	G()
	H()
	I()
}
`)
	writeE2EFile(t, filepath.Join(consumer, ".golangci.yml"), `version: "2"
linters:
  default: none
  enable: [solidlint]
  settings:
    custom:
      solidlint:
        type: module
        description: explainable package-scoped SOLID checks
        settings:
          enabled_rules: [I]
          enabled_checks: [SOLID-I/fat-interface]
`)

	custom := exec.Command("go", "run", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2", "custom", "-v")
	custom.Dir = work
	custom.Env = append(os.Environ(), "GOCACHE="+filepath.Join(work, "go-build"))
	if output, err := custom.CombinedOutput(); err != nil {
		t.Fatalf("build custom golangci-lint: %v\n%s", err, output)
	}

	command := exec.Command(filepath.Join(binDir, "solidlint-golangci-e2e"), "run", "./...")
	command.Dir = consumer
	command.Env = append(os.Environ(), "GOCACHE="+filepath.Join(work, "consumer-go-build"))
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("custom golangci-lint unexpectedly accepted violation fixture:\n%s", output)
	}
	if !strings.Contains(string(output), "SOLID-I/fat-interface") {
		t.Fatalf("custom golangci-lint did not report selected check:\n%s", output)
	}
	if strings.Contains(string(output), "SOLID-S/") || strings.Contains(string(output), "SOLID-O/") || strings.Contains(string(output), "SOLID-L/") || strings.Contains(string(output), "SOLID-D/") {
		t.Fatalf("custom golangci-lint ran checks outside enabled selection:\n%s", output)
	}
}

func writeE2EFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
