package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestWorkspacePackagesLoadModeAvoidsExportFileMetadata(t *testing.T) {
	typed := workspacePackagesLoadMode(false, "types")
	for _, unused := range []packages.LoadMode{packages.NeedDeps, packages.NeedExportFile, packages.NeedTypesSizes} {
		if typed&unused != 0 {
			t.Fatalf("typed load requests unused %v metadata: %v", unused, typed)
		}
	}
	for _, required := range []packages.LoadMode{packages.NeedImports, packages.NeedTypes, packages.NeedTypesInfo} {
		if typed&required == 0 {
			t.Fatalf("typed load is missing %v: %v", required, typed)
		}
	}

	syntax := workspacePackagesLoadMode(false, syntaxAnalysisMode)
	for _, unused := range []packages.LoadMode{packages.NeedImports, packages.NeedDeps, packages.NeedExportFile, packages.NeedTypes, packages.NeedTypesInfo, packages.NeedTypesSizes} {
		if syntax&unused != 0 {
			t.Fatalf("syntax load requests unused %v: %v", unused, syntax)
		}
	}
}

func TestRelativeRecursivePatternStaysInCallerDirectory(t *testing.T) {
	root := t.TempDir()
	initTempModule(t, root)
	for _, name := range []string{"a", "b"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
		writePolicyFixture(t, filepath.Join(root, name, name+".go"), "package "+name+"\n")
	}
	t.Chdir(filepath.Join(root, "a"))

	pkgs, _, err := LoadWorkspace([]string{"./..."}, false, syntaxAnalysisMode)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, pkg := range pkgs {
		paths = append(paths, pkg.pkgPath)
	}
	if strings.Join(paths, ",") != "tempmod/a" {
		t.Fatalf("./... from tempmod/a loaded %v, want only tempmod/a", paths)
	}
	patterns, err := workspacePatterns([]string{"example.com/mod/..."})
	if err != nil || len(patterns) != 1 || patterns[0] != "example.com/mod/..." {
		t.Fatalf("import-path pattern rewritten: %v, %v", patterns, err)
	}
}

func TestWorkspaceLoadRequirementsFollowSelectedChecks(t *testing.T) {
	only := func(id CheckID) ExecutionPlan {
		t.Helper()
		cfg := DefaultConfig()
		cfg.Profile = ProfileAll
		for _, candidate := range RegisteredCheckIDs() {
			if candidate != id {
				cfg.DisabledChecks = append(cfg.DisabledChecks, candidate)
			}
		}
		plan, err := NewExecutionPlan(cfg, nil, SurfaceCLI)
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}

	equivalent := workspaceLoadRequirements(analysisModeAuto, only(CheckISPFatInterface))
	if equivalent.needsTypeInfo {
		t.Fatal("syntax-equivalent auto selection unexpectedly requested type information")
	}
	if mode := workspacePackagesLoadModeForRequirements(false, equivalent); mode&(packages.NeedImports|packages.NeedTypes|packages.NeedTypesInfo|packages.NeedDeps) != 0 {
		t.Fatalf("syntax-equivalent auto load requested type/dependency fields: %v", mode)
	}

	conservative := workspaceLoadRequirements(analysisModeAuto, only(CheckSRPLargeType))
	if !conservative.needsTypeInfo {
		t.Fatal("conservative auto selection did not retain initial package type information")
	}
	mode := workspacePackagesLoadModeForRequirements(false, conservative)
	if mode&(packages.NeedTypes|packages.NeedTypesInfo) != packages.NeedTypes|packages.NeedTypesInfo {
		t.Fatalf("conservative auto load is missing initial type fields: %v", mode)
	}
	if mode&(packages.NeedDeps|packages.NeedExportFile|packages.NeedTypesSizes) != 0 {
		t.Fatalf("conservative auto load retained dependency body metadata: %v", mode)
	}
}

func TestTypedWorkspaceDoesNotLoadDependencySyntax(t *testing.T) {
	dir := t.TempDir()
	initTempModule(t, dir)
	depDir := filepath.Join(dir, "dep")
	consumerDir := filepath.Join(dir, "consumer")
	for _, path := range []string{depDir, consumerDir} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(depDir, "dep.go"), []byte("package dep\n\ntype API interface { Run() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(consumerDir, "consumer.go"), []byte("package consumer\n\nimport \"tempmod/dep\"\n\ntype Consumer struct { dependency dep.API }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	cfg.Profile = ProfileAll
	plan, err := NewExecutionPlan(cfg, nil, SurfaceCLI)
	if err != nil {
		t.Fatal(err)
	}
	requirements := workspaceLoadRequirements(analysisModeAuto, plan)
	if !requirements.needsTypeInfo {
		t.Fatal("all-check auto plan unexpectedly avoided initial type information")
	}
	patterns, err := workspacePatterns([]string{consumerDir})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := loadWorkspacePackages([]string{consumerDir}, false, requirements, patterns)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded package count = %d, want one", len(loaded))
	}
	initial := loaded[0]
	if initial.Types == nil || initial.TypesInfo == nil {
		t.Fatal("initial package lost required type information")
	}
	dependency := initial.Imports["tempmod/dep"]
	if dependency == nil {
		t.Fatal("initial package lost direct import metadata")
	}
	if len(dependency.Syntax) != 0 || dependency.TypesInfo != nil {
		t.Fatalf("dependency retained syntax/type-info body: syntax=%d typesInfo=%v", len(dependency.Syntax), dependency.TypesInfo != nil)
	}
	var importedTypeAvailable bool
	for _, imported := range initial.Types.Imports() {
		if imported != nil && imported.Path() == "tempmod/dep" {
			importedTypeAvailable = true
		}
	}
	if !importedTypeAvailable {
		t.Fatal("initial package type graph lost imported package identity")
	}

	packages, _, err := LoadWorkspaceForPlan([]string{consumerDir}, false, analysisModeAuto, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 1 || packages[0].typeImports["tempmod/dep"] == nil {
		t.Fatalf("workspace package lost imported type identity: %+v", packages)
	}
	if !strings.Contains(packages[0].typeImports["tempmod/dep"].Path(), "tempmod/dep") {
		t.Fatalf("unexpected imported type package: %q", packages[0].typeImports["tempmod/dep"].Path())
	}
}

func TestPackageNeedsWorkspaceFilePolicy(t *testing.T) {
	parse := func(name string) *packageFiles {
		t.Helper()
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, "package fixture", 0)
		if err != nil {
			t.Fatal(err)
		}
		return &packageFiles{fset: fset, files: []*ast.File{file}, analysisRoot: "/workspace", generated: map[*ast.File]bool{}}
	}

	clean := parse("/workspace/keep.go")
	if packageNeedsWorkspaceFilePolicy(clean, nil) {
		t.Fatal("clean package unexpectedly requires workspace file policy")
	}
	if packageNeedsWorkspaceFilePolicy(clean, []string{"excluded.go"}) {
		t.Fatal("non-matching exclusion unexpectedly requires workspace file policy")
	}

	excluded := parse("/workspace/excluded.go")
	if !packageNeedsWorkspaceFilePolicy(excluded, []string{"excluded.go"}) {
		t.Fatal("matching exclusion did not require workspace file policy")
	}

	generated := parse("/workspace/generated.go")
	generated.generated[generated.files[0]] = true
	if !packageNeedsWorkspaceFilePolicy(generated, nil) {
		t.Fatal("generated source did not require workspace file policy")
	}
}
