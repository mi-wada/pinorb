package orb

import "testing"

func mustVersions(t *testing.T, raws ...string) []Version {
	t.Helper()
	vs := make([]Version, 0, len(raws))
	for _, r := range raws {
		v, ok := ParseVersion(r)
		if !ok {
			t.Fatalf("ParseVersion(%q) failed", r)
		}
		vs = append(vs, v)
	}
	return vs
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		in     string
		ok     bool
		pinned bool
	}{
		{"3", true, false},
		{"3.1", true, false},
		{"3.1.0", true, true},
		{"latest", true, false},
		{"volatile", false, false},
		{"dev:abc123", false, false},
		{"1.2.3.4", false, false},
		{"", false, false},
	}
	for _, tt := range tests {
		v, ok := ParseVersion(tt.in)
		if ok != tt.ok {
			t.Errorf("ParseVersion(%q) ok = %v, want %v", tt.in, ok, tt.ok)
			continue
		}
		if ok && v.IsPinned() != tt.pinned {
			t.Errorf("ParseVersion(%q).IsPinned() = %v, want %v", tt.in, v.IsPinned(), tt.pinned)
		}
	}
}

func TestResolve(t *testing.T) {
	cands := mustVersions(t,
		"3.0.0", "2.1.0", "2.0.4", "2.0.3", "1.3.0", "1.2.1", "1.2.0",
	)

	tests := []struct {
		spec string
		want string
	}{
		{"3", "3.0.0"},
		{"2", "2.1.0"},
		{"2.0", "2.0.4"},
		{"1.2", "1.2.1"},
		{"latest", "3.0.0"},
	}
	for _, tt := range tests {
		spec, ok := ParseVersion(tt.spec)
		if !ok {
			t.Fatalf("ParseVersion(%q) failed", tt.spec)
		}
		got, err := spec.Resolve(cands)
		if err != nil {
			t.Errorf("Resolve(%q): %v", tt.spec, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Resolve(%q) = %q, want %q", tt.spec, got, tt.want)
		}
	}
}

func TestResolveNoMatch(t *testing.T) {
	cands := mustVersions(t, "1.0.0", "2.0.0")
	spec, _ := ParseVersion("9")
	if _, err := spec.Resolve(cands); err == nil {
		t.Fatal("expected error for unmatched major version")
	}
}

func TestParseLine(t *testing.T) {
	tests := []struct {
		line      string
		ok        bool
		name, ver string
		prefix    string
		suffix    string
	}{
		{"  aws-cli: circleci/aws-cli@5.1", true, "circleci/aws-cli", "5.1", "  aws-cli: ", ""},
		{"  slack: circleci/slack@4.13.3  # notify", true, "circleci/slack", "4.13.3", "  slack: ", "  # notify"},
		{"  acme: acme-org/deploy-tools@1.2", true, "acme-org/deploy-tools", "1.2", "  acme: ", ""},
		{"version: 2.1", false, "", "", "", ""},
		{"jobs:", false, "", "", "", ""},
	}
	for _, tt := range tests {
		ref, prefix, suffix, ok := ParseLine(tt.line)
		if ok != tt.ok {
			t.Errorf("ParseLine(%q) ok = %v, want %v", tt.line, ok, tt.ok)
			continue
		}
		if !ok {
			continue
		}
		if ref.Name != tt.name || ref.Version != tt.ver || prefix != tt.prefix || suffix != tt.suffix {
			t.Errorf("ParseLine(%q) = {name:%q ver:%q prefix:%q suffix:%q}, want {name:%q ver:%q prefix:%q suffix:%q}",
				tt.line, ref.Name, ref.Version, prefix, suffix, tt.name, tt.ver, tt.prefix, tt.suffix)
		}
	}
}
