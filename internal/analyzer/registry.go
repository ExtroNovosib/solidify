package analyzer

import "fmt"

const checkDocsBaseURI = "https://github.com/ExtroNovosib/solidify/blob/main/docs/checks"

// Scope distinguishes package-local checks from whole-program correlation.
type Scope int

const (
	ScopePackage Scope = iota
	ScopeProgram
)

// Maturity controls whether a check participates in the conservative default
// profile or requires an explicit experimental opt-in.
type Maturity string

const (
	MaturityStable       Maturity = "stable"
	MaturityExperimental Maturity = "experimental"
)

// SyntaxSupport defines what a check may do without complete go/types facts.
type SyntaxSupport string

const (
	SyntaxEquivalent   SyntaxSupport = "equivalent"
	SyntaxConservative SyntaxSupport = "conservative"
	SyntaxUnavailable  SyntaxSupport = "unavailable"
)

// Surface identifies a supported solidlint integration.
type Surface uint8

const (
	SurfaceCLI Surface = 1 << iota
	SurfaceModulePlugin
	SurfaceGoPlugin
)

func (s Surface) Supports(surface Surface) bool { return s&surface != 0 }

// Check describes one registered analyzer runner and its metadata.
type Check struct {
	ID          CheckID
	Name        string
	Rule        Rule
	Doc         string
	HelpURI     string
	Scope       Scope
	Maturity    Maturity
	Syntax      SyntaxSupport
	Surfaces    Surface
	HasSafeFix  bool
	DefaultSev  Severity
	RunnerGroup string
	RunPackage  func(pkg *packageFiles, cfg Config) []Issue
	RunProgram  func(pkgs []*packageFiles, cfg Config) []Issue
}

