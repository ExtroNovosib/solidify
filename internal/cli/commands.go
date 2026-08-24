package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ExtroNovosib/solidify/internal/analyzer"
	baselinepkg "github.com/ExtroNovosib/solidify/internal/baseline"
	configpkg "github.com/ExtroNovosib/solidify/internal/config"
)

func runCheckCommand(args []string, build BuildInfo) int {
	options, err := parseCheckOptions(args)
	if err != nil {
		return 2
	}
	if options.showVersion {
		fmt.Fprintln(os.Stdout, build.Version)
		return 0
	}
	policy, err := resolveCheckPolicy(options, build)
	if err != nil {
		fmt.Fprintln(os.Stderr, "solidlint:", err)
		return 2
	}
	if options.printConfig {
		if err := renderEffectiveConfig(policy); err != nil && !isBrokenPipe(err) {
			fmt.Fprintln(os.Stderr, "solidlint:", err)
			return 2
		}
		return 0
	}
	result, err := executeAnalysis(policy)
	if err != nil {
		fmt.Fprintln(os.Stderr, "solidlint:", err)
		return 2
	}
	for _, warning := range result.warnings {
		fmt.Fprintln(os.Stderr, "solidlint: warning:", warning)
	}
	if code := applyLegacyBaselineOptions(policy, &result); code >= 0 {
		return code
	}
	if err := renderIssues(result.issues, options.format, policy.config.Profile, build); err != nil && !isBrokenPipe(err) {
		fmt.Fprintln(os.Stderr, "solidlint:", err)
		return 2
	}
	if options.fail && hasSeverityAtLeast(result.issues, analyzer.Severity(options.failLevel)) {
		return 1
	}
	return 0
}

func applyLegacyBaselineOptions(policy checkPolicy, result *analysisResult) int {
	options := policy.options
	if options.writeBaselinePath != "" {
		annotation := baselinepkg.Annotation{Reason: options.baselineReason, Owner: options.baselineOwner, Expires: options.baselineExpires}
		document, _, err := baselinepkg.Update(baselinepkg.Document{Version: baselinepkg.Version}, result.issues, annotation, true)
		if err == nil {
			err = baselinepkg.WriteDocument(options.writeBaselinePath, document)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "solidlint:", err)
			return 2
		}
	}
	if options.baselinePath == "" {
		return -1
	}
	accepted, _, err := readBaselineInfo(options.baselinePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "solidlint:", err)
		return 2
	}
	if stale := staleBaseline(accepted, result.issues); len(stale) > 0 {
		if options.baselineStale != "ignore" {
			fmt.Fprintf(os.Stderr, "solidlint: baseline contains %d stale fingerprint(s)\n", len(stale))
		}
		if options.baselineStale == "error" {
			return 1
		}
	}
	result.issues = filterBaseline(result.issues, accepted)
	return -1
}

func runStatsCommand(args []string, build BuildInfo) int {
	options, err := parseCheckOptions(args)
	if err != nil {
		return 2
	}
	if options.format == "sarif" {
		fmt.Fprintln(os.Stderr, "solidlint: stats format must be text or json")
		return 2
	}
	policy, err := resolveCheckPolicy(options, build)
	if err != nil {
		fmt.Fprintln(os.Stderr, "solidlint:", err)
		return 2
	}
	result, err := executeAnalysis(policy)
	if err != nil {
		fmt.Fprintln(os.Stderr, "solidlint:", err)
		return 2
	}
	if err := renderStats(result.stats, options.format); err != nil && !isBrokenPipe(err) {
		fmt.Fprintln(os.Stderr, "solidlint:", err)
		return 2
	}
	return 0
}

