// Package pinner rewrites CircleCI config files, pinning orbs to an exact
// patch version.
package pinner

import (
	"context"
	"regexp"
	"strings"

	"github.com/mi-wada/pinorb/internal/orb"
)

// Resolver resolves an orb's published versions. *registry.Client implements it.
type Resolver interface {
	Versions(ctx context.Context, name string) ([]orb.Version, error)
}

// Change records a single orb version that was (or would be) updated.
type Change struct {
	Line int // 1-based line number
	Name string
	From string
	To   string
}

// Result is the outcome of pinning one file's contents.
type Result struct {
	Content string
	Changes []Change
}

// Options tunes how Pin resolves orb versions.
type Options struct {
	// Update resolves every orb to its absolute latest released version,
	// ignoring the existing version constraint and upgrading already-pinned
	// versions. Non-numeric tags (e.g. "volatile") are still left untouched.
	Update bool
}

var orbsBlockStart = regexp.MustCompile(`^(\s*)orbs:\s*(?:#.*)?$`)

// indentWidth returns the number of leading whitespace characters.
func indentWidth(s string) int {
	return len(s) - len(strings.TrimLeft(s, " \t"))
}

// latestSpec matches any released version, so Resolve picks the highest.
var latestSpec, _ = orb.ParseVersion("latest")

// Pin rewrites content, resolving every partially-specified orb version inside
// `orbs:` blocks to its latest matching patch version. Already-pinned orbs and
// non-numeric versions (e.g. "volatile", "dev:...") are left untouched. With
// opts.Update, every numeric orb is instead resolved to its absolute latest
// released version (see Options.Update).
func Pin(ctx context.Context, content string, r Resolver, opts Options) (Result, error) {
	// Preserve the original line endings/structure by splitting on "\n".
	lines := strings.Split(content, "\n")
	var res Result

	inOrbs := false
	orbsIndent := 0

	for i, line := range lines {
		if m := orbsBlockStart.FindStringSubmatch(line); m != nil {
			inOrbs = true
			orbsIndent = len(m[1])
			continue
		}
		if !inOrbs {
			continue
		}

		trimmed := strings.TrimSpace(line)
		// Blank lines and comments don't end the block.
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// A line indented no deeper than `orbs:` ends the block.
		if indentWidth(line) <= orbsIndent {
			inOrbs = false
			continue
		}

		ref, prefix, suffix, ok := orb.ParseLine(line)
		if !ok {
			continue
		}
		spec, ok := orb.ParseVersion(ref.Version)
		if !ok {
			continue
		}
		if spec.IsPinned() && !opts.Update {
			continue
		}
		if opts.Update {
			spec = latestSpec
		}

		versions, err := r.Versions(ctx, ref.Name)
		if err != nil {
			return Result{}, err
		}
		resolved, err := spec.Resolve(versions)
		if err != nil {
			return Result{}, err
		}
		if resolved == ref.Version {
			continue
		}

		lines[i] = prefix + ref.Name + "@" + resolved + suffix
		res.Changes = append(res.Changes, Change{
			Line: i + 1,
			Name: ref.Name,
			From: ref.Version,
			To:   resolved,
		})
	}

	res.Content = strings.Join(lines, "\n")
	return res, nil
}
