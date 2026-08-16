# OpenCode Go Quota Plugin

Go shared-library plugin for CLIProxyAPI. It provides a read-only Management Center page for OpenCode Go quota windows. Target CLIProxyAPI v7.2.130, standard plugin ABI v1.

## Layout

- `main.go`: C ABI exports, plugin registration, and authenticated quota route.
- `plugin_config.go`: validated plugin YAML and runtime config state.
- `page.go`: static Management Center shell; it must never embed quota data.
- `plugin_test.go`: ABI registration and management-page behavior.
- `internal/cliproxyconfig/`: protected CLIProxy config credential discovery.
- `internal/opencode/usage.go`: OpenCode `GET <base-url>/v1/usage` client.
- `internal/opencode/usage_test.go`: real HTTP behavior via `httptest`.
- `internal/pool/`: bounded bulk collection and safe result projection.
- `dist/`: generated `.so` and C header; ignored by Git.

## Toolchain

- Go 1.24 or newer.
- CGO and a C compiler are required for the shared-library build.
- YAML parsing uses the exact-pinned `gopkg.in/yaml.v3` dependency.

## Commands

```bash
make check                    # tests, go vet, shared-library build
make test                     # go test ./...
make lint                     # go vet ./...
make build                    # dist/opencode-go-quota.so
make clean                    # remove dist/
go test ./internal/opencode   # usage client only
go test .                     # plugin ABI and page only
```

Run `make check` before committing. Confirm ABI exports when changing `main.go`:

```bash
nm -D dist/opencode-go-quota.so | grep -E 'cliproxy_plugin_init|cliproxyPluginCall|cliproxyPluginFree|cliproxyPluginShutdown'
```

## Contracts

- Keep the library basename `opencode-go-quota`; CLIProxyAPI uses it as the plugin ID and config key.
- Preserve the exported C symbols and ABI version `1` in `main.go`.
- Preserve CLIProxyAPI JSON field casing such as `Name`, `StatusCode`, `Headers`, and `Body`.
- The Management Center resource path is `/status`, exposed by CLIProxyAPI at `/v0/resource/plugins/opencode-go-quota/status`.
- Quota JSON is authenticated at `/v0/management/plugins/opencode-go-quota/quotas`.
- OpenCode usage is `/zen/go/v1/usage`; avoid duplicating `/v1` when the compatible base URL already ends in `/v1`.
- HTTP 401 means usage unavailable. Never classify it as exhausted quota.
- Direct providers require HTTPS, standard port, host `opencode.ai`, and path `/zen/go` or `/zen/go/v1`; custom URLs require an explicit `provider_names` entry.

## Scope boundaries

- Read-only plugin. No routing mutation or automatic credential disabling until separately approved and canary-tested.
- No cookies, browser login, database, or New API access.
- Never render, log, persist, or return raw API keys to the browser.

## Pitfalls

- `-buildmode=c-shared` produces both `.so` and `.h`; both belong under ignored `dist/`.
- Plugin code runs in-process with CLIProxyAPI. A panic, exit, memory corruption, or leaked secret affects the proxy itself.
- CLIProxyAPI v7.2.130 resource routes are unauthenticated. Keep `/status` data-free; fetch quota only from the authenticated Management API route.
- `host.auth.list` and `host.auth.get_runtime` omit runtime API keys; `host.auth.get` requires a physical auth file. OpenAI-compatible key discovery therefore reads the protected CLIProxy config path.
- Do not edit `/home/duu/.hermes/hermes-agent/` for this plugin. CLIProxyAPI compatibility belongs in this repository.
