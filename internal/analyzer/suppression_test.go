package analyzer

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

func TestValidateSuppressions(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		wantErr      bool
	}{
		{"same-line", "package p\n type I interface { A(); B() } //solidify:ignore SOLID-I/fat-interface legacy API\n", false},
		{"missing-rule", "package p\n //solidify:ignore because\n type I interface{}\n", true},
		{"missing-justification", "package p\n //solidify:ignore SOLID-I/fat-interface\n type I interface{}\n", true},
		{"solidlint-alias", "package p\n //solidlint:ignore SOLID-I/fat-interface legacy API\n type I interface{}\n", false},
		{"rule-family", "package p\n //solidify:ignore SOLID-I legacy API\n type I interface{}\n", false},
		{"file-form", "package p\n //solidlint:ignore-file SOLID-S/large-type generated facade\n type I interface{}\n", false},
		{"file-form-solidify", "package p\n //solidify:ignore-file SOLID-D reviewed adapter package\n type I interface{}\n", false},
		{"file-form-missing-justification", "package p\n //solidlint:ignore-file SOLID-S/large-type\n type I interface{}\n", true},
		{"file-form-unknown-id", "package p\n //solidlint:ignore-file SOLID-X/whatever reason\n type I interface{}\n", true},
		{"unknown-id", "package p\n //solidlint:ignore SOLID-I/fat-interfaces legacy API\n type I interface{}\n", true},
		{"unknown-form", "package p\n //solidlint:ignore-all SOLID-I legacy API\n type I interface{}\n", true},
		{"unknown-form-solidify", "package p\n //solidify:ignorefile SOLID-I legacy API\n type I interface{}\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fset, files := parseSource(t, tc.source)
			pkg := &packageFiles{fset: fset, files: files}
			if (ValidateSuppressions([]*packageFiles{pkg}) != nil) != tc.wantErr {
				t.Fatalf("validation mismatch")
			}
		})
	}
}

func TestApplySuppressionsAtRelatedLocation(t *testing.T) {
	fset, files := parseSource(t, `package p
func first() {}
//solidify:ignore SOLID-O/type-dispatch visitor family
func second() {}
`)
	pkg := &packageFiles{fset: fset, files: files}
	issue := Issue{Rule: RuleOCP, Check: CheckOCPTypeDispatch, Pos: token.Position{Filename: "test.go", Line: 2, Column: 1}, Related: []RelatedLocation{{Pos: token.Position{Filename: "test.go", Line: 4, Column: 1}}}}
	if got := applySuppressions([]Issue{issue}, []*packageFiles{pkg}); len(got) != 0 {
		t.Fatalf("related-location suppression left findings: %v", got)
	}
}

func TestApplySuppressionsAcrossMultilineDeclarationHeader(t *testing.T) {
	fset, files := parseSource(t, `package p

func NewService(
	first *First,
	second *Second,
) *Service { //solidify:ignore SOLID-D/concrete-dependency composition root wiring
	return nil
}
`)
	pkg := &packageFiles{fset: fset, files: files}
	issues := []Issue{
		{Rule: RuleDIP, Check: CheckDIPConcreteDependency, Pos: token.Position{Filename: "test.go", Line: 4, Column: 2}},
		{Rule: RuleDIP, Check: CheckDIPConcreteDependency, Pos: token.Position{Filename: "test.go", Line: 5, Column: 2}},
	}
	if got := applySuppressions(issues, []*packageFiles{pkg}); len(got) != 0 {
		t.Fatalf("declaration-level suppression left findings: %v", got)
	}
}

const suppressionFatInterfaceFixture = `package p

%s
type Wide interface {
	A()
	B()
	C()
	D()
	E()
	F()
	G()
	H()
	I()
	J()
}
`

func fatInterfaceFindings(t *testing.T, directive string) []Issue {
	t.Helper()
	root := t.TempDir()
	writeModuleFixture(t, root, map[string]string{"p/p.go": fmt.Sprintf(suppressionFatInterfaceFixture, directive)})
	issues, _, pkgs := runAllChecks(t, root, DefaultConfig())
	if err := ValidateSuppressions(pkgs); err != nil {
		t.Fatalf("directive %q failed validation: %v", directive, err)
	}
	var found []Issue
	for _, issue := range issues {
		if issue.Check == CheckISPFatInterface {
			found = append(found, issue)
		}
	}
	return found
}

