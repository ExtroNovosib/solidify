package analyzer

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type srpTypeProfile struct {
	name             string
	pos              token.Pos
	end              token.Pos
	fields           []string
	fieldTypeKeys    []string
	serializedFields int
	methods          []*ast.FuncDecl
	lines            int
	metrics          srpMethodMetrics
}

type srpMethodMetrics struct {
	fieldUsers    map[string]map[int]bool
	callEdges     map[[2]int]bool
	methodImports map[int]map[string]bool
	complexity    int
	wmc           int
	tcc           float64
	lcom4         [][]int
	fanout        map[string]bool
	atfd          map[string]bool
}

func newSRPMethodMetrics() srpMethodMetrics {
	return srpMethodMetrics{
		fieldUsers:    map[string]map[int]bool{},
		callEdges:     map[[2]int]bool{},
		methodImports: map[int]map[string]bool{},
		fanout:        map[string]bool{},
		atfd:          map[string]bool{},
	}
}

func collectSRPStructProfiles(files []*ast.File, info *types.Info, pkg *types.Package, pkgFiles *packageFiles) map[string]*srpTypeProfile {
	profiles := map[string]*srpTypeProfile{}
	for _, file := range files {
		if skipGenerated(pkgFiles, file) {
			continue
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok.String() != "type" {
				continue
			}
			for _, spec := range gen.Specs {
				if profile := srpProfileFromTypeSpec(spec, info, pkg); profile != nil {
					profiles[profile.name] = profile
				}
			}
		}
	}
	return profiles
}

func srpProfileFromTypeSpec(spec ast.Spec, info *types.Info, pkg *types.Package) *srpTypeProfile {
	ts, ok := spec.(*ast.TypeSpec)
	if !ok {
		return nil
	}
	st, ok := ts.Type.(*ast.StructType)
	if !ok {
		return nil
	}
	profile := &srpTypeProfile{name: ts.Name.Name, pos: ts.Pos(), end: ts.End(), metrics: newSRPMethodMetrics()}
	profile.serializedFields, _ = serializedStructFieldCounts(st)
	for _, field := range st.Fields.List {
		typeKey := srpFieldTypeKey(field.Type, info)
		for _, name := range field.Names {
			profile.fields = append(profile.fields, name.Name)
			profile.fieldTypeKeys = append(profile.fieldTypeKeys, typeKey)
		}
		if len(field.Names) == 0 {
			if name := embeddedFieldName(field.Type); name != "" {
				profile.fields = append(profile.fields, name)
				profile.fieldTypeKeys = append(profile.fieldTypeKeys, typeKey)
			}
		}
		if info != nil && pkg != nil {
			collectExternalNamedTypes(info.TypeOf(field.Type), pkg, profile.metrics.fanout)
		}
	}
	return profile
}

func attachSRPMethodsToProfiles(profiles map[string]*srpTypeProfile, files []*ast.File, pkgFiles *packageFiles) {
	for _, file := range files {
		if skipGenerated(pkgFiles, file) {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
				continue
			}
			name := receiverTypeName(fn.Recv.List[0].Type)
			profile := profiles[name]
			if profile == nil {
				continue
			}
			profile.methods = append(profile.methods, fn)
		}
	}
}

