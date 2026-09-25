/** 记账页（page-03-money）：流水 + 汇总，金额分→元展示；「记一笔」FAB 快捷记账 */
import { useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router'

import { formatYuan, formatYuanGrouped, yuanToCents } from '@homeagent/shared'

import { api, unwrap } from '../../api/client'

import styles from './MoneyPage.module.css'

interface Expense {
  id: string
  category: string
  amountCents: number
  hint: string
  time: string
}

const CATEGORIES = ['食材', '日用', '外卖', '出行', '餐饮', '其他']

const CATEGORY_ICONS: Record<string, string> = {
  食材: '🥦',
  日用: '🧻',
  外卖: '🛵',
  出行: '🚗',
  餐饮: '🍲',
  其他: '💳',
}

export function MoneyPage() {
  const [items, setItems] = useState<Expense[]>([])
  const [totalCents, setTotalCents] = useState(0)
  const [loading, setLoading] = useState(true)
  const [highlightId, setHighlightId] = useState<string | null>(null)
  const [sheetOpen, setSheetOpen] = useState(false)
  const [draftAmount, setDraftAmount] = useState('')
  const [draftHint, setDraftHint] = useState('')
  const [draftCategory, setDraftCategory] = useState('')
  const [saving, setSaving] = useState(false)
  const [sheetError, setSheetError] = useState<string | null>(null)
  const [searchParams] = useSearchParams()
  const itemRefs = useRef<Record<string, HTMLDivElement | null>>({})

  const reload = async () => {
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
  }

  useEffect(() => {
    void reload()
  }, [])

  // 从对话卡片「在账本中查看」跳来：高亮目标并滚动定位
  useEffect(() => {
    const focusId = searchParams.get('focus')
    if (!focusId) return
    setHighlightId(focusId)
    const el = itemRefs.current[focusId]
    if (el) {
      el.scrollIntoView({ block: 'center', behavior: 'smooth' })
    }
    const timer = setTimeout(() => setHighlightId(null), 2400)
    return () => clearTimeout(timer)
  }, [searchParams, items])

  const openSheet = () => {
    setDraftAmount('')
    setDraftHint('')
    setDraftCategory('')
    setSheetError(null)
    setSheetOpen(true)
  }

  const handleSave = async () => {
    const amount = Number(draftAmount.trim())
    if (!Number.isFinite(amount) || amount <= 0) {
      setSheetError('金额要大于 0')
      return
    }
    const hint = draftHint.trim()
    if (!hint) {
      setSheetError('写一下买了什么')
      return
    }

    setSaving(true)
    setSheetError(null)
    try {
      const cents = yuanToCents(amount)
      const data = unwrap(
        await api.POST('/expenses', {
          body: {
            amount_cents: cents,
            hint,
            category: draftCategory || undefined,
          },
        }),
      )
      await reload()
      setHighlightId(data.id)
      const el = itemRefs.current[data.id]
      if (el) el.scrollIntoView({ block: 'center', behavior: 'smooth' })
      setTimeout(() => setHighlightId(null), 2400)
      setSheetOpen(false)
    } catch {
      setSheetError('保存失败，稍后再试')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div>
      <h1 className={styles.title}>记账</h1>

      <div className={styles.summary}>
        <div className={styles.summaryLabel}>本月支出</div>
        <div className={styles.summaryAmount}>{formatYuanGrouped(totalCents)}</div>
      </div>

      <div className={styles.list}>
        {items.map((e) => (
          <div
            key={e.id}
            ref={(el) => {
              itemRefs.current[e.id] = el
            }}
            className={`${styles.item} ${highlightId === e.id ? styles.highlight : ''}`}
          >
            <div className={styles.categoryIconBox}>
              {CATEGORY_ICONS[e.category] ?? '💳'}
            </div>
            <div className={styles.main}>
              <div className={styles.category}>{e.category}</div>
              <div className={styles.hint}>
                {e.hint ? `${e.hint} · ` : ''}{e.time}
              </div>
            </div>
            <div className={styles.amount}>-{formatYuan(e.amountCents)}</div>
          </div>
        ))}
        {items.length === 0 && !loading && (
          <div className={styles.empty}>暂无流水，点右下角记一笔</div>
        )}
      </div>

      <button
        type="button"
        className={styles.fab}
        aria-label="记一笔"
        onClick={openSheet}
      >
        ＋
      </button>

      {sheetOpen && (
        <div className={styles.sheetOverlay} onClick={() => setSheetOpen(false)}>
          <div className={styles.sheet} onClick={(e) => e.stopPropagation()}>
            <div className={styles.dragHandle} />
            <div className={styles.sheetTitle}>记一笔</div>

            <div className={styles.sheetRow}>
              <input
                className={styles.sheetInput}
                inputMode="decimal"
                placeholder="金额（元）"
                value={draftAmount}
                onChange={(e) => setDraftAmount(e.target.value)}
                autoFocus
              />
            </div>
            <div className={styles.sheetRow}>
              <input
                className={styles.sheetInput}
                placeholder="买了什么（如：买菜）"
                value={draftHint}
                onChange={(e) => setDraftHint(e.target.value)}
              />
            </div>
            <div className={styles.chips}>
              {CATEGORIES.map((c) => (
                <button
                  type="button"
                  key={c}
                  className={`${styles.chip} ${draftCategory === c ? styles.chipOn : ''}`}
                  onClick={() => setDraftCategory(draftCategory === c ? '' : c)}
                >
                  {c}
                </button>
              ))}
            </div>

            {sheetError && <div className={styles.sheetError}>{sheetError}</div>}

            <button
              type="button"
              className={styles.sheetSave}
              disabled={saving}
              onClick={() => void handleSave()}
            >
              {saving ? '保存中…' : '保存'}
            </button>
          </div>
        </div>
      )}
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
