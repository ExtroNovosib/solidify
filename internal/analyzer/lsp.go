package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"
)

// CheckLSP is retained for callers that only have syntax. LSP checks rely on
// resolved types and deliberately make no claim when type information is not
// available.
func CheckLSP(fset *token.FileSet, files []*ast.File, cfg Config) []Issue {
	return nil
}

// CheckLSPWithTypes performs package-local, contract-backed checks. It does
// not duplicate unsupported-operation detection: that remains an ISP concern
// because it identifies interfaces that force a type to implement an operation
// it does not support.
func CheckLSPWithTypes(fset *token.FileSet, files []*ast.File, info *types.Info, cfg Config, pkg *packageFiles) []Issue {
	if info == nil {
		return nil
	}
	var issues []Issue
	for _, f := range files {
		if skipGenerated(pkg, f) {
			continue
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !readerReadMethod(fn, info) {
				continue
			}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				if _, ok := node.(*ast.FuncLit); ok {
					return false
				}
				ret, ok := node.(*ast.ReturnStmt)
				if !ok || len(ret.Results) != 2 {
					return true
				}
				kind, ok := nonExactEOF(ret.Results[1], info)
				if !ok {
					return true
				}
				issues = append(issues, issueAt(fset, ret, Issue{
					Rule:     RuleLSP,
					Check:    CheckLSPNonExactEOF,
					Severity: SeverityWarning,
					Message:  "io.Reader-compatible Read reconstructs or wraps io.EOF; return io.EOF itself because callers may compare it using ==",
					Evidence: "non-exact-eof:method=Read;kind=" + kind,
				}))
				return true
			})
		}
	}
	return issues
}

// CheckLSPProgram performs checks that need the entire loaded workspace. It
// intentionally reports a possible nil embedded interface only when no
// non-nil initialization is visible anywhere in that workspace.
func CheckLSPProgram(pkgs []*packageFiles, cfg Config) []Issue {
	candidates := collectEmbeddedInterfaceCandidates(pkgs)
	if len(candidates) == 0 {
		return nil
	}
	initialized := initializedEmbeddedInterfaceFields(pkgs, candidates)
	issues := make([]Issue, 0, len(candidates))
	for _, candidate := range candidates {
		if initialized[candidate.field] {
			continue
		}
		issues = append(issues, issueSpan(candidate.pkg.fset, candidate.pos, candidate.end, Issue{
			Rule:     RuleLSP,
			Check:    CheckLSPNilEmbeddedInterface,
			Severity: SeverityNote,
			Message: fmt.Sprintf(
				"embedded interface %q in type %q supplies promoted methods but is never initialized in the analyzed workspace; calls through the zero value can panic",
				candidate.interfaceName, candidate.typeName,
			),
			Evidence: "nil-embedded-interface:type=" + candidate.typeName + ";field=" + candidate.field.Name() + ";methods=" + strings.Join(candidate.methods, ","),
			Groups:   []SymbolGroup{{Label: "promoted-methods", Symbols: append([]string(nil), candidate.methods...)}},
		}))
	}
	return issues
}

func readerReadMethod(fn *ast.FuncDecl, info *types.Info) bool {
	if fn == nil || fn.Name == nil || fn.Name.Name != "Read" || fn.Recv == nil || fn.Body == nil {
		return false
	}
	obj, ok := info.Defs[fn.Name].(*types.Func)
	if !ok {
		return false
	}
	sig, ok := obj.Type().(*types.Signature)
	if !ok || sig.Recv() == nil || sig.Params().Len() != 1 || sig.Results().Len() != 2 {
		return false
	}
	slice, ok := sig.Params().At(0).Type().Underlying().(*types.Slice)
	if !ok || !types.Identical(slice.Elem().Underlying(), types.Typ[types.Uint8]) {
		return false
	}
	if !types.Identical(sig.Results().At(0).Type(), types.Typ[types.Int]) {
		return false
	}
	errorType := types.Universe.Lookup("error").Type()
	return types.Identical(sig.Results().At(1).Type(), errorType)
}

func nonExactEOF(expr ast.Expr, info *types.Info) (string, bool) {
	expr = unparen(expr)
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return "", false
	}
	fn := calledFunction(call.Fun, info)
	if fn == nil || fn.Pkg() == nil {
		return "", false
	}
	pkg, name := fn.Pkg().Path(), fn.Name()
	if (pkg == fmtPackagePath && name == errorfFuncName) || (pkg == errorsPackagePath && name == joinFuncName) {
		if callMentionsIOEOF(call, info) {
			return "wrapped-eof", true
		}
	}
	if (pkg == errorsPackagePath && name == "New") || (pkg == fmtPackagePath && name == errorfFuncName) {
		if len(call.Args) > 0 && stringLiteralValue(call.Args[0]) == eofIdentifier {
			return "recreated-eof", true
		}
	}
	return "", false
}

func unparen(expr ast.Expr) ast.Expr {
	for {
		paren, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = paren.X
	}
}

func calledFunction(expr ast.Expr, info *types.Info) *types.Func {
	switch fn := expr.(type) {
	case *ast.Ident:
		result, _ := info.Uses[fn].(*types.Func)
		return result
	case *ast.SelectorExpr:
		result, _ := info.Uses[fn.Sel].(*types.Func)
		return result
	default:
		return nil
	}
}

func callMentionsIOEOF(call *ast.CallExpr, info *types.Info) bool {
	found := false
	ast.Inspect(call, func(node ast.Node) bool {
		if found {
			return false
		}
		selector, ok := node.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != eofIdentifier {
			return true
		}
		obj, ok := info.Uses[selector.Sel].(*types.Var)
		if ok && obj.Pkg() != nil && obj.Pkg().Path() == ioPackagePath && obj.Name() == eofIdentifier {
			found = true
		}
		return true
	})
	return found
}