func finalizeSRPTypeProfiles(profiles map[string]*srpTypeProfile, fset *token.FileSet, files []*ast.File, info *types.Info, pkg *types.Package) []*srpTypeProfile {
	for _, profile := range profiles {
		sort.Slice(profile.methods, func(i, j int) bool { return profile.methods[i].Pos() < profile.methods[j].Pos() })
		analyzeSRPTypeProfile(profile, info, pkg, buildImportMap(files), pkgPath(pkg))
		profile.lines = srpProfileTypeLines(profile, fset)
	}
	out := make([]*srpTypeProfile, 0, len(profiles))
	for _, profile := range profiles {
		out = append(out, profile)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

func embeddedFieldName(expr ast.Expr) string {
	if name := unwrapIndexedTypeExpr(expr); name != "" {
		return name
	}
	switch field := expr.(type) {
	case *ast.Ident:
		return field.Name
	case *ast.StarExpr:
		return embeddedFieldName(field.X)
	case *ast.SelectorExpr:
		return field.Sel.Name
	}
	return ""
}

func unwrapIndexedTypeExpr(expr ast.Expr) string {
	switch field := expr.(type) {
	case *ast.IndexExpr:
		return embeddedFieldName(field.X)
	case *ast.IndexListExpr:
		return embeddedFieldName(field.X)
	}
	return ""
}

func pkgPath(pkg *types.Package) string {
	if pkg == nil {
		return ""
	}
	return pkg.Path()
}

func buildImportMap(files []*ast.File) map[string]string {
	imports := map[string]string{}
	for _, file := range files {
		for _, spec := range file.Imports {
			path := strings.Trim(spec.Path.Value, `"`)
			localName := importLocalName(spec, path)
			if localName == "" {
				continue
			}
			imports[localName] = path
		}
	}
	return imports
}

func importLocalName(spec *ast.ImportSpec, path string) string {
	if spec.Name != nil {
		switch spec.Name.Name {
		case "_", ".":
			return ""
		default:
			return spec.Name.Name
		}
	}
	if slash := strings.LastIndex(path, "/"); slash >= 0 {
		return path[slash+1:]
	}
	return path
}

func analyzeSRPTypeProfile(profile *srpTypeProfile, info *types.Info, pkg *types.Package, importMap map[string]string, localPkgPath string) {
	fieldSet := map[string]bool{}
	for _, field := range profile.fields {
		fieldSet[field] = true
	}
	methodIndex := map[string]int{}
	for i, method := range profile.methods {
		methodIndex[method.Name.Name] = i
		scoreSRPMethod(profile, method, info, pkg)
		if method.Body == nil {
			continue
		}
		receivers := receiverNames(method)
		ctx := selectionRecordContext{receivers: receivers, fieldSet: fieldSet, methodIndex: methodIndex, info: info, pkg: pkg}
		methodIdx := methodIndex[method.Name.Name]
		if info != nil && pkg != nil && method.Body != nil {
			profile.metrics.methodImports[methodIdx] = collectMethodExternalImports(method.Body, info, pkg, importMap, localPkgPath)
		} else if method.Body != nil {
			profile.metrics.methodImports[methodIdx] = collectMethodExternalImports(method.Body, nil, nil, importMap, localPkgPath)
		}
		ast.Inspect(method.Body, func(node ast.Node) bool {
			sel, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if info != nil {
				recordSRPSelection(profile, sel, method, ctx)
			}
			recordSRPSyntaxFallback(profile, sel, method, ctx)
			return true
		})
	}
	computeSRPProfileCohesion(profile)
}

type selectionRecordContext struct {
	receivers   []string
	fieldSet    map[string]bool
	methodIndex map[string]int
	info        *types.Info
	pkg         *types.Package
}

func scoreSRPMethod(profile *srpTypeProfile, method *ast.FuncDecl, info *types.Info, pkg *types.Package) {
	complexity := functionComplexity(method)
	profile.metrics.complexity += complexity
	profile.metrics.wmc += complexity
	collectSRPProfileSignatureDependencies(profile, method, info, pkg)
}

func recordSRPSelection(profile *srpTypeProfile, sel *ast.SelectorExpr, method *ast.FuncDecl, ctx selectionRecordContext) {
	selection := ctx.info.Selections[sel]
	if selection == nil {
		return
	}
	if selection.Kind() == types.FieldVal {
		if obj := selection.Obj(); obj != nil {
			if obj.Pkg() != nil && ctx.pkg != nil && obj.Pkg() != ctx.pkg {
				profile.metrics.atfd[obj.Pkg().Path()+"/"+obj.Name()] = true
			}
		}
		if obj := selection.Obj(); obj != nil && selectionRecvIsLocal(sel, ctx.receivers) {
			if ctx.fieldSet[obj.Name()] {
				if profile.metrics.fieldUsers[obj.Name()] == nil {
					profile.metrics.fieldUsers[obj.Name()] = map[int]bool{}
				}
				profile.metrics.fieldUsers[obj.Name()][ctx.methodIndex[method.Name.Name]] = true
			}
		}
	} else if selection.Kind() == types.MethodVal || selection.Kind() == types.MethodExpr {
		if obj := selection.Obj(); obj != nil {
			if target, found := ctx.methodIndex[obj.Name()]; found && target != ctx.methodIndex[method.Name.Name] {
				profile.metrics.callEdges[[2]int{ctx.methodIndex[method.Name.Name], target}] = true
			}
		}
	}
}

func recordSRPSyntaxFallback(profile *srpTypeProfile, sel *ast.SelectorExpr, method *ast.FuncDecl, ctx selectionRecordContext) {
	if receiver := selectorReceiverName(sel.X); receiver != "" && containsString(ctx.receivers, receiver) {
		if ctx.fieldSet[sel.Sel.Name] {
			idx := ctx.methodIndex[method.Name.Name]
			if profile.metrics.fieldUsers[sel.Sel.Name] == nil {
				profile.metrics.fieldUsers[sel.Sel.Name] = map[int]bool{}
			}
			profile.metrics.fieldUsers[sel.Sel.Name][idx] = true
		}
		if target, found := ctx.methodIndex[sel.Sel.Name]; found && target != ctx.methodIndex[method.Name.Name] {
			profile.metrics.callEdges[[2]int{ctx.methodIndex[method.Name.Name], target}] = true
		}
	}
}

func receiverNames(fn *ast.FuncDecl) []string {
	if fn.Recv == nil {
		return nil
	}
	var names []string
	for _, field := range fn.Recv.List {
		for _, name := range field.Names {
			names = append(names, name.Name)
		}
	}
	return names
}

func selectorReceiverName(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.StarExpr:
		return selectorReceiverName(x.X)
	case *ast.ParenExpr:
		return selectorReceiverName(x.X)
	}
	return ""
}

func selectionRecvIsLocal(sel *ast.SelectorExpr, receivers []string) bool {
	return containsString(receivers, selectorReceiverName(sel.X))
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func computeSRPProfileCohesion(profile *srpTypeProfile) {
	n := len(profile.methods)
	if n == 0 {
		return
	}
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[rb] = ra
		}
	}
	for _, users := range profile.metrics.fieldUsers {
		var indices []int
		for idx := range users {
			indices = append(indices, idx)
		}
		for i := 1; i < len(indices); i++ {
			union(indices[0], indices[i])
		}
	}
	for edge := range profile.metrics.callEdges {
		union(edge[0], edge[1])
	}
	components := map[int][]int{}
	for i := range parent {
		root := find(i)
		components[root] = append(components[root], i)
	}
	for _, component := range components {
		sort.Ints(component)
		profile.metrics.lcom4 = append(profile.metrics.lcom4, component)
	}
	sort.Slice(profile.metrics.lcom4, func(i, j int) bool { return profile.metrics.lcom4[i][0] < profile.metrics.lcom4[j][0] })
	sharedPairs := map[[2]int]bool{}
	for _, users := range profile.metrics.fieldUsers {
		var indices []int
		for idx := range users {
			indices = append(indices, idx)
		}
		for i := 0; i < len(indices); i++ {
			for j := i + 1; j < len(indices); j++ {
				left, right := indices[i], indices[j]
				if left > right {
					left, right = right, left
				}
				sharedPairs[[2]int{left, right}] = true
			}
		}
	}
	if n > 1 {
		profile.metrics.tcc = 100 * float64(len(sharedPairs)) / float64(n*(n-1)/2)
		if profile.metrics.tcc < 0 {
			profile.metrics.tcc = 0
		} else if profile.metrics.tcc > 100 {
			profile.metrics.tcc = 100
		}
	}
}

