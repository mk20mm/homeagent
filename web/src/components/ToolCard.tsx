/**
 * 工具结果卡片（A-03）：task / task_done / meal / event 的结构化展示。
 * 取代旧的 JSON.stringify(card)。expense 仍走 ExpenseCard，本组件不处理。
 *
 * 状态表达原则（PRD A-03）：
 * - 只在服务端确认成功后展示成功态；
 * - duplicated（幂等命中）按类型给准确文案，不说「记账」；
 * - 「需要他人回应」类操作如实表达（本期催办=复制文案，未实现发送）。
 */
import { useNavigate } from 'react-router'

import styles from './ToolCard.module.css'

interface Card {
  type: string
  [key: string]: unknown
}

interface Props {
  card: Card
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

function visibilityLabel(visibility: string): string {
  return visibility === 'private' ? '仅自己可见' : '全家可见'
}

/** 把 RFC3339 时间转成「M月D日 HH:MM」 */
function dateTimeLabel(iso: string): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return `${d.getMonth() + 1}月${d.getDate()}日 ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

export function ToolCard({ card }: Props) {
  const navigate = useNavigate()

  switch (card.type) {
    case 'task': {
      const title = String(card.title ?? '')
      const duplicated = Boolean(card.duplicated)
      if (duplicated) {
        return <div className={styles.dup}>这个任务已经派过了，未重复创建</div>
      }
      return (
        <div className={styles.body}>
          <div className={styles.headline}>已派任务「{title}」</div>
          <div className={styles.facts}>
            {card.assignee ? <div>负责人：{String(card.assignee)}</div> : null}
            {card.due_at ? <div>截止：{String(card.due_at)}</div> : null}
          </div>
          <div className={styles.actions}>
            <button
              type="button"
              className={styles.linkBtn}
              onClick={() => navigate('/chores')}
            >
              去家务页查看
            </button>
          </div>
        </div>
      )
    }

    case 'task_done': {
      return (
        <div className={styles.body}>
          <div className={styles.headline}>
            已完成「{String(card.title ?? '')}」打卡
          </div>
          <div className={styles.actions}>
            <button
              type="button"
              className={styles.linkBtn}
              onClick={() => navigate('/chores')}
            >
              去家务页查看
            </button>
          </div>
        </div>
      )
    }

    case 'meal': {
      const atHome = Boolean(card.at_home)
      return (
        <div className={styles.body}>
          <div className={styles.headline}>
            已报饭（{String(card.date ?? '今天')}）：{atHome ? '今晚在家吃' : '今晚不在家吃'}
          </div>
          {card.note ? <div className={styles.facts}>{String(card.note)}</div> : null}
          <div className={styles.actions}>
            <button
              type="button"
              className={styles.linkBtn}
              onClick={() => navigate('/meal')}
            >
              去报饭页查看
            </button>
          </div>
        </div>
      )
    }

    case 'event': {
      const id = card.event_id ? String(card.event_id) : ''
      const duplicated = Boolean(card.duplicated)
      if (duplicated) {
        return <div className={styles.dup}>这个日程已经建过了，未重复创建</div>
      }
      return (
        <div className={styles.body}>
          <div className={styles.headline}>已建日程「{String(card.title ?? '')}」</div>
          <div className={styles.facts}>
            <div>时间：{dateTimeLabel(String(card.start_at ?? ''))}</div>
            <div>重复：{repeatLabel(String(card.repeat ?? 'once'))}</div>
            <div>可见：{visibilityLabel(String(card.visibility ?? 'family'))}</div>
          </div>
          <div className={styles.actions}>
            <button
              type="button"
              className={styles.linkBtn}
              onClick={() => id && navigate(`/events/${id}`)}
              disabled={!id}
            >
              去日程详情
            </button>
          </div>
        </div>
      )
    }

    default:
      // 未知卡片类型：兜底，不展示原始 JSON
      return <div className={styles.dup}>已处理</div>
  }
}
