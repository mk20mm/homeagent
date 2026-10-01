/**
 * 家庭出行页（M6）：爱车台账 + 出行/充电/加油/停车记录 + 财务记账联动（Apple 极简风格）
 */
import { useCallback, useEffect, useState } from 'react'

import { formatYuan, yuanToCents } from '@homeagent/shared'
import { api, unwrap } from '../../api/client'
import type { components } from '../../api/schema'

import styles from './TravelPage.module.css'

type Vehicle = components['schemas']['Vehicle']
type Trip = components['schemas']['Trip']

const TRIP_TYPE_MAP: Record<string, { label: string; icon: string }> = {
  gas: { label: '加油', icon: '⛽' },
  charging: { label: '充电', icon: '⚡' },
  parking: { label: '停车费', icon: '🅿️' },
  toll: { label: '高速通行费', icon: '🛣️' },
  maintenance: { label: '保养维修', icon: '🔧' },
  ride: { label: '出行交通', icon: '🚗' },
}

export function TravelPage() {
  const [vehicles, setVehicles] = useState<Vehicle[]>([])
  const [trips, setTrips] = useState<Trip[]>([])
  const [loading, setLoading] = useState(true)

  // 弹窗状态
  const [modalOpen, setModalOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [tripType, setTripType] = useState<
    'gas' | 'charging' | 'parking' | 'toll' | 'maintenance' | 'ride'
  >('charging')
  const [amountYuan, setAmountYuan] = useState('')
  const [mileage, setMileage] = useState('')
  const [note, setNote] = useState('')

  const loadData = useCallback(async () => {
    try {
      const [vRes, tRes] = await Promise.all([api.GET('/vehicles'), api.GET('/trips')])
      const vData = unwrap(vRes)
      const tData = unwrap(tRes)
      setVehicles(vData.items ?? [])
      setTrips(tData.items ?? [])
    } catch {
      // 忽略初次加载错误
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadData()
  }, [loadData])

  const primaryVehicle = vehicles[0]

  const handleQuickAction = (
    type: 'gas' | 'charging' | 'parking' | 'toll' | 'maintenance' | 'ride',
  ) => {
    setTripType(type)
    setAmountYuan('')
    setMileage(primaryVehicle ? String(primaryVehicle.current_mileage || '') : '')
    setNote('')
    setModalOpen(true)
  }

  const handleCreateDefaultVehicle = async () => {
    try {
      await api.POST('/vehicles', {
        body: {
          name: '家庭爱车 (Model Y)',
          plate_number: '京A·88888',
          vehicle_type: 'ev',
          current_mileage: 24800,
        },
      })
      await loadData()
    } catch (err) {
      alert(err instanceof Error ? err.message : '创建车辆失败')
    }
  }

  const handleSubmitTrip = async (e: React.FormEvent) => {
    e.preventDefault()
    const cents = yuanToCents(parseFloat(amountYuan.trim()) || 0)
    setSubmitting(true)
    try {
      await api.POST('/trips', {
        body: {
          vehicle_id: primaryVehicle?.id,
          trip_type: tripType,
          amount_cents: cents,
          mileage: mileage ? parseInt(mileage, 10) : undefined,
          note: note.trim() || undefined,
        },
      })
      setModalOpen(false)
      await loadData()
    } catch (err) {
      alert(err instanceof Error ? err.message : '记录失败')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className={styles.container}>
      <header className={styles.header}>
        <div className={styles.titleArea}>
          <div className={styles.title}>家庭出行 🚗</div>
          <div className={styles.subtitle}>爱车台账与出行开销管理</div>
        </div>
        <button
          type="button"
          className={styles.addBtn}
          onClick={() => handleQuickAction('charging')}
        >
          ＋ 记一笔
        </button>
      </header>

      {/* 爱车台账卡片 */}
      {primaryVehicle ? (
        <div className={styles.vehicleCard}>
          <div className={styles.vehicleHeader}>
            <div>
              <div className={styles.vehicleName}>{primaryVehicle.name}</div>
              <div className={styles.vehicleBadges}>
                <span className={styles.badgePlate}>{primaryVehicle.plate_number}</span>
                <span className={styles.badgeType}>
                  {primaryVehicle.vehicle_type === 'ev'
                    ? '⚡ 纯电 EV'
                    : primaryVehicle.vehicle_type === 'hybrid'
                      ? '🔋 混动'
                      : '⛽ 燃油车'}
                </span>
              </div>
            </div>
          </div>
          <div className={styles.statsGrid}>
            <div className={styles.statItem}>
              <span className={styles.statLabel}>当前总里程</span>
              <span className={styles.statVal}>
                {primaryVehicle.current_mileage.toLocaleString()}
                <span className={styles.statUnit}>km</span>
              </span>
            </div>
            <div className={styles.statItem}>
              <span className={styles.statLabel}>上次保养</span>
              <span className={styles.statVal}>
                {(primaryVehicle.last_maintenance_mileage || 20000).toLocaleString()}
                <span className={styles.statUnit}>km</span>
              </span>
            </div>
            <div className={styles.statItem}>
              <span className={styles.statLabel}>建议下次保养</span>
              <span className={styles.statVal}>
                {(primaryVehicle.next_maintenance_mileage || 30000).toLocaleString()}
                <span className={styles.statUnit}>km</span>
              </span>
            </div>
          </div>
        </div>
      ) : (
        <div className={styles.noVehicleCard}>
          <div className={styles.noVehicleTitle}>尚未登记家庭车辆</div>
          <div className={styles.noVehicleDesc}>登记爱车后可统计行驶里程、油耗电耗与保养提醒</div>
          <button
            type="button"
            className={styles.regBtn}
            onClick={() => void handleCreateDefaultVehicle()}
          >
            一键登记示例车辆（Model Y）
          </button>
        </div>
      )}

      {/* 快捷动作胶囊栏 */}
      <div className={styles.sectionTitle}>快捷记用车</div>
      <div className={styles.actionRow}>
        <button
          type="button"
          className={styles.actionBtn}
          onClick={() => handleQuickAction('charging')}
        >
          <span className={styles.actionIcon}>⚡</span>
          <span className={styles.actionText}>充电</span>
        </button>
        <button
          type="button"
          className={styles.actionBtn}
          onClick={() => handleQuickAction('gas')}
        >
          <span className={styles.actionIcon}>⛽</span>
          <span className={styles.actionText}>加油</span>
        </button>
        <button
          type="button"
          className={styles.actionBtn}
          onClick={() => handleQuickAction('parking')}
        >
          <span className={styles.actionIcon}>🅿️</span>
          <span className={styles.actionText}>停车费</span>
        </button>
        <button
          type="button"
          className={styles.actionBtn}
          onClick={() => handleQuickAction('toll')}
        >
          <span className={styles.actionIcon}>🛣️</span>
          <span className={styles.actionText}>高速通行</span>
        </button>
        <button
          type="button"
          className={styles.actionBtn}
          onClick={() => handleQuickAction('maintenance')}
        >
          <span className={styles.actionIcon}>🔧</span>
          <span className={styles.actionText}>保养维修</span>
        </button>
      </div>

      {/* 出行与用车记录流水 */}
      <div className={styles.sectionTitle}>近期出行开销</div>
      <div className={styles.tripList}>
        {trips.length > 0 ? (
          trips.map((t) => {
            const meta = TRIP_TYPE_MAP[t.trip_type] || { label: '出行', icon: '🚗' }
            const dateStr = t.occurred_at ? new Date(t.occurred_at).toLocaleDateString() : ''
            return (
              <div key={t.id} className={styles.tripItem}>
                <div className={styles.tripLeft}>
                  <div className={styles.tripIconBox}>{meta.icon}</div>
                  <div className={styles.tripDetails}>
                    <div className={styles.tripTitle}>{meta.label}</div>
                    <div className={styles.tripSub}>
                      <span>{dateStr}</span>
                      {t.mileage ? <span>{t.mileage.toLocaleString()} km</span> : null}
                      {t.note ? <span>{t.note}</span> : null}
                    </div>
                  </div>
                </div>
                <div className={styles.tripRight}>
                  {t.amount_cents > 0 ? (
                    <>
                      <span className={styles.tripAmount}>-¥{formatYuan(t.amount_cents)}</span>
                      <span className={styles.linkedBadge}>联动记账</span>
                    </>
                  ) : (
                    <span style={{ fontSize: 13, color: '#8e8e93' }}>无费用</span>
                  )}
                </div>
              </div>
            )
          })
        ) : (
          <div className={styles.emptyTrips}>
            {loading ? '正在加载出行记录…' : '暂无出行记录，对 Agent 说『今天加油 300 元』试试'}
          </div>
        )}
      </div>

      {/* 记出行浮层弹窗 */}
      {modalOpen && (
        <div className={styles.modalOverlay} onClick={() => setModalOpen(false)}>
          <div className={styles.modalSheet} onClick={(e) => e.stopPropagation()}>
            <div className={styles.modalHeader}>
              <div className={styles.modalTitle}>记录出行与用车</div>
              <button
                type="button"
                className={styles.closeBtn}
                onClick={() => setModalOpen(false)}
              >
                ✕
              </button>
            </div>
            <form onSubmit={(e) => void handleSubmitTrip(e)}>
              <div className={styles.formGroup}>
                <label className={styles.label}>类型</label>
                <div className={styles.typeSelector}>
                  {(
                    ['charging', 'gas', 'parking', 'toll', 'maintenance', 'ride'] as const
                  ).map((t) => (
                    <button
                      key={t}
                      type="button"
                      className={`${styles.typeBtn} ${tripType === t ? styles.typeBtnActive : ''}`}
                      onClick={() => setTripType(t)}
                    >
                      {TRIP_TYPE_MAP[t].icon} {TRIP_TYPE_MAP[t].label}
                    </button>
                  ))}
                </div>
              </div>

              <div className={styles.formGroup}>
                <label className={styles.label}>费用金额（元，将自动记入财务账本）</label>
                <input
                  type="number"
                  step="0.01"
                  placeholder="0.00"
                  className={styles.input}
                  value={amountYuan}
                  onChange={(e) => setAmountYuan(e.target.value)}
                  autoFocus
                />
              </div>

              <div className={styles.formGroup}>
                <label className={styles.label}>当前车辆里程（公里，选填）</label>
                <input
                  type="number"
                  placeholder={
                    primaryVehicle ? `当前：${primaryVehicle.current_mileage}` : '例如 24850'
                  }
                  className={styles.input}
                  value={mileage}
                  onChange={(e) => setMileage(e.target.value)}
                />
              </div>

              <div className={styles.formGroup}>
                <label className={styles.label}>备注说明（选填）</label>
                <input
                  type="text"
                  placeholder="例如：万达商场地库、京港澳高速通行"
                  className={styles.input}
                  value={note}
                  onChange={(e) => setNote(e.target.value)}
                />
              </div>

              <button type="submit" className={styles.submitBtn} disabled={submitting}>
                {submitting ? '保存中…' : '保存并同步记账'}
              </button>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
