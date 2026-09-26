/**
 * Today 首页（PRD A-01）：个人今日行动摘要。
 * 数据来自 GET /today（服务端聚合 + 权限一次性裁剪），页面不维护独立状态。
 * 空态是正常状态（A-01-06）：明确「今天没有需要你处理的事项」。
 */
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router'

import { api, unwrap } from '../../api/client'
import type { paths } from '../../api/schema'

import styles from './TodayPage.module.css'

type Summary =
  paths['/today']['get']['responses'][200]['content']['application/json']

function timeLabel(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

function dateLabel(): string {
  const d = new Date()
  const wd = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'][d.getDay()]
  return `${d.getMonth() + 1}月${d.getDate()}日 ${wd}`
}

export function TodayPage() {
  const [data, setData] = useState<Summary | null>(null)
  const [loading, setLoading] = useState(true)
  const navigate = useNavigate()

  const reload = async () => {
    try {
      const d = unwrap(await api.GET('/today'))
      setData(d)
    } catch {
      setData(null)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void reload()
  }, [])

  if (loading) {
    return (
      <div>
        <h1 className={styles.title}>今天</h1>
        <div className={styles.sub}>加载中…</div>
      </div>
    )
  }

  if (!data) {
    return (
      <div>
        <h1 className={styles.title}>今天</h1>
        <div className={styles.error}>摘要加载失败</div>
        <button type="button" className={styles.chatBtn} onClick={() => reload()}>
          重试
        </button>
      </div>
    )
  }

  const need = data.need_action
  const hasNeed =
    need.tasks.length > 0 || need.meal_unreported || need.notifications.length > 0
  const hasAnything = hasNeed || data.schedule.length > 0 || data.activity.length > 0

  return (
    <div>
      <div className={styles.head}>
        <div>
          <h1 className={styles.title}>今天</h1>
          <div className={styles.date}>{dateLabel()}</div>
        </div>
        <button
          type="button"
          className={styles.chatBtn}
          onClick={() => navigate('/chat')}
        >
          跟管家说一句
        </button>
      </div>

      {!hasAnything && (
        <div className={styles.empty}>
          <div className={styles.emptyTitle}>今天没有需要你处理的事项</div>
          <div className={styles.emptySub}>
            有事随时跟管家说，比如「买菜花了 120」「今晚不回家吃」
          </div>
          <button
            type="button"
            className={styles.chatBtn}
            onClick={() => navigate('/chat')}
          >
            去对话
          </button>
        </div>
      )}

      {/* 需要我处理（强行动区） */}
      {hasNeed && (
        <section className={styles.section}>
          <h2 className={styles.sectionTitle}>需要我处理</h2>
          <div className={styles.card}>
            {need.tasks.map((t) => (
              <button
                type="button"
                key={t.id}
                className={styles.row}
                onClick={() => navigate('/chores')}
              >
                <span className={styles.dot}>✅</span>
                <span className={styles.rowMain}>{t.title}</span>
                <span className={styles.rowMeta}>
                  {t.due_at ? `截止 ${timeLabel(t.due_at)}` : '待处理'}
                </span>
              </button>
            ))}
            {need.meal_unreported && (
              <button
                type="button"
                className={styles.row}
                onClick={() => navigate('/meal')}
              >
                <span className={styles.dot}>🍚</span>
                <span className={styles.rowMain}>今天还没报饭</span>
                <span className={styles.rowMeta}>去申报</span>
              </button>
            )}
            {need.notifications.map((n) => (
              <button
                type="button"
                key={n.id}
                className={styles.row}
                onClick={() =>
                  n.action_path ? navigate(n.action_path) : undefined
                }
              >
                <span className={styles.dot}>🔔</span>
                <span className={styles.rowMain}>{n.title}</span>
                <span className={styles.rowMeta}>{n.action_label || '查看'}</span>
              </button>
            ))}
          </div>
        </section>
      )}

      {/* 今日安排 */}
      {data.schedule.length > 0 && (
        <section className={styles.section}>
          <h2 className={styles.sectionTitle}>今日安排</h2>
          <div className={styles.card}>
            {data.schedule.map((s) => (
              <button
                type="button"
                key={`${s.id}-${s.start_at}`}
                className={styles.row}
                onClick={() => navigate(`/events/${s.id}`)}
              >
                <span className={styles.time}>{timeLabel(s.start_at)}</span>
                <span className={styles.rowMain}>{s.title}</span>
                {s.visibility === 'private' && (
                  <span className={styles.rowMeta}>仅自己</span>
                )}
              </button>
            ))}
          </div>
        </section>
      )}

      {/* 家庭用餐 */}
      {data.meal && (
        <section className={styles.section}>
          <h2 className={styles.sectionTitle}>家庭用餐</h2>
          <div className={styles.card}>
            <div className={styles.mealLine}>
              <span className={styles.mealTag}>在家</span>
              <span className={styles.mealNames}>
                {data.meal.at_home.length > 0
                  ? data.meal.at_home.join('、')
                  : '还没有人'}
              </span>
            </div>
            <div className={styles.mealLine}>
              <span className={styles.mealTag}>不在家</span>
              <span className={styles.mealNames}>
                {data.meal.not_at_home.length > 0
                  ? data.meal.not_at_home.join('、')
                  : '—'}
              </span>
            </div>
            <div className={styles.mealLine}>
              <span className={styles.mealTag}>未申报</span>
              <span className={styles.mealNames}>
                {data.meal.unreported.length > 0
                  ? data.meal.unreported.join('、')
                  : '都报了'}
              </span>
            </div>
            <div className={styles.mealMine}>
              {data.meal.mine === 'at_home'
                ? '我：今晚在家吃'
                : data.meal.mine === 'not_at_home'
                  ? '我：今晚不在家吃'
                  : '我：今天还没报饭'}
            </div>
          </div>
        </section>
      )}

      {/* 代理人动态 */}
      {data.activity.length > 0 && (
        <section className={styles.section}>
          <h2 className={styles.sectionTitle}>代理人动态</h2>
          <div className={styles.card}>
            {data.activity.map((n) => (
              <div key={n.id} className={styles.activityRow}>
                <div className={styles.activityTitle}>
                  {n.read ? '' : <span className={styles.unreadDot} />}
                  {n.title}
                </div>
                {n.body && <div className={styles.activityBody}>{n.body}</div>}
              </div>
            ))}
          </div>
        </section>
      )}
    </div>
  )
}
