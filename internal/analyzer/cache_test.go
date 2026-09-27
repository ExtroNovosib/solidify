package analyzer

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCacheDigestIOIsLazyAndCompact(t *testing.T) {
	root := testdataDir(t, "violations")
	pkgs, _, err := LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range pkgs {
		if pkg.dependencyFacts != "" {
			t.Fatalf("package %s eagerly retained dependency source/export data", pkg.pkgPath)
		}
	}
	cfg := DefaultConfig()
	cfg.CacheDir = filepath.Join(t.TempDir(), "cache")
	plan, err := NewExecutionPlan(cfg, allRulesEnabled(), SurfaceCLI)
	if err != nil {
		t.Fatal(err)
	}
	cache := newPackageCache(cfg.CacheDir, cfg, plan, workspacePackagePaths(pkgs))
	if reads := cache.sourceReads.Load(); reads != 0 {
		t.Fatalf("source reads before cache key demand = %d", reads)
	}
	for _, pkg := range pkgs {
		digest := cache.packageHash(pkg)
		if len(digest) != 64 || strings.Contains(digest, "package ") {
			t.Fatalf("package digest is not compact SHA-256: %q", digest)
		}
	}
	if reads := cache.sourceReads.Load(); reads == 0 {
		t.Fatal("cache key demand did not hash local sources")
	}
}

func TestProgramCacheWarmHitPreservesFindings(t *testing.T) {
	root := testdataDir(t, "violations")
	cfg := DefaultConfig()
	cfg.CacheDir = filepath.Join(t.TempDir(), "cache")
	enabled := map[Rule]bool{RuleOCP: true}
	plan, err := NewExecutionPlan(cfg, enabled, SurfaceCLI)
	if err != nil {
		t.Fatal(err)
	}
	pkgs, _, err := LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	cold, coldStats := RunPlan(pkgs, cfg, plan)
	pkgs, _, err = LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	warm, warmStats := RunPlan(pkgs, cfg, plan)
	if issueSignatures(cold) != issueSignatures(warm) {
		t.Fatalf("program cache changed findings\ncold=%v\nwarm=%v", cold, warm)
	}
	if len(coldStats.Groups) != 1 || coldStats.Groups[0].Executions != 1 || coldStats.Groups[0].CacheMisses != 1 {
		t.Fatalf("cold program stats = %+v", coldStats.Groups)
	}
	if len(warmStats.Groups) != 1 || warmStats.Groups[0].Executions != 0 || warmStats.Groups[0].CacheHits != 1 {
		t.Fatalf("warm program stats = %+v", warmStats.Groups)
	}
}

func TestPackageCacheCorruptEntryCountsAsMiss(t *testing.T) {
	dir := t.TempDir()
	initTempModule(t, dir)
	pkgDir := filepath.Join(dir, "service")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "service.go"), []byte(`package service

import "tempmod/other"

type Service struct { driver *other.Driver }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	otherDir := filepath.Join(dir, "other")
	if err := os.MkdirAll(otherDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherDir, "driver.go"), []byte("package other\n\ntype Driver struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	cfg.CacheDir = filepath.Join(dir, "cache")
	enabled := map[Rule]bool{RuleDIP: true}
	pkgs, _, err := LoadWorkspace([]string{pkgDir}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := NewExecutionPlan(cfg, enabled, SurfaceCLI)
	if err != nil {
		t.Fatal(err)
	}
	cache := newPackageCache(cfg.CacheDir, cfg, plan, workspacePackagePaths(pkgs))
	issues := Run(pkgs, cfg, enabled)
	if len(issues) != 1 {
		t.Fatalf("initial findings = %v, want one", issues)
	}

	cacheID := groupCacheID(plan.Groups()[0])
	path := cache.entryPath(pkgs[0], cacheID)
	if err := os.WriteFile(path, []byte(`{"version":"solidlint-cache-v6","hash":"truncated`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := cache.load(pkgs[0], cacheID); ok {
		t.Fatal("truncated cache entry should miss")
	}
	if cache.corrupt.Load() != 1 {
		t.Fatalf("corrupt count = %d, want 1", cache.corrupt.Load())
	}
}

