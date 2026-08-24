package analyzer

import (
	"go/ast"
	"go/token"
	"go/types"
)

const ocpKindTypeAssertion = "type assertion"

type ocpDispatchSite struct {
	pkg           *packageFiles
	node          ast.Node
	pos           token.Position
	function      string
	source        types.Type
	sourceKey     string
	variants      []string
	kind          string
	defaultBad    bool
	serialization bool
}

type ocpDiscriminatorSite struct {
	pkg           *packageFiles
	node          ast.Node
	pos           token.Position
	function      string
	fieldKey      string
	values        []string
	defaultBad    bool
	serialization bool
}

type ocpAnalysis struct {
	dispatches      []*ocpDispatchSite
	discriminators  []*ocpDiscriminatorSite
	flaggedDispatch map[*ocpDispatchSite]bool
	flaggedDisc     map[*ocpDiscriminatorSite]bool
}

// CheckOCP retains the package-local API used by the focused unit tests. The
// CLI and Run use CheckOCPProgram so dispatch families can span packages.
func CheckOCP(fset *token.FileSet, files []*ast.File, cfg Config) []Issue {
	pkg := &packageFiles{fset: fset, files: files, generated: map[*ast.File]bool{}}
	for _, file := range files {
		pkg.generated[file] = ast.IsGenerated(file)
	}
	analysis := collectOCPAnalysis([]*packageFiles{pkg}, cfg)
	return emitOCPDispatch(analysis, cfg)
}

// CheckOCPProgram runs all OCP checks over one consistent package universe.
func CheckOCPProgram(pkgs []*packageFiles, cfg Config) []Issue {
	needsAnalysis := checkEnabled(cfg, CheckOCPTypeDispatch) || checkEnabled(cfg, CheckOCPDiscriminatorDispatch) || checkEnabled(cfg, CheckOCPRuntimeExhaustiveness) || checkEnabled(cfg, CheckOCPClosedFactory)
	analysis := ocpAnalysis{flaggedDispatch: map[*ocpDispatchSite]bool{}, flaggedDisc: map[*ocpDiscriminatorSite]bool{}}
	if needsAnalysis {
		analysis = collectOCPAnalysis(pkgs, cfg)
	}
	var issues []Issue
	if checkEnabled(cfg, CheckOCPTypeDispatch) {
		issues = emitOCPDispatch(analysis, cfg)
	}
	if checkEnabled(cfg, CheckOCPDiscriminatorDispatch) {
		issues = append(issues, emitOCPDiscriminators(analysis, cfg)...)
	}
	if checkEnabled(cfg, CheckOCPRuntimeExhaustiveness) {
		issues = append(issues, emitOCPRuntime(analysis)...)
	}
	if checkEnabled(cfg, CheckOCPConcreteParameter) {
		issues = append(issues, emitOCPConcreteParameters(pkgs, cfg)...)
	}
	var factoryIssues []Issue
	var factorySitePositions map[string]bool
	if checkEnabled(cfg, CheckOCPClosedFactory) || checkEnabled(cfg, CheckOCPTypeDispatch) {
		factoryIssues, factorySitePositions = emitOCPFactories(pkgs, cfg)
	}
	if checkEnabled(cfg, CheckOCPTypeDispatch) && len(factorySitePositions) > 0 {
		filtered := issues[:0]
		for _, issue := range issues {
			if issue.ID() == string(CheckOCPTypeDispatch) && factorySitePositions[positionKey(issue.Pos)] {
				continue
			}
			filtered = append(filtered, issue)
		}
		issues = filtered
	}
	if checkEnabled(cfg, CheckOCPClosedFactory) {
		issues = append(issues, factoryIssues...)
	}
	if checkEnabled(cfg, CheckOCPImplementationCoupling) {
		issues = append(issues, emitOCPImplementationCoupling(pkgs, cfg)...)
	}
	if checkEnabled(cfg, CheckOCPParallelImplementations) {
		issues = append(issues, emitOCPParallelImplementations(pkgs, cfg)...)
	}
	return issues
}