func srpProfileTypeLines(profile *srpTypeProfile, fset *token.FileSet) int {
	type lineInterval struct{ start, end int }
	byFile := map[string][]lineInterval{}
	add := func(startPos, endPos token.Pos) {
		start, end := fset.Position(startPos), fset.Position(endPos)
		if start.Filename == "" || start.Filename != end.Filename || start.Line <= 0 || end.Line < start.Line {
			return
		}
		byFile[start.Filename] = append(byFile[start.Filename], lineInterval{start: start.Line, end: end.Line})
	}
	add(profile.pos, profile.end)
	for _, method := range profile.methods {
		add(method.Pos(), method.End())
	}
	total := 0
	for _, intervals := range byFile {
		sort.Slice(intervals, func(i, j int) bool { return intervals[i].start < intervals[j].start })
		merged := intervals[0]
		for _, current := range intervals[1:] {
			if current.start <= merged.end+1 {
				if current.end > merged.end {
					merged.end = current.end
				}
				continue
			}
			total += merged.end - merged.start + 1
			merged = current
		}
		total += merged.end - merged.start + 1
	}
	return total
}

func srpProfileLargeTypeIssue(profile *srpTypeProfile, fset *token.FileSet, cfg Config, typeComplete bool) *Issue {
	if len(profile.methods) == 0 {
		return nil
	}
	if len(profile.fields) >= 1 && len(profile.fields) <= 2 {
		return nil
	}
	if len(profile.methods) <= 3 && len(profile.fields) > cfg.MaxFieldsPerType {
		return nil
	}
	if len(profile.methods) <= 3 && allMethodsTrivialAccessors(profile.methods) {
		return nil
	}
	if typeComplete && profile.metrics.tcc >= float64(cfg.MinTCCPercent) {
		return nil
	}
	if hasHomogeneousFieldSet(profile) && hasRepetitiveHomogeneousMethods(profile) {
		return nil
	}
	exported := 0
	for _, method := range profile.methods {
		if ast.IsExported(method.Name.Name) {
			exported++
		}
	}
	signals := 0
	if len(profile.methods) > cfg.MaxMethodsPerType {
		signals++
	}
	if exported > cfg.MaxExportedMethods {
		signals++
	}
	if len(profile.fields) > cfg.MaxFieldsPerType && !hasHomogeneousFieldSet(profile) {
		signals++
	}
	if profile.lines > cfg.MaxTypeLines {
		signals++
	}
	if profile.metrics.wmc >= cfg.MaxTypeComplexity {
		signals++
	}
	minSignals := cfg.MinLargeTypeSignals
	if minSignals <= 0 {
		minSignals = DefaultConfig().MinLargeTypeSignals
	}
	extremeSurface := len(profile.methods) > 2*cfg.MaxMethodsPerType && len(profile.fields) > cfg.MaxFieldsPerType
	if signals < minSignals && !extremeSurface {
		return nil
	}
	issue := issueSpan(fset, profile.pos, profile.end, Issue{Rule: RuleSRP, Check: CheckSRPLargeType, Severity: SeverityWarning, Evidence: fmt.Sprintf("large-type:type=%s;methods=%d;exported_methods=%d;fields=%d;loc=%d;wmc=%d;signals=%d", profile.name, len(profile.methods), exported, len(profile.fields), profile.lines, profile.metrics.wmc, signals), Message: fmt.Sprintf("type %q exceeds %d independent size signals; split its responsibilities or collaborators", profile.name, signals), Metrics: []Metric{{Name: "methods", Value: float64(len(profile.methods)), Threshold: float64(cfg.MaxMethodsPerType), Comparator: ">"}, {Name: "exported_methods", Value: float64(exported), Threshold: float64(cfg.MaxExportedMethods), Comparator: ">"}, {Name: "fields", Value: float64(len(profile.fields)), Threshold: float64(cfg.MaxFieldsPerType), Comparator: ">"}, {Name: "loc", Value: float64(profile.lines), Threshold: float64(cfg.MaxTypeLines), Comparator: ">"}, {Name: "wmc", Value: float64(profile.metrics.wmc), Threshold: float64(cfg.MaxTypeComplexity), Comparator: ">="}}})
	return &issue
}

