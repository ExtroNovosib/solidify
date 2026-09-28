package analyzer

import (
	"go/ast"
	"go/token"
	"go/types"
)

// SRPCheckInput groups the package context for SRP analysis.
type SRPCheckInput struct {
	Fset         *token.FileSet
	Files        []*ast.File
	Info         *types.Info
	Pkg          *types.Package
	TypeComplete bool
	Config       Config
	PkgFiles     *packageFiles
}

// CheckSRPWithTypes combines the always-available syntax checks with the
// package-wide metrics that need a complete type graph.  A syntax-only run
// deliberately emits advisory findings but never guesses at strict cohesion
// or god-type violations. SOLID-S/large-type applies the same multi-signal
// profile rule in both modes, so its findings and fingerprints agree.
func CheckSRPWithTypes(in SRPCheckInput) []Issue {
	return checkSRPWithTypes(in)
}

func checkSRPWithTypes(in SRPCheckInput) []Issue {
	issues := checkSRPSyntax(in.Fset, in.Files, in.Config, in.PkgFiles)
	if checkEnabled(in.Config, CheckSRPTransportWorkflow) {
		issues = append(issues, checkTransportWorkflows(in.Fset, in.Files, in.Info, in.Config, in.PkgFiles)...)
	}
	parameterChecks := checkEnabled(in.Config, CheckSRPMixedInputSurface) || checkEnabled(in.Config, CheckSRPDataClump) || checkEnabled(in.Config, CheckSRPFlagArgument)
	if parameterChecks && in.TypeComplete && in.Info != nil {
		issues = removeIssuesByCheck(issues, CheckSRPMixedInputSurface)
		issues = removeIssuesByCheck(issues, CheckSRPDataClump)
		issues = removeIssuesByCheck(issues, CheckSRPFlagArgument)
		issues = append(issues, filterSelectedIssues(typedParameterIssues(in.Fset, in.Files, in.Info, in.Config, in.PkgFiles), in.Config)...)
	}
	profileChecks := checkEnabled(in.Config, CheckSRPLargeType) || checkEnabled(in.Config, CheckSRPGodType) || checkEnabled(in.Config, CheckSRPHighFanOutType) || checkEnabled(in.Config, CheckSRPMixedImportClusters) || checkEnabled(in.Config, CheckSRPLowCohesionType)
	if !profileChecks {
		return issues
	}
	if !in.TypeComplete || in.Info == nil || in.Pkg == nil {
		return append(issues, syntaxLargeTypeIssues(in)...)
	}
	profiles := buildSRPTypeProfiles(in.Fset, in.Files, in.Info, in.Pkg, in.PkgFiles)
	for _, profile := range profiles {
		if pureDelegatingProfile(profile, in.Info) {
			continue
		}
		if checkEnabled(in.Config, CheckSRPLargeType) {
			if large := srpProfileLargeTypeIssue(profile, in.Fset, in.Config, in.TypeComplete); large != nil {
				issues = append(issues, *large)
			}
		}
		var god *Issue
		if in.TypeComplete && checkEnabled(in.Config, CheckSRPGodType) {
			god = srpProfileGodTypeIssue(profile, in.Fset, in.Config)
		}
		if god == nil && checkEnabled(in.Config, CheckSRPHighFanOutType) {
			if fanout := srpProfileFanOutIssue(profile, in.Fset, in.Config); fanout != nil {
				issues = append(issues, *fanout)
			}
		}
		if checkEnabled(in.Config, CheckSRPMixedImportClusters) {
			if mixed := srpProfileMixedImportClustersIssue(profile, in.Fset, in.Config); mixed != nil {
				issues = append(issues, *mixed)
			}
		}
		if !in.TypeComplete {
			continue
		}
		if god != nil {
			issues = append(issues, *god)
			continue
		}
		if checkEnabled(in.Config, CheckSRPLowCohesionType) {
			if low := srpProfileLowCohesionIssue(profile, in.Fset, in.Config); low != nil {
				issues = append(issues, *low)
			}
		}
	}
	return issues
}

// syntaxLargeTypeIssues evaluates the typed large-type rule on profiles built
// from syntax alone. Every size signal is syntactic; only the TCC exemption
// needs resolved selections and is therefore not applied.
func syntaxLargeTypeIssues(in SRPCheckInput) []Issue {
	if !checkEnabled(in.Config, CheckSRPLargeType) {
		return nil
	}
	var issues []Issue
	for _, profile := range buildSRPTypeProfiles(in.Fset, in.Files, nil, nil, in.PkgFiles) {
		if large := srpProfileLargeTypeIssue(profile, in.Fset, in.Config, false); large != nil {
			issues = append(issues, *large)
		}
	}
	return issues
}

func filterSelectedIssues(issues []Issue, cfg Config) []Issue {
	out := issues[:0]
	for _, issue := range issues {
		if checkEnabled(cfg, issue.Check) {
			out = append(out, issue)
		}
	}
	return out
}

func removeIssuesByCheck(issues []Issue, check CheckID) []Issue {
	out := issues[:0]
	for _, issue := range issues {
		if issue.Check != check {
			out = append(out, issue)
		}
	}
	return out
}

func buildSRPTypeProfiles(fset *token.FileSet, files []*ast.File, info *types.Info, pkg *types.Package, pkgFiles *packageFiles) []*srpTypeProfile {
	profiles := collectSRPStructProfiles(files, info, pkg, pkgFiles)
	attachSRPMethodsToProfiles(profiles, files, pkgFiles)
	return finalizeSRPTypeProfiles(profiles, fset, files, info, pkg)
}

func pureDelegatingProfile(profile *srpTypeProfile, info *types.Info) bool {
	if len(profile.methods) == 0 {
		return false
	}
	for _, method := range profile.methods {
		if pureForwardingField(method, info) == "" && !guardedForwardingMethod(method, info) {
			return false
		}
	}
	return true
}

func guardedForwardingMethod(fn *ast.FuncDecl, info *types.Info) bool {
	if fn.Body == nil || len(fn.Body.List) < 2 {
		return false
	}
	for _, statement := range fn.Body.List[:len(fn.Body.List)-1] {
		guard, ok := statement.(*ast.IfStmt)
		if !ok || guard.Init != nil || guard.Else != nil {
			return false
		}
		safe := nilAvailabilityCondition(guard.Cond)
		if !safe {
			return false
		}
		for _, body := range guard.Body.List {
			ret, ok := body.(*ast.ReturnStmt)
			if !ok {
				return false
			}
			for _, expr := range ret.Results {
				ast.Inspect(expr, func(n ast.Node) bool {
					if _, ok := n.(*ast.CallExpr); ok {
						safe = false
					}
					return true
				})
			}
		}
		if !safe {
			return false
		}
	}
	clone := *fn
	clone.Body = &ast.BlockStmt{List: fn.Body.List[len(fn.Body.List)-1:]}
	return pureForwardingField(&clone, info) != ""
}

func nilAvailabilityCondition(expr ast.Expr) bool {
	binary, ok := expr.(*ast.BinaryExpr)
	if !ok {
		return false
	}
	if binary.Op == token.LOR || binary.Op == token.LAND {
		return nilAvailabilityCondition(binary.X) && nilAvailabilityCondition(binary.Y)
	}
	if binary.Op != token.EQL && binary.Op != token.NEQ {
		return false
	}
	if !isNilExpression(binary.X) && !isNilExpression(binary.Y) {
		return false
	}
	other := binary.X
	if isNilExpression(other) {
		other = binary.Y
	}
	switch other.(type) {
	case *ast.Ident, *ast.SelectorExpr:
		return true
	}
	return false
}
