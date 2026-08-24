package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/ExtroNovosib/solidify/internal/analyzer"
	"github.com/ExtroNovosib/solidify/internal/report"
)

func renderIssues(issues []analyzer.Issue, format string, profile analyzer.Profile, build BuildInfo) error {
	switch format {
	case "json":
		data, err := report.EncodeJSON(issues)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(data)
		return err
	case "sarif":
		return report.EncodeSARIF(os.Stdout, issues, report.SARIFMetadata{ToolName: "solidlint", ToolVersion: build.Version})
	default:
		for _, issue := range issues {
			metadata, known := analyzer.CheckMetadata(issue.Check)
			if profile == analyzer.ProfileAll && known && metadata.Maturity == analyzer.MaturityExperimental {
				fmt.Println(issue.String(), "[experimental]")
			} else {
				fmt.Println(issue.String())
			}
		}
		fmt.Printf("\n%d issue(s) found\n", len(issues))
		return nil
	}
}

func renderEffectiveConfig(policy checkPolicy) error {
	rules := make([]string, 0, len(policy.enabled))
	for rule, enabled := range policy.enabled {
		if enabled {
			rules = append(rules, string(rule))
		}
	}
	sort.Strings(rules)
	output := struct {
		SchemaVersion int                `json:"schemaVersion"`
		ConfigFile    string             `json:"configFile,omitempty"`
		Profile       analyzer.Profile   `json:"profile"`
		EnabledRules  []string           `json:"enabledRules"`
		EnabledChecks []analyzer.CheckID `json:"enabledChecks"`
		FailLevel     string             `json:"failLevel"`
		Config        analyzer.Config    `json:"config"`
	}{1, policy.configFile, policy.config.Profile, rules, policy.plan.SelectedCheckIDs(), policy.options.failLevel, policy.config}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func renderStats(stats analyzer.ExecutionStats, format string) error {
	if format == "json" {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(stats)
	}
	fmt.Printf("plan %s\n", stats.PlanIdentity)
	fmt.Printf("selected checks: %d\n", len(stats.SelectedChecks))
	for _, group := range stats.Groups {
		fmt.Printf("%s scope=%s executions=%d cache_hits=%d cache_misses=%d\n", group.Name, group.Scope, group.Executions, group.CacheHits, group.CacheMisses)
	}
	return nil
}
