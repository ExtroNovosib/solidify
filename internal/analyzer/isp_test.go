package analyzer

import (
	"go/ast"
	"strings"
	"testing"
)

func TestCheckISP_FatInterface(t *testing.T) {
	src := `package p

type Machine interface {
	A()
	B()
	C()
	D()
	E()
	F()
}
`
	fset, files := parseSource(t, src)
	cfg := DefaultConfig()
	cfg.MaxInterfaceMethods = 5

	issues := CheckISP(fset, files, cfg)
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1", len(issues))
	}
	if issues[0].Rule != RuleISP {
		t.Errorf("rule = %q, want SOLID-I", issues[0].Rule)
	}
	if issues[0].Check != CheckISPFatInterface {
		t.Errorf("check = %q, want %q", issues[0].Check, CheckISPFatInterface)
	}
	if !strings.Contains(issues[0].Message, `interface "Machine" declares 6 methods`) {
		t.Errorf("unexpected message: %s", issues[0].Message)
	}
}

func TestCheckISP_ImportedEmbeddedAndGenericInterfaces(t *testing.T) {
	src := `package p
import "io"
type Local[T any] interface { Get() T }
type Combined interface { io.Reader; Local[string]; Read([]byte) (int, error); Close() error }
`
	fset, files := parseSource(t, src)
	info := typeCheckSource(t, fset, files)
	cfg := DefaultConfig()
	cfg.MaxInterfaceMethods = 2
	issues := CheckISPWithTypes(fset, files, info, cfg, nil)
	if len(issues) != 1 || !strings.Contains(issues[0].Message, `interface "Combined" declares 3 methods`) {
		t.Fatalf("unexpected issues: %v", issues)
	}
}

func TestCheckISP_EmbeddedInterfacesCounted(t *testing.T) {
	src := `package p

type Reader interface { Read() }
type Writer interface { Write() }
type RW interface {
	Reader
	Writer
	A()
	B()
	C()
}
`
	fset, files := parseSource(t, src)
	cfg := DefaultConfig()
	cfg.MaxInterfaceMethods = 2

	issues := CheckISP(fset, files, cfg)
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1", len(issues))
	}
	if !strings.Contains(issues[0].Message, `interface "RW" declares 5 methods`) {
		t.Errorf("unexpected message: %s", issues[0].Message)
	}
}

func TestCheckISP_AggregateOfFatInterfaceIsNote(t *testing.T) {
	src := `package p

type BusinessStore interface {
	A()
	B()
	C()
}

type WiringRepository interface {
	BusinessStore
	D()
}
`
	fset, files := parseSource(t, src)
	cfg := DefaultConfig()
	cfg.MaxInterfaceMethods = 2
	issues := CheckISPWithTypes(fset, files, typeCheckSource(t, fset, files), cfg, nil)
	if len(issues) != 2 {
		t.Fatalf("got %d issues, want business and aggregate findings: %v", len(issues), issues)
	}
	if issues[0].Severity != SeverityWarning || issues[1].Severity != SeverityNote {
		t.Fatalf("severities = %s, %s; want warning, note", issues[0].Severity, issues[1].Severity)
	}
	if !strings.Contains(issues[1].Evidence, "aggregate=true") {
		t.Fatalf("aggregate evidence missing: %+v", issues[1])
	}
}

func TestCheckISP_UsageRatio_LowUsageReported(t *testing.T) {
	src := `package p

type Store interface {
	Get(string) (string, error)
	Save(string) error
	Delete(string) error
}

func SaveOnly(s Store) error {
	return s.Save("x")
}
`
	fset, files := parseSource(t, src)
	info := typeCheckSource(t, fset, files)
	issues := CheckISPWithTypes(fset, files, info, DefaultConfig(), nil)
	var found bool
	for _, issue := range issues {
		if issue.Check == CheckISPUsageRatio {
			found = true
			if !strings.Contains(issue.Message, `only uses 1`) {
				t.Errorf("unexpected message: %s", issue.Message)
			}
		}
	}
	if !found {
		t.Fatalf("expected usage-ratio issue, got %v", issues)
	}
}

