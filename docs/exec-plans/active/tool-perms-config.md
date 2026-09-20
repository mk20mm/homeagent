# 下期计划：工具权限可配置化（解耦）

> 对齐 **AI-STD-006（仓库 Harness）/ AI-STD-007（身份与最小权限）**。

> 状态：⏳ 下期（排在 P2 前端联调之后）
> 来源：家庭实际使用反馈——工具权限分配不该写死在代码里，应能后台配置 + 用户自助。

## 背景（为什么现在要做）

当前工具权限**硬编码在种子数据里**（`internal/store/store.go` 的 `MustSeed`）：

```go
{"爸爸", member.RoleParent, ..., map[string]bool{
    "expense.write": true, "expense.read": true, "task.write": true, ...
}}
```

问题：

- 改权限 = 改代码 + 重新部署，家事场景里「这周让孩子也能记账」这种事做不到
- 工具的权限键（`expense.write` 等）散落在各工具 Spec 里，新增工具时权限矩阵和工具定义两处维护，容易错配（ADR-005 的负面后果之一：两套逻辑需保持一致）
- admin 权限矩阵页目前是只读占位（技术债 T4）

## 目标

1. **后台配置**：admin 端可为每个成员开关工具权限（当前 T4 只读占位的真实化）
2. **用户自助**：成员自己能在「我的设置」里开关自己有资格调整的权限（如「记账提醒」「任务推送」），敏感权限（`system.admin`、改他人权限）不可自配
3. **解耦**：权限矩阵成为库里可变的数据，不再随代码发版；工具注册时自动声明所需权限，矩阵自动出现对应开关

## 不变量（改任何代码前先读）

- **权限双保险不能破**（ADR-005）：提示词源头过滤与执行层校验**都从同一份矩阵读**。配置化后矩阵是唯一事实源，代码只读取不缓存
- **改权限立即生效**：JWT 只放 member_id + role、不放权限（已满足），每次请求实时查矩阵 → 改完下一次对话即生效
- **审计留痕**：权限变更（谁改了谁的什么开关）必须写 audit_log，`permission_denied` 事件照常记录
- **默认安全**：新成员默认最小权限；新工具上线默认对非 admin 关闭

## 任务拆解

### 1. 工具权限声明集中化

把散落在各工具 `Spec().Permission` 里的权限键收敛为**权限注册表**（`internal/agent/tool/permissions.go`），工具引用而非自造字符串：

```go
// 权限键唯一事实源：工具注册表引用，admin 渲染开关也引用它
var PermissionCatalog = []PermissionDef{
    {Key: "expense.write", Module: "expense", Label: "记账", SelfService: false, Risk: "high"},
    {Key: "task.read",     Module: "task",    Label: "看任务", SelfService: true,  Risk: "low"},
    ...
}
```

- `SelfService` 标记用户可自助开关的权限（低风险只读类），敏感权限只能 admin 改
- admin 前端的权限矩阵页直接由 catalog 渲染，新增工具 = 自动出现开关，不再两处维护

### 2. 权限写端点（admin + 自助）

- `PUT /api/v1/admin/members/{id}/permissions`（`system.admin` 才能调）：整体设置某成员权限
- `PATCH /api/v1/me/permissions`（本人才能调）：只能翻转 `SelfService=true` 的开关，否则 403
- 两者都写 audit_log（工具名记为 `permission_change`，params 含变更前后）

### 3. 权限模板（角色预设）

保留 `permission_template`（admin/full/limited/child）作为**一键套用模板**的入口，套用后仍可逐项微调。模板定义移到 catalog，不写死在种子。

### 4. 前端

- admin 权限矩阵页从只读占位改为可编辑（T4 偿还）
- web「我的设置」页：`SelfService` 权限的开关列表

### 5. 种子数据瘦身

`MustSeed` 只保留模板套用 + 家庭初始成员，不再硬编码每个人的 `permissions` map。

## 验收用例

```json
{
  "category": "functional",
  "description": "admin 改权限立即生效：关闭孩子的 expense.write 后，孩子下次对话看不到记账工具",
  "steps": [
    "admin 调 PUT /admin/members/{孩子}/permissions 关闭 expense.write",
    "audit_log 记录该变更",
    "孩子重新 GET /tools：record_expense 消失",
    "孩子 /chat 试图记账：执行层拒绝（兜底仍有效）"
  ],
  "passes": false
}
```

```json
{
  "category": "safety",
  "description": "自助权限边界：用户只能翻 SelfService 开关，敏感权限改不了",
  "steps": [
    "PATCH /me/permissions 翻转 task.read（SelfService=true）：成功",
    "PATCH /me/permissions 翻转 expense.write（SelfService=false）：403",
    "PATCH /me/permissions 尝试改 system.admin：403 + 审计留痕"
  ],
  "passes": false
}
```

```json
{
  "category": "functional",
  "description": "新工具自动出现在权限矩阵：注册新工具后 admin 页面自动多出开关，无需改前端",
  "steps": ["registry.Register(newTool) 后 GET 权限 catalog 包含新工具的权限键"],
  "passes": false
}
```

## 依赖

- 需 P2 的 admin 前端联调完成（否则改了权限没法验）
- 与 T4（admin 权限矩阵只读）、T5（JWT 已落地，不缓存权限所以天然支持热更新）相关
