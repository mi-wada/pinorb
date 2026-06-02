package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCollectFiles(t *testing.T) {
	dir := t.TempDir()
	// Split "setup workflow" layout under .circleci/.
	mustWrite(t, filepath.Join(dir, ".circleci/config.yml"), "version: 2.1\n")
	mustWrite(t, filepath.Join(dir, ".circleci/config/orbs.yml"), "orbs:\n")
	mustWrite(t, filepath.Join(dir, ".circleci/config/jobs.yaml"), "jobs:\n")
	mustWrite(t, filepath.Join(dir, ".circleci/README.md"), "ignore me\n")

	chdir(t, dir)

	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "default walks .circleci recursively",
			in:   nil,
			want: []string{
				".circleci/config.yml",
				".circleci/config/jobs.yaml",
				".circleci/config/orbs.yml",
			},
		},
		{
			name: "explicit directory",
			in:   []string{".circleci/config"},
			want: []string{
				".circleci/config/jobs.yaml",
				".circleci/config/orbs.yml",
			},
		},
		{
			name: "explicit file",
			in:   []string{".circleci/config/orbs.yml"},
			want: []string{".circleci/config/orbs.yml"},
		},
		{
			name: "dedupes file already covered by dir",
			in:   []string{".circleci/config", ".circleci/config/orbs.yml"},
			want: []string{
				".circleci/config/jobs.yaml",
				".circleci/config/orbs.yml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := collectFiles(tt.in)
			if err != nil {
				t.Fatalf("collectFiles: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collectFiles(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestCollectFilesFallsBackToConfigYml(t *testing.T) {
	chdir(t, t.TempDir()) // no .circleci/ present
	got, err := collectFiles(nil)
	if err != nil {
		t.Fatalf("collectFiles: %v", err)
	}
	want := []string{defaultConfigPath}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("collectFiles(nil) = %v, want %v", got, want)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(prev) })
}
