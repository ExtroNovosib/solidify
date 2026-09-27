package analyzer

import (
	"errors"
	"go/build"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveGOROOTPrefersActiveToolchain(t *testing.T) {
	fallback := goToolchainEnv{GOROOT: "/build/go", GOVERSION: "go1.0"}
	cases := []struct {
		name   string
		output string
		err    error
		want   string
	}{
		{name: "active toolchain", output: `{"GOROOT": "/active/go\n", "GOVERSION": "go1.99.0"}`, want: "/active/go"},
		{name: "go command unavailable", err: errors.New("go: not found"), want: "/build/go"},
		{name: "empty output", output: `{"GOROOT": " \n"}`, want: "/build/go"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveGoEnv(func() ([]byte, error) { return []byte(tc.output), tc.err }, fallback).GOROOT
			if got != tc.want {
				t.Fatalf("resolved GOROOT = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveGoEnvValues(t *testing.T) {
	fallback := goToolchainEnv{GOROOT: "/build/go", GOVERSION: "go1.0"}
	cases := []struct {
		name   string
		output string
		err    error
		want   goToolchainEnv
	}{
		{name: "both values", output: "{\n\t\"GOROOT\": \"/active/go\",\n\t\"GOVERSION\": \"go1.26.3\"\n}\n", want: goToolchainEnv{GOROOT: "/active/go", GOVERSION: "go1.26.3"}},
		{name: "missing version", output: `{"GOROOT": "/active/go"}`, want: goToolchainEnv{GOROOT: "/active/go", GOVERSION: "go1.0"}},
		{name: "blank root", output: `{"GOROOT": "", "GOVERSION": "go1.26.3"}`, want: goToolchainEnv{GOROOT: "/build/go", GOVERSION: "go1.26.3"}},
		{name: "malformed JSON", output: "/active/go\n", want: fallback},
		{name: "go command unavailable", err: errors.New("go: not found"), want: fallback},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			got := resolveGoEnv(func() ([]byte, error) {
				calls++
				return []byte(tc.output), tc.err
			}, fallback)
			if got != tc.want || calls != 1 {
				t.Fatalf("resolveGoEnv = %+v after %d go env call(s), want %+v after one", got, calls, tc.want)
			}
		})
	}
	first, second := activeGoToolchain(), activeGoToolchain()
	if first != second || first.GOROOT == "" || !strings.HasPrefix(first.GOVERSION, "go") {
		t.Fatalf("active toolchain = %+v then %+v, want one stable resolved value", first, second)
	}
}

// Prebuilt binaries embed the release builder's GOROOT, which does not exist
// on the machine that runs them. Standard-library types must still be
// recognized there.
func TestStdlibConcreteDependenciesIgnoredWithStaleBuildGOROOT(t *testing.T) {
	fset, files := parseSource(t, `package p
import (
	"database/sql"
	"net"
	"net/http"
)
type Service struct {
	client *http.Client
	db     *sql.DB
	conn   *net.UDPConn
}
func NewService(client *http.Client, db *sql.DB, conn *net.UDPConn) *Service {
	return &Service{client: client, db: db, conn: conn}
}
`)
	info := typeCheckSource(t, fset, files)
	previous := build.Default.GOROOT
	build.Default.GOROOT = filepath.Join(t.TempDir(), "missing-goroot")
	t.Cleanup(func() { build.Default.GOROOT = previous })

	if issues := CheckDIPWithTypes(fset, files, info, DefaultConfig(), nil); len(issues) != 0 {
		t.Fatalf("got %d issues, want stdlib concrete dependencies ignored: %v", len(issues), issues)
	}
}