func TestCheckISP_UsageRatio_InterfaceFieldReported(t *testing.T) {
	src := `package p

type Store interface {
	Get() error
	Save() error
	Delete() error
	List() error
}

type Service struct {
	store Store
}

func (s *Service) Save() error {
	return s.store.Save()
}
`
	fset, files := parseSource(t, src)
	info := typeCheckSource(t, fset, files)
	issues := issuesWithCheck(CheckISPWithTypes(fset, files, info, DefaultConfig(), nil), CheckISPUsageRatio)
	if len(issues) != 1 {
		t.Fatalf("got %d usage-ratio issues, want field finding: %v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Evidence, "type=Service;field=store;used=1;total=4;methods=Save") {
		t.Fatalf("unexpected field usage evidence: %q", issues[0].Evidence)
	}
}

func TestCheckISP_UsageRatio_InterfaceFieldFollowsLocalHelper(t *testing.T) {
	src := `package p

type Store interface {
	Get() error
	Save() error
	Delete() error
	List() error
}

type deleter interface {
	Delete() error
}

type Service struct {
	store Store
}

func consume(store deleter) error {
	return store.Delete()
}

func (s *Service) Delete() error {
	return consume(s.store)
}
`
	fset, files := parseSource(t, src)
	info := typeCheckSource(t, fset, files)
	issues := issuesWithCheck(CheckISPWithTypes(fset, files, info, DefaultConfig(), nil), CheckISPUsageRatio)
	if len(issues) != 1 {
		t.Fatalf("got %d usage-ratio issues, want helper-propagated field finding: %v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Evidence, "type=Service;field=store;used=1;total=4;methods=Delete") {
		t.Fatalf("local helper usage was not propagated: %q", issues[0].Evidence)
	}
}

func TestCheckISP_UsageRatio_LogicPackageFilter(t *testing.T) {
	src := `package p

type Store interface {
	Get() error
	Save() error
	Delete() error
}

func SaveOnly(store Store) error {
	return store.Save()
}
`
	fset, files := parseSource(t, src)
	info := typeCheckSource(t, fset, files)
	cfg := DefaultConfig()
	cfg.OCPLogicPackages = []string{"example.com/app/internal/*/application/**"}

	adapter := &packageFiles{pkgPath: "example.com/app/internal/auth/adapters/http", modulePath: "p"}
	if issues := CheckISPWithTypes(fset, files, info, cfg, adapter); hasCheck(issues, CheckISPUsageRatio) {
		t.Fatalf("adapter package should be excluded by logic package filter: %v", issues)
	}
	application := &packageFiles{pkgPath: "example.com/app/internal/auth/application", modulePath: "p"}
	if issues := CheckISPWithTypes(fset, files, info, cfg, application); !hasCheck(issues, CheckISPUsageRatio) {
		t.Fatalf("application package should retain usage-ratio analysis: %v", issues)
	}
}

func TestCheckISP_UsageRatio_UnexportedFunctionsAndMethodsIgnored(t *testing.T) {
	src := `package p

type Store interface {
	Get(string) (string, error)
	Save(string) error
	Delete(string) error
}

func saveOnly(s Store) error {
	return s.Save("x")
}

type Consumer struct{}

func (Consumer) UseStore(s Store) error {
	return s.Save("x")
}

func (Consumer) useStore(s Store) error {
	return s.Save("x")
}
`
	fset, files := parseSource(t, src)
	issues := CheckISPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
	usage := issuesWithCheck(issues, CheckISPUsageRatio)
	if len(usage) != 1 {
		t.Fatalf("got %d usage-ratio issues, want only the exported method: %v", len(usage), usage)
	}
	if !strings.Contains(usage[0].Evidence, "function=UseStore") {
		t.Fatalf("unexpected usage-ratio issue: %v", usage[0])
	}
}

func TestCheckISP_UsageRatio_CompositionRootsAndEmbeddedInterfaces(t *testing.T) {
	src := `package p

type BaseStore interface {
	Get(string) (string, error)
	Save(string) error
	Delete(string) error
}

type Store interface {
	BaseStore
	List() error
}

func Consume(store Store) error {
	return store.Save("x")
}
`
	fset, files := parseSource(t, src)
	info := typeCheckSource(t, fset, files)
	cfg := DefaultConfig()
	cfg.OCPCompositionRoots = []string{"example.com/app/cmd/**"}

	rootPkg := &packageFiles{pkgPath: "example.com/app/cmd/server", modulePath: "p"}
	if issues := CheckISPWithTypes(fset, files, info, cfg, rootPkg); hasCheck(issues, CheckISPUsageRatio) {
		t.Fatalf("composition root should skip usage-ratio, got: %v", issues)
	}

	outsidePkg := &packageFiles{pkgPath: "example.com/app/internal/service", modulePath: "p"}
	issues := CheckISPWithTypes(fset, files, info, cfg, outsidePkg)
	usage := issuesWithCheck(issues, CheckISPUsageRatio)
	if len(usage) != 1 {
		t.Fatalf("got %d usage-ratio issues outside the composition root, want 1: %v", len(usage), issues)
	}
	if !strings.Contains(usage[0].Evidence, "function=Consume;parameter=store;used=1;total=4") {
		t.Fatalf("embedded interface methods were not counted in usage-ratio evidence: %q", usage[0].Evidence)
	}
}

func TestCheckISP_UsageRatio_ExternalInterfaceIgnored(t *testing.T) {
	src := `package p

import "io"

func Consume(stream io.ReadWriteCloser) error {
	return stream.Close()
}
`
	fset, files := parseSource(t, src)
	pkg := &packageFiles{pkgPath: "example.com/app/internal/service", modulePath: "example.com/app"}
	issues := CheckISPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), pkg)
	if hasCheck(issues, CheckISPUsageRatio) {
		t.Fatalf("external interface should not trigger usage-ratio: %v", issues)
	}
}

