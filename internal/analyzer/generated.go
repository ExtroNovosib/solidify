package analyzer

import "go/ast"

// skipGenerated reports whether analysis should skip a file. Workspace loads
// record ast.IsGenerated in packageFiles.generated; callers without a package
// fall back to the standard library helper.
func skipGenerated(pkg *packageFiles, f *ast.File) bool {
	if pkg != nil && pkg.generated != nil {
		return pkg.generated[f]
	}
	return ast.IsGenerated(f)
}
