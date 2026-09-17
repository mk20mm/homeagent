/**
 * 记账结果卡片：结构化展示 + 金额/类目行内修正 + 在账本中查看。
 * card 由 record_expense 工具回传（{type:'expense', amount, category, hint, ...}）。
 */
import { useRef, useState } from 'react'
import { useNavigate } from 'react-router'

import { formatYuan } from '@homeagent/shared'

import { api, unwrap } from '../api/client'

import styles from './ExpenseCard.module.css'

const CATEGORIES = ['食材', '日用', '外卖', '出行', '餐饮', '其他']

interface Props {
  card: {
    expense_id?: string
    amount?: number
    category?: string
    hint?: string
    time?: string
    duplicated?: boolean
  }
  /** 是否在 24h 撤销窗口内 */
  undoable?: boolean
  onUndo?: () => void
  /** 修正成功后回调，通知父组件更新 card */
  onUpdated?: (next: {
    expense_id?: string
    amount?: number
    category?: string
    hint?: string
  }) => void
}

export function ExpenseCard({ card, undoable, onUndo, onUpdated }: Props) {
  const navigate = useNavigate()
  const cardRef = useRef<HTMLDivElement>(null)
  const [editing, setEditing] = useState(false)
  const [amountText, setAmountText] = useState(
    card.amount != null ? String(card.amount) : '',
  )
  const [category, setCategory] = useState(card.category ?? '')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const expenseId = card.expense_id
  const canEdit = Boolean(expenseId) && !card.duplicated

  const startEdit = () => {
    setAmountText(card.amount != null ? String(card.amount) : '')
    setCategory(card.category ?? '')
    setError(null)
    setEditing(true)
    // 滚动到可视区，避免保存按钮被底部输入栏遮挡
    setTimeout(() => {
      cardRef.current?.scrollIntoView({ block: 'center', behavior: 'smooth' })
    }, 60)
  }

  const save = async () => {
    if (!expenseId) return
    const amount = Number(amountText.trim())
    if (!Number.isFinite(amount) || amount <= 0) {
      setError('金额要大于 0')
      return
    }
    setSaving(true)
    setError(null)
    try {
      const data = unwrap(
        await api.PATCH('/expenses/{expenseId}', {
          params: { path: { expenseId } },
          body: { amount_cents: Math.round(amount * 100), category },
        }),
      )
      onUpdated?.({
        expense_id: expenseId,
        amount: data.amount_cents / 100,
        category: data.category,
        hint: card.hint,
      })
      setEditing(false)
    } catch {
      setError('修正失败，请重试')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className={styles.card} ref={cardRef}>
      <div className={styles.header}>
        <span className={styles.title}>记账结果</span>
        {undoable && (
          <button type="button" className={styles.undoBtn} onClick={onUndo}>
            撤销
          </button>
        )}
      </div>

      {card.duplicated ? (
        <div className={styles.dup}>今天已记过这笔，未重复记账</div>
      ) : editing ? (
        <div className={styles.edit}>
          <div className={styles.editRow}>
            <span className={styles.editLabel}>金额</span>
            <input
              className={styles.editInput}
              inputMode="decimal"
              value={amountText}
              onChange={(e) => setAmountText(e.target.value)}
            />
            <span className={styles.yuan}>元</span>
          </div>
          <div className={styles.chips}>
            {CATEGORIES.map((c) => (
              <button
                type="button"
                key={c}
                className={`${styles.chip} ${category === c ? styles.chipOn : ''}`}
                onClick={() => setCategory(c)}
              >
                {c}
              </button>
            ))}
          </div>
          {error && <div className={styles.error}>{error}</div>}
          <div className={styles.editActions}>
            <button
              type="button"
              className={styles.cancelBtn}
              onClick={() => setEditing(false)}
            >
              取消
            </button>
            <button
              type="button"
              className={styles.saveBtn}
              disabled={saving}
              onClick={() => void save()}
            >
              {saving ? '保存中…' : '保存修正'}
            </button>
          </div>
        </div>
      ) : (
        <>
          <div className={styles.amountRow}>
            <span className={styles.amount}>
              {card.amount != null ? formatYuan(Math.round(card.amount * 100)) : '—'}
            </span>
            <span className={styles.categoryTag}>{card.category ?? '其他'}</span>
          </div>
          {card.hint && <div className={styles.hint}>{card.hint}</div>}
          {canEdit && (
            <div className={styles.actions}>
              <button type="button" className={styles.linkBtn} onClick={startEdit}>
                修正
              </button>
              <span className={styles.dot}>·</span>
              <button
                type="button"
                className={styles.linkBtn}
                onClick={() => navigate(`/money?focus=${expenseId}`)}
              >
                在账本中查看
              </button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
