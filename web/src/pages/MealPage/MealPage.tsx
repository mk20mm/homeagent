/** 报饭页（page-04-meal）：申报今晚是否在家用餐 + 汇总给做饭人（含未申报缺口） */
import { useEffect, useState } from 'react'

import { getAuth } from '@homeagent/shared'

import { api, unwrap } from '../../api/client'
import type { paths } from '../../api/schema'

import styles from './MealPage.module.css'

type MealSummary = paths['/meals']['get']['responses'][200]['content']['application/json']

export function MealPage() {
  const [summary, setSummary] = useState<MealSummary | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const [nudged, setNudged] = useState<string | null>(null)

  const myId = getAuth()?.memberId ?? ''
  const mine = summary?.members.find((m) => m.member_id === myId)
  // 未申报时 mine 为 undefined，两个按钮都不高亮
  const myAtHome = mine?.at_home ?? null

  const reload = async () => {
    try {
      const data = unwrap(await api.GET('/meals'))
      setSummary(data)
      setError(null)
    } catch {
      setError('报饭加载失败，下拉重试')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void reload()
  }, [])

  const report = async (atHome: boolean) => {
    if (saving || myAtHome === atHome) return
    setSaving(true)
    setError(null)
    try {
      unwrap(await api.POST('/meals', { body: { at_home: atHome } }))
      await reload()
    } catch {
      setError('申报失败，请重试')
    } finally {
      setSaving(false)
    }
  }

  const nudge = async (name: string) => {
    const text = `${name}，今晚在家吃吗？在报饭里说一声～`
    if (await copyText(text)) {
      setNudged(name)
      setTimeout(() => setNudged(null), 2400)
    } else {
      setNudged('复制失败，请手动提醒')
      setTimeout(() => setNudged(null), 2400)
    }
  }

  if (loading) {
    return (
      <div>
        <h1 className={styles.title}>报饭</h1>
        <div className={styles.sub}>加载中…</div>
      </div>
    )
  }

  return (
    <div>
      <h1 className={styles.title}>报饭</h1>

      {error && <div className={styles.error}>{error}</div>}

      <div className={styles.today}>
        <div className={styles.count}>
          今晚 {summary?.at_home_count ?? 0} 人在家吃
        </div>
        <div className={styles.sub}>
          {summary?.not_at_home_count ?? 0} 人不回来
          {summary && summary.unreported_count > 0
            ? ` · 还缺 ${summary.unreported_count} 人没报`
            : ' · 都报齐了'}
        </div>
      </div>

      <div className={styles.section}>我的申报</div>
      <div className={styles.actions}>
        <button
          type="button"
          className={`${styles.btn} ${myAtHome === true ? styles.btnOn : styles.btnOff}`}
          disabled={saving}
          onClick={() => void report(true)}
        >
          🏠 在家吃
        </button>
        <button
          type="button"
          className={`${styles.btn} ${myAtHome === false ? styles.btnOffActive : styles.btnOff}`}
          disabled={saving}
          onClick={() => void report(false)}
        >
          🚪 不在家吃
        </button>
      </div>
      {myAtHome === null && !error && (
        <div className={styles.hint}>你还没报今晚的饭</div>
      )}

      {summary && summary.at_home_count + summary.not_at_home_count > 0 && (
        <>
          <div className={styles.section}>已申报</div>
          <div className={styles.list}>
            {summary.members.map((m) => (
              <div key={m.member_id} className={styles.item}>
                <span className={styles.name}>{m.name}</span>
                <span className={m.at_home ? styles.tagOn : styles.tagOff}>
                  {m.at_home ? '在家吃' : '不在家'}
                </span>
              </div>
            ))}
          </div>
        </>
      )}

      {summary && summary.unreported_count > 0 && (
        <>
          <div className={styles.section}>还没报（缺 {summary.unreported_count} 人）</div>
          <div className={styles.list}>
            {summary.unreported.map((m) => (
              <div key={m.member_id} className={styles.item}>
                <span className={styles.name}>{m.name}</span>
                <button
                  type="button"
                  className={styles.nudgeBtn}
                  onClick={() => void nudge(m.name)}
                >
                  {nudged === m.name ? '已复制提醒' : '催办'}
                </button>
              </div>
            ))}
          </div>
          {nudged && nudged !== '复制失败，请手动提醒' && (
            <div className={styles.hint}>提醒文案已复制，发给 {nudged} 即可</div>
          )}
        </>
      )}
    </div>
  )
}

/** 复制文本：剪贴板 API 不可用时降级到 execCommand */
async function copyText(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard) {
      await navigator.clipboard.writeText(text)
      return true
    }
  } catch {
    // 落到降级方案
  }
  try {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.position = 'fixed'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.select()
    const ok = document.execCommand('copy')
    document.body.removeChild(ta)
    return ok
  } catch {
    return false
  }
}
