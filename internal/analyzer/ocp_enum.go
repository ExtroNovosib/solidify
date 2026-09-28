package analyzer

import (
	"go/ast"
	"go/types"
	"strings"
)

func switchDiscriminatorKey(stmt *ast.SwitchStmt, info *types.Info) (string, bool) {
	if key, ok := discriminatorFieldKey(stmt.Tag, info); ok {
		return key, true
	}
	// Normalized query parsing retains the enum identity in converted constants.
	key := ""
	conflict := false
	for _, raw := range stmt.Body.List {
		clause, ok := raw.(*ast.CaseClause)
		if !ok {
			continue
		}
		for _, expr := range clause.List {
			ast.Inspect(expr, func(n ast.Node) bool {
				var obj types.Object
				switch node := n.(type) {
				case *ast.Ident:
					obj = info.Uses[node]
				case *ast.SelectorExpr:
					obj = info.Uses[node.Sel]
				}
				constant, ok := obj.(*types.Const)
				if !ok {
					return true
				}
				named, ok := types.Unalias(constant.Type()).(*types.Named)
				if !ok {
					return true
				}
				candidate := "enum:" + canonicalTypeKey(named)
				if key != "" && key != candidate {
					conflict = true
				}
				key = candidate
				return true
			})
		}
	}
	return key, key != "" && !conflict
}

func discriminatorControl(fn *ast.FuncDecl, info *types.Info) bool {
	if fn == nil {
		return false
	}
	object, ok := info.Defs[fn.Name].(*types.Func)
	if !ok {
		return false
	}
	sig, ok := object.Type().(*types.Signature)
	if !ok {
		return false
	}
	for i := 0; i < sig.Results().Len(); i++ {
		if sealedInterface(sig.Results().At(i).Type()) {
			return true
		}
	}
	if sig.Results().Len() == 1 && types.Identical(sig.Results().At(0).Type(), types.Typ[types.Bool]) {
		literalOnly := true
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			ret, ok := n.(*ast.ReturnStmt)
			if !ok {
				return true
			}
			for _, expr := range ret.Results {
				id, ok := expr.(*ast.Ident)
				if !ok || id.Name != "true" && id.Name != "false" {
					literalOnly = false
				}
			}
			return true
		})
		if literalOnly {
			return true
		}
	}
	return isSerializationFunction(fn.Name.Name) || strings.HasPrefix(fn.Name.Name, "decode") || strings.HasPrefix(fn.Name.Name, "encode")
}
func enclosingDeclaration(functions []*ast.FuncDecl, pos ast.Node) *ast.FuncDecl {
	for _, fn := range functions {
		if fn.Pos() <= pos.Pos() && fn.End() >= pos.End() {
			return fn
		}
	}
	return nil
}

func concreteAssertionTarget(site *ocpDispatchSite) bool {
	if site.pkg.info == nil {
		return true
	}
	concrete := false
	ast.Inspect(site.node, func(n ast.Node) bool {
		assertion, ok := n.(*ast.TypeAssertExpr)
		if !ok || assertion.Type == nil {
			return true
		}
		typ := site.pkg.info.TypeOf(assertion.Type)
		if typ != nil && !isInterface(typ) {
			concrete = true
		}
		return true
	})
	return concrete
}

// A function may express one discriminator decision as several guard clauses.
// Correlate their variants before computing cross-function extension pressure.
func mergeEnumFunctionSites(sites []*ocpDiscriminatorSite) []*ocpDiscriminatorSite {
	out := make([]*ocpDiscriminatorSite, 0, len(sites))
	groups := map[string]*ocpDiscriminatorSite{}
	for _, site := range sites {
		if !strings.HasPrefix(site.fieldKey, "enum:") {
			out = append(out, site)
			continue
		}
		key := site.fieldKey + ";" + site.function
		if primary := groups[key]; primary != nil {
			primary.values = uniqueSorted(append(primary.values, site.values...))
			primary.defaultBad = primary.defaultBad || site.defaultBad
			continue
		}
		groups[key] = site
		out = append(out, site)
	}
	return out
}
