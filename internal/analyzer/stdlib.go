package analyzer

import (
	"go/build"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// stdlibSourceRoot is the GOROOT of the go command that loads packages.
// build.Default.GOROOT is fixed when solidlint itself is compiled: prebuilt
// binaries carry the release builder's toolchain path, and local installs go
// stale after a Go upgrade. Either way every standard-library package would
// look external, so the active toolchain is asked once per process.
var stdlibSourceRoot = sync.OnceValue(func() string {
	return resolveGOROOT(func() (string, error) {
		output, err := exec.Command("go", "env", "GOROOT").Output()
		return string(output), err
	}, build.Default.GOROOT)
})

func resolveGOROOT(goEnv func() (string, error), fallback string) string {
	if output, err := goEnv(); err == nil {
		if root := strings.TrimSpace(output); root != "" {
			return root
		}
	}
	return fallback
}

var stdlibImportPaths sync.Map

// isStdlibImportPath reports whether path is a package directory of the
// active toolchain's standard library. Results are memoized because callers
// ask once per selector expression.
func isStdlibImportPath(path string) bool {
	if cached, ok := stdlibImportPaths.Load(path); ok {
		return cached.(bool)
	}
	info, err := os.Stat(filepath.Join(stdlibSourceRoot(), "src", path))
	standard := err == nil && info.IsDir()
	stdlibImportPaths.Store(path, standard)
	return standard
}

func isStdlibPackage(pkg *types.Package) bool {
	if pkg == nil {
		return false
	}
	path := pkg.Path()
	if path == "" || strings.Contains(path, ".") {
		return false
	}
	return isStdlibImportPath(path)
}
