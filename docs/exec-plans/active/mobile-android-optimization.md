# 执行计划：移动端体验优化与安卓 App 原生化适配

> 状态：🚀 进行中
> 目标：将 Web 前端全面升级为具有原生安卓 App / PWA 体验的移动应用，具备沉浸式独立窗口（Standalone）、100dvh 软键盘自适应、防误触与触控微动效、TabBar 5 入口对齐及移动端 E2E 走查。

---

## 最小任务拆分与执行流

```
Task 1: PWA Manifest 与移动端 App Shell 规范
   │
   ▼
Task 2: 全局移动端触控人体工学与手势体验 (Tap, Overscroll, Selection)
   │
   ▼
Task 3: 安卓软键盘与 100dvh 动态视口高度自适应
   │
   ▼
Task 4: TabBar 移动端居中约束与 5 入口闭环 (对话/家务/记账/报饭/设置)
   │
   ▼
Task 5: 对话页触控靶心与会话抽屉交互优化 (Touch Targets & Drawer)
   │
   ▼
Task 6: 移动端端对端自动化测试 (Playwright Mobile) 与报告产出
```

---

## 详细任务与验收标准

### Task 1: PWA Manifest 与移动端 App Shell 规范
- [ ] 创建 `web/public/manifest.webmanifest`，包含名称、`display: standalone`、`orientation: portrait`、主题色彩 `#F2F3F7`、图标配置；
- [ ] 更新 `web/index.html`，补充 `theme-color`、`mobile-web-app-capable`、优化 `viewport` 禁用误触缩放。

### Task 2: 全局移动端触控人体工学与手势体验
- [ ] 消除全站点击灰蓝方块高亮：`-webkit-tap-highlight-color: transparent`；
- [ ] 消除 300ms 触控双击延迟：`touch-action: manipulation`；
- [ ] 导航与按钮禁用文本误选：`user-select: none`，正文与气泡保留复制能力；
- [ ] 屏蔽外部下拉回弹刷新：`overscroll-behavior-y: contain`；
- [ ] 全站按钮与卡片增加原生触感反馈：`:active { transform: scale(0.96); opacity: 0.85; }`。

### Task 3: 安卓软键盘与 100dvh 动态视口高度自适应
- [ ] 布局根容器与页面使用 `100dvh`（Dynamic Viewport Height），解决安卓软键盘弹出时 `100vh` 布局破坏；
- [ ] 输入框增加 `enterKeyHint="send"`，键盘右下角显示“发送”。

### Task 4: TabBar 移动端居中约束与 5 入口闭环
- [ ] `TabBar.module.css` 补充 `max-width: 480px; margin: 0 auto; left: 0; right: 0;`；
- [ ] 增加安全区自适应：`padding-bottom: max(env(safe-area-inset-bottom), 8px)`；
- [ ] 补齐第 5 个 Tab 入口：`设置`（/settings），与 `App.tsx` 路由闭环。

### Task 5: 对话页触控靶心与会话抽屉优化
- [ ] 确保按钮靶心符合无障碍触摸规范（≥44×44px）；
- [ ] 优化会话侧边栏抽屉的滑入动效与点击遮罩关闭。

### Task 6: 移动端端对端自动化测试与报告
- [ ] 编写并执行移动端场景 Playwright E2E 测试；
- [ ] 验证全流程无报错，产出测试报告。
