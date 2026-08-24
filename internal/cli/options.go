package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/ExtroNovosib/solidify/internal/analyzer"
)

type checkOptions struct {
	rules, profile, enabledChecks                  string
	format, analysis                               string
	showVersion, includeTests, fail, cache         bool
	failLevel                                      string
	maxMethods, maxFuncLines, maxParams            int
	maxSwitchCases, maxInterfaceMethods            int
	ispMinMethods, ispUsageRatioPercent            int
	configPath, baselinePath, writeBaselinePath    string
	baselineReason, baselineOwner, baselineExpires string
	baselineStale, cacheDir                        string
	cacheDebug, printConfig, baselinePrune         bool
	paths                                          []string
	set                                            map[string]bool
}

func parseCheckOptions(args []string) (checkOptions, error) {
	defaults := analyzer.DefaultConfig()
	fs := flag.NewFlagSet("solidlint", flag.ContinueOnError)
	var options checkOptions
	fs.StringVar(&options.rules, "rules", "S,O,L,I,D", "comma-separated list of rules to run (S,O,L,I,D)")
	fs.StringVar(&options.profile, "profile", string(analyzer.ProfileStable), "check profile: stable|all|calibration")
	fs.StringVar(&options.enabledChecks, "enable-checks", "", "comma-separated concrete check IDs to enable")
	fs.StringVar(&options.format, "format", "text", "output format: text|json|sarif")
	fs.StringVar(&options.analysis, "analysis", "auto", "analysis mode: auto|syntax|types")
	fs.BoolVar(&options.showVersion, "version", false, "print version and exit")
	fs.BoolVar(&options.includeTests, "tests", false, "also lint _test.go files")
	fs.BoolVar(&options.fail, "fail", true, "exit with a non-zero status if any issue is found")
	fs.StringVar(&options.failLevel, "fail-level", string(analyzer.SeverityWarning), "minimum severity that fails: note|warning|error")
	fs.IntVar(&options.maxMethods, "max-methods", defaults.MaxMethodsPerType, "SRP: max methods per type")
	fs.IntVar(&options.maxFuncLines, "max-func-lines", defaults.MaxFuncLines, "SRP: max lines per function body")
	fs.IntVar(&options.maxParams, "max-params", defaults.MaxFuncParams, "SRP: max cohesive parameters before mixed/repeated lists are reported")
	fs.IntVar(&options.maxSwitchCases, "max-switch-cases", defaults.MaxTypeSwitchCases, "OCP: max cases in a type switch / type-assertion chain")
	fs.IntVar(&options.maxInterfaceMethods, "max-interface-methods", defaults.MaxInterfaceMethods, "ISP: max methods per interface")
	fs.IntVar(&options.ispMinMethods, "isp-min-methods", defaults.ISPMinMethods, "ISP: minimum interface methods for usage-ratio and stub checks")
	fs.IntVar(&options.ispUsageRatioPercent, "isp-usage-ratio-percent", defaults.ISPUsageRatioPercent, "ISP: minimum used-method percentage")
	fs.StringVar(&options.configPath, "config", "", "path to .solidify.yml (default: discover)")
	fs.StringVar(&options.baselinePath, "baseline", "", "JSON baseline of accepted finding fingerprints")
	fs.StringVar(&options.writeBaselinePath, "write-baseline", "", "write current findings as a baseline JSON file")
	fs.StringVar(&options.baselineReason, "baseline-reason", "", "review reason required for newly accepted findings")
	fs.StringVar(&options.baselineOwner, "baseline-owner", "", "optional owner for newly accepted findings")
	fs.StringVar(&options.baselineExpires, "baseline-expires", "", "optional expiry date (YYYY-MM-DD)")
	fs.StringVar(&options.baselineStale, "baseline-stale", "warn", "stale baseline policy: ignore|warn|error")
	fs.BoolVar(&options.baselinePrune, "prune", false, "remove stale entries during baseline update")
	fs.StringVar(&options.cacheDir, "cache-dir", "", "cache directory (default: platform user cache)")
	fs.BoolVar(&options.cache, "cache", true, "enable the analysis cache")
	fs.BoolVar(&options.cacheDebug, "cache-debug", false, "print cache diagnostics to stderr")
	fs.BoolVar(&options.printConfig, "print-config", false, "print effective configuration and exit")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "solidlint checks Go source for heuristic SOLID principle violations.")
		fmt.Fprintln(fs.Output(), "\nUsage: solidlint [check] [flags] <path> [<path> ...]")
		fmt.Fprintln(fs.Output(), "A .go target analyzes its containing package; directory targets are recursive.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return checkOptions{}, err
	}
	options.set = map[string]bool{}
	fs.Visit(func(item *flag.Flag) { options.set[item.Name] = true })
	options.paths = fs.Args()
	if len(options.paths) == 0 {
		options.paths = []string{"."}
	}
	if err := options.validate(); err != nil {
		fmt.Fprintln(os.Stderr, "solidlint:", err)
		return checkOptions{}, err
	}
	return options, nil
}

func (options checkOptions) validate() error {
	if options.format != "text" && options.format != "json" && options.format != "sarif" {
		return fmt.Errorf("unknown format %q (expected text, json, or sarif)", options.format)
	}
	if options.analysis != "auto" && options.analysis != "syntax" && options.analysis != "types" {
		return fmt.Errorf("unknown analysis mode %q (expected auto, syntax, or types)", options.analysis)
	}
	if options.profile != string(analyzer.ProfileStable) && options.profile != string(analyzer.ProfileAll) && options.profile != string(analyzer.ProfileCalibration) {
		return fmt.Errorf("unknown profile %q (expected stable, all, or calibration)", options.profile)
	}
	if !validFailLevel(options.failLevel) {
		return fmt.Errorf("unknown fail level %q (expected note, warning, or error)", options.failLevel)
	}
	if options.baselineStale != "ignore" && options.baselineStale != "warn" && options.baselineStale != "error" {
		return fmt.Errorf("unknown baseline stale policy %q (expected ignore, warn, or error)", options.baselineStale)
	}
	return nil
}
