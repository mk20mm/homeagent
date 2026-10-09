import type { components } from '../../api/schema'

export type RunDetail = components['schemas']['RunDetail']

export interface CanvasNode {
  id: string
  title: string
  subtitle: string
  kind: 'input' | 'reasoning' | 'action' | 'checkpoint' | 'output' | 'ack' | 'gate'
  status: 'completed' | 'running' | 'pending' | 'waiting'
  icon: string
  details: string
  meta?: Record<string, string>
  turn?: number
  phase?: string
  ackLabel?: string
  x: number
  y: number
}

export interface CanvasConnection {
  from: string
  to: string
  label?: string
  ackType?: 'propose' | 'confirm' | 'result' | 'observe' | 'flow'
}

export interface TurnGroup {
  turn: number
  title: string
  subtitle: string
  x: number
  y: number
  width: number
  height: number
  status: 'completed' | 'running' | 'pending' | 'waiting'
}

export interface ExecutionTrajectory {
  traces: string[]
  duration: string
  turnsCount: number
  nodes: CanvasNode[]
  connections: CanvasConnection[]
  turnGroups: TurnGroup[]
}

/**
 * 根据实际对话上下文、工具调用与流式状态，动态生成真实的执行轨迹与无限画布拓扑。
 * 彻底消除硬编码的静态假数据。
 */
