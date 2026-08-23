package analyzer

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Bump the version when cache-key semantics change so entries produced by an
// older solidlint cannot be reused with the new analyzer.
const cacheVersion = "solidlint-cache-v7"

type packageCache struct {
	root      string
	enabled   string
	config    string
	version   string
	hits      atomic.Int64
	misses    atomic.Int64
	stale     atomic.Int64
	corrupt   atomic.Int64
	hashNanos atomic.Int64
	loadNanos atomic.Int64
	hashes    sync.Map
}

func newPackageCache(root string, cfg Config, enabled map[Rule]bool) *packageCache {
	letters := make([]string, 0, len(enabled))
	for rule, on := range enabled {
		if on {
			letters = append(letters, string(rule))
		}
	}
	sort.Strings(letters)
	configData, _ := json.Marshal(struct {
		Config  Config
		Enabled []string
	}{cfg, letters})
	sum := sha256.Sum256(configData)
	return &packageCache{
		root:    filepath.Clean(root),
		enabled: strings.Join(letters, ","),
		config:  fmt.Sprintf("%x", sum[:8]),
		version: cfg.ToolVersion,
	}
}

func (c *packageCache) load(pkg *packageFiles, checkID CheckID) ([]Issue, bool) {
	if c == nil || pkg == nil {
		return nil, false
	}
	path := c.entryPath(pkg, checkID)
	loadStart := time.Now()
	defer func() {
		c.loadNanos.Add(time.Since(loadStart).Nanoseconds())
	}()
	data, err := os.ReadFile(path)
	if err != nil {
		c.misses.Add(1)
		return nil, false
	}
	var entry struct {
		Version string        `json:"version"`
		Hash    string        `json:"hash"`
		Issues  []cachedIssue `json:"issues"`
	}
	if err := json.Unmarshal(data, &entry); err != nil || entry.Version != cacheVersion || entry.Hash != c.packageHash(pkg) {
		c.misses.Add(1)
		if err != nil {
			c.corrupt.Add(1)
		} else {
			c.stale.Add(1)
		}
		return nil, false
	}
	c.hits.Add(1)
	issues := make([]Issue, 0, len(entry.Issues))
	for _, cached := range entry.Issues {
		issue := cached.Issue
		issue.analysisRoot = pkg.analysisRoot
		issues = append(issues, issue)
	}
	return issues, true
}

func (c *packageCache) diagnostics() string {
	if c == nil {
		return ""
	}
	hashMs := float64(c.hashNanos.Load()) / 1e6
	loadMs := float64(c.loadNanos.Load()) / 1e6
	return fmt.Sprintf(
		"cache location=%s version=%s hits=%d misses=%d invalidations=%d corrupt=%d hash_time_ms=%.2f load_time_ms=%.2f",
		c.root, c.version, c.hits.Load(), c.misses.Load(), c.stale.Load(), c.corrupt.Load(), hashMs, loadMs,
	)
}

func (c *packageCache) store(pkg *packageFiles, checkID CheckID, issues []Issue) {
	if c == nil || pkg == nil {
		return
	}
	path := c.entryPath(pkg, checkID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	cached := make([]cachedIssue, 0, len(issues))
	for _, issue := range issues {
		cached = append(cached, cachedIssue{Issue: issue})
	}
	entry := struct {
		Version string        `json:"version"`
		Hash    string        `json:"hash"`
		Issues  []cachedIssue `json:"issues"`
	}{cacheVersion, c.packageHash(pkg), cached}
	data, err := json.Marshal(entry)
	if err != nil {
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
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return
	}
	if err := tmp.Close(); err != nil {
		return
	}
	_ = os.Rename(tmpName, path)
}

type cachedIssue struct {
	Issue Issue `json:"issue"`
}

// packageCheckCache caches package-scoped check results.
type packageCheckCache interface {
	load(pkg *packageFiles, checkID CheckID) ([]Issue, bool)
	store(pkg *packageFiles, checkID CheckID, issues []Issue)
}

func (c *packageCache) entryPath(pkg *packageFiles, checkID CheckID) string {
	key := strings.NewReplacer("/", "_", "\\", "_").Replace(pkg.pkgPath)
	if key == "" {
		key = strings.NewReplacer("/", "_", "\\", "_").Replace(pkg.dir)
	}
	return filepath.Join(c.root, "entries", c.enabled, c.config, string(checkID), key+".json")
}

func (c *packageCache) packageHash(pkg *packageFiles) string {
	if cached, ok := c.hashes.Load(pkg); ok {
		return cached.(string)
	}
	hashStart := time.Now()
	defer func() {
		c.hashNanos.Add(time.Since(hashStart).Nanoseconds())
	}()
	h := sha256.New()
	h.Write([]byte(pkg.pkgPath))
	h.Write([]byte{0})
	h.Write([]byte(pkg.modulePath))
	h.Write([]byte{0})
	h.Write([]byte(pkg.dependencyFacts))
	h.Write([]byte{0})
	files := append([]*ast.File(nil), pkg.files...)
	sort.Slice(files, func(i, j int) bool {
		return pkg.fset.Position(files[i].Pos()).Filename < pkg.fset.Position(files[j].Pos()).Filename
	})
	for _, file := range files {
		name := pkg.fset.Position(file.Pos()).Filename
		h.Write([]byte(PortablePath(pkg.analysisRoot, name)))
		h.Write([]byte{0})
		if pkg.generated[file] {
			h.Write([]byte("generated"))
		}
		h.Write([]byte{0})
		// A source file can retain its path and package name while changing an
		// interface into a concrete dependency (or the reverse). Include its
		// contents so cached findings always describe the current source.
		if source, err := os.ReadFile(name); err == nil {
			h.Write(source)
		} else {
			// Keep the fallback deterministic for virtual or deleted files. A
			// later successful read will produce a different hash and refresh the
			// cache entry.
			h.Write([]byte("unreadable-source"))
			h.Write([]byte(err.Error()))
		}
		h.Write([]byte{0})
	}
	result := fmt.Sprintf("%x", h.Sum(nil))
	actual, _ := c.hashes.LoadOrStore(pkg, result)
	return actual.(string)
}
