# 执行计划：模型管理打通与 Grok × 苹果风 UI 升级

> 状态：🚀 进行中
> 目标：
> 1. 彻底解决模型管理断层：动态多供应商路由、密钥动态解密、配置热更新、连接测试探针端点。
> 2. Web 前端 UI 升级为 Grok Bot 现代交互质感 × 苹果 HIG 极简美学（模型胶囊切换器、悬浮毛玻璃输入栏、执行状态药丸）。
> 3. 全量单元测试与 Playwright 端对端测试验证并输出报告。

---

## 最小任务拆分与依赖关系

```
Task 1: API 契约扩展 (OpenAPI)
   │
   ▼
Task 2: 存储层模型解析与动态解密
   │
   ▼
Task 3: 网关层 DynamicGateway 与运行时贯通 (ReAct + ModelID)
   │
   ▼
Task 4: 管理端接口与配置热失效
   │
   ▼
Task 5: Web 端 Grok × Apple 风格重构 (Model Pill + Floating Capsule)
   │
   ▼
Task 6: Admin 端连接测试与模型增删 UI
   │
   ▼
Task 7: 全量测试验证 (Unit + E2E) 与结果报告
```

---

## 任务详情与验收标准

### Task 1: 契约先行（OpenAPI 扩展）
- [ ] `api/openapi.yaml` 补充 `POST /admin/providers/{providerId}/test` 连接测试接口定义
- [ ] `api/openapi.yaml` 补充 `POST /admin/models` 与 `DELETE /admin/models/{modelId}`
- [ ] 执行 `make generate`（生成 Go 桩与 TS schema）

### Task 2: 存储层增强（Repository）
- [ ] `internal/domain/model/service.go` 扩展 `ResolveConnection`、`TestProvider` 等接口
- [ ] `internal/store/repo/provider.go` 实现按 `modelID` 动态获取解密连接信息与增删模型
- [ ] 编写单测验证模型解析与解密正确性

### Task 3: 动态网关与运行时贯通
- [ ] `internal/agent/gateway/openai.go` 修正 `buildRequest` 优先使用 `req.Model`
- [ ] 实现 `DynamicGateway`（支持多供应商客户端缓存池与热失效）
- [ ] `internal/agent/runtime/runtime.go` 与 `internal/api/v1/chat.go` 透传 `ModelID` 并更新会话关联
- [ ] 编写网关与调度测试

### Task 4: 管理端接口与热更新
- [ ] `internal/api/v1/admin.go` 实现连接测试端点与模型增删端点
- [ ] 配置修改后自动清理 Client 缓存，确保无需重启服务热生效

### Task 5: Web 端 Grok × Apple UI 重构
- [ ] 模型切换器升级为 Grok 风格苹果毛玻璃胶囊（Model Pill）
- [ ] 底部输入栏升级为悬浮式大胶囊（Floating Pill Container）
- [ ] 优化思考中与工具调用的状态展示卡片
- [ ] 保留关键 aria-label 与 placeholder 确保兼容已有 E2E

### Task 6: Admin 端连接测试与模型增删
- [ ] 供应商表格增加「测试连接」按钮及结果展示
- [ ] 支持新增与删除自定义模型

### Task 7: 端对端测试与结果报告
- [ ] `go test ./...` 单元测试验证
- [ ] `pnpm -r run typecheck` 类型检查
- [ ] 执行 Playwright E2E 全量测试
- [ ] 产出完整审计报告