func srpProfileGodTypeIssue(profile *srpTypeProfile, fset *token.FileSet, cfg Config) *Issue {
	if profile.metrics.wmc < cfg.MaxTypeComplexity || profile.metrics.tcc >= float64(cfg.MinTCCPercent) || (len(profile.metrics.atfd) <= cfg.MaxATFD && len(profile.metrics.fanout) <= cfg.MaxFanOut) {
		return nil
	}
	issue := issueSpan(fset, profile.pos, profile.end, Issue{Rule: RuleSRP, Check: CheckSRPGodType, Severity: SeverityWarning, Evidence: fmt.Sprintf("god-type:type=%s;wmc=%d;tcc=%.2f;atfd=%d;fanout=%d", profile.name, profile.metrics.wmc, profile.metrics.tcc, len(profile.metrics.atfd), len(profile.metrics.fanout)), Metrics: []Metric{{Name: "wmc", Value: float64(profile.metrics.wmc), Threshold: float64(cfg.MaxTypeComplexity), Comparator: ">="}, {Name: "tcc_percent", Value: profile.metrics.tcc, Threshold: float64(cfg.MinTCCPercent), Comparator: "<"}, {Name: "atfd", Value: float64(len(profile.metrics.atfd)), Threshold: float64(cfg.MaxATFD), Comparator: ">"}, {Name: "fan_out", Value: float64(len(profile.metrics.fanout)), Threshold: float64(cfg.MaxFanOut), Comparator: ">"}}, Groups: srpProfileSymbolGroups(profile), Related: srpProfileRelatedLocations(profile, fset), Message: fmt.Sprintf("type %q has high complexity, low cohesion, and excessive foreign collaboration: extract focused responsibilities", profile.name)})
	return &issue
}

