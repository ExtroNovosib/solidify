package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

// checkISPConsumerRoles is the explicit, higher-precision companion to the
// stable usage-ratio check. It evaluates every application consumer field,
// including private collaborators, while remaining conservative whenever an
// interface escapes through an unresolved call or assignment.
func checkISPConsumerRoles(
	fset *token.FileSet,
	files []*ast.File,
	info *types.Info,
	cfg Config,
	pkg *packageFiles,
) []Issue {
	if !shouldCheckISPUsageRatio(info, cfg, pkg) {
		return nil
	}
	localFunctions := localFunctionDeclarations(files, info)
	var issues []Issue
	for _, file := range files {
		if skipGenerated(pkg, file) {
			continue
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok || structType.Fields == nil {
					continue
				}
				for _, field := range structType.Fields.List {
					fieldType := info.TypeOf(field.Type)
					iface, ok := eligibleUsageInterface(fieldType, info, cfg, pkg)
					if !ok || isRegistryInterface(fieldType) {
						continue
					}
					for _, name := range field.Names {
						fieldObject, ok := info.Defs[name].(*types.Var)
						if !ok {
							continue
						}
						usage := receiverFieldUsage(typeSpec.Name.Name, fieldObject, files, info, localFunctions)
						if usage.indirect || len(usage.methods) == 0 {
							continue
						}
						used := methodSetSymbols(usage.methods)
						ratioPercent := 100 * len(used) / iface.NumMethods()
						unused := unusedInterfaceMethods(iface, usage.methods)
						capability := detectCapabilityMismatch(iface.NumMethods(), used, unused)
						numericCandidate := iface.NumMethods() >= 4 && ratioPercent < cfg.ISPUsageRatioPercent
						if !numericCandidate && !capability.detected {
							continue
						}
						issues = append(issues, issueAt(fset, name, consumerRoleIssue(
							typeSpec.Name.Name,
							name.Name,
							interfaceIdentity(fieldType),
							iface.NumMethods(),
							used,
							unused,
							ratioPercent,
							capability,
						)))
					}
				}
			}
		}
	}
	return issues
}

func methodSetSymbols(methods map[string]bool) []string {
	values := make([]string, 0, len(methods))
	for method := range methods {
		values = append(values, method)
	}
	return SortedSymbols(values)
}

func consumerRoleIssue(
	owner, field, interfaceName string,
	total int,
	used, unused []string,
	ratioPercent int,
	capability capabilityMismatch,
) Issue {
	severity := consumerRoleSeverity(ratioPercent, used, unused, capability)
	reason := "usage-ratio"
	if capability.detected {
		reason = "capability-" + capability.direction
	}
	message := fmt.Sprintf(
		"field %s.%s accepts interface %s with %d methods but its consumer role uses %d (%d%%): consider a narrower interface with just %s",
		owner, field, interfaceName, total, len(used), ratioPercent, strings.Join(used, ", "),
	)
	if capability.detected {
		message = fmt.Sprintf(
			"field %s.%s is a %s consumer of interface %s but also receives %s capabilities: consider a narrower role with just %s",
			owner, field, capability.direction, interfaceName, capability.opposite, strings.Join(used, ", "),
		)
	}
	return Issue{
		Rule:     RuleISP,
		Check:    CheckISPConsumerRole,
		Severity: severity,
		Message:  message,
		Evidence: fmt.Sprintf(
			"consumer-role:type=%s;field=%s;interface=%s;used=%d;total=%d;ratio=%d;methods=%s;unused=%s;reason=%s",
			owner, field, interfaceName, len(used), total, ratioPercent, strings.Join(used, ","), strings.Join(unused, ","), reason,
		),
	}
}

func consumerRoleSeverity(ratioPercent int, used, unused []string, capability capabilityMismatch) Severity {
	if capability.detected || ratioPercent <= 30 || queueWorkerCapabilitySplit(used, unused) {
		return SeverityWarning
	}
	return SeverityNote
}

