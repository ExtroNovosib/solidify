package analyzer

import (
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Rules that can individually be enabled/disabled from the CLI.
var All = []Rule{RuleSRP, RuleOCP, RuleLSP, RuleISP, RuleDIP}

// packageFiles groups parsed files by their package's directory, so SRP's
// method-count-per-type and DIP's local-type index work across every file
// of a package, not just one file at a time (mirroring how `go vet`
// reasons about a package).
type packageFiles struct {
	dir              string
	fset             *token.FileSet
	files            []*ast.File
	info             *types.Info
	typePkg          *types.Package
	typeComplete     bool
	pkgPath          string
	pkgName          string
	modulePath       string
	imports          []string
	typeImports      map[string]*types.Package
	dependencyFacts  string
	analysisRoot     string
	generated        map[*ast.File]bool
	filteredTypeErr  string
	filteredRebuilds int
}

// Load walks root recursively, parses every non-test, non-vendor .go file,
// and groups them by directory (== package, for our purposes).
//
// Deprecated: use LoadWorkspace.
func Load(root string, includeTests bool) ([]*packageFiles, error) {
	pkgs, _, err := LoadWorkspace([]string{root}, includeTests, syntaxAnalysisMode)
	return pkgs, err
}

// LoadWithTypes optionally enriches parsed packages with standard-library
// go/types information. A type-check failure leaves syntax analysis usable.
//
// Deprecated: use LoadWorkspace.
func LoadWithTypes(root string, includeTests, withTypes bool) ([]*packageFiles, error) {
	mode := syntaxAnalysisMode
	if withTypes {
		mode = "auto"
	}
	pkgs, _, err := LoadWorkspace([]string{root}, includeTests, mode)
	return pkgs, err
}

func parsePackageFiles(dir string, paths []string, withTypes bool) (*packageFiles, error) {
	sort.Strings(paths)
	fset := token.NewFileSet()
	var files []*ast.File
	for _, p := range paths {
		f, err := parser.ParseFile(fset, p, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	pkg := &packageFiles{dir: dir, fset: fset, files: files}
	if withTypes && len(files) > 0 {
		info := &types.Info{
			Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{},
			Types: map[ast.Expr]types.TypeAndValue{}, Selections: map[*ast.SelectorExpr]*types.Selection{},
		}
		conf := types.Config{Importer: importer.Default(), Error: func(error) {}}
		checkedPkg, checkErr := conf.Check(files[0].Name.Name, fset, files, info)
		pkg.info = info
		pkg.typePkg = checkedPkg
		pkg.typeComplete = checkErr == nil && checkedPkg != nil
	}
	return pkg, nil
}

// Run executes every registered check against loaded packages and returns all
// issues, sorted by file/line for stable, readable output.
func Run(pkgs []*packageFiles, cfg Config, enabled map[Rule]bool) []Issue {
	pkgs = prepareRunPackages(pkgs, cfg)
	cache := initRunCache(pkgs, cfg, enabled)
	all := runPackageScopedChecks(pkgs, cfg, enabled, cache)
	all = append(all, runProgramScopedChecks(pkgs, cfg, enabled)...)
	reportRunDiagnostics(cfg, cache, pkgs)
	stampAnalysisRoots(all, pkgs)
	all = filterDisabledChecks(all, cfg.DisabledChecks)
	if selection, err := ResolveCheckSelection(cfg.Profile, enabled, cfg.EnabledChecks, cfg.DisabledChecks); err == nil {
		all = filterSelectedChecks(all, selection)
	}
	all = filterModeUnsupported(all, pkgs, cfg.AnalysisMode)
	sortIssues(all)
	all = applySuppressions(all, pkgs)
	AttachDefaultSuppressions(all)
	for index := range all {
		packagePath := "workspace"
		for _, pkg := range pkgs {
			if issueBelongsToPackage(all[index], pkg) {
				packagePath = pkg.pkgPath
				break
			}
		}
		if all[index].Subject == "" || all[index].Identity == "" {
			all[index].Subject, all[index].Identity = deriveIssueIdentity(all[index], packagePath)
		}
	}
	_ = FinalizeIssues(all, "workspace")
	return all
}

func issueBelongsToPackage(issue Issue, pkg *packageFiles) bool {
	filename := filepath.Clean(issue.Pos.Filename)
	for _, file := range pkg.files {
		if filepath.Clean(pkg.fset.Position(file.Pos()).Filename) == filename {
			return true
		}
	}
	return false
}

func filterSelectedChecks(issues []Issue, selection map[CheckID]bool) []Issue {
	out := issues[:0]
	for _, issue := range issues {
		if selection[issue.Check] {
			out = append(out, issue)
		}
	}
	return out
}

func filterModeUnsupported(issues []Issue, pkgs []*packageFiles, mode string) []Issue {
	if mode == "" {
		mode = analysisModeAuto
	}
	out := issues[:0]
	for _, issue := range issues {
		metadata, ok := CheckMetadata(issue.Check)
		if !ok {
			continue
		}
		if mode == syntaxAnalysisMode && metadata.Syntax == SyntaxUnavailable {
			continue
		}
		if mode == analysisModeAuto && metadata.Syntax == SyntaxUnavailable && !issuePackageTypeComplete(issue, pkgs) {
			continue
		}
		out = append(out, issue)
	}
	return out
}

func issuePackageTypeComplete(issue Issue, pkgs []*packageFiles) bool {
	filename := filepath.Clean(issue.Pos.Filename)
	for _, pkg := range pkgs {
		for _, file := range pkg.files {
			if filepath.Clean(pkg.fset.Position(file.Pos()).Filename) == filename {
				return pkg.typeComplete
			}
		}
	}
	return false
}

func prepareRunPackages(pkgs []*packageFiles, cfg Config) []*packageFiles {
	return ApplyWorkspaceFilePolicy(pkgs, cfg.ExcludedFiles)
}

func initRunCache(pkgs []*packageFiles, cfg Config, enabled map[Rule]bool) *packageCache {
	if !cfg.CacheEnabled {
		return nil
	}
	return newPackageCache(cacheRootDir(pkgs, cfg), cfg, enabled)
}

type packageJob struct {
	pkg   *packageFiles
	check Check
}

func runPackageScopedChecks(pkgs []*packageFiles, cfg Config, enabled map[Rule]bool, cache packageCheckCache) []Issue {
	jobs := make([]packageJob, 0)
	for _, check := range checkRegistry {
		if !registryEnabled(check, enabled) || check.Scope != ScopePackage || check.RunPackage == nil {
			continue
		}
		for _, pkg := range pkgs {
			jobs = append(jobs, packageJob{pkg: pkg, check: check})
		}
	}
	return executePackageJobs(jobs, cfg, cache)
}

func executePackageJobs(jobs []packageJob, cfg Config, cache packageCheckCache) []Issue {
	if len(jobs) == 0 {
		return nil
	}
	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		workers = 1
	}
	if workers > len(jobs) {
		workers = len(jobs)
	}
	jobCh := make(chan packageJob)
	type jobResult struct {
		issues []Issue
		done   bool
	}
	resultCh := make(chan jobResult, workers)
	for n := 0; n < workers; n++ {
		go func() {
			for job := range jobCh {
				var local []Issue
				if cached, ok := cache.load(job.pkg, job.check.ID); ok {
					local = cached
				} else {
					local = job.check.RunPackage(job.pkg, cfg)
					cache.store(job.pkg, job.check.ID, local)
				}
				for index := range local {
					local[index].analysisRoot = job.pkg.analysisRoot
				}
				resultCh <- jobResult{issues: local}
			}
			resultCh <- jobResult{done: true}
		}()
	}
	go func() {
		for _, job := range jobs {
			jobCh <- job
		}
		close(jobCh)
	}()
	var all []Issue
	finished := 0
	for finished < workers {
		result := <-resultCh
		if result.done {
			finished++
			continue
		}
		all = append(all, result.issues...)
	}
	return all
}

func runProgramScopedChecks(pkgs []*packageFiles, cfg Config, enabled map[Rule]bool) []Issue {
	var all []Issue
	for _, check := range checkRegistry {
		if !registryEnabled(check, enabled) || check.Scope != ScopeProgram || check.RunProgram == nil {
			continue
		}
		all = append(all, check.RunProgram(pkgs, cfg)...)
	}
	return all
}

func reportRunDiagnostics(cfg Config, cache *packageCache, pkgs []*packageFiles) {
	if cfg.CacheDiagnostics && cache != nil {
		fmt.Fprintln(os.Stderr, "solidlint:", cache.diagnostics())
	}
	if cfg.CacheDiagnostics {
		for _, pkg := range pkgs {
			if pkg.filteredTypeErr != "" {
				fmt.Fprintf(os.Stderr, "solidlint: filtered type information incomplete for %s: %s\n", pkg.pkgPath, pkg.filteredTypeErr)
			}
		}
	}
}

func stampAnalysisRoots(issues []Issue, pkgs []*packageFiles) {
	root := canonicalRoot(pkgs)
	for index := range issues {
		if issues[index].analysisRoot == "" {
			issues[index].analysisRoot = root
		}
	}
}

func cacheRootDir(pkgs []*packageFiles, cfg Config) string {
	if cfg.CacheDir != "" {
		return cfg.CacheDir
	}
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	root := canonicalRoot(pkgs)
	sum := sha256.Sum256([]byte(root))
	return filepath.Join(base, "solidlint", fmt.Sprintf("%x", sum[:8]))
}

func sortIssues(all []Issue) {
	sort.Slice(all, func(i, j int) bool {
		if all[i].Pos.Filename != all[j].Pos.Filename {
			return all[i].Pos.Filename < all[j].Pos.Filename
		}
		if all[i].Pos.Line != all[j].Pos.Line {
			return all[i].Pos.Line < all[j].Pos.Line
		}
		if all[i].Pos.Column != all[j].Pos.Column {
			return all[i].Pos.Column < all[j].Pos.Column
		}
		if all[i].ID() != all[j].ID() {
			return all[i].ID() < all[j].ID()
		}
		return all[i].Evidence < all[j].Evidence
	})
}

func filterDisabledChecks(issues []Issue, disabled []CheckID) []Issue {
	if len(disabled) == 0 {
		return issues
	}
	set := make(map[string]bool, len(disabled))
	for _, id := range disabled {
		set[string(id)] = true
	}
	out := issues[:0]
	for _, issue := range issues {
		if set[issue.ID()] || set[string(issue.Rule)] {
			continue
		}
		out = append(out, issue)
	}
	return out
}

// applySuppressions accepts `//solidify:ignore RULE-ID justification` on the
// same line, immediately preceding a finding, or anywhere in the declaration
// header that owns the finding. Declaration-header matching lets one justified
// directive cover every parameter in a multi-line function signature without
// suppressing findings from the function body.
func applySuppressions(issues []Issue, pkgs []*packageFiles) []Issue {
	byFile, spansByFile := collectSuppressionMetadata(pkgs)
	out := issues[:0]
	for _, issue := range issues {
		if !issueSuppressed(issue, byFile, spansByFile) {
			out = append(out, issue)
		}
	}
	return out
}

type suppressionDirective struct {
	rule string
	line int
}

type declarationSpan struct {
	start int
	end   int
}

func collectSuppressionMetadata(pkgs []*packageFiles) (
	map[string][]suppressionDirective,
	map[string][]declarationSpan,
) {
	byFile := map[string][]suppressionDirective{}
	spansByFile := map[string][]declarationSpan{}
	for _, pkg := range pkgs {
		for _, f := range pkg.files {
			filename := pkg.fset.Position(f.Pos()).Filename
			spansByFile[filename] = append(spansByFile[filename], declarationHeaderSpans(pkg.fset, f)...)
			for directiveFilename, directives := range suppressionDirectives(pkg.fset, f) {
				byFile[directiveFilename] = append(byFile[directiveFilename], directives...)
			}
		}
	}
	return byFile, spansByFile
}

func declarationHeaderSpans(fset *token.FileSet, file *ast.File) []declarationSpan {
	var spans []declarationSpan
	for _, decl := range file.Decls {
		switch node := decl.(type) {
		case *ast.FuncDecl:
			spans = append(spans, functionDeclarationSpan(fset, node))
		case *ast.GenDecl:
			spans = append(spans, typeDeclarationSpans(fset, node)...)
		}
	}
	return spans
}

func functionDeclarationSpan(fset *token.FileSet, function *ast.FuncDecl) declarationSpan {
	start := fset.Position(function.Pos()).Line
	if function.Doc != nil {
		start = fset.Position(function.Doc.Pos()).Line
	}
	end := fset.Position(function.End()).Line
	if function.Body != nil {
		end = fset.Position(function.Body.Lbrace).Line
	}
	return declarationSpan{start: start, end: end}
}

func typeDeclarationSpans(fset *token.FileSet, declaration *ast.GenDecl) []declarationSpan {
	var spans []declarationSpan
	for _, spec := range declaration.Specs {
		typeSpec, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		start := fset.Position(typeSpec.Pos()).Line
		if declaration.Doc != nil {
			start = fset.Position(declaration.Doc.Pos()).Line
		}
		spans = append(spans, declarationSpan{
			start: start,
			end:   fset.Position(typeSpec.End()).Line,
		})
	}
	return spans
}

func suppressionDirectives(fset *token.FileSet, file *ast.File) map[string][]suppressionDirective {
	byFile := map[string][]suppressionDirective{}
	for _, group := range file.Comments {
		for _, comment := range group.List {
			text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
			parts := strings.Fields(text)
			if len(parts) < 3 || parts[0] != "solidify:ignore" {
				continue
			}
			position := fset.Position(comment.Pos())
			byFile[position.Filename] = append(byFile[position.Filename], suppressionDirective{
				rule: parts[1],
				line: position.Line,
			})
		}
	}
	return byFile
}

func issueSuppressed(
	issue Issue,
	byFile map[string][]suppressionDirective,
	spansByFile map[string][]declarationSpan,
) bool {
	if locationHasSuppression(issue, issue.Pos, byFile, spansByFile) {
		return true
	}
	for _, related := range issue.Related {
		if locationHasSuppression(issue, related.Pos, byFile, spansByFile) {
			return true
		}
	}
	return false
}

func locationHasSuppression(
	issue Issue,
	position token.Position,
	byFile map[string][]suppressionDirective,
	spansByFile map[string][]declarationSpan,
) bool {
	for _, directive := range byFile[position.Filename] {
		if directiveMatchesIssue(directive, issue) &&
			matchesSuppressionLocation(directive.line, position.Line, spansByFile[position.Filename]) {
			return true
		}
	}
	return false
}

func directiveMatchesIssue(directive suppressionDirective, issue Issue) bool {
	return directive.rule == issue.ID()
}

func matchesSuppressionLocation(directiveLine, findingLine int, spans []declarationSpan) bool {
	if suppressionMatchesLine(directiveLine, findingLine) {
		return true
	}
	for _, span := range spans {
		directiveInHeader := directiveLine >= span.start && directiveLine <= span.end
		if (directiveInHeader || directiveLine+1 == span.start) &&
			findingLine >= span.start && findingLine <= span.end {
			return true
		}
	}
	return false
}

func suppressionMatchesLine(directiveLine, findingLine int) bool {
	return directiveLine == findingLine || directiveLine+1 == findingLine
}

// ValidateSuppressions rejects broad or unexplained suppression directives
// before analysis output is produced.
func ValidateSuppressions(pkgs []*packageFiles) error {
	for _, pkg := range pkgs {
		for _, f := range pkg.files {
			for _, group := range f.Comments {
				for _, c := range group.List {
					text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
					if !strings.HasPrefix(text, "solidify:ignore") {
						continue
					}
					parts := strings.Fields(text)
					if len(parts) < 3 || parts[0] != "solidify:ignore" || !IsKnownCheckID(parts[1]) {
						return fmt.Errorf("%s:%d: suppression must name a specific rule ID and non-empty justification", pkg.fset.Position(c.Pos()).Filename, pkg.fset.Position(c.Pos()).Line)
					}
				}
			}
		}
	}
	return nil
}
