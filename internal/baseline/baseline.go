// Package baseline owns versioned finding acceptance documents.
package baseline

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ExtroNovosib/solidify/internal/analyzer"
)

const (
	Version       = 5
	LegacyVersion = 4
)

var fingerprintPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Entry struct {
	Fingerprint string           `json:"fingerprint"`
	CheckID     analyzer.CheckID `json:"checkId"`
	Path        string           `json:"path"`
	Subject     string           `json:"subject"`
	Reason      string           `json:"reason"`
	Owner       string           `json:"owner,omitempty"`
	Expires     string           `json:"expires,omitempty"`
}

type Document struct {
	Version int     `json:"version"`
	Entries []Entry `json:"entries"`
}

type Annotation struct {
	Reason  string
	Owner   string
	Expires string
}

type DiffResult struct {
	Added []analyzer.Issue
	Stale []Entry
	Live  []Entry
}

type legacyDocument struct {
	Version      int      `json:"version"`
	Fingerprints []string `json:"fingerprints"`
}

// Load reads v5 documents and converts v4 documents into unannotated entries.
func Load(path string) (Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Document{}, err
	}
	var header struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return Document{}, err
	}
	switch header.Version {
	case LegacyVersion:
		var legacy legacyDocument
		if err := decodeStrict(data, &legacy); err != nil {
			return Document{}, err
		}
		document := Document{Version: LegacyVersion, Entries: make([]Entry, 0, len(legacy.Fingerprints))}
		for index, fingerprint := range legacy.Fingerprints {
			if !fingerprintPattern.MatchString(fingerprint) {
				return Document{}, fmt.Errorf("baseline fingerprint %d must be a 64-character lowercase v4 hash", index)
			}
			document.Entries = append(document.Entries, Entry{Fingerprint: fingerprint})
		}
		sortEntries(document.Entries)
		return document, nil
	case Version:
		var document Document
		if err := decodeStrict(data, &document); err != nil {
			return Document{}, err
		}
		if err := validateDocument(document); err != nil {
			return Document{}, err
		}
		sortEntries(document.Entries)
		return document, nil
	default:
		return Document{}, fmt.Errorf("baseline version %d is unsupported; expected v4 or v5", header.Version)
	}
}

func decodeStrict(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("multiple JSON documents are not supported")
		}
		return err
	}
	return nil
}

func validateDocument(document Document) error {
	seen := map[string]bool{}
	for index, entry := range document.Entries {
		if !fingerprintPattern.MatchString(entry.Fingerprint) {
			return fmt.Errorf("baseline entry %d fingerprint must be a 64-character lowercase v4 hash", index)
		}
		if seen[entry.Fingerprint] {
			return fmt.Errorf("baseline entry %d duplicates fingerprint %s", index, entry.Fingerprint)
		}
		seen[entry.Fingerprint] = true
		if _, ok := analyzer.CheckMetadata(entry.CheckID); !ok {
			return fmt.Errorf("baseline entry %d has unknown check ID %q", index, entry.CheckID)
		}
		if strings.TrimSpace(entry.Path) == "" || strings.TrimSpace(entry.Subject) == "" {
			return fmt.Errorf("baseline entry %d requires path and subject", index)
		}
		if err := ValidateReason(entry.Reason); err != nil {
			return fmt.Errorf("baseline entry %d: %w", index, err)
		}
		if entry.Expires != "" {
			if _, err := time.Parse("2006-01-02", entry.Expires); err != nil {
				return fmt.Errorf("baseline entry %d expiry must use YYYY-MM-DD", index)
			}
		}
	}
	return nil
}

func ValidateReason(reason string) error {
	reason = strings.TrimSpace(reason)
	if len(reason) < 12 {
		return fmt.Errorf("baseline reason must contain at least 12 characters")
	}
	switch strings.ToLower(reason) {
	case "accepted for now", "todo", "ignore", "temporary":
		return fmt.Errorf("baseline reason %q is a placeholder", reason)
	}
	return nil
}

// Read returns the compatibility fingerprint set for filtering.
func Read(path string) (map[string]bool, error) {
	document, err := Load(path)
	if err != nil {
		return nil, err
	}
	accepted := make(map[string]bool, len(document.Entries))
	for _, entry := range document.Entries {
		accepted[entry.Fingerprint] = true
	}
	return accepted, nil
}

