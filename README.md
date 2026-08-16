# OpenCode Go Quota

Read-only quota plugin for CLIProxyAPI. It targets the standard dynamic-library ABI shipped in CLIProxyAPI v7.2.130 and exposes a Management Center resource at `/v0/resource/plugins/opencode-go-quota/status`.

## Current scaffold

- Builds as a Linux `c-shared` plugin.
- Registers the `management_api` capability and a read-only status page.
- Includes a tested client for `GET <base-url>/v1/usage`.
- Treats HTTP 401 as usage unavailable, never as quota exhaustion.
- Does not yet discover CLIProxyAPI OpenAI-compatible credentials.

## Requirements

- Go 1.24 or newer.
- A C compiler for `-buildmode=c-shared`.
- CLIProxyAPI v7.2.130 or a compatible plugin ABI v1 build.

## Verify

```bash
make check
```

The plugin artifact is written to `dist/opencode-go-quota.so`. CLIProxyAPI discovers plugins from `plugins/<GOOS>/<GOARCH>` and `plugins`, with configuration keyed by the library basename:

```yaml
plugins:
  enabled: true
  dir: "plugins"
  configs:
    opencode-go-quota:
      enabled: true
      priority: 1
```

## Scope

Keep the plugin read-only until credential discovery and quota rendering are verified against a canary CLIProxyAPI instance. Do not add cookies, browser login, a database, New API access, routing mutation, or automatic credential disabling to the first release.
