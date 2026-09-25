package analyzer

import (
	"fmt"
	"sort"
	"sync"
)

// GroupExecutionStats describes actual work performed for one runner group.
type GroupExecutionStats struct {
	Name        string `json:"name"`
	Scope       string `json:"scope"`
	Executions  int    `json:"executions"`
	CacheHits   int    `json:"cacheHits"`
	CacheMisses int    `json:"cacheMisses"`
}

// PackageAnalysisStats reports whether a package supplied complete go/types
// facts to the selected analysis run. Syntax-capable checks may still produce
// findings when this is false.
type PackageAnalysisStats struct {
	Package      string `json:"package"`
	TypeComplete bool   `json:"typeComplete"`
}

// CheckCoverageStats explains the analysis coverage for every registered
// check. Status and Reason are deliberately additive operational metadata: a
// finding's ID, evidence, fingerprint, and report schema do not depend on
// them.
type CheckCoverageStats struct {
	Check  CheckID `json:"check"`
	Status string  `json:"status"`
	Reason string  `json:"reason"`
}

// ExecutionStats is deterministic, machine-readable evidence of plan and
// cache behavior. Timings remain diagnostic-only and are intentionally absent.
type ExecutionStats struct {
	PlanIdentity   string                 `json:"planIdentity"`
	SelectedChecks []CheckID              `json:"selectedChecks"`
	SkippedChecks  []CheckID              `json:"skippedChecks"`
	Groups         []GroupExecutionStats  `json:"groups"`
	Packages       []PackageAnalysisStats `json:"packages"`
	CheckCoverage  []CheckCoverageStats   `json:"checkCoverage"`
	// Warnings describe degraded analysis inputs that did not stop the run.
	Warnings []string `json:"warnings,omitempty"`
}

type runStats struct {
	mu     sync.Mutex
	plan   ExecutionPlan
	config Config
	groups map[string]*GroupExecutionStats
}

func newRunStats(plan ExecutionPlan, cfg Config) *runStats {
	stats := &runStats{plan: plan, config: cfg, groups: map[string]*GroupExecutionStats{}}
	for _, group := range plan.groups {
		stats.groups[group.Name] = &GroupExecutionStats{Name: group.Name, Scope: scopeName(group.Scope)}
	}
	return stats
}

func scopeName(scope Scope) string {
	if scope == ScopeProgram {
		return "program"
	}
	return "package"
}

func (s *runStats) execution(group string, cacheHit bool, cacheUsed bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.groups[group]
	if entry == nil {
		return
	}
	if cacheUsed {
		if cacheHit {
			entry.CacheHits++
		} else {
			entry.CacheMisses++
		}
	}
	if !cacheHit {
		entry.Executions++
	}
}

func (s *runStats) snapshot(pkgs []*packageFiles) ExecutionStats {
	if s == nil {
		return ExecutionStats{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	result := ExecutionStats{PlanIdentity: s.plan.Identity(), SelectedChecks: s.plan.SelectedCheckIDs()}
	for _, check := range checkRegistry {
		if !s.plan.selected[check.ID] {
			result.SkippedChecks = append(result.SkippedChecks, check.ID)
		}
	}
	for _, group := range s.plan.groups {
		if entry := s.groups[group.Name]; entry != nil {
			result.Groups = append(result.Groups, *entry)
		}
	}
	for _, pkg := range pkgs {
		if pkg == nil {
			continue
		}
		result.Packages = append(result.Packages, PackageAnalysisStats{Package: pkg.pkgPath, TypeComplete: pkg.typeComplete})
		if pkg.filteredTypeErr != "" {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"%s: excluded files could not be removed from type information (%s); their declarations stay visible to type-based checks",
				pkg.pkgPath, pkg.filteredTypeErr,
			))
		}
	}
	for _, check := range checkRegistry {
		result.CheckCoverage = append(result.CheckCoverage, checkCoverageFor(check, s.plan, s.config, pkgs))
	}
	sort.SliceStable(result.Packages, func(i, j int) bool { return result.Packages[i].Package < result.Packages[j].Package })
	sort.SliceStable(result.Groups, func(i, j int) bool { return result.Groups[i].Name < result.Groups[j].Name })
	return result
}

func checkCoverageFor(check Check, plan ExecutionPlan, cfg Config, pkgs []*packageFiles) CheckCoverageStats {
	coverage := CheckCoverageStats{Check: check.ID}
	if !check.Surfaces.Supports(plan.Surface()) {
		coverage.Status = "unsupported-surface"
		coverage.Reason = "the " + surfaceName(plan.Surface()) + " integration does not support this check"
		return coverage
	}
	if !plan.Includes(check.ID) {
		coverage.Status = "not-selected"
		coverage.Reason = "not selected by the resolved profile, rule, and check configuration"
		return coverage
	}
	if requirement := missingArchitectureRequirement(check.ID, cfg); requirement != "" {
		coverage.Status = "configuration-unavailable"
		coverage.Reason = requirement
		return coverage
	}
	if len(pkgs) == 0 {
		coverage.Status = "no-packages"
		coverage.Reason = "no analyzable packages were loaded"
		return coverage
	}
	complete, incomplete := packageTypeCoverage(pkgs)
	if incomplete == 0 {
		coverage.Status = "complete"
		coverage.Reason = "complete go/types information was available for every analyzed package"
		return coverage
	}
	switch check.Syntax {
	case SyntaxEquivalent:
		coverage.Status = "syntax-equivalent"
		coverage.Reason = "this check is equivalent without complete go/types information"
	case SyntaxConservative:
		coverage.Status = "conservative"
		coverage.Reason = "one or more packages lack complete go/types information; this check ran its conservative syntax fallback"
	case SyntaxUnavailable:
		if complete == 0 {
			coverage.Status = "unavailable"
			coverage.Reason = "complete go/types information was unavailable for every analyzed package"
		} else {
			coverage.Status = "partial"
			coverage.Reason = "complete go/types information was unavailable for one or more packages; findings from those packages were omitted"
		}
	default:
		coverage.Status = "unavailable"
		coverage.Reason = "the check has no declared analysis-coverage contract"
	}
	return coverage
}

func packageTypeCoverage(pkgs []*packageFiles) (complete, incomplete int) {
	for _, pkg := range pkgs {
		if pkg != nil && pkg.typeComplete {
			complete++
		} else {
			incomplete++
		}
	}
	return complete, incomplete
}

func surfaceName(surface Surface) string {
	switch surface {
	case SurfaceCLI:
		return "CLI"
	case SurfaceModulePlugin:
		return "module-plugin"
	case SurfaceGoPlugin:
		return "go-plugin"
	default:
		return "requested"
	}
}

func missingArchitectureRequirement(id CheckID, cfg Config) string {
	hasLogicPackages := len(cfg.OCPLogicPackages) > 0
	hasImplementationPackages := len(cfg.OCPImplementationPackages) > 0
	//nolint:exhaustive // Only architecture-gated checks need an explicit reason.
	switch id {
	case CheckOCPImplementationCoupling, CheckDIPLayerImport, CheckDIPHiddenConstruction, CheckDIPWiringOutsideRoot:
		if !hasLogicPackages || !hasImplementationPackages {
			return "architecture.logic_packages and architecture.implementation_packages must both be configured"
		}
	case CheckDIPInfraErrorLeak, CheckDIPTransportLeak:
		if !hasLogicPackages {
			return "architecture.logic_packages must be configured"
		}
	default:
		// The remaining checks do not require architecture package lists.
	}
	return ""
}
