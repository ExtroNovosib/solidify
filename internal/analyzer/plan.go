package analyzer

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// ExecutionGroup is one registry runner and the selected checks it owns.
// Checks and groups retain registry order so execution and diagnostics are
// deterministic across processes.
type ExecutionGroup struct {
	Name   string
	Scope  Scope
	Checks []CheckID
}

// ExecutionPlan is the resolved, surface-aware analysis contract used by all
// execution adapters. Its internal selection is never exposed for mutation.
type ExecutionPlan struct {
	profile  Profile
	mode     string
	surface  Surface
	selected map[CheckID]bool
	groups   []ExecutionGroup
	identity string
}

// NewExecutionPlan resolves public selection once and groups selected checks
// by their shared runner before any analysis work is scheduled.
func NewExecutionPlan(cfg Config, enabled map[Rule]bool, surface Surface) (ExecutionPlan, error) {
	if surface == 0 {
		surface = SurfaceCLI
	}
	selection, err := ResolveCheckSelection(cfg.Profile, enabled, cfg.EnabledChecks, cfg.DisabledChecks)
	if err != nil {
		return ExecutionPlan{}, err
	}
	selected := make(map[CheckID]bool, len(selection))
	groupIndex := map[string]int{}
	groups := make([]ExecutionGroup, 0)
	for _, check := range checkRegistry {
		if !selection[check.ID] || !check.Surfaces.Supports(surface) {
			continue
		}
		selected[check.ID] = true
		index, ok := groupIndex[check.RunnerGroup]
		if !ok {
			index = len(groups)
			groupIndex[check.RunnerGroup] = index
			groups = append(groups, ExecutionGroup{Name: check.RunnerGroup, Scope: check.Scope})
		}
		groups[index].Checks = append(groups[index].Checks, check.ID)
	}
	profile := cfg.Profile
	if profile == "" {
		profile = ProfileStable
	}
	mode := cfg.AnalysisMode
	if mode == "" {
		mode = analysisModeAuto
	}
	plan := ExecutionPlan{profile: profile, mode: mode, surface: surface, selected: selected, groups: groups}
	plan.identity = planIdentity(plan)
	return plan, nil
}

func planIdentity(plan ExecutionPlan) string {
	data, _ := json.Marshal(struct {
		Profile Profile
		Mode    string
		Surface Surface
		Checks  []CheckID
	}{plan.profile, plan.mode, plan.surface, plan.SelectedCheckIDs()})
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:8])
}

// Profile returns the resolved profile.
func (p ExecutionPlan) Profile() Profile { return p.profile }

// AnalysisMode returns the resolved analysis mode.
func (p ExecutionPlan) AnalysisMode() string { return p.mode }

// Surface returns the target integration surface.
func (p ExecutionPlan) Surface() Surface { return p.surface }

// Identity returns a stable digest of the selection-affecting plan fields.
func (p ExecutionPlan) Identity() string { return p.identity }

// Includes reports whether a concrete check is selected for this surface.
func (p ExecutionPlan) Includes(id CheckID) bool { return p.selected[id] }

// SelectedCheckIDs returns selected checks in registry order.
func (p ExecutionPlan) SelectedCheckIDs() []CheckID {
	ids := make([]CheckID, 0, len(p.selected))
	for _, check := range checkRegistry {
		if p.selected[check.ID] {
			ids = append(ids, check.ID)
		}
	}
	return ids
}

// Groups returns a defensive copy of the scheduled runner groups.
func (p ExecutionPlan) Groups() []ExecutionGroup {
	groups := make([]ExecutionGroup, len(p.groups))
	for i, group := range p.groups {
		groups[i] = group
		groups[i].Checks = append([]CheckID(nil), group.Checks...)
	}
	return groups
}

func (p ExecutionPlan) selectionCopy() map[CheckID]bool {
	selection := make(map[CheckID]bool, len(p.selected))
	for id, selected := range p.selected {
		selection[id] = selected
	}
	return selection
}

func runnerForGroup(group ExecutionGroup) (Check, bool) {
	for _, check := range checkRegistry {
		if check.RunnerGroup != group.Name {
			continue
		}
		if group.Scope == ScopePackage && check.RunPackage != nil {
			return check, true
		}
		if group.Scope == ScopeProgram && check.RunProgram != nil {
			return check, true
		}
	}
	return Check{}, false
}

func filterGroupIssues(issues []Issue, group ExecutionGroup) []Issue {
	selected := make(map[CheckID]bool, len(group.Checks))
	for _, id := range group.Checks {
		selected[id] = true
	}
	out := issues[:0]
	for _, issue := range issues {
		if selected[issue.Check] {
			out = append(out, issue)
		}
	}
	return out
}

func groupCacheID(group ExecutionGroup) CheckID {
	return CheckID("runner-group/" + group.Name)
}
