package analyzer

import (
	"strings"
	"testing"
)

func calibrationISPConfig() Config {
	cfg := DefaultConfig()
	cfg.Profile = ProfileCalibration
	cfg.ISPUsageRatioPercent = 61
	return cfg
}

func calibrationISPIssues(t *testing.T, source string) []Issue {
	t.Helper()
	fset, files := parseSource(t, source)
	return CheckISPWithTypes(fset, files, typeCheckSource(t, fset, files), calibrationISPConfig(), nil)
}

func TestCheckISPConsumerRole_CalibratedFieldAnalysis(t *testing.T) {
	source := `package p

type SourceStore interface {
	List() error
	Get() error
	Create() error
	Update() error
	Delete() error
	Archive() error
	Schedule() error
	Publish() error
	Refresh() error
	Sync() error
}

type QueueStore interface {
	Enqueue() error
	Create() error
	Cancel() error
	Request() error
	IsStopped() error
	Claim() error
	Renew() error
	Recover() error
	Requeue() error
}

type TraceStore interface {
	ListStages() error
	ListChunks() error
	GetChunks() error
	ListExtractions() error
	StartStage() error
	FinishStage() error
}

type RevisionStore interface {
	Create() error
	Get() error
	GetActive() error
}

type ScannerRegistry interface {
	Get() error
	Register() error
	Unregister() error
	List() error
}

type SourceService struct { sources SourceStore }
func (s *SourceService) List() error { return s.sources.List() }
func (s *SourceService) Get() error { return s.sources.Get() }
func (s *SourceService) Create() error { return s.sources.Create() }
func (s *SourceService) Update() error { return s.sources.Update() }
func (s *SourceService) Delete() error { return s.sources.Delete() }
func (s *SourceService) Archive() error { return wrap(s.sources.Archive()) }
func wrap(err error) error { return err }

type TraceService struct { trace TraceStore }
func (s *TraceService) Read() error {
	if err := s.trace.ListStages(); err != nil { return err }
	if err := s.trace.ListChunks(); err != nil { return err }
	if err := s.trace.GetChunks(); err != nil { return err }
	return s.trace.ListExtractions()
}

type writer struct { trace TraceStore }
func (s *writer) Write() error {
	if err := s.trace.StartStage(); err != nil { return err }
	return s.trace.FinishStage()
}

type worker struct { queue QueueStore }
func (s *worker) Work() error {
	if err := s.queue.Claim(); err != nil { return err }
	return s.queue.Recover()
}

type executor struct { revisions RevisionStore }
func (s *executor) Read() error {
	if err := s.revisions.Get(); err != nil { return err }
	return s.revisions.GetActive()
}

type collector struct { registry ScannerRegistry }
func (s *collector) Collect() error { return s.registry.Get() }
`
	issues := issuesWithCheck(calibrationISPIssues(t, source), CheckISPConsumerRole)
	want := map[string]Severity{
		"SourceService.sources": SeverityNote,
		"TraceService.trace":    SeverityWarning,
		"writer.trace":          SeverityWarning,
		"worker.queue":          SeverityWarning,
	}
	if len(issues) != len(want) {
		t.Fatalf("consumer-role issues = %d, want %d: %v", len(issues), len(want), issues)
	}
	for _, issue := range issues {
		field := issueIdentityField(issue.Evidence)
		severity, ok := want[field]
		if !ok {
			t.Fatalf("unexpected consumer-role issue: %+v", issue)
		}
		if issue.Severity != severity {
			t.Errorf("%s severity = %s, want %s", field, issue.Severity, severity)
		}
		if field == "SourceService.sources" && !strings.Contains(issue.Evidence, "methods=Archive,Create,Delete,Get,List,Update") {
			t.Errorf("nested field method was not collected: %q", issue.Evidence)
		}
	}
}

func TestCheckISPConsumerRole_UsesIndependentStableIdentities(t *testing.T) {
	source := `package p

type Store interface { Get() error; Save() error; Delete() error; Archive() error }
type First struct { store Store }
func (s *First) Use() error { return s.store.Get() }
type Second struct { store Store }
func (s *Second) Use() error { return s.store.Get() }
`
	issues := issuesWithCheck(calibrationISPIssues(t, source), CheckISPConsumerRole)
	if err := FinalizeIssues(issues, "example.com/p"); err != nil {
		t.Fatal(err)
	}
	if len(issues) != 2 {
		t.Fatalf("consumer-role issues = %d, want 2: %v", len(issues), issues)
	}
	seen := map[string]bool{}
	for _, issue := range issues {
		if !strings.Contains(issue.Identity, "interface=p.Store") || !strings.Contains(issue.Identity, "field=store") {
			t.Fatalf("identity misses interface or field: %+v", issue)
		}
		if seen[issue.Fingerprint()] {
			t.Fatalf("duplicate fingerprint: %v", issues)
		}
		seen[issue.Fingerprint()] = true
	}
}

func TestCheckISPUnusedDependency_FollowsOnlyUnreadFieldFlows(t *testing.T) {
	source := `package p

type Store interface { Get() error; Save() error }

type Ports struct {
	Direct Store
	Chained Store
	Used Store
}

type Runner struct {
	chained Store
	used Store
}

func NewRunner(ports Ports) *Runner {
	return &Runner{chained: ports.Chained, used: ports.Used}
}

func (r *Runner) Use() error { return r.used.Get() }

type Bundle struct { Unused Store }
type RefreshStores struct { Unused Store }
`
	issues := issuesWithCheck(calibrationISPIssues(t, source), CheckISPUnusedDependency)
	if len(issues) != 2 {
		t.Fatalf("unused-dependency issues = %d, want 2: %v", len(issues), issues)
	}
	seen := map[string]bool{}
	for _, issue := range issues {
		seen[issueIdentityField(issue.Evidence)] = true
	}
	if !seen["Ports.Direct"] || !seen["Ports.Chained"] {
		t.Fatalf("unused dependency fields = %v, want Ports.Direct and Ports.Chained", seen)
	}
	if seen["Ports.Used"] || seen["Bundle.Unused"] || seen["RefreshStores.Unused"] {
		t.Fatalf("wiring or consumed dependency was reported: %v", seen)
	}
}

func issueIdentityField(evidence string) string {
	_, fields := parseEvidenceIdentity(evidence)
	return fields["type"] + "." + fields["field"]
}
