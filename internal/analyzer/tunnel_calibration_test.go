package analyzer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type tunnelManifest struct {
	Counts        map[string]int `json:"counts"`
	SchemaVersion int            `json:"schemaVersion"`
	Sites         []tunnelSite   `json:"sites"`
}
type tunnelSite struct {
	Symbol  string          `json:"symbol"`
	ID      string          `json:"id"`
	Family  string          `json:"family"`
	Expect  string          `json:"expect"`
	Role    string          `json:"role"`
	Check   json.RawMessage `json:"check"`
	Fixture struct {
		Positive string `json:"positive"`
		Negative string `json:"negative"`
	} `json:"fixture"`
	FixtureEvidence struct {
		Methods   []string `json:"methods"`
		Broad     []string `json:"broad"`
		Narrow    []string `json:"narrow"`
		Parameter string   `json:"parameter"`
	} `json:"fixtureEvidence"`
}

func readTunnelManifest(t *testing.T) tunnelManifest {
	t.Helper()
	data, err := os.ReadFile(testdataDir(t, "tunnel_calibration/manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m tunnelManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}
func (s tunnelSite) checks() []CheckID {
	var one CheckID
	if json.Unmarshal(s.Check, &one) == nil {
		return []CheckID{one}
	}
	var all []CheckID
	_ = json.Unmarshal(s.Check, &all)
	return all
}
func tunnelCorpusConfig(check CheckID) Config {
	cfg := tunnelAllConfig()
	for _, id := range RegisteredCheckIDs() {
		if id != check {
			cfg.DisabledChecks = append(cfg.DisabledChecks, id)
		}
	}
	cfg.ISPExecutionMethods = []string{"TryClaimRuleUse"}
	cfg.OCPLogicPackages = []string{"example.com/tunnelcalibration/positive/*", "example.com/tunnelcalibration/negative/*"}
	cfg.OCPImplementationPackages = []string{"example.com/tunnelcalibration/dep"}
	cfg.DIPDetailImports = []string{"database/sql", "net/http"}
	return cfg
}
func TestTunnelCalibrationManifest(t *testing.T) {
	m := readTunnelManifest(t)
	if m.SchemaVersion != 1 {
		t.Fatal("unexpected manifest schema")
	}
	pinned := map[string]int{"CP-01": 3, "CP-02": 3, "CP-03": 2, "CP-04": 4, "CP-05": 15, "CP-06": 3, "CP-07": 18, "CP-08": 5, "CP-09": 4, "CP-10": 2, "CP-11": 2, "CP-12": 9, "CP-13": 5, "CP-14": 6, "CP-15": 5, "CP-16": 2, "CP-17": 3, "CP-18": 2, "CN-01": 6, "CN-02": 3, "CN-03": 106, "CN-04": 22, "CN-05": 1, "CN-06": 5, "CN-07": 2, "CN-08": 1, "CN-09": 4, "CN-10": 11}
	actual := map[string]int{}
	primary, clusters, controls := 0, 0, 0
	for _, site := range m.Sites {
		actual[site.Family]++
		if site.Expect == "present" && site.Role == "primary" {
			primary++
		}
		if site.Expect == "present" && site.Role == "cluster-member" {
			clusters++
		}
		if site.Expect == "absent" && site.Role == "primary" {
			controls++
		}
	}
	if !reflect.DeepEqual(actual, pinned) || !reflect.DeepEqual(m.Counts, pinned) || primary != 70 || clusters != 11 || controls != 52 {
		t.Fatalf("review inventory changed: families=%v primary=%d clusters=%d controls=%d", actual, primary, clusters, controls)
	}
	families := map[string]bool{}
	ids := map[string]bool{}
	packages, _, err := LoadWorkspace([]string{testdataDir(t, "tunnel_calibration/corpus")}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	grouped := map[CheckID][]Issue{}
	for _, site := range m.Sites {
		families[site.Family] = true
		if site.Role == "primary" || site.Role == "cluster-member" {
			checks := site.checks()
			if len(checks) == 0 {
				t.Fatalf("missing check IDs for %s", site.ID)
			}
			for _, check := range checks {
				if _, known := CheckMetadata(check); !known {
					t.Fatalf("unknown check %s for %s", check, site.ID)
				}
			}
		}
		if ids[site.ID] {
			t.Fatalf("duplicate site %s", site.ID)
		}
		ids[site.ID] = true
		if site.Role != "primary" && site.Role != "cluster-member" {
			continue
		}
		if site.Fixture.Negative == "" {
			t.Fatalf("missing corrected/control fixture %s", site.ID)
		}
		for _, check := range site.checks() {
			if _, loaded := grouped[check]; !loaded {
				grouped[check] = Run(packages, tunnelCorpusConfig(check), allRulesEnabled())
			}
		}
		if site.Expect == "present" {
			if site.Fixture.Positive == "" {
				t.Fatalf("missing positive %s", site.ID)
			}
			for _, check := range site.checks() {
				issues := tunnelFixtureIssues(grouped[check], site.Fixture.Positive, check)
				if len(issues) == 0 {
					t.Errorf("%s missing %s at %s", site.ID, check, site.Fixture.Positive)
					continue
				}
				for _, method := range site.FixtureEvidence.Methods {
					if !strings.Contains(issues[0].Evidence, method) {
						t.Errorf("%s lacks method %s: %s", site.ID, method, issues[0].Evidence)
					}
				}
				if check == CheckISPConstructorRole {
					for _, method := range append(site.FixtureEvidence.Broad, site.FixtureEvidence.Narrow...) {
						if !strings.Contains(issues[0].Evidence, method) {
							t.Errorf("%s missing constructor method %s", site.ID, method)
						}
					}
					if len(issues[0].Related) == 0 {
						t.Errorf("%s missing stored field location", site.ID)
					}
				}
			}
		}
		for _, check := range site.checks() {
			if site.Family == "CN-08" {
				cfg := tunnelCorpusConfig(check)
				cfg.OCPCompositionRoots = []string{"example.com/tunnelcalibration/" + site.Fixture.Negative}
				issues := Run(packages, cfg, allRulesEnabled())
				if bad := tunnelFixtureIssues(issues, site.Fixture.Negative, check); len(bad) > 0 {
					t.Errorf("explicit root %s flagged: %v", site.ID, bad)
				}
				continue
			}
			if bad := tunnelNegativeIssues(tunnelFixtureIssues(grouped[check], site.Fixture.Negative, check), site); len(bad) > 0 {
				t.Errorf("corrected/control %s flagged: %v", site.ID, bad)
			}
		}
	}
	if len(families) != 28 {
		t.Fatalf("covered %d families, want28", len(families))
	}
}
func tunnelFixtureIssues(issues []Issue, fixture string, check CheckID) []Issue {
	var out []Issue
	needle := filepath.FromSlash("/" + fixture + "/")
	for _, issue := range issues {
		matches := strings.Contains(issue.Pos.Filename, needle)
		if check == CheckOCPDiscriminatorDispatch {
			for _, related := range issue.Related {
				matches = matches || strings.Contains(related.Pos.Filename, needle)
			}
		}
		if issue.Check == check && matches {
			out = append(out, issue)
		}
	}
	return out
}
func TestTunnelCalibrationControls(t *testing.T) { TestTunnelCalibrationManifest(t) }
func TestTunnelCalibrationCacheParity(t *testing.T) {
	packages, _, err := LoadWorkspace([]string{testdataDir(t, "tunnel_calibration/corpus")}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	cfg := tunnelAllConfig()
	cfg.ISPExecutionMethods = []string{"TryClaimRuleUse"}
	cfg.CacheEnabled = false
	uncached := Run(packages, cfg, allRulesEnabled())
	cfg.CacheDir = t.TempDir()
	cfg.CacheEnabled = true
	first := Run(packages, cfg, allRulesEnabled())
	second := Run(packages, cfg, allRulesEnabled())
	if !reflect.DeepEqual(uncached, first) || !reflect.DeepEqual(first, second) {
		t.Fatal("cached and uncached semantic diagnostics differ")
	}
	cfg.ISPExecutionMethods = nil
	changed := Run(packages, cfg, allRulesEnabled())
	if reflect.DeepEqual(changed, second) {
		t.Fatal("execution-method policy did not invalidate cached findings")
	}
	for _, issue := range second {
		if issue.Subject == "" || issue.Identity == "" || len(issue.Fingerprint()) != 64 {
			t.Fatalf("missing stable identity: %+v", issue)
		}
	}
}

func tunnelNegativeIssues(issues []Issue, site tunnelSite) []Issue {
	if site.Family != "CN-04" {
		return issues
	}
	var out []Issue
	for _, issue := range issues {
		_, fields := parseEvidenceIdentity(issue.Evidence)
		if fields["field"] == site.Symbol {
			out = append(out, issue)
		}
	}
	return out
}
