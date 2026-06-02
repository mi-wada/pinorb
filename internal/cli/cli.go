// Package cli implements the pinorb command-line interface.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mi-wada/pinorb/internal/pinner"
	"github.com/mi-wada/pinorb/internal/registry"
)

const defaultConfigPath = ".circleci/config.yml"

const usage = `pinorb pins CircleCI orb versions to an exact patch version.

Usage:
  pinorb run [flags] [files...]

If no files are given, .circleci/config.yml is used.

Flags:
  --token string   CircleCI API token (or set $CIRCLE_TOKEN / $CIRCLECI_TOKEN).
                   Required for private orbs and to avoid registry rate limits.
  --check          Don't write changes; exit non-zero if any file is not pinned.
`

// Main is the entry point. It returns a process exit code.
func Main(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	switch args[0] {
	case "run":
		return runCmd(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "pinorb: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

func runCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }
	token := fs.String("token", "", "CircleCI API token")
	check := fs.Bool("check", false, "verify only; do not write")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	files := fs.Args()
	if len(files) == 0 {
		files = []string{defaultConfigPath}
	}

	if *token == "" {
		*token = firstEnv("CIRCLE_TOKEN", "CIRCLECI_TOKEN")
	}

	client := registry.New(*token)
	ctx := context.Background()

	unpinned := false
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(stderr, "pinorb: %v\n", err)
			return 1
		}

		res, err := pinner.Pin(ctx, string(src), client)
		if err != nil {
			fmt.Fprintf(stderr, "pinorb: %s: %v\n", path, err)
			return 1
		}

		if len(res.Changes) == 0 {
			continue
		}
		unpinned = true

		for _, c := range res.Changes {
			fmt.Fprintf(stdout, "%s:%d %s @%s -> @%s\n", path, c.Line, c.Name, c.From, c.To)
		}

		if !*check {
			if err := os.WriteFile(path, []byte(res.Content), 0o644); err != nil {
				fmt.Fprintf(stderr, "pinorb: %v\n", err)
				return 1
			}
		}
	}

	if *check && unpinned {
		fmt.Fprintln(stderr, "pinorb: some orbs are not pinned (run `pinorb run` to fix)")
		return 1
	}
	return 0
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}
