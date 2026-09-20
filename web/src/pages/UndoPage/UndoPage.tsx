/** 撤销中心（T-A04）：24h 内可撤销的操作，一处统一回退（ADR-004） */
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router'

import { api, unwrap, ApiError } from '../../api/client'

import styles from './UndoPage.module.css'

type UndoItem = {
  undo_id: string
  tool_name: string
  summary: string
  created_at: string
  expires_at: string
}

/** 剩余有效窗口：剩 23 小时 / 剩 40 分钟 / 即将过期 */
function remaining(expiresAt: string): string {
  const left = Date.parse(expiresAt) - Date.now()
  if (left <= 0) return '即将过期'
  const mins = Math.floor(left / 60_000)
  if (mins < 60) return `剩 ${Math.max(mins, 1)} 分钟`
  const hours = Math.floor(mins / 60)
  const m = mins % 60
  return m > 0 ? `剩 ${hours} 小时 ${m} 分钟` : `剩 ${hours} 小时`
}

export function UndoPage() {
  const [items, setItems] = useState<UndoItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busyId, setBusyId] = useState<string | null>(null)
  const navigate = useNavigate()

  const reload = async () => {
    try {
      const data = unwrap(await api.GET('/undo'))
      setItems(data.items)
      setError(null)
    } catch {
      setError('加载失败，下拉重试')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void reload()
  }, [])

  const undo = async (id: string) => {
    if (busyId) return
    setBusyId(id)
    setError(null)
    // 乐观移除：服务端确认前先消失，失败再捞回来
    const snapshot = items
    setItems((prev) => prev.filter((t) => t.undo_id !== id))
    try {
      unwrap(
        await api.POST('/undo/{undoId}', {
          params: { path: { undoId: id } },
        }),
      )
    } catch (e) {
      setItems(snapshot)
      if (e instanceof ApiError && e.code === 'not_found') {
        setError('这条记录已过期或已被撤销')
        void reload()
      } else if (e instanceof ApiError && e.code === 'conflict') {
        setError('该操作已撤销或已失效')
        void reload()
      } else {
        setError('撤销失败，请重试')
      }
    } finally {
      setBusyId(null)
    }
  }

  if (loading) {
    return (
      <div>
        <h1 className={styles.title}>撤销</h1>
        <div className={styles.sub}>加载中…</div>
      </div>
    )
  }

  return (
    <div>
      <h1 className={styles.title}>撤销</h1>
      <div className={styles.sub}>24 小时内的操作可以在这里回退</div>

      {error && <div className={styles.error}>{error}</div>}

      {items.length === 0 ? (
        <div className={styles.empty}>
          <div className={styles.emptyTitle}>最近没有可撤销的操作</div>
          <div className={styles.emptySub}>
            记账、派任务、打卡、报饭之后，这里会出现「反悔」入口
          </div>
          <button
            type="button"
            className={styles.emptyBtn}
            onClick={() => navigate('/')}
          >
            回去聊聊
          </button>
        </div>
      ) : (
        <div className={styles.list}>
          {items.map((t) => (
            <div key={t.undo_id} className={styles.item}>
              <div className={styles.main}>
                <div className={styles.summary}>{t.summary}</div>
                <div className={styles.meta}>{remaining(t.expires_at)}</div>
              </div>
              <button
                type="button"
                className={styles.undoBtn}
                disabled={busyId === t.undo_id}
                onClick={() => void undo(t.undo_id)}
              >
                {busyId === t.undo_id ? '…' : '撤销'}
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
