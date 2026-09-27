package analyzer

import (
	"encoding/json"
	"go/build"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// goToolchainEnv describes the go command that loads packages.
type goToolchainEnv struct {
	GOROOT    string
	GOVERSION string
}

// activeGoToolchain asks the go command on PATH once per process for both
// values. build.Default.GOROOT and runtime.Version are fixed when solidlint
// itself is compiled: prebuilt binaries carry the release builder's toolchain,
// and local installs go stale after a Go upgrade. Either way every
// standard-library package would look external and standard-library cache
// keys would name the wrong release, so they serve only as fallbacks.
var activeGoToolchain = sync.OnceValue(func() goToolchainEnv {
	return resolveGoEnv(func() ([]byte, error) {
		return exec.Command("go", "env", "-json", "GOROOT", "GOVERSION").Output()
	}, goToolchainEnv{GOROOT: build.Default.GOROOT, GOVERSION: runtime.Version()})
})

// resolveGoEnv parses `go env -json GOROOT GOVERSION` output, keeping the
// fallback for any value the command could not supply.
func resolveGoEnv(goEnv func() ([]byte, error), fallback goToolchainEnv) goToolchainEnv {
	output, err := goEnv()
	if err != nil {
		return fallback
	}
	var reported goToolchainEnv
	if json.Unmarshal(output, &reported) != nil {
		return fallback
	}
	resolved := fallback
	if root := strings.TrimSpace(reported.GOROOT); root != "" {
		resolved.GOROOT = root
	}
	if version := strings.TrimSpace(reported.GOVERSION); version != "" {
		resolved.GOVERSION = version
	}
	return resolved
}

// stdlibSourceRoot is the GOROOT of the go command that loads packages.
func stdlibSourceRoot() string { return activeGoToolchain().GOROOT }

// activeGoVersion names the standard-library release packages are loaded
// against; it keys cache entries that depend only on the standard library.
func activeGoVersion() string { return activeGoToolchain().GOVERSION }

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
