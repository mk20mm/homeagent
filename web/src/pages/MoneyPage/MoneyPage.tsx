/** 记账页（page-03-money）：流水 + 汇总，金额分→元展示 */
import { formatYuan, formatYuanGrouped } from '@homeagent/shared'

import styles from './MoneyPage.module.css'

interface Expense {
  id: string
  category: string
  amountCents: number
  hint: string
}

// 骨架阶段静态数据；后续接 GET /expenses（按成员隔离）
const MOCK: Expense[] = [
  { id: '1', category: '食材', amountCents: 12000, hint: '买菜' },
  { id: '2', category: '出行', amountCents: 3500, hint: '打车' },
  { id: '3', category: '外卖', amountCents: 2800, hint: '午餐外卖' },
]

export function MoneyPage() {
  const total = MOCK.reduce((sum, e) => sum + e.amountCents, 0)

  return (
    <div>
      <h1 className={styles.title}>记账</h1>

      <div className={styles.summary}>
        <div className={styles.summaryLabel}>本月支出</div>
        <div className={styles.summaryAmount}>{formatYuanGrouped(total)}</div>
      </div>

      <div className={styles.list}>
        {MOCK.map((e) => (
          <div key={e.id} className={styles.item}>
            <div className={styles.main}>
              <div className={styles.category}>{e.category}</div>
              <div className={styles.hint}>{e.hint}</div>
            </div>
            <div className={styles.amount}>{formatYuan(e.amountCents)}</div>
          </div>
        ))}
      </div>
    </div>
  )
}
