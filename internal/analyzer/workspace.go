package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

const syntaxAnalysisMode = "syntax"
const analysisModeAuto = "auto"

// LoadWorkspace loads the requested package universe once. Keeping one
// go/packages load is important: go/types identities are only comparable when
// they come from the same load graph, which is required by module-wide OCP
// correlation.
//
// mode is syntax, auto, or types. Syntax mode deliberately avoids type
// checking. Auto retains syntax findings when a package is ill-typed, while
// types returns an error for an incomplete target package.
func LoadWorkspace(paths []string, includeTests bool, mode string) ([]*packageFiles, []string, error) {
	return loadWorkspace(paths, includeTests, workspaceLoadRequirements(mode, ExecutionPlan{}))
}

// LoadWorkspaceForPlan loads exactly the package information required by a
// resolved CLI plan. It keeps the historical LoadWorkspace API available to
// callers that do not have a plan, while the CLI avoids type work when every
// selected check is syntax-equivalent.
func LoadWorkspaceForPlan(paths []string, includeTests bool, mode string, plan ExecutionPlan) ([]*packageFiles, []string, error) {
	return loadWorkspace(paths, includeTests, workspaceLoadRequirements(mode, plan))
}

type workspaceRequirements struct {
	mode           string
	needsTypeInfo  bool
	strictTypeInfo bool
}

func workspaceLoadRequirements(mode string, plan ExecutionPlan) workspaceRequirements {
	requirements := workspaceRequirements{mode: mode, strictTypeInfo: mode == "types"}
	if mode == syntaxAnalysisMode {
		return requirements
	}
	if mode == "types" || len(plan.selected) == 0 {
		requirements.needsTypeInfo = true
		return requirements
	}
	for id := range plan.selected {
		metadata, ok := CheckMetadata(id)
		if !ok || metadata.Syntax != SyntaxEquivalent {
			requirements.needsTypeInfo = true
			return requirements
		}
	}
	return requirements
}

func loadWorkspace(paths []string, includeTests bool, requirements workspaceRequirements) ([]*packageFiles, []string, error) {
	patterns, err := workspacePatterns(paths)
	if err != nil {
		return nil, nil, err
	}
	loaded, err := loadWorkspacePackages(paths, includeTests, requirements, patterns)
	if err != nil {
		if requirements.mode == analysisModeAuto && requirements.needsTypeInfo {
			pkgs, _, fallbackErr := loadWorkspace(paths, includeTests, workspaceLoadRequirements(syntaxAnalysisMode, ExecutionPlan{}))
			if fallbackErr == nil {
				return pkgs, []string{fmt.Sprintf("type resolution incomplete (%s); ran syntax-capable checks", compactLoadError(err))}, nil
			}
		}
		if requirements.strictTypeInfo {
			return nil, nil, fmt.Errorf("type analysis failed:\n%w", err)
		}
		return nil, nil, err
	}
	typeFailures, hardErr := collectWorkspaceLoadErrors(loaded, requirements)
	if hardErr != nil {
		if requirements.mode == analysisModeAuto && requirements.needsTypeInfo {
			pkgs, _, fallbackErr := loadWorkspace(paths, includeTests, workspaceLoadRequirements(syntaxAnalysisMode, ExecutionPlan{}))
			if fallbackErr == nil {
				return pkgs, []string{fmt.Sprintf("type resolution incomplete (%s); ran syntax-capable checks", compactLoadError(hardErr))}, nil
			}
		}
		if requirements.strictTypeInfo {
			return nil, nil, fmt.Errorf("type analysis failed:\n%w", hardErr)
		}
		return nil, nil, hardErr
	}
	selected := selectLoadedPackages(loaded, includeTests)
	root := canonicalWorkspaceRoot(loaded, workspaceDir(paths))
	pkgs := packageFilesFromLoaded(selected, root, requirements.needsTypeInfo)
	if requirements.mode == analysisModeAuto {
		sort.Strings(typeFailures)
		typeFailures = uniqueStrings(typeFailures)
	}
	return pkgs, typeFailures, nil
}

