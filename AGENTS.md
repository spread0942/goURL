# goURL — agent notes

Terminal API client (Postman-like CLI) built with Go + [tview](https://github.com/rivo/tview). Users create/save HTTP requests, switch environments with `{{variables}}`, and inspect responses in a TUI.

## Layout

| Path | Role |
| --- | --- |
| `cmd/gourl` | CLI flags (`--data-dir`, `--timeout`), load/save gate, start UI |
| `internal/core` | Domain: state, persistence, pair parsing, variable resolve, HTTP execute |
| `internal/ui` | tview TUI: editor, environments, keybindings, async send |

## Non-goals (this version)

No OAuth, scripting, file uploads, nested collections, or Postman import/export. Do not add these unless explicitly requested.

## Build / test

Prefer Makefile (Docker/Podman Go container; no local Go required):

```sh
make test    # go test -race ./...
make vet
make fmt
make build   # ./gourl
make run     # interactive Docker TUI
```

With local Go 1.26+: `go test -race ./...` and `go run ./cmd/gourl`.

## Persistence

- Atomic `state.json` (version `1`), mode `0600`; data dir `0700`
- Default dir: OS config + `gourl`; Docker: `/data`
- Secrets stored **plaintext** — never commit `data/`, volumes, or tokens
- Responses are not persisted; one process per data directory

## Product invariants

- Methods: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS
- `{{var}}` expansion: single-pass, no OS env, missing vars block send
- Headers `key: value`, query `key=value` (repeats OK); env vars unique keys
- Body cap 5 MiB (`core.MaxBody`); TLS verify on; proxy env honored
- One in-flight request; cancel via Ctrl+X
