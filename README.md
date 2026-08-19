# OpenCode Go Pool

English | [简体中文](README.zh-CN.md)

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
- Provides authenticated `Show`/`Hide` and `Copy` controls for API key, email, password, and referral URL in the Add/Edit dialog.
- Defaults a newly added account expiry to today plus 31 days; existing expiry dates are preserved when editing.
- Bulk-imports human-friendly TSV rows with preview, duplicate detection, validation, and one revision-safe save.

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
      accounts_path: "/CLIProxyAPI/accounts/opencode-accounts.yaml"
      max_concurrency: 4
      timeout_seconds: 15
```

Mount the account directory, not the individual YAML file:

```yaml
services:
  cli-proxy-api:
    volumes:
      - ./accounts:/CLIProxyAPI/accounts
```

Registry updates use a temporary file and atomic rename. A single-file bind mount cannot be replaced and fails with `device or resource busy`.

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

## How switching works

There are two different switches:

1. **Pool switch - Enable / Disable**
   - **Enable** adds the account API key to the configured CLIProxy OpenCode provider.
   - **Disable** removes that key from the provider, so CLIProxy stops sending traffic to it.
   - CLIProxy provider configuration is the routing source of truth. The registry alone does not make a disabled account route traffic.
   - Deletion is blocked until the account is disabled.

2. **Automatic request-time switch - `auto_pool: true`**
   - The scheduler only handles candidates belonging to the recognized OpenCode provider.
   - It chooses among eligible OpenCode keys using round-robin selection.
   - Keys in cooldown, manually held, disabled, or unavailable are skipped.
   - A recognized 5h/weekly quota error parks that credential in memory until its reset time. It then becomes eligible again automatically.
   - Automatic parking does not rewrite provider configuration and does not touch unrelated providers or models.
   - If credential or hold-state discovery fails, scheduling fails closed and delegates to the native CLIProxy scheduler. It never assumes a key is safe.

Quota labels mean:

- `quota limited`: at least one of 5h, weekly, or monthly windows is exhausted.
- `quota depleted`: all three windows are exhausted.
- `unavailable`: the usage check could not authenticate or complete.
- A `0%` 5h window means that window currently has no usage. It does not mean the account is dead.

Typical operator flow:

1. Add the account. Expiry defaults to 31 days from today.
2. Use `Copy` in the authenticated dialog when you need the API key, email, password, or referral URL.
3. Enable the account to place its key in the CLIProxy provider.
4. Leave `auto_pool: true` enabled for automatic quota-aware switching.
5. Use **Manual hold** when an account must not route even if its quota is available.
6. Disable before deleting or retiring an account.

### One-month account policy

- A monthly limit hit does not delete the account. Keep the registry row, referral URL, referral relationship, and credit history.
- If referral credit restores upstream usage before expiry, click **Resume routing**. The button clears only plugin cooldown memory; the next request re-tests the key and a new quota error cools it again.
- If the account will not be revived, Disable it first and select **Referral only**. This state persists across restarts, keeps account metadata, and cannot be enabled until restored.
- When `expires_at` passes, the scheduler treats the account as referral-only automatically. If every OpenCode credential is cooling, held, expired, or referral-only, the plugin rejects scheduling instead of delegating dead credentials to native routing.

### Bulk import

Click **Bulk add** and paste one tab-separated row per account. Header row is optional:

```tsv
label	email	password	api_key	referral_url	referred_by	expires_at	notes
go-001	user1@example.com	pass-001	sk-...	https://opencode.ai/go?ref=...
```

Choose the default OpenCode provider, then review the preview. Missing `expires_at` becomes today plus 31 days. Existing API keys and duplicate keys within the paste are rejected; accepted rows are saved together with revision protection. The importer does not overwrite existing accounts.

## Credential handling

The registry intentionally stores disposable account credentials in plaintext with file mode `0600`. The UI masks API keys and passwords by default. `Show` and `Copy` are explicit operator actions inside the authenticated Add/Edit dialog; credentials are not rendered in the account table.

## Scope

No browser account login, database, New API access, or provider-config rewriting for automatic parking.

## License

MIT
