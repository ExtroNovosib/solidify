package analyzer

import (
	"fmt"
	"strings"
)

func emitOCPDispatch(analysis ocpAnalysis, cfg Config) []Issue {
	groups := map[string][]*ocpDispatchSite{}
	for _, site := range analysis.dispatches {
		if site.sourceKey == "" || dispatchTypeAllowed(site.sourceKey, cfg) {
			continue
		}
		groups[site.sourceKey] = append(groups[site.sourceKey], site)
	}
	var issues []Issue
	for _, sites := range groups {
		sortDispatchSites(sites)
		components := dispatchComponents(sites, cfg)
		for _, component := range components {
			shared := frequentVariants(component, cfg.OCPMinSharedVariants)
			large := false
			maxCases := 0
			for _, site := range component {
				if len(site.variants) > maxCases {
					maxCases = len(site.variants)
				}
				if len(site.variants) > cfg.MaxTypeSwitchCases {
					large = true
				}
			}
			// Repeated dispatch is intentionally a high-confidence signal: a
			// family must either contain a large site or share more variants than
			// the ordinary switch threshold. This keeps exact-threshold boundary
			// code and small visitors out of the default findings.
			allVariants := unionVariants(component)
			largeFamily := large || len(shared) > cfg.MaxTypeSwitchCases || (len(component) >= cfg.OCPMinDispatchSites && len(allVariants) > cfg.MaxTypeSwitchCases)
			hasStandaloneAssertion := false
			for _, site := range component {
				if site.kind == ocpKindTypeAssertion {
					hasStandaloneAssertion = true
					break
				}
			}
			needsSharedVariants := len(component) > 1 && len(shared) < cfg.OCPMinSharedVariants && !hasStandaloneAssertion
			if len(component) == 0 || !largeFamily || (len(component) > 1 && (len(component) < cfg.OCPMinDispatchSites || needsSharedVariants)) {
				continue
			}
			primary := component[0]
			for _, site := range component {
				analysis.flaggedDispatch[site] = true
			}
			locations := relatedLocations(component[1:], "same dispatch family")
			message := fmt.Sprintf("dispatches on %s in %d place(s) with %d shared variant(s) (%s): adding a variant requires editing multiple locations; prefer behavior on an interface", displaySourceType(primary), len(component), len(shared), strings.Join(allVariants, ", "))
			if len(component) == 1 {
				if primary.kind == "if/else-if chain" {
					message = fmt.Sprintf("if/else-if chain has %d type assertions (max %d): consider replacing it with a type switch behind an interface, or polymorphic dispatch", maxCases, cfg.MaxTypeSwitchCases)
				} else {
					message = fmt.Sprintf("type switch has %d cases (max %d): adding a new type requires editing this dispatch; prefer behavior on an interface", maxCases, cfg.MaxTypeSwitchCases)
				}
			}
			issues = append(issues, issueAt(primary.pkg.fset, primary.node, Issue{Rule: RuleOCP, Check: CheckOCPTypeDispatch, Severity: SeverityWarning,
				Message:  message,
				Evidence: fmt.Sprintf("type-dispatch:source=%s;sites=%d;shared=%s;variants=%s;max_cases=%d", primary.sourceKey, len(component), strings.Join(shared, ","), strings.Join(allVariants, ","), maxCases),
				Metrics:  []Metric{{Name: "sites", Value: float64(len(component)), Threshold: float64(cfg.OCPMinDispatchSites)}, {Name: "shared_variants", Value: float64(len(shared)), Threshold: float64(cfg.OCPMinSharedVariants)}, {Name: "max_cases", Value: float64(maxCases), Threshold: float64(cfg.MaxTypeSwitchCases)}},
				Related:  locations}))
		}
	}
	return issues
}

func emitOCPDiscriminators(analysis ocpAnalysis, cfg Config) []Issue {
	groups := map[string][]*ocpDiscriminatorSite{}
	for _, site := range analysis.discriminators {
		if site.serialization {
			continue
		}
		groups[site.fieldKey] = append(groups[site.fieldKey], site)
	}
	var issues []Issue
	for _, sites := range groups {
		sortDiscriminatorSites(sites)
		for _, component := range discriminatorComponents(sites, cfg.OCPMinSharedVariants) {
			shared := frequentDiscriminatorValues(component, cfg.OCPMinSharedVariants)
			if len(component) < cfg.OCPMinDispatchSites || len(shared) < cfg.OCPMinSharedVariants {
				continue
			}
			for _, site := range component {
				analysis.flaggedDisc[site] = true
			}
			severity := SeverityNote
			if len(component) >= cfg.OCPMinDispatchSites+1 {
				severity = SeverityWarning
			}
			primary := component[0]
			issues = append(issues, issueAt(primary.pkg.fset, primary.node, Issue{Rule: RuleOCP, Check: CheckOCPDiscriminatorDispatch, Severity: severity,
				Message:  fmt.Sprintf("discriminator field %s is branched on in %d places with shared values (%s): adding a variant requires editing multiple branches", primary.fieldKey, len(component), strings.Join(shared, ", ")),
				Evidence: fmt.Sprintf("discriminator-dispatch:field=%s;sites=%d;shared=%s", primary.fieldKey, len(component), strings.Join(shared, ",")),
				Metrics:  []Metric{{Name: "sites", Value: float64(len(component)), Threshold: float64(cfg.OCPMinDispatchSites)}, {Name: "shared_values", Value: float64(len(shared)), Threshold: float64(cfg.OCPMinSharedVariants)}}, Related: relatedDiscriminatorLocations(component[1:])}))
		}
	}
	return issues
}

func emitOCPRuntime(analysis ocpAnalysis) []Issue {
	var issues []Issue
	for _, site := range analysis.dispatches {
		if !analysis.flaggedDispatch[site] || !site.defaultBad || site.serialization || sealedInterface(site.source) {
			continue
		}
		issues = append(issues, issueAt(site.pkg.fset, site.node, Issue{Rule: RuleOCP, Check: CheckOCPRuntimeExhaustiveness, Severity: SeverityNote,
			Message:  fmt.Sprintf("default branch rejects an unhandled %s at runtime; extending the dispatch can fail only after execution", displaySourceType(site)),
			Evidence: "runtime-exhaustiveness:source=" + site.sourceKey, Related: nil}))
	}
	for _, site := range analysis.discriminators {
		if !analysis.flaggedDisc[site] || !site.defaultBad {
			continue
		}
		issues = append(issues, issueAt(site.pkg.fset, site.node, Issue{Rule: RuleOCP, Check: CheckOCPRuntimeExhaustiveness, Severity: SeverityNote,
			Message: fmt.Sprintf("default branch rejects an unhandled value of discriminator %s at runtime", site.fieldKey), Evidence: "runtime-exhaustiveness:field=" + site.fieldKey}))
	}
	return issues
}
