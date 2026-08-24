package analyzer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFileConfigAndApply(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".solidify.yml")
	if err := os.WriteFile(path, []byte("enabled_rules: [S, I]\nexclude: [generated/**]\nthresholds:\n  max_methods: 2\nseverity:\n  SOLID-I/fat-interface: error\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadFileConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.EnabledRules) != 2 || len(c.Excludes) != 1 || c.Severities["SOLID-I/fat-interface"] != SeverityError {
		t.Fatalf("unexpected config: %+v", c)
	}
	cfg := DefaultConfig()
	c.Apply(&cfg)
	if cfg.MaxMethodsPerType != 2 {
		t.Fatalf("max methods = %d", cfg.MaxMethodsPerType)
	}
}

func TestLoadFileConfigAcceptsCalibrationProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".solidify.yml")
	if err := os.WriteFile(path, []byte("profile: calibration\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadFileConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Profile != ProfileCalibration {
		t.Fatalf("profile = %q, want %q", c.Profile, ProfileCalibration)
	}
}

func TestLoadFileConfigSRPControls(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".solidify.yml")
	content := `fail_level: note
disabled_checks: [SOLID-S/high-fan-out-type]
thresholds:
  max_fields: 7
  max_type_lines: 200
  max_func_complexity: 12
  max_type_complexity: 40
  max_fanout: 11
  max_atfd: 4
  min_large_type_signals: 3
  min_tcc_percent: 30
  min_cohesion_methods: 3
  min_cohesion_fields: 2
  min_component_methods: 2
severity:
  SOLID-S/god-type: error
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadFileConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	c.Apply(&cfg)
	if c.FailLevel != SeverityNote || len(cfg.DisabledChecks) != 1 || cfg.MaxFieldsPerType != 7 || cfg.MaxTypeComplexity != 40 || cfg.MaxFanOut != 11 || cfg.MinLargeTypeSignals != 3 || cfg.MinTCCPercent != 30 {
		t.Fatalf("unexpected SRP config: %+v / %+v", c, cfg)
	}
	issues := []Issue{{Rule: RuleSRP, Check: CheckSRPGodType, Severity: SeverityWarning}}
	ApplySeverity(issues, c.Severities)
	if issues[0].Severity != SeverityError {
		t.Fatalf("severity override = %s, want error", issues[0].Severity)
	}
}

func TestLoadFileConfigRejectsInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".solidify.yml")
	if err := os.WriteFile(path, []byte("severity:\n  SOLID-S/design-smell: loud\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFileConfig(path); err == nil {
		t.Fatal("expected invalid config error")
	}
}

func TestLoadFileConfigRejectsUnknownFieldsAndSeverityTargets(t *testing.T) {
	for name, content := range map[string]string{
		"unknown-field":     "enabled_rules: [S]\ntypo: true\n",
		"unknown-severity":  "severity:\n  SOLID-X/new-check: warning\n",
		"conflicting-alias": "enabled_rules: [S]\nenabled-rules: [I]\n",
		"unknown-disabled":  "disabled_checks: [SOLID-X/not-real]\n",
		"unknown-ocp-field": "ocp:\n  typo: true\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".solidify.yml")
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadFileConfig(path); err == nil {
				t.Fatal("expected strict configuration error")
			}
		})
	}
}

func TestLoadFileConfigRejectsLegacyKeyWithCanonicalSuggestion(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".solidify.yml")
	if err := os.WriteFile(path, []byte("enabled-rules: [S]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFileConfig(path)
	if err == nil || !strings.Contains(err.Error(), "enabled_rules") {
		t.Fatalf("legacy key error = %v", err)
	}
}

func TestLoadFileConfigRejectsImpossibleThresholds(t *testing.T) {
	for _, content := range []string{
		"thresholds:\n  isp_usage_ratio_percent: 101\n",
		"thresholds:\n  max_methods: 0\n",
	} {
		path := filepath.Join(t.TempDir(), ".solidify.yml")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadFileConfig(path); err == nil {
			t.Fatal("expected semantic threshold error")
		}
	}
}

func TestLoadFileConfigSemanticErrorsIncludeLineAndSuggestion(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".solidify.yml")
	content := "thresholds:\n  max_methodz: 3\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFileConfig(path)
	if err == nil {
		t.Fatal("expected semantic configuration error")
	}
	errText := err.Error()
	if !strings.Contains(errText, ":2:") || !strings.Contains(errText, "max_methods") {
		t.Fatalf("threshold error = %q", errText)
	}
}

func TestValidateExcludePatternsRejectsMalformedGlobs(t *testing.T) {
	if err := ValidateExcludePatterns([]string{"[invalid"}); err == nil {
		t.Fatal("expected malformed exclude pattern error")
	}
}

func TestApplySeverityPrecedence(t *testing.T) {
	issues := []Issue{{Rule: RuleSRP, Check: CheckSRPGodType, Severity: SeverityNote}}
	ApplySeverity(issues, map[string]Severity{
		"SOLID-S":               SeverityWarning,
		string(CheckSRPGodType): SeverityNote,
	})
	if issues[0].Severity != SeverityNote {
		t.Fatalf("exact check severity was overridden: %s", issues[0].Severity)
	}
}

func TestLoadFileConfigOCPAndArchitecture(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".solidify.yml")
	content := `thresholds:
  ocp_min_dispatch_sites: 3
  ocp_parallel_similarity_percent: 95
