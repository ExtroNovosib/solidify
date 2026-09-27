package analyzer

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// cacheKeyMaterial lists every policy input that can change findings. Fields
// are named explicitly rather than serializing Config wholesale, so a new
// run-only setting cannot silently fragment the cache and a new policy field
// cannot silently be ignored: TestCacheKeyMaterialCoversPolicyFields fails
// until each exported Config field is either listed here or in
// cacheRunOnlyConfigFields.
type cacheKeyMaterial struct {
	Thresholds     map[string]int
	Profile        Profile
	EnabledChecks  []CheckID
	DisabledChecks []CheckID
	AnalysisMode   string
	IncludeTests   bool
	ExcludedFiles  []string

	SRPOrchestratorSuffixes    []string
	OCPDiscriminatorFields     []string
	OCPAllowDispatchTypes      []string
	OCPAllowPackages           []string
	OCPLogicPackages           []string
	OCPImplementationPackages  []string
	OCPCompositionRoots        []string
	ISPWiringAggregateSuffixes []string
	DIPAllowDependencies       []string
	DIPInfraErrorPackages      []string
	DIPTransportTypes          []string
	DIPDomainPackages          []string
	DIPDataBagSuffixes         []string
	DIPDetailImports           []string

	PlanIdentity string
	Build        string
}

// cacheRunOnlyConfigFields never influence findings: they choose where and
// whether results are cached, what is reported about the cache, and how the
// build labels itself. The executable digest identifies the analyzer build.
var cacheRunOnlyConfigFields = map[string]bool{
	"CacheDir": true, "CacheEnabled": true, "CacheDiagnostics": true, "ToolVersion": true,
}

func newCacheKeyMaterial(cfg Config, plan ExecutionPlan, build string) cacheKeyMaterial {
	return cacheKeyMaterial{
		Thresholds:     EffectiveThresholds(cfg),
		Profile:        cfg.Profile,
		EnabledChecks:  cfg.EnabledChecks,
		DisabledChecks: cfg.DisabledChecks,
		AnalysisMode:   cfg.AnalysisMode,
		IncludeTests:   cfg.IncludeTests,
		ExcludedFiles:  cfg.ExcludedFiles,

		SRPOrchestratorSuffixes:    cfg.SRPOrchestratorSuffixes,
		OCPDiscriminatorFields:     cfg.OCPDiscriminatorFields,
		OCPAllowDispatchTypes:      cfg.OCPAllowDispatchTypes,
		OCPAllowPackages:           cfg.OCPAllowPackages,
		OCPLogicPackages:           cfg.OCPLogicPackages,
		OCPImplementationPackages:  cfg.OCPImplementationPackages,
		OCPCompositionRoots:        cfg.OCPCompositionRoots,
		ISPWiringAggregateSuffixes: cfg.ISPWiringAggregateSuffixes,
		DIPAllowDependencies:       cfg.DIPAllowDependencies,
		DIPInfraErrorPackages:      cfg.DIPInfraErrorPackages,
		DIPTransportTypes:          cfg.DIPTransportTypes,
		DIPDomainPackages:          cfg.DIPDomainPackages,
		DIPDataBagSuffixes:         cfg.DIPDataBagSuffixes,
		DIPDetailImports:           cfg.DIPDetailImports,

		PlanIdentity: plan.Identity(),
		Build:        build,
	}
}

// digest returns the short key that namespaces cache entries.
func (m cacheKeyMaterial) digest() string {
	data, _ := json.Marshal(m)
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:8])
}

// executableDigestMemos holds one executable digest per cache root per process.
var executableDigestMemos sync.Map

// executableDigest identifies the running analyzer build. Version strings do
// not: test binaries and unversioned builds all report "dev", and dirty builds
// of one commit share a pseudo-version, so they would reuse each other's
// findings after an analyzer change. Hashing a large executable on every run
// dominated warm runs, so the digest is remembered in the cache root and
// reused while the executable's path, size, and modification time match.
func executableDigest(cacheRoot string) string {
	if cached, ok := executableDigestMemos.Load(cacheRoot); ok {
		return cached.(string)
	}
	path, err := os.Executable()
	if err != nil {
		return ""
	}
	digest := memoizedFileDigest(filepath.Join(cacheRoot, "exe-digest.json"), path)
	actual, _ := executableDigestMemos.LoadOrStore(cacheRoot, digest)
	return actual.(string)
}

type fileDigestMemo struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTimeUnixNano"`
	Digest  string `json:"digest"`
}

// memoizedFileDigest returns the content digest of path, reusing the digest
// recorded in memoPath when path, size, and modification time still match.
func memoizedFileDigest(memoPath, path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	current := fileDigestMemo{Path: path, Size: info.Size(), ModTime: info.ModTime().UnixNano()}
	if data, readErr := os.ReadFile(memoPath); readErr == nil {
		var memo fileDigestMemo
		if json.Unmarshal(data, &memo) == nil && memo.Path == current.Path && memo.Size == current.Size &&
			memo.ModTime == current.ModTime && len(memo.Digest) == sha256.Size*2 {
			return memo.Digest
		}
	}
	digest, err := fileContentDigest(path)
	if err != nil {
		return ""
	}
	current.Digest = digest
	if data, marshalErr := json.Marshal(current); marshalErr == nil {
		writeFileAtomically(memoPath, data)
	}
	return digest
}

// writeFileAtomically replaces path with data; failures are ignored because
// every caller can recompute what it would have saved.
func writeFileAtomically(path string, data []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".solidlint-cache-*.tmp")
	if err != nil {
		return
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return
	}
	if err := tmp.Close(); err != nil {
		return
	}
	_ = os.Rename(tmpName, path)
}

