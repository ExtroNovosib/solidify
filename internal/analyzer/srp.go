package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"strings"
)

// CheckSRP runs syntax-only SRP checks. Type-dependent strict checks are
// intentionally unavailable through this compatibility entry point.
func CheckSRP(fset *token.FileSet, files []*ast.File, cfg Config) []Issue {
	// Preserve the historical direct API's line-only behavior. Package runs
	// use CheckSRPWithTypes and apply the stricter LOC+complexity signal.
	cfg.MaxFuncComplexity = 0
	return checkSRPSyntax(fset, files, cfg, nil)
}

// checkSRPSyntax flags several explainable symptoms of a type/function doing too
// much:
//
//  1. A type (struct) that accumulates more methods than MaxMethodsPerType.
//     A large method set is the most common real-world signal of a
//     "god object" that mixes several responsibilities.
//  2. A function/method body longer than MaxFuncLines lines.
//  3. A long parameter list that spans several distinct types, or repeats as
//     a data clump in another function. Raw parameter count alone is
//     deliberately not a finding: homogeneous mathematical and batch APIs
//     can have many inputs while remaining cohesive.
//  4. A boolean parameter used to choose between two behaviors. Such flag
//     arguments expose two reasons for the function to change and are better
//     represented by intention-revealing entry points or an options type.
func checkSRPSyntax(fset *token.FileSet, files []*ast.File, cfg Config, pkg *packageFiles) []Issue {
	var issues []Issue
	methodCount := map[string]int{}         // receiver type name -> number of methods
	methodExample := map[string]token.Pos{} // first method position, for reporting
	methodExampleEnd := map[string]token.Pos{}
	var parameterProfiles []*functionParameterProfile
	parameterChecks := checkEnabled(cfg, CheckSRPFlagArgument) || checkEnabled(cfg, CheckSRPMixedInputSurface) || checkEnabled(cfg, CheckSRPDataClump)

	for _, f := range files {
		if skipGenerated(pkg, f) {
			continue
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if checkEnabled(cfg, CheckSRPComplexFunction) {
				if issue, found := functionLengthIssue(fset, fn, cfg); found {
					issues = append(issues, issue)
				}
			}
			if parameterChecks {
				profile, issue, found := parameterResponsibilityIssue(fset, fn, cfg)
				parameterProfiles = append(parameterProfiles, profile)
				if found {
					issues = append(issues, issue)
				}
			}
			if checkEnabled(cfg, CheckSRPLargeType) && fn.Recv != nil && len(fn.Recv.List) > 0 {
				typeName := receiverTypeName(fn.Recv.List[0].Type)
				if typeName != "" {
					methodCount[typeName]++
					if _, seen := methodExample[typeName]; !seen {
						methodExample[typeName] = fn.Pos()
						methodExampleEnd[typeName] = fn.End()
					}
				}
			}
		}
	}
	if checkEnabled(cfg, CheckSRPDataClump) {
		issues = append(issues, parameterDataClumpIssues(fset, parameterProfiles, cfg)...)
	}
	if checkEnabled(cfg, CheckSRPLargeType) {
		issues = append(issues, largeMethodSetIssues(fset, methodCount, methodExample, methodExampleEnd, cfg)...)
	}
	return issues
}

func functionLengthIssue(fset *token.FileSet, fn *ast.FuncDecl, cfg Config) (Issue, bool) {
	if fn.Body == nil {
		return Issue{}, false
	}
	start := fset.Position(fn.Body.Lbrace).Line
	end := fset.Position(fn.Body.Rbrace).Line
	lines := end - start
	complexity := functionComplexity(fn)
	if lines <= cfg.MaxFuncLines || complexity < cfg.MaxFuncComplexity {
		return Issue{}, false
	}
	return issueAt(fset, fn, Issue{
		Rule:     RuleSRP,
		Check:    CheckSRPComplexFunction,
		Severity: SeverityNote,
		Metrics:  []Metric{{Name: "loc", Value: float64(lines), Threshold: float64(cfg.MaxFuncLines), Comparator: ">"}, {Name: "complexity", Value: float64(complexity), Threshold: float64(cfg.MaxFuncComplexity), Comparator: ">="}},
		Evidence: fmt.Sprintf("complex-function:function=%s;loc=%d;complexity=%d", fn.Name.Name, lines, complexity),
		Message: fmt.Sprintf(
			"function %q is %d lines long with cyclomatic complexity %d: split its independent responsibilities",
			fn.Name.Name, lines, complexity,
		),
	}), true
}

