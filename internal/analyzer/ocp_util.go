package analyzer

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/constant"
	"go/printer"
	"go/token"
	"go/types"
	"path"
	"sort"
	"strconv"
	"strings"
)

// The helpers below are intentionally kept small and deterministic. They are
// also used by the syntax-only compatibility tests.
func countCaseClauses(body *ast.BlockStmt) int {
	n := 0
	for _, stmt := range body.List {
		if cc, ok := stmt.(*ast.CaseClause); ok && cc.List != nil {
			n += len(cc.List)
		}
	}
	return n
}

func isTypeAssertionIf(stmt *ast.IfStmt) bool {
	if stmt == nil {
		return false
	}
	if assign, ok := stmt.Init.(*ast.AssignStmt); ok {
		for _, rhs := range assign.Rhs {
			if _, ok := rhs.(*ast.TypeAssertExpr); ok {
				return true
			}
		}
	}
	return containsTypeAssert(stmt.Cond)
}

func containsTypeAssert(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.TypeAssertExpr:
		return true
	case *ast.BinaryExpr:
		return containsTypeAssert(e.X) || containsTypeAssert(e.Y)
	default:
		return false
	}
}

func typeAssertChain(stmt *ast.IfStmt) (count int, links []*ast.IfStmt) {
	operand, ok := typeAssertionOperand(stmt)
	if !ok {
		return 0, nil
	}
	key := expressionKey(operand)
	for current := stmt; current != nil; {
		currentOperand, isAssertion := typeAssertionOperand(current)
		if !isAssertion || expressionKey(currentOperand) != key {
			break
		}
		links = append(links, current)
		count++
		next, ok := current.Else.(*ast.IfStmt)
		if !ok {
			break
		}
		current = next
	}
	return count, links
}

func typeAssertionOperand(stmt *ast.IfStmt) (ast.Expr, bool) {
	if stmt == nil {
		return nil, false
	}
	if assign, ok := stmt.Init.(*ast.AssignStmt); ok {
		for _, rhs := range assign.Rhs {
			if assertion, ok := rhs.(*ast.TypeAssertExpr); ok {
				return assertion.X, true
			}
		}
	}
	return typeAssertionOperandInExpr(stmt.Cond)
}

func typeAssertionOperandInExpr(expr ast.Expr) (ast.Expr, bool) {
	switch e := expr.(type) {
	case *ast.TypeAssertExpr:
		return e.X, true
	case *ast.BinaryExpr:
		if operand, ok := typeAssertionOperandInExpr(e.X); ok {
			return operand, true
		}
		return typeAssertionOperandInExpr(e.Y)
	default:
		return nil, false
	}
}

func expressionKey(expr ast.Expr) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, token.NewFileSet(), expr); err != nil {
		return ""
	}
	return buf.String()
}

func typeSwitchSource(stmt *ast.TypeSwitchStmt) ast.Expr {
	switch assign := stmt.Assign.(type) {
	case *ast.AssignStmt:
		for _, rhs := range assign.Rhs {
			if assertion, ok := rhs.(*ast.TypeAssertExpr); ok {
				return assertion.X
			}
		}
	case *ast.ExprStmt:
		if assertion, ok := assign.X.(*ast.TypeAssertExpr); ok {
			return assertion.X
		}
	}
	return nil
}

func typeSwitchVariants(stmt *ast.TypeSwitchStmt, info *types.Info) []string {
	var variants []string
	for _, raw := range stmt.Body.List {
		cc, ok := raw.(*ast.CaseClause)
		if !ok || cc.List == nil {
			continue
		}
		for _, expr := range cc.List {
			variants = append(variants, typeExpressionKey(expr, info))
		}
	}
	return variants
}

func typeAssertChainTargets(stmt *ast.IfStmt, info *types.Info) []string {
	var out []string
	if assign, ok := stmt.Init.(*ast.AssignStmt); ok {
		for _, rhs := range assign.Rhs {
			if assertion, ok := rhs.(*ast.TypeAssertExpr); ok && assertion.Type != nil {
				out = append(out, typeExpressionKey(assertion.Type, info))
			}
		}
	}
	if len(out) == 0 {
		out = append(out, typeAssertTypesInExpr(stmt.Cond, info)...)
	}
	return out
}

