package analyzer

import (
	"errors"
	"go/build"
	"path/filepath"
	"testing"
)

func TestResolveGOROOTPrefersActiveToolchain(t *testing.T) {
	cases := []struct {
		name   string
		output string
		err    error
		want   string
	}{
		{name: "active toolchain", output: "/active/go\n", want: "/active/go"},
		{name: "go command unavailable", err: errors.New("go: not found"), want: "/build/go"},
		{name: "empty output", output: " \n", want: "/build/go"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveGOROOT(func() (string, error) { return tc.output, tc.err }, "/build/go")
			if got != tc.want {
				t.Fatalf("resolveGOROOT = %q, want %q", got, tc.want)
			}
		})
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