// checkRegistry is the authoritative definition for every concrete check.
// Checks sharing one analyzer pass name the same RunnerGroup; only the group
// leader stores the runner so Run executes each pass exactly once.
var checkRegistry = completeCheckMetadata([]Check{
	{
		ID: CheckSRPGodType, Name: "God type", Rule: RuleSRP, Scope: ScopePackage,
		Doc:     "Flags types that accumulate too many responsibilities.",
		HelpURI: checkHelpURI(CheckSRPGodType), DefaultSev: SeverityWarning,
		RunnerGroup: "srp-package",
		RunPackage:  runSRPCheck,
	},
	{ID: CheckSRPLowCohesionType, Name: "Low cohesion type", Rule: RuleSRP, Scope: ScopePackage, Doc: "Methods on a type access disjoint field sets.", HelpURI: checkHelpURI(CheckSRPLowCohesionType), DefaultSev: SeverityWarning, RunnerGroup: "srp-package"},
	{ID: CheckSRPLargeType, Name: "Large type", Rule: RuleSRP, Scope: ScopePackage, Doc: "A type exceeds size or complexity thresholds.", HelpURI: checkHelpURI(CheckSRPLargeType), DefaultSev: SeverityWarning, RunnerGroup: "srp-package"},
	{ID: CheckSRPHighFanOutType, Name: "High fan-out type", Rule: RuleSRP, Scope: ScopePackage, Doc: "A type depends on too many external symbols.", HelpURI: checkHelpURI(CheckSRPHighFanOutType), DefaultSev: SeverityNote, RunnerGroup: "srp-package"},
	{ID: CheckSRPComplexFunction, Name: "Complex function", Rule: RuleSRP, Scope: ScopePackage, Doc: "A function body is longer or more complex than configured limits.", HelpURI: checkHelpURI(CheckSRPComplexFunction), DefaultSev: SeverityNote, RunnerGroup: "srp-package"},
	{ID: CheckSRPMixedInputSurface, Name: "Mixed input surface", Rule: RuleSRP, Scope: ScopePackage, Doc: "A function accepts many parameters spanning unrelated types.", HelpURI: checkHelpURI(CheckSRPMixedInputSurface), DefaultSev: SeverityNote, RunnerGroup: "srp-package"},
	{ID: CheckSRPDataClump, Name: "Data clump", Rule: RuleSRP, Scope: ScopePackage, Doc: "The same parameter tuple repeats across functions.", HelpURI: checkHelpURI(CheckSRPDataClump), DefaultSev: SeverityNote, RunnerGroup: "srp-package"},
	{ID: CheckSRPFlagArgument, Name: "Flag argument", Rule: RuleSRP, Scope: ScopePackage, Doc: "A boolean parameter selects between behaviors.", HelpURI: checkHelpURI(CheckSRPFlagArgument), DefaultSev: SeverityNote, RunnerGroup: "srp-package"},
	{ID: CheckSRPMixedImportClusters, Name: "Mixed import clusters", Rule: RuleSRP, Scope: ScopePackage, Doc: "Methods on one type import unrelated package clusters.", HelpURI: checkHelpURI(CheckSRPMixedImportClusters), DefaultSev: SeverityNote, RunnerGroup: "srp-package"},
	{
		ID: CheckOCPTypeDispatch, Name: "Concrete type dispatch", Rule: RuleOCP, Scope: ScopeProgram,
		Doc:     "Flags repeated dispatch on concrete types instead of polymorphism.",
		HelpURI: checkHelpURI(CheckOCPTypeDispatch), DefaultSev: SeverityWarning,
		RunnerGroup: "ocp-program",
		RunProgram:  CheckOCPProgram,
	},
	{ID: CheckOCPDiscriminatorDispatch, Name: "Repeated discriminator dispatch", Rule: RuleOCP, Scope: ScopeProgram, Doc: "Repeated dispatch on a discriminator field.", HelpURI: checkHelpURI(CheckOCPDiscriminatorDispatch), DefaultSev: SeverityWarning, RunnerGroup: "ocp-program"},
	{ID: CheckOCPRuntimeExhaustiveness, Name: "Runtime exhaustiveness failure", Rule: RuleOCP, Scope: ScopeProgram, Doc: "A type switch or serialization path may miss variants.", HelpURI: checkHelpURI(CheckOCPRuntimeExhaustiveness), DefaultSev: SeverityNote, RunnerGroup: "ocp-program"},
	{ID: CheckOCPConcreteParameter, Name: "Concrete parameter", Rule: RuleOCP, Scope: ScopeProgram, Doc: "A concrete type is used where an interface would allow extension.", HelpURI: checkHelpURI(CheckOCPConcreteParameter), DefaultSev: SeverityNote, RunnerGroup: "ocp-program"},
	{ID: CheckOCPClosedFactory, Name: "Closed factory", Rule: RuleOCP, Scope: ScopeProgram, Doc: "A factory returns a closed set of concrete types.", HelpURI: checkHelpURI(CheckOCPClosedFactory), DefaultSev: SeverityWarning, RunnerGroup: "ocp-program"},
	{ID: CheckOCPImplementationCoupling, Name: "Implementation package coupling", Rule: RuleOCP, Scope: ScopeProgram, Doc: "Logic packages import too many implementation packages.", HelpURI: checkHelpURI(CheckOCPImplementationCoupling), DefaultSev: SeverityWarning, RunnerGroup: "ocp-program"},
	{ID: CheckOCPParallelImplementations, Name: "Parallel implementations", Rule: RuleOCP, Scope: ScopeProgram, Doc: "Several functions implement the same algorithm in parallel.", HelpURI: checkHelpURI(CheckOCPParallelImplementations), DefaultSev: SeverityNote, RunnerGroup: "ocp-program"},
	{
		ID: CheckLSPNonExactEOF, Name: "Non-exact io.EOF", Rule: RuleLSP, Scope: ScopePackage,
		Doc:     "Flags io.Reader implementations that do not return io.EOF exactly.",
		HelpURI: checkHelpURI(CheckLSPNonExactEOF), DefaultSev: SeverityWarning,
		RunnerGroup: "lsp-package",
		RunPackage:  runLSPPackageCheck,
	},
	{
		ID: CheckLSPNilEmbeddedInterface, Name: "Nil embedded interface", Rule: RuleLSP, Scope: ScopeProgram,
		Doc:     "Flags embedded interfaces that may remain nil across the workspace.",
		HelpURI: checkHelpURI(CheckLSPNilEmbeddedInterface), DefaultSev: SeverityWarning,
		RunnerGroup: "lsp-program",
		RunProgram:  CheckLSPProgram,
	},
	{
		ID: CheckISPFatInterface, Name: "Fat interface", Rule: RuleISP, Scope: ScopePackage,
		Doc:     "Flags interfaces with more methods than clients typically need.",
		HelpURI: checkHelpURI(CheckISPFatInterface), DefaultSev: SeverityWarning,
		RunnerGroup: "isp-package",
		RunPackage:  runISPCheck,
	},
	{ID: CheckISPUsageRatio, Name: "Interface usage ratio", Rule: RuleISP, Scope: ScopePackage, Doc: "A client uses only a small fraction of an interface.", HelpURI: checkHelpURI(CheckISPUsageRatio), DefaultSev: SeverityWarning, RunnerGroup: "isp-package"},
	{ID: CheckISPConsumerRole, Name: "Consumer interface role", Rule: RuleISP, Scope: ScopePackage, Doc: "Flags a consumer field that receives a broader interface than its role needs.", HelpURI: checkHelpURI(CheckISPConsumerRole), DefaultSev: SeverityWarning, RunnerGroup: "isp-package"},
	{ID: CheckISPUnusedDependency, Name: "Unused injected dependency", Rule: RuleISP, Scope: ScopePackage, Doc: "Flags an injected interface dependency that is never consumed, including short unread field flows.", HelpURI: checkHelpURI(CheckISPUnusedDependency), DefaultSev: SeverityWarning, RunnerGroup: "isp-package"},
	{ID: CheckISPStubImplementation, Name: "Stub implementation", Rule: RuleISP, Scope: ScopePackage, Doc: "A method unconditionally panics or returns ErrUnsupported.", HelpURI: checkHelpURI(CheckISPStubImplementation), DefaultSev: SeverityWarning, RunnerGroup: "isp-package"},
	{
		ID: CheckDIPConcreteDependency, Name: "Concrete dependency", Rule: RuleDIP, Scope: ScopePackage,
		Doc:     "Flags struct fields wired directly to concrete collaborators.",
		HelpURI: checkHelpURI(CheckDIPConcreteDependency), DefaultSev: SeverityWarning,
		RunnerGroup: "dip-package",
		RunPackage:  runDIPPackageCheck,
	},
	{
		ID: CheckDIPLayerImport, Name: "Layer import violation", Rule: RuleDIP, Scope: ScopeProgram,
		Doc:     "Flags logic packages importing implementation packages.",
		HelpURI: checkHelpURI(CheckDIPLayerImport), DefaultSev: SeverityWarning,
		RunnerGroup: "dip-program",
		RunProgram:  CheckDIPProgram,
	},
	{ID: CheckDIPWiringOutsideRoot, Name: "Wiring outside composition root", Rule: RuleDIP, Scope: ScopeProgram, Doc: "Concrete wiring happens outside a composition root.", HelpURI: checkHelpURI(CheckDIPWiringOutsideRoot), DefaultSev: SeverityWarning, RunnerGroup: "dip-program"},
	{ID: CheckDIPHiddenConstruction, Name: "Hidden construction", Rule: RuleDIP, Scope: ScopeProgram, Doc: "A type constructs its own concrete collaborators.", HelpURI: checkHelpURI(CheckDIPHiddenConstruction), DefaultSev: SeverityWarning, RunnerGroup: "dip-program"},
	{ID: CheckDIPInfraErrorLeak, Name: "Infrastructure error leak", Rule: RuleDIP, Scope: ScopeProgram, Doc: "Infrastructure errors leak into logic packages.", HelpURI: checkHelpURI(CheckDIPInfraErrorLeak), DefaultSev: SeverityWarning, RunnerGroup: "dip-program"},
	{ID: CheckDIPTransportLeak, Name: "Transport type leak", Rule: RuleDIP, Scope: ScopeProgram, Doc: "Transport types appear in logic-layer signatures.", HelpURI: checkHelpURI(CheckDIPTransportLeak), DefaultSev: SeverityWarning, RunnerGroup: "dip-program"},
})