type importKeyKind uint8

const (
	importKeyAPI    importKeyKind = iota // exported API digest of the loaded package
	importKeyStdlib                      // active Go release
	importKeyPinned                      // module go.mod and go.sum contents
)

// moduleDependencyKey records how a module pins its external dependencies.
type moduleDependencyKey struct {
	digest string
	pinned bool
}

// classifyImport chooses the cheapest dependency key that still changes
// whenever the imported package's API can change. Workspace and same-module
// packages are edited in place, so they keep the exported-API digest. The
// standard library changes only with the Go release. Any other import is
// fixed by the importing module's go.mod and go.sum unless a filesystem
// replace, a go.work file, or a vendor directory lets its source change
// without a version change.
func (c *packageCache) classifyImport(pkg *packageFiles, path string) (importKeyKind, string) {
	if c.workspace[path] || pkg.modulePath != "" && (path == pkg.modulePath || strings.HasPrefix(path, pkg.modulePath+"/")) {
		return importKeyAPI, ""
	}
	if !strings.Contains(firstPathElement(path), ".") && isStdlibImportPath(path) {
		return importKeyStdlib, activeGoVersion()
	}
	if key := c.moduleDependencyKey(pkg); key.pinned {
		return importKeyPinned, key.digest
	}
	return importKeyAPI, ""
}

func firstPathElement(path string) string {
	if slash := strings.IndexByte(path, '/'); slash >= 0 {
		return path[:slash]
	}
	return path
}

func (c *packageCache) moduleDependencyKey(pkg *packageFiles) moduleDependencyKey {
	if pkg.moduleGoMod == "" {
		return moduleDependencyKey{}
	}
	if cached, ok := c.moduleKeys.Load(pkg.moduleGoMod); ok {
		return cached.(moduleDependencyKey)
	}
	key := computeModuleDependencyKey(pkg.moduleGoMod, pkg.analysisRoot)
	actual, _ := c.moduleKeys.LoadOrStore(pkg.moduleGoMod, key)
	return actual.(moduleDependencyKey)
}

func computeModuleDependencyKey(goModPath, analysisRoot string) moduleDependencyKey {
	goMod, err := os.ReadFile(goModPath)
	if err != nil {
		return moduleDependencyKey{}
	}
	moduleDir := filepath.Dir(goModPath)
	if goModHasFilesystemReplace(goMod) || goWorkGoverns(moduleDir, analysisRoot) || fileExists(filepath.Join(moduleDir, "vendor", "modules.txt")) {
		return moduleDependencyKey{}
	}
	goSum, err := os.ReadFile(filepath.Join(moduleDir, "go.sum"))
	if err != nil && !os.IsNotExist(err) {
		return moduleDependencyKey{}
	}
	h := sha256.New()
	h.Write(goMod)
	h.Write([]byte{0})
	h.Write(goSum)
	return moduleDependencyKey{digest: fmt.Sprintf("%x", h.Sum(nil)), pinned: true}
}

// goModHasFilesystemReplace reports whether any replace directive targets a
// directory, whose contents can change without a version change.
func goModHasFilesystemReplace(goMod []byte) bool {
	scanner := bufio.NewScanner(bytes.NewReader(goMod))
	inReplaceBlock := false
	for scanner.Scan() {
		line := scanner.Text()
		if comment := strings.Index(line, "//"); comment >= 0 {
			line = line[:comment]
		}
		fields := strings.Fields(line)
		switch {
		case len(fields) == 0:
			continue
		case inReplaceBlock && fields[0] == ")":
			inReplaceBlock = false
			continue
		case !inReplaceBlock && (fields[0] == "replace" || fields[0] == "replace("):
			if fields[0] == "replace(" || len(fields) == 2 && fields[1] == "(" {
				inReplaceBlock = true
				continue
			}
			fields = fields[1:]
		case !inReplaceBlock:
			continue
		}
		if arrow := indexOf(fields, "=>"); arrow >= 0 && arrow+1 < len(fields) && isFilesystemModulePath(strings.Trim(fields[arrow+1], "\"`")) {
			return true
		}
	}
	return false
}

func indexOf(values []string, target string) int {
	for index, value := range values {
		if value == target {
			return index
		}
	}
	return -1
}

// isFilesystemModulePath mirrors the go command's rule for replacement
// targets that name a directory rather than a module path.
func isFilesystemModulePath(target string) bool {
	for _, prefix := range []string{"./", "../", "/", `.\`, `..\`, `\`} {
		if strings.HasPrefix(target, prefix) {
			return true
		}
	}
	return target == "." || target == ".." ||
		len(target) >= 2 && target[1] == ':' && ('A' <= target[0] && target[0] <= 'Z' || 'a' <= target[0] && target[0] <= 'z')
}

// goWorkGoverns reports whether a go.work file may govern the load: GOWORK
// names one, or one exists above the module or the analysis root. Workspace
// modules are used from their directories, so their go.sum pins nothing.
func goWorkGoverns(dirs ...string) bool {
	switch gowork := strings.TrimSpace(os.Getenv("GOWORK")); gowork {
	case "off":
		return false
	case "":
	default:
		return true
	}
	for _, start := range dirs {
		if start == "" {
			continue
		}
		for dir := filepath.Clean(start); ; dir = filepath.Dir(dir) {
			if fileExists(filepath.Join(dir, "go.work")) {
				return true
			}
			if parent := filepath.Dir(dir); parent == dir {
				break
			}
		}
	}
	return false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
