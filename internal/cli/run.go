// Command solidlint is a typed, heuristic static analyzer that
// checks Go source for common violations of the SOLID design principles:
//
//	S — Single Responsibility  (types/functions doing too much)
//	O — Open/Closed            (switching on concrete type instead of polymorphism)
//	L — Liskov Substitution    (methods that unconditionally panic / "not implemented")
//	I — Interface Segregation  (fat interfaces)
//	D — Dependency Inversion   (structs wired directly to concrete collaborators)
//
// Usage:
//
//	solidlint [flags] <path> [<path> ...]
//
// Example:
//
//	solidlint ./...
//	solidlint ./internal/analyzer/
//	solidlint -rules=S,I -max-methods=8 ./internal/...
//	solidlint -format=json ./cmd/myapp > report.json
//
// Type-level SRP metrics aggregate receivers across every file in a package.
// Scan package directories (for example ./internal/analyzer/), not single
// definition files such as run.go.
package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/ExtroNovosib/solidify/internal/analyzer"
	baselinepkg "github.com/ExtroNovosib/solidify/internal/baseline"
	"github.com/ExtroNovosib/solidify/internal/report"
)

type BuildInfo struct {
	Version   string
	Commit    string
	BuildDate string
}

var activeBuild BuildInfo

func Run(args []string, build BuildInfo) int {
	activeBuild = build
	fs := flag.NewFlagSet("solidlint", flag.ContinueOnError)

	cfg := analyzer.DefaultConfig()
	cfg.Profile = analyzer.ProfileStable
	var (
		rulesFlag     = fs.String("rules", "S,O,L,I,D", "comma-separated list of rules to run (S,O,L,I,D)")
		profileFlag   = fs.String("profile", string(analyzer.ProfileStable), "check profile: stable|all")
		enableChecks  = fs.String("enable-checks", "", "comma-separated concrete check IDs to enable")
		format        = fs.String("format", "text", "output format: text|json|sarif")
		analysis      = fs.String("analysis", "auto", "analysis mode: auto|syntax|types")
		showVersion   = fs.Bool("version", false, "print version and exit")
		tests         = fs.Bool("tests", false, "also lint _test.go files")
		failOnFind    = fs.Bool("fail", true, "exit with a non-zero status if any issue is found")
		failLevel     = fs.String("fail-level", string(analyzer.SeverityWarning), "minimum severity that fails: note|warning|error")
		maxMethods    = fs.Int("max-methods", cfg.MaxMethodsPerType, "SRP: max methods per type")
		maxFuncLine   = fs.Int("max-func-lines", cfg.MaxFuncLines, "SRP: max lines per function body")
		maxParams     = fs.Int("max-params", cfg.MaxFuncParams, "SRP: max cohesive parameters before mixed/repeated lists are reported")
		maxSwitch     = fs.Int("max-switch-cases", cfg.MaxTypeSwitchCases, "OCP: max cases in a type switch / type-assertion chain")
		maxIfaceM     = fs.Int("max-interface-methods", cfg.MaxInterfaceMethods, "ISP: max methods per interface")
		ispMinM       = fs.Int("isp-min-methods", cfg.ISPMinMethods, "ISP: minimum interface methods for usage-ratio and stub checks")
		ispUsagePct   = fs.Int("isp-usage-ratio-percent", cfg.ISPUsageRatioPercent, "ISP: flag clients using fewer than this percent of interface methods")
		configPath    = fs.String("config", "", "path to .solidify.yml (default: discover)")
		baseline      = fs.String("baseline", "", "JSON baseline of accepted finding fingerprints")
		writeBase     = fs.String("write-baseline", "", "write current findings as a baseline JSON file")
		baselineStale = fs.String("baseline-stale", "warn", "stale baseline policy: ignore|warn|error")
		cacheDir      = fs.String("cache-dir", "", "cache directory (default: platform user cache)")
		cache         = fs.Bool("cache", true, "enable the analysis cache")
		cacheDebug    = fs.Bool("cache-debug", false, "print cache diagnostics to stderr")
		printConfig   = fs.Bool("print-config", false, "print effective configuration and exit")
	)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "solidlint checks Go source for heuristic SOLID principle violations.")
		fmt.Fprintln(fs.Output(), "\nUsage: solidlint [flags] <path> [<path> ...]")
		fmt.Fprintln(fs.Output(), "\nType-level SRP metrics (large-type, god-type, cohesion) require package")
		fmt.Fprintln(fs.Output(), "directory scans when receivers span multiple files in the same package.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Fprintln(os.Stdout, activeBuild.Version)
		return 0
	}
	if *format != "text" && *format != "json" && *format != "sarif" {
		fmt.Fprintf(os.Stderr, "solidlint: unknown format %q (expected text, json, or sarif)\n", *format)
		return 2
	}
	if *analysis != "auto" && *analysis != "syntax" && *analysis != "types" {
		fmt.Fprintf(os.Stderr, "solidlint: unknown analysis mode %q (expected auto, syntax, or types)\n", *analysis)
		return 2
	}
	if *profileFlag != string(analyzer.ProfileStable) && *profileFlag != string(analyzer.ProfileAll) {
		fmt.Fprintf(os.Stderr, "solidlint: unknown profile %q (expected stable or all)\n", *profileFlag)
		return 2
	}
	if !validFailLevel(*failLevel) {
		fmt.Fprintf(os.Stderr, "solidlint: unknown fail level %q (expected note, warning, or error)\n", *failLevel)
		return 2
	}
	if *baselineStale != "ignore" && *baselineStale != "warn" && *baselineStale != "error" {
		fmt.Fprintf(os.Stderr, "solidlint: unknown baseline stale policy %q (expected ignore, warn, or error)\n", *baselineStale)
		return 2
	}

	paths := fs.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}

	// Configuration supplies defaults; explicit command-line values take
	// precedence (the flag package does not expose this directly, so inspect
	// visited flags after loading the file).
	configFile := *configPath
	configDiscovered := false
	var err error
	if configFile == "" {
		configFile, err = analyzer.FindConfigForTargets(paths)
		if err != nil {
			fmt.Fprintln(os.Stderr, "solidlint:", err)
			return 2
		}
		configDiscovered = configFile != ""
	}
	var fileConfig analyzer.FileConfig
	if configFile != "" {
		fileConfig, err = analyzer.LoadFileConfig(configFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "solidlint:", err)
			return 2
		}
		fileConfig.Apply(&cfg)
		if configDiscovered {
			fmt.Fprintln(os.Stderr, "solidlint: using config", configFile, "(discovered from scan target)")
		}
	}
	if flagWasSet(fs, "max-methods") {
		cfg.MaxMethodsPerType = *maxMethods
	}
	if flagWasSet(fs, "max-func-lines") {
		cfg.MaxFuncLines = *maxFuncLine
	}
	if flagWasSet(fs, "max-params") {
		cfg.MaxFuncParams = *maxParams
	}
	if flagWasSet(fs, "max-switch-cases") {
		cfg.MaxTypeSwitchCases = *maxSwitch
	}
	if flagWasSet(fs, "max-interface-methods") {
		cfg.MaxInterfaceMethods = *maxIfaceM
	}
	if flagWasSet(fs, "isp-min-methods") {
		cfg.ISPMinMethods = *ispMinM
	}
	if flagWasSet(fs, "isp-usage-ratio-percent") {
		cfg.ISPUsageRatioPercent = *ispUsagePct
	}
	if fileConfig.FailLevel != "" && !flagWasSet(fs, "fail-level") {
		*failLevel = string(fileConfig.FailLevel)
	}
	if flagWasSet(fs, "profile") || fileConfig.Profile == "" {
		cfg.Profile = analyzer.Profile(*profileFlag)
	}
	if flagWasSet(fs, "enable-checks") {
		cfg.EnabledChecks = parseCheckIDs(*enableChecks)
	}
	if !validFailLevel(*failLevel) {
		fmt.Fprintf(os.Stderr, "solidlint: unknown fail level %q (expected note, warning, or error)\n", *failLevel)
		return 2
	}
	cfg.CacheDir = *cacheDir
	cfg.CacheEnabled = *cache
	cfg.CacheDiagnostics = *cacheDebug
	cfg.AnalysisMode = *analysis
	cfg.IncludeTests = *tests
	cfg.ToolVersion = activeBuild.Version
	cfg.ExcludedFiles = append([]string(nil), fileConfig.Excludes...)
	if err := analyzer.ValidateConfig(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "solidlint:", err)
		return 2
	}

	ruleSpec := *rulesFlag
	if len(fileConfig.EnabledRules) > 0 && !flagWasSet(fs, "rules") {
		ruleSpec = strings.Join(fileConfig.EnabledRules, ",")
	}
	enabled, err := parseRules(ruleSpec)
	if err != nil {
		fmt.Fprintln(os.Stderr, "solidlint:", err)
		return 2
	}
	selection, err := analyzer.ResolveCheckSelection(cfg.Profile, enabled, cfg.EnabledChecks, cfg.DisabledChecks)
	if err != nil {
		fmt.Fprintln(os.Stderr, "solidlint:", err)
		return 2
	}
	if *printConfig {
		if err := printEffectiveConfig(cfg, enabled, selection, *failLevel, configFile); err != nil {
			fmt.Fprintln(os.Stderr, "solidlint:", err)
			return 2
		}
		return 0
	}

	pkgs, loadWarnings, err := analyzer.LoadWorkspace(paths, *tests, *analysis)
	if err != nil {
		fmt.Fprintln(os.Stderr, "solidlint:", err)
		return 2
	}
	for _, warning := range loadWarnings {
		fmt.Fprintln(os.Stderr, "solidlint: warning:", warning)
	}
	if err := analyzer.ValidateSuppressions(pkgs); err != nil {
		fmt.Fprintln(os.Stderr, "solidlint:", err)
		return 2
	}
	allIssues := analyzer.Run(pkgs, cfg, enabled)
	if err := analyzer.FinalizeIssues(allIssues, "workspace"); err != nil {
		fmt.Fprintln(os.Stderr, "solidlint: internal analysis error:", err)
		return 2
	}
	analyzer.ApplySeverity(allIssues, fileConfig.Severities)
	allIssues = filterIssues(allIssues, fileConfig.Excludes)
	if *writeBase != "" {
		if err := writeBaseline(*writeBase, allIssues); err != nil {
			fmt.Fprintln(os.Stderr, "solidlint:", err)
			return 2
		}
	}
	if *baseline != "" {
		accepted, _, err := readBaselineInfo(*baseline)
		if err != nil {
			fmt.Fprintln(os.Stderr, "solidlint:", err)
			return 2
		}
		if stale := staleBaseline(accepted, allIssues); len(stale) > 0 {
			if *baselineStale != "ignore" {
				fmt.Fprintf(os.Stderr, "solidlint: baseline contains %d stale fingerprint(s)\n", len(stale))
			}
			if *baselineStale == "error" {
				return 1
			}
		}
		allIssues = filterBaseline(allIssues, accepted)
	}

	if allPathsAreGoFiles(paths) {
		fmt.Fprintln(os.Stderr, "solidlint: tip: type-level SRP metrics (large-type, god-type, cohesion) aggregate receivers across all files in a package; scan the package directory instead of a single .go file")
	}

	switch *format {
	case "json":
		data, err := report.EncodeJSON(allIssues)
		if err != nil && !isBrokenPipe(err) {
			fmt.Fprintln(os.Stderr, "solidlint:", err)
			return 2
		}
		if _, err := os.Stdout.Write(data); err != nil && !isBrokenPipe(err) {
			fmt.Fprintln(os.Stderr, "solidlint:", err)
			return 2
		}
	case "text":
		for _, is := range allIssues {
			if metadata, ok := analyzer.CheckMetadata(is.Check); cfg.Profile == analyzer.ProfileAll && ok && metadata.Maturity == analyzer.MaturityExperimental {
				fmt.Println(is.String(), "[experimental]")
			} else {
				fmt.Println(is.String())
			}
		}
		fmt.Printf("\n%d issue(s) found\n", len(allIssues))
	case "sarif":
		if err := writeSARIF(os.Stdout, allIssues); err != nil && !isBrokenPipe(err) {
			fmt.Fprintln(os.Stderr, "solidlint:", err)
			return 2
		}
	}

	if *failOnFind && hasSeverityAtLeast(allIssues, analyzer.Severity(*failLevel)) {
		return 1
	}
	return 0
}

