---
name: homeagent-e2e-verifier
description: >-
  Autonomous E2E testing, visual inspection, and regression walk-through workflow.
  Use when validating mobile UI, chat SSE streaming, card rollbacks, or generating test screenshots.
---

# HomeAgent E2E 走查与视觉自检工作流 (E2E Verifier)

本工作流用于指导 Agent 独立完成真机视口模拟、无头浏览器链路走查、截图捕获与视觉质量评估。

## 测试环境准备

1. **清理残留与释放端口**：
   ```powershell
   .\scripts\harness-env.ps1
   ```
2. **启动后端服务**（8080 端口）：
   ```powershell
   # 推荐先编译为独立二进制再启动，防止 go run 编译阶段占住端口
   go build -o homeagent.exe ./cmd/homeagent
   # 后台启动 homeagent.exe
   ```
3. **启动前端开发服务**（5173 端口）：
   ```powershell
   # 后台启动 Vite
   pnpm --filter web dev
   ```

---

## E2E 自动化测试执行

### 1. 移动端与安卓原生体验走查
```powershell
pnpm --filter web exec playwright test e2e/mobile-android.spec.ts
```
**走查重点**：
- PWA `manifest.json` 与 App Shell 配置。
- Pixel 7 视口与 `100dvh` 容器高度，防止移动端滚动条穿透。
- 底部 TabBar 5 大入口单手触控切换。
- 会话侧边抽屉滑动与遮罩交互。

### 2. UX 痛批官模式全链路走查
```powershell
pnpm --filter web exec playwright test e2e/ux-walkthrough.spec.ts
```
**走查重点**：
- 空表单与错误口令拦截。
- 对话记账与 SSE 实时打字机流式输出。
- 执行结果卡片与 24h 撤销乐观回滚。
- 暴力连击（防重复点击）与超长文本防御。
- 各 Tab 数据与状态跨模块一致性。

---

## 视觉走查与截图审查标准

所有走查截图保存在 `web/e2e-shots/`：
- `ux-01-login.png`: 登录页毛玻璃卡片与居中输入框。
- `ux-04-home.png`: Grok 胶囊推荐词与悬浮胶囊输入框。
- `ux-06-expense-result.png`: 苹果蓝气泡与白色结果卡片。
- `ux-07-after-undo.png`: 撤销成功卡片（状态显示「已撤销」）。
- `ux-10-sidebar.png`: 会话侧边栏滑动覆盖。
- `ux-13-chores.png`: 家务分工与打卡分段控制器。
- `ux-15-money.png`: 财务总览黑曜石渐变卡与分类图标。
- `ux-16-kitchen.png`: 厨房暖橙正在下厨卡与大字做饭模式。
- `ux-18-settings.png`: iOS Inset Grouped 设置卡片与角色标签。

### Agent 自动视觉审查规范
- 使用 `view_file` 工具直接查看 `web/e2e-shots/` 下的 PNG 图片。
- 审查核对点：
  1. 界面元素是否有文字折行遮挡或重叠。
  2. 是否有明显的样式塌陷或水平横向滚动条。
  3. 颜色对比度是否符合 Apple HIG 简约规范。
  4. 按钮在流式结束后是否正常复位为可点击状态。
