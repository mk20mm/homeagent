/**
 * 快捷 chips：0 输入路径（T-A05）。
 *
 * 设计约束：
 * - 候选只来自既有工具，不引入新后端能力（决策日志：快捷 chips 候选来自既有工具与高频动作）
 * - chips 按成员权限过滤：tool 不在 GET /tools 返回清单里就不出现，避免「点了才发现没权限」
 * - 需要补全参数的动作用 draft：点击只填入输入框并聚焦，不直接发送（如 记一笔 还缺金额）
 * - 时段加权：报饭类在晚间排序靠前；非活跃时段的 chip 排到后面但不消失（家人可能补报）
 */
import type { components } from '../../api/schema'

export type ToolSpec = components['schemas']['ToolSpec']

export interface QuickChip {
  /** chip 显示文案 */
  label: string
  /** 点击后发送或填入的消息内容 */
  text: string
  /** 命中的工具名（权限过滤的键） */
  tool: string
  /** true=只填入输入框并聚焦（参数待补全）；false=直接发送 */
  draft?: boolean
  /** 活跃时段（24h，[from, to) 闭开区间），不填=常驻 */
  peakFrom?: number
  peakTo?: number
}

/** chip 候选全集（顺序即默认展示顺序） */
export const QUICK_CHIPS: QuickChip[] = [
  {
    label: '记一笔',
    text: '记一笔：买菜花了 ',
    tool: 'record_expense',
    draft: true,
  },
  {
    label: '今晚不回家吃',
    text: '今晚我不回家吃',
    tool: 'report_meal',
    peakFrom: 15,
    peakTo: 22,
  },
  {
    label: '今晚在家吃',
    text: '今晚我在家吃',
    tool: 'report_meal',
    peakFrom: 15,
    peakTo: 22,
  },
  {
    label: '提醒谁洗碗',
    text: '提醒 洗碗',
    tool: 'assign_task',
    draft: true,
  },
  {
    label: '洗碗打卡',
    text: '我洗完碗了，打卡',
    tool: 'complete_task',
  },
  {
    label: '我的任务',
    text: '我名下还有什么任务？',
    tool: 'list_my_tasks',
  },
  {
    label: '本月预算',
    text: '这个月预算还剩多少？',
    tool: 'query_budget',
  },
]

/** 欢迎页可点示例（空态教学，一次点击 = 一次教学） */
export const WELCOME_EXAMPLES: QuickChip[] = [
  {
    label: '今天买菜花了 120',
    text: '今天买菜花了 120',
    tool: 'record_expense',
  },
  {
    label: '今晚不回家吃',
    text: '今晚我不回家吃',
    tool: 'report_meal',
  },
  {
    label: '提醒媳妇洗碗',
    text: '提醒媳妇洗碗',
    tool: 'assign_task',
  },
]

/** 当前小时是否落在 chip 的活跃时段内 */
function isPeak(chip: QuickChip, hour: number): boolean {
  if (chip.peakFrom === undefined || chip.peakTo === undefined) return true
  return hour >= chip.peakFrom && hour < chip.peakTo
}

/**
 * 按成员工具清单过滤 chip：只保留有权限工具的项。
 * 活跃时段的 chip 排前面（稳定排序，同段内保持声明顺序）。
 */
export function filterChips(
  chips: QuickChip[],
  tools: ToolSpec[],
  now = new Date(),
): QuickChip[] {
  const names = new Set(tools.map((t) => t.name))
  const hour = now.getHours()
  return chips
    .filter((c) => names.has(c.tool))
    .sort((a, b) => {
      const pa = isPeak(a, hour) ? 0 : 1
      const pb = isPeak(b, hour) ? 0 : 1
      return pa - pb
    })
}