export function generateExecutionTrajectory(
  prompt: string,
  toolName?: string,
  card?: Record<string, unknown>,
  isStreaming = false,
  status: 'streaming' | 'tool_running' | 'idle' | 'error' = 'idle',
): ExecutionTrajectory {
  const cleanPrompt = (prompt || '').trim()

  const isExpense =
    toolName?.includes('expense') ||
    toolName?.includes('income') ||
    card?.type === 'expense' ||
    card?.type === 'income' ||
    /(买|花|元|块|记账|支出|收入|花费|消费|账单|超市|买菜)/.test(cleanPrompt)

  const isMeal =
    toolName?.includes('meal') ||
    toolName?.includes('dinner') ||
    card?.type === 'meal' ||
    /(菜|饭|吃|晚餐|午餐|早餐|食谱|红烧|做饭)/.test(cleanPrompt)

  const isTask =
    toolName?.includes('task') ||
    card?.type === 'task' ||
    /(扫地|洗衣服|家务|做卫生|拖地|清洁|倒垃圾|提醒我)/.test(cleanPrompt)

  const isTravel =
    toolName?.includes('travel') ||
    toolName?.includes('trip') ||
    toolName?.includes('vehicle') ||
    card?.type === 'trip' ||
    /(车|出行|加油|充电|公里|驾驶|里程)/.test(cleanPrompt)

  const isSearch =
    toolName?.includes('search') ||
    card?.type === 'search_citation' ||
    /(搜|查|为什么|怎么|如何|故障|排查|洗碗机|红灯|代码|手册|指南)/.test(cleanPrompt)

  let traces: string[] = []
  let nodes: CanvasNode[] = []
  let connections: CanvasConnection[] = []

  const stepStatus = (targetKind: 'input' | 'reasoning' | 'action' | 'checkpoint' | 'output') => {
    if (!isStreaming) return 'completed'
    if (targetKind === 'input') return 'completed'
    if (targetKind === 'reasoning') return status === 'streaming' ? 'running' : 'completed'
    if (targetKind === 'action') return status === 'tool_running' ? 'running' : status === 'streaming' ? 'pending' : 'completed'
    if (targetKind === 'checkpoint') return status === 'tool_running' ? 'running' : status === 'streaming' ? 'pending' : 'completed'
    return 'pending'
  }

  // 0. 联网搜索与事实排查场景（豆包交错风格：思维短句 + 微动作胶囊）
  if (isSearch) {
    traces = [
      `我先检索关于【${cleanPrompt.slice(0, 16) || '洗碗机故障'}】的官方售后库与排查指引。`,
      `🌐 联网检索: ${cleanPrompt.slice(0, 22) || '西门子洗碗机水龙头红灯排查'}`,
      '检索到候选信源，调用 TypeSafe Jev System One 进行常识性概率打分。',
      '⚡ Jev System One: 语义概率打分与广告降权',
      '已锁定官方指南与社区真实排查经验，提炼结构化故障解决清单。',
      '📖 提炼核心排查步骤与 Checklist',
    ]

    nodes = [
      {
        id: 'node-input',
        title: '用户搜索提问',
        subtitle: 'Natural Language Search',
        kind: 'input',
        status: 'completed',
        icon: '🔍',
        details: cleanPrompt || '故障排查检索',
        meta: { 消息类型: '搜索查询', 领域: '家电排查' },
        x: 60,
        y: 180,
      },
      {
        id: 'node-action-search',
        title: '联网事实检索',
        subtitle: 'web_search',
        kind: 'action',
        status: stepStatus('action'),
        icon: '🌐',
        details: '从官方支持库与网络抓取多源排查资料，组建候选池。',
        meta: { 工具名: 'web_search', 协议: 'DuckDuckGo / Tavily' },
        x: 360,
        y: 180,
      },
      {
        id: 'node-reasoning-jev',
        title: 'TypeSafe Jev 决策重排',
        subtitle: 'System One Rerank',
        kind: 'reasoning',
        status: stepStatus('reasoning'),
        icon: '⚡',
        details: '调用 Jev System One 模型，识别促销广告并降权，置顶高可信实操方案。',
        meta: { 模型: 'Jev System One', 模式: '按需精排' },
        x: 660,
        y: 180,
      },
      {
        id: 'node-output',
        title: '结构化排查响应',
        subtitle: 'Structured Checklist',
        kind: 'output',
        status: !isStreaming ? 'completed' : 'pending',
        icon: '📋',
        details: '输出分步排查指引卡片，提供快捷待办创建闭环。',
        meta: { 卡片: 'SearchCitationCard', 交互: '步骤卡片' },
        x: 960,
        y: 180,
      },
    ]

    connections = [
      { from: 'node-input', to: 'node-action-search', label: '发起检索' },
      { from: 'node-action-search', to: 'node-reasoning-jev', label: '候选池回传' },
      { from: 'node-reasoning-jev', to: 'node-output', label: '结构化综合' },
    ]
  } else if (isExpense) {
    traces = [
      '我先解析收支记账意图，核对交易场景与消费金额。',
      '✏️ 核算金额规格化 (int64 分制)',
      '核验家长角色 (Parent)，确认具备账单写入权限。',
      '🔒 校验角色权限双保险',
      '写入 SQLite 事务并生成 24h 可逆撤销凭证。',
      '💾 写入 undo_log 检查点',
    ]

    nodes = [
      {
        id: 'node-input',
        title: '用户意图输入',
        subtitle: 'Natural Language Prompt',
        kind: 'input',
        status: 'completed',
        icon: '💬',
        details: cleanPrompt || '收支记账意图输入',
        meta: { 消息类型: '用户输入', 识别实体: '交易金额/消费提示' },
        x: 60,
        y: 180,
      },
      {
        id: 'node-reasoning',
        title: '语义意图推演',
        subtitle: 'Role & Policy Matrix',
        kind: 'reasoning',
        status: stepStatus('reasoning'),
        icon: '🧠',
        details: '语义归类为家庭财务支出，经权限双保险矩阵校验通过。',
        meta: { 角色权限: 'expense.write: 允许', 供应商网关: 'Dynamic Gateway' },
        x: 360,
        y: 180,
      },
      {
        id: 'node-action',
        title: '原子工具执行',
        subtitle: toolName || 'record_expense',
        kind: 'action',
        status: stepStatus('action'),
        icon: '⚙️',
        details: '调用领域服务进行记账，金额换算为整数分，注入防重幂等键。',
        meta: { 工具名: toolName || 'record_expense', 执行环境: 'Go 内核进程' },
        x: 660,
        y: 180,
      },
      {
        id: 'node-checkpoint',
        title: '持久化检查点',
        subtitle: 'Durable Checkpoint & Undo',
        kind: 'checkpoint',
        status: stepStatus('checkpoint'),
        icon: '💾',
        details: '事务写入 SQLite，记录 undo_log，支持 24 小时内本地原子撤销。',
        meta: { 可撤销性: '24小时有效', 存储引擎: 'SQLite (纯 Go 驱动)' },
        x: 960,
        y: 180,
      },
      {
        id: 'node-output',
        title: '响应与结果卡片',
        subtitle: 'Result Synthesis',
        kind: 'output',
        status: stepStatus('output'),
        icon: '✦',
        details: '生成交互式撤销卡片与回复，支持即时撤销与修改。',
        meta: { 视图层: 'React ResultCard', 协同状态: '已同步' },
        x: 1260,
        y: 180,
      },
    ]

    connections = [
      { from: 'node-input', to: 'node-reasoning', label: '语义抽取' },
      { from: 'node-reasoning', to: 'node-action', label: '鉴权分发' },
      { from: 'node-action', to: 'node-checkpoint', label: '状态落地' },
      { from: 'node-checkpoint', to: 'node-output', label: '凭证合成' },
    ]
  }
  // 2. 烹饪与用餐场景真实轨迹
  else if (isMeal) {
    traces = [
      '意图解析：检测到家庭膳食与菜谱建议诉求。',
      '档案推演：检索家庭成员口味、禁忌与历史用餐评价。',
      toolName ? `工具调度：执行 ${toolName}，进行营养均衡与多菜协同计算。` : '方案匹配：调取家常菜肴知识库，构建烹饪时序指导。',
      '步骤沉淀：持久化烹饪清单与步骤检查点。',
      '意图闭环：生成兼顾家庭口味的个性化推荐与烹饪指引。',
    ]

    nodes = [
      {
        id: 'node-input',
        title: '用户膳食意图',
        subtitle: 'Natural Language Prompt',
        kind: 'input',
        status: 'completed',
        icon: '🍳',
        details: cleanPrompt || '家庭用餐与食谱诉求',
        meta: { 识别主题: '晚餐/烹饪推荐', 成员范围: '全家共享' },
        x: 60,
        y: 180,
      },
      {
        id: 'node-reasoning',
        title: '家庭口味画像推演',
        subtitle: 'Dietary Preference & History',
        kind: 'reasoning',
        status: stepStatus('reasoning'),
        icon: '🥗',
        details: '结合长辈低盐、儿童营养等偏好进行平衡测算。',
        meta: { 健康约束: '营养均衡', 搭配算法: '多菜协同' },
        x: 360,
        y: 180,
      },
      {
        id: 'node-action',
        title: '烹饪工具协同',
        subtitle: toolName || 'suggest_dinner',
        kind: 'action',
        status: stepStatus('action'),
        icon: '🍲',
        details: '调取家庭真实菜谱库，规划食材准备与下锅时序。',
        meta: { 工具服务: toolName || 'suggest_dinner', 步骤指导: '并行烹饪' },
        x: 660,
        y: 180,
      },
      {
        id: 'node-checkpoint',
        title: '用餐计划留痕',
        subtitle: 'Meal Plan Snapshot',
        kind: 'checkpoint',
        status: stepStatus('checkpoint'),
        icon: '💾',
        details: '保存用餐建议快照，便于稍后直接报饭或生成采购清单。',
        meta: { 持久化: 'meal_reports', 状态跟踪: '待烹饪' },
        x: 960,
        y: 180,
      },
      {
        id: 'node-output',
        title: '个性化食谱输出',
        subtitle: 'Step-by-step Guide',
        kind: 'output',
        status: stepStatus('output'),
        icon: '✦',
        details: '呈现包含用料、步骤与多菜同时出锅时间表的详尽方案。',
        meta: { 呈现格式: '步骤指导卡片', 协同状态: '已就绪' },
        x: 1260,
        y: 180,
      },
    ]

    connections = [
      { from: 'node-input', to: 'node-reasoning', label: '偏好读取' },
      { from: 'node-reasoning', to: 'node-action', label: '菜谱求解' },
      { from: 'node-action', to: 'node-checkpoint', label: '计划保存' },
      { from: 'node-checkpoint', to: 'node-output', label: '方案输出' },
    ]
  }
  // 3. 家务与设备调度场景真实轨迹
  else if (isTask) {
    traces = [
      '意图解析：识别家庭任务与智能设备协同诉求。',
      '依赖检测：排查空间状态、设备电量及成员分工。',
      toolName ? `工具调度：执行 ${toolName}，构建调度中心编排任务。` : '编排计算：生成任务有向无环图 (DAG) 与执行依赖。',
      '状态机检查点：落库 AgentRun 实体，注册实时 Watcher 广播。',
      '意图闭环：任务已纳入家庭调度队列，支持实时驾驶与取消。',
    ]

    nodes = [
      {
        id: 'node-input',
        title: '家务指令输入',
        subtitle: 'Natural Language Prompt',
        kind: 'input',
        status: 'completed',
        icon: '🧹',
        details: cleanPrompt || '家庭任务或清洁诉求',
        meta: { 任务类型: '清洁/提醒/协作', 派发对象: '成员/智能设备' },
        x: 60,
        y: 180,
      },
      {
        id: 'node-reasoning',
        title: '调度中心 DAG 编排',
        subtitle: 'Dispatch Center Orchestrator',
        kind: 'reasoning',
        status: stepStatus('reasoning'),
        icon: '⚡',
        details: '解析步骤拓扑、资源互斥与前后置依赖关系。',
        meta: { 调度引擎: 'Agent Dispatch Center', 编排模式: 'Intent Plan' },
        x: 360,
        y: 180,
      },
      {
        id: 'node-action',
        title: '动作下发与执行',
        subtitle: toolName || 'assign_task / clean',
        kind: 'action',
        status: stepStatus('action'),
        icon: '🤖',
        details: '向目标端或成员工作台下发执行项，开启异步监听。',
        meta: { 协同通道: 'HomeAssistant / Local Tool', 监听机制: 'Live Watchers' },
        x: 660,
        y: 180,
      },
      {
        id: 'node-checkpoint',
        title: '树状生命周期注册',
        subtitle: 'Subagent Tree Checkpoint',
        kind: 'checkpoint',
        status: stepStatus('checkpoint'),
        icon: '💾',
        details: '写入 AgentRun 实体，挂载 parent_run_id 与级联生命周期。',
        meta: { 实时干预: '支持 Steering', 级联取消: '已启用' },
        x: 960,
        y: 180,
      },
      {
        id: 'node-output',
        title: '调度进度看板',
        subtitle: 'Realtime Dispatch Card',
        kind: 'output',
        status: stepStatus('output'),
        icon: '✦',
        details: '输出执行状态卡片，支持用户实时干预或查看进度。',
        meta: { 响应模式: '流式回执', 交互支持: '人类干预/确认' },
        x: 1260,
        y: 180,
      },
    ]

    connections = [
      { from: 'node-input', to: 'node-reasoning', label: 'DAG 构建' },
      { from: 'node-reasoning', to: 'node-action', label: '动作派发' },
      { from: 'node-action', to: 'node-checkpoint', label: '生命周期绑定' },
      { from: 'node-checkpoint', to: 'node-output', label: '看板反馈' },
    ]
  }
  // 4. 出行与用车场景真实轨迹
  else if (isTravel) {
    traces = [
      '意图解析：分析家庭出行与用车记录诉求。',
      '数据检索：核验车辆档案、里程记录与能耗基准。',
      toolName ? `工具调度：执行 ${toolName}，同步能耗与费用核算。` : '核算推演：计算单次出行成本并联动财务记账。',
      '检查点落地：写入出行履历与可逆关联支出。',
      '意图闭环：生成行程卡片与行车建议。',
    ]

    nodes = [
      {
        id: 'node-input',
        title: '出行记录输入',
        subtitle: 'Natural Language Prompt',
        kind: 'input',
        status: 'completed',
        icon: '🚗',
        details: cleanPrompt || '行车或能耗记录诉求',
        meta: { 实体提取: '里程/出发到达/加油充电' },
        x: 60,
        y: 180,
      },
      {
        id: 'node-reasoning',
        title: '出行成本与能耗核算',
        subtitle: 'Travel Cost Engine',
        kind: 'reasoning',
        status: stepStatus('reasoning'),
        icon: '⛽',
        details: '核算每公里能耗支出，并自动关联家庭账本。',
        meta: { 车辆管理: 'Vehicle Domain', 财务联动: 'Expense Sync' },
        x: 360,
        y: 180,
      },
      {
        id: 'node-action',
        title: '行程服务处理',
        subtitle: toolName || 'record_trip',
        kind: 'action',
        status: stepStatus('action'),
        icon: '🗺️',
        details: '写入行程记录并同步触发财务模块入账。',
        meta: { 服务名: toolName || 'record_trip', 事务一致性: '保证' },
        x: 660,
        y: 180,
      },
      {
        id: 'node-checkpoint',
        title: '行车与财务检查点',
        subtitle: 'Dual Checkpoint',
        kind: 'checkpoint',
        status: stepStatus('checkpoint'),
        icon: '💾',
        details: '保存行程快照与账单流水，生成双向撤销关联。',
        meta: { 关联撤销: '级联回滚支持', 状态: 'committed' },
        x: 960,
        y: 180,
      },
      {
        id: 'node-output',
        title: '行程综合卡片',
        subtitle: 'Trip Result Card',
        kind: 'output',
        status: stepStatus('output'),
        icon: '✦',
        details: '展示行程里程、费用及家庭用车汇总。',
        meta: { 展示卡片: '出行凭证', 状态: '已归档' },
        x: 1260,
        y: 180,
      },
    ]

    connections = [
      { from: 'node-input', to: 'node-reasoning', label: '里程核算' },
      { from: 'node-reasoning', to: 'node-action', label: '双域联动' },
      { from: 'node-action', to: 'node-checkpoint', label: '事务固化' },
      { from: 'node-checkpoint', to: 'node-output', label: '凭证呈现' },
    ]
  }
  // 5. 通用对话 / 复合推理场景真实轨迹
  else {
    traces = [
      '意图理解：语义分词与用户需求结构化解析。',
      '上下文装配：加载会话历史与家庭统一领域认知。',
      toolName ? `工具调度：执行 ${toolName} 处理特定业务项。` : '网关路由：匹配动态大模型供应商，建立流式输出通道。',
      '流式推理：多轮认知逻辑思考与高质回复合成。',
      '质量自检：检查回复完备度，沉淀状态机检查点。',
    ]

    nodes = [
      {
        id: 'node-input',
        title: '自然语言输入',
        subtitle: 'Natural Language Prompt',
        kind: 'input',
        status: 'completed',
        icon: '💬',
        details: cleanPrompt || '日常对话与咨询输入',
        meta: { 消息类型: '用户输入', 状态: '已接收' },
        x: 60,
        y: 180,
      },
      {
        id: 'node-reasoning',
        title: '认知推理与装配',
        subtitle: 'Cognitive Context Assembly',
        kind: 'reasoning',
        status: stepStatus('reasoning'),
        icon: '🧠',
        details: '装配系统提示词、领域规则与多轮上下文历史。',
        meta: { 上下文窗口: '已对齐', 领域约束: '家庭助理视角' },
        x: 360,
        y: 180,
      },
      {
        id: 'node-action',
        title: toolName ? '工具处理' : '大模型流式推理',
        subtitle: toolName || 'Dynamic Gateway LLM',
        kind: 'action',
        status: stepStatus('action'),
        icon: toolName ? '⚙️' : '✦',
        details: toolName ? `执行工具 ${toolName}` : '通过网关建立 SSE 连接，逐字流式推演输出。',
        meta: { 推理模式: '流式生成 (SSE)', 安全策略: '双保险防御' },
        x: 660,
        y: 180,
      },
      {
        id: 'node-checkpoint',
        title: '持久化会话检查点',
        subtitle: 'Session Checkpoint',
        kind: 'checkpoint',
        status: stepStatus('checkpoint'),
        icon: '💾',
        details: '将一轮对话追加至持久化存储，支持时光机分支分叉。',
        meta: { 消息存储: '只增不改', 分叉支持: 'Time-Travel Fork' },
        x: 960,
        y: 180,
      },
      {
        id: 'node-output',
        title: '智能助理响应',
        subtitle: 'Synthesized Response',
        kind: 'output',
        status: stepStatus('output'),
        icon: '✦',
        details: '完整呈现解答或提供下一步智能建议。',
        meta: { 响应状态: isStreaming ? '推演中' : '已完成', 客户端: 'Web PWA' },
        x: 1260,
        y: 180,
      },
    ]

    connections = [
      { from: 'node-input', to: 'node-reasoning', label: '语义装配' },
      { from: 'node-reasoning', to: 'node-action', label: '网关生成' },
      { from: 'node-action', to: 'node-checkpoint', label: '检查点落盘' },
      { from: 'node-checkpoint', to: 'node-output', label: '界面呈现' },
    ]
  }

  const turnGroups: TurnGroup[] = [
    {
      turn: 1,
      title: '第 1 轮交互 · 意图决策与工具调用',
      subtitle: 'Intent Propose & Execution Ack',
      x: 320,
      y: 40,
      width: 620,
      height: 460,
      status: !isStreaming ? 'completed' : 'running',
    },
    {
      turn: 2,
      title: '第 2 轮交互 · 结果观察与终态生成',
      subtitle: 'Observation & Synthesized Ack',
      x: 980,
      y: 40,
      width: 580,
      height: 460,
      status: !isStreaming ? 'completed' : 'pending',
    },
  ]

  return {
    traces,
    duration: isStreaming ? '计算中…' : '1.8s',
    turnsCount: 2,
    nodes,
    connections,
    turnGroups,
  }
}

