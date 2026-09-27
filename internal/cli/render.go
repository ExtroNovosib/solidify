package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
		cwd, _ := os.Getwd()
		for _, issue := range issues {
			line := formatIssueLine(issue, cwd)
			metadata, known := analyzer.CheckMetadata(issue.Check)
			if (profile == analyzer.ProfileAll || profile == analyzer.ProfileCalibration) && known && metadata.Maturity == analyzer.MaturityExperimental {
				fmt.Println(line, "[experimental]")
			} else {
				fmt.Println(line)
			}
		}
		fmt.Printf("\n%d issue(s) found\n", len(issues))
		return nil
	}
}

// formatIssueLine renders a text finding with its path relative to the
// working directory when the file lies under it, so editors and terminals can
// open it directly. Other paths are printed unchanged; JSON and SARIF keep
// their portable module-relative paths.
func formatIssueLine(issue analyzer.Issue, cwd string) string {
	issue.Pos.Filename = workingDirectoryPath(issue.Pos.Filename, cwd)
	return issue.String()
}

func workingDirectoryPath(filename, cwd string) string {
	if cwd == "" || !filepath.IsAbs(filename) {
		return filename
	}
	relative, err := filepath.Rel(cwd, filename)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return filename
	}
	return relative
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
	for _, pkg := range stats.Packages {
		fmt.Printf("package %s type_complete=%t\n", pkg.Package, pkg.TypeComplete)
	}
	for _, coverage := range stats.CheckCoverage {
		fmt.Printf("check %s status=%s reason=%s\n", coverage.Check, coverage.Status, coverage.Reason)
	}
	for _, warning := range stats.Warnings {
		fmt.Printf("warning %s\n", warning)
	}
	return nil
}
