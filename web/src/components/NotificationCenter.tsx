/**
 * 通知中心抽屉（ADR-006）：铃铛角标 + 列表 + 标记已读。
 *
 * 静默通知：无声音无震动，只有未读角标（微信式，99+ 封顶）。
 * 打开抽屉不自动全部已读——需显式点「全部已读」（避免瞄一眼就没了）。
 */
import { useCallback, useEffect, useState } from 'react'

import { api, unwrap } from '../api/client'

import styles from './NotificationCenter.module.css'

type Notification = {
  id: string
  type: string
  title: string
  body: string
  action_label?: string
  action_path?: string
  read: boolean
  created_at: string
}

interface Props {
  open: boolean
  onClose: () => void
}

export function NotificationCenter({ open, onClose }: Props) {
  const [items, setItems] = useState<Notification[]>([])
  const [unread, setUnread] = useState(0)
  const [loading, setLoading] = useState(false)

  const reload = useCallback(async () => {
    setLoading(true)
    try {
      const data = unwrap(await api.GET('/notifications'))
      setItems(data.items)
      setUnread(data.unread_count)
    } catch {
      // 加载失败不阻塞，下次打开重试
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (open) void reload()
  }, [open, reload])

  const handleReadOne = async (id: string) => {
    try {
      await unwrap(
        await api.POST('/notifications', { body: { notification_id: id } }),
      )
      setItems((prev) =>
        prev.map((n) => (n.id === id ? { ...n, read: true } : n)),
      )
      setUnread((u) => Math.max(0, u - 1))
    } catch {
      // 失败不回滚，下次打开重试
    }
  }

  const handleReadAll = async () => {
    try {
      await unwrap(await api.POST('/notifications', { body: {} }))
      setItems((prev) => prev.map((n) => ({ ...n, read: true })))
      setUnread(0)
    } catch {
      // 失败不回滚
    }
  }

  return (
    <>
      {open && (
        <div
          className={styles.overlay}
          onClick={onClose}
          aria-hidden="true"
        />
      )}
      <aside className={`${styles.drawer} ${open ? styles.open : ''}`}>
        <header className={styles.header}>
          <h2 className={styles.title}>通知</h2>
          {unread > 0 && (
            <button
              type="button"
              className={styles.readAll}
              onClick={() => void handleReadAll()}
            >
              全部已读
            </button>
          )}
        </header>

        <div className={styles.list}>
          {loading && items.length === 0 && (
            <div className={styles.empty}>加载中…</div>
          )}
          {!loading && items.length === 0 && (
            <div className={styles.empty}>
              <div className={styles.emptyTitle}>暂无通知</div>
              <div className={styles.emptySub}>
                任务快到期、报饭缺口会在这里提醒你
              </div>
            </div>
          )}
          {items.map((n) => (
            <div
              key={n.id}
              className={`${styles.item} ${n.read ? '' : styles.unread}`}
            >
              <div className={styles.main}>
                <div className={styles.itemTitle}>
                  {!n.read && <span className={styles.dot} aria-hidden="true" />}
                  {n.title}
                </div>
                <div className={styles.body}>{n.body}</div>
                <div className={styles.meta}>{formatTime(n.created_at)}</div>
              </div>
              <div className={styles.actions}>
                {n.action_path && (
                  <a
                    className={styles.action}
                    href={n.action_path}
                    onClick={onClose}
                  >
                    {n.action_label ?? '去看'}
                  </a>
                )}
                {!n.read && (
                  <button
                    type="button"
                    className={styles.readBtn}
                    onClick={() => void handleReadOne(n.id)}
                  >
                    已读
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      </aside>
    </>
  )
}

/** 相对时间：刚刚 / N 分钟前 / 今天 HH:MM / M/D */
function formatTime(iso: string): string {
  const t = new Date(iso)
  if (Number.isNaN(t.getTime())) return ''
  const now = new Date()
  const diff = now.getTime() - t.getTime()
  const minute = 60_000
  if (diff < minute) return '刚刚'
  if (diff < 60 * minute) return `${Math.floor(diff / minute)} 分钟前`
  if (t.toDateString() === now.toDateString()) {
    return `${String(t.getHours()).padStart(2, '0')}:${String(t.getMinutes()).padStart(2, '0')}`
  }
  return `${t.getMonth() + 1}/${t.getDate()}`
}
