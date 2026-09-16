/**
 * 金额工具：后端存「分」（Int），前端展示「元」。
 * 财务系统标准做法：最小货币单位整数存储，避免 float 精度漂移。
 */

/** 分 → 元（展示用，保留 2 位小数） */
export function formatMoney(cents: number): string {
  return (cents / 100).toFixed(2)
}

/** 分 → 元（带 ¥ 符号） */
export function formatYuan(cents: number): string {
  return `¥${formatMoney(cents)}`
}

/** 分 → 中文紧凑展示：12000 → ¥120 */
export function formatYuanCompact(cents: number): string {
  const yuan = cents / 100
  return Number.isInteger(yuan) ? `¥${yuan}` : `¥${yuan.toFixed(2)}`
}

/** 元 → 分（输入用，四舍五入防浮点误差） */
export function yuanToCents(yuan: number): number {
  return Math.round(yuan * 100)
}

/** 千分位分 → 元，如 1234567 → ¥12,345.67 */
export function formatYuanGrouped(cents: number): string {
  return `¥${(cents / 100).toLocaleString('zh-CN', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })}`
}
