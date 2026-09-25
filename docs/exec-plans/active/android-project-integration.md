# 执行计划：接入 Android 原生工程与首个 APK 打包

> 状态：🚀 进行中
> 目标：在 `web/` 移动端引入 Capacitor，生成标准 Android Studio 原生工程目录，配置网络权限与软键盘自适应，通过 Gradle 命令行编译生成 `homeagent-debug.apk`，并支持直接由 `studio64` 调起打开。

---

## 最小任务拆分与执行流

```
Task 1: Capacitor 依赖引入与 capacitor.config.ts 配置
   │
   ▼
Task 2: 生成 Android 原生工程 (web/android)
   │
   ▼
Task 3: Android 原生工程配置 (网络权限、Cleartext、输入法自适应、应用名)
   │
   ▼
Task 4: 构建 Web 资产并调用 Gradle 编译生成 APK
   │
   ▼
Task 5: 产出物验证、studio64 联动测试与报告
```

---

## 详细任务与验收标准

### Task 1: 引入 Capacitor 依赖与初始化配置
- [ ] 在 `web/` 安装 `@capacitor/core`、`@capacitor/cli`、`@capacitor/android`；
- [ ] 创建 `web/capacitor.config.ts`，配置 `appId: com.homeagent.app`、`appName: 家事 Agent`、`webDir: dist`。

### Task 2: 生成 Android 原生工程
- [ ] 运行 `npx cap add android`，在 `web/android` 生成完整的标准 Gradle 原生工程。

### Task 3: Android 原生工程配置优化
- [ ] 在 `AndroidManifest.xml` 中配置网络权限 `android.permission.INTERNET`；
- [ ] 开启 `usesCleartextTraffic="true"`（允许局域网自建后端 HTTP 联调）；
- [ ] 设置 `windowSoftInputMode="adjustResize"` 确保软键盘与 100dvh 视口自适应。

### Task 4: 编译打包生成 APK
- [ ] 执行 `pnpm --filter @homeagent/web run build` 生成最新 Web 静态资产；
- [ ] 执行 `npx cap sync android` 同步资产至原生工程；
- [ ] 配置本机 Java 环境变量（`C:\app\wrok\wrok1\jbr`）；
- [ ] 执行 `.\gradlew.bat assembleDebug`，产出 `app-debug.apk`。

### Task 5: 验证与 studio64 调起
- [ ] 验证 APK 存在且体积合规；
- [ ] 产出构建报告与 studio64 打开指引。