func loadWorkspacePackages(paths []string, includeTests bool, requirements workspaceRequirements, patterns []string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode:       workspacePackagesLoadModeForRequirements(includeTests, requirements),
		Tests:      includeTests,
		Dir:        workspaceDir(paths),
		BuildFlags: []string{"-e"},
	}
	return packages.Load(cfg, patterns...)
}

func compactLoadError(err error) string {
	if err == nil {
		return "unknown type-resolution failure"
	}
	message := strings.Join(strings.Fields(err.Error()), " ")
	if message == "" {
		return "unknown type-resolution failure"
	}
	return message
}

func workspacePackagesLoadMode(includeTests bool, mode string) packages.LoadMode {
	return workspacePackagesLoadModeForRequirements(includeTests, workspaceLoadRequirements(mode, ExecutionPlan{}))
}

func workspacePackagesLoadModeForRequirements(includeTests bool, requirements workspaceRequirements) packages.LoadMode {
	loadMode := packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
		packages.NeedSyntax | packages.NeedModule
	if requirements.needsTypeInfo {
		// NeedDeps would retain dependency syntax and TypesInfo bodies. The
		// initial package's go/types graph already exposes direct imported
		// package identities through pkg.Types.Imports().
		loadMode |= packages.NeedImports | packages.NeedTypes | packages.NeedTypesInfo
	}
	if includeTests {
		loadMode |= packages.NeedForTest
	}
	return loadMode
}

func collectWorkspaceLoadErrors(loaded []*packages.Package, requirements workspaceRequirements) ([]string, error) {
	var typeFailures []string
	for _, pkg := range loaded {
		var packageTypeFailures []string
		for _, loadErr := range pkg.Errors {
			if loadErr.Kind == packages.ListError || loadErr.Kind == packages.ParseError {
				return nil, fmt.Errorf("%s: %s", pkg.PkgPath, loadErr.Msg)
			}
			if loadErr.Kind == packages.TypeError {
				packageTypeFailures = append(packageTypeFailures, loadErr.Msg)
			}
		}
		if requirements.needsTypeInfo && (pkg.IllTyped || pkg.Types == nil || pkg.TypesInfo == nil) {
			packageTypeFailures = append(packageTypeFailures, "package or dependency is ill-typed")
		}
		if len(packageTypeFailures) > 0 {
			sort.Strings(packageTypeFailures)
			typeFailures = append(typeFailures, fmt.Sprintf("%s: type resolution incomplete (%s)", pkg.PkgPath, packageTypeFailures[0]))
		}
	}
	if requirements.strictTypeInfo && len(typeFailures) > 0 {
		sort.Strings(typeFailures)
		return nil, fmt.Errorf("type analysis failed:\n%s", strings.Join(uniqueStrings(typeFailures), "\n"))
	}
	return typeFailures, nil
}

func selectLoadedPackages(loaded []*packages.Package, includeTests bool) map[string]*packages.Package {
	selected := map[string]*packages.Package{}
	for _, pkg := range loaded {
		if pkg == nil || strings.HasSuffix(pkg.PkgPath, ".test") {
			continue
		}
		key := pkg.PkgPath + "\x00" + pkg.Name
		if current := selected[key]; current != nil {
			if includeTests && pkg.ForTest != "" && current.ForTest == "" {
				selected[key] = pkg
			}
			continue
		}
		if !includeTests && pkg.ForTest != "" {
			continue
		}
		selected[key] = pkg
	}
	return selected
}

func packageFilesFromLoaded(selected map[string]*packages.Package, root string, requestedTypeInfo bool) []*packageFiles {
	pkgs := make([]*packageFiles, 0, len(selected))
	for _, pkg := range selected {
		if len(pkg.Syntax) == 0 {
			continue
		}
		fset := pkg.Fset
		if fset == nil {
			fset = token.NewFileSet()
		}
		pf := &packageFiles{
			dir:          pkg.Dir,
			fset:         fset,
			files:        pkg.Syntax,
			info:         pkg.TypesInfo,
			typePkg:      pkg.Types,
			typeComplete: requestedTypeInfo && pkg.Types != nil && pkg.TypesInfo != nil && !pkg.IllTyped,
			pkgPath:      pkg.PkgPath,
			pkgName:      pkg.Name,
			modulePath:   modulePath(pkg),
			imports:      sortedImportPaths(pkg.Imports),
			typeImports:  packageTypeImportsFromLoaded(pkg),
			analysisRoot: root,
			generated:    map[*ast.File]bool{},
		}
		for _, file := range pf.files {
			pf.generated[file] = ast.IsGenerated(file)
		}
		pkgs = append(pkgs, pf)
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].pkgPath < pkgs[j].pkgPath })
	return pkgs
}

