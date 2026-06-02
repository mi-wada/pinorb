// Package cli implements the pinorb command-line interface.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mi-wada/pinorb/internal/pinner"
	"github.com/mi-wada/pinorb/internal/registry"
)

const (
	defaultConfigPath = ".circleci/config.yml"
	defaultConfigDir  = ".circleci"
)

const usage = `pinorb pins CircleCI orbs to an exact patch version.

Usage:
  pinorb run [flags] [paths...]
  pinorb version

Each path may be a file or a directory; directories are searched recursively
for *.yml / *.yaml. If no paths are given, the .circleci/ directory is searched
(falling back to .circleci/config.yml). This covers both single-file configs
and split "setup workflow" configs (.circleci/config/*.yml).

Flags:
  --token string   CircleCI API token (or set $CIRCLE_TOKEN / $CIRCLECI_TOKEN).
                   Required for private orbs and to avoid registry rate limits.
  --check          Don't write changes; exit non-zero if any file is not pinned.
`

// Main is the entry point. version is the build version stamped into the
// binary. It returns a process exit code.
func Main(version string, args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	switch args[0] {
	case "run":
		return runCmd(args[1:], stdout, stderr)
	case "version", "-v", "--version":
		fmt.Fprintf(stdout, "pinorb %s\n", version)
		return 0
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

	files, err := collectFiles(fs.Args())
	if err != nil {
		fmt.Fprintf(stderr, "pinorb: %v\n", err)
		return 1
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

// collectFiles expands the given paths into a de-duplicated, sorted list of
// config files. Directories are walked recursively for *.yml / *.yaml. With no
// paths, it defaults to the .circleci/ directory, falling back to
// .circleci/config.yml when that directory is absent.
func collectFiles(paths []string) ([]string, error) {
	if len(paths) == 0 {
		if fi, err := os.Stat(defaultConfigDir); err == nil && fi.IsDir() {
			paths = []string{defaultConfigDir}
		} else {
			// No .circleci/ at all: return the canonical path and let the
			// caller surface a clear "no such file" error on read.
			return []string{defaultConfigPath}, nil
		}
	}

	seen := map[string]bool{}
	var files []string
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			files = append(files, p)
		}
	}

	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !fi.IsDir() {
			add(p)
			continue
		}
		err = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if ext := strings.ToLower(filepath.Ext(path)); ext == ".yml" || ext == ".yaml" {
				add(path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	sort.Strings(files)
	return files, nil
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}