func typeAssertTypesInExpr(expr ast.Expr, info *types.Info) []string {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case *ast.TypeAssertExpr:
		if e.Type == nil {
			return nil
		}
		return []string{typeExpressionKey(e.Type, info)}
	case *ast.BinaryExpr:
		var out []string
		out = append(out, typeAssertTypesInExpr(e.X, info)...)
		out = append(out, typeAssertTypesInExpr(e.Y, info)...)
		return out
	default:
		return nil
	}
}

func markTypeAssertions(stmt *ast.IfStmt, covered map[*ast.TypeAssertExpr]bool) {
	ast.Inspect(stmt, func(node ast.Node) bool {
		if assertion, ok := node.(*ast.TypeAssertExpr); ok {
			covered[assertion] = true
		}
		return true
	})
}

func typeExpressionKey(expr ast.Expr, info *types.Info) string {
	if info != nil {
		if typ := info.TypeOf(expr); typ != nil {
			return canonicalTypeKey(typ)
		}
	}
	return expressionKey(expr)
}

func expressionType(info *types.Info, expr ast.Expr) types.Type {
	if info == nil {
		return nil
	}
	return info.TypeOf(expr)
}

func dispatchSourceKey(pkg *packageFiles, file *ast.File, source types.Type, expr ast.Expr) string {
	if source != nil {
		return canonicalTypeKey(source)
	}
	ident, ok := unparenOCP(expr).(*ast.Ident)
	if !ok {
		return syntaxBindingKey(pkg, expr.Pos())
	}
	if explicit := explicitSyntaxType(ident); explicit != nil {
		return "syntax-type:" + syntaxTypeKey(file, explicit)
	}
	if ident.Obj != nil {
		return syntaxBindingKey(pkg, ident.Obj.Pos())
	}
	return syntaxBindingKey(pkg, ident.Pos())
}

func unparenOCP(expr ast.Expr) ast.Expr {
	for {
		paren, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = paren.X
	}
}

func explicitSyntaxType(ident *ast.Ident) ast.Expr {
	if ident == nil || ident.Obj == nil {
		return nil
	}
	switch decl := ident.Obj.Decl.(type) {
	case *ast.Field:
		return decl.Type
	case *ast.ValueSpec:
		return decl.Type
	}
	return nil
}

func syntaxTypeKey(file *ast.File, expr ast.Expr) string {
	selector, ok := unparenOCP(expr).(*ast.SelectorExpr)
	if !ok {
		if pointer, isPointer := unparenOCP(expr).(*ast.StarExpr); isPointer {
			return "*" + syntaxTypeKey(file, pointer.X)
		}
		return expressionKey(expr)
	}
	alias, ok := selector.X.(*ast.Ident)
	if !ok {
		return expressionKey(expr)
	}
	for _, spec := range file.Imports {
		path := strings.Trim(spec.Path.Value, "\"")
		name := ""
		if spec.Name != nil {
			name = spec.Name.Name
		} else if slash := strings.LastIndex(path, "/"); slash >= 0 {
			name = path[slash+1:]
		} else {
			name = path
		}
		if name == alias.Name {
			return path + "." + selector.Sel.Name
		}
	}
	return expressionKey(expr)
}

func syntaxBindingKey(pkg *packageFiles, pos token.Pos) string {
	if pkg == nil || pkg.fset == nil {
		return fmt.Sprintf("syntax-binding:%d", pos)
	}
	position := pkg.fset.Position(pos)
	return fmt.Sprintf("syntax-binding:%s:%d:%d", PortablePath(pkg.analysisRoot, position.Filename), position.Line, position.Column)
}

func concreteTypeCandidate(typ types.Type) bool {
	if typ == nil || isInterface(typ) {
		return false
	}
	if isStdlibConcreteType(typ) {
		return false
	}
	if pointer, ok := types.Unalias(typ).(*types.Pointer); ok {
		typ = pointer.Elem()
	}
	_, ok := types.Unalias(typ).(*types.Named)
	return ok
}