func collectOCPAnalysis(pkgs []*packageFiles, cfg Config) ocpAnalysis {
	result := ocpAnalysis{flaggedDispatch: map[*ocpDispatchSite]bool{}, flaggedDisc: map[*ocpDiscriminatorSite]bool{}}
	for _, pkg := range pkgs {
		for _, file := range pkg.files {
			if !ocpFileEnabled(pkg, file, cfg) {
				continue
			}
			functions := functionDecls(file)
			visited := map[*ast.IfStmt]bool{}
			coveredAssertions := map[*ast.TypeAssertExpr]bool{}
			ast.Inspect(file, func(node ast.Node) bool {
				switch current := node.(type) {
				case *ast.TypeSwitchStmt:
					sourceExpr := typeSwitchSource(current)
					if sourceExpr == nil {
						return true
					}
					variants := typeSwitchVariants(current, pkg.info)
					source := expressionType(pkg.info, sourceExpr)
					result.dispatches = append(result.dispatches, &ocpDispatchSite{
						pkg: pkg, node: current, pos: pkg.fset.Position(current.Pos()),
						function: enclosingFunction(functions, current.Pos(), pkg.pkgPath),
						source:   source, sourceKey: dispatchSourceKey(pkg, file, source, sourceExpr, false), variants: uniqueSorted(variants),
						kind:          "type switch",
						defaultBad:    typeSwitchHasUnsupportedDefault(current),
						serialization: isSerializationFunction(enclosingFunctionName(functions, current.Pos())),
					})
				case *ast.IfStmt:
					if visited[current] || !isTypeAssertionIf(current) {
						return true
					}
					length, links := typeAssertChain(current)
					if length == 0 {
						return true
					}
					for _, link := range links {
						visited[link] = true
						markTypeAssertions(link, coveredAssertions)
					}
					sourceExpr, ok := typeAssertionOperand(current)
					if !ok {
						return true
					}
					variants := make([]string, 0, length)
					for _, link := range links {
						variants = append(variants, typeAssertChainTargets(link, pkg.info)...)
					}
					source := expressionType(pkg.info, sourceExpr)
					result.dispatches = append(result.dispatches, &ocpDispatchSite{
						pkg: pkg, node: current, pos: pkg.fset.Position(current.Pos()),
						function: enclosingFunction(functions, current.Pos(), pkg.pkgPath),
						source:   source, sourceKey: dispatchSourceKey(pkg, file, source, sourceExpr, length == 1), variants: uniqueSorted(variants),
						kind: func() string {
							if length == 1 {
								return ocpKindTypeAssertion
							}
							return "if/else-if chain"
						}(),
						serialization: isSerializationFunction(enclosingFunctionName(functions, current.Pos())),
					})
				case *ast.TypeAssertExpr:
					if current.Type == nil || coveredAssertions[current] {
						return true
					}
					source := expressionType(pkg.info, current.X)
					result.dispatches = append(result.dispatches, &ocpDispatchSite{
						pkg: pkg, node: current, pos: pkg.fset.Position(current.Pos()),
						function: enclosingFunction(functions, current.Pos(), pkg.pkgPath),
						source:   source, sourceKey: dispatchSourceKey(pkg, file, source, current.X, true), variants: []string{typeExpressionKey(current.Type, pkg.info)},
						kind:          ocpKindTypeAssertion,
						serialization: isSerializationFunction(enclosingFunctionName(functions, current.Pos())),
					})
				case *ast.SwitchStmt:
					if current.Tag == nil || pkg.info == nil {
						return true
					}
					if fieldKey, ok := discriminatorFieldKey(current.Tag, pkg.info); ok && discriminatorFieldAllowed(fieldKey, cfg) {
						values, badDefault := discriminatorSwitchValues(current, pkg.info)
						result.discriminators = append(result.discriminators, &ocpDiscriminatorSite{
							pkg: pkg, node: current, pos: pkg.fset.Position(current.Pos()),
							function: enclosingFunction(functions, current.Pos(), pkg.pkgPath), fieldKey: fieldKey,
							values: uniqueSorted(values), defaultBad: badDefault, serialization: isSerializationFunction(enclosingFunctionName(functions, current.Pos())),
						})
					}
				}
				return true
			})
			collectDiscriminatorIfChains(pkg, file, functions, cfg, &result)
		}
	}
	return result
}