type capabilityMismatch struct {
	detected  bool
	direction string
	opposite  string
}

func detectCapabilityMismatch(total int, used, unused []string) capabilityMismatch {
	usedRead, usedWrite := capabilityMethodCounts(used)
	unusedRead, unusedWrite := capabilityMethodCounts(unused)
	if usedRead > 0 && usedWrite == 0 && unusedWrite > 0 && (len(used) <= 1 || total >= 6 && unusedWrite <= 2) {
		return capabilityMismatch{detected: true, direction: "read-only", opposite: "mutation"}
	}
	if usedWrite > 0 && usedRead == 0 && unusedRead >= 3 && len(used) <= 2 {
		return capabilityMismatch{detected: true, direction: "write-only", opposite: "query"}
	}
	return capabilityMismatch{}
}

func capabilityMethodCounts(methods []string) (read, write int) {
	for _, method := range methods {
		switch methodCapability(method) {
		case methodCapabilityOther:
			continue
		case methodCapabilityRead:
			read++
		case methodCapabilityWrite:
			write++
		}
	}
	return read, write
}

type capability int

const (
	methodCapabilityOther capability = iota
	methodCapabilityRead
	methodCapabilityWrite
)

func methodCapability(name string) capability {
	if hasMethodPrefix(name, "Get", "List", "Find", "Lookup", "Load", "Read", "Count", "Is", "Has") {
		return methodCapabilityRead
	}
	if hasMethodPrefix(name, "Add", "Apply", "Archive", "Cancel", "Claim", "Create", "Delete", "Dismiss", "Enqueue", "Finish", "Mark", "Publish", "Recover", "Remember", "Remove", "Renew", "Requeue", "Request", "Save", "Set", "Start", "Store", "Update", "Write") {
		return methodCapabilityWrite
	}
	return methodCapabilityOther
}

func queueWorkerCapabilitySplit(used, unused []string) bool {
	workerMethods := 0
	for _, method := range unused {
		if hasMethodPrefix(method, "Claim", "Renew", "Recover", "Requeue") {
			workerMethods++
		}
	}
	if workerMethods < 2 {
		return false
	}
	for _, method := range used {
		if hasMethodPrefix(method, "Cancel", "Create", "Enqueue", "Request") {
			return true
		}
	}
	return false
}

func hasMethodPrefix(name string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func unusedInterfaceMethods(iface *types.Interface, used map[string]bool) []string {
	iface.Complete()
	unused := make([]string, 0, iface.NumMethods())
	for index := 0; index < iface.NumMethods(); index++ {
		name := iface.Method(index).Name()
		if !used[name] {
			unused = append(unused, name)
		}
	}
	return SortedSymbols(unused)
}

func interfaceIdentity(t types.Type) string {
	named, ok := types.Unalias(t).(*types.Named)
	if ok && named.Obj() != nil {
		if pkg := named.Obj().Pkg(); pkg != nil {
			return pkg.Path() + "." + named.Obj().Name()
		}
		return named.Obj().Name()
	}
	return types.TypeString(t, func(pkg *types.Package) string { return pkg.Path() })
}

func isRegistryInterface(t types.Type) bool {
	named, ok := types.Unalias(t).(*types.Named)
	return ok && named.Obj() != nil && strings.HasSuffix(named.Obj().Name(), "Registry")
}

type receiverUsage struct {
	methods  map[string]bool
	indirect bool
}

func receiverFieldUsage(
	owner string,
	fieldObject *types.Var,
	files []*ast.File,
	info *types.Info,
	localFunctions map[*types.Func]*ast.FuncDecl,
) receiverUsage {
	usage := receiverUsage{methods: map[string]bool{}}
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !isReceiverMethodOf(fn, owner) {
				continue
			}
			parents := astParentIndex(fn.Body)
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				if usage.indirect {
					return false
				}
				selector, ok := node.(*ast.SelectorExpr)
				if !ok || selectionObject(info, selector) != fieldObject {
					return true
				}
				usage.record(selector, parents[selector], info, localFunctions)
				return false
			})
			if usage.indirect {
				return usage
			}
		}
	}
	return usage
}

