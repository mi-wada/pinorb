package pinner

import (
	"context"
	"testing"

	"github.com/mi-wada/pinorb/internal/orb"
)

// fakeResolver returns canned versions per orb name.
type fakeResolver map[string][]string

func (f fakeResolver) Versions(_ context.Context, name string) ([]orb.Version, error) {
	var out []orb.Version
	for _, raw := range f[name] {
		v, ok := orb.ParseVersion(raw)
		if !ok {
			panic("bad test version: " + raw)
		}
		out = append(out, v)
	}
	return out, nil
}

func TestPin(t *testing.T) {
	r := fakeResolver{
		"circleci/aws-cli":        {"5.4.1", "5.2.0", "5.1.2", "4.1.2"},
		"circleci/path-filtering": {"3.0.0", "2.1.0", "1.1.0"},
		"circleci/slack":          {"6.1.2", "5.1.1", "4.13.3"},
	}

	in := `version: 2.1
orbs:
  aws-cli: circleci/aws-cli@5.1   # keep comment
  path-filtering: circleci/path-filtering@3
  slack: circleci/slack@4.13.3
  cont: circleci/continuation@volatile

jobs:
  build:
    docker:
      - image: cimg/base:2024.01
`
	want := `version: 2.1
orbs:
  aws-cli: circleci/aws-cli@5.1.2   # keep comment
  path-filtering: circleci/path-filtering@3.0.0
  slack: circleci/slack@4.13.3
  cont: circleci/continuation@volatile

jobs:
  build:
    docker:
      - image: cimg/base:2024.01
`

	res, err := Pin(context.Background(), in, r, Options{})
	if err != nil {
		t.Fatalf("Pin: %v", err)
	}
	if res.Content != want {
		t.Errorf("Pin content mismatch:\n--- got ---\n%s\n--- want ---\n%s", res.Content, want)
	}
	if len(res.Changes) != 2 {
		t.Fatalf("got %d changes, want 2: %+v", len(res.Changes), res.Changes)
	}
}

// With Options{Update: true}, every numeric orb is bumped to its absolute
// latest version, ignoring the existing constraint and upgrading already-pinned
// orbs. Non-numeric tags are still left untouched.
func TestPinUpdate(t *testing.T) {
	r := fakeResolver{
		"circleci/aws-cli":        {"5.4.1", "5.2.0", "5.1.2", "4.1.2"},
		"circleci/path-filtering": {"3.0.0", "2.1.0", "1.1.0"},
		"circleci/slack":          {"6.1.2", "5.1.1", "4.13.3"},
	}

	in := `version: 2.1
orbs:
  aws-cli: circleci/aws-cli@5.1   # keep comment
  path-filtering: circleci/path-filtering@3
  slack: circleci/slack@4.13.3
  cont: circleci/continuation@volatile
`
	want := `version: 2.1
orbs:
  aws-cli: circleci/aws-cli@5.4.1   # keep comment
  path-filtering: circleci/path-filtering@3.0.0
  slack: circleci/slack@6.1.2
  cont: circleci/continuation@volatile
`

	res, err := Pin(context.Background(), in, r, Options{Update: true})
	if err != nil {
		t.Fatalf("Pin: %v", err)
	}
	if res.Content != want {
		t.Errorf("Pin content mismatch:\n--- got ---\n%s\n--- want ---\n%s", res.Content, want)
	}
	// aws-cli 5.1->5.4.1, path-filtering 3->3.0.0, slack 4.13.3->6.1.2.
	if len(res.Changes) != 3 {
		t.Fatalf("got %d changes, want 3: %+v", len(res.Changes), res.Changes)
	}
}

// An orb-like "name/orb@ver" string outside an orbs: block must be ignored.
func TestPinIgnoresOutsideOrbsBlock(t *testing.T) {
	r := fakeResolver{"circleci/aws-cli": {"5.4.1"}}
	in := `commands:
  foo:
    steps:
      - run: echo not-an/orb@5.1
`
	res, err := Pin(context.Background(), in, r, Options{})
	if err != nil {
		t.Fatalf("Pin: %v", err)
	}
	if len(res.Changes) != 0 {
		t.Fatalf("expected no changes, got %+v", res.Changes)
	}
}
