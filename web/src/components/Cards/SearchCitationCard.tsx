import React, { useState } from 'react'
import { api, unwrap } from '../../api/client'
import styles from './SearchCitationCard.module.css'

export interface SearchCitationItem {
  title: string
  url: string
  source_name: string
  snippet: string
  published_date?: string
  is_official?: boolean
  jev_score?: number
  jev_label?: string
}

export interface SearchCitationCardProps {
  card: {
    type: 'search_citation'
    query: string
    results: SearchCitationItem[]
  }
}

/** 智能搜索权威信源卡片：支持默认轻量展示与右上角按需呼叫 TypeSafe Jev System One 决策精排 */
export const SearchCitationCard: React.FC<SearchCitationCardProps> = ({ card }) => {
  const originalResults = card.results || []
  const [items, setItems] = useState<SearchCitationItem[]>(originalResults)
  const [isJevActive, setIsJevActive] = useState<boolean>(false)
  const [isLoading, setIsLoading] = useState<boolean>(false)
  const [cachedJevResults, setCachedJevResults] = useState<SearchCitationItem[] | null>(null)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)

  if (items.length === 0) return null

  const handleToggleJev = async (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()

    // 若已经处于 Jev 精排模式，再次点击则切回初始顺序
    if (isJevActive) {
      setItems(originalResults)
      setIsJevActive(false)
      return
    }

    // 若已有缓存的 Jev 结果，直接切换展示
    if (cachedJevResults) {
      setItems(cachedJevResults)
      setIsJevActive(true)
      return
    }

    // 发起按需精排请求
    setIsLoading(true)
    setErrorMessage(null)
    try {
      const res = unwrap(
        await api.POST('/search/rerank', {
          body: {
            query: card.query,
            candidates: originalResults,
          },
        })
      )
      const ranked = res.results || []
      setCachedJevResults(ranked)
      setItems(ranked)
      setIsJevActive(true)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Jev 精排服务暂时不可用'
      setErrorMessage(msg)
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className={`${styles.card} ${isJevActive ? styles.cardJevActive : ''}`}>
      <div className={styles.header}>
        <div className={styles.titleBox}>
          <span className={styles.globeIcon}>🌐</span>
          <span className={styles.title}>权威信源参考</span>
          <span className={styles.countBadge}>({items.length} 个来源)</span>
        </div>

        <div className={styles.headerActions}>
          <button
            type="button"
            className={`${styles.jevActionBtn} ${isJevActive ? styles.jevActiveBtn : ''}`}
            onClick={handleToggleJev}
            disabled={isLoading}
            title={isJevActive ? '点击还原初始自然顺序' : '按需调用 TypeSafe Jev 深度决策模型打分重排'}
          >
            {isLoading ? (
              <>
                <span className={styles.spinner} />
                <span>Jev 分析中...</span>
              </>
            ) : isJevActive ? (
              <>
                <span className={styles.sparkleIcon}>⚡</span>
                <span>Jev 已精排 ✓</span>
              </>
            ) : (
              <>
                <span className={styles.sparkleIcon}>⚡</span>
                <span>Jev 智能精排</span>
              </>
            )}
          </button>

          <div className={styles.verifiedBadge}>
            <span>✓</span>
            <span>验证无误</span>
          </div>
        </div>
      </div>

      {isJevActive && (
        <div className={styles.jevTipBar}>
          <span className={styles.jevTipIcon}>🤖</span>
          <span className={styles.jevTipText}>
            已由 <strong>TypeSafe Jev (System One)</strong> 决策模型完成概率打分，优先置顶高针对性实操方案
          </span>
        </div>
      )}

      {errorMessage && (
        <div className={styles.errorBar}>
          <span>⚠️ {errorMessage}（已保持默认顺序）</span>
        </div>
      )}

      <div className={styles.list}>
        {items.map((item, idx) => {
          const scorePercent = typeof item.jev_score === 'number' ? Math.round(item.jev_score * 100) : null

          return (
            <a
              key={item.url || idx}
              href={item.url}
              target="_blank"
              rel="noopener noreferrer"
              className={`${styles.item} ${isJevActive ? styles.itemJevRanked : ''}`}
              title={`在浏览器中打开来源: ${item.title}`}
            >
              <div className={styles.itemHeader}>
                <div className={styles.itemTitleRow}>
                  <span className={`${styles.indexPill} ${isJevActive && idx === 0 ? styles.topIndexPill : ''}`}>
                    {idx + 1}
                  </span>
                  <span className={styles.itemTitle}>{item.title}</span>
                  {item.is_official && <span className={styles.officialTag}>官方信源</span>}

                  {isJevActive && scorePercent !== null && (
                    <span
                      className={
                        item.jev_score! >= 0.85
                          ? styles.jevHighTag
                          : item.jev_score! >= 0.5
                            ? styles.jevMidTag
                            : styles.jevLowTag
                      }
                    >
                      ⚡ Jev {scorePercent}% · {item.jev_label || '已研判'}
                    </span>
                  )}
                </div>
                <span className={styles.linkArrow}>↗</span>
              </div>

              <p className={styles.snippet}>{item.snippet}</p>

              <div className={styles.itemFooter}>
                <span className={styles.sourceDomain}>{item.source_name}</span>
                {item.published_date && <span>{item.published_date}</span>}
              </div>
            </a>
          )
        })}
      </div>
    </div>
  )
}
