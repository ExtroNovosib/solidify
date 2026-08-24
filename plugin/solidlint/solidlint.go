// Package solidlint provides the recommended GolangCI module-plugin adapter.
package solidlint

import (
	"github.com/ExtroNovosib/solidify/internal/analysisapi"
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
)

func init() {
	register.Plugin("solidlint", New)
}

type Plugin struct {
	analyzers []*analysis.Analyzer
}

func New(settings any) (register.LinterPlugin, error) {
	analyzers, err := analysisapi.NewAnalyzers(settings)
	if err != nil {
		return nil, err
	}
	return &Plugin{analyzers: analyzers}, nil
}

func (p *Plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return append([]*analysis.Analyzer(nil), p.analyzers...), nil
}

func (*Plugin) GetLoadMode() string { return register.LoadModeTypesInfo }
