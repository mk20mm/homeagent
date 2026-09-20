# 前端代码规范（React + TypeScript）

> 对齐 **AI-STD-006（仓库 Harness）**。

> 适配两端：**移动端**（Vite+React+PWA，家人日常）与 **Web 管理端**（Ant Design，家长管理）。共用同一套 API client 与类型。

---

## 1. 仓库结构（monorepo）

```
ai_agent/
├── web/              # 移动端（家人用，对话为主入口）
│   └── src/
│       ├── pages/        # 页面：Chat/Chores/Money/Meal/Settings
│       ├── components/   # 复用组件（气泡、结果卡片、TabBar）
│       ├── stores/       # zustand 状态
│       ├── api/          # OpenAPI 生成的 client（勿手改）
│       ├── hooks/        # useChat/useSSE/usePermission
│       └── styles/       # design-system 令牌
├── admin/            # Web 管理端（Ant Design）
│   └── src/
│       ├── pages/        # Dashboard/Debug/Admin/Permission/Audit
│       ├── layouts/      # 侧栏布局
│       └── ...
└── packages/
    └── shared/       # 两端共用：类型、常量、工具函数
```

- 两端共享 `packages/shared`（枚举如 RiskLevel、错误码、格式化函数）。
- API client 由 OpenAPI 生成到各端的 `api/`，**禁止手写 fetch 绕过类型**。

---

## 2. 组件与命名

- 组件文件 PascalCase（`ResultCard.tsx`）；工具函数 camelCase（`formatMoney.ts`）。
- 页面组件放 `pages/`，可复用组件放 `components/`；一个组件一个目录（含 index + 样式 + 测试）。
- Props 接口名 = 组件名 + `Props`（`type ResultCardProps = {...}`），用 `type` 而非 `interface`（联合类型场景更多）。
- 禁止默认导出组件（`export function ResultCard`），便于重构与 IDE 跳转。

---

## 3. 状态管理

- **全局状态**用 zustand（会话、登录态、家庭成员）；不用 redux。
- **服务端状态**用生成的 client + 自己包的 query hook（`useTasks()`），缓存策略统一。
- **局部状态**用 useState/useReducer；跨层传递超过 3 层必须提 store。
- 会话消息流是**状态机**（idle/streaming/tool_running/error），用 useReducer 管理，避免散落 useState。

---

## 4. 对话页核心规范（移动端）

- SSE 流：`useSSE` hook 封装断线重连；**消息先乐观渲染，工具结果以服务器回执为准**。
- **结果卡片**：工具返回的撤销卡片必须渲染「撤销」按钮，撤销走 `undo` API，撤销后本地立即更新（乐观）+ 服务端确认回滚。
- 模型切换：下拉只列后端返回的**已启用**模型，切换只改会话 modelId，不改工具集。
- 危险操作（如大额记账）前端展示 Confirm（A3 级 Preview），不用 alert/confirm，用项目内 Modal 组件。

---

## 5. 样式与设计系统

- 设计令牌（色彩/圆角/间距）统一在 `styles/tokens.ts`，对齐 [ui/design-system.md](ui/design-system.md)：主蓝 `#0A84FF`、AI 紫 `#7C3AED`。
- 移动端用 CSS Modules（作用域隔离，零运行时）；**管理端用 Ant Design 内置 token + less 变量**，不混用第三方 UI 库。
- 禁止内联样式写颜色/间距（必须引用 token）；动态样式用 clsx 组合类名。
- 适老模式：通过 `data-mode="elder"` 切换大字号 token 主题，不改组件逻辑。

---

## 6. 类型与 API

- `strict: true`；禁止 `any`（需 `unknown` + 收窄并注释理由）。
- 所有 API 响应类型来自生成 client；后端改契约后 `make gen-web` 重新生成。
- 错误处理统一：`api/` 抛出带 code 的错误，页面层用 `useToast` 或内联错误态渲染；**禁止吞错误**（catch 后无任何反馈）。

---

## 7. 测试

| 层   | 工具                     | 范围                              |
| ---- | ------------------------ | --------------------------------- |
| 组件 | Vitest + Testing Library | 结果卡片渲染/撤销交互、状态机流转 |
| 页面 | Vitest + MSW             | 对话流、权限路由                  |
| E2E  | Playwright（二期）       | 记账→撤销全链路                   |

- 关键交互（撤销、危险确认、模型切换）必须有测试，防回归。

---

## 8. 性能与体验

- 路由懒加载（React.lazy）；对话列表虚拟化（消息多时）。
- PWA：Service Worker 缓存静态资源；离线时输入框提示「已离线，消息将稍后发送」。
- 移动端交互遵守苹果 HIG（右滑返回、底部安全区适配）。

---

## 9. Definition of Done

- `pnpm lint`（ESLint+Prettier+Stylelint）过、`pnpm typecheck` 过。
- 涉及 API 的改动必须基于最新生成的 client，且后端契约已合并。
- UI 改动附截图（移动端 390×844 / 管理端 1440×900）。