func (usage *receiverUsage) record(
	selector *ast.SelectorExpr,
	parent ast.Node,
	info *types.Info,
	localFunctions map[*types.Func]*ast.FuncDecl,
) {
	switch current := parent.(type) {
	case *ast.SelectorExpr:
		if current.X == selector {
			usage.methods[current.Sel.Name] = true
			return
		}
	case *ast.BinaryExpr:
		if (current.Op == token.EQL || current.Op == token.NEQ) &&
			((current.X == selector && isNilExpression(current.Y)) || (current.Y == selector && isNilExpression(current.X))) {
			return
		}
	case *ast.CallExpr:
		if argumentIndex, ok := directArgumentIndex(current, selector); ok {
			parameter, callee, ok := localCallParameter(current, argumentIndex, localFunctions, info)
			if !ok {
				usage.indirect = true
				return
			}
			methods, indirect := interfaceMethodsUsed(callee.Body, parameter, info)
			if indirect {
				usage.indirect = true
				return
			}
			for _, method := range methods {
				usage.methods[method] = true
			}
			return
		}
	}
	usage.indirect = true
}

func directArgumentIndex(call *ast.CallExpr, expression ast.Expr) (int, bool) {
	for index, argument := range call.Args {
		if argument == expression {
			return index, true
		}
	}
	return 0, false
}

func isNilExpression(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "nil"
}

func astParentIndex(root ast.Node) map[ast.Node]ast.Node {
	parents := map[ast.Node]ast.Node{}
	stack := []ast.Node{}
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			return false
		}
		if len(stack) > 0 {
			parents[node] = stack[len(stack)-1]
		}
		stack = append(stack, node)
		return true
	})
	return parents
}

func checkISPUnusedDependencies(
	fset *token.FileSet,
	files []*ast.File,
	info *types.Info,
	cfg Config,
	pkg *packageFiles,
) []Issue {
	if !shouldCheckISPUsageRatio(info, cfg, pkg) {
		return nil
	}
	flows := newDependencyFieldFlows(files, info)
	var issues []Issue
	for _, file := range files {
		if skipGenerated(pkg, file) {
			continue
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || !typeSpec.Name.IsExported() || isWiringAggregateName(typeSpec.Name.Name) {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok || structType.Fields == nil {
					continue
				}
				for _, field := range structType.Fields.List {
					fieldType := info.TypeOf(field.Type)
					if !eligibleOwnedInterface(fieldType, info, pkg) {
						continue
					}
					for _, name := range field.Names {
						if !name.IsExported() {
							continue
						}
						fieldObject, ok := info.Defs[name].(*types.Var)
						if !ok || flows.consumed(fieldObject) {
							continue
						}
						issues = append(issues, issueAt(fset, name, Issue{
							Rule:     RuleISP,
							Check:    CheckISPUnusedDependency,
							Severity: SeverityWarning,
							Message: fmt.Sprintf(
								"field %s.%s injects interface %s but its dependency flow is never consumed: remove it or wire it into a real collaborator",
								typeSpec.Name.Name, name.Name, interfaceIdentity(fieldType),
							),
							Evidence: fmt.Sprintf(
								"unused-dependency:type=%s;field=%s;interface=%s;methods=;flow=unread",
								typeSpec.Name.Name, name.Name, interfaceIdentity(fieldType),
							),
						}))
					}
				}
			}
		}
	}
	return issues
}

func isWiringAggregateName(name string) bool {
	return strings.HasSuffix(name, "Bundle") ||
		strings.HasSuffix(name, "Deps") ||
		strings.HasSuffix(name, "Dependencies") ||
		strings.HasSuffix(name, "Stores")
}

