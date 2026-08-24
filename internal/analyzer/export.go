package analyzer

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"

	"golang.org/x/tools/go/packages"
)

// PackageSnapshot exposes loaded package state to external analysis drivers.
type PackageSnapshot struct {
	pkg *packageFiles
}

// SnapshotInput contains the complete package context supplied by an analysis
// driver. Named fields prevent package identity and type completeness from
// being inferred differently by the CLI and plugin bridges.
type SnapshotInput struct {
	Fset        *token.FileSet
	Files       []*ast.File
	PackagePath string
	PackageName string
	ModulePath  string
	Types       *types.Package
	TypesInfo   *types.Info
	TypeErrors  []types.Error
	Imports     map[string]*types.Package
	Generated   map[*ast.File]bool
}

// SnapshotFromSyntax builds a package snapshot for go/analysis bridges.
func SnapshotFromSyntax(input SnapshotInput) *PackageSnapshot {
	generated := make(map[*ast.File]bool, len(input.Files))
	for _, file := range input.Files {
		value, ok := input.Generated[file]
		if !ok {
			value = ast.IsGenerated(file)
		}
		generated[file] = value
	}
	pkgName := input.PackageName
	if pkgName == "" && len(input.Files) > 0 {
		pkgName = input.Files[0].Name.Name
	}
	pkgPath := input.PackagePath
	if pkgPath == "" {
		pkgPath = pkgName
	}
	imports := make([]string, 0, len(input.Imports))
	typeImports := make(map[string]*types.Package, len(input.Imports))
	for path, imported := range input.Imports {
		imports = append(imports, path)
		if imported != nil {
			typeImports[path] = imported
		}
	}
	sort.Strings(imports)
	return &PackageSnapshot{pkg: &packageFiles{
		fset: input.Fset, files: input.Files, info: input.TypesInfo, typePkg: input.Types,
		typeComplete: input.Types != nil && input.TypesInfo != nil && len(input.TypeErrors) == 0,
		pkgPath:      pkgPath, pkgName: pkgName, modulePath: input.ModulePath,
		imports: imports, typeImports: typeImports, generated: generated,
	}}
}

// RunGroup executes one selected package runner group. The group's member
// selection is installed before the shared runner starts, so disabled members
// cannot perform check-specific work or emit diagnostics.
func (p *PackageSnapshot) RunGroup(group ExecutionGroup, cfg Config) []Issue {
	if p == nil || p.pkg == nil || group.Scope != ScopePackage {
		return nil
	}
	cfg.selectedChecks = make(map[CheckID]bool, len(group.Checks))
	for _, id := range group.Checks {
		cfg.selectedChecks[id] = true
	}
	runner, ok := runnerForGroup(group)
	if !ok || runner.RunPackage == nil {
		return nil
	}
	issues := filterGroupIssues(runner.RunPackage(p.pkg, cfg), group)
	_ = FinalizeIssues(issues, p.pkg.pkgPath)
	return issues
}

// RunISP executes package-scoped ISP checks on the snapshot.
func (p *PackageSnapshot) RunISP(cfg Config) []Issue {
	if p == nil || p.pkg == nil {
		return nil
	}
	issues := CheckISPWithTypes(p.pkg.fset, p.pkg.files, p.pkg.info, cfg, p.pkg)
	_ = FinalizeIssues(issues, p.pkg.pkgPath)
	return issues
}

// RunSRP executes all nine package-scoped SRP checks on the snapshot.
func (p *PackageSnapshot) RunSRP(cfg Config) []Issue {
	if p == nil || p.pkg == nil {
		return nil
	}
	issues := runSRPCheck(p.pkg, cfg)
	_ = FinalizeIssues(issues, p.pkg.pkgPath)
	return issues
}

// RunLSP executes the package-scoped non-exact-EOF check on the snapshot.
func (p *PackageSnapshot) RunLSP(cfg Config) []Issue {
	if p == nil || p.pkg == nil {
		return nil
	}
	issues := runLSPPackageCheck(p.pkg, cfg)
	_ = FinalizeIssues(issues, p.pkg.pkgPath)
	return issues
}

// RunDIP executes package-scoped DIP checks on the snapshot.
func (p *PackageSnapshot) RunDIP(cfg Config) []Issue {
	if p == nil || p.pkg == nil {
		return nil
	}
	issues := CheckDIPWithTypes(p.pkg.fset, p.pkg.files, p.pkg.info, cfg, p.pkg)
	_ = FinalizeIssues(issues, p.pkg.pkgPath)
	return issues
}

// SnapshotFromPackages converts a go/packages entry into a snapshot.
func SnapshotFromPackages(pkg *packages.Package) *PackageSnapshot {
	if pkg == nil || len(pkg.Syntax) == 0 {
		return nil
	}
	fset := pkg.Fset
	if fset == nil {
		fset = token.NewFileSet()
	}
	generated := map[*ast.File]bool{}
	for _, file := range pkg.Syntax {
		generated[file] = ast.IsGenerated(file)
	}
	imports := packageTypeImports(pkg.Imports)
	return SnapshotFromSyntax(SnapshotInput{
		Fset: fset, Files: pkg.Syntax, PackagePath: pkg.PkgPath, PackageName: pkg.Name,
		ModulePath: modulePath(pkg), Types: pkg.Types, TypesInfo: pkg.TypesInfo,
		TypeErrors: packageTypeErrors(pkg), Imports: imports, Generated: generated,
	})
}

func packageTypeErrors(pkg *packages.Package) []types.Error {
	if pkg == nil || !pkg.IllTyped {
		return nil
	}
	// packages.Package does not retain typed errors in analysis.Pass form. A
	// sentinel preserves the only fact SnapshotInput needs: completeness.
	return []types.Error{{Msg: "package is ill-typed"}}
}