func stringLiteralValue(expr ast.Expr) string {
	lit, ok := unparen(expr).(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return ""
	}
	return value
}

type embeddedInterfaceCandidate struct {
	pkg           *packageFiles
	named         *types.Named
	field         *types.Var
	fieldIndex    int
	pos           token.Pos
	end           token.Pos
	typeName      string
	interfaceName string
	methods       []string
}

func collectEmbeddedInterfaceCandidates(pkgs []*packageFiles) []*embeddedInterfaceCandidate {
	var candidates []*embeddedInterfaceCandidate
	for _, pkg := range pkgs {
		if pkg == nil || !pkg.typeComplete || pkg.info == nil {
			continue
		}
		for _, file := range pkg.files {
			if skipGenerated(pkg, file) {
				continue
			}
			for _, decl := range file.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.TYPE {
					continue
				}
				for _, spec := range gen.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					obj, ok := pkg.info.Defs[ts.Name].(*types.TypeName)
					if !ok {
						continue
					}
					named, ok := obj.Type().(*types.Named)
					if !ok {
						continue
					}
					strct, ok := named.Underlying().(*types.Struct)
					if !ok {
						continue
					}
					astStruct, _ := ts.Type.(*ast.StructType)
					for index := 0; index < strct.NumFields(); index++ {
						field := strct.Field(index)
						if !field.Embedded() {
							continue
						}
						iface, ok := field.Type().Underlying().(*types.Interface)
						if !ok {
							continue
						}
						methods := promotedInterfaceMethods(named, iface, index)
						if len(methods) == 0 {
							continue
						}
						end := field.Pos()
						if astStruct != nil && index < len(astStruct.Fields.List) {
							end = astStruct.Fields.List[index].End()
						}
						candidates = append(candidates, &embeddedInterfaceCandidate{
							pkg:           pkg,
							named:         named,
							field:         field,
							fieldIndex:    index,
							pos:           field.Pos(),
							end:           end,
							typeName:      named.Obj().Name(),
							interfaceName: typeDisplayName(field.Type()),
							methods:       methods,
						})
					}
				}
			}
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if left.pkg.fset.Position(left.pos).Filename != right.pkg.fset.Position(right.pos).Filename {
			return left.pkg.fset.Position(left.pos).Filename < right.pkg.fset.Position(right.pos).Filename
		}
		return left.pos < right.pos
	})
	return candidates
}

func promotedInterfaceMethods(named *types.Named, iface *types.Interface, fieldIndex int) []string {
	iface.Complete()
	methodSet := types.NewMethodSet(types.NewPointer(named))
	methods := make([]string, 0, iface.NumMethods())
	for index := 0; index < iface.NumMethods(); index++ {
		method := iface.Method(index)
		selection := methodSet.Lookup(method.Pkg(), method.Name())
		if selection == nil || len(selection.Index()) < 2 || selection.Index()[0] != fieldIndex {
			continue
		}
		methods = append(methods, method.Name())
	}
	sort.Strings(methods)
	return methods
}

func typeDisplayName(t types.Type) string {
	return types.TypeString(t, func(pkg *types.Package) string { return pkg.Name() })
}

func initializedEmbeddedInterfaceFields(pkgs []*packageFiles, candidates []*embeddedInterfaceCandidate) map[*types.Var]bool {
	fields := make(map[*types.Var]*embeddedInterfaceCandidate, len(candidates))
	for _, candidate := range candidates {
		fields[candidate.field] = candidate
	}
	initialized := make(map[*types.Var]bool, len(candidates))
	for _, pkg := range pkgs {
		if pkg == nil || pkg.info == nil {
			continue
		}
		for _, file := range pkg.files {
			ast.Inspect(file, func(node ast.Node) bool {
				switch current := node.(type) {
				case *ast.AssignStmt:
					for index, lhs := range current.Lhs {
						selector, ok := lhs.(*ast.SelectorExpr)
						if !ok || index >= len(current.Rhs) || isNilExpr(current.Rhs[index]) {
							continue
						}
						selection := pkg.info.Selections[selector]
						if selection == nil {
							continue
						}
						field, ok := selection.Obj().(*types.Var)
						if ok && fields[field] != nil {
							initialized[field] = true
						}
					}
				case *ast.CompositeLit:
					markCompositeLiteralInitialization(current, pkg.info, fields, initialized)
				}
				return true
			})
		}
	}
	return initialized
}

func markCompositeLiteralInitialization(lit *ast.CompositeLit, info *types.Info, fields map[*types.Var]*embeddedInterfaceCandidate, initialized map[*types.Var]bool) {
	named, ok := info.TypeOf(lit).(*types.Named)
	if !ok {
		return
	}
	strct, ok := named.Underlying().(*types.Struct)
	if !ok {
		return
	}
	for index, element := range lit.Elts {
		fieldIndex, value := index, element
		if keyed, ok := element.(*ast.KeyValueExpr); ok {
			name, ok := keyed.Key.(*ast.Ident)
			if !ok {
				continue
			}
			fieldIndex = structFieldIndex(strct, name.Name)
			value = keyed.Value
		}
		if fieldIndex < 0 || fieldIndex >= strct.NumFields() || isNilExpr(value) {
			continue
		}
		field := strct.Field(fieldIndex)
		if fields[field] != nil {
			initialized[field] = true
		}
	}
}

func structFieldIndex(strct *types.Struct, name string) int {
	for index := 0; index < strct.NumFields(); index++ {
		if strct.Field(index).Name() == name {
			return index
		}
	}
	return -1
}

func isNilExpr(expr ast.Expr) bool {
	ident, ok := unparen(expr).(*ast.Ident)
	return ok && ident.Name == "nil"
}