func eligibleOwnedInterface(t types.Type, info *types.Info, pkg *packageFiles) bool {
	if info == nil || isExternalInterface(t, pkg) || isWellKnownWideInterface(t) {
		return false
	}
	iface, ok := underlyingInterface(t)
	if !ok {
		return false
	}
	iface.Complete()
	return iface.NumMethods() > 0
}

type dependencyFieldFlows struct {
	info      *types.Info
	parents   map[ast.Node]ast.Node
	selectors map[*types.Var][]*ast.SelectorExpr
	memo      map[*types.Var]bool
	visiting  map[*types.Var]bool
}

func newDependencyFieldFlows(files []*ast.File, info *types.Info) *dependencyFieldFlows {
	flows := &dependencyFieldFlows{
		info:      info,
		parents:   map[ast.Node]ast.Node{},
		selectors: map[*types.Var][]*ast.SelectorExpr{},
		memo:      map[*types.Var]bool{},
		visiting:  map[*types.Var]bool{},
	}
	for _, file := range files {
		for node, parent := range astParentIndex(file) {
			flows.parents[node] = parent
		}
		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			field, ok := selectionObject(info, selector).(*types.Var)
			if !ok || !field.IsField() {
				return true
			}
			flows.selectors[field] = append(flows.selectors[field], selector)
			return true
		})
	}
	return flows
}

func (flows *dependencyFieldFlows) consumed(field *types.Var) bool {
	if value, ok := flows.memo[field]; ok {
		return value
	}
	if flows.visiting[field] {
		return true
	}
	flows.visiting[field] = true
	deferred := map[*types.Var]bool{}
	consumed := false
	for _, selector := range flows.selectors[field] {
		if destination, ok := flows.flowDestination(selector); ok {
			deferred[destination] = true
			continue
		}
		consumed = true
		break
	}
	if !consumed {
		for destination := range deferred {
			if flows.consumed(destination) {
				consumed = true
				break
			}
		}
	}
	delete(flows.visiting, field)
	flows.memo[field] = consumed
	return consumed
}

func (flows *dependencyFieldFlows) flowDestination(selector *ast.SelectorExpr) (*types.Var, bool) {
	parent := flows.parents[selector]
	switch current := parent.(type) {
	case *ast.KeyValueExpr:
		if current.Value != selector {
			return nil, false
		}
		literal, ok := flows.parents[current].(*ast.CompositeLit)
		if !ok {
			return nil, false
		}
		return compositeLiteralField(flows.info, literal, current.Key)
	case *ast.AssignStmt:
		for index, value := range current.Rhs {
			if value != selector || index >= len(current.Lhs) {
				continue
			}
			destination, ok := current.Lhs[index].(*ast.SelectorExpr)
			if !ok {
				return nil, false
			}
			field, ok := selectionObject(flows.info, destination).(*types.Var)
			return field, ok && field.IsField()
		}
	}
	return nil, false
}

func compositeLiteralField(info *types.Info, literal *ast.CompositeLit, key ast.Expr) (*types.Var, bool) {
	ident, ok := key.(*ast.Ident)
	if !ok {
		return nil, false
	}
	if object, found := info.Uses[ident].(*types.Var); found && object.IsField() {
		return object, true
	}
	structType, ok := dereferencedStructType(info.TypeOf(literal))
	if !ok {
		return nil, false
	}
	for index := 0; index < structType.NumFields(); index++ {
		field := structType.Field(index)
		if field.Name() == ident.Name {
			return field, true
		}
	}
	return nil, false
}

func dereferencedStructType(t types.Type) (*types.Struct, bool) {
	if pointer, ok := t.(*types.Pointer); ok {
		t = pointer.Elem()
	}
	structType, ok := t.Underlying().(*types.Struct)
	return structType, ok
}
