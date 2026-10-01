import { NavLink } from 'react-router'

import styles from './TabBar.module.css'

/** 底部 Tab 栏：毛玻璃半透明，5 个入口（对话/家务/记账/厨房/出行） */
const TABS = [
  { to: '/', label: '对话', icon: '💬' },
  { to: '/chores', label: '家务', icon: '✅' },
  { to: '/money', label: '财务', icon: '💰' },
  { to: '/kitchen', label: '厨房', icon: '🍳' },
  { to: '/travel', label: '出行', icon: '🚗' },
] as const

export function TabBar() {
  return (
    <nav className={styles.bar}>
      {TABS.map((t) => (
        <NavLink
          key={t.to}
          to={t.to}
          end={t.to === '/'}
          className={({ isActive }) => `${styles.item} ${isActive ? styles.active : ''}`}
        >
          <span className={styles.icon}>{t.icon}</span>
          <span className={styles.label}>{t.label}</span>
        </NavLink>
      ))}
    </nav>
  )
}
