/** 记账页（page-03-money）：流水 + 汇总，金额分→元展示 */
import { useEffect, useState } from 'react'

import { formatYuan, formatYuanGrouped } from '@homeagent/shared'

import { api, unwrap } from '../../api/client'

import styles from './MoneyPage.module.css'

interface Expense {
  id: string
  category: string
  amountCents: number
  hint: string
  time: string
}

export function MoneyPage() {
  const [items, setItems] = useState<Expense[]>([])
  const [totalCents, setTotalCents] = useState(0)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    void (async () => {
      try {
        const summary = unwrap(await api.GET('/expenses/summary'))
        setTotalCents(summary.total_cents)
        const list = unwrap(
          await api.GET('/expenses', { params: { query: { page_size: 50 } } }),
        )
        setItems(
          list.items.map((e) => ({
            id: e.id,
            category: e.category ?? '其他',
            amountCents: e.amount_cents,
            hint: e.hint ?? '',
            time: formatDay(e.occurred_at),
          })),
        )
      } catch {
        // 加载失败保留空列表，不阻塞页面
      } finally {
        setLoading(false)
      }
    })()
  }, [])

  return (
    <div>
      <h1 className={styles.title}>记账</h1>

      <div className={styles.summary}>
        <div className={styles.summaryLabel}>本月支出</div>
        <div className={styles.summaryAmount}>{formatYuanGrouped(totalCents)}</div>
      </div>

      <div className={styles.list}>
        {items.map((e) => (
          <div key={e.id} className={styles.item}>
            <div className={styles.main}>
              <div className={styles.category}>{e.category}</div>
              <div className={styles.hint}>
                {e.hint} · {e.time}
              </div>
            </div>
            <div className={styles.amount}>{formatYuan(e.amountCents)}</div>
          </div>
        ))}
        {items.length === 0 && !loading && <div className={styles.empty}>暂无流水，去对话里记一笔</div>}
      </div>
    </div>
  )
}

/** RFC3339 → M/D HH:MM 展示 */
function formatDay(iso: string): string {
  const t = new Date(iso)
  if (Number.isNaN(t.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${t.getMonth() + 1}/${t.getDate()} ${pad(t.getHours())}:${pad(t.getMinutes())}`
}