func validFailLevel(level string) bool {
	return level == string(analyzer.SeverityNote) || level == string(analyzer.SeverityWarning) || level == string(analyzer.SeverityError)
}

func printEffectiveConfig(cfg analyzer.Config, enabled map[analyzer.Rule]bool, selection map[analyzer.CheckID]bool, failLevel, configFile string) error {
	rules := make([]string, 0, len(enabled))
	for rule, isEnabled := range enabled {
		if isEnabled {
			rules = append(rules, string(rule))
		}
	}
	sort.Strings(rules)
	output := struct {
		SchemaVersion int                `json:"schemaVersion"`
		ConfigFile    string             `json:"configFile,omitempty"`
		Profile       analyzer.Profile   `json:"profile"`
		EnabledRules  []string           `json:"enabledRules"`
		EnabledChecks []analyzer.CheckID `json:"enabledChecks"`
		FailLevel     string             `json:"failLevel"`
		Config        analyzer.Config    `json:"config"`
	}{1, configFile, cfg.Profile, rules, analyzer.SelectedCheckIDs(selection), failLevel, cfg}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	err := enc.Encode(output)
	if isBrokenPipe(err) {
		return nil
	}
	return err
}

func parseCheckIDs(spec string) []analyzer.CheckID {
	var ids []analyzer.CheckID
	for _, value := range strings.Split(spec, ",") {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			ids = append(ids, analyzer.CheckID(trimmed))
		}
	}
	return ids
}

