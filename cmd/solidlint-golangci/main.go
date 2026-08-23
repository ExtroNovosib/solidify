//go:build plugin

package main

import (
	"github.com/ExtroNovosib/solidify/internal/analysisapi"
	"golang.org/x/tools/go/analysis"
)

// New is the symbol required by GolangCI-Lint's Go-plugin ABI.
func New(conf any) ([]*analysis.Analyzer, error) {
	return analysisapi.NewAnalyzers(conf)
}