func isInterface(typ types.Type) bool {
	return typ != nil && func() bool { _, ok := types.Unalias(typ).Underlying().(*types.Interface); return ok }()
}

func dispatchTypeAllowed(key string, cfg Config) bool {
	// AST/type-model visitors are intentionally closed over the standard
	// library's node set. Treat those external visitor patterns as an explicit
	// safe default; project-specific visitors remain configurable below.
	for _, prefix := range []string{"go/", "encoding/", "encoding.", "reflect."} {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	for _, allowed := range cfg.OCPAllowDispatchTypes {
		if key == allowed || strings.HasSuffix(key, "."+allowed) {
			return true
		}
	}
	return false
}

func ocpFileEnabled(pkg *packageFiles, file *ast.File, cfg Config) bool {
	if skipGenerated(pkg, file) {
		return false
	}
	if len(cfg.ExcludedFiles) > 0 && Excluded(pkg.fset.Position(file.Pos()).Filename, cfg.ExcludedFiles) {
		return false
	}
	return !matchesAnyPackagePattern(pkg.pkgPath, cfg.OCPAllowPackages)
}

func functionDecls(file *ast.File) []*ast.FuncDecl {
	var functions []*ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			functions = append(functions, fn)
		}
	}
	return functions
}

func enclosingFunction(functions []*ast.FuncDecl, pos token.Pos, pkgPath string) string {
	for _, fn := range functions {
		if fn.Pos() <= pos && pos <= fn.End() {
			return pkgPath + ":" + fn.Name.Name
		}
	}
	return pkgPath + ":<file>"
}

func enclosingFunctionName(functions []*ast.FuncDecl, pos token.Pos) string {
	for _, fn := range functions {
		if fn.Pos() <= pos && pos <= fn.End() {
			return fn.Name.Name
		}
	}
	return ""
}

func typeSwitchHasUnsupportedDefault(stmt *ast.TypeSwitchStmt) bool {
	for _, raw := range stmt.Body.List {
		cc, ok := raw.(*ast.CaseClause)
		if ok && cc.List == nil && clauseHasUnsupportedReturn(cc.Body) {
			return true
		}
	}
	return false
}

func clauseHasUnsupportedReturn(body []ast.Stmt) bool {
	for _, stmt := range body {
		found := false
		ast.Inspect(stmt, func(node ast.Node) bool {
			switch current := node.(type) {
			case *ast.CallExpr:
				if ident, ok := current.Fun.(*ast.Ident); ok && ident.Name == "panic" {
					found = true
				}
				if selector, ok := current.Fun.(*ast.SelectorExpr); ok && (selector.Sel.Name == "Errorf" || selector.Sel.Name == "New") && callHasUnsupportedText(current) {
					found = true
				}
			case *ast.ReturnStmt:
				if callHasUnsupportedTextInExprs(current.Results) {
					found = true
				}
			}
			return !found
		})
		if found {
			return true
		}
	}
	return false
}

func callHasUnsupportedText(call *ast.CallExpr) bool {
	if len(call.Args) == 0 {
		return false
	}
	return unsupportedText(expressionKey(call.Args[0]))
}

func callHasUnsupportedTextInExprs(exprs []ast.Expr) bool {
	for _, expr := range exprs {
		if unsupportedText(expressionKey(expr)) {
			return true
		}
		found := false
		ast.Inspect(expr, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if ok && callHasUnsupportedText(call) {
				found = true
				return false
			}
			return true
		})
		if found {
			return true
		}
	}
	return false
}

func unsupportedText(text string) bool {
	text = strings.ToLower(text)
	for _, word := range []string{"unknown", "unsupported", "unhandled", "unexpected", "invalid kind", "invalid type", "unknown variant"} {
		if strings.Contains(text, word) {
			return true
		}
	}
	return false
}

