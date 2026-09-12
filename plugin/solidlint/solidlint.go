// Package solidlint provides the recommended GolangCI module-plugin adapter.
package solidlint

import (
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/ExtroNovosib/solidify/internal/analysisapi"
)

// init registers the adapter through the module-plugin registry at package load.
//
//nolint:gochecknoinits // The plugin-module registry discovers adapters through registration side effects.
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
