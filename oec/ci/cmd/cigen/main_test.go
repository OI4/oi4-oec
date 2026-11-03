package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCheck(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "workflows")
	if err := run(dir, true); err == nil {
		t.Fatal("-check passed without the directory")
	}
	if err := run(dir, false); err != nil {
		t.Fatal(err)
	}
	if err := run(dir, true); err != nil {
		t.Fatalf("-check after writing: %v", err)
	}
	gate := filepath.Join(dir, "gate.yml")
	b, _ := os.ReadFile(gate)
	if err := os.WriteFile(gate, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run(dir, true); err == nil || !strings.Contains(err.Error(), "gate.yml is outdated") {
		t.Errorf("-check with a changed file: %v", err)
	}
	if err := run(dir, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "other.yml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run(dir, true); err == nil || !strings.Contains(err.Error(), "other.yml is not generated") {
		t.Errorf("-check with a foreign file: %v", err)
	}
}
