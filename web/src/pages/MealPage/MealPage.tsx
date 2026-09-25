/** 报饭页（page-04-meal）：申报今晚是否在家用餐 + 汇总给做饭人 */
import { useCallback, useEffect, useState } from 'react'

import { api, unwrap } from '../../api/client'

import styles from './MealPage.module.css'

interface MealMember {
  member_id: string
  name: string
  at_home: boolean
}

interface MealData {
  date: string
  at_home_count: number
  not_at_home_count: number
  members: MealMember[]
}

export function MealPage() {
  const [data, setData] = useState<MealData | null>(null)
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)

  const fetchMeals = useCallback(async () => {
    try {
      const res = await api.GET('/meals')
      const d = unwrap(res)
      setData(d)
    } catch {
      // 接口可能尚未上线（渐进式）
      setData(null)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchMeals()
  }, [fetchMeals])

  const handleReport = async (atHome: boolean) => {
    setSubmitting(true)
    try {
      await api.POST('/meals', {
        body: { at_home: atHome },
      })
      // 刷新汇总
      await fetchMeals()
    } catch (err) {
      const msg = err instanceof Error ? err.message : '报饭失败'
      alert(msg)
    } finally {
      setSubmitting(false)
    }
  }

  if (loading) {
    return (
      <div>
        <h1 className={styles.title}>报饭</h1>
        <div className={styles.empty}>加载中...</div>
      </div>
    )
  }

  const atHomeCount = data?.at_home_count ?? 0
  const notAtHomeCount = data?.not_at_home_count ?? 0
  const members = data?.members ?? []
  const reported = members.filter((m) => m.at_home !== undefined)
  const atHomeMembers = reported.filter((m) => m.at_home)
  const notAtHomeMembers = reported.filter((m) => !m.at_home)

  return (
    <div>
      <h1 className={styles.title}>报饭</h1>

      <div className={styles.today}>
        <div className={styles.count}>
          今晚 {atHomeCount} 人在家吃
          {notAtHomeCount > 0 && ` · ${notAtHomeCount} 人不在家`}
        </div>
        <div className={styles.sub}>
          {members.length === 0
            ? '暂无人申报，试试在对话里说「今晚不回家吃」'
            : `共 ${members.length} 人已申报`}
        </div>
      </div>

      <div className={styles.section}>我的申报</div>
      <div className={styles.actions}>
        <button
          type="button"
          className={`${styles.btn} ${styles.btnOn}`}
          disabled={submitting}
          onClick={() => handleReport(true)}
        >
          🏠 在家吃
        </button>
        <button
          type="button"
          className={`${styles.btn} ${styles.btnOff}`}
          disabled={submitting}
          onClick={() => handleReport(false)}
        >
          🚪 不在家吃
        </button>
      </div>

      {atHomeMembers.length > 0 && (
        <>
          <div className={styles.section}>
            在家吃（{atHomeMembers.length}）
          </div>
          <div className={styles.list}>
            {atHomeMembers.map((m) => (
              <div key={m.member_id} className={styles.item}>
                <span className={styles.name}>{m.name}</span>
                <span className={styles.tagOn}>在家吃</span>
              </div>
            ))}
          </div>
        </>
      )}

      {notAtHomeMembers.length > 0 && (
        <>
          <div className={styles.section}>
            不在家（{notAtHomeMembers.length}）
          </div>
          <div className={styles.list}>
            {notAtHomeMembers.map((m) => (
              <div key={m.member_id} className={styles.item}>
                <span className={styles.name}>{m.name}</span>
                <span className={styles.tagOff}>不在家</span>
              </div>
            ))}
          </div>
        </>
      )}
    </div>
  )
}
