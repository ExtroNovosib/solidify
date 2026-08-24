package analysisapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/types"
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
	plan     analyzer.ExecutionPlan
	severity map[string]analyzer.Severity
}

// NewAnalyzers is the single factory used by both GolangCI integrations.
func NewAnalyzers(settings any) ([]*analysis.Analyzer, error) {
	resolved, err := resolveSettings(settings)
	if err != nil {
		return nil, err
	}
	groups := resolved.plan.Groups()
	analyzers := make([]*analysis.Analyzer, 0, len(groups))
	for _, group := range groups {
		name, doc, ok := packageAnalyzerIdentity(group.Name)
		if !ok {
			continue
		}
		analyzers = append(analyzers, newPackageAnalyzer(name, doc, resolved, group))
	}
	return analyzers, nil
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
	cfg.EnabledChecks = append([]analyzer.CheckID(nil), settings.EnabledChecks...)
	cfg.DisabledChecks = append([]analyzer.CheckID(nil), settings.DisabledChecks...)
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
	plan, err := analyzer.NewExecutionPlan(cfg, enabledRules, analyzer.SurfaceModulePlugin)
	if err != nil {
		return resolvedSettings{}, err
	}
	for target, severity := range settings.Severity {
		if severity != analyzer.SeverityNote && severity != analyzer.SeverityWarning && severity != analyzer.SeverityError {
			return resolvedSettings{}, fmt.Errorf("invalid severity %q for %q", severity, target)
		}
		if !analyzer.IsKnownSeverityTarget(target) {
			return resolvedSettings{}, fmt.Errorf("unknown severity target %q", target)
		}
	}
	return resolvedSettings{config: cfg, plan: plan, severity: settings.Severity}, nil
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

func packageAnalyzerIdentity(group string) (name, doc string, ok bool) {
	switch group {
	case "srp-package":
		return "solidsrp", "reports package-scoped SRP findings", true
	case "lsp-package":
		return "solidlsp", "reports package-scoped LSP findings", true
	case "isp-package":
		return "solidisp", "reports package-scoped ISP findings", true
	case "dip-package":
		return "soliddip", "reports package-scoped DIP findings", true
	default:
		return "", "", false
	}
}

func newPackageAnalyzer(name, doc string, settings resolvedSettings, group analyzer.ExecutionGroup) *analysis.Analyzer {
	return &analysis.Analyzer{Name: name, Doc: doc, RunDespiteErrors: true, Run: func(pass *analysis.Pass) (any, error) {
		modulePath := ""
		if pass.Module != nil {
			modulePath = pass.Module.Path
		}
		pkgPath, pkgName := "", ""
		imports := map[string]*types.Package{}
		if pass.Pkg != nil {
			pkgPath, pkgName = pass.Pkg.Path(), pass.Pkg.Name()
			for _, imported := range pass.Pkg.Imports() {
				imports[imported.Path()] = imported
			}
		}
		snapshot := analyzer.SnapshotFromSyntax(analyzer.SnapshotInput{
			Fset: pass.Fset, Files: pass.Files, PackagePath: pkgPath, PackageName: pkgName,
			ModulePath: modulePath, Types: pass.Pkg, TypesInfo: pass.TypesInfo,
			TypeErrors: pass.TypeErrors, Imports: imports,
		})
		issues := snapshot.RunGroup(group, settings.config)
		for index := range issues {
			if severity := settings.severity[issues[index].ID()]; severity != "" {
				issues[index].Severity = severity
			} else if severity := settings.severity[string(issues[index].Rule)]; severity != "" {
				issues[index].Severity = severity
			}
		}
		reportIssues(pass, issues)
		return nil, nil
	}}
}
