/** 日程列表页（A-02-01）：按日期分组展示日程实例，复用 GET /events。 */
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router'

import { api, unwrap } from '../../api/client'
import type { paths } from '../../api/schema'

import styles from './EventsPage.module.css'

type EventInstance =
  paths['/events']['get']['responses'][200]['content']['application/json']['items'][number]

/** 把实例按日期（本地）分组，同日内按时间升序 */
function groupByDate(items: EventInstance[]): Array<{ date: string; items: EventInstance[] }> {
  const map = new Map<string, EventInstance[]>()
  for (const it of items) {
    const d = new Date(it.start_at)
    const key = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
    const arr = map.get(key) ?? []
    arr.push(it)
    map.set(key, arr)
  }
  return [...map.entries()].map(([date, list]) => ({
    date,
    items: list.sort((a, b) => a.start_at.localeCompare(b.start_at)),
  }))
}

function dayLabel(dateStr: string): string {
  const d = new Date(dateStr + 'T00:00:00')
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const diff = Math.round((d.getTime() - today.getTime()) / 86_400_000)
  if (diff === 0) return '今天'
  if (diff === 1) return '明天'
  if (diff === -1) return '昨天'
  return `${d.getMonth() + 1}月${d.getDate()}日`
}

function timeLabel(iso: string): string {
  const d = new Date(iso)
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

export function EventsPage() {
  const [items, setItems] = useState<EventInstance[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()

  const reload = async () => {
    try {
      const data = unwrap(await api.GET('/events'))
      setItems(data.items)
      setError(null)
    } catch {
      setError('日程加载失败，下拉重试')
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
        <h1 className={styles.title}>日程</h1>
        <div className={styles.sub}>加载中…</div>
      </div>
    )
  }

  const groups = groupByDate(items)

  return (
    <div>
      <h1 className={styles.title}>日程</h1>

      {error && <div className={styles.error}>{error}</div>}

      {groups.length === 0 ? (
        <div className={styles.empty}>
          <div className={styles.emptyTitle}>最近没有日程</div>
          <div className={styles.emptySub}>
            跟管家说一句就能建，比如「下周三下午三点开家长会」
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
        groups.map((g) => (
          <div key={g.date} className={styles.group}>
            <div className={styles.dateHeader}>{dayLabel(g.date)}</div>
            <div className={styles.list}>
              {g.items.map((it) => (
                <button
                  type="button"
                  key={`${it.id}-${it.occurrence}`}
                  className={styles.item}
                  onClick={() => navigate(`/events/${it.id}`)}
                >
                  <span className={styles.time}>{timeLabel(it.start_at)}</span>
                  <div className={styles.main}>
                    <div className={styles.eventTitle}>{it.title}</div>
                    <div className={styles.meta}>
                      {it.owner_name ? `${it.owner_name} · ` : ''}
                      {it.repeat === 'once' ? '单次' : repeatLabel(it.repeat)}
                      {it.is_lunar ? ' · 农历' : ''}
                    </div>
                  </div>
                  <span className={styles.chevron}>›</span>
                </button>
              ))}
            </div>
          </div>
        ))
      )}
    </div>
  )
}

function repeatLabel(repeat: string): string {
  switch (repeat) {
    case 'daily':
      return '每天'
    case 'weekly':
      return '每周'
    case 'monthly':
      return '每月'
    default:
      return '单次'
  }
}
