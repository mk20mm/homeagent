---
name: homeagent-contract-workflow
description: >-
  Contract-first codegen and invariant enforcement workflow for HomeAgent.
  Use when modifying APIs, OpenAPI schemas, Ent database models, or backend/frontend shared types.
---

# HomeAgent 契约先行工作流 (Contract-First Codegen)

本规范确保全系统数据契约强一致。OpenAPI 契约是唯一真理源，禁止手动修改生成文件。

## 不可违反的架构不变量

1. **主键全 UUID**：禁止自增整数 ID。
2. **金额一律 `int64` 分**：后端与数据库一律存储整数分；前端展示用 `formatYuanGrouped(cents)` / `formatYuan(cents)`，输入入库用 `yuanToCents(yuan)`。严禁 float 浮点存储与运算。
3. **写操作全量可撤销**：新增的业务写操作工具必须接入 `undo_log`，提供 24h 逆向回滚。
4. **权限双保险**：角色源头过滤（`GET /tools`）+ 运行时鉴权双层校验。
5. **严禁手改生成文件**：
   - 🚫 `internal/openapi/api.gen.go`
   - 🚫 `internal/store/ent/` 下非 schema 目录的文件
   - 🚫 `web/src/api/schema.d.ts` / `admin/src/api/schema.d.ts`

---

## 契约变更标准操作步骤

### 1. 修改 OpenAPI 契约
- 文件：`api/openapi.yaml`
- 新增/修改 paths、parameters、requestBody、responses、components/schemas。
- 确保响应结构遵循统一错误标准：`{ code, message, trace_id }`。

### 2. 执行全量代码生成
运行专用生成脚本：
```powershell
.\scripts\codegen.ps1
```
该脚本按严格顺序执行三步：
1. `go generate ./internal/store/...`（刷新 Ent ORM）
2. `cd internal/openapi; go generate`（刷新 Go server 接口桩）
3. `pnpm -r run gen-api`（刷新前端 Web 与 Admin 的 TypeScript 类型）

### 3. Ent Schema 踩坑检查表
编写 `internal/store/ent/schema/*.go` 时特别注意：
- **无 gcc 环境**：驱动使用 `modernc.org/sqlite`，ent dialect 使用 `dialect.SQLite`。
- **enum 必填字段默认值**：如果 enum 字段未提供默认值，且 Save() 时未显式传入，ent 会静默失败并不返回报错！**所有 enum 字段必须使用 `.Default(...)` 或在创建前必填赋值**。
- **索引不支持 `.Comment()`**：ent v0.14.6 索引不支持 `.Comment()`，不要在索引链式调用上加注释。
- **外键联合索引**：使用 `index.Fields("column").Edges("edge")`。

### 4. 编译与类型验证
生成后立即执行编译与类型验证：
```powershell
go build ./cmd/homeagent
pnpm -r run typecheck
```
确保全仓库 0 类型错误。
