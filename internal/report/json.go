// Package report owns machine-facing rendering entry points.
package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/ExtroNovosib/solidify/internal/analyzer"
)

type jsonRelatedLocation struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Message string `json:"message,omitempty"`
}

type jsonIssue struct {
	SchemaVersion      int                    `json:"schemaVersion"`
	FingerprintVersion int                    `json:"fingerprintVersion"`
	ID                 string                 `json:"id"`
	Fingerprint        string                 `json:"fingerprint"`
	Rule               string                 `json:"rule"`
	Severity           string                 `json:"severity"`
	Maturity           analyzer.Maturity      `json:"maturity"`
	Subject            string                 `json:"subject"`
	Identity           string                 `json:"identity"`
	File               string                 `json:"file"`
	Line               int                    `json:"line"`
	Column             int                    `json:"column"`
	EndLine            int                    `json:"endLine,omitempty"`
	EndColumn          int                    `json:"endColumn,omitempty"`
	HelpURI            string                 `json:"helpUri,omitempty"`
	Message            string                 `json:"message"`
	Evidence           string                 `json:"evidence"`
	Metrics            []analyzer.Metric      `json:"metrics,omitempty"`
	Groups             []analyzer.SymbolGroup `json:"groups,omitempty"`
	Related            []jsonRelatedLocation  `json:"relatedLocations,omitempty"`
}

// EncodeJSON serializes findings in the canonical schema version 3 format.
func EncodeJSON(issues []analyzer.Issue) ([]byte, error) {
	if err := analyzer.FinalizeIssues(issues, "workspace"); err != nil {
		return nil, err
	}
	out := make([]jsonIssue, 0, len(issues))
	for _, issue := range issues {
		if issue.Pos.Filename == "" || issue.Pos.Line < 1 || issue.Pos.Column < 1 {
			return nil, fmt.Errorf("%s has an invalid primary location", issue.ID())
		}
		if issue.End.Line > 0 && (issue.End.Line < issue.Pos.Line || issue.End.Line == issue.Pos.Line && issue.End.Column > 0 && issue.End.Column < issue.Pos.Column) {
			return nil, fmt.Errorf("%s has an invalid source range", issue.ID())
		}
		related, err := jsonRelatedLocations(issue)
		if err != nil {
			return nil, err
		}
		checkID := issue.Check
		if checkID == "" {
			checkID = analyzer.CheckID(issue.ID())
		}
		metadata, _ := analyzer.CheckMetadata(checkID)
		out = append(out, jsonIssue{
			SchemaVersion: 3, FingerprintVersion: 4, ID: issue.ID(), Fingerprint: issue.Fingerprint(),
			Rule: string(issue.Rule), Severity: string(issue.Severity), Maturity: metadata.Maturity,
			Subject: issue.Subject, Identity: issue.Identity, File: issue.PortablePath(),
			Line: issue.Pos.Line, Column: issue.Pos.Column, EndLine: issue.End.Line, EndColumn: issue.End.Column,
			HelpURI: analyzer.CheckHelpURI(checkID), Message: issue.Message, Evidence: issue.Evidence,
			Metrics: issue.Metrics, Groups: issue.Groups, Related: related,
		})
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(out); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func jsonRelatedLocations(issue analyzer.Issue) ([]jsonRelatedLocation, error) {
	positions := append([]analyzer.RelatedLocation(nil), issue.Related...)
	sort.SliceStable(positions, func(i, j int) bool {
		left, right := positions[i], positions[j]
		leftPath := analyzer.PortablePathForIssue(issue, left.Pos.Filename)
		rightPath := analyzer.PortablePathForIssue(issue, right.Pos.Filename)
		if leftPath != rightPath {
			return leftPath < rightPath
		}
		if left.Pos.Line != right.Pos.Line {
			return left.Pos.Line < right.Pos.Line
		}
		return left.Pos.Column < right.Pos.Column
	})
	related := make([]jsonRelatedLocation, 0, len(positions))
	for _, location := range positions {
		if location.Pos.Filename == "" || location.Pos.Line < 1 || location.Pos.Column < 1 {
			return nil, fmt.Errorf("%s has an invalid related location", issue.ID())
		}
		related = append(related, jsonRelatedLocation{
			File: analyzer.PortablePathForIssue(issue, location.Pos.Filename),
			Line: location.Pos.Line, Column: location.Pos.Column, Message: location.Message,
		})
	}
	return related, nil
}