func isBrokenPipe(err error) bool {
	return errors.Is(err, syscall.EPIPE)
}

func hasSeverityAtLeast(issues []analyzer.Issue, minimum analyzer.Severity) bool {
	rank := map[analyzer.Severity]int{analyzer.SeverityNote: 1, analyzer.SeverityWarning: 2, analyzer.SeverityError: 3}
	for _, issue := range issues {
		if rank[issue.Severity] >= rank[minimum] {
			return true
		}
	}
	return false
}

func flagWasSet(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func filterIssues(in []analyzer.Issue, excludes []string) []analyzer.Issue {
	out := in[:0]
	for _, issue := range in {
		if !analyzer.Excluded(filepath.ToSlash(issue.Pos.Filename), excludes) {
			out = append(out, issue)
		}
	}
	return out
}

func writeBaseline(path string, issues []analyzer.Issue) error {
	return baselinepkg.Write(path, issues)
}

func readBaselineInfo(path string) (map[string]bool, int, error) {
	accepted, err := baselinepkg.Read(path)
	return accepted, baselinepkg.Version, err
}

func filterBaseline(in []analyzer.Issue, accepted map[string]bool) []analyzer.Issue {
	return baselinepkg.Filter(in, accepted)
}

func staleBaseline(accepted map[string]bool, current []analyzer.Issue) []string {
	return baselinepkg.Stale(accepted, current)
}

func writeSARIF(w io.Writer, issues []analyzer.Issue) error {
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
	type text struct {
		Text string `json:"text"`
	}
	const sarifUriBaseID = "ROOT"
	type artifactLocation struct {
		URI       string `json:"uri"`
		UriBaseID string `json:"uriBaseId,omitempty"`
	}
	type region struct {
		StartLine   int `json:"startLine"`
		StartColumn int `json:"startColumn,omitempty"`
		EndLine     int `json:"endLine,omitempty"`
		EndColumn   int `json:"endColumn,omitempty"`
	}
	type physicalLocation struct {
		ArtifactLocation artifactLocation `json:"artifactLocation"`
		Region           region           `json:"region"`
	}
	type sarifRule struct {
		ID                   string `json:"id"`
		Name                 string `json:"name"`
		HelpURI              string `json:"helpUri,omitempty"`
		ShortDescription     text   `json:"shortDescription"`
		FullDescription      text   `json:"fullDescription,omitempty"`
		DefaultConfiguration struct {
			Level string `json:"level"`
		} `json:"defaultConfiguration"`
	}
	type replacement struct {
		DeletedRegion   region `json:"deletedRegion"`
		InsertedContent text   `json:"insertedContent"`
	}
	type artifactChange struct {
		ArtifactLocation artifactLocation `json:"artifactLocation"`
		Replacements     []replacement    `json:"replacements"`
	}
	type sarifFix struct {
		Description     text             `json:"description"`
		ArtifactChanges []artifactChange `json:"artifactChanges"`
	}
	type relatedLocation struct {
		ID               int              `json:"id"`
		PhysicalLocation physicalLocation `json:"physicalLocation"`
		Message          text             `json:"message,omitempty"`
	}
	type sarifLocation struct {
		PhysicalLocation physicalLocation `json:"physicalLocation"`
	}
	type sarifResult struct {
		RuleID              string            `json:"ruleId"`
		Level               string            `json:"level"`
		Message             text              `json:"message"`
		Locations           []sarifLocation   `json:"locations"`
		RelatedLocations    []relatedLocation `json:"relatedLocations,omitempty"`
		PartialFingerprints map[string]string `json:"partialFingerprints,omitempty"`
		Properties          map[string]any    `json:"properties,omitempty"`
		Fixes               []sarifFix        `json:"fixes,omitempty"`
	}
	type driver struct {
		Name    string      `json:"name"`
		Version string      `json:"version"`
		Rules   []sarifRule `json:"rules"`
	}
	type run struct {
		OriginalUriBaseIds map[string]artifactLocation `json:"originalUriBaseIds,omitempty"`
		Tool               struct {
			Driver driver `json:"driver"`
		} `json:"tool"`
		Results []sarifResult `json:"results"`
	}
	analysisRoot := ""
	for _, issue := range ordered {
		if analysisRoot == "" {
			analysisRoot = issue.AnalysisRoot()
		}
	}
	sarifArtifact := func(issue analyzer.Issue, filename string) artifactLocation {
		loc := artifactLocation{URI: analyzer.PortableURIForIssue(issue, filename)}
		if analysisRoot != "" && analyzer.InsideAnalysisRoot(analysisRoot, filename) {
			loc.UriBaseID = sarifUriBaseID
		}
		return loc
	}
	var reportRun run
	if analysisRoot != "" {
		if baseURI := analyzer.FileURI(analysisRoot); baseURI != "" {
			reportRun.OriginalUriBaseIds = map[string]artifactLocation{
				sarifUriBaseID: {URI: baseURI},
			}
		}
	}
	reportRun.Tool.Driver.Name, reportRun.Tool.Driver.Version = "solidlint", activeBuild.Version
	reportRun.Tool.Driver.Rules = []sarifRule{}
	reportRun.Results = []sarifResult{}
	rules := map[string]sarifRule{}
	for _, original := range ordered {
		issue := original
		checkID := issue.Check
		if checkID == "" {
			checkID = analyzer.CheckID(issue.ID())
		}
		item := sarifResult{
			RuleID: issue.ID(), Level: string(issue.Severity),
			Message: text{Text: issue.Message},
			PartialFingerprints: map[string]string{
				"primaryLocationLineHash": issue.PrimaryLocationLineHash(),
				"solidlint/v4":            issue.Fingerprint(),
			},
			Properties: map[string]any{
				"subject": issue.Subject, "identity": issue.Identity,
				"evidence": issue.Evidence, "metrics": issue.Metrics, "groups": issue.Groups,
			},
		}
		if metadata, found := analyzer.CheckMetadata(checkID); found {
			item.Properties["maturity"] = metadata.Maturity
		}
		location := physicalLocation{
			ArtifactLocation: sarifArtifact(issue, issue.Pos.Filename),
			Region:           region{StartLine: issue.Pos.Line, StartColumn: issue.Pos.Column, EndLine: issue.End.Line, EndColumn: issue.End.Column},
		}
		item.Locations = append(item.Locations, sarifLocation{PhysicalLocation: location})
		relatedLocations := append([]analyzer.RelatedLocation(nil), issue.Related...)
		sort.SliceStable(relatedLocations, func(i, j int) bool {
			left, right := relatedLocations[i], relatedLocations[j]
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
		for index, related := range relatedLocations {
			item.RelatedLocations = append(item.RelatedLocations, relatedLocation{
				ID: index + 1,
				PhysicalLocation: physicalLocation{
					ArtifactLocation: sarifArtifact(issue, related.Pos.Filename),
					Region:           region{StartLine: related.Pos.Line, StartColumn: related.Pos.Column},
				},
				Message: text{Text: related.Message},
			})
		}
		for _, fix := range issue.SuggestedFixes {
			changes := make(map[string]*artifactChange)
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
					change = &artifactChange{ArtifactLocation: sarifArtifact(issue, edit.Filename)}
					changes[filename] = change
				}
				end := edit.End
				if end.Line <= 0 {
					end = edit.Start
				}
				change.Replacements = append(change.Replacements, replacement{
					DeletedRegion:   region{StartLine: edit.Start.Line, StartColumn: edit.Start.Column, EndLine: end.Line, EndColumn: end.Column},
					InsertedContent: text{Text: edit.NewText},
				})
			}
			if len(changes) == 0 {
				continue
			}
			filenames := make([]string, 0, len(changes))
			for filename := range changes {
				filenames = append(filenames, filename)
			}
			sort.Strings(filenames)
			sarifFixEntry := sarifFix{Description: text{Text: fix.Message}}
			for _, filename := range filenames {
				sarifFixEntry.ArtifactChanges = append(sarifFixEntry.ArtifactChanges, *changes[filename])
			}
			item.Fixes = append(item.Fixes, sarifFixEntry)
		}
		reportRun.Results = append(reportRun.Results, item)
		if _, ok := rules[issue.ID()]; !ok {
			description := analyzer.CheckDoc(checkID)
			if description == "" {
				description = issue.Message
			}
			ruleName := issue.ID()
			defaultSeverity := issue.Severity
			if metadata, found := analyzer.CheckMetadata(checkID); found {
				ruleName = metadata.Name
				defaultSeverity = metadata.DefaultSev
			}
			rule := sarifRule{
				ID: issue.ID(), Name: ruleName, HelpURI: analyzer.CheckHelpURI(checkID),
				ShortDescription: text{Text: description}, FullDescription: text{Text: description},
			}
			rule.DefaultConfiguration.Level = string(defaultSeverity)
			rules[issue.ID()] = rule
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
	report := struct {
		Version string `json:"version"`
		Schema  string `json:"$schema"`
		Runs    []run  `json:"runs"`
	}{"2.1.0", "https://json.schemastore.org/sarif-2.1.0.json", []run{reportRun}}
	err := json.NewEncoder(w).Encode(report)
	if isBrokenPipe(err) {
		return nil
	}
	return err
}

func parseRules(spec string) (map[analyzer.Rule]bool, error) {
	code2rule := map[string]analyzer.Rule{
		"S": analyzer.RuleSRP, "O": analyzer.RuleOCP, "L": analyzer.RuleLSP,
		"I": analyzer.RuleISP, "D": analyzer.RuleDIP,
	}
	enabled := map[analyzer.Rule]bool{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(strings.ToUpper(part))
		if part == "" {
			continue
		}
		rule, ok := code2rule[part]
		if !ok {
			return nil, fmt.Errorf("unknown rule %q (expected one of S,O,L,I,D)", part)
		}
		enabled[rule] = true
	}
	if len(enabled) == 0 {
		return nil, fmt.Errorf("no rules selected")
	}
	return enabled, nil
}

func allPathsAreGoFiles(paths []string) bool {
	if len(paths) == 0 {
		return false
	}
	for _, path := range paths {
		path = strings.TrimSuffix(path, "/...")
		if path == "" || !strings.HasSuffix(path, ".go") {
			return false
		}
	}
	return true
}
