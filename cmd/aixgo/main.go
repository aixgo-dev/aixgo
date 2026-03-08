package main

import (
	"github.com/aixgo-dev/aixgo/cmd/aixgo/cmd"
)

var (
	// Version information (set via ldflags)
	Version = "dev"
)

func main() {
	cmd.SetVersion(Version)
	cmd.Execute()
}
