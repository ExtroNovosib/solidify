package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
)

// checkLSPStreamContracts identifies statically proven violations of standard
// stream contracts, rather than assuming every record API is a byte stream.
func checkLSPStreamContracts(fset *token.FileSet, files []*ast.File, info *types.Info, pkg *packageFiles) []Issue {
	var issues []Issue
	for _, file := range files {
		if skipGenerated(pkg, file) {
			continue
		}
		for _, declaration := range file.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Body == nil {
				continue
			}
			object, ok := info.Defs[fn.Name].(*types.Func)
			if !ok {
				continue
			}
			signature, ok := object.Type().(*types.Signature)
			if !ok {
				continue
			}
			if readerReadMethod(fn, info) && exposedStandardStream(signature.Recv().Type(), files, info) {
				if copyNode, branch, source, ok := discardedReadFlow(readerFlowFunction(fn, files, info), files, info); ok {
					issues = append(issues, issueAt(fset, fn.Name, Issue{Rule: RuleLSP, Check: CheckLSPDiscardedRead, Severity: SeverityWarning,
						Message:  "stream Read consumes a record and discards its unread suffix when the caller supplies a short buffer; retain the suffix for subsequent reads",
						Evidence: fmt.Sprintf("discarded-read:type=%s;method=Read;record=%s;contract=io.Reader;remainder=discarded", receiverTypeName(fn.Recv.List[0].Type), source),
						Related:  []RelatedLocation{{Pos: fset.Position(copyNode.Pos()), Message: "prefix copied into caller buffer"}, {Pos: fset.Position(branch.Pos()), Message: "short-buffer return loses unread suffix"}},
					}))
				}
			}
			if standardDeadline(signature, fn.Name.Name, info) && successfulIgnoredParameter(fn, signature, info) {
				issues = append(issues, issueAt(fset, fn.Name, Issue{Rule: RuleLSP, Check: CheckLSPNoopDeadline, Severity: SeverityWarning,
					Message:  "net.Conn deadline setter ignores its timestamp and reports success; delegate or apply the deadline",
					Evidence: "noop-deadline:type=" + receiverTypeName(fn.Recv.List[0].Type) + ";method=" + fn.Name.Name + ";contract=net.Conn;timestamp=ignored;result=nil",
				}))
			}
		}
	}
	return issues
}

func exposedStandardStream(receiver types.Type, files []*ast.File, info *types.Info) bool {
	found := false
	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			fn, ok := node.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				return true
			}
			object, ok := info.Defs[fn.Name].(*types.Func)
			if !ok {
				return false
			}
			sig, ok := object.Type().(*types.Signature)
			if !ok {
				return false
			}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				if _, closure := node.(*ast.FuncLit); closure {
					return false
				}
				ret, ok := node.(*ast.ReturnStmt)
				if !ok {
					return true
				}
				for i, expr := range ret.Results {
					if i >= sig.Results().Len() {
						break
					}
					target := sig.Results().At(i).Type()
					named, ok := types.Unalias(target).(*types.Named)
					if !ok || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != "io" {
						continue
					}
					name := named.Obj().Name()
					if name != "Reader" && name != "ReadCloser" && name != "ReadWriter" && name != "ReadWriteCloser" {
						continue
					}
					if types.Identical(info.TypeOf(expr), receiver) {
						found = true
					}
				}
				return true
			})
			return false
		})
	}
	return found
}

func standardDeadline(sig *types.Signature, name string, info *types.Info) bool {
	if name != "SetDeadline" && name != "SetReadDeadline" && name != "SetWriteDeadline" {
		return false
	}
	if sig.Params().Len() != 1 || sig.Results().Len() != 1 || !types.Identical(sig.Results().At(0).Type(), types.Universe.Lookup("error").Type()) {
		return false
	}
	named, ok := types.Unalias(sig.Params().At(0).Type()).(*types.Named)
	if !ok || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != "time" || named.Obj().Name() != "Time" {
		return false
	}
	for _, obj := range info.Uses {
		tn, ok := obj.(*types.TypeName)
		if !ok || tn.Pkg() == nil || tn.Pkg().Path() != "net" || tn.Name() != "Conn" {
			continue
		}
		iface, ok := tn.Type().Underlying().(*types.Interface)
		if ok && types.Implements(sig.Recv().Type(), iface) {
			return true
		}
	}
	return false
}

func successfulIgnoredParameter(fn *ast.FuncDecl, sig *types.Signature, info *types.Info) bool {
	if fn.Body == nil || len(fn.Body.List) != 1 {
		return false
	}
	ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return false
	}
	id, ok := ret.Results[0].(*ast.Ident)
	if !ok || id.Name != "nil" {
		return false
	}
	used := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && info.Uses[id] == sig.Params().At(0) {
			used = true
		}
		return true
	})
	return !used
}

func expressionObject(expr ast.Expr, info *types.Info) types.Object {
	id, ok := unparen(expr).(*ast.Ident)
	if !ok {
		return nil
	}
	if obj := info.Uses[id]; obj != nil {
		return obj
	}
	return info.Defs[id]
}

