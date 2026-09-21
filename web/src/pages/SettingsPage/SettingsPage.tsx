/**
 * 设置页（page-05-settings）：家人端个人设置。
 *
 * 家人端只放真实可用的入口；供应商密钥、权限矩阵、审计、用量统计归
 * admin 管理端（独立部署），不在这里出现死链（T-A08）。
 */
import { useState } from 'react'
import { useNavigate } from 'react-router'

import { MEMBER_ROLE_LABEL, clearAuth, getAuth } from '@homeagent/shared'

import styles from './SettingsPage.module.css'

/** 应用版本号：构建时注入，默认本地开发值 */
const APP_VERSION = import.meta.env.VITE_APP_VERSION ?? 'dev'

export function SettingsPage() {
  const navigate = useNavigate()
  const auth = getAuth()
  const [confirming, setConfirming] = useState(false)

  const handleLogout = () => {
    clearAuth()
    navigate('/login', { replace: true })
  }

  return (
    <div>
      <h1 className={styles.title}>我的</h1>

      {auth && (
        <div className={styles.section}>当前登录</div>
      )}
      {auth && (
        <div className={styles.list}>
          <div className={styles.item}>
            <span>成员角色</span>
            <span className={styles.value}>{MEMBER_ROLE_LABEL[auth.role]}</span>
          </div>
          <div className={styles.item}>
            <span>登录状态</span>
            <span className={styles.value}>已登录</span>
          </div>
          <div className={styles.item}>
            <span>退出登录</span>
            {confirming ? (
              <span className={styles.confirm}>
                <button
                  type="button"
                  className={styles.confirmBtn}
                  onClick={handleLogout}
                >
                  确认退出
                </button>
                <button
                  type="button"
                  className={styles.cancelBtn}
                  onClick={() => setConfirming(false)}
                >
                  取消
                </button>
              </span>
            ) : (
              <button
                type="button"
                className={styles.configure}
                onClick={() => setConfirming(true)}
              >
                退出
              </button>
            )}
          </div>
        </div>
      )}

      <div className={styles.section}>数据与安全</div>
      <div className={styles.list}>
        <button
          type="button"
          className={styles.item}
          onClick={() => navigate('/undo')}
        >
          <span>可撤销的操作</span>
          <span className={styles.configure}>24 小时内可回退 →</span>
        </button>
      </div>

      <div className={styles.section}>使用帮助</div>
      <div className={styles.list}>
        <div className={styles.item}>
          <span>快捷按钮</span>
          <span className={styles.value}>对话框上方，点一下直接办事</span>
        </div>
        <div className={styles.item}>
          <span>记账</span>
          <span className={styles.value}>「买菜花了 120」直接说金额</span>
        </div>
        <div className={styles.item}>
          <span>报饭</span>
          <span className={styles.value}>「今晚不回家吃」一句话申报</span>
        </div>
        <div className={styles.item}>
          <span>反悔</span>
          <span className={styles.value}>操作后 24 小时内可撤销</span>
        </div>
      </div>

      <div className={styles.section}>关于</div>
      <div className={styles.list}>
        <div className={styles.item}>
          <span>版本</span>
          <span className={styles.value}>{APP_VERSION}</span>
        </div>
      </div>
    </div>
  )
}
