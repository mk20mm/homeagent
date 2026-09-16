/** 家务任务页（page-02-chores）：任务列表/状态机/打卡 */
import { TASK_STATUS_LABEL, type TaskStatus } from '@homeagent/shared'

import styles from './ChoresPage.module.css'

interface Task {
  id: string
  title: string
  status: TaskStatus
  points: number
}

// 骨架阶段用静态数据；后续接 GET /tasks（list_my_tasks 出口）
const MOCK: Task[] = [
  { id: '1', title: '洗碗', status: 'pending', points: 1 },
  { id: '2', title: '倒垃圾', status: 'in_progress', points: 1 },
  { id: '3', title: '大扫除', status: 'done', points: 3 },
]

export function ChoresPage() {
  return (
    <div>
      <h1 className={styles.title}>家务任务</h1>
      <div className={styles.list}>
        {MOCK.map((t) => (
          <div key={t.id} className={styles.item}>
            <span className={styles.icon}>
              {t.status === 'done' ? '✅' : t.status === 'in_progress' ? '🔄' : '⏳'}
            </span>
            <div className={styles.main}>
              <div className={styles.taskTitle}>{t.title}</div>
              <div className={styles.meta}>
                {TASK_STATUS_LABEL[t.status]} · {t.points} 分
              </div>
            </div>
            {t.status !== 'done' && (
              <button type="button" className={styles.doneBtn}>
                打卡
              </button>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}