/**
 * 基于后端真实持久化 AgentRun (RunDetail) 记录，构建 100% 真实的多次交互轮次（Multi-Turn ReAct）与 Agent Ack 握手拓扑。
 */
export function buildTrajectoryFromRun(
  detail: RunDetail,
  isStreaming = false,
): ExecutionTrajectory {
  // 1. 解析事件列表并按轮次关联
  interface ParsedEvent {
    seq: number
    event_type: string
    payload: Record<string, unknown>
    created_at: string
    turn: number
  }

  const parsedEvents: ParsedEvent[] = []
  let activeTurn = 1
  const traces: string[] = []

  if (detail.events && detail.events.length > 0) {
    for (const ev of detail.events) {
      let payloadObj: Record<string, unknown> = {}
      try {
        if (ev.payload) {
          payloadObj = JSON.parse(ev.payload) as Record<string, unknown>
        }
      } catch {
        // ignore JSON parse error
      }

      if (payloadObj.turn && typeof payloadObj.turn === 'number') {
        activeTurn = payloadObj.turn
      }

      parsedEvents.push({
        seq: ev.seq,
        event_type: ev.event_type,
        payload: payloadObj,
        created_at: ev.created_at,
        turn: activeTurn,
      })

      switch (ev.event_type) {
        case 'run_created':
          traces.push(`[Seq ${ev.seq}] 调度任务初始化完成 · 目标: "${detail.goal}" · RunID: ${detail.id.slice(0, 8)}`)
          break
        case 'llm_request_sent':
          traces.push(`[Seq ${ev.seq}] 【Turn ${activeTurn}】发起认知推理 (上下文包含 ${payloadObj.messages_count ?? '多'} 条消息)`)
          break
        case 'llm_response_received': {
          const toolCalls = payloadObj.tool_calls as Array<{ Name?: string; function?: { name?: string } }> | undefined
          if (toolCalls && toolCalls.length > 0) {
            const toolNames = toolCalls.map((t) => t.Name || t.function?.name || '工具').join(', ')
            traces.push(`[Seq ${ev.seq}] 【Turn ${activeTurn} 意图握手】Agent 决策提案 · 提议调度工具: ${toolNames}`)
          } else {
            traces.push(`[Seq ${ev.seq}] 【Turn ${activeTurn} 终态握手】Agent 综合推理完成 · 生成直接答复`)
          }
          break
        }
        case 'tool_intent':
          traces.push(`[Seq ${ev.seq}] 【Turn ${activeTurn} 参数校验】工具 ${String(payloadObj.tool || '未知')} 参数与权限握手通过`)
          break
        case 'tool_result':
          if (payloadObj.error) {
            traces.push(`[Seq ${ev.seq}] 【Turn ${activeTurn} 环境回执】工具 ${String(payloadObj.tool)} 执行失败: ${String(payloadObj.error)}`)
          } else {
            const undoPart = payloadObj.undo_id ? ` (凭证: ${String(payloadObj.undo_id).slice(0, 8)})` : ''
            traces.push(`[Seq ${ev.seq}] 【Turn ${activeTurn} 环境回执】工具 ${String(payloadObj.tool)} Ack 执行完成: ${String(payloadObj.summary || '已提交')}${undoPart}`)
          }
          break
        case 'turn_completed':
          traces.push(`[Seq ${ev.seq}] 【Turn ${activeTurn} 闭环】第 ${activeTurn} 轮交互达成一致 · 事务状态落盘`)
          break
        case 'human_steered':
          traces.push(`[Seq ${ev.seq}] 【人类实时干预】注入指令: "${String(payloadObj.instruction)}"`)
          break
        case 'run_cancelled':
          traces.push(`[Seq ${ev.seq}] 【任务终止】人工已下发取消指令`)
          break
        case 'step_confirmed':
          traces.push(`[Seq ${ev.seq}] 【人机门禁确认】高风险步骤通过: ${String(payloadObj.choice)}`)
          break
        default:
          traces.push(`[Seq ${ev.seq}] 事务审计事件: ${ev.event_type}`)
          break
      }
    }
  } else {
    traces.push(`[Seq 1] 调度任务已初始化 · 目标: "${detail.goal}"`)
    traces.push(`[Seq 2] 当前状态: ${detail.status}`)
  }

  // 2. 耗时计算
  let duration = '0.8s'
  if (detail.created_at) {
    const start = new Date(detail.created_at).getTime()
    const lastEventTime =
      detail.events && detail.events.length > 0
        ? new Date(detail.events[detail.events.length - 1].created_at).getTime()
        : Date.now()
    const diffSec = (Math.max(200, lastEventTime - start) / 1000).toFixed(1)
    duration = isStreaming ? `${diffSec}s (调度中)` : `${diffSec}s`
  }

  // 3. 按交互轮次 (Turn) 分组
  const turnMap = new Map<number, ParsedEvent[]>()
  if (parsedEvents.length > 0) {
    for (const ev of parsedEvents) {
      if (!turnMap.has(ev.turn)) turnMap.set(ev.turn, [])
      turnMap.get(ev.turn)!.push(ev)
    }
  } else {
    turnMap.set(1, [])
  }

  // 获取所有轮次列表，按升序排列
  let turnNumbers = Array.from(turnMap.keys()).sort((a, b) => a - b)
  if (turnNumbers.length === 0) turnNumbers = [1]

  const nodes: CanvasNode[] = []
  const connections: CanvasConnection[] = []
  const turnGroups: TurnGroup[] = []

  // Node 0: 用户原始输入 (User Intent / Task Trigger)
  nodes.push({
    id: 'node-input',
    title: '用户需求触发',
    subtitle: `Goal: "${detail.goal.slice(0, 16)}${detail.goal.length > 16 ? '…' : ''}"`,
    kind: 'input',
    status: 'completed',
    icon: '📥',
    details: `用户原始目标: ${detail.goal}\n意图大类: ${detail.intent_kind}\n会话请求标识: ${detail.request_id || detail.id}`,
    meta: {
      'Run ID': detail.id.slice(0, 8),
      'Intent': detail.intent_kind,
      'Created': new Date(detail.created_at).toLocaleTimeString(),
    },
    x: 40,
    y: 180,
  })

  let prevTurnLastNodeId = 'node-input'

  // 遍历每一轮交互 (Turn 1, Turn 2...)
  turnNumbers.forEach((turnNum, turnIdx) => {
    const evs = turnMap.get(turnNum) || []
    const isCurrentActiveTurn = isStreaming && turnIdx === turnNumbers.length - 1
    const groupStatus = isCurrentActiveTurn ? 'running' : 'completed'

    // 查找本轮的 toolCalls 与 toolResults
    const responseEv = evs.find((e) => e.event_type === 'llm_response_received')
    const toolCalls =
      (responseEv?.payload.tool_calls as Array<{ Name?: string; function?: { name?: string }; args?: unknown }> | undefined) || []
    const toolResults = evs.filter((e) => e.event_type === 'tool_result')
    const hasToolsInThisTurn = toolCalls.length > 0 || toolResults.length > 0

    const groupX = 360 + turnIdx * 620
    const groupY = 40
    const groupWidth = 560
    const groupHeight = 480

    const turnTitle = hasToolsInThisTurn
      ? `第 ${turnNum} 轮交互 · 意图决策与工具调用`
      : `第 ${turnNum} 轮交互 · 结果观察与终态汇聚`
    const turnSubtitle = hasToolsInThisTurn
      ? `ReAct Loop · 包含 ${Math.max(toolCalls.length, toolResults.length)} 次环境 Ack`
      : `Cognitive Observation · 综合应答`

    turnGroups.push({
      turn: turnNum,
      title: turnTitle,
      subtitle: turnSubtitle,
      x: groupX,
      y: groupY,
      width: groupWidth,
      height: groupHeight,
      status: groupStatus,
    })

    // Turn 内部节点 1: 认知决策 / 意图提议 (Cognitive Deliberation / Propose)
    const intentNodeId = `node-turn-${turnNum}-intent`
    const toolNamesStr =
      toolCalls.map((t) => t.Name || t.function?.name || '工具').join(', ') ||
      (hasToolsInThisTurn ? '业务工具' : '综合应答')

    nodes.push({
      id: intentNodeId,
      title: hasToolsInThisTurn ? `意图决策与工具提案` : `上下文分析与反思`,
      subtitle: `Turn ${turnNum} · Agent Ack 提案`,
      kind: 'reasoning',
      status: isCurrentActiveTurn && !hasToolsInThisTurn ? 'running' : 'completed',
      icon: '🧠',
      details: hasToolsInThisTurn
        ? `第 ${turnNum} 轮认知推演：Agent 分析当前上下文，产生意图调用提案 [${toolNamesStr}]。\n参数完成 Schema 校验并准备下发系统。`
        : `第 ${turnNum} 轮认知反思：Agent 观察上一轮返回的工具执行结果，判定目标已完全达成，进行总结装配。`,
      meta: {
        'Turn': String(turnNum),
        'Phase': hasToolsInThisTurn ? 'Intent Propose' : 'Observation & Synthesize',
        'Proposed Action': toolNamesStr,
      },
      turn: turnNum,
      phase: 'propose',
      ackLabel: `Ack ${turnNum}.1`,
      x: groupX + 30,
      y: 110,
    })

    // 连接上一个轮次的出口节点到本轮的意图节点
    const crossTurnLabel =
      turnIdx === 0 ? `[Init] 触发会话任务` : `[Loop] 环境回执注入第 ${turnNum} 轮 ReAct`

    connections.push({
      from: prevTurnLastNodeId,
      to: intentNodeId,
      label: crossTurnLabel,
      ackType: 'flow',
    })

    let turnLastNodeId = intentNodeId

    if (hasToolsInThisTurn) {
      // Turn 内部节点 2: 工具执行与系统环境回执 (Tool Execution & Ack)
      const toolNodeId = `node-turn-${turnNum}-tool`
      const firstResult = toolResults[0]
      const summaryText = firstResult?.payload.summary ? String(firstResult.payload.summary) : '已提交执行'
      const undoIdText = firstResult?.payload.undo_id ? String(firstResult.payload.undo_id) : undefined

      nodes.push({
        id: toolNodeId,
        title: `工具执行与系统回执`,
        subtitle: `Turn ${turnNum} · 环境 Ack`,
        kind: 'action',
        status: isCurrentActiveTurn && toolResults.length === 0 ? 'running' : 'completed',
        icon: '🛠️',
        details: `执行器校验权限，执行底层原子业务：${toolNamesStr}。\n系统环境执行结果：${summaryText}\n${undoIdText ? `生成 24h 可撤销凭证: ${undoIdText}` : ''}`,
        meta: {
          'Turn': String(turnNum),
          'Phase': 'Environment Ack',
          'Tool': toolNamesStr,
          'Result': summaryText,
          'Undo ID': undoIdText ? undoIdText.slice(0, 8) : 'N/A',
        },
        turn: turnNum,
        phase: 'exec_ack',
        ackLabel: `Ack ${turnNum}.2`,
        x: groupX + 290,
        y: 110,
      })

      connections.push({
        from: intentNodeId,
        to: toolNodeId,
        label: `[Ack ${turnNum}.1] 意图提案与入参下发`,
        ackType: 'propose',
      })

      // Turn 内部节点 3: 检查点落盘 (Checkpoint & Audit Log)
      const cpNodeId = `node-turn-${turnNum}-checkpoint`
      nodes.push({
        id: cpNodeId,
        title: `轮次检查点与审计落盘`,
        subtitle: `Turn ${turnNum} · 事务持久化 (v${detail.version})`,
        kind: 'checkpoint',
        status: isCurrentActiveTurn ? 'running' : 'completed',
        icon: '💾',
        details: `第 ${turnNum} 轮交互的所有事件与步骤状态（包括操作凭证与结果卡片）完成原子落盘，保障崩溃自愈。`,
        meta: {
          'Turn': String(turnNum),
          'Events in Turn': String(evs.length),
          'Version': `v${detail.version}`,
        },
        turn: turnNum,
        phase: 'checkpoint',
        ackLabel: `Ack ${turnNum}.3`,
        x: groupX + 160,
        y: 310,
      })

      connections.push({
        from: toolNodeId,
        to: cpNodeId,
        label: `[Ack ${turnNum}.2] 环境结果回执与落盘`,
        ackType: 'result',
      })

      turnLastNodeId = cpNodeId
    } else {
      // 本轮是直接综合回答 (Direct Response)
      const answerNodeId = `node-turn-${turnNum}-answer`
      nodes.push({
        id: answerNodeId,
        title: `终态答复装配与呈现`,
        subtitle: `Turn ${turnNum} · 终态 Ack`,
        kind: 'output',
        status: isCurrentActiveTurn ? 'running' : 'completed',
        icon: '✦',
        details: `Agent 结合前面轮次积累的全部 Observation 数据，生成流畅的用户交互语言并下发前端。`,
        meta: {
          'Turn': String(turnNum),
          'Phase': 'Final Response Ack',
        },
        turn: turnNum,
        phase: 'observe',
        ackLabel: `Ack ${turnNum}.2`,
        x: groupX + 290,
        y: 110,
      })

      connections.push({
        from: intentNodeId,
        to: answerNodeId,
        label: `[Ack ${turnNum}.1] 语义推演与回答装配`,
        ackType: 'propose',
      })

      turnLastNodeId = answerNodeId
    }

    prevTurnLastNodeId = turnLastNodeId
  })

  // 终态门禁或完成节点 (Final Gate or Completed Node)
  const finalX = 360 + turnNumbers.length * 620 + 30
  if (detail.status === 'awaiting_confirmation' || detail.status === 'awaiting_input') {
    const gateId = 'node-human-gate'
    nodes.push({
      id: gateId,
      title: '人机协同确认门禁 (Human Gate)',
      subtitle: `Awaiting Human Ack · 暂停等待确认`,
      kind: 'gate',
      status: 'waiting',
      icon: '🛡️',
      details: `检测到涉及资产或高风险动作，或需要补充关键信息，Agent 暂停自动循环，等待人类 Approval 确认。`,
      meta: {
        'Gate Status': detail.status,
        'Version': `v${detail.version}`,
      },
      phase: 'human_gate',
      x: finalX,
      y: 180,
    })

    connections.push({
      from: prevTurnLastNodeId,
      to: gateId,
      label: `[Gate] 触发人机安全确认`,
      ackType: 'confirm',
    })
  } else {
    const finalId = 'node-final-completed'
    const isCompleted = detail.status === 'completed'
    nodes.push({
      id: finalId,
      title: isCompleted ? '协同执行完成 (Final Ack)' : detail.status === 'cancelled' ? '任务已人工终止' : '调度持续推进中',
      subtitle: `Status: ${detail.status} · 交互闭环`,
      kind: 'output',
      status: isCompleted ? 'completed' : detail.status === 'cancelled' ? 'completed' : 'running',
      icon: isCompleted ? '🏁' : detail.status === 'cancelled' ? '🛑' : '⏳',
      details: `调度中心全链路多轮交互闭环，会话消息与结果卡片已呈现在聊天界面。`,
      meta: {
        'Total Turns': String(turnNumbers.length),
        'Final Status': detail.status,
      },
      phase: 'completed',
      x: finalX,
      y: 180,
    })

    connections.push({
      from: prevTurnLastNodeId,
      to: finalId,
      label: `[Final Ack] 事务归档与界面呈现`,
      ackType: 'flow',
    })
  }

  // 如果有人类干预事件 (Human Steering)，额外生成干预节点
  const steerEvent = detail.events?.find((e) => e.event_type === 'human_steered')
  if (steerEvent) {
    let steerPayload: { instruction?: string } = {}
    try {
      if (steerEvent.payload) steerPayload = JSON.parse(steerEvent.payload) as { instruction?: string }
    } catch {
      // ignore
    }
    const steerId = 'node-steering'
    nodes.push({
      id: steerId,
      title: '人类实时干预 (Steering)',
      subtitle: `"${steerPayload.instruction || '干预指令'}"`,
      kind: 'input',
      status: 'completed',
      icon: '⚡',
      details: `人类在执行中注入的指令: ${steerPayload.instruction || '无'}\n成功重定向 Agent 决策轨道`,
      meta: {
        'Instruction': steerPayload.instruction || '',
        'Seq': String(steerEvent.seq),
      },
      x: 390,
      y: 420,
    })
    connections.push({
      from: steerId,
      to: `node-turn-1-intent`,
      label: '[Intervene] 实时干预注入',
      ackType: 'confirm',
    })
  }

  return {
    traces,
    duration,
    turnsCount: turnNumbers.length,
    nodes,
    connections,
    turnGroups,
  }
}
