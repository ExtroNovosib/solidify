package analyzer

import (
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

// ExecutionStats is deterministic, machine-readable evidence of plan and
// cache behavior. Timings remain diagnostic-only and are intentionally absent.
type ExecutionStats struct {
	PlanIdentity   string                `json:"planIdentity"`
	SelectedChecks []CheckID             `json:"selectedChecks"`
	SkippedChecks  []CheckID             `json:"skippedChecks"`
	Groups         []GroupExecutionStats `json:"groups"`
}

type runStats struct {
	mu     sync.Mutex
	plan   ExecutionPlan
	groups map[string]*GroupExecutionStats
}

func newRunStats(plan ExecutionPlan) *runStats {
	stats := &runStats{plan: plan, groups: map[string]*GroupExecutionStats{}}
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

func (s *runStats) snapshot() ExecutionStats {
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
	sort.SliceStable(result.Groups, func(i, j int) bool { return result.Groups[i].Name < result.Groups[j].Name })
	return result
}