func srpProfileLowCohesionIssue(profile *srpTypeProfile, fset *token.FileSet, cfg Config) *Issue {
	if len(profile.methods) < cfg.MinCohesionMethods || len(profile.fields) < cfg.MinCohesionFields {
		return nil
	}
	if strings.HasSuffix(profile.name, "Handler") {
		return nil
	}
	if isSerializedDataCarrier(profile.serializedFields, len(profile.fields), profile.methods) {
		return nil
	}
	var components [][]int
	for _, component := range profile.metrics.lcom4 {
		if len(component) >= cfg.MinComponentMethods {
			components = append(components, component)
		}
	}
	if len(components) < 2 {
		return nil
	}
	issue := issueSpan(fset, profile.pos, profile.end, Issue{Rule: RuleSRP, Check: CheckSRPLowCohesionType, Severity: SeverityWarning, Evidence: fmt.Sprintf("low-cohesion-type:type=%s;methods=%d;fields=%d;lcom4=%d;tcc=%.2f", profile.name, len(profile.methods), len(profile.fields), len(profile.metrics.lcom4), profile.metrics.tcc), Metrics: []Metric{{Name: "lcom4", Value: float64(len(profile.metrics.lcom4)), Threshold: 2, Comparator: ">="}, {Name: "tcc_percent", Value: profile.metrics.tcc, Threshold: float64(cfg.MinTCCPercent), Comparator: "<"}}, Groups: srpProfileSymbolGroups(profile), Related: srpProfileRelatedLocations(profile, fset), Message: fmt.Sprintf("type %q contains %d disconnected method responsibilities (LCOM4); extract cohesive collaborators", profile.name, len(components))})
	return &issue
}