func isSerializationFunction(name string) bool {
	for _, prefix := range []string{"Marshal", "Unmarshal", "Encode", "Decode", "Scan", "Value"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func sealedInterface(typ types.Type) bool {
	if typ == nil {
		return false
	}
	named, ok := types.Unalias(typ).(*types.Named)
	if !ok {
		return false
	}
	iface, ok := named.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	for i := 0; i < iface.NumMethods(); i++ {
		if !iface.Method(i).Exported() {
			return true
		}
	}
	return false
}

func displaySourceType(site *ocpDispatchSite) string {
	if site.sourceKey != "" {
		return site.sourceKey
	}
	return "interface value"
}

func dispatchComponents(sites []*ocpDispatchSite, cfg Config) [][]*ocpDispatchSite {
	parent := make([]int, len(sites))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}
		return parent[i]
	}
	union := func(a, b int) {
		a, b = find(a), find(b)
		if a != b {
			parent[b] = a
		}
	}
	for i := range sites {
		for j := i + 1; j < len(sites); j++ {
			shared := intersection(sites[i].variants, sites[j].variants)
			standaloneAssertions := sites[i].kind == ocpKindTypeAssertion && sites[j].kind == ocpKindTypeAssertion
			if standaloneAssertions || (len(shared) >= cfg.OCPMinSharedVariants && overlapPercent(sites[i].variants, sites[j].variants) >= cfg.OCPDispatchOverlapPercent) {
				union(i, j)
			}
		}
	}
	components := map[int][]*ocpDispatchSite{}
	for i, site := range sites {
		root := find(i)
		components[root] = append(components[root], site)
	}
	out := make([][]*ocpDispatchSite, 0, len(components))
	for _, component := range components {
		sortDispatchSites(component)
		out = append(out, component)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i][0].pos.Filename+fmt.Sprint(out[i][0].pos.Line) < out[j][0].pos.Filename+fmt.Sprint(out[j][0].pos.Line)
	})
	return out
}

func overlapPercent(a, b []string) int {
	shared := len(intersection(a, b))
	union := len(uniqueSorted(append(append([]string(nil), a...), b...)))
	if union == 0 {
		return 0
	}
	return 100 * shared / union
}

func discriminatorComponents(sites []*ocpDiscriminatorSite, minShared int) [][]*ocpDiscriminatorSite {
	parent := make([]int, len(sites))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}
		return parent[i]
	}
	for i := range sites {
		for j := i + 1; j < len(sites); j++ {
			if len(intersection(sites[i].values, sites[j].values)) >= minShared {
				a, b := find(i), find(j)
				if a != b {
					parent[b] = a
				}
			}
		}
	}
	components := map[int][]*ocpDiscriminatorSite{}
	for i, site := range sites {
		components[find(i)] = append(components[find(i)], site)
	}
	out := make([][]*ocpDiscriminatorSite, 0, len(components))
	for _, component := range components {
		sortDiscriminatorSites(component)
		out = append(out, component)
	}
	return out
}

func frequentVariants(sites []*ocpDispatchSite, minimum int) []string {
	counts := map[string]int{}
	for _, site := range sites {
		for _, variant := range uniqueSorted(site.variants) {
			counts[variant]++
		}
	}
	var out []string
	for variant, count := range counts {
		if count >= minimum {
			out = append(out, variant)
		}
	}
	sort.Strings(out)
	return out
}

