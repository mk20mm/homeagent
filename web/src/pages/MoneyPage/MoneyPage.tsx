/** 财务页（page-03-money）：收支汇总 + 结余 Apple Card + 年月切换 + 收支流水 + 快捷记一笔 */
import { useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router'

import { formatYuan, yuanToCents } from '@homeagent/shared'

import { api, unwrap } from '../../api/client'

import styles from './MoneyPage.module.css'

interface TransactionItem {
  id: string
  type: 'expense' | 'income'
  categoryOrSource: string
  amountCents: number
  hint: string
  occurredAt: string
  timeStr: string
}

interface FinanceSummary {
  totalIncomeCents: number
  totalExpenseCents: number
  balanceCents: number
}

const EXPENSE_CATEGORIES = ['食材', '日用', '外卖', '出行', '餐饮', '其他']

const EXPENSE_ICONS: Record<string, string> = {
  食材: '🥦',
  日用: '🧻',
  外卖: '🛵',
  出行: '🚗',
  餐饮: '🍲',
  其他: '💳',
}

const INCOME_SOURCES = ['工资', '奖金', '兼职', '报销', '退款', '红包', '利息', '其他']

const INCOME_ICONS: Record<string, string> = {
  工资: '💼',
  奖金: '🎁',
  兼职: '💻',
  报销: '🧾',
  退款: '↩️',
  红包: '🧧',
  利息: '📈',
  其他: '💵',
}

export function MoneyPage() {
  const now = new Date()
  const currentYear = now.getFullYear()
  const currentMonth = now.getMonth() + 1

  // 年月过滤状态：month === 0 表示全年视图
  const [year, setYear] = useState(currentYear)
  const [month, setMonth] = useState(currentMonth)
  const [pickerOpen, setPickerOpen] = useState(false)

  // 流水与汇总状态
  const [items, setItems] = useState<TransactionItem[]>([])
  const [summary, setSummary] = useState<FinanceSummary>({
    totalIncomeCents: 0,
    totalExpenseCents: 0,
    balanceCents: 0,
  })
  const [loading, setLoading] = useState(true)
  const [tabFilter, setTabFilter] = useState<'all' | 'expense' | 'income'>('all')
  const [highlightId, setHighlightId] = useState<string | null>(null)

  // 快捷记账/记收入浮层状态
  const [sheetOpen, setSheetOpen] = useState(false)
  const [sheetType, setSheetType] = useState<'expense' | 'income'>('expense')
  const [draftAmount, setDraftAmount] = useState('')
  const [draftHint, setDraftHint] = useState('')
  const [draftCategory, setDraftCategory] = useState('')
  const [draftSource, setDraftSource] = useState('工资')
  const [draftDate, setDraftDate] = useState(() => formatYMD(new Date()))
  const [saving, setSaving] = useState(false)
  const [sheetError, setSheetError] = useState<string | null>(null)

  const [searchParams] = useSearchParams()
  const itemRefs = useRef<Record<string, HTMLDivElement | null>>({})

  const reload = async (y = year, m = month) => {
    try {
      // 1. 获取综合财务总览 (结余 = 收入 - 支出)
      const period = m === 0 ? 'year' : 'month'
      const finSummary = unwrap(
        await api.GET('/finance/summary', {
          params: {
            query: {
              year: y,
              month: m > 0 ? m : undefined,
              period,
            },
          },
        }),
      )
      setSummary({
        totalIncomeCents: finSummary.total_income_cents,
        totalExpenseCents: finSummary.total_expense_cents,
        balanceCents: finSummary.balance_cents,
      })

      // 2. 并行获取支出与收入流水
      const [expRes, incRes] = await Promise.all([
        api.GET('/expenses', {
          params: {
            query: {
              page_size: 50,
              year: y,
              month: m > 0 ? m : undefined,
            },
          },
        }),
        api.GET('/incomes', {
          params: {
            query: {
              page_size: 50,
              year: y,
              month: m > 0 ? m : undefined,
            },
          },
        }),
      ])

      const expList = unwrap(expRes)
      const incList = unwrap(incRes)

      const merged: TransactionItem[] = [
        ...expList.items.map((e) => ({
          id: e.id,
          type: 'expense' as const,
          categoryOrSource: e.category ?? '其他',
          amountCents: e.amount_cents,
          hint: e.hint ?? '',
          occurredAt: e.occurred_at,
          timeStr: formatDay(e.occurred_at),
        })),
        ...incList.items.map((i) => ({
          id: i.id,
          type: 'income' as const,
          categoryOrSource: i.source,
          amountCents: i.amount_cents,
          hint: i.hint ?? '',
          occurredAt: i.occurred_at,
          timeStr: formatDay(i.occurred_at),
        })),
      ].sort((a, b) => new Date(b.occurredAt).getTime() - new Date(a.occurredAt).getTime())

      setItems(merged)
    } catch {
      // 容错处理
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void reload(year, month)
  }, [year, month])

  // 定位高亮
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

  // 时间前后切换
  const handlePrev = () => {
    if (month === 0) {
      setYear((y) => y - 1)
    } else if (month === 1) {
      setYear((y) => y - 1)
      setMonth(12)
    } else {
      setMonth((m) => m - 1)
    }
  }

  const handleNext = () => {
    if (month === 0) {
      setYear((y) => y + 1)
    } else if (month === 12) {
      setYear((y) => y + 1)
      setMonth(1)
    } else {
      setMonth((m) => m + 1)
    }
  }

  const handleResetToCurrentMonth = () => {
    setYear(currentYear)
    setMonth(currentMonth)
    setPickerOpen(false)
  }

  // 打开记账/记收入模态框
  const openSheet = (type: 'expense' | 'income' = 'expense') => {
    setSheetType(type)
    setDraftAmount('')
    setDraftHint('')
    setDraftCategory('')
    setDraftSource('工资')
    setDraftDate(formatYMD(new Date()))
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
      setSheetError(sheetType === 'expense' ? '写一下买了什么' : '写一下收入来源说明')
      return
    }

    setSaving(true)
    setSheetError(null)
    try {
      const cents = yuanToCents(amount)
      let occurredAtISO = new Date().toISOString()
      if (draftDate) {
        occurredAtISO = `${draftDate}T12:00:00Z`
      }

      if (sheetType === 'expense') {
        const data = unwrap(
          await api.POST('/expenses', {
            body: {
              amount_cents: cents,
              hint,
              category: draftCategory || undefined,
              occurred_at: occurredAtISO,
            },
          }),
        )
        await reload()
        setHighlightId(data.id)
      } else {
        const data = unwrap(
          await api.POST('/incomes', {
            body: {
              amount_cents: cents,
              hint,
              source: draftSource || '其他',
              occurred_at: occurredAtISO,
            },
          }),
        )
        await reload()
        setHighlightId(data.id)
      }

      setSheetOpen(false)
    } catch {
      setSheetError('保存失败，稍后再试')
    } finally {
      setSaving(false)
    }
  }

  // 流水过滤
  const filteredItems = items.filter((item) => {
    if (tabFilter === 'expense') return item.type === 'expense'
    if (tabFilter === 'income') return item.type === 'income'
    return true
  })

  // 格式化金额辅助
  const formatBalance = (cents: number) => {
    const abs = Math.abs(cents / 100).toLocaleString('zh-CN', {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    })
    if (cents > 0) return `+¥${abs}`
    if (cents < 0) return `-¥${abs}`
    return `¥${abs}`
  }

  return (
    <div className={styles.container}>
      {/* 头部标题与年月导航胶囊 */}
      <div className={styles.header}>
        <h1 className={styles.title}>财务</h1>
        <div className={styles.timeCapsule}>
          <button
            type="button"
            className={styles.timeNavBtn}
            aria-label="上一个周期"
            onClick={handlePrev}
          >
            ◀
          </button>
          <button
            type="button"
            className={styles.timeDisplayBtn}
            onClick={() => setPickerOpen(true)}
          >
            <span className={styles.timeText}>
              {month > 0 ? `${year}年${month}月` : `${year}全年`}
            </span>
            <span className={styles.pickerIcon}>▾</span>
          </button>
          <button
            type="button"
            className={styles.timeNavBtn}
            aria-label="下一个周期"
            onClick={handleNext}
          >
            ▶
          </button>
        </div>
      </div>

      {/* Apple Card 风格收支总览卡片 */}
      <div className={styles.summaryCard}>
        <div className={styles.summaryCardHeader}>
          <span className={styles.cardPeriodBadge}>
            {month > 0 ? `${month}月收支结余` : `${year}年收支结余`}
          </span>
        </div>
        <div
          className={`${styles.balanceAmount} ${
            summary.balanceCents > 0
              ? styles.balancePositive
              : summary.balanceCents < 0
              ? styles.balanceNegative
              : ''
          }`}
        >
          {formatBalance(summary.balanceCents)}
        </div>

        <div className={styles.summaryDivider} />

        <div className={styles.subStatsRow}>
          <div className={styles.subStatItem}>
            <div className={styles.subStatLabel}>
              {month > 0 ? '本月收入' : '全年收入'}
            </div>
            <div className={styles.subStatIncome}>
              +{formatYuan(summary.totalIncomeCents)}
            </div>
          </div>
          <div className={styles.subStatDivider} />
          <div className={styles.subStatItem}>
            <div className={styles.subStatLabel}>
              {month > 0 ? '本月支出' : '全年支出'}
            </div>
            <div className={styles.subStatExpense}>
              -{formatYuan(summary.totalExpenseCents)}
            </div>
          </div>
        </div>
      </div>

      {/* 收支明细列表筛选栏 */}
      <div className={styles.filterSection}>
        <div className={styles.segmentedControl}>
          <button
            type="button"
            className={`${styles.segmentBtn} ${tabFilter === 'all' ? styles.segmentActive : ''}`}
            onClick={() => setTabFilter('all')}
          >
            全部
          </button>
          <button
            type="button"
            className={`${styles.segmentBtn} ${tabFilter === 'expense' ? styles.segmentActive : ''}`}
            onClick={() => setTabFilter('expense')}
          >
            支出
          </button>
          <button
            type="button"
            className={`${styles.segmentBtn} ${tabFilter === 'income' ? styles.segmentActive : ''}`}
            onClick={() => setTabFilter('income')}
          >
            收入
          </button>
        </div>
      </div>

      {/* 收支流水列表 */}
      <div className={styles.list}>
        {filteredItems.map((item) => {
          const isIncome = item.type === 'income'
          const icon = isIncome
            ? INCOME_ICONS[item.categoryOrSource] ?? '💵'
            : EXPENSE_ICONS[item.categoryOrSource] ?? '💳'

          return (
            <div
              key={item.id}
              ref={(el) => {
                itemRefs.current[item.id] = el
              }}
              className={`${styles.item} ${highlightId === item.id ? styles.highlight : ''}`}
            >
              <div
                className={`${styles.categoryIconBox} ${
                  isIncome ? styles.incomeIconBox : styles.expenseIconBox
                }`}
              >
                {icon}
              </div>
              <div className={styles.main}>
                <div className={styles.categoryRow}>
                  <span className={styles.category}>{item.categoryOrSource}</span>
                  <span
                    className={`${styles.typeBadge} ${
                      isIncome ? styles.incomeBadge : styles.expenseBadge
                    }`}
                  >
                    {isIncome ? '收入' : '支出'}
                  </span>
                </div>
                <div className={styles.hint}>
                  {item.hint ? `${item.hint} · ` : ''}
                  {item.timeStr}
                </div>
              </div>
              <div
                className={`${styles.amount} ${
                  isIncome ? styles.incomeAmount : styles.expenseAmount
                }`}
              >
                {isIncome ? `+${formatYuan(item.amountCents)}` : `-${formatYuan(item.amountCents)}`}
              </div>
            </div>
          )
        })}
        {filteredItems.length === 0 && !loading && (
          <div className={styles.empty}>
            {month > 0 ? `${year}年${month}月暂无记录` : `${year}年暂无记录`}，点右下角记一笔
          </div>
        )}
      </div>

      {/* FAB 快捷按钮 */}
      <button
        type="button"
        className={styles.fab}
        aria-label="记一笔"
        onClick={() => openSheet('expense')}
      >
        ＋
      </button>

      {/* 年月选择器弹窗 */}
      {pickerOpen && (
        <div className={styles.modalOverlay} onClick={() => setPickerOpen(false)}>
          <div className={styles.pickerModal} onClick={(e) => e.stopPropagation()}>
            <div className={styles.pickerHeader}>
              <div className={styles.pickerTitle}>选择时间</div>
              <button
                type="button"
                className={styles.resetBtn}
                onClick={handleResetToCurrentMonth}
              >
                本月
              </button>
            </div>

            {/* 年份切换 */}
            <div className={styles.yearRow}>
              {[currentYear - 2, currentYear - 1, currentYear, currentYear + 1].map((y) => (
                <button
                  type="button"
                  key={y}
                  className={`${styles.yearBtn} ${year === y ? styles.yearBtnActive : ''}`}
                  onClick={() => setYear(y)}
                >
                  {y}年
                </button>
              ))}
            </div>

            {/* 月份网格 */}
            <div className={styles.monthGrid}>
              {Array.from({ length: 12 }, (_, i) => i + 1).map((m) => (
                <button
                  type="button"
                  key={m}
                  className={`${styles.monthBtn} ${month === m ? styles.monthBtnActive : ''}`}
                  onClick={() => {
                    setMonth(m)
                    setPickerOpen(false)
                  }}
                >
                  {m}月
                </button>
              ))}
            </div>

            {/* 全年视图选项 */}
            <button
              type="button"
              className={`${styles.fullYearBtn} ${month === 0 ? styles.monthBtnActive : ''}`}
              onClick={() => {
                setMonth(0)
                setPickerOpen(false)
              }}
            >
              查看 {year} 全年汇总
            </button>
          </div>
        </div>
      )}

      {/* 记一笔浮层 Sheet */}
      {sheetOpen && (
        <div className={styles.sheetOverlay} onClick={() => setSheetOpen(false)}>
          <div className={styles.sheet} onClick={(e) => e.stopPropagation()}>
            <div className={styles.dragHandle} />

            {/* 顶栏收支切换 Tab */}
            <div className={styles.sheetTypeTabs}>
              <button
                type="button"
                className={`${styles.sheetTypeBtn} ${
                  sheetType === 'expense' ? styles.sheetTypeBtnExpenseActive : ''
                }`}
                onClick={() => setSheetType('expense')}
              >
                记支出
              </button>
              <button
                type="button"
                className={`${styles.sheetTypeBtn} ${
                  sheetType === 'income' ? styles.sheetTypeBtnIncomeActive : ''
                }`}
                onClick={() => setSheetType('income')}
              >
                记收入
              </button>
            </div>

            {/* 金额输入 */}
            <div className={styles.sheetRow}>
              <div className={styles.currencyPrefix}>¥</div>
              <input
                className={styles.sheetAmountInput}
                inputMode="decimal"
                placeholder="金额（元）"
                value={draftAmount}
                onChange={(e) => setDraftAmount(e.target.value)}
                autoFocus
              />
            </div>

            {/* 描述输入 */}
            <div className={styles.sheetRow}>
              <input
                className={styles.sheetInput}
                placeholder={
                  sheetType === 'expense'
                    ? '买了什么（如：买菜）'
                    : '收入来源说明（如：9月工资、兼职外包）'
                }
                value={draftHint}
                onChange={(e) => setDraftHint(e.target.value)}
              />
            </div>

            {/* 分类/来源选择 Chips */}
            <div className={styles.chipsLabel}>
              {sheetType === 'expense' ? '选择分类' : '收入来源'}
            </div>
            <div className={styles.chips}>
              {sheetType === 'expense'
                ? EXPENSE_CATEGORIES.map((c) => (
                    <button
                      type="button"
                      key={c}
                      className={`${styles.chip} ${draftCategory === c ? styles.chipOn : ''}`}
                      onClick={() => setDraftCategory(draftCategory === c ? '' : c)}
                    >
                      {EXPENSE_ICONS[c]} {c}
                    </button>
                  ))
                : INCOME_SOURCES.map((s) => (
                    <button
                      type="button"
                      key={s}
                      className={`${styles.chip} ${draftSource === s ? styles.chipIncomeOn : ''}`}
                      onClick={() => setDraftSource(s)}
                    >
                      {INCOME_ICONS[s]} {s}
                    </button>
                  ))}
            </div>

            {/* 日期选择 (默认今天，支持补记昨天/前天) */}
            <div className={styles.dateSelectorRow}>
              <div className={styles.dateChips}>
                {[
                  { label: '今天', date: formatYMD(new Date()) },
                  {
                    label: '昨天',
                    date: formatYMD(new Date(Date.now() - 86400000)),
                  },
                  {
                    label: '前天',
                    date: formatYMD(new Date(Date.now() - 172800000)),
                  },
                ].map((d) => (
                  <button
                    type="button"
                    key={d.label}
                    className={`${styles.dateChip} ${
                      draftDate === d.date ? styles.dateChipActive : ''
                    }`}
                    onClick={() => setDraftDate(d.date)}
                  >
                    {d.label}
                  </button>
                ))}
              </div>
              <input
                type="date"
                className={styles.dateInput}
                value={draftDate}
                onChange={(e) => setDraftDate(e.target.value)}
              />
            </div>

            {sheetError && <div className={styles.sheetError}>{sheetError}</div>}

            <button
              type="button"
              className={`${styles.sheetSave} ${
                sheetType === 'income' ? styles.sheetSaveIncome : ''
              }`}
              disabled={saving}
              onClick={() => void handleSave()}
            >
              {saving ? '保存中…' : sheetType === 'expense' ? '保存支出' : '保存收入'}
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

/** Date → YYYY-MM-DD */
function formatYMD(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}