func srpProfileFanOutIssue(profile *srpTypeProfile, fset *token.FileSet, cfg Config) *Issue {
	if len(profile.metrics.fanout) <= cfg.MaxFanOut {
		return nil
	}
	foreign := make([]string, 0, len(profile.metrics.fanout))
	for name := range profile.metrics.fanout {
		foreign = append(foreign, name)
	}
	issue := issueSpan(fset, profile.pos, profile.end, Issue{Rule: RuleSRP, Check: CheckSRPHighFanOutType, Severity: SeverityNote, Evidence: fmt.Sprintf("high-fan-out-type:type=%s;fanout=%d;max=%d;types=%s", profile.name, len(profile.metrics.fanout), cfg.MaxFanOut, strings.Join(SortedSymbols(foreign), ",")), Metrics: []Metric{{Name: "fan_out", Value: float64(len(profile.metrics.fanout)), Threshold: float64(cfg.MaxFanOut), Comparator: ">"}}, Groups: []SymbolGroup{{Label: "foreign-types", Symbols: SortedSymbols(foreign)}}, Message: fmt.Sprintf("type %q depends on %d foreign types (max %d): narrow its collaborators", profile.name, len(profile.metrics.fanout), cfg.MaxFanOut)})
	return &issue
}

func srpProfileSymbolGroups(profile *srpTypeProfile) []SymbolGroup {
	groups := make([]SymbolGroup, 0, len(profile.metrics.lcom4))
	for idx, component := range profile.metrics.lcom4 {
		symbols := make([]string, 0, len(component))
		for _, methodIndex := range component {
			symbols = append(symbols, profile.methods[methodIndex].Name.Name)
		}
		groups = append(groups, SymbolGroup{Label: fmt.Sprintf("lcom4-component-%d", idx+1), Symbols: SortedSymbols(symbols)})
	}
	return groups
}

func srpProfileRelatedLocations(profile *srpTypeProfile, fset *token.FileSet) []RelatedLocation {
	related := make([]RelatedLocation, 0, len(profile.methods))
	for _, method := range profile.methods {
		related = append(related, RelatedLocation{Pos: fset.Position(method.Pos()), Message: fmt.Sprintf("method %s contributes to this type profile", method.Name.Name)})
	}
	return related
}

func collectSRPProfileSignatureDependencies(profile *srpTypeProfile, fn *ast.FuncDecl, info *types.Info, pkg *types.Package) {
	if info == nil || pkg == nil || fn.Type == nil {
		return
	}
	collect := func(fieldList *ast.FieldList) {
		if fieldList == nil {
			return
		}
		for _, field := range fieldList.List {
			t := info.TypeOf(field.Type)
			collectExternalNamedTypes(t, pkg, profile.metrics.fanout)
		}
	}
	collect(fn.Type.Params)
	collect(fn.Type.Results)
}

func collectExternalNamedTypes(t types.Type, pkg *types.Package, out map[string]bool) {
	if t == nil {
		return
	}
	collectNamedOrSignature(t, pkg, out)
	collectCompositeNamedTypes(t, pkg, out)
}

func collectNamedOrSignature(t types.Type, pkg *types.Package, out map[string]bool) {
	switch x := t.(type) {
	case *types.Named:
		obj := x.Obj()
		if obj != nil && obj.Pkg() != nil && obj.Pkg() != pkg {
			out[obj.Pkg().Path()+"/"+obj.Name()] = true
		}
		collectExternalNamedTypes(x.Underlying(), pkg, out)
	case *types.Signature:
		collectTupleTypes(x.Params(), pkg, out)
		collectTupleTypes(x.Results(), pkg, out)
	}
}

func collectCompositeNamedTypes(t types.Type, pkg *types.Package, out map[string]bool) {
	if p, ok := t.(*types.Pointer); ok {
		collectExternalNamedTypes(p.Elem(), pkg, out)
		return
	}
	if s, ok := t.(*types.Slice); ok {
		collectExternalNamedTypes(s.Elem(), pkg, out)
		return
	}
	if a, ok := t.(*types.Array); ok {
		collectExternalNamedTypes(a.Elem(), pkg, out)
		return
	}
	if m, ok := t.(*types.Map); ok {
		collectExternalNamedTypes(m.Key(), pkg, out)
		collectExternalNamedTypes(m.Elem(), pkg, out)
		return
	}
	if c, ok := t.(*types.Chan); ok {
		collectExternalNamedTypes(c.Elem(), pkg, out)
	}
}

