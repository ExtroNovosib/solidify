package analyzer

import (
	"go/ast"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func findPackageP(t *testing.T, pkgs []*packageFiles) *packageFiles {
	t.Helper()
	for _, pkg := range pkgs {
		if pkg.pkgName == "p" {
			return pkg
		}
	}
	t.Fatalf("package %q not found in %#v", "p", pkgs)
	return nil
}

func TestPointerToIdent(t *testing.T) {
	fset, files := parseSource(t, `package p

type Mailer struct{}
type Order struct { m *Mailer }
`)
	st := files[0].Decls[1].(*ast.GenDecl).Specs[0].(*ast.TypeSpec).Type.(*ast.StructType)
	field := st.Fields.List[0].Type

	name, isPtr := pointerToIdent(field)
	if name != "Mailer" || !isPtr {
		t.Errorf("pointerToIdent(*Mailer) = (%q, %v), want (Mailer, true)", name, isPtr)
	}
	_ = fset
}

func TestPointerToIdent_GenericLocalType(t *testing.T) {
	fset, files := parseSource(t, `package p

type Box[T any] struct{}
type Order struct { box *Box[string] }
`)
	st := files[0].Decls[1].(*ast.GenDecl).Specs[0].(*ast.TypeSpec).Type.(*ast.StructType)
	name, pointer := pointerToIdent(st.Fields.List[0].Type)
	if name != "Box" || !pointer {
		t.Errorf("pointerToIdent(*Box[string]) = (%q, %v), want (Box, true)", name, pointer)
	}
	_ = fset
}

func TestCheckDIP_SamePackageFieldIgnored(t *testing.T) {
	src := `package p

type SmtpMailer struct{}
type Order struct {
	mailer *SmtpMailer
}
`
	fset, files := parseSource(t, src)
	issues := CheckDIP(fset, files, DefaultConfig())
	if len(issues) != 0 {
		t.Fatalf("got %d issues, want 0 for same-package wiring: %v", len(issues), issues)
	}
}

func TestCheckDIP_InterfaceDependencyIgnored(t *testing.T) {
	src := `package p

type Notifier interface { Notify(string) error }
type EmailNotifier struct{}
func (e *EmailNotifier) Notify(msg string) error { return nil }
type Order struct { notifier Notifier }
`
	fset, files := parseSource(t, src)
	issues := CheckDIP(fset, files, DefaultConfig())
	if len(issues) != 0 {
		t.Fatalf("got %d issues, want 0: %v", len(issues), issues)
	}
}

func TestCheckDIP_ExternalTypeIgnored(t *testing.T) {
	src := `package p

import "bytes"

type S struct { buf *bytes.Buffer }
`
	fset, files := parseSource(t, src)
	issues := CheckDIP(fset, files, DefaultConfig())
	if len(issues) != 0 {
		t.Fatalf("got %d issues, want 0", len(issues))
	}
}

func TestCheckDIPWithTypes_ImportedConcreteAndGeneric(t *testing.T) {
	src := `package p
import "bytes"
type Box[T any] struct { value T }
type S struct { buf *bytes.Buffer; box *Box[string] }
`
	fset, files := parseSource(t, src)
	issues := CheckDIPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
	if len(issues) != 0 {
		t.Fatalf("got %d issues, want 0 (stdlib and same-package wiring ignored): %v", len(issues), issues)
	}
}

func TestCheckDIPWithTypes_StdlibFieldIgnored(t *testing.T) {
	src := `package p
import "net"
type Flow struct { conn *net.UDPConn; addr *net.UDPAddr }
`
	fset, files := parseSource(t, src)
	issues := CheckDIPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
	if len(issues) != 0 {
		t.Fatalf("got %d issues, want stdlib concrete fields ignored: %v", len(issues), issues)
	}
}

func TestCheckDIPWithTypes_SerializedDataCarrierIgnored(t *testing.T) {
	src := "package p\n" +
		"type RateLimitGlobal struct{}\n" +
		"type RateLimits struct {\n" +
		"    Enabled bool `json:\"enabled\"`\n" +
		"    TCPIngress *RateLimitGlobal `json:\"tcp_ingress\"`\n" +
		"}\n"
	fset, files := parseSource(t, src)
	issues := CheckDIPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
	if len(issues) != 0 {
		t.Fatalf("serialized data carrier should not trigger field DIP: %v", issues)
	}
}

func TestCheckDIPWithTypes_BehavioralSerializedTypeStillFlags(t *testing.T) {
	src := "package p\n" +
		"type Driver struct{}\n" +
		"type Service struct {\n" +
		"    Driver *Driver `json:\"driver\"`\n" +
		"    Name string `json:\"name\"`\n" +
		"    Mode string `json:\"mode\"`\n" +
		"    Retries int `json:\"retries\"`\n" +
		"}\n" +
		"func (s *Service) Start() { work(s.Driver) }\n" +
		"func (s *Service) Stop() { work(s.Driver) }\n" +
		"func work(any) {}\n"
	fset, files := parseSource(t, src)
	issues := CheckDIPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
	if len(issues) != 0 {
		t.Fatalf("same-package serialized behavioral type should not trigger field DIP: %v", issues)
	}
}

func TestCheckDIPWithTypes_CompositionRootSuppressesFieldDIP(t *testing.T) {
	src := `package p
type A struct{}
type B struct{}
type C struct{}
type D struct{}
type E struct{}
type Root struct {
	a *A
	b *B
	c *C
	d *D
	e *E
}
`
	fset, files := parseSource(t, src)
	issues := CheckDIPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
	for _, issue := range issues {
		if issue.Rule == RuleDIP && strings.Contains(issue.Message, "Root.") {
			t.Fatalf("composition root should suppress field DIP: %v", issues)
		}
	}
}

func TestCheckDIPWithTypes_ConfigFieldIgnored(t *testing.T) {
	src := `package p
type TunnelConfig struct { Host string }
type Runtime struct { cfg *TunnelConfig }
`
	fset, files := parseSource(t, src)
	issues := CheckDIPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
	if len(issues) != 0 {
		t.Fatalf("config data bag should not trigger field DIP: %v", issues)
	}
}

func TestCheckDIPWithTypes_SmallStructStillFlagsDIP(t *testing.T) {
	src := `package p
type Driver struct{}
type Service struct { driver *Driver }
`
	fset, files := parseSource(t, src)
	issues := CheckDIPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
	if len(issues) != 0 {
		t.Fatalf("same-package field wiring should not trigger DIP: %v", issues)
	}
}

func TestCheckDIPWithTypes_ExplicitlyExposedConcreteFieldIgnored(t *testing.T) {
	src := `package p
type Manager struct{}
type Server struct { manager *Manager }
func (s *Server) Manager() *Manager { return s.manager }
`
	fset, files := parseSource(t, src)
	issues := CheckDIPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
	if len(issues) != 0 {
		t.Fatalf("concrete field deliberately exposed by the owner API should not trigger DIP: %v", issues)
	}
}

func TestCheckDIPWithTypes_SamePackageConstructorIgnored(t *testing.T) {
	src := `package p
type Manager struct{}
func NewService(m *Manager) *Manager { return m }
`
	fset, files := parseSource(t, src)
	info := typeCheckSource(t, fset, files)
	if issues := CheckDIPWithTypes(fset, files, info, DefaultConfig(), nil); len(issues) != 0 {
		t.Fatalf("same-package constructor should not trigger DIP: %v", issues)
	}
}

func TestCheckDIPWithTypes_ConfigConstructorIgnored(t *testing.T) {
	src := `package p
type TunnelConfig struct { Host string }
func NewRuntime(cfg *TunnelConfig) *TunnelConfig { return cfg }
`
	fset, files := parseSource(t, src)
	info := typeCheckSource(t, fset, files)
	if issues := CheckDIPWithTypes(fset, files, info, DefaultConfig(), nil); len(issues) != 0 {
		t.Fatalf("config constructor should not trigger DIP: %v", issues)
	}
}

func TestCheckDIPWithTypes_CrossPackageConstructorStillFlags(t *testing.T) {
	dir := t.TempDir()
	initTempModule(t, dir)
	otherDir := filepath.Join(dir, "other")
	if err := os.MkdirAll(otherDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherDir, "driver.go"), []byte("package other\n\ntype Driver struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "service.go"), []byte(`package p

import "tempmod/other"

type Service struct{}

func NewService(driver *other.Driver) *Service { return &Service{} }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgs := loadWorkspaceDir(t, dir, false, "fast")
	pkg := findPackageP(t, pkgs)
	issues := CheckDIPWithTypes(pkg.fset, pkg.files, pkg.info, DefaultConfig(), pkg)
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want cross-package constructor DIP only: %v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Message, `NewService`) {
		t.Fatalf("unexpected issue: %+v", issues[0])
	}
}

func TestCheckDIPWithTypes_CrossPackageDependencyKeepsQualifier(t *testing.T) {
	dir := t.TempDir()
	initTempModule(t, dir)
	otherDir := filepath.Join(dir, "minify")
	if err := os.MkdirAll(otherDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherDir, "minify.go"), []byte("package minify\n\ntype M struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "service.go"), []byte(`package p

import "tempmod/minify"

type Service struct { compressor *minify.M }
`), 0o644); err != nil {
		t.Fatal(err)
	}

	pkgs := loadWorkspaceDir(t, dir, false, "fast")
	pkg := findPackageP(t, pkgs)
	issues := CheckDIPWithTypes(pkg.fset, pkg.files, pkg.info, DefaultConfig(), pkg)
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want one cross-package field DIP issue: %v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Message, "*tempmod/minify.M") || issues[0].Evidence != "concrete-dependency:type=Service;field=compressor;dependency=tempmod/minify.M" {
		t.Fatalf("dependency name lost its package qualifier: %+v", issues[0])
	}
}

func TestCheckDIPWithTypes_ConstructorAndAllowlist(t *testing.T) {
	dir := t.TempDir()
	initTempModule(t, dir)
	otherDir := filepath.Join(dir, "other")
	if err := os.MkdirAll(otherDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherDir, "driver.go"), []byte("package other\n\ntype Driver struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "service.go"), []byte(`package p

import "tempmod/other"

type Service struct{}

func NewService(driver *other.Driver) *Service { return &Service{} }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgs := loadWorkspaceDir(t, dir, false, "fast")
	pkg := findPackageP(t, pkgs)
	if issues := CheckDIPWithTypes(pkg.fset, pkg.files, pkg.info, DefaultConfig(), pkg); len(issues) != 1 {
		t.Fatalf("got %d issues, want constructor DIP only", len(issues))
	}
	cfg := DefaultConfig()
	cfg.DIPAllowDependencies = []string{"tempmod/other.Driver"}
	if issues := CheckDIPWithTypes(pkg.fset, pkg.files, pkg.info, cfg, pkg); len(issues) != 0 {
		t.Fatalf("canonical imported-type allowlist did not suppress: %v", issues)
	}
	cfg = DefaultConfig()
	cfg.OCPCompositionRoots = []string{"tempmod"}
	if issues := CheckDIPWithTypes(pkg.fset, pkg.files, pkg.info, cfg, pkg); len(issues) != 0 {
		t.Fatalf("configured composition root did not suppress constructor dependency: %v", issues)
	}
}

func TestCheckDIPWithTypes_PassiveDomainDataIgnored(t *testing.T) {
	dir := t.TempDir()
	initTempModule(t, dir)
	domainDir := filepath.Join(dir, "domain")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "types.go"), []byte(`package domain

type Revision struct { ID string }
type Worker struct{}

func (*Worker) Run() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "service.go"), []byte(`package p

import "tempmod/domain"

type prepared struct {
	revision *domain.Revision
	worker *domain.Worker
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	pkgs := loadWorkspaceDir(t, dir, false, "fast")
	pkg := findPackageP(t, pkgs)
	issues := issuesWithCheck(CheckDIPWithTypes(pkg.fset, pkg.files, pkg.info, DefaultConfig(), pkg), CheckDIPConcreteDependency)
	if len(issues) != 1 {
		t.Fatalf("got %d concrete-dependency issues, want only behavioral domain dependency: %v", len(issues), issues)
	}
	if issues[0].Evidence != "concrete-dependency:type=prepared;field=worker;dependency=tempmod/domain.Worker" {
		t.Fatalf("unexpected domain dependency issue: %+v", issues[0])
	}
}

func TestCheckDIPWithTypes_TestDomainStateNeedsBehavioralEvidence(t *testing.T) {
	dir := t.TempDir()
	initTempModule(t, dir)
	domainDir := filepath.Join(dir, "domain")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "user.go"), []byte(`package domain

type User struct{}

func (*User) Display() string { return "user" }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "service.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "service_test.go"), []byte(`package p

import "tempmod/domain"

type fakeRepository struct {
	stored       *domain.User
	collaborator *domain.User
	name         string
	calls        int
}

func (f *fakeRepository) Get() *domain.User { return f.stored }
func (f *fakeRepository) Display() string    { return f.collaborator.Display() }
`), 0o644); err != nil {
		t.Fatal(err)
	}

	var pkg *packageFiles
	for _, candidate := range loadWorkspaceDir(t, dir, true, "types") {
		for _, file := range candidate.files {
			if strings.HasSuffix(candidate.fset.Position(file.Pos()).Filename, "service_test.go") {
				pkg = candidate
			}
		}
	}
	if pkg == nil {
		t.Fatal("test package was not loaded")
	}
	issues := issuesWithCheck(CheckDIPWithTypes(pkg.fset, pkg.files, pkg.info, DefaultConfig(), pkg), CheckDIPConcreteDependency)
	if len(issues) != 1 {
		t.Fatalf("test domain state issues = %d, want only behavioral collaborator: %v", len(issues), issues)
	}
	if issues[0].Evidence != "concrete-dependency:type=fakeRepository;field=collaborator;dependency=tempmod/domain.User" {
		t.Fatalf("unexpected behavioral test dependency: %+v", issues[0])
	}
}
