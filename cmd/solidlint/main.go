package main

import (
	"os"
	"runtime/debug"

	"github.com/ExtroNovosib/solidify/internal/cli"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], resolvedBuildInfo(debug.ReadBuildInfo)))
}

func resolvedBuildInfo(read func() (*debug.BuildInfo, bool)) cli.BuildInfo {
	build := cli.BuildInfo{Version: version, Commit: commit, BuildDate: buildDate}
	info, ok := read()
	if !ok {
		return build
	}
	if build.Version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		build.Version = info.Main.Version
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			if build.Commit == "unknown" && setting.Value != "" {
				build.Commit = setting.Value
			}
		case "vcs.time":
			if build.BuildDate == "unknown" && setting.Value != "" {
				build.BuildDate = setting.Value
			}
		}
	}
	return build
}