func collectTupleTypes(tuple *types.Tuple, pkg *types.Package, out map[string]bool) {
	if tuple == nil {
		return
	}
	for i := 0; i < tuple.Len(); i++ {
		collectExternalNamedTypes(tuple.At(i).Type(), pkg, out)
	}
}

func srpFieldTypeKey(expr ast.Expr, info *types.Info) string {
	if info != nil {
		return canonicalTypeKey(info.TypeOf(expr))
	}
	return astTypeKey(expr)
}

func astTypeKey(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return "*" + astTypeKey(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return astTypeKey(t.X) + "." + t.Sel.Name
	case *ast.MapType:
		return "map[" + astTypeKey(t.Key) + "]" + astTypeKey(t.Value)
	default:
		return astTypeKeyComposite(t)
	}
}

func astTypeKeyComposite(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.ArrayType:
		return "[]" + astTypeKey(t.Elt)
	case *ast.IndexExpr:
		return astTypeKey(t.X) + "[" + astTypeKey(t.Index) + "]"
	case *ast.IndexListExpr:
		parts := make([]string, 0, len(t.Indices))
		for _, index := range t.Indices {
			parts = append(parts, astTypeKey(index))
		}
		return astTypeKey(t.X) + "[" + strings.Join(parts, ",") + "]"
	default:
		return ""
	}
}

func hasHomogeneousFieldSet(profile *srpTypeProfile) bool {
	if len(profile.fieldTypeKeys) < 2 {
		return false
	}
	counts := map[string]int{}
	for _, key := range profile.fieldTypeKeys {
		if key == "" {
			continue
		}
		counts[key]++
	}
	maxCount := 0
	for _, count := range counts {
		if count > maxCount {
			maxCount = count
		}
	}
	return maxCount*2 >= len(profile.fieldTypeKeys)
}

func hasRepetitiveHomogeneousMethods(profile *srpTypeProfile) bool {
	if len(profile.methods) < 3 {
		return false
	}
	shapes := map[string]int{}
	for _, method := range profile.methods {
		if ast.IsExported(method.Name.Name) && strings.HasPrefix(method.Name.Name, "New") {
			continue
		}
		shapes[methodShapeKey(method)]++
	}
	maxShape := 0
	for _, count := range shapes {
		if count > maxShape {
			maxShape = count
		}
	}
	return maxShape*2 >= len(profile.methods)
}

func methodShapeKey(fn *ast.FuncDecl) string {
	if fn.Body == nil {
		return fn.Name.Name + ":empty"
	}
	var parts []string
	for _, stmt := range fn.Body.List {
		parts = append(parts, stmtKind(stmt))
	}
	return strings.Join(parts, ";")
}

func stmtKind(stmt ast.Stmt) string {
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		if call, ok := s.X.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				return "call:" + selectorName(sel)
			}
		}
	case *ast.AssignStmt:
		return "assign"
	case *ast.IfStmt:
		return "if"
	case *ast.ReturnStmt:
		return "return"
	default:
		return stmtKindTail(s)
	}
	return fmt.Sprintf("%T", stmt)
}

func stmtKindTail(stmt ast.Stmt) string {
	switch s := stmt.(type) {
	case *ast.DeclStmt:
		return "decl"
	case *ast.DeferStmt:
		if sel, ok := s.Call.Fun.(*ast.SelectorExpr); ok {
			return "defer:" + selectorName(sel)
		}
	}
	return fmt.Sprintf("%T", stmt)
}

func selectorName(sel *ast.SelectorExpr) string {
	if id, ok := sel.X.(*ast.Ident); ok {
		return id.Name + "." + sel.Sel.Name
	}
	return sel.Sel.Name
}

