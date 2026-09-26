/**
 * 日程详情页（A-02-02/03）：对话结果卡片 / Today / 日程列表的深链目标。
 * 接 GET /events/{eventId}；无权/不存在统一 404（后端不区分二者，防探测）。
 * 修改入口按服务端给的 can_edit 渲染（本期固定 false，不承诺不存在的能力）。
 */
import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router'

import { api, unwrap, ApiError } from '../../api/client'
import type { paths } from '../../api/schema'

import styles from './EventDetailPage.module.css'

type EventDetail =
  paths['/events/{eventId}']['get']['responses'][200]['content']['application/json']

function fullTimeLabel(iso: string): string {
  if (!iso) return '—'
  const d = new Date(iso)
  return `${d.getFullYear()}年${d.getMonth() + 1}月${d.getDate()}日 ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

function repeatLabel(detail: EventDetail): string {
  const n = detail.interval ?? 1
  switch (detail.repeat) {
    case 'daily':
      return n > 1 ? `每 ${n} 天` : '每天'
    case 'weekly':
      return n > 1 ? `每 ${n} 周` : '每周'
    case 'monthly':
      return n > 1 ? `每 ${n} 个月` : '每月'
    default:
      return '单次'
  }
}

export function EventDetailPage() {
  const { eventId } = useParams<{ eventId: string }>()
  const [detail, setDetail] = useState<EventDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const navigate = useNavigate()

  useEffect(() => {
    if (!eventId) {
      setError('缺少事件 id')
      setLoading(false)
      return
    }
    void (async () => {
      try {
        const data = unwrap(
          await api.GET('/events/{eventId}', { params: { path: { eventId } } }),
        )
        setDetail(data)
        setError(null)
      } catch (e) {
        // 无权与不存在都是 404，统一提示，不区分
        if (e instanceof ApiError && (e.code === 'not_found' || e.code === 'permission_denied')) {
          setError('日程不存在或你没有查看权限')
        } else {
          setError('加载失败，请重试')
        }
      } finally {
        setLoading(false)
      }
    })()
  }, [eventId])

  const cancelSeries = async () => {
    if (!eventId || !detail) return
    setBusy(true)
    setError(null)
    try {
      unwrap(
        await api.DELETE('/events/{eventId}', { params: { path: { eventId } } }),
      )
      navigate('/events', { replace: true })
    } catch (e) {
      if (e instanceof ApiError && e.code === 'not_found') {
        setError('该日程已不存在')
      } else {
        setError('删除失败，请重试')
      }
    } finally {
      setBusy(false)
      setConfirming(false)
    }
  }

  if (loading) {
    return (
      <div>
        <h1 className={styles.title}>日程详情</h1>
        <div className={styles.sub}>加载中…</div>
      </div>
    )
  }

  if (error || !detail) {
    return (
      <div>
        <h1 className={styles.title}>日程详情</h1>
        {error && <div className={styles.error}>{error}</div>}
        <button
          type="button"
          className={styles.backBtn}
          onClick={() => navigate('/events', { replace: true })}
        >
          返回日程列表
        </button>
      </div>
    )
  }

  return (
    <div>
      <button
        type="button"
        className={styles.backBtn}
        onClick={() => navigate('/events')}
      >
        ‹ 日程
      </button>

      <h1 className={styles.title}>{detail.title}</h1>

      <div className={styles.section}>
        <div className={styles.row}>
          <span className={styles.label}>开始</span>
          <span className={styles.value}>{fullTimeLabel(detail.start_at)}</span>
        </div>
        {detail.end_at && (
          <div className={styles.row}>
            <span className={styles.label}>结束</span>
            <span className={styles.value}>{fullTimeLabel(detail.end_at)}</span>
          </div>
        )}
        <div className={styles.row}>
          <span className={styles.label}>重复</span>
          <span className={styles.value}>{repeatLabel(detail)}</span>
        </div>
        <div className={styles.row}>
          <span className={styles.label}>可见范围</span>
          <span className={styles.value}>
            {detail.visibility === 'private' ? '仅自己可见' : '全家可见'}
          </span>
        </div>
        <div className={styles.row}>
          <span className={styles.label}>创建人</span>
          <span className={styles.value}>{detail.owner_name}</span>
        </div>
        {detail.location && (
          <div className={styles.row}>
            <span className={styles.label}>地点</span>
            <span className={styles.value}>{detail.location}</span>
          </div>
        )}
        {detail.is_lunar && (
          <div className={styles.row}>
            <span className={styles.label}>农历</span>
            <span className={styles.value}>按农历计算（生日/纪念日）</span>
          </div>
        )}
      </div>

      {detail.description && (
        <div className={styles.section}>
          <div className={styles.sectionTitle}>说明</div>
          <div className={styles.description}>{detail.description}</div>
        </div>
      )}

      {!detail.can_edit && (
        <div className={styles.note}>
          修改单次或整条日程暂未开放；如需调整，可以删除后重新创建。
        </div>
      )}

      {detail.can_cancel_series && (
        <div className={styles.section}>
          {confirming ? (
            <div className={styles.confirm}>
              <div className={styles.confirmText}>
                {detail.repeat === 'once'
                  ? '确定删除这条日程吗？删除后 24 小时内可撤销。'
                  : '这会删除整条重复日程（包括未来所有次）。确定吗？删除后 24 小时内可撤销。'}
              </div>
              <div className={styles.confirmRow}>
                <button
                  type="button"
                  className={styles.cancelBtn}
                  disabled={busy}
                  onClick={() => setConfirming(false)}
                >
                  取消
                </button>
                <button
                  type="button"
                  className={styles.dangerBtn}
                  disabled={busy}
                  onClick={() => void cancelSeries()}
                >
                  {busy ? '删除中…' : '确认删除'}
                </button>
              </div>
            </div>
          ) : (
            <button
              type="button"
              className={styles.dangerLink}
              onClick={() => setConfirming(true)}
            >
              删除整条日程
            </button>
          )}
        </div>
      )}
    </div>
  )
}
