package analyzer

import (
	"go/token"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxMethodsPerType != 10 {
		t.Errorf("MaxMethodsPerType = %d, want 10", cfg.MaxMethodsPerType)
	}
	if cfg.MinLargeTypeSignals != 4 {
		t.Errorf("MinLargeTypeSignals = %d, want 4", cfg.MinLargeTypeSignals)
	}
	if cfg.MaxFuncLines != 60 {
		t.Errorf("MaxFuncLines = %d, want 60", cfg.MaxFuncLines)
	}
	if cfg.MaxFuncParams != 8 {
		t.Errorf("MaxFuncParams = %d, want 8", cfg.MaxFuncParams)
	}
	if cfg.MaxTypeSwitchCases != 4 {
		t.Errorf("MaxTypeSwitchCases = %d, want 4", cfg.MaxTypeSwitchCases)
	}
	if cfg.MaxInterfaceMethods != 8 {
		t.Errorf("MaxInterfaceMethods = %d, want 8", cfg.MaxInterfaceMethods)
	}
	if cfg.ISPMinMethods != 3 {
		t.Errorf("ISPMinMethods = %d, want 3", cfg.ISPMinMethods)
	}
	if cfg.ISPUsageRatioPercent != 50 {
		t.Errorf("ISPUsageRatioPercent = %d, want 50", cfg.ISPUsageRatioPercent)
	}
}

func TestIssueString(t *testing.T) {
	is := Issue{
		Rule:     RuleSRP,
		Check:    CheckSRPLargeType,
		Severity: SeverityWarning,
		Pos: token.Position{
			Filename: "foo.go",
			Line:     12,
			Column:   3,
		},
		Message: "too many methods",
	}
	got := is.String()
	want := `foo.go:12:3: [SOLID-S/large-type] too many methods`
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestRuleIDsAndFingerprintsAreStable(t *testing.T) {
	checks := map[Rule]CheckID{RuleSRP: CheckSRPLargeType, RuleOCP: CheckOCPTypeDispatch, RuleLSP: CheckLSPNonExactEOF, RuleISP: CheckISPFatInterface, RuleDIP: CheckDIPConcreteDependency}
	for rule, check := range checks {
		issue := Issue{Rule: rule, Check: check, Subject: "p.Symbol", Identity: "symbol=Symbol", Pos: token.Position{Filename: "fixture.go", Line: 2}, Message: "message", Evidence: "evidence"}
		first := issue.Fingerprint()
		issue.Pos.Line = 99
		if issue.Fingerprint() != first {
			t.Fatalf("%s fingerprint changed after line-only edit", rule)
		}
	}
}

func TestIssueFingerprintUsesPortableAnalysisPath(t *testing.T) {
	left := Issue{Rule: RuleISP, Check: CheckISPFatInterface, Evidence: "methods=9", Pos: token.Position{Filename: "/tmp/checkout-a/pkg/port.go"}, analysisRoot: "/tmp/checkout-a"}
	right := left
	right.Pos.Filename = "/tmp/checkout-b/pkg/port.go"
	right.analysisRoot = "/tmp/checkout-b"
	if left.Fingerprint() != right.Fingerprint() {
		t.Fatalf("portable fingerprints differ: %s != %s", left.Fingerprint(), right.Fingerprint())
	}
}