func parameterResponsibilityIssue(fset *token.FileSet, fn *ast.FuncDecl, cfg Config) (*functionParameterProfile, Issue, bool) {
	profile := profileFunctionParameters(fn)
	if flagNames := behaviorSelectingFlags(fn, profile.boolParameters); checkEnabled(cfg, CheckSRPFlagArgument) && len(flagNames) > 0 {
		profile.reported = true
		return profile, issueAt(fset, fn, Issue{
			Rule:     RuleSRP,
			Check:    CheckSRPFlagArgument,
			Severity: SeverityNote,
			Evidence: fmt.Sprintf("flag-argument:function=%s;parameters=%s", fn.Name.Name, strings.Join(flagNames, ",")),
			Message: fmt.Sprintf(
				"function %q uses boolean parameter(s) %q to select between behaviors: expose intention-revealing functions or an options type instead",
				fn.Name.Name, strings.Join(flagNames, ", "),
			),
		}), true
	}
	if !checkEnabled(cfg, CheckSRPMixedInputSurface) || profile.count <= cfg.MaxFuncParams || profile.distinctTypes < 3 {
		return profile, Issue{}, false
	}
	profile.reported = true
	return profile, issueAt(fset, fn, Issue{
		Rule:     RuleSRP,
		Check:    CheckSRPMixedInputSurface,
		Severity: SeverityNote,
		Evidence: fmt.Sprintf("mixed-parameters:function=%s;count=%d;types=%d;max=%d", fn.Name.Name, profile.count, profile.distinctTypes, cfg.MaxFuncParams),
		Message: fmt.Sprintf(
			"function %q takes %d parameters spanning %d distinct types (max %d): this broad input surface suggests mixed responsibilities; introduce a cohesive request or collaborator",
			fn.Name.Name, profile.count, profile.distinctTypes, cfg.MaxFuncParams,
		),
	}), true
}

func parameterDataClumpIssues(fset *token.FileSet, parameterProfiles []*functionParameterProfile, cfg Config) []Issue {
	clumps := buildParameterClumps(parameterProfiles, cfg)
	pruneSubsumedClumps(clumps)
	return clumpToIssues(fset, clumps)
}

type parameterClump struct {
	params []functionParameter
	funcs  []*functionParameterProfile
}

func buildParameterClumps(parameterProfiles []*functionParameterProfile, cfg Config) map[string]*parameterClump {
	clumps := map[string]*parameterClump{}
	for i, left := range parameterProfiles {
		if left.reported {
			continue
		}
		for j := i + 1; j < len(parameterProfiles); j++ {
			right := parameterProfiles[j]
			if right.reported {
				continue
			}
			shared := sharedParameters(left, right)
			if len(shared) <= cfg.MaxFuncParams {
				continue
			}
			keys := make([]string, 0, len(shared))
			for _, parameter := range shared {
				keys = append(keys, parameter.name+"\x00"+parameter.typeKey)
			}
			sort.Strings(keys)
			signature := strings.Join(keys, "\x00")
			candidate := clumps[signature]
			if candidate == nil {
				candidate = &parameterClump{params: shared}
				clumps[signature] = candidate
			}
			if !containsProfile(candidate.funcs, left) {
				candidate.funcs = append(candidate.funcs, left)
			}
			if !containsProfile(candidate.funcs, right) {
				candidate.funcs = append(candidate.funcs, right)
			}
		}
	}
	return clumps
}

func pruneSubsumedClumps(clumps map[string]*parameterClump) {
	for signature, candidate := range clumps {
		for otherSignature, other := range clumps {
			if signature == otherSignature || len(candidate.params) >= len(other.params) {
				continue
			}
			if parameterSetContains(other.params, candidate.params) {
				delete(clumps, signature)
				break
			}
		}
	}
}

func clumpToIssues(fset *token.FileSet, clumps map[string]*parameterClump) []Issue {
	var issues []Issue
	for _, candidate := range clumps {
		if len(candidate.funcs) < 2 {
			continue
		}
		sort.Slice(candidate.funcs, func(i, j int) bool { return candidate.funcs[i].pos < candidate.funcs[j].pos })
		current := candidate.funcs[len(candidate.funcs)-1]
		peer := candidate.funcs[0]
		if current == peer {
			peer = candidate.funcs[1]
		}
		sharedNames := make([]string, 0, len(candidate.params))
		for _, parameter := range candidate.params {
			sharedNames = append(sharedNames, parameter.name)
		}
		parameterNames := strings.Join(sharedNames, ",")
		message := fmt.Sprintf("function %q repeats %d parameters also used by %q: this data clump is a missing domain type; extract it into a parameter object", current.name, len(candidate.params), peer.name)
		issues = append(issues, issueSpan(fset, current.pos, current.end, Issue{Rule: RuleSRP, Check: CheckSRPDataClump, Severity: SeverityNote, Evidence: fmt.Sprintf("data-clump:function=%s;peer=%s;parameters=%s", current.name, peer.name, parameterNames), Message: message, Groups: []SymbolGroup{{Label: "data-clump-functions", Symbols: SortedSymbols(profileNames(candidate.funcs))}}}))
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].Pos.Line < issues[j].Pos.Line })
	return issues
}

