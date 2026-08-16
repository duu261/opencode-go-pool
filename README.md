# OpenCode Go Quota

Read-only bulk quota plugin for CLIProxyAPI. It targets the standard dynamic-library ABI in CLIProxyAPI v7.2.130 and adds an OpenCode Go quota page to Management Center.

## What it does

- Reads OpenAI-compatible credentials from CLIProxyAPI's protected config file.
- Auto-detects direct `https://opencode.ai/zen/go` and CLIProxy-compatible `https://opencode.ai/zen/go/v1` providers on the standard HTTPS port.
- Resolves the OpenCode `/v1/usage` endpoint without duplicating a configured `/v1` suffix, then queries enabled keys with bounded concurrency.
- Shows masked key fingerprints, status, 5h/weekly/monthly usage, and all reset times.
- Marks HTTP 401 as `unavailable`, never as exhausted quota.
- Shows weight-disabled credentials without querying them.
- Deduplicates repeated keys and never returns or logs raw keys.

## Requirements

- Go 1.24 or newer.
- CGO and a C compiler.
- CLIProxyAPI v7.2.130 or a compatible plugin ABI v1 build.

## Verify

```bash
make check
go test -race ./...
```

The artifact is `dist/opencode-go-quota.so`.

## CLIProxyAPI configuration

Mount the plugin artifact into CLIProxyAPI's plugin directory. The existing production config is already mounted read-only at `/CLIProxyAPI/config.yaml`, which is the plugin default.

```yaml
plugins:
  enabled: true
  dir: "plugins"
  configs:
    opencode-go-quota:
      enabled: true
      priority: 1
      config_path: "/CLIProxyAPI/config.yaml"
      max_concurrency: 4
      timeout_seconds: 15
```

Direct OpenCode Go providers need no name configuration. Both base URL forms avoid duplicating `/v1` when querying usage. Plain HTTP is never auto-detected. For an intentional local canary, private proxy, or alternate compatible URL, explicitly allow the CLIProxy provider name:

```yaml
      provider_names:
        - opencode-canary
```

## Routes and security

- Page shell: `/v0/resource/plugins/opencode-go-quota/status`
- Quota data: `/v0/management/plugins/opencode-go-quota/quotas`

CLIProxyAPI v7.2.130 plugin resource routes are unauthenticated. Therefore the public page is only a static shell and contains no quota or credential data. The page asks for the management key and sends it once as a Bearer header to the authenticated quota endpoint. It does not place the key in a URL or browser storage.

## Scope

Read-only only. No cookies, browser account login, database, New API access, routing mutation, or automatic credential disabling.