func frequentDiscriminatorValues(sites []*ocpDiscriminatorSite, minimum int) []string {
	counts := map[string]int{}
	for _, site := range sites {
		for _, value := range uniqueSorted(site.values) {
			counts[value]++
		}
	}
	var out []string
	for value, count := range counts {
		if count >= minimum {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func unionVariants(sites []*ocpDispatchSite) []string {
	var values []string
	for _, site := range sites {
		values = append(values, site.variants...)
	}
	return uniqueSorted(values)
}
func intersection(a, b []string) []string {
	set := map[string]bool{}
	for _, x := range a {
		set[x] = true
	}
	var out []string
	for _, x := range b {
		if set[x] {
			out = append(out, x)
		}
	}
	return uniqueSorted(out)
}
func uniqueSorted(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		if value != "" {
			set[value] = true
		}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
func sortDispatchSites(sites []*ocpDispatchSite) {
	sort.Slice(sites, func(i, j int) bool { return positionKey(sites[i].pos) < positionKey(sites[j].pos) })
}
func sortDiscriminatorSites(sites []*ocpDiscriminatorSite) {
	sort.Slice(sites, func(i, j int) bool { return positionKey(sites[i].pos) < positionKey(sites[j].pos) })
}
func positionKey(pos token.Position) string {
	return fmt.Sprintf("%s:%08d:%08d", pos.Filename, pos.Line, pos.Column)
}
func relatedLocations(sites []*ocpDispatchSite, message string) []RelatedLocation {
	out := make([]RelatedLocation, 0, len(sites))
	for _, site := range sites {
		out = append(out, RelatedLocation{Pos: site.pos, Message: message})
	}
	return out
}
func relatedDiscriminatorLocations(sites []*ocpDiscriminatorSite) []RelatedLocation {
	out := make([]RelatedLocation, 0, len(sites))
	for _, site := range sites {
		out = append(out, RelatedLocation{Pos: site.pos, Message: "same discriminator family"})
	}
	return out
}

func discriminatorFieldKey(expr ast.Expr, info *types.Info) (string, bool) {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || info == nil {
		return "", false
	}
	selection := info.Selections[sel]
	if selection == nil || selection.Kind() != types.FieldVal {
		return "", false
	}
	return canonicalTypeKey(info.TypeOf(sel.X)) + "." + sel.Sel.Name, true
}

func discriminatorSwitchValues(stmt *ast.SwitchStmt, info *types.Info) ([]string, bool) {
	var values []string
	bad := false
	for _, raw := range stmt.Body.List {
		cc, ok := raw.(*ast.CaseClause)
		if !ok {
			continue
		}
		if cc.List == nil {
			bad = clauseHasUnsupportedReturn(cc.Body)
			continue
		}
		for _, expr := range cc.List {
			values = append(values, valueExpressionKey(expr, info))
		}
	}
	return values, bad
}

func valueExpressionKey(expr ast.Expr, info *types.Info) string {
	if info != nil {
		if value := info.Types[expr].Value; value != nil {
			if value.Kind() == constant.String {
				return constant.StringVal(value)
			}
			return value.String()
		}
	}
	return expressionKey(expr)
}

func collectDiscriminatorIfChains(pkg *packageFiles, file *ast.File, functions []*ast.FuncDecl, cfg Config, analysis *ocpAnalysis) {
	if pkg.info == nil {
		return
	}
	visited := map[*ast.IfStmt]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		stmt, ok := node.(*ast.IfStmt)
		if !ok || visited[stmt] {
			return true
		}
		fieldKey, values, ok := discriminatorCondition(stmt, pkg.info)
		if ok && !discriminatorFieldAllowed(fieldKey, cfg) {
			ok = false
		}
		if !ok {
			return true
		}
		for current := stmt; current != nil; {
			visited[current] = true
			if current != stmt {
				if nextField, nextValues, nextOK := discriminatorCondition(current, pkg.info); nextOK && nextField == fieldKey {
					values = append(values, nextValues...)
				}
			}
			next, ok := current.Else.(*ast.IfStmt)
			if !ok {
				break
			}
			current = next
		}
		analysis.discriminators = append(analysis.discriminators, &ocpDiscriminatorSite{pkg: pkg, node: stmt, pos: pkg.fset.Position(stmt.Pos()), function: enclosingFunction(functions, stmt.Pos(), pkg.pkgPath), fieldKey: fieldKey, values: uniqueSorted(values), serialization: isSerializationFunction(enclosingFunctionName(functions, stmt.Pos()))})
		return true
	})
}

func discriminatorFieldAllowed(key string, cfg Config) bool {
	name := key
	if index := strings.LastIndexByte(name, '.'); index >= 0 {
		name = name[index+1:]
	}
	for _, allowed := range cfg.OCPDiscriminatorFields {
		if name == allowed {
			return true
		}
	}
	return false
}

func discriminatorCondition(stmt *ast.IfStmt, info *types.Info) (string, []string, bool) {
	var fieldKey string
	var value ast.Expr
	ast.Inspect(stmt.Cond, func(node ast.Node) bool {
		binary, ok := node.(*ast.BinaryExpr)
		if !ok || (binary.Op.String() != "==" && binary.Op.String() != "!=") {
			return true
		}
		if key, ok := discriminatorFieldKey(binary.X, info); ok {
			fieldKey, value = key, binary.Y
			return false
		}
		if key, ok := discriminatorFieldKey(binary.Y, info); ok {
			fieldKey, value = key, binary.X
			return false
		}
		return true
	})
	if fieldKey == "" || value == nil {
		return "", nil, false
	}
	return fieldKey, []string{valueExpressionKey(value, info)}, true
}

func concreteParameterMethods(body *ast.BlockStmt, param *types.Var, info *types.Info) ([]*types.Func, bool) {
	methods := map[string]*types.Func{}
	safe := true
	ast.Inspect(body, func(node ast.Node) bool {
		if !safe {
			return false
		}
		sel, ok := node.(*ast.SelectorExpr)
		if ok {
			if ident, typeOK := sel.X.(*ast.Ident); typeOK && info.Uses[ident] == param {
				selection := info.Selections[sel]
				if selection == nil || selection.Kind() != types.MethodVal {
					safe = false
					return false
				}
				method, isMethod := selection.Obj().(*types.Func)
				if !isMethod {
					safe = false
					return false
				}
				methods[sel.Sel.Name] = method
				return false
			}
		}
		ident, ok := node.(*ast.Ident)
		if ok && info.Uses[ident] == param {
			safe = false
			return false
		}
		return true
	})
	var out []*types.Func
	for _, method := range methods {
		out = append(out, method)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out, safe
}

func methodNames(methods []*types.Func) []string {
	names := make([]string, len(methods))
	for index, method := range methods {
		names[index] = method.Name()
	}
	return names
}

func matchingInterface(pkg *packageFiles, methods []*types.Func) string {
	if pkg.info == nil {
		return ""
	}
	wanted := map[string]*types.Func{}
	for _, method := range methods {
		wanted[method.Name()] = method
	}
	var best *types.Named
	var bestCount int
	if pkg.typePkg == nil {
		return ""
	}
	scope := pkg.typePkg.Scope()
	for _, name := range scope.Names() {
		named, ok := scope.Lookup(name).Type().(*types.Named)
		if !ok {
			continue
		}
		iface, ok := named.Underlying().(*types.Interface)
		if !ok || iface.NumMethods() < len(wanted) {
			continue
		}
		all := true
		for method, concreteMethod := range wanted {
			found := false
			for i := 0; i < iface.NumMethods(); i++ {
				candidate := iface.Method(i)
				if candidate.Name() == method && identicalMethodSignature(concreteMethod, candidate) {
					found = true
					break
				}
			}
			if !found {
				all = false
				break
			}
		}
		if all && (best == nil || iface.NumMethods() < bestCount || (iface.NumMethods() == bestCount && canonicalTypeKey(named) < canonicalTypeKey(best))) {
			best, bestCount = named, iface.NumMethods()
		}
	}
	if best == nil {
		return ""
	}
	return canonicalTypeKey(best)
}

func identicalMethodSignature(left, right *types.Func) bool {
	leftSig, leftOK := left.Type().(*types.Signature)
	rightSig, rightOK := right.Type().(*types.Signature)
	if !leftOK || !rightOK {
		return false
	}
	withoutReceiver := func(sig *types.Signature) *types.Signature {
		params := make([]*types.TypeParam, sig.TypeParams().Len())
		for index := range params {
			params[index] = sig.TypeParams().At(index)
		}
		return types.NewSignatureType(nil, nil, params, sig.Params(), sig.Results(), sig.Variadic())
	}
	return types.Identical(withoutReceiver(leftSig), withoutReceiver(rightSig))
}

func functionReturnsInterface(fn *ast.FuncDecl, info *types.Info) bool {
	if fn.Type.Results == nil || info == nil {
		return false
	}
	for _, field := range fn.Type.Results.List {
		if isInterface(info.TypeOf(field.Type)) {
			return true
		}
	}
	return false
}

func importSpecSpan(pkg *packageFiles, importPath string) (start, end token.Pos) {
	for _, file := range pkg.files {
		for _, spec := range file.Imports {
			value, err := strconv.Unquote(spec.Path.Value)
			if err == nil && value == importPath {
				return spec.Pos(), spec.End()
			}
		}
	}
	return token.NoPos, token.NoPos
}

func importRelatedLocations(pkg *packageFiles, imports []string) []RelatedLocation {
	wanted := map[string]bool{}
	for _, imported := range imports {
		wanted[imported] = true
	}
	var out []RelatedLocation
	for _, file := range pkg.files {
		for _, spec := range file.Imports {
			value, err := strconv.Unquote(spec.Path.Value)
			if err == nil && wanted[value] {
				out = append(out, RelatedLocation{Pos: pkg.fset.Position(spec.Pos()), Message: value})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return positionKey(out[i].Pos) < positionKey(out[j].Pos) })
	return out
}

func matchesAnyPackagePattern(value string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchPackagePattern(value, pattern) {
			return true
		}
	}
	return false
}
func matchPackagePattern(value, pattern string) bool {
	value, pattern = strings.TrimSuffix(value, "/"), strings.TrimSuffix(strings.ReplaceAll(pattern, "\\", "/"), "/")
	if value == pattern {
		return true
	}
	if strings.HasSuffix(pattern, "/**") {
		base := strings.TrimSuffix(pattern, "/**")
		if strings.ContainsAny(base, "*?[") {
			for candidate := value; candidate != "." && candidate != "/"; candidate = path.Dir(candidate) {
				if matched, _ := path.Match(base, candidate); matched {
					return true
				}
				parent := path.Dir(candidate)
				if parent == candidate {
					break
				}
			}
			return false
		}
		return value == base || strings.HasPrefix(value, base+"/")
	}
	matched, _ := path.Match(pattern, value)
	return matched
}

func concreteParameterTypeKeys(fn *ast.FuncDecl, info *types.Info) []string {
	var out []string
	if fn.Type.Params == nil {
		return nil
	}
	for _, field := range fn.Type.Params.List {
		typ := info.TypeOf(field.Type)
		if concreteTypeCandidate(typ) {
			out = append(out, canonicalTypeKey(typ))
		}
	}
	return out
}

func normalizedFunctionTokens(fn *ast.FuncDecl, info *types.Info) ([]string, int) {
	params := map[types.Object]bool{}
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			for _, name := range field.Names {
				if obj := info.Defs[name]; obj != nil {
					params[obj] = true
				}
			}
		}
	}
	var tokens []string
	nodes := 0
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if node == nil {
			return true
		}
		nodes++
		tokens = append(tokens, fmt.Sprintf("%T", node))
		if ident, ok := node.(*ast.Ident); ok {
			if params[info.Uses[ident]] || params[info.Defs[ident]] {
				tokens = append(tokens, "$param")
			} else {
				tokens = append(tokens, ident.Name)
			}
		}
		if selector, ok := node.(*ast.SelectorExpr); ok {
			tokens = append(tokens, selector.Sel.Name)
		}
		if literal, ok := node.(*ast.BasicLit); ok {
			tokens = append(tokens, literal.Kind.String())
		}
		return true
	})
	return tokens, nodes
}

func tokenShingles(tokens []string) map[string]bool {
	out := map[string]bool{}
	if len(tokens) < 5 {
		return out
	}
	for i := 0; i+5 <= len(tokens); i++ {
		out[strings.Join(tokens[i:i+5], "|")] = true
	}
	return out
}
func similarityPercent(a, b map[string]bool) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	common := 0
	for value := range a {
		if b[value] {
			common++
		}
	}
	return 100 * common / (len(a) + len(b) - common)
}
func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