func completeCheckMetadata(checks []Check) []Check {
	stable := map[CheckID]bool{
		CheckSRPLargeType: true, CheckSRPDataClump: true,
		CheckOCPTypeDispatch: true, CheckISPFatInterface: true,
		CheckISPUsageRatio: true, CheckISPStubImplementation: true,
		CheckDIPConcreteDependency: true,
	}
	equivalent := map[CheckID]bool{
		CheckSRPComplexFunction: true, CheckOCPImplementationCoupling: true,
		CheckISPFatInterface: true, CheckDIPLayerImport: true,
	}
	conservative := map[CheckID]bool{
		CheckSRPLargeType: true, CheckSRPHighFanOutType: true,
		CheckSRPMixedInputSurface: true, CheckSRPDataClump: true,
		CheckSRPFlagArgument: true, CheckSRPMixedImportClusters: true,
		CheckOCPTypeDispatch: true, CheckOCPRuntimeExhaustiveness: true,
		CheckDIPConcreteDependency: true,
	}
	pluginChecks := map[CheckID]bool{
		CheckSRPGodType: true, CheckSRPLowCohesionType: true,
		CheckSRPLargeType: true, CheckSRPHighFanOutType: true,
		CheckSRPComplexFunction: true, CheckSRPMixedInputSurface: true,
		CheckSRPDataClump: true, CheckSRPFlagArgument: true,
		CheckSRPMixedImportClusters: true, CheckLSPNonExactEOF: true,
		CheckISPFatInterface: true, CheckISPUsageRatio: true,
		CheckISPConsumerRole: true, CheckISPUnusedDependency: true,
		CheckISPStubImplementation: true, CheckDIPConcreteDependency: true,
	}
	for index := range checks {
		check := &checks[index]
		check.Maturity = MaturityExperimental
		if stable[check.ID] {
			check.Maturity = MaturityStable
		}
		switch {
		case equivalent[check.ID]:
			check.Syntax = SyntaxEquivalent
		case conservative[check.ID]:
			check.Syntax = SyntaxConservative
		default:
			check.Syntax = SyntaxUnavailable
		}
		check.Surfaces = SurfaceCLI
		if pluginChecks[check.ID] {
			check.Surfaces |= SurfaceModulePlugin | SurfaceGoPlugin
		}
	}
	return checks
}

