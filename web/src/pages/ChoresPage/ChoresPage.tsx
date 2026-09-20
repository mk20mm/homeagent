/** 家务任务页（page-02-chores）：任务列表/状态机/打卡，数据来自 GET /tasks */
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router'

import { TASK_STATUS_LABEL, type TaskStatus } from '@homeagent/shared'

import { api, unwrap, ApiError } from '../../api/client'
import type { paths } from '../../api/schema'

import styles from './ChoresPage.module.css'

type TaskItem = paths['/tasks']['get']['responses'][200]['content']['application/json']['items'][number]

interface Task extends TaskItem {
  /** 最近一次打卡的撤销 id（有了才显示「撤销」） */
  undoId?: string
}

export function ChoresPage() {
  const [items, setItems] = useState<Task[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busyId, setBusyId] = useState<string | null>(null)
  const navigate = useNavigate()

  const reload = async () => {
    try {
      const data = unwrap(await api.GET('/tasks'))
      setItems(data.items)
      setError(null)
    } catch {
      setError('任务加载失败，下拉重试')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void reload()
  }, [])

  const complete = async (id: string) => {
    if (busyId) return
    setBusyId(id)
    setError(null)
    try {
      const res = unwrap(
        await api.POST('/tasks/{taskId}/complete', {
          params: { path: { taskId: id } },
        }),
      )
      // 乐观更新：状态推进到 done + 记下撤销 id（撤销 = 回退未完成）
      setItems((prev) =>
        prev.map((t) =>
          t.id === id
            ? { ...t, status: res.status, undoId: res.undo_id }
            : t,
        ),
      )
    } catch (e) {
      if (e instanceof ApiError && e.code === 'conflict') {
        setError('这个任务已经完成了')
      } else {
        setError('打卡失败，请重试')
      }
    } finally {
      setBusyId(null)
    }
  }

  const undoComplete = async (id: string, undoId: string) => {
    if (busyId) return
    setBusyId(id)
    setError(null)
    try {
      unwrap(
        await api.POST('/undo/{undoId}', {
          params: { path: { undoId } },
        }),
      )
      await reload()
    } catch {
      setError('撤销失败，请重试')
    } finally {
      setBusyId(null)
    }
  }

  if (loading) {
    return (
      <div>
        <h1 className={styles.title}>家务任务</h1>
        <div className={styles.sub}>加载中…</div>
      </div>
    )
  }

  return (
    <div>
      <h1 className={styles.title}>家务任务</h1>

      {error && <div className={styles.error}>{error}</div>}

      {items.length === 0 ? (
        <div className={styles.empty}>
          <div className={styles.emptyTitle}>今天没有待办</div>
          <div className={styles.emptySub}>
            跟管家说一句就能派活，比如「提醒媳妇洗碗」
          </div>
          <button
            type="button"
            className={styles.emptyBtn}
            onClick={() => navigate('/chat')}
          >
            去跟管家说一句
          </button>
        </div>
      ) : (
        <div className={styles.list}>
          {items.map((t) => {
            const done = t.status === 'done'
            return (
              <div key={t.id} className={styles.item}>
                <span className={styles.icon}>
                  {done
                    ? '✅'
                    : t.status === 'in_progress'
                      ? '🔄'
                      : '⏳'}
                </span>
                <div className={styles.main}>
                  <div className={styles.taskTitle}>{t.title}</div>
                  <div className={styles.meta}>
                    {TASK_STATUS_LABEL[t.status as TaskStatus]} · {t.points} 分
                  </div>
                </div>
                {t.undoId && (
                  <button
                    type="button"
                    className={styles.undoBtn}
                    disabled={busyId === t.id}
                    onClick={() => void undoComplete(t.id, t.undoId as string)}
                  >
                    撤销
                  </button>
                )}
                {!done && (
                  <button
                    type="button"
                    className={styles.doneBtn}
                    disabled={busyId === t.id}
                    onClick={() => void complete(t.id)}
                  >
                    {busyId === t.id ? '…' : '打卡'}
                  </button>
                )}
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
