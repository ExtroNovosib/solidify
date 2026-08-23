package analyzer

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
)

// PackageSnapshot exposes loaded package state to external analysis drivers.
type PackageSnapshot struct {
	pkg *packageFiles
}

// SnapshotFromSyntax builds a package snapshot for go/analysis bridges.
func SnapshotFromSyntax(fset *token.FileSet, files []*ast.File, info *types.Info, typeComplete bool) *PackageSnapshot {
	generated := map[*ast.File]bool{}
	for _, file := range files {
		generated[file] = ast.IsGenerated(file)
	}
	pkgName := ""
	if len(files) > 0 {
		pkgName = files[0].Name.Name
	}
	return &PackageSnapshot{pkg: &packageFiles{
		fset: fset, files: files, info: info, typeComplete: typeComplete && info != nil,
		pkgPath: pkgName, pkgName: pkgName, generated: generated,
	}}
}

// RunISP executes package-scoped ISP checks on the snapshot.
func (p *PackageSnapshot) RunISP(cfg Config) []Issue {
	if p == nil || p.pkg == nil {
		return nil
	}
	issues := CheckISPWithTypes(p.pkg.fset, p.pkg.files, p.pkg.info, cfg, p.pkg)
	AttachDefaultSuppressions(issues)
	_ = FinalizeIssues(issues, p.pkg.pkgPath)
	return issues
}

// RunSRP executes all nine package-scoped SRP checks on the snapshot.
func (p *PackageSnapshot) RunSRP(cfg Config) []Issue {
	if p == nil || p.pkg == nil {
		return nil
	}
	issues := runSRPCheck(p.pkg, cfg)
	AttachDefaultSuppressions(issues)
	_ = FinalizeIssues(issues, p.pkg.pkgPath)
	return issues
}

// RunLSP executes the package-scoped non-exact-EOF check on the snapshot.
func (p *PackageSnapshot) RunLSP(cfg Config) []Issue {
	if p == nil || p.pkg == nil {
		return nil
	}
	issues := runLSPPackageCheck(p.pkg, cfg)
	AttachDefaultSuppressions(issues)
	_ = FinalizeIssues(issues, p.pkg.pkgPath)
	return issues
}

// RunDIP executes package-scoped DIP checks on the snapshot.
func (p *PackageSnapshot) RunDIP(cfg Config) []Issue {
	if p == nil || p.pkg == nil {
		return nil
	}
	issues := CheckDIPWithTypes(p.pkg.fset, p.pkg.files, p.pkg.info, cfg, p.pkg)
	AttachDefaultSuppressions(issues)
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
	return &PackageSnapshot{pkg: &packageFiles{
		dir: pkg.Dir, fset: fset, files: pkg.Syntax, info: pkg.TypesInfo, typePkg: pkg.Types,
		typeComplete: pkg.Types != nil && pkg.TypesInfo != nil && !pkg.IllTyped,
		pkgPath:      pkg.PkgPath, pkgName: pkg.Name, modulePath: modulePath(pkg), imports: sortedImportPaths(pkg.Imports),
		generated: generated,
	}}
}
