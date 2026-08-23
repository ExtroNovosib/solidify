package analysisapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/ExtroNovosib/solidify/internal/analyzer"
)

type pluginSettings struct {
	Profile           analyzer.Profile             `json:"profile"`
	EnabledRules      []string                     `json:"enabled_rules"`
	EnabledChecks     []analyzer.CheckID           `json:"enabled_checks"`
	DisabledChecks    []analyzer.CheckID           `json:"disabled_checks"`
	Thresholds        map[string]int               `json:"thresholds"`
	Severity          map[string]analyzer.Severity `json:"severity"`
	AllowDependencies []string                     `json:"allow_dependencies"`
}

type resolvedSettings struct {
	config   analyzer.Config
	selected map[analyzer.CheckID]bool
	severity map[string]analyzer.Severity
}

// NewAnalyzers is the single factory used by both GolangCI integrations.
func NewAnalyzers(settings any) ([]*analysis.Analyzer, error) {
	resolved, err := resolveSettings(settings)
	if err != nil {
		return nil, err
	}
	return []*analysis.Analyzer{
		newPackageAnalyzer("solidsrp", "reports package-scoped SRP findings", resolved, func(snapshot *analyzer.PackageSnapshot, cfg analyzer.Config) []analyzer.Issue {
			return snapshot.RunSRP(cfg)
		}),
		newPackageAnalyzer("solidlsp", "reports package-scoped LSP findings", resolved, func(snapshot *analyzer.PackageSnapshot, cfg analyzer.Config) []analyzer.Issue {
			return snapshot.RunLSP(cfg)
		}),
		newPackageAnalyzer("solidisp", "reports package-scoped ISP findings", resolved, func(snapshot *analyzer.PackageSnapshot, cfg analyzer.Config) []analyzer.Issue {
			return snapshot.RunISP(cfg)
		}),
		newPackageAnalyzer("soliddip", "reports package-scoped DIP findings", resolved, func(snapshot *analyzer.PackageSnapshot, cfg analyzer.Config) []analyzer.Issue {
			return snapshot.RunDIP(cfg)
		}),
	}, nil
}

func resolveSettings(value any) (resolvedSettings, error) {
	settings := pluginSettings{Profile: analyzer.ProfileStable}
	if value != nil {
		data, err := json.Marshal(value)
		if err != nil {
			return resolvedSettings{}, fmt.Errorf("encode plugin settings: %w", err)
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&settings); err != nil {
			return resolvedSettings{}, fmt.Errorf("invalid solidlint plugin settings: %w", err)
		}
	}
	cfg := analyzer.DefaultConfig()
	cfg.Profile = settings.Profile
	cfg.DIPAllowDependencies = append([]string(nil), settings.AllowDependencies...)
	if err := analyzer.ApplyThresholds(&cfg, settings.Thresholds); err != nil {
		return resolvedSettings{}, err
	}
	enabledRules := map[analyzer.Rule]bool{}
	for _, code := range settings.EnabledRules {
		rule, ok := pluginRule(code)
		if !ok {
			return resolvedSettings{}, fmt.Errorf("unknown enabled rule %q", code)
		}
		enabledRules[rule] = true
	}
	for _, id := range settings.EnabledChecks {
		metadata, ok := analyzer.CheckMetadata(id)
		if !ok {
			return resolvedSettings{}, fmt.Errorf("unknown enabled check %q", id)
		}
		if !metadata.Surfaces.Supports(analyzer.SurfaceModulePlugin) {
			return resolvedSettings{}, fmt.Errorf("check %q is CLI-only and cannot run in a package plugin", id)
		}
	}
	selection, err := analyzer.ResolveCheckSelection(settings.Profile, enabledRules, settings.EnabledChecks, settings.DisabledChecks)
	if err != nil {
		return resolvedSettings{}, err
	}
	for id := range selection {
		metadata, _ := analyzer.CheckMetadata(id)
		selection[id] = selection[id] && metadata.Surfaces.Supports(analyzer.SurfaceModulePlugin)
	}
	for target, severity := range settings.Severity {
		if severity != analyzer.SeverityNote && severity != analyzer.SeverityWarning && severity != analyzer.SeverityError {
			return resolvedSettings{}, fmt.Errorf("invalid severity %q for %q", severity, target)
		}
		if !analyzer.IsKnownSeverityTarget(target) {
			return resolvedSettings{}, fmt.Errorf("unknown severity target %q", target)
		}
	}
	return resolvedSettings{config: cfg, selected: selection, severity: settings.Severity}, nil
}

func pluginRule(code string) (analyzer.Rule, bool) {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "S", string(analyzer.RuleSRP):
		return analyzer.RuleSRP, true
	case "L", string(analyzer.RuleLSP):
		return analyzer.RuleLSP, true
	case "I", string(analyzer.RuleISP):
		return analyzer.RuleISP, true
	case "D", string(analyzer.RuleDIP):
		return analyzer.RuleDIP, true
	default:
		return "", false
	}
}

func newPackageAnalyzer(name, doc string, settings resolvedSettings, run func(*analyzer.PackageSnapshot, analyzer.Config) []analyzer.Issue) *analysis.Analyzer {
	return &analysis.Analyzer{Name: name, Doc: doc, RunDespiteErrors: true, Run: func(pass *analysis.Pass) (any, error) {
		snapshot := analyzer.SnapshotFromSyntax(pass.Fset, pass.Files, pass.TypesInfo, pass.TypesInfo != nil)
		issues := run(snapshot, settings.config)
		filtered := issues[:0]
		for index := range issues {
			if !settings.selected[issues[index].Check] {
				continue
			}
			if severity := settings.severity[issues[index].ID()]; severity != "" {
				issues[index].Severity = severity
			} else if severity := settings.severity[string(issues[index].Rule)]; severity != "" {
				issues[index].Severity = severity
			}
			filtered = append(filtered, issues[index])
		}
		reportIssues(pass, filtered)
		return nil, nil
	}}
}
