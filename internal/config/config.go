// Package config is the application-facing configuration boundary. The
// analyzer types are re-exported during the v0.1 extraction so callers do not
// need to depend on YAML parsing details.
package config

import "github.com/ExtroNovosib/solidify/internal/analyzer"

type File = analyzer.FileConfig

func Load(path string) (File, error)                  { return analyzer.LoadFileConfig(path) }
func FindForTargets(targets []string) (string, error) { return analyzer.FindConfigForTargets(targets) }
func ApplyThresholds(cfg *analyzer.Config, thresholds map[string]int) error {
	return analyzer.ApplyThresholds(cfg, thresholds)
}
