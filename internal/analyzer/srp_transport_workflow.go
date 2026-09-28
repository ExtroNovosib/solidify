package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

// checkTransportWorkflows measures owned domain and persistence operations in
// typed HTTP entry points. Receiver names and bag depth do not imply ownership.
func checkTransportWorkflows(fset *token.FileSet, files []*ast.File, info *types.Info, cfg Config, pkg *packageFiles) []Issue {
	if info == nil {
		return nil
	}
	var issues []Issue
	declarations := localFunctionDeclarations(files, info)
	for _, file := range files {
		if skipGenerated(pkg, file) {
			continue
		}
		for _, declaration := range file.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || fn.Body == nil || !httpEntryPoint(fn, info) {
				continue
			}
			domainOps := []string{}
			writes := []string{}
			related := []RelatedLocation{}
			visited := map[*ast.FuncDecl]bool{}
			var inspect func(*ast.FuncDecl)
			inspect = func(current *ast.FuncDecl) {
				if current == nil || current.Body == nil || visited[current] {
					return
				}
				visited[current] = true
				ast.Inspect(current.Body, func(n ast.Node) bool {
					if _, closure := n.(*ast.FuncLit); closure {
						return false
					}
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					callee := calledFunction(call.Fun, info)
					if callee == nil {
						return true
					}
					signature, _ := callee.Type().(*types.Signature)
					ownedDomain := false
					if callee.Pkg() != nil && domainOperationPackage(callee.Pkg(), cfg) {
						ownedDomain = strings.HasPrefix(callee.Name(), "New")
						if signature != nil && signature.Recv() != nil {
							ownedDomain = !isTrivialDomainAccessor(callee.Name()) && (signature.Params().Len() > 0 || domainMutationName(callee.Name()))
						}
					}
					if ownedDomain {
						domainOps = append(domainOps, callee.Name())
						related = append(related, RelatedLocation{Pos: fset.Position(call.Pos()), Message: "owned domain operation " + callee.Name()})
					}
					if member, ok := call.Fun.(*ast.SelectorExpr); ok && receiverFieldPath(member.X, current) && hasMethodPrefix(callee.Name(), "Create", "Insert", "Save", "Update", "Delete", "Set") {
						domainArgument := false
						for _, arg := range call.Args {
							if isDomainStructType(info.TypeOf(arg), cfg) {
								domainArgument = true
							}
						}
						if domainArgument {
							writes = append(writes, callee.Name())
							related = append(related, RelatedLocation{Pos: fset.Position(call.Pos()), Message: "persistence operation " + callee.Name()})
						}
					}
					if helper := declarations[callee]; helper != nil && !httpEntryPoint(helper, info) && helper.Recv != nil && fn.Recv != nil && receiverTypeName(helper.Recv.List[0].Type) == receiverTypeName(fn.Recv.List[0].Type) {
						inspect(helper)
					}
					return true
				})
			}
			inspect(fn)
			if len(domainOps) == 0 || len(writes) < 2 {
				continue
			}
			issues = append(issues, issueAt(fset, fn.Name, Issue{Rule: RuleSRP, Check: CheckSRPTransportWorkflow, Severity: SeverityWarning,
				Message:  fmt.Sprintf("HTTP method %s owns domain operations (%s) and ordered persistence (%s); move the workflow into an input-independent application command", fn.Name.Name, strings.Join(domainOps, ", "), strings.Join(writes, ", ")),
				Evidence: fmt.Sprintf("transport-workflow:method=%s;domain=%s;persistence=%s", fn.Name.Name, strings.Join(domainOps, ","), strings.Join(writes, ",")), Groups: []SymbolGroup{{Label: "domain-operations", Symbols: SortedSymbols(domainOps)}, {Label: "persistence-operations", Symbols: SortedSymbols(writes)}}, Related: related,
			}))
		}
	}
	return issues
}
func httpEntryPoint(fn *ast.FuncDecl, info *types.Info) bool {
	object, ok := info.Defs[fn.Name].(*types.Func)
	if !ok {
		return false
	}
	sig, ok := object.Type().(*types.Signature)
	if !ok {
		return false
	}
	request, writer := false, false
	for i := 0; i < sig.Params().Len(); i++ {
		typ := sig.Params().At(i).Type()
		if pointer, ok := typ.(*types.Pointer); ok {
			typ = pointer.Elem()
		}
		named, ok := typ.(*types.Named)
		if !ok || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != "net/http" {
			continue
		}
		request = request || named.Obj().Name() == "Request"
		writer = writer || named.Obj().Name() == "ResponseWriter"
	}
	return request && writer
}
func receiverFieldPath(expr ast.Expr, fn *ast.FuncDecl) bool {
	depth := 0
	for {
		member, ok := expr.(*ast.SelectorExpr)
		if !ok {
			break
		}
		depth++
		expr = member.X
	}
	id, ok := expr.(*ast.Ident)
	return ok && depth > 0 && containsString(receiverNames(fn), id.Name)
}
func isTrivialDomainAccessor(name string) bool {
	return hasMethodPrefix(name, "Get", "Is", "Has", "ID", "Name", "Status", "Created", "Updated", "Version", "Source", "Steps")
}

func domainOperationPackage(pkg *types.Package, cfg Config) bool {
	if pkg == nil {
		return false
	}
	if len(cfg.DIPDomainPackages) > 0 {
		return matchesAnyPackagePattern(pkg.Path(), cfg.DIPDomainPackages)
	}
	return pkg.Name() == "domain" || strings.HasSuffix(strings.TrimSuffix(pkg.Path(), "/"), "/domain")
}

func domainMutationName(name string) bool {
	for _, verb := range []string{"Publish", "Activate", "Deactivate", "Advance", "Execute", "Apply", "Attach", "Detach", "Delete", "Cancel", "Complete", "Start", "Stop", "Enable", "Disable", "Reset", "Increment", "Consume", "Revoke"} {
		if strings.HasPrefix(name, verb) && (len(name) == len(verb) || name[len(verb)] >= 'A' && name[len(verb)] <= 'Z') {
			return true
		}
	}
	return false
}
