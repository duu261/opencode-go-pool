# OpenCode Go Pool

Account registry, quota dashboard, and operator pool controls for CLIProxyAPI. It targets the standard dynamic-library ABI in CLIProxyAPI v7.2.130 and runs inside Management Center.

## What it does

- Reads OpenAI-compatible credentials from CLIProxyAPI's protected config file.
- Joins credentials with a writable plaintext account registry containing labels, email, password, referral relationships, expiry, and notes.
- Auto-detects direct `https://opencode.ai/zen/go` and CLIProxy-compatible `https://opencode.ai/zen/go/v1` providers on the standard HTTPS port.
- Resolves the OpenCode `/v1/usage` endpoint without duplicating a configured `/v1` suffix, then queries enabled keys with bounded concurrency.
- Shows recognizable account identity, status, 5h/weekly/monthly usage, and reset times for both active and parked registry accounts.
- Auto-parks OpenCode credentials at request time after a recognized 5h/weekly exhaustion response, then restores routing when the reset expires.
- Manual hold overrides automatic routing and is visible in the Attention view.
- Adds, edits, removes, enables, and disables accounts through CLIProxyAPI's authenticated OpenAI-compatible provider Management API.
- Reuses the Management Center login stored by its **Remember password** option. The plugin does not ask for a second management key.
- Marks HTTP 401 as `unavailable`, never as exhausted quota.
- Deduplicates repeated keys.

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
The plugin ID and library basename remain `opencode-go-quota` for CLIProxyAPI compatibility.

## CLIProxyAPI configuration

Mount the plugin artifact into CLIProxyAPI's plugin directory. The CLIProxy config may remain at `/CLIProxyAPI/config.yaml`; the account registry path must be writable.

```yaml
plugins:
  enabled: true
  dir: "plugins"
  configs:
    opencode-go-quota:
      enabled: true
      priority: 1
      config_path: "/CLIProxyAPI/config.yaml"
      accounts_path: "/CLIProxyAPI/opencode-accounts.yaml"
      max_concurrency: 4
      timeout_seconds: 15
```

Example account registry:

```yaml
accounts:
  - api_key: "sk-..."
    provider_name: "Opencode-go"
    label: "go-01"
    email: "go01@example.com"
    password: "disposable-password"
    referral_url: "https://opencode.ai/go?ref=..."
    referred_by_api_key: "sk-parent-account..."
    expires_at: "2026-09-17"
    notes: "Disposable account"
```

Direct OpenCode Go providers need no name configuration. Both base URL forms avoid duplicating `/v1` when querying usage. Plain HTTP is never auto-detected. For an intentional local canary, private proxy, or alternate compatible URL, explicitly allow the CLIProxy provider name:

```yaml
      provider_names:
        - opencode-canary
```

## Routes and authentication

- Page shell: `/v0/resource/plugins/opencode-go-quota/status`
- Quota data: `GET /v0/management/plugins/opencode-go-quota/quotas`
- Account registry and merged quota data: `GET /v0/management/plugins/opencode-go-quota/accounts`
- Replace account registry: `PUT /v0/management/plugins/opencode-go-quota/accounts` with the revision returned by `GET`; stale writes receive `409 Conflict`.

CLIProxyAPI plugin resource routes are unauthenticated, so the public route serves only a static shell. Management Center loads it in a same-origin iframe. The page decodes Management Center's remembered `cli-proxy-auth` session and uses that key only for authenticated Management API calls. If **Remember password** is disabled, the page directs the operator to enable it instead of presenting another key input.

## Pool controls

CLIProxyAPI remains the routing source of truth. New accounts are saved parked; **Enable** and **Disable** are separate routing actions. An active account must be disabled before deletion, avoiding partial cross-store mutations. Each pool mutation refreshes the provider immediately before PATCH and serializes same-browser tabs with a Web Lock. When `auto_pool: true`, request-time scheduler routing skips recognized OpenCode quota exhaustion without rewriting provider config. Reset expiry restores eligibility; **Manual hold** always wins.

## Scope

No browser account login, database, New API access, or provider-config rewriting for automatic parking. The account registry intentionally stores disposable account credentials in plaintext with mode `0600`.

## License

MIT
