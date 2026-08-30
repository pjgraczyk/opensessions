package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeRoot(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "Users", "test")
	for _, bad := range []string{"/", home, filepath.Dir(home)} {
		if safeRoot(bad, home) {
			t.Fatalf("safeRoot(%q) = true", bad)
		}
	}
	if !safeRoot(filepath.Join(home, ".codex"), home) {
		t.Fatal("expected child directory to be safe")
	}
}

func TestDiscoverOnlyListedItems(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".tool")
	if err := os.MkdirAll(filepath.Join(root, "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "auth.json"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	found := discover([]harness{{name: "tool", roots: []string{root}, items: []string{"sessions"}}}, home)
	if len(found) != 1 || found[0].path != filepath.Join(root, "sessions") {
		t.Fatalf("unexpected targets: %#v", found)
	}
}
