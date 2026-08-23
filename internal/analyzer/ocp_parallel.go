package analyzer

import (
	"fmt"
	"go/ast"
	"sort"
	"strings"
)

func emitOCPParallelImplementations(pkgs []*packageFiles, cfg Config) []Issue {
	type functionShape struct {
		pkg      *packageFiles
		fn       *ast.FuncDecl
		name     string
		tokens   []string
		shingles map[string]bool
		typeKey  string
		nodes    int
		arity    int
		band     int
	}
	var functions []functionShape
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
				if !ok || fn.Body == nil {
					continue
				}
				keys := concreteParameterTypeKeys(fn, pkg.info)
				if len(keys) == 0 {
					continue
				}
				tokens, nodes := normalizedFunctionTokens(fn, pkg.info)
				if nodes < cfg.OCPMinParallelNodes {
					continue
				}
				functions = append(functions, functionShape{pkg: pkg, fn: fn, name: pkg.pkgPath + ":" + fn.Name.Name, tokens: tokens, shingles: tokenShingles(tokens), typeKey: strings.Join(keys, ","), nodes: nodes, arity: len(keys), band: nodes / 10})
			}
		}
	}
	var issues []Issue
	used := map[int]bool{}
	buckets := map[string][]int{}
	for index, function := range functions {
		buckets[fmt.Sprintf("%d:%d", function.arity, function.band)] = append(buckets[fmt.Sprintf("%d:%d", function.arity, function.band)], index)
	}
	for i := range functions {
		if used[i] {
			continue
		}
		group := []int{i}
		candidates := map[int]bool{}
		for band := functions[i].band - 2; band <= functions[i].band+2; band++ {
			for _, candidate := range buckets[fmt.Sprintf("%d:%d", functions[i].arity, band)] {
				candidates[candidate] = true
			}
		}
		for j := range candidates {
			if j == i || functions[i].typeKey == functions[j].typeKey || abs(functions[i].nodes-functions[j].nodes) > functions[i].nodes/5+1 {
				continue
			}
			if similarityPercent(functions[i].shingles, functions[j].shingles) >= cfg.OCPParallelSimilarityPercent {
				group = append(group, j)
			}
		}
		if len(group) < cfg.OCPMinParallelFunctions {
			continue
		}
		for _, index := range group {
			used[index] = true
		}
		sort.Slice(group, func(a, b int) bool { return functions[group[a]].name < functions[group[b]].name })
		primary := functions[group[0]]
		related := make([]RelatedLocation, 0, len(group)-1)
		allNames := make([]string, 0, len(group))
		for _, index := range group {
			allNames = append(allNames, functions[index].name)
			if index != group[0] {
				related = append(related, RelatedLocation{Pos: functions[index].pkg.fset.Position(functions[index].fn.Pos()), Message: functions[index].name})
			}
		}
		issues = append(issues, issueAt(primary.pkg.fset, primary.fn, Issue{Rule: RuleOCP, Check: CheckOCPParallelImplementations, Severity: SeverityNote,
			Message:  fmt.Sprintf("functions %s have nearly identical structure for different concrete types; consider a shared interface or generic helper", strings.Join(allNames, ", ")),
			Evidence: fmt.Sprintf("parallel-implementations:functions=%s;similarity=%d", strings.Join(allNames, ","), similarityPercent(primary.shingles, functions[group[1]].shingles)), Related: related}))
	}
	return issues
}
