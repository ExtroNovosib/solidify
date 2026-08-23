package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCanonicalProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".solidify.yml")
	if err := os.WriteFile(path, []byte("profile: stable\nenabled_checks: [SOLID-S/complex-function]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Profile != "stable" || len(loaded.EnabledChecks) != 1 {
		t.Fatalf("loaded = %+v", loaded)
	}
}
