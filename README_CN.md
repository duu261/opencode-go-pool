# OpenCode Go Pool

面向 CLIProxyAPI 的账号注册表、配额面板和账号池控制插件。插件使用 CLIProxyAPI v7.2.130 的标准动态库 ABI，在 Management Center 内运行。

## 功能

- 从 CLIProxyAPI 受保护的配置文件读取 OpenAI-compatible 凭据。
- 将凭据与可写的明文账号注册表合并，注册表包含标签、邮箱、密码、推荐关系、过期时间和备注。
- 自动识别标准 HTTPS 端口上的直连 `https://opencode.ai/zen/go` 和 CLIProxy 兼容的 `https://opencode.ai/zen/go/v1` provider。
- 请求 OpenCode `/v1/usage` 时避免重复添加已配置的 `/v1` 后缀，并使用受限并发检查已启用的 API key。
- 为 active 和 parked 账号显示账号身份、状态、5 小时、Weekly、Monthly 配额和重置时间。
- 收到可识别的 5 小时或 Weekly 配额耗尽响应后，在请求时自动暂存对应的 OpenCode 凭据；重置时间到达后自动恢复路由资格。
- Manual hold 覆盖自动路由，并显示在 Attention 视图中。
- 通过 CLIProxyAPI 已认证的 OpenAI-compatible provider Management API 添加、编辑、删除、启用和禁用账号。
- 复用 Management Center 的 **Remember password** 登录状态，不要求第二个管理密钥。
- 将 HTTP 401 标记为 `unavailable`，不会误判为配额耗尽。
- 去重重复 API key。
- 在已认证的 Add/Edit 对话框中提供 API key、邮箱、密码和 Referral URL 的 `Show`/`Hide` 与 `Copy` 控制。
- 新账号的过期时间默认设置为今天之后 31 天；编辑已有账号时保留原过期时间。

## 要求

- Go 1.24 或更高版本。
- CGO 和 C 编译器。
- CLIProxyAPI v7.2.130，或兼容 plugin ABI v1 的版本。

## 验证

```bash
make check
go test -race ./...
```

构建产物为 `dist/opencode-go-quota.so`。插件 ID 和动态库文件名保持为 `opencode-go-quota`，以兼容 CLIProxyAPI。

## CLIProxyAPI 配置

将插件产物挂载到 CLIProxyAPI 的 plugin 目录。CLIProxy 配置可以继续使用 `/CLIProxyAPI/config.yaml`；账号注册表路径必须可写。

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

账号注册表示例：

```yaml
accounts:
  - api_key: "sk-..."
    provider_name: "Opencode-go"
    label: "go-01"
    email: "go01@example.com"
    password: "disposable-password"
    referral_url: "https://opencode.ai/go?ref=..."
    referred_by_api_key: "«redacted:sk-…»..."
    expires_at: "2026-09-17"
    notes: "Disposable account"
```

直连 OpenCode Go provider 不需要配置名称。两种 base URL 都能避免请求配额时重复添加 `/v1`。插件不会自动识别普通 HTTP。若要使用本地 canary、私有代理或其他兼容 URL，请显式允许 provider 名称：

```yaml
      provider_names:
        - opencode-canary
```

## 路由和认证

- 页面 shell：`/v0/resource/plugins/opencode-go-quota/status`
- 配额数据：`GET /v0/management/plugins/opencode-go-quota/quotas`
- 账号注册表和合并配额数据：`GET /v0/management/plugins/opencode-go-quota/accounts`
- 替换账号注册表：`PUT /v0/management/plugins/opencode-go-quota/accounts`，请求必须带上 `GET` 返回的 revision；旧 revision 会返回 `409 Conflict`。

CLIProxyAPI plugin resource route 不要求认证，因此公开页面只提供静态 shell。Management Center 会在同源 iframe 中加载页面。页面读取 Management Center 的 `cli-proxy-auth` 记住的会话，只使用该会话调用已认证的 Management API。如果没有开启 **Remember password**，页面会提示先开启，而不是再次要求输入管理密钥。

## 账号切换如何工作

这里有两种不同的切换：

### 1. Pool switch - Enable / Disable

- **Enable**：将账号 API key 加入配置中的 CLIProxy OpenCode provider。
- **Disable**：从 provider 中移除该 key，CLIProxy 不再向它发送流量。
- CLIProxy provider 配置是实际路由的唯一事实来源。仅修改注册表不会让 disabled 账号参与路由。
- 账号必须先 Disable，才能删除。

### 2. 自动请求时切换 - `auto_pool: true`

- scheduler 只处理已识别的 OpenCode provider candidate。
- 在符合条件的 OpenCode key 中使用 round-robin 选择。
- cooldown、Manual hold、disabled 或 unavailable 的 key 会被跳过。
- 收到可识别的配额耗尽错误后，对应凭据会在内存中暂存到 reset 时间；时间到达后自动重新可用。
- 自动暂存不会重写 provider 配置，也不会影响其他 provider 或模型。
- 如果凭据发现或 hold 状态发现失败，scheduler 会 fail closed，并交回 CLIProxy 原生 scheduler；不会假设某个 key 安全可用。

配额状态含义：

- `quota limited`：5h、Weekly、Monthly 中至少一个窗口已耗尽。
- `quota depleted`：三个窗口都已耗尽。
- `unavailable`：配额检查无法认证或请求未完成。
- 5h 显示 `0%` 表示当前 5 小时窗口没有使用量，不代表账号已失效。

典型操作流程：

1. 添加账号，过期时间默认是今天之后 31 天。
2. 需要使用 API key、邮箱、密码或 Referral URL 时，在已认证对话框中点击 `Copy`。
3. Enable 账号，将 key 放入 CLIProxy provider。
4. 保持 `auto_pool: true`，启用自动配额感知切换。
5. 账号即使有配额也不能路由时，使用 **Manual hold**。
6. 删除或退役账号前先 Disable。

## 凭据处理

注册表会以明文保存 disposable 账号凭据，文件权限为 `0600`。UI 默认隐藏 API key 和密码。`Show` 和 `Copy` 只能在已认证的 Add/Edit 对话框中由操作员主动触发；凭据不会显示在账号表格中。

## Scope

自动暂存不会执行浏览器账号登录、数据库写入、New API 访问或 provider 配置重写。

## License

MIT
