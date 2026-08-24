package report

import (
	"encoding/json"
	"io"
	"sort"

	"github.com/ExtroNovosib/solidify/internal/analyzer"
)

const sarifURIBaseID = "ROOT"

type SARIFMetadata struct {
	ToolName    string
	ToolVersion string
}

type sarifText struct {
	Text string `json:"text"`
}

type sarifArtifactLocation struct {
	URI       string `json:"uri"`
	URIBaseID string `json:"uriBaseId,omitempty"`
}

type sarifRegion struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn,omitempty"`
	EndLine     int `json:"endLine,omitempty"`
	EndColumn   int `json:"endColumn,omitempty"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           sarifRegion           `json:"region"`
}

type sarifRule struct {
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	HelpURI              string    `json:"helpUri,omitempty"`
	ShortDescription     sarifText `json:"shortDescription"`
	FullDescription      sarifText `json:"fullDescription,omitempty"`
	DefaultConfiguration struct {
		Level string `json:"level"`
	} `json:"defaultConfiguration"`
}

type sarifReplacement struct {
	DeletedRegion   sarifRegion `json:"deletedRegion"`
	InsertedContent sarifText   `json:"insertedContent"`
}

type sarifArtifactChange struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Replacements     []sarifReplacement    `json:"replacements"`
}

type sarifFix struct {
	Description     sarifText             `json:"description"`
	ArtifactChanges []sarifArtifactChange `json:"artifactChanges"`
}

type sarifRelatedLocation struct {
	ID               int                   `json:"id"`
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
	Message          sarifText             `json:"message,omitempty"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifResult struct {
	RuleID              string                 `json:"ruleId"`
	Level               string                 `json:"level"`
	Message             sarifText              `json:"message"`
	Locations           []sarifLocation        `json:"locations"`
	RelatedLocations    []sarifRelatedLocation `json:"relatedLocations,omitempty"`
	PartialFingerprints map[string]string      `json:"partialFingerprints,omitempty"`
	Properties          map[string]any         `json:"properties,omitempty"`
	Fixes               []sarifFix             `json:"fixes,omitempty"`
}

type sarifDriver struct {
	Name    string      `json:"name"`
	Version string      `json:"version"`
	Rules   []sarifRule `json:"rules"`
}

type sarifRun struct {
	OriginalURIBaseIDs map[string]sarifArtifactLocation `json:"originalUriBaseIds,omitempty"`
	Tool               struct {
		Driver sarifDriver `json:"driver"`
	} `json:"tool"`
	Results []sarifResult `json:"results"`
}

// EncodeSARIF writes a deterministic SARIF 2.1.0 report. The caller owns the
// build metadata so report rendering remains independent of CLI globals.
func EncodeSARIF(writer io.Writer, issues []analyzer.Issue, metadata SARIFMetadata) error {
	ordered := append([]analyzer.Issue(nil), issues...)
	sort.SliceStable(ordered, func(i, j int) bool {
		left, right := ordered[i], ordered[j]
		if left.PortablePath() != right.PortablePath() {
			return left.PortablePath() < right.PortablePath()
		}
		if left.Pos.Line != right.Pos.Line {
			return left.Pos.Line < right.Pos.Line
		}
		if left.Pos.Column != right.Pos.Column {
			return left.Pos.Column < right.Pos.Column
		}
		if left.ID() != right.ID() {
			return left.ID() < right.ID()
		}
		return left.Evidence < right.Evidence
	})
	analysisRoot := firstAnalysisRoot(ordered)
	reportRun := newSARIFRun(analysisRoot, metadata)
	rules := map[string]sarifRule{}
	for _, issue := range ordered {
		item, rule := sarifResultForIssue(issue, analysisRoot)
		reportRun.Results = append(reportRun.Results, item)
		if _, exists := rules[rule.ID]; !exists {
			rules[rule.ID] = rule
		}
	}
	ruleIDs := make([]string, 0, len(rules))
	for id := range rules {
		ruleIDs = append(ruleIDs, id)
	}
	sort.Strings(ruleIDs)
	for _, id := range ruleIDs {
		reportRun.Tool.Driver.Rules = append(reportRun.Tool.Driver.Rules, rules[id])
	}
	document := struct {
		Version string     `json:"version"`
		Schema  string     `json:"$schema"`
		Runs    []sarifRun `json:"runs"`
	}{"2.1.0", "https://json.schemastore.org/sarif-2.1.0.json", []sarifRun{reportRun}}
	return json.NewEncoder(writer).Encode(document)
}

func firstAnalysisRoot(issues []analyzer.Issue) string {
	for _, issue := range issues {
		if root := issue.AnalysisRoot(); root != "" {
			return root
		}
	}
	return ""
}

func newSARIFRun(analysisRoot string, metadata SARIFMetadata) sarifRun {
	if metadata.ToolName == "" {
		metadata.ToolName = "solidlint"
	}
	var result sarifRun
	result.Tool.Driver.Name = metadata.ToolName
	result.Tool.Driver.Version = metadata.ToolVersion
	result.Tool.Driver.Rules = []sarifRule{}
	result.Results = []sarifResult{}
	if baseURI := analyzer.FileURI(analysisRoot); analysisRoot != "" && baseURI != "" {
		result.OriginalURIBaseIDs = map[string]sarifArtifactLocation{sarifURIBaseID: {URI: baseURI}}
	}
	return result
}