var checkByID = buildCheckIndex()

func buildCheckIndex() map[CheckID]Check {
	index := make(map[CheckID]Check, len(checkRegistry))
	for _, check := range checkRegistry {
		index[check.ID] = check
	}
	return index
}

func checkHelpURI(id CheckID) string {
	return checkDocsBaseURI + "/" + string(id) + ".md"
}

// CheckHelpURI returns documentation URL for a check ID when known.
func CheckHelpURI(id CheckID) string {
	if check, ok := checkByID[id]; ok {
		return check.HelpURI
	}
	return checkHelpURI(id)
}

// CheckDoc returns a short description for a check ID when known.
func CheckDoc(id CheckID) string {
	if check, ok := checkByID[id]; ok {
		return check.Doc
	}
	return ""
}

// CheckMetadata returns the authoritative public metadata for a concrete check.
func CheckMetadata(id CheckID) (Check, bool) {
	check, ok := checkByID[id]
	return check, ok
}

// RegisteredCheckIDs returns every concrete check ID in deterministic order.
func RegisteredCheckIDs() []CheckID {
	ids := make([]CheckID, 0, len(checkRegistry))
	for _, check := range checkRegistry {
		ids = append(ids, check.ID)
	}
	return ids
}

// ResolveCheckSelection applies the public profile/check/family precedence in
// one deterministic place. Explicit enables add checks to the profile before
// family and disabled-check filters are applied.
func ResolveCheckSelection(profile Profile, enabledRules map[Rule]bool, enabledChecks, disabledChecks []CheckID) (map[CheckID]bool, error) {
	if profile == "" {
		profile = ProfileStable
	}
	if profile != ProfileStable && profile != ProfileAll && profile != ProfileCalibration {
		return nil, fmt.Errorf("profile must be stable, all, or calibration, got %q", profile)
	}
	enabledExact := make(map[CheckID]bool, len(enabledChecks))
	for _, id := range enabledChecks {
		if _, ok := checkByID[id]; !ok {
			return nil, fmt.Errorf("unknown enabled check %q", id)
		}
		enabledExact[id] = true
	}
	disabledExact := make(map[CheckID]bool, len(disabledChecks))
	for _, id := range disabledChecks {
		if _, ok := checkByID[id]; !ok {
			return nil, fmt.Errorf("unknown disabled check %q", id)
		}
		if enabledExact[id] {
			return nil, fmt.Errorf("check %q cannot be both enabled and disabled", id)
		}
		disabledExact[id] = true
	}
	selection := make(map[CheckID]bool, len(checkRegistry))
	for _, check := range checkRegistry {
		inProfile := profile == ProfileAll ||
			(profile == ProfileStable && check.Maturity == MaturityStable) ||
			(profile == ProfileCalibration && calibrationChecks[check.ID])
		familyEnabled := len(enabledRules) == 0 || enabledRules[check.Rule]
		selection[check.ID] = (inProfile || enabledExact[check.ID]) && familyEnabled && !disabledExact[check.ID]
	}
	return selection, nil
}