func TestApplySuppressionsFamilyDirective(t *testing.T) {
	if got := fatInterfaceFindings(t, "// Wide is a legacy API."); len(got) != 1 {
		t.Fatalf("control fixture fat-interface findings = %v, want one", got)
	}
	for _, directive := range []string{
		"//solidify:ignore SOLID-I legacy RPC API",
		"//solidlint:ignore SOLID-I legacy RPC API",
	} {
		if got := fatInterfaceFindings(t, directive); len(got) != 0 {
			t.Fatalf("family directive %q left findings: %v", directive, got)
		}
	}
	if got := fatInterfaceFindings(t, "//solidlint:ignore SOLID-D unrelated family"); len(got) != 1 {
		t.Fatalf("unrelated family suppressed fat-interface: %v", got)
	}
}

func TestApplySuppressionsSolidlintAlias(t *testing.T) {
	if got := fatInterfaceFindings(t, "//solidlint:ignore SOLID-I/fat-interface legacy RPC API"); len(got) != 0 {
		t.Fatalf("solidlint alias left findings: %v", got)
	}
	fset, files := parseSource(t, `package p
type I interface { A(); B() } //solidlint:ignore SOLID-I/fat-interface legacy API
`)
	pkg := &packageFiles{fset: fset, files: files}
	issue := Issue{Rule: RuleISP, Check: CheckISPFatInterface, Pos: token.Position{Filename: "test.go", Line: 2, Column: 6}}
	if got := applySuppressions([]Issue{issue}, []*packageFiles{pkg}); len(got) != 0 {
		t.Fatalf("same-line solidlint alias left findings: %v", got)
	}
}

func TestApplySuppressionsFileDirective(t *testing.T) {
	fset := token.NewFileSet()
	sources := map[string]string{
		"a.go": `package p

type First struct{}

//solidlint:ignore-file SOLID-S/large-type generated-style facade kept for compatibility

type Second struct{}
`,
		"b.go": `package p

type Third struct{}
`,
	}
	var files []*ast.File
	for _, name := range []string{"a.go", "b.go"} {
		file, err := parser.ParseFile(fset, filepath.Join("dir", name), sources[name], parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	pkg := &packageFiles{fset: fset, files: files}
	at := func(name string, line int) token.Position {
		return token.Position{Filename: filepath.Join("dir", name), Line: line, Column: 6}
	}
	largeFirst := Issue{Rule: RuleSRP, Check: CheckSRPLargeType, Pos: at("a.go", 3)}
	largeSecond := Issue{Rule: RuleSRP, Check: CheckSRPLargeType, Pos: at("a.go", 7)}
	otherCheck := Issue{Rule: RuleSRP, Check: CheckSRPLowCohesionType, Pos: at("a.go", 3)}
	otherFile := Issue{Rule: RuleSRP, Check: CheckSRPLargeType, Pos: at("b.go", 3)}
	relatedOnly := Issue{Rule: RuleSRP, Check: CheckSRPLargeType, Pos: at("b.go", 3), Related: []RelatedLocation{{Pos: at("a.go", 3)}}}

	got := applySuppressions([]Issue{largeFirst, largeSecond, otherCheck, otherFile, relatedOnly}, []*packageFiles{pkg})
	if len(got) != 3 {
		t.Fatalf("file directive result = %v, want low-cohesion in a.go plus both b.go findings", got)
	}
	if got[0].Check != CheckSRPLowCohesionType || got[1].Pos.Filename != at("b.go", 3).Filename || got[2].Pos.Filename != at("b.go", 3).Filename {
		t.Fatalf("file directive kept the wrong findings: %v", got)
	}
	if len(got[2].Related) != 1 {
		t.Fatalf("related-location-only finding was suppressed through the file directive: %v", got)
	}
}
