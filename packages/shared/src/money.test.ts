import { describe, expect, it } from 'vitest'

import { formatMoney, formatYuan, formatYuanCompact, formatYuanGrouped, yuanToCents } from './money'

describe('money', () => {
  it('分 → 元保留两位小数', () => {
    expect(formatMoney(1250)).toBe('12.50')
    expect(formatMoney(5)).toBe('0.05')
  })

  it('带 ¥ 符号', () => {
    expect(formatYuan(1250)).toBe('¥12.50')
  })

  it('整数元紧凑展示', () => {
    expect(formatYuanCompact(12000)).toBe('¥120')
    expect(formatYuanCompact(1250)).toBe('¥12.50')
  })

  it('千分位分组', () => {
    expect(formatYuanGrouped(1234567)).toBe('¥12,345.67')
  })

  it('元 → 分四舍五入防浮点误差', () => {
    expect(yuanToCents(12.5)).toBe(1250)
    expect(yuanToCents(0.1 + 0.2)).toBe(30)
  })
})
