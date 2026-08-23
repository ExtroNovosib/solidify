package main

import (
	"runtime/debug"
	"testing"
)

func TestResolvedBuildInfoUsesModuleMetadataForGoInstall(t *testing.T) {
	originalVersion, originalCommit, originalBuildDate := version, commit, buildDate
	version, commit, buildDate = "dev", "unknown", "unknown"
	t.Cleanup(func() {
		version, commit, buildDate = originalVersion, originalCommit, originalBuildDate
	})

	got := resolvedBuildInfo(func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main: debug.Module{Version: "v0.1.0"},
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "abc123"},
				{Key: "vcs.time", Value: "2026-08-23T12:00:00Z"},
			},
		}, true
	})

	if got.Version != "v0.1.0" {
		t.Fatalf("version = %q, want v0.1.0", got.Version)
	}
	if got.Commit != "abc123" {
		t.Fatalf("commit = %q, want abc123", got.Commit)
	}
	if got.BuildDate != "2026-08-23T12:00:00Z" {
		t.Fatalf("build date = %q, want module VCS time", got.BuildDate)
	}
}

func TestResolvedBuildInfoKeepsLinkerValues(t *testing.T) {
	originalVersion, originalCommit, originalBuildDate := version, commit, buildDate
	version, commit, buildDate = "v0.1.0-release", "release-commit", "release-date"
	t.Cleanup(func() {
		version, commit, buildDate = originalVersion, originalCommit, originalBuildDate
	})

	got := resolvedBuildInfo(func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main: debug.Module{Version: "v0.1.0"},
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "module-commit"},
				{Key: "vcs.time", Value: "module-date"},
			},
		}, true
	})

	if got.Version != "v0.1.0-release" || got.Commit != "release-commit" || got.BuildDate != "release-date" {
		t.Fatalf("linker build info was overwritten: %+v", got)
	}
}

func TestResolvedBuildInfoHandlesUnavailableMetadata(t *testing.T) {
	originalVersion, originalCommit, originalBuildDate := version, commit, buildDate
	version, commit, buildDate = "dev", "unknown", "unknown"
	t.Cleanup(func() {
		version, commit, buildDate = originalVersion, originalCommit, originalBuildDate
	})

	got := resolvedBuildInfo(func() (*debug.BuildInfo, bool) { return nil, false })
	if got.Version != "dev" || got.Commit != "unknown" || got.BuildDate != "unknown" {
		t.Fatalf("fallback build info = %+v", got)
	}
}
