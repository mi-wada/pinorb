// Package orb handles parsing and resolution of CircleCI orb references.
package orb

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Ref is a single orb declaration found in a config file, e.g.
//
//	aws-cli: circleci/aws-cli@5.1
//
// where Alias is "aws-cli", Name is "circleci/aws-cli" and Version is "5.1".
type Ref struct {
	Alias   string
	Name    string // "<namespace>/<orb>"
	Version string
}

// Version classifies an orb version string.
type Version struct {
	Raw    string
	Major  int
	Minor  int
	Patch  int
	hasMin bool
	hasPat bool
	latest bool // "latest"
}

// ParseVersion parses a version string. ok is false when the version is not a
// numeric semver-ish version we know how to resolve (e.g. "volatile" or
// "dev:abc123"); such versions are left untouched by the pinner.
func ParseVersion(s string) (Version, bool) {
	if s == "latest" {
		return Version{Raw: s, latest: true}, true
	}
	parts := strings.Split(s, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return Version{}, false
	}
	v := Version{Raw: s}
	nums := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return Version{}, false
		}
		nums[i] = n
	}
	v.Major = nums[0]
	if len(nums) >= 2 {
		v.Minor = nums[1]
		v.hasMin = true
	}
	if len(nums) >= 3 {
		v.Patch = nums[2]
		v.hasPat = true
	}
	return v, true
}

// IsPinned reports whether the version is already fixed to an exact patch
// version (major.minor.patch) and therefore needs no resolution.
func (v Version) IsPinned() bool {
	return v.hasPat
}

// matches reports whether a fully-specified candidate version is compatible
// with the (possibly partial) constraint v.
func (v Version) matches(c Version) bool {
	if v.latest {
		return true
	}
	if c.Major != v.Major {
		return false
	}
	if v.hasMin && c.Minor != v.Minor {
		return false
	}
	return true
}

// Resolve picks the highest candidate version compatible with the constraint v.
// candidates must be fully-specified versions. It returns the resolved version
// string (e.g. "3.2.1").
func (v Version) Resolve(candidates []Version) (string, error) {
	var best *Version
	for i := range candidates {
		c := candidates[i]
		if !c.hasPat || !v.matches(c) {
			continue
		}
		if best == nil || best.Less(c) {
			best = &candidates[i]
		}
	}
	if best == nil {
		return "", fmt.Errorf("no released version matching %q", v.Raw)
	}
	return best.Raw, nil
}

// Less reports whether v sorts before o by semver precedence.
func (v Version) Less(o Version) bool {
	if v.Major != o.Major {
		return v.Major < o.Major
	}
	if v.Minor != o.Minor {
		return v.Minor < o.Minor
	}
	return v.Patch < o.Patch
}

// orbLine matches an orb declaration line such as:
//
//	aws-cli: circleci/aws-cli@5.1   # comment
//
// capturing the indent+alias prefix, the orb name, the version, and any
// trailing whitespace/comment so the line can be rewritten in place.
var orbLine = regexp.MustCompile(`^(\s+[\w.-]+:\s+)([\w-]+/[\w.-]+)@([^\s#]+)(\s*(?:#.*)?)$`)

// ParseLine extracts an orb Ref from a single line, if the line is an orb
// declaration. The boolean matched parameters allow callers to rewrite the
// line: prefix + Name + "@" + Version + suffix reconstructs the original line.
func ParseLine(line string) (ref Ref, prefix, suffix string, ok bool) {
	m := orbLine.FindStringSubmatch(line)
	if m == nil {
		return Ref{}, "", "", false
	}
	alias := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(m[1]), ":"))
	return Ref{Alias: alias, Name: m[2], Version: m[3]}, m[1], m[4], true
}
