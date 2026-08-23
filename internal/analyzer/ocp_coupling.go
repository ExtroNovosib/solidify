package analyzer

import (
	"fmt"
	"go/token"
	"sort"
	"strings"
)

func emitOCPImplementationCoupling(pkgs []*packageFiles, cfg Config) []Issue {
	if len(cfg.OCPLogicPackages) == 0 || len(cfg.OCPImplementationPackages) == 0 {
		return nil
	}
	var issues []Issue
	for _, pkg := range pkgs {
		if !matchesAnyPackagePattern(pkg.pkgPath, cfg.OCPLogicPackages) || matchesAnyPackagePattern(pkg.pkgPath, cfg.OCPCompositionRoots) {
			continue
		}
		var concrete []string
		for _, imported := range pkg.imports {
			if matchesAnyPackagePattern(imported, cfg.OCPImplementationPackages) {
				concrete = append(concrete, imported)
			}
		}
		if len(concrete) < cfg.OCPMinImplementationImports {
			continue
		}
		sort.Strings(concrete)
		locations := importRelatedLocations(pkg, concrete)
		start, end := token.NoPos, token.NoPos
		if len(concrete) > 0 {
			start, end = importSpecSpan(pkg, concrete[0])
		}
		issues = append(issues, issueSpan(pkg.fset, start, end, Issue{Rule: RuleOCP, Check: CheckOCPImplementationCoupling, Severity: SeverityWarning,
			Message:  fmt.Sprintf("logic package %s imports %d implementation packages directly (%s); depend on a boundary interface and compose implementations at the application edge", pkg.pkgPath, len(concrete), strings.Join(concrete, ", ")),
			Evidence: fmt.Sprintf("implementation-coupling:logic=%s;imports=%s", pkg.pkgPath, strings.Join(concrete, ",")), Metrics: []Metric{{Name: "implementation_imports", Value: float64(len(concrete)), Threshold: float64(cfg.OCPMinImplementationImports)}}, Related: locations}))
	}
	return issues
}
