package analysisapi_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/ExtroNovosib/solidify/internal/analysisapi"
)

func TestISPAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analysisapi.ISPAnalyzer, "fat")
}

func TestISPAnalyzerClean(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analysisapi.ISPAnalyzer, "clean")
}

func TestISPAnalyzerStub(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analysisapi.ISPAnalyzer, "stub")
}

func TestISPAnalyzerStubSuggestedFix(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), analysisapi.ISPAnalyzer, "stub")
}

func TestISPAnalyzerGenerated(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analysisapi.ISPAnalyzer, "generated")
}

func TestISPAnalyzerIncompleteTypes(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analysisapi.ISPAnalyzer, "incomplete")
}

func TestDIPAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analysisapi.DIPAnalyzer, "concrete")
}

func TestDIPAnalyzerClean(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analysisapi.DIPAnalyzer, "clean")
}

func TestDIPAnalyzerGenerated(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analysisapi.DIPAnalyzer, "generated")
}

func TestDIPAnalyzerIncompleteTypes(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), analysisapi.DIPAnalyzer, "incomplete-dip")
}