func TestCheckISP_UsageRatio_IndirectUseIgnored(t *testing.T) {
	src := `package p

type Store interface {
	Get(string) (string, error)
	Save(string) error
	Delete(string) error
}

func Delegate(s Store) error {
	return Forward(s)
}

func Forward(s Store) error {
	return s.Save("x")
}
`
	fset, files := parseSource(t, src)
	info := typeCheckSource(t, fset, files)
	issues := CheckISPWithTypes(fset, files, info, DefaultConfig(), nil)
	for _, issue := range issues {
		if issue.Check == CheckISPUsageRatio && strings.Contains(issue.Evidence, "function=Delegate") {
			t.Fatalf("unexpected usage-ratio on delegated parameter: %v", issue)
		}
	}
}

func TestCheckISP_UsageRatio_MethodValueCounts(t *testing.T) {
	src := `package p

type Store interface {
	Get(string) (string, error)
	Save(string) error
	Delete(string) error
}

func BindSave(s Store) func(string) error {
	return s.Save
}
`
	fset, files := parseSource(t, src)
	info := typeCheckSource(t, fset, files)
	issues := CheckISPWithTypes(fset, files, info, DefaultConfig(), nil)
	for _, issue := range issues {
		if issue.Check == CheckISPUsageRatio && strings.Contains(issue.Evidence, "function=BindSave") {
			return
		}
	}
	t.Fatalf("expected usage-ratio for method value binding, got %v", issues)
}

func TestCheckISP_UsageRatio_SmallInterfaceIgnored(t *testing.T) {
	src := `package p

type Reader interface { Read() error }

func ReadOnly(r Reader) error { return r.Read() }
`
	fset, files := parseSource(t, src)
	info := typeCheckSource(t, fset, files)
	issues := CheckISPWithTypes(fset, files, info, DefaultConfig(), nil)
	for _, issue := range issues {
		if issue.Check == CheckISPUsageRatio {
			t.Fatalf("unexpected usage-ratio on small interface: %v", issue)
		}
	}
}

func TestCheckISP_Stub_Panic(t *testing.T) {
	src := `package p

type RO struct{}
type Store interface {
	Get(string) (string, error)
	Save(string) error
	Delete(string) error
}

func (r *RO) Get(string) (string, error) { return "", nil }
func (r *RO) Delete(string) error { return nil }

func (r *RO) Save(v string) error {
	panic("not supported")
}
`
	fset, files := parseSource(t, src)
	issues := CheckISPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1", len(issues))
	}
	if issues[0].Check != CheckISPStubImplementation {
		t.Errorf("check = %q, want %q", issues[0].Check, CheckISPStubImplementation)
	}
	if !strings.Contains(issues[0].Message, `method "Save"`) {
		t.Errorf("unexpected message: %s", issues[0].Message)
	}
}

func TestQualifyingInterfaceForMethod_PrefersFattestInterface(t *testing.T) {
	src := `package p

type RO struct{}
type Reader interface {
	Get(string) (string, error)
	Save(string) error
	Delete(string) error
}
type Store interface {
	Get(string) (string, error)
	Save(string) error
	Delete(string) error
	List() error
}

func (r *RO) Get(string) (string, error) { return "", nil }
func (r *RO) Delete(string) error { return nil }
func (r *RO) List() error { return nil }

func (r *RO) Save(v string) error {
	panic("not supported")
}
`
	fset, files := parseSource(t, src)
	info := typeCheckSource(t, fset, files)
	fn := files[0].Decls[len(files[0].Decls)-1].(*ast.FuncDecl)
	ifaceName, ok := qualifyingInterfaceForMethod(fn, info, DefaultConfig().ISPMinMethods)
	if !ok {
		t.Fatal("expected qualifying interface")
	}
	if ifaceName != "Store" {
		t.Fatalf("interface = %q, want Store", ifaceName)
	}
}

