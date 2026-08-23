package analyzer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

type jsonRelatedLocation struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Message string `json:"message,omitempty"`
}

type jsonIssue struct {
	SchemaVersion      int                   `json:"schemaVersion"`
	FingerprintVersion int                   `json:"fingerprintVersion"`
	ID                 string                `json:"id"`
	Fingerprint        string                `json:"fingerprint"`
	Rule               string                `json:"rule"`
	Severity           string                `json:"severity"`
	Maturity           Maturity              `json:"maturity"`
	Subject            string                `json:"subject"`
	Identity           string                `json:"identity"`
	File               string                `json:"file"`
	Line               int                   `json:"line"`
	Column             int                   `json:"column"`
	EndLine            int                   `json:"endLine,omitempty"`
	EndColumn          int                   `json:"endColumn,omitempty"`
	HelpURI            string                `json:"helpUri,omitempty"`
	Message            string                `json:"message"`
	Evidence           string                `json:"evidence"`
	Metrics            []Metric              `json:"metrics,omitempty"`
	Groups             []SymbolGroup         `json:"groups,omitempty"`
	Related            []jsonRelatedLocation `json:"relatedLocations,omitempty"`
}

// EncodeIssuesJSON serializes findings in the canonical schema version 3 JSON format.
func EncodeIssuesJSON(issues []Issue) ([]byte, error) {
	if err := FinalizeIssues(issues, "workspace"); err != nil {
		return nil, err
	}
	out := make([]jsonIssue, 0, len(issues))
	for _, is := range issues {
		if is.Pos.Filename == "" || is.Pos.Line < 1 || is.Pos.Column < 1 {
			return nil, fmt.Errorf("%s has an invalid primary location", is.ID())
		}
		if is.End.Line > 0 && (is.End.Line < is.Pos.Line || is.End.Line == is.Pos.Line && is.End.Column > 0 && is.End.Column < is.Pos.Column) {
			return nil, fmt.Errorf("%s has an invalid source range", is.ID())
		}
		related := make([]jsonRelatedLocation, 0, len(is.Related))
		relatedPositions := append([]RelatedLocation(nil), is.Related...)
		sort.SliceStable(relatedPositions, func(i, j int) bool {
			left, right := relatedPositions[i], relatedPositions[j]
			leftPath := PortablePathForIssue(is, left.Pos.Filename)
			rightPath := PortablePathForIssue(is, right.Pos.Filename)
			if leftPath != rightPath {
				return leftPath < rightPath
			}
			if left.Pos.Line != right.Pos.Line {
				return left.Pos.Line < right.Pos.Line
			}
			return left.Pos.Column < right.Pos.Column
		})
		for _, location := range relatedPositions {
			if location.Pos.Filename == "" || location.Pos.Line < 1 || location.Pos.Column < 1 {
				return nil, fmt.Errorf("%s has an invalid related location", is.ID())
			}
			related = append(related, jsonRelatedLocation{
				File:    PortablePathForIssue(is, location.Pos.Filename),
				Line:    location.Pos.Line,
				Column:  location.Pos.Column,
				Message: location.Message,
			})
		}
		checkID := is.Check
		if checkID == "" {
			checkID = CheckID(is.ID())
		}
		metadata, _ := CheckMetadata(checkID)
		out = append(out, jsonIssue{
			SchemaVersion:      3,
			FingerprintVersion: 4,
			ID:                 is.ID(),
			Fingerprint:        is.Fingerprint(),
			Rule:               string(is.Rule),
			Severity:           string(is.Severity),
			Maturity:           metadata.Maturity,
			Subject:            is.Subject,
			Identity:           is.Identity,
			File:               is.PortablePath(),
			Line:               is.Pos.Line,
			Column:             is.Pos.Column,
			EndLine:            is.End.Line,
			EndColumn:          is.End.Column,
			HelpURI:            CheckHelpURI(checkID),
			Message:            is.Message,
			Evidence:           is.Evidence,
			Metrics:            is.Metrics,
			Groups:             is.Groups,
			Related:            related,
		})
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
