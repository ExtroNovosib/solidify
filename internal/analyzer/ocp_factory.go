package analyzer

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"
)

func emitOCPConcreteParameters(pkgs []*packageFiles, cfg Config) []Issue {
	var issues []Issue
	for _, pkg := range pkgs {
		if pkg.info == nil || !pkg.typeComplete {
			continue
		}
		for _, file := range pkg.files {
			if !ocpFileEnabled(pkg, file, cfg) {
				continue
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || fn.Type.Params == nil {
					continue
				}
				for _, field := range fn.Type.Params.List {
					paramType := pkg.info.TypeOf(field.Type)
					if !concreteTypeCandidate(paramType) || allowedDependency(canonicalTypeKey(paramType), cfg) {
						continue
					}
					for _, name := range field.Names {
						obj, ok := pkg.info.Defs[name].(*types.Var)
						if !ok {
							continue
						}
						methods, safe := concreteParameterMethods(fn.Body, obj, pkg.info)
						if !safe || len(methods) < cfg.OCPMinConcreteParameterMethods {
							continue
						}
						interfaceName := matchingInterface(pkg, methods)
						methodNameList := methodNames(methods)
						message := fmt.Sprintf("parameter %q has concrete type %s but is only used through methods %s; consider a consumer-defined interface", name.Name, canonicalTypeKey(paramType), strings.Join(methodNameList, ", "))
						if interfaceName != "" {
							message += fmt.Sprintf(" (matching interface: %s)", interfaceName)
						}
						issues = append(issues, issueAt(pkg.fset, name, Issue{Rule: RuleOCP, Check: CheckOCPConcreteParameter, Severity: SeverityNote, Message: message,
							Evidence: fmt.Sprintf("concrete-parameter:function=%s;parameter=%s;type=%s;methods=%s", fn.Name.Name, name.Name, canonicalTypeKey(paramType), strings.Join(methodNameList, ","))}))
					}
				}
			}
		}
	}
	return issues
}

func emitOCPFactories(pkgs []*packageFiles, cfg Config) ([]Issue, map[string]bool) {
	issues := []Issue{}
	positions := map[string]bool{}
	for _, pkg := range pkgs {
		if pkg.info == nil || !pkg.typeComplete {
			continue
		}
		for _, file := range pkg.files {
			if !ocpFileEnabled(pkg, file, cfg) {
				continue
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || !isFactoryName(fn.Name.Name) || !functionReturnsInterface(fn, pkg.info) {
					continue
				}
				for _, stmt := range fn.Body.List {
					sw, ok := stmt.(*ast.SwitchStmt)
					if !ok {
						continue
					}
					cases := countCaseClauses(sw.Body)
					if cases <= cfg.MaxTypeSwitchCases {
						continue
					}
					pos := pkg.fset.Position(sw.Pos())
					positions[positionKey(pos)] = true
					issues = append(issues, issueAt(pkg.fset, sw, Issue{Rule: RuleOCP, Check: CheckOCPClosedFactory, Severity: SeverityWarning,
						Message:  fmt.Sprintf("factory %q has %d hardcoded branches for an interface result (max %d); register constructors or inject a registry", fn.Name.Name, cases, cfg.MaxTypeSwitchCases),
						Evidence: fmt.Sprintf("closed-factory:function=%s;cases=%d;max=%d", fn.Name.Name, cases, cfg.MaxTypeSwitchCases), Metrics: []Metric{{Name: "cases", Value: float64(cases), Threshold: float64(cfg.MaxTypeSwitchCases)}}}))
				}
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					literal, ok := node.(*ast.CompositeLit)
					if !ok || pkg.info.TypeOf(literal.Type) == nil {
						return true
					}
					mapType, ok := pkg.info.TypeOf(literal.Type).Underlying().(*types.Map)
					if !ok || len(literal.Elts) <= cfg.MaxTypeSwitchCases || !factoryMapValue(mapType) {
						return true
					}
					pos := pkg.fset.Position(literal.Pos())
					if positions[positionKey(pos)] {
						return true
					}
					positions[positionKey(pos)] = true
					issues = append(issues, issueAt(pkg.fset, literal, Issue{Rule: RuleOCP, Check: CheckOCPClosedFactory, Severity: SeverityWarning,
						Message:  fmt.Sprintf("factory %q contains a static constructor table with %d entries (max %d); expose registration or inject the registry", fn.Name.Name, len(literal.Elts), cfg.MaxTypeSwitchCases),
						Evidence: fmt.Sprintf("closed-factory:function=%s;map_entries=%d;max=%d", fn.Name.Name, len(literal.Elts), cfg.MaxTypeSwitchCases), Metrics: []Metric{{Name: "map_entries", Value: float64(len(literal.Elts)), Threshold: float64(cfg.MaxTypeSwitchCases)}}}))
					return true
				})
			}
		}
	}
	return issues, positions
}

func factoryMapValue(mapType *types.Map) bool {
	value := mapType.Elem()
	if signature, ok := value.(*types.Signature); ok {
		if signature.Results() == nil {
			return false
		}
		for index := 0; index < signature.Results().Len(); index++ {
			if isInterface(signature.Results().At(index).Type()) {
				return true
			}
		}
	}
	return isInterface(value)
}

func isFactoryName(name string) bool {
	for _, prefix := range []string{"New", "Make", "Create", "Build", "Parse"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
