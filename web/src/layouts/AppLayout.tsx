import { tokens } from '../styles/tokens'

import styles from './AppLayout.module.css'

/** 移动端布局：390×844 视口，底部 TabBar，内容区滚动 */
export function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className={styles.app} style={{ fontFamily: tokens.fontFamily }}>
      <main className={styles.main}>{children}</main>
    </div>
  )
}
