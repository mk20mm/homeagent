/**
 * 会话侧边栏（豆包式抽屉）：新建对话 / 会话列表 / 删除 / 切换。
 * 移动端从左侧滑出，点遮罩或列表项后关闭。
 */
import { useNavigate } from 'react-router'
import type { MouseEvent } from 'react'

import { useChatStore } from '../stores/chat'

import styles from './ConversationSidebar.module.css'

interface Props {
  open: boolean
  onClose: () => void
}

export function ConversationSidebar({ open, onClose }: Props) {
  const navigate = useNavigate()
  const {
    conversations,
    currentConversationId,
    selectConversation,
    newConversation,
    deleteConversation,
  } = useChatStore()

  const handleSelect = async (id: string) => {
    await selectConversation(id)
    onClose()
  }

  const handleDelete = async (e: MouseEvent, id: string) => {
    e.stopPropagation()
    if (window.confirm('删除这个会话？删除后不可恢复。')) {
      await deleteConversation(id)
    }
  }

  const handleNew = async () => {
    await newConversation()
    onClose()
  }

  const handleGoSettings = () => {
    onClose()
    void navigate('/settings')
  }

  return (
    <>
      {open && <div className={styles.overlay} onClick={onClose} aria-hidden="true" />}
      <aside className={`${styles.drawer} ${open ? styles.open : ''}`}>
        <button type="button" className={styles.newBtn} onClick={() => void handleNew()}>
          ＋ 新建对话
        </button>
        <div className={styles.list}>
          {conversations.map((c) => (
            <div
              key={c.id}
              className={`${styles.item} ${c.id === currentConversationId ? styles.active : ''}`}
              onClick={() => void handleSelect(c.id)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') void handleSelect(c.id)
              }}
              role="button"
              tabIndex={0}
            >
              <div className={styles.itemMain}>
                <div className={styles.itemTitle}>{c.title || '新对话'}</div>
                <div className={styles.itemTime}>{formatTime(c.lastMessageAt)}</div>
              </div>
              <button
                type="button"
                className={styles.delBtn}
                onClick={(e) => void handleDelete(e, c.id)}
                aria-label="删除会话"
              >
                ✕
              </button>
            </div>
          ))}
          {conversations.length === 0 && <div className={styles.empty}>暂无会话，点上方新建</div>}
        </div>
        <div className={styles.sidebarFooter}>
          <button type="button" className={styles.footerItem} onClick={handleGoSettings}>
            <span className={styles.footerIcon}>⚙️</span>
            <span>系统设置与模型管理</span>
          </button>
        </div>
      </aside>
    </>
  )
}

/** 相对时间：刚刚 / N 分钟前 / 今天 HH:MM / M/D */
function formatTime(iso?: string): string {
  if (!iso) return ''
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
