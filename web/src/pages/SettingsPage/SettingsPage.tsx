/** 系统管理页（page-05-admin）：iOS Inset Grouped 极简美学设置 */
import { useNavigate } from 'react-router'

import {
  MEMBER_ROLE_LABEL,
  clearAuth,
  getAuth,
} from '@homeagent/shared'

import styles from './SettingsPage.module.css'

export function SettingsPage() {
  const navigate = useNavigate()
  const auth = getAuth()

  const handleLogout = () => {
    clearAuth()
    navigate('/login', { replace: true })
  }

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

      {/* Group 1: AI 核心与网关 */}
      <div className={styles.groupHeader}>AI 模型与网关</div>
      <div className={styles.groupCard}>
        <div className={styles.row}>
          <div className={`${styles.iconBox} ${styles.iconPurple}`}>✦</div>
          <div className={styles.rowMain}>
            <div className={styles.rowLabel}>模型动态网关</div>
            <div className={styles.rowSub}>自动探测与多供应商热重载</div>
          </div>
          <span className={styles.statusPill}>正常在线</span>
        </div>
        <div className={styles.divider} />
        <div className={styles.row}>
          <div className={`${styles.iconBox} ${styles.iconBlue}`}>⚡</div>
          <div className={styles.rowMain}>
            <div className={styles.rowLabel}>流式传输响应</div>
            <div className={styles.rowSub}>SSE 全双工逐字打字机</div>
          </div>
          <span className={styles.valueText}>已启用</span>
        </div>
      </div>

      {/* Group 2: 家庭成员权限 */}
      <div className={styles.groupHeader}>家庭角色与权限</div>
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

      {/* Group 3: 数据安全与隐私 */}
      <div className={styles.groupHeader}>数据与安全</div>
      <div className={styles.groupCard}>
        <div className={styles.row}>
          <div className={`${styles.iconBox} ${styles.iconIndigo}`}>🛡️</div>
          <div className={styles.rowMain}>
            <div className={styles.rowLabel}>操作撤销窗口</div>
            <div className={styles.rowSub}>记账与打卡支持 24 小时逆向回滚</div>
          </div>
          <span className={styles.valueText}>24h 保护</span>
        </div>
        <div className={styles.divider} />
        <div className={styles.row}>
          <div className={`${styles.iconBox} ${styles.iconGray}`}>💾</div>
          <div className={styles.rowMain}>
            <div className={styles.rowLabel}>私有化存储</div>
            <div className={styles.rowSub}>SQLite 本地家庭专有库，数据不外流</div>
          </div>
          <span className={styles.valueText}>安全加密</span>
        </div>
        <div className={styles.divider} />
        <div className={styles.row}>
          <div className={`${styles.iconBox} ${styles.iconCyan}`}>📱</div>
          <div className={styles.rowMain}>
            <div className={styles.rowLabel}>客户端版本</div>
            <div className={styles.rowSub}>家事 Agent 移动端 / 安卓原生版</div>
          </div>
          <span className={styles.valueText}>v1.0.0</span>
        </div>
      </div>
    </div>
  )
}
