// Package baseline owns versioned finding acceptance documents. It deliberately
// depends only on analyzer's stable Issue contract and never on CLI concerns.
package baseline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/ExtroNovosib/solidify/internal/analyzer"
)

const Version = 4

var fingerprintPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Document struct {
	Version      int      `json:"version"`
	Fingerprints []string `json:"fingerprints"`
}

func Write(path string, issues []analyzer.Issue) error {
	if err := analyzer.FinalizeIssues(issues, "workspace"); err != nil {
		return err
	}
	seen := map[string]bool{}
	values := make([]string, 0, len(issues))
	for _, issue := range issues {
		fingerprint := issue.Fingerprint()
		if !seen[fingerprint] {
			seen[fingerprint] = true
			values = append(values, fingerprint)
		}
	}
	sort.Strings(values)
	data, err := json.MarshalIndent(Document{Version: Version, Fingerprints: values}, "", "  ")
	if err != nil {
		return err
	}
	destination := filepath.Clean(path)
	tmp, err := os.CreateTemp(filepath.Dir(destination), ".solidlint-baseline-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, destination)
}

func Read(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if doc.Version != Version {
		return nil, fmt.Errorf("baseline version %d is unsupported; regenerate it with solidlint baseline v4", doc.Version)
	}
	accepted := make(map[string]bool, len(doc.Fingerprints))
	for index, fingerprint := range doc.Fingerprints {
		if !fingerprintPattern.MatchString(fingerprint) {
			return nil, fmt.Errorf("baseline fingerprint %d must be a 64-character lowercase v4 hash", index)
		}
		accepted[fingerprint] = true
	}
	return accepted, nil
}

func Filter(issues []analyzer.Issue, accepted map[string]bool) []analyzer.Issue {
	out := issues[:0]
	for _, issue := range issues {
		if !accepted[issue.Fingerprint()] {
			out = append(out, issue)
		}
	}
	return out
}

func Stale(accepted map[string]bool, current []analyzer.Issue) []string {
	present := make(map[string]bool, len(current))
	for _, issue := range current {
		present[issue.Fingerprint()] = true
	}
	var stale []string
	for fingerprint := range accepted {
		if !present[fingerprint] {
			stale = append(stale, fingerprint)
		}
	}
	sort.Strings(stale)
	return stale
}
