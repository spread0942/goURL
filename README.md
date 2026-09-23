# goURL

A Go terminal API client built with [tview](https://github.com/rivo/tview).
Create and save HTTP requests, switch environments, and inspect responses without a browser.

## Run with Docker

No local Go installation or sudo is needed when your user can access Docker.

```sh
docker build -t gourl .
docker run --rm -it -e TERM -v gourl-data:/data gourl
```

Keep `-it`: this is an interactive terminal application, not a web server.
Requests and environments survive container removal in the `gourl-data` volume.
The image runs as UID 10001, includes HTTPS CA certificates, and exposes no ports.
Use `-e TERM=xterm-256color` if your terminal type is unavailable in the container.

On Linux, add `--network host` to reach APIs running on your host's `localhost`.
Alternatively use `--add-host=host.docker.internal:host-gateway` and call
`http://host.docker.internal:PORT`; the API must listen on a reachable host interface.
For a bind mount instead of a named volume, the directory must be writable by UID
10001 (or use `--user "$(id -u):$(id -g)"` with a directory you own). On SELinux
systems add `:Z` to that bind mount.

Rootless Podman is also supported:

```sh
podman build -t gourl .
podman run --rm -it -e TERM -v gourl-data:/data gourl
```

## Requests and Environments

- Requests support GET, POST, PUT, PATCH, DELETE, HEAD and OPTIONS.
- Edit a name, URL, headers, query parameters and raw body, then Save or Send.
- Headers use one `key: value` per line; query parameters use `key=value`.
  Repeated keys are supported. Blank lines are ignored; outer whitespace is trimmed.
- Open Environments, choose New environment, enter a name and one `key=value`
  variable per line, then Save & use. Existing environments can be renamed or deleted.
- Choose an environment in the request editor; choose None to disable variables.
- Use `{{variable}}` in URL, header keys/values, query keys/values or body.
  Missing variables prevent sending. Empty values are allowed. Expansion is
  single-pass, does not read OS variables, and never changes the saved template.
- Query parameters are URL-encoded. URL and body substitutions are literal:
  encode URL fragments and JSON-escape body values yourself when necessary.
- Tokens work through headers such as `Authorization: Bearer {{token}}`.
  Set `Content-Type: application/json` yourself when sending JSON.

Example environment:

```text
base_url=http://localhost:8080
token=your-token
search=hello world
```

Request URL: `{{base_url}}/items`. Query: `q={{search}}`.
Header: `Authorization: Bearer {{token}}`.

Responses include status, duration, headers and body. Valid JSON is formatted;
binary content gets a hex preview. Bodies are limited to 5 MiB and visibly marked
when truncated. Non-2xx responses are displayed normally. TLS verification stays
enabled and standard `HTTP_PROXY`, `HTTPS_PROXY`, and `NO_PROXY` settings apply.
Pass these through with Docker `-e` options if needed. Redirects follow Go's default
HTTP client behavior, including sensitive-header restrictions across domains.

## Controls

| Key | Action |
| --- | --- |
| Tab / Shift+Tab | Next / previous field or button |
| Ctrl+N | New request |
| Ctrl+S | Save request; save and select environment inside its editor |
| Ctrl+D | Duplicate request |
| Ctrl+R / F5 | Send request |
| Ctrl+X | Cancel running request |
| Ctrl+E | Environments |
| F2 | Saved requests; Enter opens selected request |
| F3 / F4 | Request / response view |
| Ctrl+Q / Ctrl+C | Quit, confirming unsaved request changes |
| Escape | Back from environment dialogs |

Mouse navigation and bracketed paste are enabled. Arrow keys scroll responses.
On terminals narrower than 80 columns, F2 opens the saved-request list full-width;
F3 and F4 return to the editor and response. Forms scroll as focus moves.
One request can run at a time; the interface remains usable while waiting.
Default timeout is 30 seconds; append `--timeout 60s` to the Docker run command
after the image name to change it.

## Build and Test Without Installing Go

All Makefile targets use a Go container and keep generated files owned by your
user. Only Docker (or Podman) and make are needed on the host.

```sh
make build       # builds ./gourl
make test        # tests with the race detector
make vet
make fmt
make image
make run
```

For rootless Podman, pass `ENGINE=podman` and configure user mapping as appropriate
for your host. The build cache is stored in the ignored `.cache/` directory.
Without make:

```sh
docker run --rm --user "$(id -u):$(id -g)" \
  -v "$PWD:/workspace:z" -w /workspace \
  -e GOCACHE=/workspace/.cache/build -e GOPATH=/workspace/.cache/go \
  golang:1.26-bookworm go test -race ./...
```

With Go 1.26 or later already installed:

```sh
go run ./cmd/gourl
go build -o gourl ./cmd/gourl
./gourl --data-dir ./data --timeout 30s
```

## Storage and Scope

The default local directory is the OS user config directory plus `gourl`
(normally `$XDG_CONFIG_HOME/gourl` or `~/.config/gourl` on Linux). Docker uses `/data`.
The versioned `state.json` is saved atomically with mode `0600`; newly created
directories use `0700`. Use one running instance per data directory.

**Values, including tokens, are stored in plaintext, not encrypted.** Keep the
data directory private, do not commit it, and protect backups and Docker volumes.
Existing directory permissions are not changed automatically. Corrupt or unsupported
state files stop startup without being replaced. Responses are not persisted.

This version does not include OAuth flows, scripting, file uploads, nested
collections, or Postman import/export.