var calibrationChecks = map[CheckID]bool{
	CheckISPConsumerRole:     true,
	CheckISPUnusedDependency: true,
}

// SelectedCheckIDs returns selected IDs in registry order.
func SelectedCheckIDs(selection map[CheckID]bool) []CheckID {
	ids := make([]CheckID, 0, len(selection))
	for _, check := range checkRegistry {
		if selection[check.ID] {
			ids = append(ids, check.ID)
		}
	}
	return ids
}

var knownCheckIDs = buildKnownCheckIDs()

func buildKnownCheckIDs() map[string]bool {
	ids := map[string]bool{
		string(RuleSRP): true, string(RuleOCP): true, string(RuleLSP): true,
		string(RuleISP): true, string(RuleDIP): true,
	}
	for _, id := range RegisteredCheckIDs() {
		ids[string(id)] = true
	}
	return ids
}

// IsKnownCheckID reports whether id is a registered concrete check.
func IsKnownCheckID(id string) bool {
	return knownCheckIDs[id]
}

// IsKnownSeverityTarget reports whether key is a valid severity override target.
func IsKnownSeverityTarget(key string) bool {
	return IsKnownCheckID(key)
}

func runSRPCheck(pkg *packageFiles, cfg Config) []Issue {
	return CheckSRPWithTypes(SRPCheckInput{
		Fset: pkg.fset, Files: pkg.files, Info: pkg.info,
		Pkg: pkg.typePkg, TypeComplete: pkg.typeComplete, Config: cfg,
		PkgFiles: pkg,
	})
}

func runLSPPackageCheck(pkg *packageFiles, cfg Config) []Issue {
	return CheckLSPWithTypes(pkg.fset, pkg.files, pkg.info, cfg, pkg)
}

func runISPCheck(pkg *packageFiles, cfg Config) []Issue {
	return CheckISPWithTypes(pkg.fset, pkg.files, pkg.info, cfg, pkg)
}

func runDIPPackageCheck(pkg *packageFiles, cfg Config) []Issue {
	return CheckDIPWithTypes(pkg.fset, pkg.files, pkg.info, cfg, pkg)
}
