package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type harness struct {
	name  string
	roots []string
	items []string
}

type target struct {
	harness string
	path    string
	size    int64
}

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot determine home directory:", err)
		os.Exit(1)
	}

	harnesses := definitions(home)
	fmt.Print(`
   ___  ____  ___  ____  ___ ___  ____  ____  ___
  / _ \/ __ \/ _ \/ __ \/ __/ _ \/ __ \/ __/ (_-<
  \___/ .__/\___/_/ /_/\__/\___/_/ /_/\__/_/___/
     /_/

`)
	fmt.Println("opensessions (credentials and configuration are preserved)")
	fmt.Println("Commands: scan [name], clean <name|all>, help, quit")

	in := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("cleaner> ")
		if !in.Scan() {
			break
		}
		fields := strings.Fields(in.Text())
		if len(fields) == 0 {
			continue
		}
		switch strings.ToLower(fields[0]) {
		case "help":
			fmt.Println("scan [name]       show removable artifacts")
			fmt.Println("clean <name|all>  review and remove artifacts")
			fmt.Println("quit              exit")
		case "scan":
			name := "all"
			if len(fields) > 1 {
				name = strings.ToLower(fields[1])
			}
			show(discover(selectHarnesses(harnesses, name), home))
		case "clean":
			if len(fields) != 2 {
				fmt.Println("usage: clean <codex|opencode|antigravity|all>")
				continue
			}
			selected := selectHarnesses(harnesses, strings.ToLower(fields[1]))
			if selected == nil {
				fmt.Println("unknown harness:", fields[1])
				continue
			}
			clean(in, discover(selected, home))
		case "quit", "exit":
			return
		default:
			fmt.Println("unknown command; type help")
		}
	}
	if err := in.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "input error:", err)
	}
}

func definitions(home string) []harness {
	config := envOr("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	cache := envOr("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	data := envOr("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	state := envOr("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))

	return []harness{
		{
			name:  "codex",
			roots: unique(envOr("CODEX_HOME", filepath.Join(home, ".codex"))),
			items: []string{"cache", "tmp", ".tmp", "sessions", "shell_snapshots", "thread-writer-locks", "history.jsonl", "session_index.jsonl", "memories_1.sqlite", "logs_2.sqlite", "logs_2.sqlite-shm", "logs_2.sqlite-wal", "thread_history_1.sqlite", "thread_history_1.sqlite-shm", "thread_history_1.sqlite-wal", "queue_1.sqlite", "queue_1.sqlite-shm", "queue_1.sqlite-wal", "goals_1.sqlite", "goals_1.sqlite-shm", "goals_1.sqlite-wal"},
		},
		{
			name: "opencode",
			roots: unique(
				envOr("OPENCODE_DATA_HOME", filepath.Join(data, "opencode")),
				envOr("OPENCODE_STATE_HOME", filepath.Join(state, "opencode")),
				envOr("OPENCODE_CACHE_HOME", filepath.Join(cache, "opencode")),
			),
			items: []string{"snapshot", "shell", "storage", "tool-output", "log", "repos", "opencode.db", "opencode.db-shm", "opencode.db-wal", "session.json", "prompt-history.jsonl", "prompt-stash.jsonl", "frecency.jsonl", "locks", "beta/locks", "bin"},
		},
		{
			name: "antigravity",
			roots: unique(
				os.Getenv("ANTIGRAVITY_HOME"),
				filepath.Join(home, ".antigravity"),
				filepath.Join(home, ".gemini", "antigravity"),
				filepath.Join(config, "antigravity"),
				filepath.Join(cache, "antigravity"),
				filepath.Join(data, "antigravity"),
				filepath.Join(state, "antigravity"),
			),
			items: []string{"cache", "caches", "tmp", "temp", "sessions", "history", "logs", "memory", "memories", "snapshots", "workspaceStorage"},
		},
	}
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func unique(paths ...string) []string {
	seen := map[string]bool{}
	var result []string
	for _, path := range paths {
		if path == "" {
			continue
		}
		path = filepath.Clean(path)
		if !seen[path] {
			seen[path] = true
			result = append(result, path)
		}
	}
	return result
}

func selectHarnesses(all []harness, name string) []harness {
	if name == "all" {
		return all
	}
	for _, h := range all {
		if h.name == name {
			return []harness{h}
		}
	}
	return nil
}

func discover(harnesses []harness, home string) []target {
	var found []target
	seen := map[string]bool{}
	for _, h := range harnesses {
		for _, root := range h.roots {
			if !safeRoot(root, home) {
				fmt.Fprintf(os.Stderr, "warning: ignoring unsafe root %q\n", root)
				continue
			}
			for _, item := range h.items {
				path := filepath.Join(root, item)
				if seen[path] {
					continue
				}
				if _, err := os.Lstat(path); err != nil {
					continue
				}
				seen[path] = true
				found = append(found, target{harness: h.name, path: path, size: treeSize(path)})
			}
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].path < found[j].path })
	return found
}

func safeRoot(root, home string) bool {
	absRoot, err1 := filepath.Abs(root)
	absHome, err2 := filepath.Abs(home)
	if err1 != nil || err2 != nil || absRoot == string(filepath.Separator) || absRoot == absHome {
		return false
	}
	rel, err := filepath.Rel(absHome, absRoot)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func treeSize(path string) int64 {
	var total int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total
}

func show(targets []target) {
	if len(targets) == 0 {
		fmt.Println("No removable artifacts found.")
		return
	}
	var total int64
	for _, t := range targets {
		fmt.Printf("%-12s %8s  %s\n", t.harness, humanSize(t.size), t.path)
		total += t.size
	}
	fmt.Printf("%d artifact paths, %s total\n", len(targets), humanSize(total))
}

func clean(in *bufio.Scanner, targets []target) {
	show(targets)
	if len(targets) == 0 {
		return
	}
	fmt.Print("Type DELETE to permanently remove exactly these paths: ")
	if !in.Scan() || in.Text() != "DELETE" {
		fmt.Println("Cancelled.")
		return
	}
	removed := 0
	for _, t := range targets {
		if err := os.RemoveAll(t.path); err != nil {
			fmt.Fprintf(os.Stderr, "failed: %s: %v\n", t.path, err)
			continue
		}
		removed++
	}
	fmt.Printf("Removed %d of %d artifact paths. Restart active harnesses before rescanning.\n", removed, len(targets))
}

func humanSize(n int64) string {
	units := []string{"B", "KiB", "MiB", "GiB"}
	v := float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}
