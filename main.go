package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
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

// ANSI colors (stdlib only, no dependencies). Disabled with NO_COLOR=1,
// TERM=dumb, or non-terminal output.
var useColor = supportsColor()

const (
	cReset   = "0"
	cBold    = "1"
	cRed     = "31"
	cGreen   = "32"
	cYellow  = "33"
	cBlue    = "34"
	cMagenta = "35"
	cCyan    = "36"
	cGray    = "90"
)

func paint(code, s string) string {
	if !useColor || s == "" {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func bold(s string) string   { return paint(cBold, s) }
func red(s string) string    { return paint(cRed, s) }
func yellow(s string) string { return paint(cYellow, s) }
func gray(s string) string   { return paint(cGray, s) }

func harnessColor(name string) func(string) string {
	switch name {
	case "codex":
		return func(s string) string { return paint(cGreen, s) }
	case "opencode":
		return func(s string) string { return paint(cBlue, s) }
	case "antigravity":
		return func(s string) string { return paint(cMagenta, s) }
	default:
		return func(s string) string { return paint(cCyan, s) }
	}
}

// Size buckets: dim for trivia, yellow when noticeable, red when heavy.
func sizeColor(n int64) func(string) string {
	const (
		mib = 1024 * 1024
	)
	switch {
	case n >= 100*mib:
		return func(s string) string { return paint(cRed, s) }
	case n >= 10*mib:
		return func(s string) string { return paint(cYellow, s) }
	default:
		return func(s string) string { return paint(cGray, s) }
	}
}

func supportsColor() bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, red("cannot determine home directory:"), err)
		os.Exit(1)
	}

	harnesses := definitions(home)
	fmt.Print(paint(cCyan, `
   ___  ____  ___  ____  ___ ___  ____  ____  ___
  / _ \/ __ \/ _ \/ __ \/ __/ _ \/ __ \/ __/ (_-<
  \___/ .__/\___/_/ /_/\__/\___/_/ /_/\__/_/___/
     /_/

`))
	fmt.Println(bold("opensessions") + gray(" (credentials and configuration are preserved)"))
	fmt.Println(gray("Commands: ") + bold("scan [name]") + gray(", ") + bold("clean <name|all>") + gray(", ") + bold("help") + gray(", ") + bold("quit"))

	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1024*1024), 1024*1024)
	for {
		fmt.Print(bold("cleaner> "))
		if !in.Scan() {
			break
		}
		fields := strings.Fields(in.Text())
		if len(fields) == 0 {
			continue
		}
		switch strings.ToLower(fields[0]) {
		case "help":
			fmt.Println(bold("scan [name]") + "       show removable artifacts")
			fmt.Println(bold("clean <name|all>") + "  review and remove artifacts")
			fmt.Println(bold("quit") + "              exit")
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
				fmt.Println(red("unknown harness:"), fields[1])
				continue
			}
			clean(in, discover(selected, home))
		case "quit", "exit":
			return
		default:
			fmt.Println("unknown command; type " + bold("help"))
		}
	}
	if err := in.Err(); err != nil {
		fmt.Fprintln(os.Stderr, red("input error:"), err)
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
	type candidate struct {
		harness string
		path    string
	}
	var candidates []candidate
	seen := map[string]bool{}
	for _, h := range harnesses {
		for _, root := range h.roots {
			if !safeRoot(root, home) {
				fmt.Fprintf(os.Stderr, "%s: ignoring unsafe root %q\n", yellow("warning"), root)
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
				candidates = append(candidates, candidate{harness: h.name, path: path})
			}
		}
	}

	// Size every tree concurrently: directory walks dominate scan time and
	// are independent, so fan out over a worker pool instead of walking
	// each target one after another.
	found := make([]target, len(candidates))
	workers := min(len(candidates), 4*runtime.NumCPU())
	if workers < 1 {
		return nil
	}
	var wg sync.WaitGroup
	jobs := make(chan int)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				found[i] = target{
					harness: candidates[i].harness,
					path:    candidates[i].path,
					size:    treeSize(candidates[i].path),
				}
			}
		}()
	}
	for i := range candidates {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

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

// WalkDir (not Walk) avoids a stat call per directory; only regular files
// contribute to the total.
func treeSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.Type().IsRegular() {
			if info, err := entry.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

func show(targets []target) {
	if len(targets) == 0 {
		fmt.Println(gray("No removable artifacts found."))
		return
	}
	var total int64
	for _, t := range targets {
		size := sizeColor(t.size)(humanSize(t.size))
		fmt.Printf("%s %8s  %s\n", harnessColor(t.harness)(fmt.Sprintf("%-12s", t.harness)), size, t.path)
		total += t.size
	}
	fmt.Printf("%s, %s total\n",
		bold(fmt.Sprintf("%d artifact paths", len(targets))),
		bold(humanSize(total)))
}

func clean(in *bufio.Scanner, targets []target) {
	show(targets)
	if len(targets) == 0 {
		return
	}
	fmt.Print(bold(red("Type DELETE to permanently remove exactly these paths: ")))
	if !in.Scan() || in.Text() != "DELETE" {
		fmt.Println(gray("Cancelled."))
		return
	}
	removed := 0
	for _, t := range targets {
		if err := os.RemoveAll(t.path); err != nil {
			fmt.Fprintf(os.Stderr, "%s %s: %v\n", red("failed:"), t.path, err)
			continue
		}
		fmt.Printf("%s  %s\n", paint(cGreen, "removed"), t.path)
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