// Write creates a canonical v5 document. A reason is mandatory when findings
// are accepted; the variadic form preserves source compatibility for empty
// baselines while preventing unannotated debt.
func Write(path string, issues []analyzer.Issue, reason ...string) error {
	annotation := Annotation{}
	if len(reason) > 0 {
		annotation.Reason = reason[0]
	}
	document, _, err := Update(Document{Version: Version}, issues, annotation, true)
	if err != nil {
		return err
	}
	return WriteDocument(path, document)
}

func Diff(document Document, current []analyzer.Issue) DiffResult {
	currentByFingerprint := make(map[string]analyzer.Issue, len(current))
	for _, issue := range current {
		currentByFingerprint[issue.Fingerprint()] = issue
	}
	existing := make(map[string]Entry, len(document.Entries))
	result := DiffResult{}
	for _, entry := range document.Entries {
		existing[entry.Fingerprint] = entry
		if _, ok := currentByFingerprint[entry.Fingerprint]; ok {
			result.Live = append(result.Live, entry)
		} else {
			result.Stale = append(result.Stale, entry)
		}
	}
	for _, issue := range current {
		if _, ok := existing[issue.Fingerprint()]; !ok {
			result.Added = append(result.Added, issue)
		}
	}
	sort.Slice(result.Added, func(i, j int) bool { return result.Added[i].Fingerprint() < result.Added[j].Fingerprint() })
	sortEntries(result.Stale)
	sortEntries(result.Live)
	return result
}

func Update(document Document, current []analyzer.Issue, annotation Annotation, prune bool) (Document, DiffResult, error) {
	if err := analyzer.FinalizeIssues(current, "workspace"); err != nil {
		return Document{}, DiffResult{}, err
	}
	diff := Diff(document, current)
	requiresReason := len(diff.Added) > 0 || document.Version == LegacyVersion && len(diff.Live) > 0
	if requiresReason {
		if err := ValidateReason(annotation.Reason); err != nil {
			return Document{}, diff, err
		}
	}
	entries := make([]Entry, 0, len(current)+len(diff.Stale))
	currentByFingerprint := make(map[string]analyzer.Issue, len(current))
	for _, issue := range current {
		currentByFingerprint[issue.Fingerprint()] = issue
	}
	for _, entry := range diff.Live {
		issue := currentByFingerprint[entry.Fingerprint]
		if entry.Reason == "" {
			entry = entryForIssue(issue, annotation)
		} else {
			entry.CheckID, entry.Path, entry.Subject = issue.Check, issue.PortablePath(), issue.Subject
		}
		entries = append(entries, entry)
	}
	for _, issue := range diff.Added {
		entries = append(entries, entryForIssue(issue, annotation))
	}
	if !prune {
		entries = append(entries, diff.Stale...)
	}
	sortEntries(entries)
	result := Document{Version: Version, Entries: entries}
	if err := validateDocument(result); err != nil {
		return Document{}, diff, err
	}
	return result, diff, nil
}

func Prune(document Document, current []analyzer.Issue) (Document, DiffResult, error) {
	return Update(document, current, Annotation{}, true)
}

func entryForIssue(issue analyzer.Issue, annotation Annotation) Entry {
	return Entry{
		Fingerprint: issue.Fingerprint(), CheckID: issue.Check, Path: issue.PortablePath(),
		Subject: issue.Subject, Reason: strings.TrimSpace(annotation.Reason),
		Owner: strings.TrimSpace(annotation.Owner), Expires: strings.TrimSpace(annotation.Expires),
	}
}

func WriteDocument(path string, document Document) error {
	document.Version = Version
	sortEntries(document.Entries)
	if err := validateDocument(document); err != nil {
		return err
	}
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	destination := filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(destination), ".solidlint-baseline-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer func() { _ = os.Remove(tempName) }()
	if err := temp.Chmod(0o644); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(append(data, '\n')); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, destination)
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

func sortEntries(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool { return entries[i].Fingerprint < entries[j].Fingerprint })
}
