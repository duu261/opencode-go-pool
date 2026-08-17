# OpenCode Go Pool

[English](README.md) | 简体中文

这是一个面向 CLIProxyAPI 的 OpenCode Go 账号注册表、额度面板和账号池控制插件。它针对 CLIProxyAPI v7.2.130 的标准动态库 ABI，并直接运行在 Management Center 中。

## 功能

- 从 CLIProxyAPI 受保护的配置文件中读取 OpenAI 兼容凭据。
- 将凭据与可写的明文账号注册表合并，保存标签、邮箱、密码、推荐关系、到期时间和备注。
- 自动识别标准 HTTPS 端口上的 `https://opencode.ai/zen/go`，以及 CLIProxy 兼容的 `https://opencode.ai/zen/go/v1` provider。
- 正确解析 OpenCode `/v1/usage` 接口，避免在已包含 `/v1` 的 base URL 后重复拼接，并以有限并发查询 key。
- 同时显示已启用和停放账号的身份、状态、5 小时/每周/每月额度及重置时间。
- 通过 CLIProxyAPI 已认证的 OpenAI 兼容 provider Management API 添加、编辑、删除、启用和停用账号。
- 复用 Management Center 的 **Remember password** 所保存的登录信息，不要求再次输入管理密钥。
- 将 HTTP 401 标记为 `unavailable`，不会错误地判断为额度耗尽。
- 自动去除重复 key。

## 中文适配

- 本 README 提供完整简体中文说明。
- 账号标签、邮箱、备注、搜索和 YAML 注册表均支持 UTF-8，可正常使用中文。
- Management Center 内的插件界面目前仍为英文，但核心操作不依赖英文账号数据。
- 配置字段、API 路由、provider 名称和技术标识符保持英文，避免破坏兼容性。

## 环境要求

- Go 1.24 或更高版本。
- CGO 和 C 编译器。
- CLIProxyAPI v7.2.130，或兼容 plugin ABI v1 的构建版本。

## 验证

```bash
make check
go test -race ./...
```

构建产物为 `dist/opencode-go-quota.so`。
为了兼容 CLIProxyAPI，plugin ID 和动态库基础名称仍然保留为 `opencode-go-quota`。

## CLIProxyAPI 配置

将插件构建产物挂载到 CLIProxyAPI 的插件目录。CLIProxy 配置文件可以继续位于 `/CLIProxyAPI/config.yaml`，但账号注册表路径必须可写并持久化。

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
    referred_by_api_key: "sk-parent-account..."
    expires_at: "2026-09-17"
    notes: "Disposable account"
```

直接连接 OpenCode Go 的 provider 不需要额外配置名称。两种 base URL 写法在查询额度时都不会重复拼接 `/v1`。插件不会自动识别明文 HTTP。对于有意使用的本地 canary、私有代理或其他兼容 URL，需要显式允许对应的 CLIProxy provider 名称：

```yaml
      provider_names:
        - opencode-canary
```

## 路由和认证

- 页面外壳：`/v0/resource/plugins/opencode-go-quota/status`
- 额度数据：`GET /v0/management/plugins/opencode-go-quota/quotas`
- 账号注册表和合并额度数据：`GET /v0/management/plugins/opencode-go-quota/accounts`
- 替换账号注册表：`PUT /v0/management/plugins/opencode-go-quota/accounts`，请求必须携带 `GET` 返回的 revision；过期写入会收到 `409 Conflict`。

CLIProxyAPI 的插件 resource 路由无需认证，因此公开路由只返回静态页面外壳。Management Center 通过同源 iframe 加载页面。页面会解码 Management Center 记住的 `cli-proxy-auth` 会话，并仅使用该密钥调用已认证的 Management API。如果没有启用 **Remember password**，页面会提示管理员启用它，而不是再显示一个管理密钥输入框。

## 账号池控制

CLIProxyAPI 始终是路由状态的唯一真实来源。新账号保存后默认处于停放状态；**Enable** 和 **Disable** 是独立的路由操作。已启用账号必须先停用才能删除，避免账号注册表和路由池之间出现部分更新。

每次修改账号池前，插件都会重新读取目标 provider，然后再执行 PATCH。同一浏览器的多个标签页通过 Web Lock 串行执行修改。插件不会自动停用凭据。

## 范围和安全说明

插件不负责浏览器账号登录、数据库、New API 接入或自动停用凭据。账号注册表会有意以明文保存一次性账号凭据，并使用 `0600` 文件权限保护。因此它只适合可信管理员控制的自托管环境。

## 许可证

MIT