func TestCheckISP_Stub_ErrUnsupported(t *testing.T) {
	src := `package p

import "errors"

type RO struct{}
type Store interface {
	Get(string) (string, error)
	Save(string) error
	Delete(string) error
}

func (r *RO) Get(string) (string, error) { return "", nil }
func (r *RO) Save(string) error { return nil }

func (r *RO) Delete(id string) error {
	return errors.ErrUnsupported
}
`
	fset, files := parseSource(t, src)
	issues := CheckISPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1", len(issues))
	}
	if !strings.Contains(issues[0].Message, `returns errors.ErrUnsupported`) {
		t.Errorf("unexpected message: %s", issues[0].Message)
	}
}

func TestCheckISP_Stub_ErrUnsupportedWrapped(t *testing.T) {
	src := `package p

import (
	"errors"
	"fmt"
)

type RO struct{}
type Store interface {
	Get(string) (string, error)
	Save(string) error
	Delete(string) error
}

func (r *RO) Get(string) (string, error) { return "", nil }
func (r *RO) Save(string) error { return nil }

func (r *RO) Delete(id string) error {
	return fmt.Errorf("delete: %w", errors.ErrUnsupported)
}
`
	fset, files := parseSource(t, src)
	issues := CheckISPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1", len(issues))
	}
}

func TestCheckISP_Stub_SmallInterfaceIgnored(t *testing.T) {
	src := `package p

type RO struct{}
type Saver interface { Save(string) error }

func (r *RO) Save(v string) error {
	panic("not supported")
}
`
	fset, files := parseSource(t, src)
	issues := CheckISPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
	if len(issues) != 0 {
		t.Fatalf("got %d issues, want 0", len(issues))
	}
}

func TestCheckISP_Stub_MultiStatementIgnored(t *testing.T) {
	src := `package p

import "errors"

type RO struct{}
type Store interface {
	Get(string) (string, error)
	Save(string) error
	Delete(string) error
}

func (r *RO) Delete(id string) error {
	_ = id
	return errors.ErrUnsupported
}
`
	fset, files := parseSource(t, src)
	issues := CheckISPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
	if len(issues) != 0 {
		t.Fatalf("got %d issues, want 0", len(issues))
	}
}

func TestCheckISP_DeprecatedFatInterfaceIsNote(t *testing.T) {
	src := `package p

// DEPRECATED as a dependency — migrate callers to Narrower.
type LegacyStore interface {
	A()
	B()
	C()
	D()
	E()
	F()
	G()
	H()
	I()
}
`
	fset, files := parseSource(t, src)
	issues := CheckISPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1", len(issues))
	}
	if issues[0].Severity != SeverityNote {
		t.Fatalf("severity = %q, want note", issues[0].Severity)
	}
}

func TestCheckISP_StandardDeprecatedMarkerIsNote(t *testing.T) {
	src := `package p

// Deprecated: use Narrower instead.
type LegacyStore interface {
	A()
	B()
	C()
	D()
	E()
	F()
	G()
	H()
	I()
}
`
	fset, files := parseSource(t, src)
	issues := CheckISPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1", len(issues))
	}
	if issues[0].Severity != SeverityNote {
		t.Fatalf("severity = %q, want note", issues[0].Severity)
	}
}

func TestCheckISP_ContextUsageRatioIgnored(t *testing.T) {
	src := `package p

import "context"

func Handle(ctx context.Context) {
	_ = ctx.Done()
}
`
	fset, files := parseSource(t, src)
	issues := CheckISPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
	for _, issue := range issues {
		if issue.Check == CheckISPUsageRatio {
			t.Fatalf("context.Context should not trigger usage-ratio: %+v", issue)
		}
	}
}

func TestCheckISP_ResponseWriterUsageRatioIgnored(t *testing.T) {
	src := `package p

import "net/http"

func Handle(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain")
}
`
	fset, files := parseSource(t, src)
	issues := CheckISPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
	for _, issue := range issues {
		if issue.Check == CheckISPUsageRatio {
			t.Fatalf("http.ResponseWriter should not trigger usage-ratio: %+v", issue)
		}
	}
}

func TestReturnsErrUnsupported(t *testing.T) {
	fset, files := parseSource(t, `package p

import (
	"errors"
	"fmt"
)

func f() error { return fmt.Errorf("x: %w", errors.ErrUnsupported) }
`)
	info := typeCheckSource(t, fset, files)
	fn := files[0].Decls[1].(*ast.FuncDecl)
	ret := fn.Body.List[0].(*ast.ReturnStmt)
	if !returnsErrUnsupported(ret.Results[0], info) {
		t.Error("returnsErrUnsupported = false, want true")
	}
	_ = fset
}
