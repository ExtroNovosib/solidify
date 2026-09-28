package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

// checkISPConstructorRoles follows constructor inputs into consumer fields.
// Unknown argument escapes prevent a narrowing recommendation.
func checkISPConstructorRoles(fset *token.FileSet, files []*ast.File, info *types.Info, pkg *packageFiles) []Issue {
	if info == nil {
		return nil
	}
	var issues []Issue
	for _, file := range files {
		if skipGenerated(pkg, file) {
			continue
		}
		for _, declaration := range file.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Recv != nil || (!strings.HasPrefix(fn.Name.Name, "New") && !strings.HasPrefix(fn.Name.Name, "Provide")) {
				continue
			}
			aliases := map[types.Object]types.Object{}
			ambiguous := map[types.Object]bool{}
			parameters := map[types.Object]*ast.Ident{}
			for _, parameter := range fn.Type.Params.List {
				for _, name := range parameter.Names {
					obj := info.Defs[name]
					aliases[obj] = obj
					parameters[obj] = name
				}
			}
			origin := func(expr ast.Expr) types.Object {
				if index, ok := expr.(*ast.IndexExpr); ok {
					expr = index.X
				}
				obj := expressionObject(expr, info)
				if ambiguous[obj] {
					return nil
				}
				return aliases[obj]
			}
			// Iterate to a fixed point to preserve aliases regardless of AST nesting.
			for changed := true; changed; {
				changed = false
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					switch statement := n.(type) {
					case *ast.AssignStmt:
						for i, lhs := range statement.Lhs {
							if i >= len(statement.Rhs) {
								break
							}
							root := origin(statement.Rhs[i])
							obj := expressionObject(lhs, info)
							if root != nil && obj != nil && !ambiguous[obj] && aliases[obj] != root {
								if previous := aliases[obj]; previous != nil {
									ambiguous[obj] = true
									changed = true
									continue
								}
								aliases[obj] = root
								changed = true
							}
						}
					case *ast.ValueSpec:
						for i, name := range statement.Names {
							if i >= len(statement.Values) {
								break
							}
							root := origin(statement.Values[i])
							if root != nil && !ambiguous[info.Defs[name]] && aliases[info.Defs[name]] != root {
								if previous := aliases[info.Defs[name]]; previous != nil {
									ambiguous[info.Defs[name]] = true
									changed = true
									continue
								}
								aliases[info.Defs[name]] = root
								changed = true
							}
						}
					}
					return true
				})
			}
			// Reassignment to an unknown value invalidates a prior constructor origin.
			for changed := true; changed; {
				changed = false
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					assignment, ok := n.(*ast.AssignStmt)
					if !ok {
						return true
					}
					for i, lhs := range assignment.Lhs {
						if i >= len(assignment.Rhs) {
							break
						}
						obj := expressionObject(lhs, info)
						if obj == nil || ambiguous[obj] || aliases[obj] == nil {
							continue
						}
						rhs := assignment.Rhs[i]
						if index, ok := rhs.(*ast.IndexExpr); ok {
							rhs = index.X
						}
						source := expressionObject(rhs, info)
						if source == nil || ambiguous[source] {
							ambiguous[obj] = true
							changed = true
						}
					}
					return true
				})
			}
			used := map[types.Object]map[string]bool{}
			escaped := map[types.Object]bool{}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if member, ok := call.Fun.(*ast.SelectorExpr); ok {
					if root := origin(member.X); root != nil {
						if used[root] == nil {
							used[root] = map[string]bool{}
						}
						used[root][member.Sel.Name] = true
					}
				}
				if builtin, ok := calledBuiltin(call.Fun, info); ok && (builtin.Name() == "len" || builtin.Name() == "cap") {
					return true
				}
				for _, arg := range call.Args {
					if root := origin(arg); root != nil {
						escaped[root] = true
					}
				}
				return true
			})
			reported := map[types.Object]bool{}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				literal, ok := n.(*ast.CompositeLit)
				if !ok {
					return true
				}
				typ := info.TypeOf(literal)
				if typ == nil {
					return true
				}
				structure, ok := typ.Underlying().(*types.Struct)
				if !ok {
					return true
				}
				for index, element := range literal.Elts {
					value := element
					var field *types.Var
					if pair, ok := element.(*ast.KeyValueExpr); ok {
						value = pair.Value
						id, ok := pair.Key.(*ast.Ident)
						if !ok {
							continue
						}
						for i := 0; i < structure.NumFields(); i++ {
							if structure.Field(i).Name() == id.Name {
								field = structure.Field(i)
								break
							}
						}
					} else if index < structure.NumFields() {
						field = structure.Field(index)
					}
					root := origin(value)
					if root == nil || field == nil || reported[root] || escaped[root] {
						continue
					}
					parameter := root.Type()
					if slice, ok := parameter.(*types.Slice); ok {
						parameter = slice.Elem()
					}
					broad, ok := parameter.Underlying().(*types.Interface)
					if !ok {
						continue
					}
					narrow, ok := field.Type().Underlying().(*types.Interface)
					if !ok {
						continue
					}
					broad.Complete()
					narrow.Complete()
					if broad.NumMethods() <= narrow.NumMethods() || !types.Implements(parameter, narrow) {
						continue
					}
					extraUsed := false
					for method := range used[root] {
						if !interfaceHasMethod(narrow, method) {
							extraUsed = true
						}
					}
					if extraUsed {
						continue
					}
					broadMethods := interfaceMethods(broad)
					narrowMethods := interfaceMethods(narrow)
					issues = append(issues, issueAt(fset, parameters[root], Issue{Rule: RuleISP, Check: CheckISPConstructorRole, Severity: SeverityWarning,
						Message:  fmt.Sprintf("constructor %s accepts %s with %d methods but stores only %s with %d methods; accept the consumer interface at this boundary", fn.Name.Name, root.Name(), broad.NumMethods(), field.Name(), narrow.NumMethods()),
						Evidence: fmt.Sprintf("constructor-role:function=%s;parameter=%s;field=%s;broad=%s;narrow=%s", fn.Name.Name, root.Name(), field.Name(), strings.Join(broadMethods, ","), strings.Join(narrowMethods, ",")),
						Groups:   []SymbolGroup{{Label: "required-methods", Symbols: broadMethods}, {Label: "stored-methods", Symbols: narrowMethods}}, Related: []RelatedLocation{{Pos: fset.Position(field.Pos()), Message: "narrow consumer field"}},
					}))
					reported[root] = true
				}
				return true
			})
		}
	}
	return issues
}
func interfaceMethods(iface *types.Interface) []string {
	names := make([]string, 0, iface.NumMethods())
	for i := 0; i < iface.NumMethods(); i++ {
		names = append(names, iface.Method(i).Name())
	}
	return SortedSymbols(names)
}
func interfaceHasMethod(iface *types.Interface, name string) bool {
	for i := 0; i < iface.NumMethods(); i++ {
		if iface.Method(i).Name() == name {
			return true
		}
	}
	return false
}
