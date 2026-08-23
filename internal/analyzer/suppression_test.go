package analyzer

import (
	"go/token"
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
