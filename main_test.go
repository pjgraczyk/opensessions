package main

import (
	"fmt"
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

func BenchmarkDiscover(b *testing.B) {
	home := b.TempDir()
	var items []string
	for i := range 8 {
		dir := filepath.Join(home, ".tool", "sessions", string(rune('a'+i)))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			b.Fatal(err)
		}
		for j := range 50 {
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%03d.bin", j)), make([]byte, 1024), 0o600); err != nil {
				b.Fatal(err)
			}
		}
		items = append(items, filepath.Join("sessions", string(rune('a'+i))))
	}
	h := []harness{{name: "tool", roots: []string{filepath.Join(home, ".tool")}, items: items}}
	b.ResetTimer()
	for range b.N {
		if got := discover(h, home); len(got) != 8 {
			b.Fatalf("unexpected targets: %d", len(got))
		}
	}
}
