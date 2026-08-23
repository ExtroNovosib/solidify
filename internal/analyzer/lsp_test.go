package analyzer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckLSP_NoLongerReportsStubMethods(t *testing.T) {
	src := `package p

import "errors"

type RO struct{}
type Store interface {
	Get(string) (string, error)
	Save(string) error
	Delete(string) error
}

func (r *RO) Save(v string) error {
	panic("not supported")
}

func (r *RO) Delete(id string) error {
	return errors.ErrUnsupported
}
`
	fset, files := parseSource(t, src)
	issues := CheckLSPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
	if len(issues) != 0 {
		t.Fatalf("got %d issues, want 0: %v", len(issues), issues)
	}
}

func TestCheckLSP_RealImplementationIgnored(t *testing.T) {
	src := `package p

type S struct{}

func (s *S) Work() error { return nil }
`
	fset, files := parseSource(t, src)
	issues := CheckLSPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
	if len(issues) != 0 {
		t.Fatalf("got %d issues, want 0", len(issues))
	}
}

func TestCheckLSP_UnsupportedNonInterfaceMethodIgnored(t *testing.T) {
	fset, files := parseSource(t, `package p
type S struct{}
func (*S) Internal() error { panic("not supported") }
`)
	if issues := CheckLSPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil); len(issues) != 0 {
		t.Fatalf("got unexpected issue: %v", issues)
	}
}

func TestCheckLSP_ConditionalFallbackIgnored(t *testing.T) {
	src := `package p

import "errors"

type S struct{}
type Worker interface { Work(bool) error }

func (s *S) Work(cached bool) error {
	if cached { return nil }
	return errors.New("not supported")
}
`
	fset, files := parseSource(t, src)
	issues := CheckLSPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
	if len(issues) != 0 {
		t.Fatalf("got %d issues, want 0: %v", len(issues), issues)
	}
}

func TestCheckLSP_NonExactEOF(t *testing.T) {
	for _, tc := range []struct {
		name   string
		body   string
		issues int
		kind   string
	}{
		{
			name: "wrapped EOF",
			body: `import (
 "fmt"
 "io"
)
type reader struct{}
func (reader) Read([]byte) (int, error) { return 0, fmt.Errorf("reader: %w", io.EOF) }`,
			issues: 1,
			kind:   "wrapped-eof",
		},
		{
			name: "recreated EOF",
			body: `import "errors"
type reader struct{}
func (reader) Read([]byte) (int, error) { return 0, errors.New("EOF") }`,
			issues: 1,
			kind:   "recreated-eof",
		},
		{
			name: "joined EOF",
			body: `import (
 "errors"
 "io"
)
type reader struct{}
func (reader) Read([]byte) (int, error) { return 0, errors.Join(io.EOF, errors.New("context")) }`,
			issues: 1,
			kind:   "wrapped-eof",
		},
		{
			name: "exact EOF",
			body: `import "io"
type reader struct{}
func (reader) Read([]byte) (int, error) { return 0, io.EOF }`,
		},
		{
			name: "ordinary error",
			body: `import "errors"
type reader struct{}
func (reader) Read([]byte) (int, error) { return 0, errors.New("broken transport") }`,
		},
		{
			name: "non Reader signature",
			body: `import "errors"
type reader struct{}
func (reader) Read([]byte) error { return errors.New("EOF") }`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fset, files := parseSource(t, "package p\n"+tc.body)
			issues := CheckLSPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
			if len(issues) != tc.issues {
				t.Fatalf("got %d issues, want %d: %v", len(issues), tc.issues, issues)
			}
			if tc.issues == 0 {
				return
			}
			if issues[0].Check != CheckLSPNonExactEOF || !strings.Contains(issues[0].Evidence, tc.kind) {
				t.Fatalf("unexpected issue: %+v", issues[0])
			}
		})
	}
}

func TestCheckLSPProgram_NilEmbeddedInterface(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
		issues int
	}{
		{
			name: "uninitialized promoted method",
			source: `package p
type Backend interface { Ping(); Close() }
type Server struct { Backend }
func (Server) Ping() {}
`,
			issues: 1,
		},
		{
			name: "constructor initializes embedded interface",
			source: `package p
type Backend interface { Ping(); Close() }
type Server struct { Backend }
func (Server) Ping() {}
func New(backend Backend) *Server { return &Server{Backend: backend} }
`,
		},
		{
			name: "all methods explicitly overridden",
			source: `package p
type Backend interface { Ping(); Close() }
type Server struct { Backend }
func (Server) Ping() {}
func (Server) Close() {}
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "server.go")
			if err := os.WriteFile(path, []byte(tc.source), 0o644); err != nil {
				t.Fatal(err)
			}
			pkgs := loadWorkspaceDir(t, dir, false, "auto")
			issues := Run(pkgs, DefaultConfig(), map[Rule]bool{RuleLSP: true})
			if len(issues) != tc.issues {
				t.Fatalf("got %d issues, want %d: %v", len(issues), tc.issues, issues)
			}
			if tc.issues == 1 {
				if issues[0].Check != CheckLSPNilEmbeddedInterface || !strings.Contains(issues[0].Evidence, "methods=Close") {
					t.Fatalf("unexpected issue: %+v", issues[0])
				}
			}
		})
	}
}
