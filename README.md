# opensessions

A deliberately small interactive cleaner for local Codex, OpenCode, and
Antigravity session artifacts. It previews every target and requires typing
`DELETE`; authentication and configuration files are not targeted.

Build and run:

```sh
go build -o opensessions .
./opensessions
```

Useful commands are `scan`, `scan codex`, and `clean all`.

Output is colorized per harness with size highlighting (gray < 10 MiB,
yellow < 100 MiB, red above); set `NO_COLOR=1` or use a dumb terminal to
disable it. Scans size every artifact tree concurrently, so large session
stores resolve in roughly the time of the single slowest tree.

Recognized overrides include `CODEX_HOME`, `OPENCODE_DATA_HOME`,
`OPENCODE_STATE_HOME`, `OPENCODE_CACHE_HOME`, `ANTIGRAVITY_HOME`, and the
standard `XDG_CONFIG_HOME`, `XDG_CACHE_HOME`, `XDG_DATA_HOME`, and
`XDG_STATE_HOME` variables. For safety, roots outside the current user's home
directory are ignored.

Quit the relevant harness before cleaning so an open process does not recreate
or continue writing deleted databases.