func TestPackageCacheWarmHitPreservesAbsenceOfUnownedFixes(t *testing.T) {
	root := testdataDir(t, "violations")
	cfg := DefaultConfig()
	cfg.CacheDir = filepath.Join(t.TempDir(), "cache")
	enabled := map[Rule]bool{RuleISP: true}
	pkgs, _, err := LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	cold := Run(pkgs, cfg, enabled)
	for _, issue := range cold {
		if len(issue.SuggestedFixes) != 0 {
			t.Fatalf("cold finding has unowned fix: %+v", issue)
		}
	}

	pkgs, _, err = LoadWorkspace([]string{root}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	warm := Run(pkgs, cfg, enabled)
	for _, issue := range warm {
		if len(issue.SuggestedFixes) != 0 {
			t.Fatalf("warm finding has unowned fix: %+v", issue)
		}
	}
}

func TestPackageCacheKeyIgnoresExecutionOnlySettings(t *testing.T) {
	base := DefaultConfig()
	plan, err := NewExecutionPlan(base, allRulesEnabled(), SurfaceCLI)
	if err != nil {
		t.Fatal(err)
	}
	debug := base
	debug.CacheDiagnostics = true
	debug.CacheDir = t.TempDir()
	root := t.TempDir()
	if newPackageCache(root, base, plan, nil).config != newPackageCache(root, debug, plan, nil).config {
		t.Fatal("cache diagnostics or location changed the cache key")
	}
	changed := base
	changed.MaxInterfaceMethods++
	if newPackageCache(root, base, plan, nil).config == newPackageCache(root, changed, plan, nil).config {
		t.Fatal("a threshold change reused the cache key")
	}
}

func TestRun_CacheInvalidatesWhenGeneratedDeclarationsChange(t *testing.T) {
	dir := t.TempDir()
	initTempModule(t, dir)
	generated := filepath.Join(dir, "querier.go")
	writePolicyFixture(t, generated, "// Code generated by sqlc. DO NOT EDIT.\n\npackage tempmod\n\ntype Querier interface {\n\tA()\n\tB()\n\tC()\n\tD()\n}\n")
	writePolicyFixture(t, filepath.Join(dir, "service.go"), "package tempmod\n\ntype Service struct{ Store Querier }\n\nfunc (s *Service) Run() { s.Store.A() }\n")
	cfg := DefaultConfig()
	cfg.CacheDir = filepath.Join(t.TempDir(), "cache")
	usageRatioFindings := func() int {
		t.Helper()
		pkgs, _, err := LoadWorkspace([]string{dir}, false, "types")
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, issue := range Run(pkgs, cfg, map[Rule]bool{RuleISP: true}) {
			if issue.Check == CheckISPUsageRatio {
				count++
			}
		}
		return count
	}
	if got := usageRatioFindings(); got != 1 {
		t.Fatalf("usage-ratio findings = %d, want 1 for an interface declared in a generated file", got)
	}
	writePolicyFixture(t, generated, "// Code generated by sqlc. DO NOT EDIT.\n\npackage tempmod\n\ntype Querier interface {\n\tA()\n}\n")
	if got := usageRatioFindings(); got != 0 {
		t.Fatalf("usage-ratio findings = %d after the generated interface shrank; a stale cache entry was reused", got)
	}
}

// TestCacheKeyMaterialCoversPolicyFields changes every exported Config field
// in turn. Policy fields must change the cache key; run-only fields must not.
// A new Config field fails here until it is added to cacheKeyMaterial or,
// when it cannot affect findings, to cacheRunOnlyConfigFields.
func TestCacheKeyMaterialCoversPolicyFields(t *testing.T) {
	base := DefaultConfig()
	plan, err := NewExecutionPlan(base, allRulesEnabled(), SurfaceCLI)
	if err != nil {
		t.Fatal(err)
	}
	baseKey := newCacheKeyMaterial(base, plan, "build").digest()
	configType := reflect.TypeOf(base)
	for index := 0; index < configType.NumField(); index++ {
		field := configType.Field(index)
		if !field.IsExported() {
			continue
		}
		t.Run(field.Name, func(t *testing.T) {
			changed := DefaultConfig()
			value := reflect.ValueOf(&changed).Elem().Field(index)
			switch value.Kind() {
			case reflect.Int:
				value.SetInt(value.Int() + 1)
			case reflect.Bool:
				value.SetBool(!value.Bool())
			case reflect.String:
				value.SetString(value.String() + "-changed")
			case reflect.Slice:
				value.Set(reflect.Append(value, reflect.ValueOf("changed").Convert(value.Type().Elem())))
			case reflect.Invalid,
				reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
				reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
				reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128,
				reflect.Array, reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
				reflect.Pointer, reflect.Struct, reflect.UnsafePointer:
				t.Fatalf("field %s has unsupported kind %s; extend this test", field.Name, value.Kind())
			}
			keyChanged := newCacheKeyMaterial(changed, plan, "build").digest() != baseKey
			if cacheRunOnlyConfigFields[field.Name] {
				if keyChanged {
					t.Fatalf("run-only field %s changed the cache key", field.Name)
				}
				return
			}
			if !keyChanged {
				t.Fatalf("policy field %s does not affect the cache key; add it to cacheKeyMaterial or cacheRunOnlyConfigFields", field.Name)
			}
		})
	}
	if newCacheKeyMaterial(base, plan, "other-build").digest() == baseKey {
		t.Fatal("the analyzer build does not affect the cache key")
	}
}

// writeYAMLDependentModule writes a module that imports only the standard
// library and gopkg.in/yaml.v3, a dependency solidlint itself pins, so its
// source is in the module cache whenever this test binary builds.
func writeYAMLDependentModule(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"go.mod": "module tempmod\n\ngo 1.25.0\n\nrequire gopkg.in/yaml.v3 v3.0.1\n",
		"go.sum": "gopkg.in/check.v1 v0.0.0-20161208181325-20d25e280405/go.mod h1:Co6ibVJAznAaIkqp8huTwlJQCZ016jof/cbN4VW5Yz0=\n" +
			"gopkg.in/yaml.v3 v3.0.1 h1:fxVm/GzAzEWqLHuvctI91KS9hhNmmWOoWu0XTYJS7CA=\n" +
			"gopkg.in/yaml.v3 v3.0.1/go.mod h1:K4uyk7z7BCEPqu6E+C64Yfv1cQ7kz7rIZviUmN+EgEM=\n",
		"app/app.go": `package app

import (
	"net/http"

	"gopkg.in/yaml.v3"
)

func Handle(w http.ResponseWriter) {
	data, _ := yaml.Marshal(map[string]int{"status": http.StatusOK})
	_, _ = w.Write(data)
}
`,
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func loadTypedAppPackage(t *testing.T, root string) (*packageFiles, []*packageFiles) {
	t.Helper()
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOFLAGS", "-mod=readonly")
	pkgs, warnings, err := LoadWorkspace([]string{filepath.Join(root, "app")}, false, "types")
	if err != nil || len(warnings) != 0 {
		t.Fatalf("load = %v, warnings = %v", err, warnings)
	}
	for _, pkg := range pkgs {
		if pkg.pkgPath == "tempmod/app" {
			if !pkg.typeComplete || pkg.moduleGoMod == "" {
				t.Fatalf("app package complete=%t go.mod=%q", pkg.typeComplete, pkg.moduleGoMod)
			}
			return pkg, pkgs
		}
	}
	t.Fatalf("app package not loaded: %v", pkgs)
	return nil, nil
}

func TestDependencyKeySkipsAPIDigestForStdlibAndPinnedModules(t *testing.T) {
	root := t.TempDir()
	writeYAMLDependentModule(t, root)
	pkg, pkgs := loadTypedAppPackage(t, root)
	for _, path := range []string{"net/http", "gopkg.in/yaml.v3"} {
		if pkg.typeImports[path] == nil {
			t.Fatalf("typed load omitted import %s: %v", path, pkg.imports)
		}
	}
	cache := newPackageCache(filepath.Join(t.TempDir(), "cache"), DefaultConfig(), ExecutionPlan{}, workspacePackagePaths(pkgs))
	if digest := cache.packageHash(pkg); len(digest) != 64 {
		t.Fatalf("package hash = %q", digest)
	}
	if count := cache.apiCount.Load(); count != 0 {
		t.Fatalf("api digests = %d, want 0 for standard-library and go.sum-pinned imports", count)
	}
	if diagnostics := cache.diagnostics(); !strings.Contains(diagnostics, " api_digests=0 ") {
		t.Fatalf("cache diagnostics omit api_digests=0: %s", diagnostics)
	}
}

func TestDependencyKeyInvalidatesWhenModuleFilesChange(t *testing.T) {
	root := t.TempDir()
	writeYAMLDependentModule(t, root)
	pkg, pkgs := loadTypedAppPackage(t, root)
	hash := func() string {
		cache := newPackageCache(filepath.Join(t.TempDir(), "cache"), DefaultConfig(), ExecutionPlan{}, workspacePackagePaths(pkgs))
		return cache.packageHash(pkg)
	}
	before := hash()
	if again := hash(); again != before {
		t.Fatalf("unchanged module files changed the package hash: %s != %s", again, before)
	}
	goSum := filepath.Join(root, "go.sum")
	data, err := os.ReadFile(goSum)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(data), "fxVm/GzAzEWqLHuvctI91KS9hhNmmWOoWu0XTYJS7CA=", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", 1)
	if err := os.WriteFile(goSum, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
	if after := hash(); after == before {
		t.Fatal("editing go.sum left the package hash unchanged")
	}
}

func TestDependencyKeyFallsBackForLocalReplace(t *testing.T) {
	writeLocalDependencyModules := func(t *testing.T, root, appGoMod string) {
		t.Helper()
		files := map[string]string{
			"app/go.mod": appGoMod,
			"app/app.go": "package app\n\nimport (\n\t\"net/http\"\n\n\t\"example.com/ext\"\n)\n\ntype Service struct {\n\tclient *ext.Client\n\tserver *http.Server\n}\n",
			"ext/go.mod": "module example.com/ext\n\ngo 1.25.0\n",
			"ext/ext.go": "package ext\n\ntype Client struct{}\n\nfunc (*Client) Do() {}\n",
		}
		for name, content := range files {
			path := filepath.Join(root, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	apiDigests := func(t *testing.T, target string) int64 {
		t.Helper()
		t.Setenv("GOPROXY", "off")
		pkgs, warnings, err := LoadWorkspace([]string{target}, false, "types")
		if err != nil || len(warnings) != 0 || len(pkgs) != 1 || pkgs[0].typeImports["example.com/ext"] == nil {
			t.Fatalf("load = %v, warnings = %v, pkgs = %v", err, warnings, pkgs)
		}
		cache := newPackageCache(filepath.Join(t.TempDir(), "cache"), DefaultConfig(), ExecutionPlan{}, workspacePackagePaths(pkgs))
		cache.packageHash(pkgs[0])
		return cache.apiCount.Load()
	}

	t.Run("filesystem replace", func(t *testing.T) {
		root := t.TempDir()
		writeLocalDependencyModules(t, root, "module example.com/app\n\ngo 1.25.0\n\nrequire example.com/ext v0.0.0\n\nreplace example.com/ext => ../ext\n")
		if got := apiDigests(t, filepath.Join(root, "app")); got != 1 {
			t.Fatalf("api digests = %d, want 1 for the locally replaced import only", got)
		}
	})
	t.Run("go.work", func(t *testing.T) {
		root := t.TempDir()
		writeLocalDependencyModules(t, root, "module example.com/app\n\ngo 1.25.0\n\nrequire example.com/ext v0.0.0\n")
		if err := os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.25.0\n\nuse (\n\t./app\n\t./ext\n)\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("GOWORK", "")
		if got := apiDigests(t, filepath.Join(root, "app")); got != 1 {
			t.Fatalf("api digests = %d, want 1 for the go.work module import only", got)
		}
	})

	for _, tc := range []struct {
		goMod string
		want  bool
	}{
		{"module m\nreplace example.com/x => ./x\n", true},
		{"module m\nreplace example.com/x v1.2.3 => ../x // local fork\n", true},
		{"module m\nreplace (\n\texample.com/a => example.com/b v1.0.0\n\texample.com/x => /abs/x\n)\n", true},
		{"module m\nreplace example.com/x => C:\\\\src\\\\x\n", true},
		{"module m\nreplace example.com/x => example.com/y v1.0.0\n", false},
		{"module m\n// replace example.com/x => ./x\nrequire example.com/x v1.0.0\n", false},
	} {
		if got := goModHasFilesystemReplace([]byte(tc.goMod)); got != tc.want {
			t.Errorf("goModHasFilesystemReplace(%q) = %t, want %t", tc.goMod, got, tc.want)
		}
	}
}

func TestExecutableDigestMemoReusesMatchingStat(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "solidlint")
	memo := filepath.Join(dir, "cache", "exe-digest.json")
	if err := os.WriteFile(executable, []byte("first build"), 0o755); err != nil {
		t.Fatal(err)
	}
	first := memoizedFileDigest(memo, executable)
	want, err := fileContentDigest(executable)
	if err != nil || first != want {
		t.Fatalf("first digest = %q, want %q (%v)", first, want, err)
	}

	// Rewrite the memo with a sentinel digest: a matching path, size, and
	// modification time must reuse it instead of rehashing the executable.
	data, err := os.ReadFile(memo)
	if err != nil {
		t.Fatal(err)
	}
	sentinel := strings.Repeat("f", 64)
	if err := os.WriteFile(memo, []byte(strings.Replace(string(data), first, sentinel, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := memoizedFileDigest(memo, executable); got != sentinel {
		t.Fatalf("matching stat digest = %q, want memoized %q", got, sentinel)
	}

	if err := os.WriteFile(executable, []byte("second build!"), 0o755); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(executable, later, later); err != nil {
		t.Fatal(err)
	}
	second := memoizedFileDigest(memo, executable)
	if want, _ := fileContentDigest(executable); second != want || second == sentinel {
		t.Fatalf("changed executable digest = %q, want fresh %q", second, want)
	}
	if got := executableDigest(filepath.Join(dir, "run-cache")); len(got) != 64 {
		t.Fatalf("executable digest = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "run-cache", "exe-digest.json")); err != nil {
		t.Fatalf("executable digest memo was not written: %v", err)
	}
}
