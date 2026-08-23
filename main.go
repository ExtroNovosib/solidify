package main

import (
	"os"

	"github.com/ExtroNovosib/solidify/internal/cli"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], cli.BuildInfo{
		Version: version, Commit: commit, BuildDate: buildDate,
	}))
}
