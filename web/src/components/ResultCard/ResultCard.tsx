import { tokens } from '../../styles/tokens'

import styles from './ResultCard.module.css'

/** 工具执行的结构化回显：白底卡片 + 右上角「撤销」次级按钮（design-system.md） */
export function ResultCard({
  title,
  children,
  undoable,
  onUndo,
}: {
  title: string
  children: React.ReactNode
  undoable?: boolean
  onUndo?: () => void
}) {
  return (
    <div className={styles.card} style={{ borderRadius: tokens.radius.card }}>
      <div className={styles.header}>
        <span className={styles.title}>{title}</span>
        {undoable && (
          <button type="button" className={styles.undo} onClick={onUndo}>
            撤销
          </button>
        )}
      </div>
      <div className={styles.body}>{children}</div>
    </div>
  )
}
