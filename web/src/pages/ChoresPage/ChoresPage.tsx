/** 家务任务页（page-02-chores）：任务列表/状态机/打卡 */
import { useCallback, useEffect, useState } from 'react'

import { TASK_STATUS_LABEL, type TaskStatus } from '@homeagent/shared'

import { api, unwrap } from '../../api/client'

import styles from './ChoresPage.module.css'

interface Task {
  id: string
  title: string
  status: TaskStatus
  points?: number
  due_at?: string
  completed_at?: string
}

export function ChoresPage() {
  const [tasks, setTasks] = useState<Task[]>([])
  const [loading, setLoading] = useState(true)
  const [completing, setCompleting] = useState<string | null>(null)

  const fetchTasks = useCallback(async () => {
    try {
      const res = await api.GET('/tasks')
      const data = unwrap(res)
      setTasks(data.items ?? [])
    } catch {
      // 静默：首次加载接口可能不存在（渐进式上线）
      setTasks([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchTasks()
  }, [fetchTasks])

  const handleComplete = async (taskId: string) => {
    setCompleting(taskId)
    try {
      await api.POST('/tasks/{taskId}/complete', {
        params: { path: { taskId } },
      })
      // 刷新列表
      await fetchTasks()
    } catch (err) {
      const msg = err instanceof Error ? err.message : '打卡失败'
      alert(msg)
    } finally {
      setCompleting(null)
    }
  }

  if (loading) {
    return (
      <div>
        <h1 className={styles.title}>家务任务</h1>
        <div className={styles.empty}>加载中...</div>
      </div>
    )
  }

  return (
    <div>
      <h1 className={styles.title}>家务任务</h1>
      {tasks.length === 0 ? (
        <div className={styles.empty}>
          <div className={styles.emptyIcon}>📋</div>
          <div className={styles.emptyText}>暂无待办任务</div>
          <div className={styles.emptyHint}>
            试试在对话里说「提醒我洗碗」或「派个任务给爸爸」
          </div>
        </div>
      ) : (
        <div className={styles.list}>
          {tasks.map((t) => (
            <div key={t.id} className={styles.item}>
              <span className={styles.icon}>
                {t.status === 'done'
                  ? '✅'
                  : t.status === 'in_progress'
                    ? '🔄'
                    : '⏳'}
              </span>
              <div className={styles.main}>
                <div className={styles.taskTitle}>{t.title}</div>
                <div className={styles.meta}>
                  {TASK_STATUS_LABEL[t.status]}
                  {t.points ? ` · ${t.points} 分` : ''}
                  {t.due_at
                    ? ` · 截止 ${new Date(t.due_at).toLocaleDateString('zh-CN', { month: 'numeric', day: 'numeric' })}`
                    : ''}
                </div>
              </div>
              {t.status !== 'done' && (
                <button
                  type="button"
                  className={styles.doneBtn}
                  disabled={completing === t.id}
                  onClick={() => handleComplete(t.id)}
                >
                  {completing === t.id ? '...' : '打卡'}
                </button>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