func collectMethodExternalImports(body *ast.BlockStmt, info *types.Info, pkg *types.Package, importMap map[string]string, localPkgPath string) map[string]bool {
	imports := map[string]bool{}
	if body == nil {
		return imports
	}
	ast.Inspect(body, func(node ast.Node) bool {
		sel, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok {
			if path, found := importMap[id.Name]; found {
				recordExternalImportPath(path, localPkgPath, imports)
			}
		}
		if info != nil && pkg != nil {
			if selection := info.Selections[sel]; selection != nil {
				if obj := selection.Obj(); obj != nil {
					recordExternalPackage(obj.Pkg(), pkg, imports)
				}
			}
		}
		return true
	})
	return imports
}

func recordExternalImportPath(path, localPkgPath string, out map[string]bool) {
	if path == "" || path == localPkgPath || path == "unsafe" {
		return
	}
	if isStdlibImportPath(path) {
		return
	}
	out[path] = true
}

func isStdlibImportPath(path string) bool {
	info, err := os.Stat(filepath.Join(build.Default.GOROOT, "src", path))
	return err == nil && info.IsDir()
}

func recordExternalPackage(objPkg, currentPkg *types.Package, out map[string]bool) {
	if objPkg == nil || objPkg == currentPkg {
		return
	}
	path := objPkg.Path()
	if path == "" || path == "unsafe" {
		return
	}
	if isStdlibPackage(objPkg) {
		return
	}
	out[path] = true
}

func srpProfileMixedImportClustersIssue(profile *srpTypeProfile, fset *token.FileSet, cfg Config) *Issue {
	minMethods := cfg.MinImportClusterMethods
	if minMethods <= 0 {
		minMethods = DefaultConfig().MinImportClusterMethods
	}
	components := importClusterComponents(profile, minMethods)
	if len(components) < 2 {
		return nil
	}
	clusterLabels := make([]string, 0, len(components))
	groups := make([]SymbolGroup, 0, len(components))
	for idx, component := range components {
		symbols := make([]string, 0, len(component))
		paths := map[string]bool{}
		for _, methodIndex := range component {
			symbols = append(symbols, profile.methods[methodIndex].Name.Name)
			for path := range profile.metrics.methodImports[methodIndex] {
				paths[path] = true
			}
		}
		pathList := SortedSymbols(keysFromBoolMap(paths))
		clusterLabels = append(clusterLabels, strings.Join(pathList, "+"))
		groups = append(groups, SymbolGroup{
			Label:   fmt.Sprintf("import-cluster-%d", idx+1),
			Symbols: SortedSymbols(symbols),
		})
	}
	issue := issueSpan(fset, profile.pos, profile.end, Issue{
		Rule:     RuleSRP,
		Check:    CheckSRPMixedImportClusters,
		Severity: SeverityNote,
		Evidence: fmt.Sprintf("mixed-import-clusters:type=%s;clusters=%d;packages=%s", profile.name, len(components), strings.Join(clusterLabels, "|")),
		Groups:   groups,
		Related:  srpProfileRelatedLocations(profile, fset),
		Message:  fmt.Sprintf("type %q methods cluster into %d unrelated external import groups; consider splitting by concern", profile.name, len(components)),
	})
	return &issue
}

func importClusterComponents(profile *srpTypeProfile, minMethods int) [][]int {
	n := len(profile.methods)
	if n == 0 {
		return nil
	}
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[rb] = ra
		}
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if methodImportsOverlap(profile.metrics.methodImports[i], profile.metrics.methodImports[j]) {
				union(i, j)
			}
		}
	}
	components := map[int][]int{}
	for i := 0; i < n; i++ {
		imports := profile.metrics.methodImports[i]
		if len(imports) == 0 {
			continue
		}
		root := find(i)
		components[root] = append(components[root], i)
	}
	var out [][]int
	for _, component := range components {
		if len(component) >= minMethods {
			sort.Ints(component)
			out = append(out, component)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

func methodImportsOverlap(a, b map[string]bool) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	for path := range a {
		if b[path] {
			return true
		}
	}
	return false
}

func keysFromBoolMap(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	return out
}