type checkDescription struct {
	ID          analyzer.CheckID       `json:"id"`
	Name        string                 `json:"name"`
	Rule        analyzer.Rule          `json:"rule"`
	Scope       string                 `json:"scope"`
	Maturity    analyzer.Maturity      `json:"maturity"`
	Syntax      analyzer.SyntaxSupport `json:"syntaxSupport"`
	RunnerGroup string                 `json:"runnerGroup"`
	Description string                 `json:"description"`
	HelpURI     string                 `json:"helpUri"`
}

func runChecksCommand(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "solidlint: usage: solidlint checks <list|explain>")
		return 2
	}
	switch args[0] {
	case "list":
		format, ok := parseMetadataFormat(args[1:])
		if !ok {
			return 2
		}
		items := allCheckDescriptions()
		if format == "json" {
			return encodeCommandJSON(items)
		}
		for _, item := range items {
			fmt.Printf("%s\t%s\t%s\t%s\n", item.ID, item.Maturity, item.Scope, item.Name)
		}
		return 0
	case "explain":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "solidlint: usage: solidlint checks explain <check-id> [-format=json]")
			return 2
		}
		format, ok := parseMetadataFormat(args[2:])
		if !ok {
			return 2
		}
		metadata, found := analyzer.CheckMetadata(analyzer.CheckID(args[1]))
		if !found {
			fmt.Fprintf(os.Stderr, "solidlint: unknown check %q\n", args[1])
			return 2
		}
		item := describeCheck(metadata)
		if format == "json" {
			return encodeCommandJSON(item)
		}
		fmt.Printf("%s — %s\n%s\nprofile: %s; scope: %s; syntax: %s\n%s\n", item.ID, item.Name, item.Description, item.Maturity, item.Scope, item.Syntax, item.HelpURI)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "solidlint: unknown checks command %q\n", args[0])
		return 2
	}
}

func allCheckDescriptions() []checkDescription {
	items := make([]checkDescription, 0, len(analyzer.RegisteredCheckIDs()))
	for _, id := range analyzer.RegisteredCheckIDs() {
		metadata, _ := analyzer.CheckMetadata(id)
		items = append(items, describeCheck(metadata))
	}
	return items
}

func describeCheck(metadata analyzer.Check) checkDescription {
	scope := "package"
	if metadata.Scope == analyzer.ScopeProgram {
		scope = "program"
	}
	return checkDescription{metadata.ID, metadata.Name, metadata.Rule, scope, metadata.Maturity, metadata.Syntax, metadata.RunnerGroup, metadata.Doc, metadata.HelpURI}
}

func parseMetadataFormat(args []string) (string, bool) {
	fs := flag.NewFlagSet("metadata", flag.ContinueOnError)
	format := fs.String("format", "text", "output format: text|json")
	if err := fs.Parse(args); err != nil {
		return "", false
	}
	if fs.NArg() != 0 || *format != "text" && *format != "json" {
		fmt.Fprintln(os.Stderr, "solidlint: format must be text or json")
		return "", false
	}
	return *format, true
}

func runConfigCommand(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "solidlint: usage: solidlint config <init|validate|schema>")
		return 2
	}
	switch args[0] {
	case "init":
		if len(args) != 1 {
			return configUsageError("init takes no arguments")
		}
		_, err := os.Stdout.Write(configpkg.InitYAML())
		if err != nil && !isBrokenPipe(err) {
			return configUsageError(err.Error())
		}
		return 0
	case "validate":
		path := ".solidify.yml"
		if len(args) > 2 {
			return configUsageError("validate accepts at most one path")
		}
		if len(args) == 2 {
			path = args[1]
		}
		if err := configpkg.Validate(path); err != nil {
			fmt.Fprintln(os.Stderr, "solidlint:", err)
			return 2
		}
		fmt.Fprintln(os.Stdout, path+": valid")
		return 0
	case "schema":
		if len(args) > 2 || len(args) == 2 && args[1] != "-format=json" {
			return configUsageError("schema supports only -format=json")
		}
		data, err := configpkg.SchemaJSON()
		if err != nil {
			return configUsageError(err.Error())
		}
		if _, err := os.Stdout.Write(data); err != nil && !isBrokenPipe(err) {
			return configUsageError(err.Error())
		}
		return 0
	default:
		return configUsageError(fmt.Sprintf("unknown config command %q", args[0]))
	}
}

