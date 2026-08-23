// Package analyzer implements deterministic heuristic checks for the SOLID
// design principles. Syntax checks always work on the Go AST; the SRP checker
// additionally uses standard-library go/types facts when a package resolves.
package analyzer

import (
	"crypto/sha256"
	"fmt"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

// Severity of a reported issue.
type Severity string

const (
	SeverityNote    Severity = "note"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// CheckID identifies the concrete check that produced a finding.
type CheckID string

// Profile names a public rule-maturity selection.
type Profile string

const (
	ProfileStable Profile = "stable"
	ProfileAll    Profile = "all"
)

const (
	CheckSRPGodType             CheckID = "SOLID-S/god-type"
	CheckSRPLowCohesionType     CheckID = "SOLID-S/low-cohesion-type"
	CheckSRPLargeType           CheckID = "SOLID-S/large-type"
	CheckSRPHighFanOutType      CheckID = "SOLID-S/high-fan-out-type"
	CheckSRPComplexFunction     CheckID = "SOLID-S/complex-function"
	CheckSRPMixedInputSurface   CheckID = "SOLID-S/mixed-input-surface"
	CheckSRPDataClump           CheckID = "SOLID-S/data-clump"
	CheckSRPFlagArgument        CheckID = "SOLID-S/flag-argument"
	CheckSRPMixedImportClusters CheckID = "SOLID-S/mixed-import-clusters"

	CheckISPFatInterface       CheckID = "SOLID-I/fat-interface"
	CheckISPUsageRatio         CheckID = "SOLID-I/usage-ratio"
	CheckISPStubImplementation CheckID = "SOLID-I/stub-implementation"

	CheckOCPTypeDispatch            CheckID = "SOLID-O/type-dispatch"
	CheckOCPDiscriminatorDispatch   CheckID = "SOLID-O/discriminator-dispatch"
	CheckOCPRuntimeExhaustiveness   CheckID = "SOLID-O/runtime-exhaustiveness"
	CheckOCPConcreteParameter       CheckID = "SOLID-O/concrete-parameter"
	CheckOCPClosedFactory           CheckID = "SOLID-O/closed-factory"
	CheckOCPImplementationCoupling  CheckID = "SOLID-O/implementation-coupling"
	CheckOCPParallelImplementations CheckID = "SOLID-O/parallel-implementations"

	CheckLSPNonExactEOF          CheckID = "SOLID-L/non-exact-eof"
	CheckLSPNilEmbeddedInterface CheckID = "SOLID-L/nil-embedded-interface"

	CheckDIPConcreteDependency CheckID = "SOLID-D/concrete-dependency"
	CheckDIPLayerImport        CheckID = "SOLID-D/layer-import"
	CheckDIPWiringOutsideRoot  CheckID = "SOLID-D/wiring-outside-root"
	CheckDIPHiddenConstruction CheckID = "SOLID-D/hidden-construction"
	CheckDIPInfraErrorLeak     CheckID = "SOLID-D/infra-error-leak"
	CheckDIPTransportLeak      CheckID = "SOLID-D/transport-leak"
)

type Metric struct {
	Name       string  `json:"name"`
	Value      float64 `json:"value"`
	Threshold  float64 `json:"threshold,omitempty"`
	Comparator string  `json:"comparator,omitempty"`
}

type SymbolGroup struct {
	Label   string   `json:"label"`
	Symbols []string `json:"symbols"`
}

type RelatedLocation struct {
	Pos     token.Position `json:"position"`
	Message string         `json:"message,omitempty"`
}

// Rule identifies which SOLID letter (and sub-check) produced an issue.
type Rule string

const (
	RuleSRP Rule = "SOLID-S" // Single Responsibility Principle
	RuleOCP Rule = "SOLID-O" // Open/Closed Principle
	RuleLSP Rule = "SOLID-L" // Liskov Substitution Principle
	RuleISP Rule = "SOLID-I" // Interface Segregation Principle
	RuleDIP Rule = "SOLID-D" // Dependency Inversion Principle
)

// TextEdit is a mechanical source edit suggested for a finding.
type TextEdit struct {
	Filename string
	Start    token.Position
	End      token.Position
	NewText  string
}

// SuggestedFix groups optional text edits for a finding.
type SuggestedFix struct {
	Message string
	Edits   []TextEdit
}

// Issue is a single finding reported by one of the rules.
type Issue struct {
	Rule     Rule
	Check    CheckID
	Severity Severity
	Pos      token.Position
	End      token.Position // zero when unknown
	Message  string
	// Evidence is a concise machine-readable explanation of the matched
	// construct. It is intentionally stable so JSON/SARIF consumers can use
	// it without parsing the human-facing message.
	Evidence       string
	Subject        string
	Identity       string
	Metrics        []Metric
	Groups         []SymbolGroup
	Related        []RelatedLocation
	SuggestedFixes []SuggestedFix

	analysisRoot string
}

// AnalysisRoot returns the canonical analysis root used for portable paths.
func (i Issue) AnalysisRoot() string {
	return i.analysisRoot
}

func (i Issue) String() string {
	return fmt.Sprintf("%s:%d:%d: [%s] %s", i.Pos.Filename, i.Pos.Line, i.Pos.Column, i.ID(), i.Message)
}

// PortablePath returns the normalized machine-facing path for this finding.
func (i Issue) PortablePath() string {
	return PortablePath(i.analysisRoot, i.Pos.Filename)
}

// PortablePathForIssue applies a finding's canonical root to a related file.
func PortablePathForIssue(issue Issue, filename string) string {
	return PortablePath(issue.analysisRoot, filename)
}

// PortableURIForIssue applies a finding's canonical root to a related file.
func PortableURIForIssue(issue Issue, filename string) string {
	return PortableURI(issue.analysisRoot, filename)
}

// PortableURI returns the SARIF artifact URI for this finding.
func (i Issue) PortableURI() string {
	return PortableURI(i.analysisRoot, i.Pos.Filename)
}

// PrimaryLocationLineHash returns a stable line hash for SARIF consumers.
func (i Issue) PrimaryLocationLineHash() string {
	return fileLineHash(i.Pos.Filename, i.Pos.Line)
}

// ID identifies the specific design smell behind a rule finding.
func (i Issue) ID() string {
	if i.Check != "" {
		return string(i.Check)
	}
	return string(i.Rule)
}

func SortedSymbols(symbols []string) []string {
	out := append([]string(nil), symbols...)
	sort.Strings(out)
	return out
}

// Fingerprint is stable across line-only changes and is suitable for baselines.
func (i Issue) Fingerprint() string {
	sum := sha256.Sum256([]byte("solidlint/v4\x00" + i.ID() + "\x00" + fingerprintPath(i.analysisRoot, i.Pos.Filename) + "\x00" + i.Subject + "\x00" + i.Identity))
	return fmt.Sprintf("%x", sum[:])
}

// FinalizeIssues supplies the required stable identity contract and rejects
// collisions before findings can reach a baseline or renderer.
func FinalizeIssues(issues []Issue, packagePath string) error {
	seen := map[string]int{}
	for index := range issues {
		if issues[index].Subject == "" || issues[index].Identity == "" {
			issues[index].Subject, issues[index].Identity = deriveIssueIdentity(issues[index], packagePath)
		}
		if issues[index].Subject == "" || issues[index].Identity == "" {
			return fmt.Errorf("%s produced an empty subject or identity", issues[index].ID())
		}
		key := issues[index].ID() + "\x00" + issues[index].PortablePath() + "\x00" + issues[index].Subject + "\x00" + issues[index].Identity
		if previous, ok := seen[key]; ok {
			return fmt.Errorf("identity collision for %s findings at indexes %d and %d", issues[index].ID(), previous, index)
		}
		seen[key] = index
	}
	return nil
}

func deriveIssueIdentity(issue Issue, packagePath string) (string, string) {
	prefix, fields := parseEvidenceIdentity(issue.Evidence)
	subjectKey := firstIdentityField(fields, "type", "function", "method", "interface", "package", "field", "factory", "source", "from")
	if subjectKey == "" {
		subjectKey = prefix
	}
	if packagePath == "" {
		packagePath = "package"
	}
	subject := packagePath + "." + subjectKey
	if strings.HasPrefix(subjectKey, packagePath+".") {
		subject = subjectKey
	}
	parts := []string{prefix}
	for _, key := range []string{"type", "function", "method", "interface", "package", "field", "factory", "source", "from", "to", "peer", "parameter", "parameters", "dependency", "import", "kind", "variants", "shared", "methods"} {
		value := fields[key]
		if value == "" || numericIdentityValue(value) {
			continue
		}
		parts = append(parts, key+"="+value)
	}
	identity := strings.Join(parts, ";")
	if identity == "" {
		identity = prefix
	}
	return subject, identity
}

func parseEvidenceIdentity(evidence string) (string, map[string]string) {
	fields := map[string]string{}
	parts := strings.Split(evidence, ";")
	prefix := strings.TrimSpace(parts[0])
	if colon := strings.Index(prefix, ":"); colon >= 0 {
		parts = append([]string{prefix[colon+1:]}, parts[1:]...)
		prefix = prefix[:colon]
	}
	for _, part := range parts {
		key, value, ok := strings.Cut(part, "=")
		if ok {
			fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return prefix, fields
}

func firstIdentityField(fields map[string]string, keys ...string) string {
	for _, key := range keys {
		if fields[key] != "" {
			return fields[key]
		}
	}
	return ""
}

func numericIdentityValue(value string) bool {
	_, err := strconv.ParseFloat(strings.TrimSuffix(value, "%"), 64)
	return err == nil
}

// Config holds the tunable thresholds for every rule. Sensible defaults are
// provided by DefaultConfig(); every value can be overridden from the CLI.
type Config struct {
	// Execution context used by cache and input-policy infrastructure.
	CacheDir         string
	CacheEnabled     bool
	CacheDiagnostics bool
	AnalysisMode     string
	IncludeTests     bool
	ToolVersion      string
	Profile          Profile
	EnabledChecks    []CheckID

	// SRP
	MaxMethodsPerType       int // flag types (struct) that own more methods than this
	MaxFuncLines            int // flag functions/methods longer than this many lines
	MaxFuncParams           int // examine longer parameter lists for mixed types or repeated data clumps
	MaxFieldsPerType        int
	MaxTypeLines            int
	MaxExportedMethods      int
	MaxFuncComplexity       int
	MaxTypeComplexity       int
	MaxFanOut               int
	MaxATFD                 int
	MinLargeTypeSignals     int
	MinTCCPercent           int
	MinCohesionMethods      int
	MinCohesionFields       int
	MinComponentMethods     int
	MinImportClusterMethods int
	DisabledChecks          []CheckID

	// OCP
	MaxTypeSwitchCases             int // flag type switches / long if-else type-assertion chains
	OCPMinDispatchSites            int
	OCPMinSharedVariants           int
	OCPDispatchOverlapPercent      int
	OCPMinConcreteParameterMethods int
	OCPMinImplementationImports    int
	OCPMinParallelFunctions        int
	OCPMinParallelNodes            int
	OCPParallelSimilarityPercent   int
	OCPDiscriminatorFields         []string
	OCPAllowDispatchTypes          []string
	OCPAllowPackages               []string
	OCPLogicPackages               []string
	OCPImplementationPackages      []string
	OCPCompositionRoots            []string
	ExcludedFiles                  []string

	// LSP
	// no threshold needed: LSP checks validate narrow, explicit contracts
	// (currently exact io.EOF handling and embedded-interface initialization).

	// ISP
	MaxInterfaceMethods  int // flag interfaces with more methods than this
	ISPMinMethods        int // minimum interface method count for usage-ratio and stub checks
	ISPUsageRatioPercent int // flag when a client uses fewer than this percent of interface methods

	// DIPAllowDependencies lists concrete type names intentionally permitted
	// at composition boundaries (for example a database driver).
	DIPAllowDependencies []string

	// DIPCompositionRootFields suppresses field-level concrete-dependency
	// findings when a struct already wires this many concrete collaborators
	// (typical composition roots).
	DIPCompositionRootFields int

	// DIPInfraErrorPackages lists import paths whose sentinel errors must not
	// appear in logic packages (for example database/sql).
	DIPInfraErrorPackages []string

	// DIPTransportTypes lists fully-qualified transport types that must not
	// appear in logic-package signatures (for example net/http.Request).
	DIPTransportTypes []string

	// DIP
	// no threshold needed: DIP checks whether a struct field / constructor
	// parameter is a concrete pointer-to-struct type from the same module
	// instead of an interface.
}

// DefaultConfig returns the recommended default thresholds.
func DefaultConfig() Config {
	return Config{
		CacheEnabled: true,
		AnalysisMode: analysisModeAuto,
		ToolVersion:  "dev",
		// Internal callers historically execute the complete analyzer. Public
		// CLI and plugin entry points explicitly select ProfileStable.
		Profile:                        ProfileAll,
		MaxMethodsPerType:              10,
		MaxFuncLines:                   60,
		MaxFuncParams:                  8,
		MaxFieldsPerType:               10,
		MaxTypeLines:                   250,
		MaxExportedMethods:             10,
		MaxFuncComplexity:              15,
		MaxTypeComplexity:              47,
		MaxFanOut:                      15,
		MaxATFD:                        5,
		MinLargeTypeSignals:            4,
		MinTCCPercent:                  33,
		MinCohesionMethods:             4,
		MinCohesionFields:              2,
		MinComponentMethods:            2,
		MinImportClusterMethods:        3,
		MaxTypeSwitchCases:             4,
		OCPMinDispatchSites:            2,
		OCPMinSharedVariants:           2,
		OCPDispatchOverlapPercent:      60,
		OCPMinConcreteParameterMethods: 2,
		OCPMinImplementationImports:    2,
		OCPMinParallelFunctions:        3,
		OCPMinParallelNodes:            20,
		OCPParallelSimilarityPercent:   90,
		OCPDiscriminatorFields:         []string{"Kind", "Type", "Status", "Mode", "Variant"},
		MaxInterfaceMethods:            8,
		ISPMinMethods:                  3,
		ISPUsageRatioPercent:           50,
		DIPCompositionRootFields:       5,
	}
}
