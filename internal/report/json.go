// Package report owns machine-facing rendering entry points.
package report

import "github.com/ExtroNovosib/solidify/internal/analyzer"

func EncodeJSON(issues []analyzer.Issue) ([]byte, error) {
	return analyzer.EncodeIssuesJSON(issues)
}
