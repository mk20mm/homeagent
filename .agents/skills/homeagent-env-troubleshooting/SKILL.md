---
name: homeagent-env-troubleshooting
description: >-
  Comprehensive troubleshooting guide and self-healing rules for the Windows, PowerShell, Vite, Go, and SQLite environment of HomeAgent.
  Use whenever encountering server errors, test timeouts, proxy issues, or build failures.
---

# HomeAgent 环境踩坑与自愈知识库 (Troubleshooting & Pitfall Guard)

本项目在 Windows 11 + PowerShell 5.1 + 无 GCC + 本地代理开发环境下踩过的全部坑点与标准自愈解法。遇阻时优先在此查找对策。

---

## 1. 代理与网络陷阱 (502 Bad Gateway)
- **现象**：通过 `curl.exe` 请求 `http://localhost:8080` 时出现 `HTTP/1.1 502 Bad Gateway`。
- **根因**：本机环境开启了代理（如 airtcp 127.0.0.1:5780 或 Clash），系统环境变量 `HTTP_PROXY` / `http_proxy` 导致本地 localhost 请求被代理拦截。
- **解法**：本地 curl 调试必须加上 `--noproxy "*"` 参数：
  ```powershell
  curl.exe --noproxy "*" -s http://localhost:8080/api/v1/health
  ```

---

## 2. PowerShell 编码陷阱 (BOM 与 GBK 乱码)
- **现象 1**：后端 JSON 反序列化报 400 语法错误。
  - **根因**：`Set-Content -Encoding UTF8` 会在文件开头写入 UTF-8 BOM，导致 Go 的 JSON 解码器解析报错。
  - **解法**：写临时 JSON 文件必须用无 BOM 编码：
    ```powershell
    [IO.File]::WriteAllText($f, $json, (New-Object Text.UTF8Encoding $false))
    ```
- **现象 2**：修改中文源码文件变成 GBK 乱码。
  - **根因**：PowerShell 5.1 默认文件编码是系统 GBK。
  - **解法**：必须使用 Agent 的 `replace_file_content` 或 `write_to_file` 工具修改源码，禁止在终端使用重定向 `>` 或 `Set-Content`。

---

## 3. Vite 热重载死循环打断 Playwright 测试
- **现象**：Playwright 走查时，对话记账后按钮一直处于 disabled 状态，超时失败。
- **根因**：Playwright 运行中向 `web/e2e-shots/` 和 `web/e2e-results/` 输出文件，Vite 开发服务器默认监听了整个 `web` 目录，触发了全页 Reload (`[vite] page reload`)，打断了正在进行的 SSE 长连接。
- **解法**：在 `web/vite.config.ts` 的 `server.watch` 中必须配置忽略项：
  ```ts
  watch: {
    ignored: ['**/e2e-shots/**', '**/e2e-results/**', '**/e2e-report/**', '**/playwright-report/**'],
  },
  ```

---

## 4. SSE 流断开与 Reader 闭环兜底
- **现象**：网络波动或服务端断开后，前端发送按钮无法恢复可用状态。
- **根因**：服务端在没有发送 explicit `data: {"type":"done"}` 的情况下关闭了连接，客户端 Reader 循环 break 退出，但未触发 `onEvent({ type: 'done' })`。
- **解法**：在 `web/src/hooks/useSSE.ts` 中维护 `receivedDone` 标志，Reader 正常退出循环时若未收到 `done`，兜底触发一次 `onEvent({ type: 'done' })` 确保状态归位 `idle`。

---

## 5. 无 GCC 环境与 SQLite 驱动限制
- **现象**：编译提示 gcc 未安装，或 `-race` 报错。
- **根因**：系统未安装 mingw/gcc，CGO 不可用。
- **解法**：
  - 使用纯 Go 实现的 `modernc.org/sqlite` 驱动（驱动名：`sqlite`，ent 方言：`dialect.SQLite` 即 "sqlite3"）。
  - `go test` 禁止加 `-race` 标志。

---

## 6. 端口冲突与僵尸进程清理
- **现象**：修改了后端代码但接口依然返回旧逻辑或 404。
- **根因**：旧的 `homeagent.exe` 进程仍在后台占用 8080 端口，新请求被路由到旧实例。
- **解法**：每次重新启动前先执行清理：
  ```powershell
  Get-Process -Name "homeagent" -ErrorAction SilentlyContinue | Stop-Process -Force
  ```
  或者执行项目内置脚本：`.\scripts\harness-env.ps1`。
