package analyzer

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"strconv"
	"strings"
)

// normalizedPath returns a slash-separated path relative to root when the
// file is inside root. The boolean is false for files outside the analysis
// root. Relative filenames are already unambiguous in parser-only tests and
// are kept relative rather than being resolved against the process cwd.
func normalizedPath(root, filename string) (string, bool) {
	if windowsStylePath(root) || windowsStylePath(filename) {
		return normalizedWindowsPath(root, filename)
	}
	if strings.HasPrefix(root, "/") || strings.HasPrefix(filename, "/") {
		return normalizedSlashPath(root, filename)
	}
	filename = filepath.Clean(filename)
	if filename == "." || filename == "" {
		return "", false
	}
	if root == "" {
		return filepath.ToSlash(filename), !filepath.IsAbs(filename)
	}
	if !filepath.IsAbs(filename) {
		return filepath.ToSlash(filename), true
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return filepath.ToSlash(filename), false
	}
	absoluteFile, err := filepath.Abs(filename)
	if err != nil {
		return filepath.ToSlash(filename), false
	}
	rel, err := filepath.Rel(absoluteRoot, absoluteFile)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(absoluteFile), false
	}
	return filepath.ToSlash(rel), true
}

func normalizedSlashPath(root, filename string) (string, bool) {
	root = pathpkg.Clean(root)
	filename = pathpkg.Clean(filename)
	if filename == "." || filename == "" {
		return "", false
	}
	if root == "." || root == "" {
		return filename, !strings.HasPrefix(filename, "/")
	}
	if !strings.HasPrefix(filename, "/") {
		return filename, true
	}
	if filename == root {
		return ".", true
	}
	prefix := strings.TrimSuffix(root, "/") + "/"
	if strings.HasPrefix(filename, prefix) {
		return strings.TrimPrefix(filename, prefix), true
	}
	return filename, false
}

// FileURI returns a file:// URI for an absolute filesystem path.
func FileURI(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}).String()
}

// InsideAnalysisRoot reports whether filename lies under root.
func InsideAnalysisRoot(root, filename string) bool {
	_, inside := normalizedPath(root, filename)
	return inside
}

// PortablePath is the machine-facing path policy used by JSON, SARIF, and
// finding fingerprints. Repository files are returned root-relative; files
// outside the root are returned as normalized absolute paths.
func PortablePath(root, filename string) string {
	path, _ := normalizedPath(root, filename)
	return path
}

// IsExternalPath reports whether filename is outside root.
func IsExternalPath(root, filename string) bool {
	_, inside := normalizedPath(root, filename)
	return !inside
}

// PortableURI converts a repository-relative path to a URI-compatible path.
// External files are emitted as file URIs so SARIF consumers do not mistake
// them for repository artifacts.
func PortableURI(root, filename string) string {
	path, inside := normalizedPath(root, filename)
	if inside {
		return path
	}
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) && !windowsStylePath(path) {
		return path
	}
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}

func windowsStylePath(path string) bool {
	path = strings.ReplaceAll(path, "\\", "/")
	return len(path) >= 3 && ((path[0] >= 'a' && path[0] <= 'z') || (path[0] >= 'A' && path[0] <= 'Z')) && path[1] == ':' && path[2] == '/'
}

func normalizedWindowsPath(root, filename string) (string, bool) {
	root = strings.TrimRight(strings.ReplaceAll(root, "\\", "/"), "/")
	filename = strings.ReplaceAll(filename, "\\", "/")
	if root == "" {
		return filename, false
	}
	if strings.EqualFold(filename, root) {
		return ".", true
	}
	if len(filename) > len(root) && strings.EqualFold(filename[:len(root)], root) && filename[len(root)] == '/' {
		return filename[len(root)+1:], true
	}
	return filename, false
}

func fingerprintPath(root, filename string) string {
	path, inside := normalizedPath(root, filename)
	if inside {
		return path
	}
	// External paths are not portable repository identities. Keep only a
	// stable opaque discriminator so the raw machine path is not embedded in
	// baseline content.
	sum := sha256.Sum256([]byte(filepath.Clean(filename)))
	return "external:" + hex.EncodeToString(sum[:])
}

func fileLineHash(filename string, line int) string {
	if filename == "" || line <= 0 {
		return ""
	}
	data, err := os.ReadFile(filename)
	if err == nil {
		lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		if line <= len(lines) {
			sum := sha256.Sum256([]byte(strings.TrimSuffix(lines[line-1], "\r")))
			return hex.EncodeToString(sum[:])
		}
	}
	sum := sha256.Sum256([]byte(filepath.ToSlash(filename) + ":" + strconv.Itoa(line)))
	return hex.EncodeToString(sum[:])
}
