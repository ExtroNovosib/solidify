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
	settings any
}

func New(settings any) (register.LinterPlugin, error) {
	if _, err := analysisapi.NewAnalyzers(settings); err != nil {
		return nil, err
	}
	return &Plugin{settings: settings}, nil
}

func (p *Plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return analysisapi.NewAnalyzers(p.settings)
}

func (*Plugin) GetLoadMode() string { return register.LoadModeTypesInfo }