func discardedReadFlow(fn *ast.FuncDecl, files []*ast.File, info *types.Info) (*ast.CallExpr, *ast.IfStmt, string, bool) {
	sig := info.Defs[fn.Name].Type().(*types.Signature)
	buffer := sig.Params().At(0)
	var copyNode *ast.CallExpr
	var count, record types.Object
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok || len(assignment.Rhs) != 1 || len(assignment.Lhs) != 1 {
			return true
		}
		call, ok := assignment.Rhs[0].(*ast.CallExpr)
		if !ok || len(call.Args) != 2 {
			return true
		}
		builtin, ok := calledBuiltin(call.Fun, info)
		if !ok || builtin.Name() != "copy" || expressionObject(call.Args[0], info) != buffer {
			return true
		}
		source := expressionObject(call.Args[1], info)
		if source == nil {
			return true
		}
		v, ok := source.(*types.Var)
		if !ok || v.IsField() {
			return true
		}
		copyNode = call
		count = expressionObject(assignment.Lhs[0], info)
		record = source
		return true
	})
	if copyNode == nil || count == nil {
		return nil, nil, "", false
	}
	var branch *ast.IfStmt
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		condition, ok := node.(*ast.IfStmt)
		if !ok {
			return true
		}
		comparison, ok := condition.Cond.(*ast.BinaryExpr)
		if !ok || comparison.Op != token.LSS || expressionObject(comparison.X, info) != count {
			return true
		}
		length, ok := comparison.Y.(*ast.CallExpr)
		if !ok || len(length.Args) != 1 || expressionObject(length.Args[0], info) != record {
			return true
		}
		b, ok := calledBuiltin(length.Fun, info)
		if !ok || b.Name() != "len" {
			return true
		}
		if len(condition.Body.List) != 1 {
			return true
		}
		ret, ok := condition.Body.List[0].(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 2 || expressionObject(ret.Results[0], info) != count {
			return true
		}
		selector, ok := ret.Results[1].(*ast.SelectorExpr)
		if !ok {
			return true
		}
		obj := info.Uses[selector.Sel]
		if obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == "io" && obj.Name() == "ErrShortBuffer" {
			branch = condition
		}
		return true
	})
	// The record must have been populated by a consuming/decoding call. A
	// caller-owned slice or cached field alone does not prove lost stream data.
	records := consumedRecordObjects(fn, localFunctionDeclarations(files, info), info, map[*ast.FuncDecl]bool{})
	consumed := records[record]
	retained := false
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, rhs := range assignment.Rhs {
			ast.Inspect(rhs, func(n ast.Node) bool {
				slice, ok := n.(*ast.SliceExpr)
				if ok && expressionObject(slice.X, info) == record && expressionObject(slice.Low, info) == count && slice.High == nil && branch != nil && assignment.Pos() < branch.Pos() {
					for _, lhs := range assignment.Lhs {
						if _, stored := lhs.(*ast.SelectorExpr); stored {
							retained = true
						}
					}
				}
				return true
			})
		}
		return true
	})
	return copyNode, branch, record.Name(), branch != nil && consumed && !retained
}

func calledBuiltin(expr ast.Expr, info *types.Info) (*types.Builtin, bool) {
	id, ok := expr.(*ast.Ident)
	if !ok {
		return nil, false
	}
	b, ok := info.Uses[id].(*types.Builtin)
	return b, ok
}

// readerFlowFunction follows an unchanged local forwarding helper. Unknown
// calls do not become proof merely because they share a method name.
func readerFlowFunction(fn *ast.FuncDecl, files []*ast.File, info *types.Info) *ast.FuncDecl {
	locals := localFunctionDeclarations(files, info)
	seen := map[*ast.FuncDecl]bool{}
	for !seen[fn] {
		seen[fn] = true
		if fn.Body == nil || len(fn.Body.List) != 1 {
			return fn
		}
		ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			return fn
		}
		call, ok := ret.Results[0].(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return fn
		}
		callee := locals[calledFunction(call.Fun, info)]
		if callee == nil || callee.Body == nil {
			return fn
		}
		sig := info.Defs[fn.Name].Type().(*types.Signature)
		if sig.Params().Len() != 1 || expressionObject(call.Args[0], info) != sig.Params().At(0) {
			return fn
		}
		fn = callee
	}
	return fn
}

func consumedRecordObjects(fn *ast.FuncDecl, locals map[*types.Func]*ast.FuncDecl, info *types.Info, visiting map[*ast.FuncDecl]bool) map[types.Object]bool {
	records := map[types.Object]bool{}
	if fn == nil || fn.Body == nil || visiting[fn] {
		return records
	}
	visiting[fn] = true
	defer delete(visiting, fn)
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		callee := calledFunction(call.Fun, info)
		if callee == nil {
			return true
		}
		index := -1
		if callee.Pkg() != nil && callee.Pkg().Path() == "io" && (callee.Name() == "ReadFull" || callee.Name() == "ReadAtLeast") {
			index = 1
		} else if callee.Name() == "Read" {
			index = 0
		}
		if index >= 0 && index < len(call.Args) {
			if obj := expressionObject(call.Args[index], info); obj != nil {
				records[obj] = true
			}
		}
		return true
	})
	for changed := true; changed; {
		changed = false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			assignment, ok := n.(*ast.AssignStmt)
			if !ok || len(assignment.Rhs) != 1 || len(assignment.Lhs) == 0 {
				return true
			}
			call, ok := assignment.Rhs[0].(*ast.CallExpr)
			if !ok {
				return true
			}
			if _, builtin := calledBuiltin(call.Fun, info); builtin {
				return true
			}
			produces := false
			for _, arg := range call.Args {
				produces = produces || records[expressionObject(arg, info)]
			}
			if helper := locals[calledFunction(call.Fun, info)]; helper != nil && helper.Body != nil {
				inner := consumedRecordObjects(helper, locals, info, visiting)
				ast.Inspect(helper.Body, func(n ast.Node) bool {
					ret, ok := n.(*ast.ReturnStmt)
					if ok && len(ret.Results) > 0 && inner[expressionObject(ret.Results[0], info)] {
						produces = true
					}
					return true
				})
			}
			obj := expressionObject(assignment.Lhs[0], info)
			if obj != nil && produces && !records[obj] {
				records[obj] = true
				changed = true
			}
			return true
		})
	}
	return records
}
