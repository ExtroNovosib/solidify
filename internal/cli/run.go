// Command solidlint is a typed, heuristic static analyzer for SOLID design smells.
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ExtroNovosib/solidify/internal/analyzer"
	baselinepkg "github.com/ExtroNovosib/solidify/internal/baseline"
)

type BuildInfo struct {
	Version   string
	Commit    string
	BuildDate string
}

// Run dispatches explicit commands while preserving the v0.1 leading-flag and
// path-only invocation as an implicit check command.
func Run(args []string, build BuildInfo) int {
	if len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		return runTopLevelHelp()
	}
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return runCheckCommand(args, build)
	}
	switch args[0] {
	case "check":
		return runCheckCommand(args[1:], build)
	case "checks":
		return runChecksCommand(args[1:])
	case "config":
		return runConfigCommand(args[1:])
	case "baseline":
		return runBaselineCommand(args[1:], build)
	case "stats":
		return runStatsCommand(args[1:], build)
	default:
		return runCheckCommand(args, build)
	}
}

func runTopLevelHelp() int {
	_, _ = fmt.Fprintln(os.Stderr, "solidlint checks Go source for heuristic SOLID design smells.")
	_, _ = fmt.Fprintln(os.Stderr, "\nUsage: solidlint [check] [flags] <path> [<path> ...]")
	_, _ = fmt.Fprintln(os.Stderr, "       solidlint <command> [arguments]")
	_, _ = fmt.Fprintln(os.Stderr, "\nCommands:")
	_, _ = fmt.Fprintln(os.Stderr, "  check      analyze one or more packages; this is also the default command")
	_, _ = fmt.Fprintln(os.Stderr, "  checks     list checks or explain one check")
	_, _ = fmt.Fprintln(os.Stderr, "  config     initialize, validate, or print the configuration schema")
	_, _ = fmt.Fprintln(os.Stderr, "  baseline   initialize, update, diff, or prune accepted findings")
	_, _ = fmt.Fprintln(os.Stderr, "  stats      report selected checks, cache activity, and analysis coverage")
	_, _ = fmt.Fprintln(os.Stderr, "  help       show this command overview")
	_, _ = fmt.Fprintln(os.Stderr, "\nExamples:")
	_, _ = fmt.Fprintln(os.Stderr, "  solidlint check ./...")
	_, _ = fmt.Fprintln(os.Stderr, "  solidlint checks explain SOLID-I/fat-interface -format=json")
	_, _ = fmt.Fprintln(os.Stderr, "  solidlint stats -format=json ./...")
	_, _ = fmt.Fprintln(os.Stderr, "\nRun `solidlint check --help` for analyzer flags.")
	return 0
}

func validFailLevel(level string) bool {
	return level == string(analyzer.SeverityNote) ||
		level == string(analyzer.SeverityWarning) ||
		level == string(analyzer.SeverityError)
}

func isBrokenPipe(err error) bool {
	return errors.Is(err, syscall.EPIPE)
}

func hasSeverityAtLeast(issues []analyzer.Issue, minimum analyzer.Severity) bool {
	rank := map[analyzer.Severity]int{
		analyzer.SeverityNote: 1, analyzer.SeverityWarning: 2, analyzer.SeverityError: 3,
	}
	for _, issue := range issues {
		if rank[issue.Severity] >= rank[minimum] {
			return true
		}
	}
	return false
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

func writeBaseline(path string, issues []analyzer.Issue, reason ...string) error {
	return baselinepkg.Write(path, issues, reason...)
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

func parseRules(spec string) (map[analyzer.Rule]bool, error) {
	codeToRule := map[string]analyzer.Rule{
		"S": analyzer.RuleSRP, "O": analyzer.RuleOCP, "L": analyzer.RuleLSP,
		"I": analyzer.RuleISP, "D": analyzer.RuleDIP,
	}
	enabled := map[analyzer.Rule]bool{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(strings.ToUpper(part))
		if part == "" {
			continue
		}
		rule, ok := codeToRule[part]
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
