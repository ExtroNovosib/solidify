package analyzer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeArchitectureFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/diparch\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	providerDir := filepath.Join(dir, "adapters", "postgres")
	if err := os.MkdirAll(providerDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(providerDir, "client.go"), []byte(`package postgres

type Client struct{}

func NewClient() *Client { return &Client{} }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	serviceDir := filepath.Join(dir, "service")
	if err := os.MkdirAll(serviceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(serviceDir, "service.go"), []byte(`package service

import "example.com/diparch/adapters/postgres"

type Service struct { db *postgres.Client }

func NewService() *Service {
	return &Service{db: postgres.NewClient()}
}

func Wire() {
	_ = postgres.NewClient()
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	rootDir := filepath.Join(dir, "cmd", "app")
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "main.go"), []byte(`package main

import "example.com/diparch/adapters/postgres"

func main() {
	_ = postgres.NewClient()
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func dipArchitectureConfig() Config {
	cfg := DefaultConfig()
	cfg.OCPLogicPackages = []string{"example.com/diparch/service"}
	cfg.OCPImplementationPackages = []string{"example.com/diparch/adapters/**"}
	cfg.OCPCompositionRoots = []string{"example.com/diparch/cmd/**"}
	return cfg
}

func TestCheckDIPProgramLayerImport(t *testing.T) {
	dir := writeArchitectureFixture(t)
	pkgs, _, err := LoadWorkspace([]string{dir}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	issues := CheckDIPProgram(pkgs, dipArchitectureConfig())
	if !hasCheck(issues, CheckDIPLayerImport) {
		t.Fatalf("want layer-import, got: %v", issues)
	}
	for _, issue := range issues {
		if issue.Check == CheckDIPLayerImport && issue.Evidence != "layer-import:from=example.com/diparch/service;to=example.com/diparch/adapters/postgres" {
			t.Fatalf("unexpected evidence: %q", issue.Evidence)
		}
	}
}

func TestCheckDIPProgramHiddenConstructionAndWiring(t *testing.T) {
	dir := writeArchitectureFixture(t)
	pkgs, _, err := LoadWorkspace([]string{dir}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	issues := CheckDIPProgram(pkgs, dipArchitectureConfig())
	if !hasCheck(issues, CheckDIPHiddenConstruction) {
		t.Fatalf("want hidden-construction, got: %v", issues)
	}
	if !hasCheck(issues, CheckDIPWiringOutsideRoot) {
		t.Fatalf("want wiring-outside-root, got: %v", issues)
	}
	for _, issue := range issues {
		if issue.Check == CheckDIPWiringOutsideRoot && issue.Pos.Line == 0 {
			t.Fatalf("wiring issue missing position: %v", issue)
		}
	}
}

func TestCheckDIPProgramCompositionRootExempt(t *testing.T) {
	dir := writeArchitectureFixture(t)
	pkgs, _, err := LoadWorkspace([]string{dir}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	issues := CheckDIPProgram(pkgs, dipArchitectureConfig())
	for _, issue := range issues {
		if issue.Check == CheckDIPWiringOutsideRoot && issue.Evidence == "wiring-outside-root:package=example.com/diparch/cmd/app;callee=example.com/diparch/adapters/postgres.NewClient" {
			t.Fatalf("composition root should be exempt: %v", issue)
		}
	}
}

func TestCheckDIPProgramImplementationPackageConstructionExempt(t *testing.T) {
	dir := writeArchitectureFixture(t)
	providerFile := filepath.Join(dir, "adapters", "postgres", "factory.go")
	if err := os.WriteFile(providerFile, []byte(`package postgres

func NewDefaultClient() *Client {
	return NewClient()
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgs, _, err := LoadWorkspace([]string{dir}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	for _, issue := range CheckDIPProgram(pkgs, dipArchitectureConfig()) {
		if strings.Contains(issue.Pos.Filename, providerFile) &&
			(issue.Check == CheckDIPHiddenConstruction || issue.Check == CheckDIPWiringOutsideRoot) {
			t.Fatalf("implementation package internal construction should be exempt: %v", issue)
		}
	}
}

func TestCheckDIPProgramInfraErrorLeak(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/diperr\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	usecaseDir := filepath.Join(dir, "usecase")
	if err := os.MkdirAll(usecaseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	source := `package usecase

import (
	"database/sql"
	"errors"
)

func Lookup(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
`
	if err := os.WriteFile(filepath.Join(usecaseDir, "lookup.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgs, _, err := LoadWorkspace([]string{dir}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.OCPLogicPackages = []string{"example.com/diperr/usecase"}
	issues := CheckDIPProgram(pkgs, cfg)
	if !hasCheck(issues, CheckDIPInfraErrorLeak) {
		t.Fatalf("want infra-error-leak, got: %v", issues)
	}
}

func TestCheckDIPProgramTransportLeak(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/diptransport\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	usecaseDir := filepath.Join(dir, "usecase")
	if err := os.MkdirAll(usecaseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	source := `package usecase

import "net/http"

func Handle(req *http.Request) error {
	_ = req
	return nil
}
`
	if err := os.WriteFile(filepath.Join(usecaseDir, "handler.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgs, _, err := LoadWorkspace([]string{dir}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.OCPLogicPackages = []string{"example.com/diptransport/usecase"}
	issues := CheckDIPProgram(pkgs, cfg)
	if !hasCheck(issues, CheckDIPTransportLeak) {
		t.Fatalf("want transport-leak, got: %v", issues)
	}
}

func TestCheckDIPProgramArchitectureDisabled(t *testing.T) {
	dir := writeArchitectureFixture(t)
	pkgs, _, err := LoadWorkspace([]string{dir}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	issues := CheckDIPProgram(pkgs, DefaultConfig())
	for _, issue := range issues {
		if issue.Check == CheckDIPLayerImport ||
			issue.Check == CheckDIPHiddenConstruction ||
			issue.Check == CheckDIPWiringOutsideRoot {
			t.Fatalf("architecture checks should no-op without config: %v", issue)
		}
	}
}

func TestCheckDIPWithTypes_SetsConcreteDependencyCheck(t *testing.T) {
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
	if len(issues) != 1 || issues[0].Check != CheckDIPConcreteDependency {
		t.Fatalf("got %#v, want CheckDIPConcreteDependency", issues)
	}
}

func TestCheckDIPProgramBridgeAdapterSkipsOnlyForwardedFields(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/bridge\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	managerDir := filepath.Join(dir, "manager")
	if err := os.MkdirAll(managerDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(managerDir, "manager.go"), []byte(`package manager

type Manager struct{}

func (m *Manager) Start() error { return nil }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cacheDir := filepath.Join(dir, "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "cache.go"), []byte(`package cache

type Cache struct{}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	portDir := filepath.Join(dir, "port")
	if err := os.MkdirAll(portDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(portDir, "port.go"), []byte(`package port

import (
	"example.com/bridge/cache"
	"example.com/bridge/manager"
)

type RuntimePort struct {
	mgr   *manager.Manager
	cache *cache.Cache
}

func (p *RuntimePort) Run() error { return p.mgr.Start() }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgs, _, err := LoadWorkspace([]string{dir}, false, "types")
	if err != nil {
		t.Fatal(err)
	}
	var issues []Issue
	for _, pkg := range pkgs {
		if pkg.pkgPath != "example.com/bridge/port" {
			continue
		}
		issues = CheckDIPWithTypes(pkg.fset, pkg.files, pkg.info, DefaultConfig(), nil)
	}
	if len(issues) == 0 {
		t.Fatalf("expected at least one DIP issue, got none")
	}
	for _, issue := range issues {
		if issue.Check == CheckDIPConcreteDependency && strings.Contains(issue.Message, "RuntimePort.mgr") {
			t.Fatalf("forwarded bridge field should not trigger DIP: %v", issues)
		}
	}
	foundCache := false
	for _, issue := range issues {
		if issue.Check == CheckDIPConcreteDependency && strings.Contains(issue.Message, "RuntimePort.cache") {
			foundCache = true
		}
	}
	if !foundCache {
		t.Fatalf("non-forwarded bridge field should trigger DIP: %v", issues)
	}
}
