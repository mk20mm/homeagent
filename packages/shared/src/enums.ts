/**
 * 前后端共享枚举。与后端 internal/agent/tool、ent schema、openapi.yaml 对齐。
 * 改动需同步三处（CONVENTIONS-frontend §6：类型来自生成 client，枚举在此统一）。
 */

/** 危险分级：控 ReAct 循环上限与确认策略（ARCHITECTURE §8） */
export type RiskLevel = 'low' | 'medium' | 'high'

export const RISK_LABEL: Record<RiskLevel, string> = {
  low: '低',
  medium: '中',
  high: '高',
}

export const MAX_TURNS: Record<RiskLevel, number> = {
  low: 15,
  medium: 8,
  high: 3,
}

/** 家务任务状态机：待认领→进行中→已完成 */
export type TaskStatus = 'pending' | 'in_progress' | 'done'

export const TASK_STATUS_LABEL: Record<TaskStatus, string> = {
  pending: '待认领',
  in_progress: '进行中',
  done: '已完成',
}

/** 消息角色 */
export type MessageRole = 'user' | 'assistant' | 'tool' | 'system'

/** 对话状态机（CONVENTIONS-frontend §3） */
export type ChatStatus = 'idle' | 'streaming' | 'tool_running' | 'error'

/** 错误码，对齐后端 apperr.Code / openapi Error.code */
export type ErrorCode =
  | 'invalid_input'
  | 'invalid_credentials'
  | 'unauthorized'
  | 'permission_denied'
  | 'not_found'
  | 'conflict'
  | 'internal'

export const ERROR_MESSAGE: Record<ErrorCode, string> = {
  invalid_input: '输入有误，请检查后重试',
  invalid_credentials: '用户名或令牌错误，请联系家庭管理员',
  unauthorized: '登录已过期，请重新登录',
  permission_denied: '没有权限执行此操作',
  not_found: '找不到对应内容',
  conflict: '操作已执行过，请勿重复提交',
  internal: '系统内部错误，请稍后重试',
}

/** 家庭成员角色 */
export type MemberRole = 'parent' | 'adult' | 'elder' | 'child' | 'roommate'

export const MEMBER_ROLE_LABEL: Record<MemberRole, string> = {
  parent: '家长',
  adult: '成人',
  elder: '老人',
  child: '小孩',
  roommate: '合租',
}

/** LLM 供应商 */
export type ProviderName = 'openai' | 'anthropic' | 'deepseek' | 'zhipu' | 'ollama'

export const PROVIDER_LABEL: Record<ProviderName, string> = {
  openai: 'OpenAI',
  anthropic: 'Anthropic',
  deepseek: 'DeepSeek',
  zhipu: '智谱',
  ollama: 'Ollama（本地）',
}
