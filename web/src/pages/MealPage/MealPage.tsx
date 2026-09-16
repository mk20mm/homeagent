/** 报饭页（page-04-meal）：申报今晚是否在家用餐 + 汇总给做饭人 */
import { useState } from 'react'

import styles from './MealPage.module.css'

interface MemberMeal {
  memberId: string
  name: string
  atHome: boolean
}

// 骨架阶段静态数据；后续接 GET /meals（今日汇总）
const MOCK: MemberMeal[] = [
  { memberId: '1', name: '爸爸', atHome: true },
  { memberId: '2', name: '奶奶', atHome: true },
  { memberId: '3', name: '孩子', atHome: false },
]

export function MealPage() {
  const [mine, setMine] = useState(true)
  const atHome = MOCK.filter((m) => m.atHome).length

  return (
    <div>
      <h1 className={styles.title}>报饭</h1>

      <div className={styles.today}>
        <div className={styles.count}>今晚 {atHome} 人在家吃</div>
        <div className={styles.sub}>不包含明确未报者以外推断</div>
      </div>

      <div className={styles.section}>我的申报</div>
      <div className={styles.actions}>
        <button
          type="button"
          className={`${styles.btn} ${mine ? styles.btnOn : styles.btnOff}`}
          onClick={() => setMine(true)}
        >
          🏠 在家吃
        </button>
        <button
          type="button"
          className={`${styles.btn} ${!mine ? styles.btnOffActive : styles.btnOff}`}
          onClick={() => setMine(false)}
        >
          🚪 不在家吃
        </button>
      </div>

      <div className={styles.section}>家人申报</div>
      <div className={styles.list}>
        {MOCK.map((m) => (
          <div key={m.memberId} className={styles.item}>
            <span className={styles.name}>{m.name}</span>
            <span className={m.atHome ? styles.tagOn : styles.tagOff}>
              {m.atHome ? '在家吃' : '不在家'}
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}
