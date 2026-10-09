import React from 'react'
import styles from './SearchCitationCard.module.css'

export interface SearchCitationItem {
  title: string
  url: string
  source_name: string
  snippet: string
  published_date?: string
  is_official?: boolean
}

export interface SearchCitationCardProps {
  card: {
    type: 'search_citation'
    query: string
    results: SearchCitationItem[]
  }
}

/** 智能搜索权威信源卡片：对齐 Stitch iOS 18 拟态磨砂玻璃风格与可溯源外链跳转 */
export const SearchCitationCard: React.FC<SearchCitationCardProps> = ({ card }) => {
  const results = card.results || []

  if (results.length === 0) return null

  return (
    <div className={styles.card}>
      <div className={styles.header}>
        <div className={styles.titleBox}>
          <span className={styles.globeIcon}>🌐</span>
          <span className={styles.title}>权威信源参考</span>
          <span className={styles.countBadge}>({results.length} 个来源)</span>
        </div>
        <div className={styles.verifiedBadge}>
          <span>✓</span>
          <span>验证无误</span>
        </div>
      </div>

      <div className={styles.list}>
        {results.map((item, idx) => (
          <a
            key={item.url || idx}
            href={item.url}
            target="_blank"
            rel="noopener noreferrer"
            className={styles.item}
            title={`在浏览器中打开来源: ${item.title}`}
          >
            <div className={styles.itemHeader}>
              <div className={styles.itemTitleRow}>
                <span className={styles.indexPill}>{idx + 1}</span>
                <span className={styles.itemTitle}>{item.title}</span>
                {item.is_official && <span className={styles.officialTag}>官方信源</span>}
              </div>
              <span className={styles.linkArrow}>↗</span>
            </div>

            <p className={styles.snippet}>{item.snippet}</p>

            <div className={styles.itemFooter}>
              <span className={styles.sourceDomain}>{item.source_name}</span>
              {item.published_date && <span>{item.published_date}</span>}
            </div>
          </a>
        ))}
      </div>
    </div>
  )
}