func runBaselineCommand(args []string, build BuildInfo) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "solidlint: usage: solidlint baseline <init|diff|update|prune> -baseline <file> [check flags] [targets]")
		return 2
	}
	operation := args[0]
	if operation != "init" && operation != "diff" && operation != "update" && operation != "prune" {
		fmt.Fprintf(os.Stderr, "solidlint: unknown baseline command %q\n", operation)
		return 2
	}
	options, err := parseCheckOptions(args[1:])
	if err != nil {
		return 2
	}
	if options.baselinePath == "" {
		fmt.Fprintln(os.Stderr, "solidlint: baseline command requires -baseline <file>")
		return 2
	}
	policy, err := resolveCheckPolicy(options, build)
	if err != nil {
		fmt.Fprintln(os.Stderr, "solidlint:", err)
		return 2
	}
	result, err := executeAnalysis(policy)
	if err != nil {
		fmt.Fprintln(os.Stderr, "solidlint:", err)
		return 2
	}
	annotation := baselinepkg.Annotation{Reason: options.baselineReason, Owner: options.baselineOwner, Expires: options.baselineExpires}
	if operation == "init" {
		document, diff, err := baselinepkg.Update(baselinepkg.Document{Version: baselinepkg.Version}, result.issues, annotation, true)
		if err != nil {
			return baselineCommandError(err)
		}
		if err := baselinepkg.WriteDocument(options.baselinePath, document); err != nil {
			return baselineCommandError(err)
		}
		return renderBaselineDiff(diff, options.format, false)
	}
	document, err := baselinepkg.Load(options.baselinePath)
	if err != nil {
		return baselineCommandError(err)
	}
	diff := baselinepkg.Diff(document, result.issues)
	if operation == "diff" {
		return renderBaselineDiff(diff, options.format, len(diff.Added) > 0 || len(diff.Stale) > 0)
	}
	var updated baselinepkg.Document
	if operation == "prune" {
		updated, diff, err = baselinepkg.Prune(document, result.issues)
	} else {
		updated, diff, err = baselinepkg.Update(document, result.issues, annotation, options.baselinePrune)
	}
	if err != nil {
		return baselineCommandError(err)
	}
	if err := baselinepkg.WriteDocument(options.baselinePath, updated); err != nil {
		return baselineCommandError(err)
	}
	return renderBaselineDiff(diff, options.format, false)
}

func renderBaselineDiff(diff baselinepkg.DiffResult, format string, fail bool) int {
	added := make([]string, len(diff.Added))
	for index, issue := range diff.Added {
		added[index] = issue.Fingerprint()
	}
	stale := make([]string, len(diff.Stale))
	for index, entry := range diff.Stale {
		stale[index] = entry.Fingerprint
	}
	if format == "json" {
		if code := encodeCommandJSON(struct {
			Added []string `json:"added"`
			Stale []string `json:"stale"`
			Live  int      `json:"live"`
		}{added, stale, len(diff.Live)}); code != 0 {
			return code
		}
	} else {
		fmt.Printf("baseline: added=%d stale=%d live=%d\n", len(added), len(stale), len(diff.Live))
	}
	if fail {
		return 1
	}
	return 0
}

func baselineCommandError(err error) int {
	fmt.Fprintln(os.Stderr, "solidlint:", err)
	return 2
}

func configUsageError(message string) int {
	fmt.Fprintln(os.Stderr, "solidlint:", message)
	return 2
}

func encodeCommandJSON(value any) int {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil && !isBrokenPipe(err) {
		fmt.Fprintln(os.Stderr, "solidlint:", err)
		return 2
	}
	return 0
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
