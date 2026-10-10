import React, { useState } from 'react'
import { useNavigate } from 'react-router'
import styles from './KnowledgeCitationCard.module.css'

export interface KnowledgeCitationItem {
  document_id: string
  document_title: string
  category: string
  visibility: string
  chunk_index: number
  content: string
  score: number
  is_private: boolean
}

export interface KnowledgeCitationCardProps {
  card: {
    type: 'knowledge_citation'
    query: string
    results: KnowledgeCitationItem[]
  }
}

const CATEGORY_MAP: Record<string, { label: string; icon: string }> = {
  manual: { label: '家电说明书', icon: '🔌' },
  insurance: { label: '家庭保单', icon: '🛡️' },
  contract: { label: '合同档案', icon: '📜' },
  health: { label: '健康档案', icon: '🩺' },
  memo: { label: '生活备忘', icon: '📝' },
  recipe: { label: '私房菜谱', icon: '🍳' },
}

/** 飞书风知识库引用卡片：展示家庭共享与个人私密知识信源、命中片段与匹配度 */
export const KnowledgeCitationCard: React.FC<KnowledgeCitationCardProps> = ({ card }) => {
  const navigate = useNavigate()
  const results = card.results || []
  const [expandedId, setExpandedId] = useState<string | null>(null)

  if (results.length === 0) return null

  const toggleExpand = (key: string) => {
    setExpandedId((prev) => (prev === key ? null : key))
  }

  return (
    <div className={styles.card}>
      <div className={styles.header}>
        <div className={styles.titleBox}>
          <span className={styles.bookIcon}>📚</span>
          <span className={styles.title}>家庭知识库引用</span>
          <span className={styles.countBadge}>· 命中 {results.length} 处</span>
        </div>
        <button
          type="button"
          className={styles.viewAllBtn}
          onClick={() => navigate('/knowledge')}
          title="打开家庭知识库"
        >
          <span>进入知识库</span>
          <span className={styles.arrowIcon}>→</span>
        </button>
      </div>

      <div className={styles.itemList}>
        {results.map((item, idx) => {
          const itemKey = `${item.document_id}_${item.chunk_index}_${idx}`
          const isExpanded = expandedId === itemKey
          const cat = CATEGORY_MAP[item.category] || { label: item.category, icon: '📄' }

          return (
            <div
              key={itemKey}
              className={`${styles.itemCard} ${item.is_private ? styles.itemPrivate : styles.itemFamily}`}
            >
              <div className={styles.itemHeader} onClick={() => toggleExpand(itemKey)}>
                <div className={styles.itemLeft}>
                  {item.is_private ? (
                    <span className={styles.privatePill}>🔒 个人私密</span>
                  ) : (
                    <span className={styles.familyPill}>🌐 家庭共享</span>
                  )}
                  <span className={styles.categoryPill} data-category={item.category}>
                    {cat.icon} {cat.label}
                  </span>
                  <span className={styles.docTitle} title={item.document_title}>
                    {item.document_title}
                  </span>
                </div>

                <div className={styles.itemRight}>
                  <span className={styles.scoreBadge}>
                    {Math.round(item.score * 100)}% 匹配
                  </span>
                  <span className={styles.expandIcon}>{isExpanded ? '▲' : '▼'}</span>
                </div>
              </div>

              <div
                className={`${styles.itemContent} ${isExpanded ? styles.expanded : styles.collapsed}`}
                onClick={() => toggleExpand(itemKey)}
              >
                <div className={styles.quoteBar} />
                <p className={styles.contentText}>{item.content}</p>
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
