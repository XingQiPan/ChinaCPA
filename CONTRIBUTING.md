# 贡献指南

感谢你对 ChinaCPA 的关注！提交代码前请先读这一页。

## 开发环境

- Go 1.27+（`go version` 确认）
- 可选：Rust 1.75+（仅在构建 `rail-rs/` 桌面浮动栏时需要）

```bat
go build ./...                 :: 编译全部包
go test ./internal/...         :: 跑测试
```

首次运行：

```bat
start.bat
```

脚本会在源码比 `ccpa-server.exe` 新时自动重新构建，然后以 `config.json` 启动网关。面板默认监听 `:8318`。

## 提交前检查

请确保以下三项都通过：

```bat
go build ./...
go vet ./internal/...
go test ./internal/...
```

改动涉及面板前端（`internal/panel/static/index.html`）时，建议实际启动服务点一遍相关页面，确认无 JS 报错。

## 代码约定

- 注释与提交信息用中文，与现有代码保持一致
- 新增上游能力（provider、鉴权、签名、指纹）时，把平台差异收敛在对应 `internal/provider/<平台>/` 包内，不要把特例散落到网关层
- 凭证、token、cookie 一律只走 `data/`（已在 `.gitignore`），**禁止**硬编码或提交进仓库
- 涉及上游请求的改动，注意 TLS 指纹（`internal/fp`）与 UA 的一致性，否则容易被风控拦截

## Pull Request

- 一个 PR 只做一件事，便于审查与回滚
- 描述里说明：改了什么、为什么改、如何验证
- 涉及行为变更的，同步更新 `README.md`

## 安全问题

发现凭证泄露、鉴权绕过、命令注入一类问题，**不要**开公开 issue。请通过仓库 owner 的邮箱私下联系，附上复现步骤。

## 许可

提交的代码将按本项目 [MIT LICENSE](LICENSE) 授权。
