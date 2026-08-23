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
// or god-type violations.
func CheckSRPWithTypes(in SRPCheckInput) []Issue {
	return checkSRPWithTypes(in)
}

func checkSRPWithTypes(in SRPCheckInput) []Issue {
	issues := checkSRPSyntax(in.Fset, in.Files, in.Config, in.PkgFiles)
	if in.TypeComplete && in.Info != nil {
		issues = removeIssuesByCheck(issues, CheckSRPMixedInputSurface)
		issues = removeIssuesByCheck(issues, CheckSRPDataClump)
		issues = removeIssuesByCheck(issues, CheckSRPFlagArgument)
		issues = append(issues, typedParameterIssues(in.Fset, in.Files, in.Info, in.Config, in.PkgFiles)...)
	}
	profiles := buildSRPTypeProfiles(in.Fset, in.Files, in.Info, in.Pkg, in.PkgFiles)
	issues = removeIssuesByCheck(issues, CheckSRPLargeType)
	for _, profile := range profiles {
		if large := srpProfileLargeTypeIssue(profile, in.Fset, in.Config, in.TypeComplete); large != nil {
			issues = append(issues, *large)
		}
		var god *Issue
		if in.TypeComplete {
			god = srpProfileGodTypeIssue(profile, in.Fset, in.Config)
		}
		if god == nil {
			if fanout := srpProfileFanOutIssue(profile, in.Fset, in.Config); fanout != nil {
				issues = append(issues, *fanout)
			}
		}
		if mixed := srpProfileMixedImportClustersIssue(profile, in.Fset, in.Config); mixed != nil {
			issues = append(issues, *mixed)
		}
		if !in.TypeComplete {
			continue
		}
		if god != nil {
			issues = append(issues, *god)
			continue
		}
		if low := srpProfileLowCohesionIssue(profile, in.Fset, in.Config); low != nil {
			issues = append(issues, *low)
		}
	}
	return issues
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
