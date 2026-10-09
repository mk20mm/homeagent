# MCP 集成体系深化设计（Model Context Protocol Integration）

> **模块编号**：M11（MCP Gateway · Model Context Protocol 协议集成）  
> **文档定位**：技术规格与架构设计说明书（Phase E 第五顺位）  
> **对齐标准**：AI-STD-003（架构标准）、ADR-004（写操作可撤销/补偿）、ADR-005（权限双保险）、ADR-007（设备动作协调）

---

## 1. 业务场景与接入价值

Model Context Protocol（MCP）是行业主流开放标准，旨在统一大模型与外部工具、资源和提示词的交互协议。接入 MCP 将为 HomeAgent 带来巨大的生态赋能：
1. **即插即用接入成熟生态**：无需从零手写每一个外部硬件或服务适配器，可直接复用开源社区丰富的 MCP Server：
   - **Home Assistant MCP**：控制全屋照明、温湿度传感器、空调、新风、扫地机实机；
   - **Filesystem MCP**：受控读取家庭 NAS / 本地共享文档；
   - **Git / SQLite MCP**：开发者家庭的自动化运维与数据查阅；
   - **Brave / Tavily / Weather MCP**：标准化的外部气象与资讯服务。
2. **协议解耦与语言无关**：外部 MCP Server 可用 TypeScript、Python、Rust 等任意语言编写，作为独立子进程或远程服务运行，不影响 HomeAgent Go 主进程的轻量与纯粹。

---

## 2. 系统拓扑与双向通信架构

```mermaid
graph TD
    subgraph HomeAgent_Core [HomeAgent Go 主核心]
        Runtime[Agent Runtime 调度中枢]
        ToolRegistry[工具注册表 & 权限双保险]
        SafetyGate[A3 强人工确认网关 / 撤销包装]
        MCPManager[MCP Client 管理器 (stdio / SSE 客户端)]
    end

    subgraph External_MCPServers [外部 MCP 插件生态]
        ServerHA["🔌 Home Assistant MCP Server (stdio)"]
        ServerFS["📁 Local Filesystem MCP Server (stdio)"]
        ServerRemote["☁️ 远程三方 MCP Server (SSE)"]
    end

    Runtime --> ToolRegistry
    ToolRegistry --> SafetyGate
    SafetyGate --> MCPManager
    
    MCPManager <-- stdio JSON-RPC --> ServerHA
    MCPManager <-- stdio JSON-RPC --> ServerFS
    MCPManager <-- HTTP / SSE --> ServerRemote
```

---

## 3. 动态工具发现与包装机制（Tool Discovery & Wrapping）

### 3.1 握手与工具发现时序
1. **启动与握手（Initialize）**：
   HomeAgent 启动时，根据配置启动外部 Server 子进程，发送 `initialize` 握手请求协商协议版本与能力（Capabilities）。
2. **工具列表拉取（tools/list）**：
   调用 MCP `tools/list`，获取其声明的工具集合（含 `name`、`description`、`inputSchema`）。
3. **动态包装为标准 `tool.Tool`**：
   - 命名空间隔离：为防止工具重名冲突，统一前缀为 `mcp_{server_name}_{tool_name}`（例如 `mcp_ha_turn_on_light`）。
   - 注册至 HomeAgent 统一注册表，与内置工具享有同等的 ReAct 调用能力。

### 3.2 Go 适配层核心代码设计

```go
package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mk20mm/homeagent/internal/agent/tool"
)

// ServerConfig MCP Server 声明配置
type ServerConfig struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Transport string           `json:"transport"` // "stdio" | "sse"
	Command  string            `json:"command"`   // 如 "node" 或 "npx"
	Args     []string          `json:"args"`      // 如 ["@modelcontextprotocol/server-filesystem", "/family/share"]
	Env      map[string]string `json:"env"`
	Enabled  bool              `json:"enabled"`
	RequireConfirm bool        `json:"require_confirm"` // 是否强制人工确认
}

// DynamicMCPTool 动态代理包装器
type DynamicMCPTool struct {
	serverID   string
	toolName   string
	fullName   string
	spec       tool.Spec
	client     Client
	isWrite    bool
}

func (t *DynamicMCPTool) Name() string { return t.fullName }
func (t *DynamicMCPTool) Spec() tool.Spec { return t.spec }

func (t *DynamicMCPTool) Execute(ctx context.Context, rawArgs json.RawMessage, meta tool.ExecutionMeta) (*tool.Result, error) {
	// 调用 MCP 标准 tools/call
	resp, err := t.client.CallTool(ctx, t.toolName, rawArgs)
	if err != nil {
		return nil, fmt.Errorf("mcp tool call failed: %w", err)
	}
	return &tool.Result{
		Summary: resp.TextSummary(),
		Card:    resp.ToJSONCard(),
	}, nil
}
```

---

## 4. 严苛的确定性安全防线与 ADR-004/005/007 适配

外部 MCP Server 是第三方代码，**绝不能盲目信任其写操作**：

1. **权限双保险拦截（ADR-005）**：
   - 在 Admin 后台，每个 MCP Server 的每个工具必须明确指定绑定的角色权限（如 `admin`、`parent`、`child`）；
   - 未在白名单中授权的成员，在提示词构建和运行时调用阶段一律进行硬阻断。
2. **写操作强制提升为 A3 确认网关（ADR-004 & ADR-007）**：
   - 外部只读工具（如查询温度传感器、读取公开文件）：定义为 **A1 级**，自动安全执行；
   - 外部写操作（如开关电源、删除文件、改变设备状态）：因外部 MCP 未原生实现 HomeAgent 的 `Undo()` 补偿契约，**代码层强制将其危险等级提升至 A3 级**！
   - 运行时必须通过无线画布或对话卡片展示明确的人工确认弹窗（“是否允许执行操作：开启客厅空调？”），经由用户点击批准后方可发出物理指令。
3. **子进程生命周期守护与沙箱防死锁**：
   - 对 `stdio` 子进程注入单次请求超时硬指标（默认 5.0 秒）；
   - 子进程崩溃或未响应时自动实施安全熔断，禁止无界挂起拖垮主 Agent 调度器。
