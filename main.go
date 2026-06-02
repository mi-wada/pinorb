package main

import (
	"os"

	"github.com/mi-wada/pinorb/internal/cli"
)

// version is stamped at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(cli.Main(version, os.Args[1:], os.Stdout, os.Stderr))
}
