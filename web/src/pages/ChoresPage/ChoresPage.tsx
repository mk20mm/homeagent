/** 家务任务页（page-02-chores）：任务列表/状态机/打卡（Apple Reminders 极简风格） */
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

type FilterType = 'all' | 'pending' | 'done'

export function ChoresPage() {
  const [tasks, setTasks] = useState<Task[]>([])
  const [loading, setLoading] = useState(true)
  const [completing, setCompleting] = useState<string | null>(null)
  const [filter, setFilter] = useState<FilterType>('all')

  const fetchTasks = useCallback(async () => {
    try {
      const res = await api.GET('/tasks')
      const data = unwrap(res)
      setTasks(data.items ?? [])
    } catch {
      // 首次加载接口可能不存在（渐进式上线）
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
      await fetchTasks()
    } catch (err) {
      const msg = err instanceof Error ? err.message : '打卡失败'
      alert(msg)
    } finally {
      setCompleting(null)
    }
  }

  const pendingCount = tasks.filter((t) => t.status !== 'done').length
  const doneCount = tasks.filter((t) => t.status === 'done').length

  const filteredTasks = tasks.filter((t) => {
    if (filter === 'pending') return t.status !== 'done'
    if (filter === 'done') return t.status === 'done'
    return true
  })

  if (loading) {
    return (
      <div className={styles.container}>
        <h1 className={styles.title}>家务任务</h1>
        <div className={styles.empty}>加载中...</div>
      </div>
    )
  }

  return (
    <div className={styles.container}>
      <div className={styles.headerRow}>
        <h1 className={styles.title}>家务任务</h1>
        <span className={styles.statsPill}>
          {pendingCount} 项待办
        </span>
      </div>

      {/* Apple 风格分段选择器 (Segmented Control) */}
      <div className={styles.segmentedControl} role="tablist">
        <button
          type="button"
          role="tab"
          aria-selected={filter === 'all'}
          className={`${styles.segmentBtn} ${filter === 'all' ? styles.segmentActive : ''}`}
          onClick={() => setFilter('all')}
        >
          全部 ({tasks.length})
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={filter === 'pending'}
          className={`${styles.segmentBtn} ${filter === 'pending' ? styles.segmentActive : ''}`}
          onClick={() => setFilter('pending')}
        >
          待办 ({pendingCount})
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={filter === 'done'}
          className={`${styles.segmentBtn} ${filter === 'done' ? styles.segmentActive : ''}`}
          onClick={() => setFilter('done')}
        >
          已完成 ({doneCount})
        </button>
      </div>

      {filteredTasks.length === 0 ? (
        <div className={styles.empty}>
          <div className={styles.emptyIcon}>📋</div>
          <div className={styles.emptyText}>
            {filter === 'done' ? '暂无已完成家务' : '暂无待办任务'}
          </div>
          <div className={styles.emptyHint}>
            试试在对话里说「提醒我洗碗」或「派个任务给爸爸」
          </div>
        </div>
      ) : (
        <div className={styles.list}>
          {filteredTasks.map((t) => {
            const isDone = t.status === 'done'
            return (
              <div
                key={t.id}
                className={`${styles.item} ${isDone ? styles.itemDone : ''}`}
              >
                {/* Apple Reminders 风格圆形勾选触发按钮 */}
                <button
                  type="button"
                  aria-label={isDone ? '已完成' : '打卡完成'}
                  className={`${styles.circleCheck} ${isDone ? styles.circleChecked : ''}`}
                  disabled={isDone || completing === t.id}
                  onClick={() => handleComplete(t.id)}
                >
                  {isDone ? '✓' : ''}
                </button>

                <div className={styles.main}>
                  <div className={`${styles.taskTitle} ${isDone ? styles.taskTitleDone : ''}`}>
                    {t.title}
                  </div>
                  <div className={styles.meta}>
                    <span className={styles.statusTag}>
                      {TASK_STATUS_LABEL[t.status]}
                    </span>
                    {t.points ? (
                      <span className={styles.pointsBadge}>+{t.points}分</span>
                    ) : null}
                    {t.due_at && (
                      <span className={styles.dueTag}>
                        截止 {new Date(t.due_at).toLocaleDateString('zh-CN', { month: 'numeric', day: 'numeric' })}
                      </span>
                    )}
                  </div>
                </div>

                {!isDone ? (
                  <button
                    type="button"
                    className={styles.doneBtn}
                    disabled={completing === t.id}
                    onClick={() => handleComplete(t.id)}
                  >
                    {completing === t.id ? '...' : '打卡'}
                  </button>
                ) : (
                  <span className={styles.completedBadge}>已打卡</span>
                )}
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
