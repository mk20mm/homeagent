/** 系统管理页（page-05-admin）：iOS Inset Grouped 极简美学设置 + 实时模型与操作网关 */
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router'

import {
  MEMBER_ROLE_LABEL,
  clearAuth,
  getAuth,
} from '@homeagent/shared'

import { api, getApiBaseUrl, unwrap } from '../../api/client'

import styles from './SettingsPage.module.css'

interface ModelItem {
  id: string
  model_name: string
  display_name: string
  provider?: string
  is_default?: boolean
}

interface AuditItem {
  id: string
  created_at: string
  tool_name: string
  result?: string
  undone?: boolean
}

export function SettingsPage() {
  const navigate = useNavigate()
  const auth = getAuth()

  const [models, setModels] = useState<ModelItem[]>([])
  const [loadingModels, setLoadingModels] = useState(true)
  const [switchingModelId, setSwitchingModelId] = useState<string | null>(null)
  
  const [recentAudits, setRecentAudits] = useState<AuditItem[]>([])
  const [showAuditModal, setShowAuditModal] = useState(false)

  const reloadModels = async () => {
    try {
      const res = await api.GET('/models')
      const data = unwrap(res)
      setModels(data.models ?? [])
    } catch {
      // 容错处理
    } finally {
      setLoadingModels(false)
    }
  }

  const reloadAudits = async () => {
    try {
      const res = await api.GET('/audit', {
        params: { query: { page_size: 10 } },
      })
      const data = unwrap(res)
      setRecentAudits(data.items ?? [])
    } catch {
      // 容错处理
    }
  }

  useEffect(() => {
    reloadModels()
    reloadAudits()
  }, [])

  const handleSetDefaultModel = async (model: ModelItem) => {
    if (model.is_default || switchingModelId) return
    setSwitchingModelId(model.id)
    try {
      await api.PUT('/admin/models/{modelId}', {
        params: { path: { modelId: model.id } },
        body: { is_default: true },
      })
      await reloadModels()
    } catch (err) {
      const msg = err instanceof Error ? err.message : '切换默认模型失败'
      alert(msg)
    } finally {
      setSwitchingModelId(null)
    }
  }

  const handleLogout = () => {
    clearAuth()
    navigate('/login', { replace: true })
  }

  const defaultModel = models.find((m) => m.is_default)

  return (
    <div className={styles.container}>
      <h1 className={styles.title}>系统管理</h1>

      {/* 用户个人名片 (Apple Profile Card) */}
      {auth && (
        <div className={styles.profileCard}>
          <div className={styles.avatar}>
            {MEMBER_ROLE_LABEL[auth.role]?.[0] || '家'}
          </div>
          <div className={styles.profileMain}>
            <div className={styles.profileName}>{MEMBER_ROLE_LABEL[auth.role]}</div>
            <div className={styles.roleBadge}>
              成员 ID: {auth.memberId.slice(0, 8)}
            </div>
          </div>
          <button
            type="button"
            className={styles.logoutBtn}
            onClick={handleLogout}
          >
            退出登录
          </button>
        </div>
      )}

      {/* Group 1: AI 核心与网关 (真实接口驱动) */}
      <div className={styles.groupHeader}>AI 模型与网关状态</div>
      <div className={styles.groupCard}>
        <div className={styles.row}>
          <div className={`${styles.iconBox} ${styles.iconPurple}`}>✦</div>
          <div className={styles.rowMain}>
            <div className={styles.rowLabel}>当前默认模型</div>
            <div className={styles.rowSub}>
              {loadingModels
                ? '检测模型中…'
                : defaultModel
                  ? `${defaultModel.display_name} (${defaultModel.provider || 'openai'})`
                  : '未设置默认模型'}
            </div>
          </div>
          <span className={styles.statusPill}>
            {defaultModel ? '已连接' : '待配置'}
          </span>
        </div>

        {/* 模型列表切换项 */}
        {models.map((m) => (
          <div key={m.id}>
            <div className={styles.divider} />
            <div
              className={styles.row}
              style={{ cursor: m.is_default ? 'default' : 'pointer' }}
              onClick={() => handleSetDefaultModel(m)}
            >
              <div className={`${styles.iconBox} ${styles.iconBlue}`}>🤖</div>
              <div className={styles.rowMain}>
                <div className={styles.rowLabel}>
                  {m.display_name}
                  {m.is_default && (
                    <span
                      style={{
                        marginLeft: 8,
                        fontSize: 11,
                        background: '#eafaf1',
                        color: '#27ae60',
                        padding: '2px 6px',
                        borderRadius: 6,
                        fontWeight: 600,
                      }}
                    >
                      默认
                    </span>
                  )}
                </div>
                <div className={styles.rowSub}>底层模型: {m.model_name}</div>
              </div>
              <span style={{ fontSize: 13, color: '#007aff', fontWeight: 500 }}>
                {switchingModelId === m.id
                  ? '切换中…'
                  : m.is_default
                    ? '生效中'
                    : '设为默认'}
              </span>
            </div>
          </div>
        ))}
      </div>

      {/* Group 2: 操作留痕与 24h 撤销中心 */}
      <div className={styles.groupHeader}>操作安全与审计</div>
      <div className={styles.groupCard}>
        <div
          className={styles.row}
          style={{ cursor: 'pointer' }}
          onClick={() => setShowAuditModal(true)}
        >
          <div className={`${styles.iconBox} ${styles.iconOrange}`}>🛡️</div>
          <div className={styles.rowMain}>
            <div className={styles.rowLabel}>近期操作日志</div>
            <div className={styles.rowSub}>
              共记录 {recentAudits.length} 项操作（点击可查看留痕与回退）
            </div>
          </div>
          <span className={styles.valueText}>查看 〉</span>
        </div>
      </div>

      {/* Group 3: 家庭成员权限 */}
      <div className={styles.groupHeader}>家庭角色说明</div>
      <div className={styles.groupCard}>
        <div className={styles.row}>
          <div className={`${styles.iconBox} ${styles.iconOrange}`}>👑</div>
          <div className={styles.rowMain}>
            <div className={styles.rowLabel}>家长角色</div>
            <div className={styles.rowSub}>记账、家务、厨房、模型管理全量权限</div>
          </div>
          <span className={styles.roleTag}>完全控制</span>
        </div>
        <div className={styles.divider} />
        <div className={styles.row}>
          <div className={`${styles.iconBox} ${styles.iconGreen}`}>👵</div>
          <div className={styles.rowMain}>
            <div className={styles.rowLabel}>老人角色</div>
            <div className={styles.rowSub}>简化记账、厨房大字分步模式、日常待办</div>
          </div>
          <span className={styles.roleTag}>常用权限</span>
        </div>
        <div className={styles.divider} />
        <div className={styles.row}>
          <div className={`${styles.iconBox} ${styles.iconPink}`}>🧒</div>
          <div className={styles.rowMain}>
            <div className={styles.rowLabel}>小孩角色</div>
            <div className={styles.rowSub}>家务认领与打卡积分，账本数据隔离保护</div>
          </div>
          <span className={styles.roleTag}>受限保护</span>
        </div>
      </div>

      {/* Group 4: 网络与服务端基址 */}
      <div className={styles.groupHeader}>服务端网络</div>
      <div className={styles.groupCard}>
        <div className={styles.row}>
          <div className={`${styles.iconBox} ${styles.iconCyan}`}>🌐</div>
          <div className={styles.rowMain}>
            <div className={styles.rowLabel}>当前 API 基址</div>
            <div className={styles.rowSub} style={{ wordBreak: 'break-all', fontFamily: 'monospace' }}>
              {getApiBaseUrl()}
            </div>
          </div>
          <span className={styles.valueText}>在线</span>
        </div>
        <div className={styles.divider} />
        <div className={styles.row}>
          <div className={`${styles.iconBox} ${styles.iconGray}`}>💾</div>
          <div className={styles.rowMain}>
            <div className={styles.rowLabel}>私有化存储</div>
            <div className={styles.rowSub}>SQLite 本地家庭专有库，数据安全留存</div>
          </div>
          <span className={styles.valueText}>安全加密</span>
        </div>
      </div>

      {/* 操作日志抽屉 Modal */}
      {showAuditModal && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            background: 'rgba(0,0,0,0.4)',
            backdropFilter: 'blur(8px)',
            zIndex: 1000,
            display: 'flex',
            alignItems: 'flex-end',
          }}
          onClick={() => setShowAuditModal(false)}
        >
          <div
            style={{
              background: '#ffffff',
              width: '100%',
              maxHeight: '75vh',
              borderTopLeftRadius: 24,
              borderTopRightRadius: 24,
              padding: '20px 20px 36px',
              overflowY: 'auto',
            }}
            onClick={(e) => e.stopPropagation()}
          >
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                marginBottom: 16,
              }}
            >
              <h2 style={{ fontSize: 18, fontWeight: 700, margin: 0 }}>近期操作审计留痕</h2>
              <button
                type="button"
                style={{
                  background: '#f2f2f7',
                  border: 'none',
                  borderRadius: 16,
                  padding: '6px 12px',
                  fontSize: 13,
                  cursor: 'pointer',
                }}
                onClick={() => setShowAuditModal(false)}
              >
                关闭
              </button>
            </div>

            {recentAudits.length === 0 ? (
              <div style={{ textAlign: 'center', padding: '30px 0', color: '#8e8e93' }}>
                暂无操作记录
              </div>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                {recentAudits.map((item) => (
                  <div
                    key={item.id}
                    style={{
                      background: '#f9f9fb',
                      borderRadius: 14,
                      padding: '12px 14px',
                      display: 'flex',
                      justifyContent: 'space-between',
                      alignItems: 'center',
                    }}
                  >
                    <div>
                      <div style={{ fontSize: 14, fontWeight: 600, color: '#1c1c1e' }}>
                        {item.tool_name}
                      </div>
                      <div style={{ fontSize: 12, color: '#8e8e93', marginTop: 2 }}>
                        {item.created_at.slice(0, 19).replace('T', ' ')}
                      </div>
                    </div>
                    <div>
                      {item.undone ? (
                        <span
                          style={{
                            fontSize: 12,
                            color: '#ff3b30',
                            background: '#ffe5e5',
                            padding: '3px 8px',
                            borderRadius: 8,
                          }}
                        >
                          已撤销
                        </span>
                      ) : (
                        <span
                          style={{
                            fontSize: 12,
                            color: '#34c759',
                            background: '#eafaf1',
                            padding: '3px 8px',
                            borderRadius: 8,
                          }}
                        >
                          执行成功
                        </span>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
