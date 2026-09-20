/** 记账页（page-03-money）：流水 + 汇总，金额分→元展示；「记一笔」FAB 快捷记账 */
import { useCallback, useEffect, useRef, useState } from 'react'
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
  occurredAt: string
}

interface DayGroup {
  key: string
  label: string
  items: Expense[]
  subtotal: number
}

const CATEGORIES = ['食材', '日用', '外卖', '出行', '餐饮', '其他']
type RangeKey = 'all' | 'week' | 'month'
const RANGE_LABELS: Record<RangeKey, string> = { all: '全部', week: '本周', month: '本月' }

function rangeToDates(range: RangeKey): { start?: string; end?: string } {
  if (range === 'all') return {}
  const now = new Date()
  const start = new Date(now)
  if (range === 'week') start.setDate(now.getDate() - now.getDay()) // 本周日开始
  if (range === 'month') start.setDate(1)
  start.setHours(0, 0, 0, 0)
  return { start: toISODate(start) }
}

function toISODate(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

export function MoneyPage() {
  const [items, setItems] = useState<Expense[]>([])
  const [totalCents, setTotalCents] = useState(0)
  const [loading, setLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [nextCursor, setNextCursor] = useState<string | null>(null)
  const [category, setCategory] = useState<string>('')
  const [range, setRange] = useState<RangeKey>('all')
  const [highlightId, setHighlightId] = useState<string | null>(null)
  const [sheetOpen, setSheetOpen] = useState(false)
  const [draftAmount, setDraftAmount] = useState('')
  const [draftHint, setDraftHint] = useState('')
  const [draftCategory, setDraftCategory] = useState('')
  const [saving, setSaving] = useState(false)
  const [sheetError, setSheetError] = useState<string | null>(null)
  const [searchParams] = useSearchParams()
  const itemRefs = useRef<Record<string, HTMLDivElement | null>>({})

  const fetchPage = useCallback(
    async (cursor: string | null, acc: Expense[]) => {
      const dates = rangeToDates(range)
      const list = unwrap(
        await api.GET('/expenses', {
          params: {
            query: {
              page_size: 20,
              ...(cursor ? { cursor } : {}),
              ...(category ? { category } : {}),
              ...dates,
            },
          },
        }),
      )
      const mapped = list.items.map((e) => ({
        id: e.id,
        category: e.category ?? '其他',
        amountCents: e.amount_cents,
        hint: e.hint ?? '',
        time: formatClock(e.occurred_at),
        occurredAt: e.occurred_at,
      }))
      setItems(acc.concat(mapped))
      setNextCursor(list.next_cursor ?? null)
    },
    [category, range],
  )

  const reload = useCallback(async () => {
    setLoading(true)
    try {
      // 汇总固定按本月（与筛选解耦，展示「家里本月花了多少」）
      const summary = unwrap(await api.GET('/expenses/summary'))
      setTotalCents(summary.total_cents)
      await fetchPage(null, [])
    } catch {
      // 加载失败保留空列表，不阻塞页面
    } finally {
      setLoading(false)
    }
  }, [fetchPage])

  useEffect(() => {
    void reload()
  }, [reload])

  const loadMore = async () => {
    if (!nextCursor || loadingMore) return
    setLoadingMore(true)
    try {
      await fetchPage(nextCursor, items)
    } catch {
      // 翻页失败静默保留已加载内容
    } finally {
      setLoadingMore(false)
    }
  }

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

  const filtered = category !== '' || range !== 'all'
  const groups = groupByDay(items)
  const filteredCents = items.reduce((sum, e) => sum + e.amountCents, 0)

  return (
    <div>
      <h1 className={styles.title}>记账</h1>

      <div className={styles.summary}>
        <div className={styles.summaryLabel}>
          {filtered ? '当前筛选合计' : '本月支出'}
        </div>
        <div className={styles.summaryAmount}>
          {formatYuanGrouped(filtered ? filteredCents : totalCents)}
        </div>
        {filtered && (
          <div className={styles.summaryHint}>
            共 {items.length} 笔 · 汇总仍按本月（{formatYuanGrouped(totalCents)}）
          </div>
        )}
      </div>

      <div className={styles.filterBar}>
        <div className={styles.chips}>
          {RANGE_LABELS &&
            (Object.keys(RANGE_LABELS) as RangeKey[]).map((r) => (
              <button
                type="button"
                key={r}
                className={`${styles.chip} ${range === r ? styles.chipOn : ''}`}
                onClick={() => setRange(r)}
              >
                {RANGE_LABELS[r]}
              </button>
            ))}
        </div>
        <div className={styles.chips}>
          <button
            type="button"
            className={`${styles.chip} ${category === '' ? styles.chipOn : ''}`}
            onClick={() => setCategory('')}
          >
            全部分类
          </button>
          {CATEGORIES.map((c) => (
            <button
              type="button"
              key={c}
              className={`${styles.chip} ${category === c ? styles.chipOn : ''}`}
              onClick={() => setCategory(category === c ? '' : c)}
            >
              {c}
            </button>
          ))}
        </div>
      </div>

      <div className={styles.list}>
        {groups.map((g) => (
          <div key={g.key} className={styles.dayGroup}>
            <div className={styles.dayHead}>
              <span>{g.label}</span>
              <span className={styles.daySubtotal}>小计 {formatYuan(g.subtotal)}</span>
            </div>
            {g.items.map((e) => (
              <div
                key={e.id}
                ref={(el) => {
                  itemRefs.current[e.id] = el
                }}
                className={`${styles.item} ${highlightId === e.id ? styles.highlight : ''}`}
              >
                <div className={styles.main}>
                  <div className={styles.category}>{e.category}</div>
                  <div className={styles.hint}>{e.hint}</div>
                </div>
                <div className={styles.amount}>{formatYuan(e.amountCents)}</div>
              </div>
            ))}
          </div>
        ))}
        {items.length === 0 && !loading && (
          <div className={styles.empty}>
            {filtered ? '这个范围没有流水，换个筛选看看' : '暂无流水，点右下角记一笔'}
          </div>
        )}
        {nextCursor && (
          <button
            type="button"
            className={styles.loadMore}
            disabled={loadingMore}
            onClick={() => void loadMore()}
          >
            {loadingMore ? '加载中…' : '加载更多'}
          </button>
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

/** 按天分组（倒序），每组算小计；label 给「今天/昨天/M/D */
function groupByDay(items: Expense[]): DayGroup[] {
  const buckets = new Map<string, Expense[]>()
  for (const e of items) {
    // 用本地时区的日期做分组键（occurred_at 可能带 Z 或 +08:00，直接切片会错一天）
    const key = localDateKey(e.occurredAt)
    const arr = buckets.get(key) ?? []
    arr.push(e)
    buckets.set(key, arr)
  }
  const today = localDateKey(new Date().toISOString())
  const yest = new Date()
  yest.setDate(yest.getDate() - 1)
  const yesterday = localDateKey(yest.toISOString())

  return [...buckets.keys()]
    .sort()
    .reverse()
    .map((key) => {
      const list = buckets.get(key) ?? []
      let label = key.slice(5).replace('-', '/') // M/D
      if (key === today) label = '今天'
      else if (key === yesterday) label = '昨天'
      return {
        key,
        label,
        items: list,
        subtotal: list.reduce((s, e) => s + e.amountCents, 0),
      }
    })
}

/** RFC3339（任意时区）→ 本地时区 YYYY-MM-DD */
function localDateKey(iso: string): string {
  const t = new Date(iso)
  if (Number.isNaN(t.getTime())) return iso.slice(0, 10)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${t.getFullYear()}-${pad(t.getMonth() + 1)}-${pad(t.getDate())}`
}

/** RFC3339 → HH:MM 展示（日期归到分组头，行内只留时间） */
function formatClock(iso: string): string {
  const t = new Date(iso)
  if (Number.isNaN(t.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(t.getHours())}:${pad(t.getMinutes())}`
}