func containsProfile(profiles []*functionParameterProfile, target *functionParameterProfile) bool {
	for _, profile := range profiles {
		if profile == target {
			return true
		}
	}
	return false
}

func profileNames(profiles []*functionParameterProfile) []string {
	names := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		names = append(names, profile.name)
	}
	return names
}

func parameterSetContains(super, subset []functionParameter) bool {
	keys := make(map[string]bool, len(super))
	for _, parameter := range super {
		keys[parameter.name+"\x00"+parameter.typeKey] = true
	}
	for _, parameter := range subset {
		if !keys[parameter.name+"\x00"+parameter.typeKey] {
			return false
		}
	}
	return true
}

func largeMethodSetIssues(fset *token.FileSet, methodCount map[string]int, methodExample map[string]token.Pos, methodExampleEnd map[string]token.Pos, cfg Config) []Issue {
	var issues []Issue
	for typeName, count := range methodCount {
		if count > cfg.MaxMethodsPerType {
			issues = append(issues, issueSpan(fset, methodExample[typeName], methodExampleEnd[typeName], Issue{
				Rule:     RuleSRP,
				Check:    CheckSRPLargeType,
				Severity: SeverityNote,
				Message: fmt.Sprintf(
					"type %q has %d methods (max %d): it likely has more than one responsibility; consider splitting it into smaller collaborating types",
					typeName, count, cfg.MaxMethodsPerType,
				),
			}))
		}
	}
	return issues
}

type functionParameter struct {
	name    string
	typeKey string
}

type functionParameterProfile struct {
	name           string
	pos            token.Pos
	end            token.Pos
	parameters     []functionParameter
	boolParameters []functionParameter
	count          int
	distinctTypes  int
	reported       bool
}

func profileFunctionParameters(fn *ast.FuncDecl) *functionParameterProfile {
	profile := &functionParameterProfile{name: fn.Name.Name, pos: fn.Pos(), end: fn.End()}
	if fn.Type.Params == nil {
		return profile
	}

	types := map[string]bool{}
	for _, field := range fn.Type.Params.List {
		if isContextTypeExpr(field.Type) {
			continue
		}
		typeKey := expressionKey(field.Type)
		types[typeKey] = true
		if len(field.Names) == 0 {
			profile.count++
			continue
		}
		for _, name := range field.Names {
			profile.count++
			if name.Name == "_" {
				continue
			}
			parameter := functionParameter{name: name.Name, typeKey: typeKey}
			profile.parameters = append(profile.parameters, parameter)
			if ident, ok := field.Type.(*ast.Ident); ok && ident.Name == "bool" {
				profile.boolParameters = append(profile.boolParameters, parameter)
			}
		}
	}
	profile.distinctTypes = len(types)
	return profile
}

// behaviorSelectingFlags returns boolean parameters used in a two-sided if or
// a switch. Merely forwarding or storing a bool is not enough evidence: the
// parameter must visibly choose between alternate control-flow paths.
func behaviorSelectingFlags(fn *ast.FuncDecl, candidates []functionParameter) []string {
	if fn.Body == nil || len(candidates) == 0 {
		return nil
	}
	selected := map[string]bool{}
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		var condition ast.Expr
		switch stmt := node.(type) {
		case *ast.IfStmt:
			if stmt.Else != nil {
				condition = stmt.Cond
			}
		case *ast.SwitchStmt:
			if len(stmt.Body.List) >= 2 {
				condition = stmt.Tag
			}
		}
		if condition == nil {
			return true
		}
		for _, candidate := range candidates {
			if expressionUsesParameter(condition, candidate) {
				selected[candidate.name] = true
			}
		}
		return true
	})

	var names []string
	for _, candidate := range candidates {
		if selected[candidate.name] {
			names = append(names, candidate.name)
		}
	}
	return names
}

func expressionUsesParameter(expr ast.Expr, parameter functionParameter) bool {
	used := false
	ast.Inspect(expr, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if !ok {
			return true
		}
		if ident.Name == parameter.name {
			used = true
			return false
		}
		return true
	})
	return used
}

func sharedParameters(left, right *functionParameterProfile) []functionParameter {
	leftKeys := make(map[string]bool, len(left.parameters))
	for _, parameter := range left.parameters {
		leftKeys[parameter.name+"\x00"+parameter.typeKey] = true
	}
	var shared []functionParameter
	for _, parameter := range right.parameters {
		if leftKeys[parameter.name+"\x00"+parameter.typeKey] {
			shared = append(shared, parameter)
		}
	}
	return shared
}

// receiverTypeName extracts "Foo" from either "Foo" or "*Foo" receiver
// expressions.
func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	case *ast.IndexExpr:
		return receiverTypeName(t.X)
	case *ast.IndexListExpr:
		return receiverTypeName(t.X)
	default:
		return ""
	}
}