ocp:
  discriminator_fields: [Kind, Mode]
  allow_dispatch_types: [error]
  allow_packages: [example.com/parser/**]
architecture:
  logic_packages: [example.com/service/**]
  implementation_packages: [example.com/providers/**]
  composition_roots: [example.com/cmd/**]
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	fileConfig, err := LoadFileConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	fileConfig.Apply(&cfg)
	if cfg.OCPMinDispatchSites != 3 || cfg.OCPParallelSimilarityPercent != 95 {
		t.Fatalf("unexpected OCP thresholds: %+v", cfg)
	}
	if len(cfg.OCPDiscriminatorFields) != 2 || len(cfg.OCPLogicPackages) != 1 || cfg.OCPCompositionRoots[0] != "example.com/cmd/**" {
		t.Fatalf("unexpected OCP lists: %+v", cfg)
	}
}

func TestExcludedDirPrefixPatterns(t *testing.T) {
	patterns := []string{"generated/**", "testdata/**"}
	cases := []struct {
		path     string
		excluded bool
	}{
		{"generated/foo.go", true},
		{"pkg/generated/foo.go", true},
		{"/project/testdata/fixture.go", true},
		{"mygenerated/foo.go", false},
		{"/project/mytestdata/fixture.go", false},
		{"src/regenerated/data.go", false},
	}
	for _, tc := range cases {
		if got := Excluded(tc.path, patterns); got != tc.excluded {
			t.Fatalf("Excluded(%q) = %v, want %v", tc.path, got, tc.excluded)
		}
	}
}

func TestFindConfigForTargets(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, ".solidify.yml")
	if err := os.WriteFile(configPath, []byte("enabled_rules: [D]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "internal", "service")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := FindConfigForTargets([]string{filepath.Join(nested, "...")})
	if err != nil {
		t.Fatal(err)
	}
	if got != configPath {
		t.Fatalf("config = %q, want %q", got, configPath)
	}
}

func TestFindConfigForTargetsRejectsMixedScopes(t *testing.T) {
	configured := t.TempDir()
	if err := os.WriteFile(filepath.Join(configured, ".solidify.yml"), []byte("enabled_rules: [D]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unconfigured := t.TempDir()

	if _, err := FindConfigForTargets([]string{configured, unconfigured}); err == nil {
		t.Fatal("expected mixed configuration scopes to require -config")
	}
}
