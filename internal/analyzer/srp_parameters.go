package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

func typedParameterIssues(fset *token.FileSet, files []*ast.File, info *types.Info, cfg Config, pkg *packageFiles) []Issue {
	var profiles []*functionParameterProfile
	var issues []Issue
	for _, file := range files {
		if skipGenerated(pkg, file) {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			profile, objects := profileTypedFunctionParameters(fn, info)
			profiles = append(profiles, profile)
			if names := typedBehaviorSelectingFlags(fn, profile.boolParameters, objects, info); len(names) > 0 {
				profile.reported = true
				message := fmt.Sprintf("function %q uses boolean parameter(s) %q to select between behaviors: expose intention-revealing functions or an options type instead", fn.Name.Name, strings.Join(names, ", "))
				issues = append(issues, issueAt(fset, fn, Issue{Rule: RuleSRP, Check: CheckSRPFlagArgument, Severity: SeverityNote, Evidence: fmt.Sprintf("flag-argument:function=%s;parameters=%s", fn.Name.Name, strings.Join(names, ",")), Message: message}))
				continue
			}
			if profile.count > cfg.MaxFuncParams && profile.distinctTypes >= 3 {
				profile.reported = true
				message := fmt.Sprintf("function %q takes %d parameters spanning %d distinct types (max %d): this broad input surface suggests mixed responsibilities; introduce a cohesive request or collaborator", fn.Name.Name, profile.count, profile.distinctTypes, cfg.MaxFuncParams)
				issues = append(issues, issueAt(fset, fn, Issue{Rule: RuleSRP, Check: CheckSRPMixedInputSurface, Severity: SeverityNote, Evidence: fmt.Sprintf("mixed-parameters:function=%s;count=%d;types=%d;max=%d", fn.Name.Name, profile.count, profile.distinctTypes, cfg.MaxFuncParams), Message: message}))
			}
		}
	}
	issues = append(issues, parameterDataClumpIssues(fset, profiles, cfg)...)
	return issues
}

func profileTypedFunctionParameters(fn *ast.FuncDecl, info *types.Info) (*functionParameterProfile, map[string]types.Object) {
	profile := &functionParameterProfile{name: fn.Name.Name, pos: fn.Pos()}
	objects := map[string]types.Object{}
	if fn.Type.Params == nil {
		return profile, objects
	}
	typesSeen := map[string]bool{}
	for _, field := range fn.Type.Params.List {
		t := info.TypeOf(field.Type)
		if isContextType(t) {
			continue
		}
		typeKey := canonicalTypeKey(t)
		typesSeen[typeKey] = true
		if len(field.Names) == 0 {
			profile.count++
			continue
		}
		for _, name := range field.Names {
			profile.count++
			if name.Name == "_" {
				continue
			}
			profile.parameters = append(profile.parameters, functionParameter{name: name.Name, typeKey: typeKey})
			if obj := info.Defs[name]; obj != nil {
				objects[name.Name] = obj
			}
			if isBooleanType(t) {
				profile.boolParameters = append(profile.boolParameters, functionParameter{name: name.Name, typeKey: typeKey})
			}
		}
	}
	profile.distinctTypes = len(typesSeen)
	return profile, objects
}

func canonicalTypeKey(t types.Type) string {
	if t == nil {
		return "<unknown>"
	}
	return types.TypeString(t, func(pkg *types.Package) string {
		if pkg == nil {
			return ""
		}
		return pkg.Path()
	})
}

func isContextType(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == "context" && named.Obj().Name() == "Context"
}

func isBooleanType(t types.Type) bool {
	underlying, ok := t.Underlying().(*types.Basic)
	return ok && underlying.Info()&types.IsBoolean != 0
}

func typedBehaviorSelectingFlags(fn *ast.FuncDecl, candidates []functionParameter, objects map[string]types.Object, info *types.Info) []string {
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
			if obj := objects[candidate.name]; obj != nil && typedExpressionUsesObject(condition, obj, info) {
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

func typedExpressionUsesObject(expr ast.Expr, object types.Object, info *types.Info) bool {
	used := false
	ast.Inspect(expr, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if !ok {
			return true
		}
		if info.Uses[ident] == object {
			used = true
			return false
		}
		return true
	})
	return used
}

func isContextTypeExpr(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == "context" && sel.Sel.Name == "Context"
}