func sarifResultForIssue(issue analyzer.Issue, analysisRoot string) (sarifResult, sarifRule) {
	checkID := issue.Check
	if checkID == "" {
		checkID = analyzer.CheckID(issue.ID())
	}
	item := sarifResult{
		RuleID: issue.ID(), Level: string(issue.Severity), Message: sarifText{Text: issue.Message},
		PartialFingerprints: map[string]string{
			"primaryLocationLineHash": issue.PrimaryLocationLineHash(), "solidlint/v4": issue.Fingerprint(),
		},
		Properties: map[string]any{
			"subject": issue.Subject, "identity": issue.Identity, "evidence": issue.Evidence,
			"metrics": issue.Metrics, "groups": issue.Groups,
		},
	}
	if metadata, found := analyzer.CheckMetadata(checkID); found {
		item.Properties["maturity"] = metadata.Maturity
	}
	item.Locations = []sarifLocation{{PhysicalLocation: sarifPhysicalLocation{
		ArtifactLocation: sarifArtifact(issue, issue.Pos.Filename, analysisRoot),
		Region:           sarifRegion{StartLine: issue.Pos.Line, StartColumn: issue.Pos.Column, EndLine: issue.End.Line, EndColumn: issue.End.Column},
	}}}
	item.RelatedLocations = sarifRelatedLocations(issue, analysisRoot)
	item.Fixes = sarifFixes(issue, analysisRoot)
	return item, sarifRuleForIssue(issue, checkID)
}

func sarifArtifact(issue analyzer.Issue, filename, analysisRoot string) sarifArtifactLocation {
	location := sarifArtifactLocation{URI: analyzer.PortableURIForIssue(issue, filename)}
	if analysisRoot != "" && analyzer.InsideAnalysisRoot(analysisRoot, filename) {
		location.URIBaseID = sarifURIBaseID
	}
	return location
}

func sarifRelatedLocations(issue analyzer.Issue, analysisRoot string) []sarifRelatedLocation {
	related := append([]analyzer.RelatedLocation(nil), issue.Related...)
	sort.SliceStable(related, func(i, j int) bool {
		left, right := related[i], related[j]
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
	locations := make([]sarifRelatedLocation, 0, len(related))
	for index, location := range related {
		locations = append(locations, sarifRelatedLocation{
			ID: index + 1,
			PhysicalLocation: sarifPhysicalLocation{
				ArtifactLocation: sarifArtifact(issue, location.Pos.Filename, analysisRoot),
				Region:           sarifRegion{StartLine: location.Pos.Line, StartColumn: location.Pos.Column},
			},
			Message: sarifText{Text: location.Message},
		})
	}
	return locations
}

func sarifFixes(issue analyzer.Issue, analysisRoot string) []sarifFix {
	fixes := make([]sarifFix, 0, len(issue.SuggestedFixes))
	for _, fix := range issue.SuggestedFixes {
		changes := map[string]*sarifArtifactChange{}
		edits := append([]analyzer.TextEdit(nil), fix.Edits...)
		sort.SliceStable(edits, func(i, j int) bool {
			if edits[i].Filename != edits[j].Filename {
				return edits[i].Filename < edits[j].Filename
			}
			if edits[i].Start.Line != edits[j].Start.Line {
				return edits[i].Start.Line < edits[j].Start.Line
			}
			return edits[i].Start.Column < edits[j].Start.Column
		})
		for _, edit := range edits {
			if edit.Filename == "" || edit.Start.Line <= 0 {
				continue
			}
			filename := analyzer.PortableURIForIssue(issue, edit.Filename)
			change := changes[filename]
			if change == nil {
				change = &sarifArtifactChange{ArtifactLocation: sarifArtifact(issue, edit.Filename, analysisRoot)}
				changes[filename] = change
			}
			end := edit.End
			if end.Line <= 0 {
				end = edit.Start
			}
			change.Replacements = append(change.Replacements, sarifReplacement{
				DeletedRegion:   sarifRegion{StartLine: edit.Start.Line, StartColumn: edit.Start.Column, EndLine: end.Line, EndColumn: end.Column},
				InsertedContent: sarifText{Text: edit.NewText},
			})
		}
		filenames := make([]string, 0, len(changes))
		for filename := range changes {
			filenames = append(filenames, filename)
		}
		sort.Strings(filenames)
		if len(filenames) == 0 {
			continue
		}
		entry := sarifFix{Description: sarifText{Text: fix.Message}}
		for _, filename := range filenames {
			entry.ArtifactChanges = append(entry.ArtifactChanges, *changes[filename])
		}
		fixes = append(fixes, entry)
	}
	return fixes
}

func sarifRuleForIssue(issue analyzer.Issue, checkID analyzer.CheckID) sarifRule {
	description := analyzer.CheckDoc(checkID)
	if description == "" {
		description = issue.Message
	}
	name := issue.ID()
	defaultSeverity := issue.Severity
	if metadata, found := analyzer.CheckMetadata(checkID); found {
		name = metadata.Name
		defaultSeverity = metadata.DefaultSev
	}
	rule := sarifRule{
		ID: issue.ID(), Name: name, HelpURI: analyzer.CheckHelpURI(checkID),
		ShortDescription: sarifText{Text: description}, FullDescription: sarifText{Text: description},
	}
	rule.DefaultConfiguration.Level = string(defaultSeverity)
	return rule
}
