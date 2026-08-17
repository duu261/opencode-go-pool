# OpenCode Go Pool Plugin

Go shared-library plugin for CLIProxyAPI. It provides an account registry, quota dashboard, and explicit pool controls for OpenCode Go. Target CLIProxyAPI v7.2.130, standard plugin ABI v1.

## Layout

- `main.go`: C ABI exports, plugin registration, authenticated quota/account routes, and account projection.
- `plugin_config.go`: validated plugin YAML and runtime config state.
- `page.go`: static Management Center shell and account-control UI; it must never embed runtime account data.
- `plugin_test.go`: ABI registration and management-page behavior.
- `accounts_route_test.go`: account registry and merged quota route behavior.
- `internal/accounts/`: plaintext YAML account registry with atomic `0600` writes.
- `internal/cliproxyconfig/`: protected CLIProxy config credential discovery.
- `internal/opencode/usage.go`: OpenCode `GET <base-url>/v1/usage` client.
- `internal/opencode/usage_test.go`: real HTTP behavior via `httptest`.
- `internal/pool/`: bounded bulk collection and result projection.
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
go test ./internal/accounts   # account registry only
go test .                     # plugin ABI, routes, and page
go test -race ./...           # race detector
```

Run `make check` before committing. Confirm ABI exports when changing `main.go`:

```bash
nm -D dist/opencode-go-quota.so | grep -E 'cliproxy_plugin_init|cliproxyPluginCall|cliproxyPluginFree|cliproxyPluginShutdown'
```

## Contracts

- Keep the library basename `opencode-go-quota`; CLIProxyAPI uses it as the plugin ID and config key.
- Preserve exported C symbols and ABI version `1` in `main.go`.
- Preserve CLIProxyAPI JSON field casing such as `Name`, `StatusCode`, `Headers`, and `Body`.
- The Management Center resource path is `/status`, exposed at `/v0/resource/plugins/opencode-go-quota/status`.
- Quota JSON is authenticated at `/v0/management/plugins/opencode-go-quota/quotas`.
- Account data is authenticated at `/v0/management/plugins/opencode-go-quota/accounts`.
- Registry replacement requires the current revision and returns `409` for stale snapshots.
- Management Center persists remembered auth under `cli-proxy-auth`; the same-origin iframe reuses it rather than asking again.
- OpenCode usage is `/zen/go/v1/usage`; avoid duplicating `/v1` when the compatible base URL already ends in `/v1`.
- HTTP 401 means usage unavailable. Never classify it as exhausted quota.
- Direct providers require HTTPS, standard port, host `opencode.ai`, and path `/zen/go` or `/zen/go/v1`; custom URLs require an explicit `provider_names` entry.

## Scope boundaries

- Pool mutations are operator-initiated only and use CLIProxyAPI's authenticated `/openai-compatibility` Management API.
- Refresh the exact provider before replacing `api-key-entries`; never fall back to another provider name.
- Keep registry metadata saves separate from pool mutations. New accounts start parked, and active accounts must be disabled before deletion.
- No automatic credential disabling until separately approved and canary-tested.
- No cookies, browser account login, database, or New API access.
- Raw API keys and disposable account credentials may be persisted in the configured plaintext registry and returned only from the authenticated account route.

## Pitfalls

- `accounts_path` must be writable by the CLIProxyAPI process. The main config may remain read-only only if native Management API writes are not required; pool controls require CLIProxyAPI to persist provider changes.
- `-buildmode=c-shared` produces both `.so` and `.h`; both belong under ignored `dist/`.
- Plugin code runs in-process with CLIProxyAPI. A panic, exit, memory corruption, or leaked secret affects the proxy itself.
- CLIProxyAPI resource routes are unauthenticated. Keep `/status` data-free; fetch runtime data only from authenticated Management API routes.
- `host.auth.list` and `host.auth.get_runtime` omit runtime API keys; `host.auth.get` requires a physical auth file. OpenAI-compatible key discovery therefore reads the protected CLIProxy config path.
- Do not edit `/home/duu/.hermes/hermes-agent/` for this plugin. CLIProxyAPI compatibility belongs in this repository.