func packageTypeImportsFromLoaded(pkg *packages.Package) map[string]*types.Package {
	if pkg == nil {
		return map[string]*types.Package{}
	}
	imports := packageTypeImports(pkg.Imports)
	if pkg.Types == nil {
		return imports
	}
	for _, imported := range pkg.Types.Imports() {
		if imported != nil {
			imports[imported.Path()] = imported
		}
	}
	return imports
}

func workspaceDir(paths []string) string {
	for _, raw := range paths {
		path := strings.TrimSuffix(strings.TrimSpace(raw), "/...")
		if path == "" {
			path = "."
		}
		if info, err := os.Stat(path); err == nil {
			if !info.IsDir() {
				path = filepath.Dir(path)
			}
			if absolute, err := filepath.Abs(path); err == nil {
				for current := absolute; ; current = filepath.Dir(current) {
					if _, err := os.Stat(filepath.Join(current, "go.work")); err == nil {
						return current
					}
					if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
						return current
					}
					parent := filepath.Dir(current)
					if parent == current {
						break
					}
				}
			}
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

func canonicalWorkspaceRoot(loaded []*packages.Package, fallback string) string {
	roots := map[string]bool{}
	for _, pkg := range loaded {
		if pkg != nil && pkg.Module != nil && pkg.Module.Dir != "" {
			if absolute, err := filepath.Abs(pkg.Module.Dir); err == nil {
				roots[filepath.Clean(absolute)] = true
			}
		}
	}
	if len(roots) == 1 {
		for root := range roots {
			return root
		}
	}
	if absolute, err := filepath.Abs(fallback); err == nil {
		return filepath.Clean(absolute)
	}
	return filepath.Clean(fallback)
}

func canonicalRoot(pkgs []*packageFiles) string {
	for _, pkg := range pkgs {
		if pkg != nil && pkg.analysisRoot != "" {
			return pkg.analysisRoot
		}
	}
	return ""
}

// FilterExcludedFiles applies configured excludes before any package or
// program-level check computes metrics, correlations, or related locations.
func FilterExcludedFiles(pkgs []*packageFiles, patterns []string) []*packageFiles {
	if len(patterns) == 0 {
		return pkgs
	}
	out := pkgs[:0]
	filteredFacts := map[string]string{}
	for _, pkg := range pkgs {
		if pkg == nil {
			continue
		}
		included := make([]*ast.File, 0, len(pkg.files))
		for _, file := range pkg.files {
			filename := pkg.fset.Position(file.Pos()).Filename
			relative := PortablePath(pkg.analysisRoot, filename)
			if Excluded(relative, patterns) || Excluded(filename, patterns) {
				continue
			}
			included = append(included, file)
		}
		pkg.files = included
		pkg.imports = importsFromSyntax(pkg.files)
		filteredFacts[pkg.pkgPath] = sourceFactManifest(pkg)
		if len(pkg.files) > 0 {
			out = append(out, pkg)
		}
	}
	recomputeDependencyFacts(out, filteredFacts)
	rebuildFilteredTypeSnapshots(out)
	return out
}

// ApplyWorkspaceFilePolicy removes generated and configured-excluded files in
// one mutation, then recomputes dependency facts and typed snapshots once.
func ApplyWorkspaceFilePolicy(pkgs []*packageFiles, patterns []string) []*packageFiles {
	out := pkgs[:0]
	changed := false
	for _, pkg := range pkgs {
		if pkg == nil {
			continue
		}
		if !packageNeedsWorkspaceFilePolicy(pkg, patterns) {
			out = append(out, pkg)
			continue
		}
		changed = true
		included := make([]*ast.File, 0, len(pkg.files))
		for _, file := range pkg.files {
			filename := pkg.fset.Position(file.Pos()).Filename
			relative := PortablePath(pkg.analysisRoot, filename)
			if pkg.generated[file] || Excluded(relative, patterns) || Excluded(filename, patterns) {
				continue
			}
			included = append(included, file)
		}
		pkg.files = included
		pkg.imports = importsFromSyntax(pkg.files)
		if len(pkg.files) > 0 {
			out = append(out, pkg)
		}
	}
	if !changed {
		return pkgs
	}
	recomputeDependencyFacts(out, dependencyFactManifests(out))
	rebuildFilteredTypeSnapshots(out)
	return out
}

// packageNeedsWorkspaceFilePolicy reports whether generated-file removal or a
// configured exclusion will mutate a package. The common no-op path avoids
// source manifests and filtered type-snapshot rebuilding altogether.
func packageNeedsWorkspaceFilePolicy(pkg *packageFiles, patterns []string) bool {
	if pkg == nil {
		return false
	}
	for _, file := range pkg.files {
		if pkg.generated[file] {
			return true
		}
		if len(patterns) == 0 {
			continue
		}
		filename := pkg.fset.Position(file.Pos()).Filename
		relative := PortablePath(pkg.analysisRoot, filename)
		if Excluded(relative, patterns) || Excluded(filename, patterns) {
			return true
		}
	}
	return false
}

func dependencyFactManifests(pkgs []*packageFiles) map[string]string {
	needed := map[string]bool{}
	for _, pkg := range pkgs {
		if pkg == nil {
			continue
		}
		for _, importPath := range pkg.imports {
			needed[importPath] = true
		}
	}
	facts := make(map[string]string, len(needed))
	for _, pkg := range pkgs {
		if pkg != nil && needed[pkg.pkgPath] {
			facts[pkg.pkgPath] = sourceFactManifest(pkg)
		}
	}
	return facts
}

func recomputeDependencyFacts(pkgs []*packageFiles, filteredFacts map[string]string) {
	for _, pkg := range pkgs {
		var facts strings.Builder
		for _, importPath := range pkg.imports {
			if fact := filteredFacts[importPath]; fact != "" {
				facts.WriteString(importPath)
				facts.WriteByte(0)
				facts.WriteString(fact)
				facts.WriteByte(0)
			}
		}
		pkg.dependencyFacts = facts.String()
	}
}

// rebuildFilteredTypeSnapshots ensures that the AST, types.Info, and
// types.Package views describe exactly the same included files. Reusing the
// original go/packages type snapshot after filtering would leave declarations
// and method sets from excluded or generated files visible to typed checks.
func rebuildFilteredTypeSnapshots(pkgs []*packageFiles) {
	builder := newFilteredSnapshotBuilder(pkgs)
	for _, pkg := range pkgs {
		builder.rebuild(pkg)
	}
}

type filteredSnapshotBuilder struct {
	byPath map[string]*packageFiles
	built  map[string]*types.Package
	state  map[string]uint8
}

func newFilteredSnapshotBuilder(pkgs []*packageFiles) *filteredSnapshotBuilder {
	byPath := make(map[string]*packageFiles, len(pkgs))
	for _, pkg := range pkgs {
		if pkg != nil && pkg.pkgPath != "" {
			byPath[pkg.pkgPath] = pkg
		}
	}
	return &filteredSnapshotBuilder{
		byPath: byPath,
		built:  map[string]*types.Package{},
		state:  map[string]uint8{},
	}
}

func (b *filteredSnapshotBuilder) rebuild(pkg *packageFiles) *types.Package {
	if pkg == nil {
		return nil
	}
	if b.state[pkg.pkgPath] == 2 {
		return b.built[pkg.pkgPath]
	}
	if b.state[pkg.pkgPath] == 1 {
		pkg.typeComplete = false
		pkg.filteredTypeErr = "import cycle while rebuilding filtered snapshot"
		return pkg.typePkg
	}
	if pkg.info == nil || pkg.typePkg == nil || !pkg.typeComplete {
		pkg.info = nil
		pkg.typePkg = nil
		pkg.typeComplete = false
		return nil
	}
	b.state[pkg.pkgPath] = 1
	pkg.filteredRebuilds++
	importer := filteredPackageImporter{
		resolve: func(path string) *types.Package {
			if dependency := b.byPath[path]; dependency != nil {
				return b.rebuild(dependency)
			}
			return pkg.typeImports[path]
		},
	}
	info := &types.Info{
		Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{},
		Uses: map[*ast.Ident]types.Object{}, Implicits: map[ast.Node]types.Object{},
		Selections: map[*ast.SelectorExpr]*types.Selection{}, Scopes: map[ast.Node]*types.Scope{},
	}
	var firstErr error
	config := types.Config{
		Importer: importer,
		Error: func(err error) {
			if firstErr == nil {
				firstErr = err
			}
		},
	}
	checked, err := config.Check(pkg.pkgPath, pkg.fset, pkg.files, info)
	if firstErr == nil {
		firstErr = err
	}
	pkg.info = info
	pkg.typePkg = checked
	pkg.typeComplete = firstErr == nil && checked != nil
	pkg.filteredTypeErr = ""
	if firstErr != nil {
		pkg.filteredTypeErr = firstErr.Error()
	}
	b.built[pkg.pkgPath] = checked
	b.state[pkg.pkgPath] = 2
	return checked
}

type filteredPackageImporter struct {
	resolve func(path string) *types.Package
}

func (i filteredPackageImporter) Import(path string) (*types.Package, error) {
	if pkg := i.resolve(path); pkg != nil {
		return pkg, nil
	}
	return nil, fmt.Errorf("filtered type snapshot cannot import %q", path)
}

func (i filteredPackageImporter) ImportFrom(path, _ string, _ types.ImportMode) (*types.Package, error) {
	return i.Import(path)
}

func sourceFactManifest(pkg *packageFiles) string {
	if pkg == nil || pkg.fset == nil {
		return ""
	}
	files := append([]*ast.File(nil), pkg.files...)
	sort.Slice(files, func(i, j int) bool {
		return pkg.fset.Position(files[i].Pos()).Filename < pkg.fset.Position(files[j].Pos()).Filename
	})
	var facts strings.Builder
	for _, file := range files {
		filename := pkg.fset.Position(file.Pos()).Filename
		facts.WriteString(PortablePath(pkg.analysisRoot, filename))
		facts.WriteByte(0)
		if digest, err := fileContentDigest(filename); err == nil {
			facts.WriteString(digest)
		}
		facts.WriteByte(0)
	}
	return facts.String()
}

func importsFromSyntax(files []*ast.File) []string {
	seen := map[string]bool{}
	for _, file := range files {
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err == nil {
				seen[path] = true
			}
		}
	}
	imports := make([]string, 0, len(seen))
	for path := range seen {
		imports = append(imports, path)
	}
	sort.Strings(imports)
	return imports
}

func workspacePatterns(paths []string) ([]string, error) {
	if len(paths) == 0 {
		paths = []string{"."}
	}
	patterns := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			path = "."
		}
		if strings.HasSuffix(path, "/...") || strings.HasSuffix(path, string(filepath.Separator)+"...") {
			patterns = append(patterns, filepath.ToSlash(path))
			continue
		}
		if info, err := os.Stat(path); err == nil {
			if info.IsDir() {
				absolute, err := filepath.Abs(path)
				if err != nil {
					return nil, err
				}
				patterns = append(patterns, filepath.ToSlash(absolute)+"/...")
				continue
			}
			if strings.HasSuffix(path, ".go") {
				absolute, err := filepath.Abs(path)
				if err != nil {
					return nil, err
				}
				patterns = append(patterns, "file="+filepath.ToSlash(absolute))
				continue
			}
		}
		patterns = append(patterns, path)
	}
	return patterns, nil
}

func modulePath(pkg *packages.Package) string {
	if pkg != nil && pkg.Module != nil {
		return pkg.Module.Path
	}
	return ""
}

func sortedImportPaths(imports map[string]*packages.Package) []string {
	paths := make([]string, 0, len(imports))
	for path := range imports {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func packageTypeImports(imports map[string]*packages.Package) map[string]*types.Package {
	out := make(map[string]*types.Package, len(imports))
	for path, pkg := range imports {
		if pkg != nil && pkg.Types != nil {
			out[path] = pkg.Types
		}
	}
	return out
}

func uniqueStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	out := values[:0]
	for _, value := range values {
		if len(out) == 0 || out[len(out)-1] != value {
			out = append(out, value)
		}
	}
	return out
}
