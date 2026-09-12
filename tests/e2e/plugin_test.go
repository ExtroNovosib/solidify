package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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

func TestGoPluginGateContract(t *testing.T) {
	makefile, err := os.ReadFile(filepath.Join(repositoryRoot(t), "Makefile"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(makefile)
	start := strings.Index(content, "plugin-go-e2e: $(BUILD_DIR)")
	end := strings.Index(content, "\nplugin-go-e2e-contract:")
	if start < 0 || end < 0 || end <= start {
		t.Fatal("plugin-go-e2e recipe boundaries are missing")
	}
	recipe := content[start:end]
	for _, required := range []string{
		"CGO_ENABLED=1 $(GO) build $(GOFLAGS) -tags plugin -buildmode=plugin -o $(PLUGIN) ./cmd/solidlint-golangci",
		"CGO_ENABLED=1 GOBIN=$(CURDIR)/$(BUILD_DIR) $(GO) install $(GOFLAGS) github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2",
		"$(CURDIR)/$(PLUGIN_HOST) run -c $(CURDIR)/.golangci-go-plugin.yml ./...",
		"if [ \"$$status\" -ne 1 ]",
		"grep -q 'SOLID-I/fat-interface' $(PLUGIN_GO_E2E_LOG)",
	} {
		if !strings.Contains(recipe, required) {
			t.Fatalf("plugin-go-e2e recipe is missing required contract %q:\n%s", required, recipe)
		}
	}
	if strings.Contains(recipe, "! GOCACHE=") {
		t.Fatalf("plugin-go-e2e must inspect the host exit status instead of negating its command:\n%s", recipe)
	}
}

func TestCanonicalGateOwnsExpensivePluginBuildOnce(t *testing.T) {
	root := repositoryRoot(t)
	makefile, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(makefile)

	ordinary := makeTargetBlock(t, content, "test-e2e")
	if !strings.Contains(ordinary, "$(GO) test $(GOFLAGS) ./tests/e2e -list '^Test'") || !strings.Contains(ordinary, "E2E_ORDINARY_EXCLUDED_TESTS") {
		t.Fatalf("test-e2e must derive its complete ordinary-test set from go test -list:\n%s", ordinary)
	}
	if strings.Contains(makeTargetBlock(t, content, "test-race"), "./tests/e2e") || strings.Contains(makeTargetBlock(t, content, "test-race"), "-race ./...") {
		t.Fatalf("test-race must not duplicate subprocess/plugin E2E work:\n%s", makeTargetBlock(t, content, "test-race"))
	}
	if !strings.Contains(content, "RACE_PKG := ./internal/... ./plugin/... ./cmd/... ./tests/integration") {
		t.Fatalf("race package ownership must retain production and integration packages:\n%s", makeTargetBlock(t, content, "test-race"))
	}

	specialOwners := map[string]string{
		"TestCustomGolangCIModulePluginHonorsSelectedChecks": "plugin-module-e2e",
		"TestGoPluginGateContract":                           "plugin-go-e2e-contract",
		"TestCanonicalGateOwnsExpensivePluginBuildOnce":      "test-ownership-contract",
	}
	excluded := exclusionNames(t, makeVariableValue(t, content, "E2E_ORDINARY_EXCLUDED_TESTS"))
	if len(excluded) != len(specialOwners) {
		t.Fatalf("ordinary E2E exclusions = %v, want exactly the dedicated owners %v", sortedMapKeys(excluded), sortedMapKeys(specialOwners))
	}
	for name := range specialOwners {
		if !excluded[name] {
			t.Fatalf("dedicated E2E test %s is not excluded from ordinary ownership", name)
		}
	}
	listed := map[string]bool{}
	for _, name := range listedE2ETestNames(t) {
		listed[name] = true
		owner, special := specialOwners[name]
		if !special {
			if excluded[name] {
				t.Fatalf("ordinary E2E test %s is silently excluded from canonical ownership", name)
			}
			continue // The dynamic test-e2e recipe discovers every non-special Test* name.
		}
		if !strings.Contains(makeTargetBlock(t, content, owner), "Test"+strings.TrimPrefix(name, "Test")) {
			t.Fatalf("special E2E test %s is not owned by %s", name, owner)
		}
	}
	for name := range specialOwners {
		if !listed[name] {
			t.Fatalf("dedicated E2E test %s is not discoverable, so its canonical target would be a false green", name)
		}
	}

	checkDependencies := strings.Fields(strings.TrimPrefix(makeTargetLine(t, content, "check"), "check:"))
	for _, target := range []string{"test-e2e", "test-race", "plugin-module-e2e", "plugin-go-e2e-contract", "test-ownership-contract", "plugin-go-e2e"} {
		if countString(checkDependencies, target) != 1 {
			t.Fatalf("canonical check ownership for %s = %d, want exactly one: %v", target, countString(checkDependencies, target), checkDependencies)
		}
	}
}

func listedE2ETestNames(t *testing.T) []string {
	t.Helper()
	command := exec.Command("go", "test", "./tests/e2e", "-list", "^Test")
	command.Dir = repositoryRoot(t)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("list E2E tests: %v\n%s", err, output)
	}
	var names []string
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "Test") {
			names = append(names, line)
		}
	}
	if len(names) == 0 {
		t.Fatal("go test -list returned no E2E tests")
	}
	sort.Strings(names)
	return names
}

func makeTargetLine(t *testing.T, content, target string) string {
	t.Helper()
	prefix := target + ":"
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	t.Fatalf("Makefile target %q is missing", target)
	return ""
}

func makeTargetBlock(t *testing.T, content, target string) string {
	t.Helper()
	line := makeTargetLine(t, content, target)
	start := strings.Index(content, line)
	if start < 0 {
		t.Fatalf("Makefile target %q disappeared", target)
	}
	rest := content[start:]
	if end := strings.Index(rest, "\n\n"); end >= 0 {
		return rest[:end]
	}
	return rest
}

func makeVariableValue(t *testing.T, content, variable string) string {
	t.Helper()
	prefix := variable + " := "
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	t.Fatalf("Makefile variable %q is missing", variable)
	return ""
}

func exclusionNames(t *testing.T, value string) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	for _, name := range strings.Split(value, "|") {
		if name == "" || names[name] {
			t.Fatalf("invalid duplicate or empty ordinary E2E exclusion in %q", value)
		}
		names[name] = true
	}
	return names
}

func sortedMapKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func countString(values []string, want string) int {
	count := 0
	for _, value := range values {
		if value == want {
			count++
		}
	}
	return count
}

func writeE2EFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
