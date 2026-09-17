/** 系统管理页（page-05-admin）：供应商配置/用量/权限矩阵入口（移动端精简版） */
import { useNavigate } from 'react-router'

import {
  MEMBER_ROLE_LABEL,
  PROVIDER_LABEL,
  type MemberRole,
  type ProviderName,
  clearAuth,
  getAuth,
} from '@homeagent/shared'

import styles from './SettingsPage.module.css'

export function SettingsPage() {
  const navigate = useNavigate()
  const auth = getAuth()

  return (
    <div>
      <h1 className={styles.title}>系统管理</h1>

      {auth && (
        <div className={styles.section}>
          当前登录
          <div className={styles.list}>
            <div className={styles.item}>
              <span>角色：{MEMBER_ROLE_LABEL[auth.role]}</span>
              <span
                className={styles.configure}
                onClick={() => {
                  clearAuth()
                  navigate('/login', { replace: true })
                }}
              >
                退出登录
              </span>
            </div>
          </div>
        </div>
      )}

      <div className={styles.section}>模型供应商</div>
      <div className={styles.list}>
        {(Object.keys(PROVIDER_LABEL) as ProviderName[]).map((p) => (
          <div key={p} className={styles.item}>
            <span>{PROVIDER_LABEL[p]}</span>
            <span className={styles.configure}>配置 →</span>
          </div>
        ))}
      </div>

      <div className={styles.section}>家庭成员权限</div>
      <div className={styles.list}>
        {(Object.keys(MEMBER_ROLE_LABEL) as MemberRole[]).map((r) => (
          <div key={r} className={styles.item}>
            <span>{MEMBER_ROLE_LABEL[r]}</span>
            <span className={styles.configure}>权限 →</span>
          </div>
        ))}
      </div>

      <div className={styles.section}>数据</div>
      <div className={styles.list}>
        <div className={styles.item}>
          <span>操作审计</span>
          <span className={styles.configure}>查看 →</span>
        </div>
        <div className={styles.item}>
          <span>用量统计</span>
          <span className={styles.configure}>查看 →</span>
        </div>
      </div>
    </div>
  )
}
