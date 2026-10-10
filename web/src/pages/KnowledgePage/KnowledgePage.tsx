import React, { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router'
import { api, unwrap } from '../../api/client'
import type { components } from '../../api/schema'
import styles from './KnowledgePage.module.css'

type SpaceItem = components['schemas']['KnowledgeSpaceItem']
type DocumentItem = components['schemas']['KnowledgeDocumentItem']
type DocumentDetail = components['schemas']['KnowledgeDocumentDetail']

const CATEGORIES = [
  { id: 'all', label: '全部', icon: '📂' },
  { id: 'manual', label: '家电说明书', icon: '🔌' },
  { id: 'insurance', label: '保单', icon: '🛡️' },
  { id: 'contract', label: '合同', icon: '📜' },
  { id: 'health', label: '健康', icon: '🩺' },
  { id: 'memo', label: '备忘', icon: '📝' },
  { id: 'recipe', label: '菜谱', icon: '🍳' },
] as const

export function KnowledgePage() {
  const navigate = useNavigate()
  const [spaces, setSpaces] = useState<SpaceItem[]>([])
  const [currentSpace, setCurrentSpace] = useState<'family' | 'private'>('family')
  const [currentCategory, setCurrentCategory] = useState<string>('all')
  const [searchKeyword, setSearchKeyword] = useState<string>('')
  const [debouncedKeyword, setDebouncedKeyword] = useState<string>('')
  const [documents, setDocuments] = useState<DocumentItem[]>([])
  const [isLoading, setIsLoading] = useState<boolean>(true)

  // 弹窗状态
  const [createModalOpen, setCreateModalOpen] = useState<boolean>(false)
  const [detailModalDoc, setDetailModalDoc] = useState<DocumentDetail | null>(null)

  // 新建表单
  const [formTitle, setFormTitle] = useState('')
  const [formCategory, setFormCategory] = useState('manual')
  const [formVisibility, setFormVisibility] = useState<'family' | 'private'>('family')
  const [formContent, setFormContent] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [toastMessage, setToastMessage] = useState<string | null>(null)

  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const showToast = (msg: string) => {
    setToastMessage(msg)
    setTimeout(() => setToastMessage(null), 3000)
  }

  // 搜索防抖 300ms
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => {
      setDebouncedKeyword(searchKeyword.trim())
    }, 300)
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current)
    }
  }, [searchKeyword])

  // 加载空间概览
  const loadSpaces = useCallback(async () => {
    try {
      const res = unwrap(await api.GET('/knowledge/spaces'))
      setSpaces(res.spaces)
    } catch {
      // 容错兜底
    }
  }, [])

  // 加载文档列表
  const loadDocuments = useCallback(async () => {
    setIsLoading(true)
    try {
      const res = unwrap(
        await api.GET('/knowledge/documents', {
          params: {
            query: {
              visibility: currentSpace,
              category: currentCategory === 'all' ? undefined : (currentCategory as any),
              keyword: debouncedKeyword || undefined,
            },
          },
        }),
      )
      setDocuments(res.documents)
    } catch {
      setDocuments([])
    } finally {
      setIsLoading(false)
    }
  }, [currentSpace, currentCategory, debouncedKeyword])

  useEffect(() => {
    void loadSpaces()
  }, [loadSpaces])

  useEffect(() => {
    void loadDocuments()
  }, [loadDocuments])

  // 打开文档详情
  const handleOpenDetail = async (id: string) => {
    try {
      const res = unwrap(
        await api.GET('/knowledge/documents/{id}', {
          params: { path: { id } },
        }),
      )
      setDetailModalDoc(res)
    } catch {
      showToast('获取文档详情失败')
    }
  }

  // 提交新建
  const handleCreateSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!formTitle.trim() || !formContent.trim()) {
      showToast('请完整填写标题与正文内容')
      return
    }

    setIsSubmitting(true)
    try {
      await api.POST('/knowledge/documents', {
        body: {
          title: formTitle.trim(),
          category: formCategory as any,
          visibility: formVisibility,
          content: formContent.trim(),
        },
      })
      showToast(`已创建《${formTitle.trim()}》并完成切片索引`)
      setCreateModalOpen(false)
      setFormTitle('')
      setFormContent('')
      void loadSpaces()
      void loadDocuments()
    } catch {
      showToast('创建知识文档失败，请检查输入')
    } finally {
      setIsSubmitting(false)
    }
  }

  // 删除文档
  const handleDeleteDoc = async (id: string, title: string, e: React.MouseEvent) => {
    e.stopPropagation()
    if (!window.confirm(`确定要删除文档《${title}》吗？支持 24 小时内撤销恢复。`)) return

    try {
      await api.DELETE('/knowledge/documents/{id}', {
        params: { path: { id } },
      })
      showToast(`已删除《${title}》（可在会话中撤销恢复）`)
      void loadSpaces()
      void loadDocuments()
      if (detailModalDoc?.id === id) {
        setDetailModalDoc(null)
      }
    } catch {
      showToast('删除失败')
    }
  }

  const familySpace = spaces.find((s) => s.visibility === 'family')
  const privateSpace = spaces.find((s) => s.visibility === 'private')
  const totalDocs = (familySpace?.document_count ?? 0) + (privateSpace?.document_count ?? 0)

  return (
    <div className={styles.container}>
      {/* 顶部导航栏 */}
      <header className={styles.header}>
        <div className={styles.headerLeft}>
          <button
            type="button"
            className={styles.backBtn}
            onClick={() => navigate('/')}
            title="返回对话"
          >
            ←
          </button>
          <div className={styles.titleGroup}>
            <h1 className={styles.title}>家庭知识库</h1>
            <p className={styles.subtitle}>
              家电说明书 · 保单合同 · 健康就医 · 生活备忘
            </p>
          </div>
        </div>

        <button
          type="button"
          className={styles.createBtn}
          onClick={() => {
            setFormVisibility(currentSpace)
            setCreateModalOpen(true)
          }}
        >
          <span>+</span>
          <span>新建</span>
        </button>
      </header>

      <div className={styles.content}>
        {/* iOS 原生毛玻璃分段控制器 */}
        <div className={styles.spaceTabs}>
          <button
            type="button"
            className={`${styles.spaceTab} ${currentSpace === 'family' ? styles.spaceTabActive : ''}`}
            onClick={() => setCurrentSpace('family')}
          >
            <span className={styles.spaceIcon}>🌐</span>
            <span className={styles.spaceName}>家庭共享</span>
            <span className={styles.spaceCount}>
              {familySpace ? familySpace.document_count : 0}
            </span>
          </button>

          <button
            type="button"
            className={`${styles.spaceTab} ${currentSpace === 'private' ? styles.spaceTabActivePrivate : ''}`}
            onClick={() => setCurrentSpace('private')}
          >
            <span className={styles.spaceIcon}>🔒</span>
            <span className={styles.spaceName}>个人私密</span>
            <span className={styles.spaceCount}>
              {privateSpace ? privateSpace.document_count : 0}
            </span>
          </button>
        </div>

        {/* 搜索与分类筛选 */}
        <div className={styles.filterSection}>
          <div className={styles.searchBox}>
            <span className={styles.searchIcon}>🔍</span>
            <input
              type="text"
              className={styles.searchInput}
              placeholder={`搜索${currentSpace === 'family' ? '家庭共享' : '个人私密'}文档…`}
              value={searchKeyword}
              onChange={(e) => setSearchKeyword(e.target.value)}
            />
            {searchKeyword && (
              <button
                type="button"
                className={styles.clearSearchBtn}
                onClick={() => setSearchKeyword('')}
              >
                ×
              </button>
            )}
          </div>

          <div className={styles.categoryChips}>
            {CATEGORIES.map((cat) => (
              <button
                key={cat.id}
                type="button"
                className={`${styles.categoryChip} ${currentCategory === cat.id ? styles.categoryChipActive : ''}`}
                onClick={() => setCurrentCategory(cat.id)}
              >
                <span>{cat.icon}</span>
                <span>{cat.label}</span>
              </button>
            ))}
          </div>
        </div>

        {/* 文档列表 */}
        <main className={styles.docList}>
          {isLoading ? (
            <div className={styles.loadingBox}>
              <span className={styles.spinner} />
              <span>正在加载知识库…</span>
            </div>
          ) : documents.length === 0 ? (
            <div className={styles.emptyState}>
              <div className={styles.emptyIcon}>
                {debouncedKeyword ? '🔍' : '📚'}
              </div>
              <h3 className={styles.emptyTitle}>
                {debouncedKeyword ? '未找到相关文档' : '暂无文档'}
              </h3>
              <p className={styles.emptyDesc}>
                {debouncedKeyword
                  ? '试试换个关键词，或在对话中让 AI 帮你记录'
                  : currentSpace === 'family'
                    ? '点击右上角「新建」收录第一份家电说明书或保单'
                    : '个人私密空间仅本人可见，适合记录体检报告或私密备忘'}
              </p>
              {!debouncedKeyword && (
                <button
                  type="button"
                  className={styles.emptyCreateBtn}
                  onClick={() => {
                    setFormVisibility(currentSpace)
                    setCreateModalOpen(true)
                  }}
                >
                  立刻新建
                </button>
              )}
            </div>
          ) : (
            documents.map((doc) => {
              const cat = CATEGORIES.find((c) => c.id === doc.category) || {
                id: 'memo',
                label: doc.category,
                icon: '📄',
              }
              return (
                <div
                  key={doc.id}
                  className={styles.docCard}
                  onClick={() => void handleOpenDetail(doc.id)}
                >
                  {/* 左侧分类图标 */}
                  <div
                    className={styles.categoryIconCircle}
                    data-category={doc.category}
                  >
                    {cat.icon}
                  </div>

                  {/* 中部信息 */}
                  <div className={styles.docInfo}>
                    <h3 className={styles.docTitle} title={doc.title}>
                      {doc.title}
                    </h3>
                    <div className={styles.docMeta}>
                      <span>{doc.author_name}</span>
                      <span>·</span>
                      <span>{doc.chunk_count} 个切片</span>
                      <span>·</span>
                      <span>{new Date(doc.updated_at).toLocaleDateString()}</span>
                    </div>
                  </div>

                  {/* 右侧徽章与操作 */}
                  <div className={styles.docActions}>
                    <span
                      className={
                        doc.visibility === 'private'
                          ? styles.badgePrivate
                          : styles.badgeFamily
                      }
                    >
                      {doc.visibility === 'private' ? '🔒 私密' : '🌐 共享'}
                    </span>
                    <button
                      type="button"
                      className={styles.moreBtn}
                      onClick={(e) => void handleDeleteDoc(doc.id, doc.title, e)}
                      title="删除文档"
                    >
                      ···
                    </button>
                  </div>
                </div>
              )
            })
          )}
        </main>

        {/* 底部记忆同步信息条 */}
        {!isLoading && totalDocs > 0 && (
          <div className={styles.bottomInfo}>
            <span>📚 已同步 {totalDocs} 篇家庭专属记忆文档 · AI 问答将实时检索</span>
            <span>🔐 端到端本地安全加密</span>
          </div>
        )}
      </div>

      {/* 新建文档弹窗（iOS 底部 Sheet） */}
      {createModalOpen && (
        <div className={styles.modalOverlay} onClick={() => setCreateModalOpen(false)}>
          <div className={styles.modalContent} onClick={(e) => e.stopPropagation()}>
            <div className={styles.sheetHandle} />

            <div className={styles.modalHeader}>
              <h2 className={styles.modalTitle}>新建知识库文档</h2>
              <button
                type="button"
                className={styles.modalClose}
                onClick={() => setCreateModalOpen(false)}
              >
                ×
              </button>
            </div>

            <form onSubmit={(e) => void handleCreateSubmit(e)} className={styles.form}>
              <div className={styles.formGroup}>
                <label className={styles.label}>文档标题</label>
                <input
                  type="text"
                  className={styles.input}
                  placeholder="如：美的净水器RO-800G使用说明书"
                  value={formTitle}
                  onChange={(e) => setFormTitle(e.target.value)}
                  required
                />
              </div>

              <div className={styles.formRow}>
                <div className={styles.formGroup}>
                  <label className={styles.label}>所属空间</label>
                  <select
                    className={styles.select}
                    value={formVisibility}
                    onChange={(e) => setFormVisibility(e.target.value as any)}
                  >
                    <option value="family">🌐 家庭共享</option>
                    <option value="private">🔒 个人私密</option>
                  </select>
                </div>

                <div className={styles.formGroup}>
                  <label className={styles.label}>分类</label>
                  <select
                    className={styles.select}
                    value={formCategory}
                    onChange={(e) => setFormCategory(e.target.value)}
                  >
                    <option value="manual">🔌 家电说明书</option>
                    <option value="insurance">🛡️ 家庭保单</option>
                    <option value="contract">📜 合同档案</option>
                    <option value="health">🩺 健康就医</option>
                    <option value="memo">📝 生活备忘</option>
                    <option value="recipe">🍳 私房菜谱</option>
                  </select>
                </div>
              </div>

              <div className={styles.formGroup}>
                <label className={styles.label}>
                  正文内容（将自动进行滑动窗口切片索引）
                </label>
                <textarea
                  className={styles.textarea}
                  rows={6}
                  placeholder="粘贴说明书文本、保单条款或备忘录正文…"
                  value={formContent}
                  onChange={(e) => setFormContent(e.target.value)}
                  required
                />
              </div>

              <div className={styles.modalActions}>
                <button
                  type="button"
                  className={styles.cancelBtn}
                  onClick={() => setCreateModalOpen(false)}
                >
                  取消
                </button>
                <button type="submit" className={styles.submitBtn} disabled={isSubmitting}>
                  {isSubmitting ? '正在切片建库…' : '创建并索引'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* 查看详情抽屉（底部 Sheet） */}
      {detailModalDoc && (
        <div className={styles.modalOverlay} onClick={() => setDetailModalDoc(null)}>
          <div className={styles.drawerContent} onClick={(e) => e.stopPropagation()}>
            <div className={styles.sheetHandle} />

            <div className={styles.modalHeader}>
              <div className={styles.drawerTitleBox}>
                <div className={styles.badgesRow}>
                  <span
                    className={
                      detailModalDoc.visibility === 'private'
                        ? styles.badgePrivate
                        : styles.badgeFamily
                    }
                  >
                    {detailModalDoc.visibility === 'private'
                      ? '🔒 个人私密空间'
                      : '🌐 家庭共享空间'}
                  </span>
                  <span className={styles.badgeCategory}>
                    {detailModalDoc.category}
                  </span>
                </div>
                <h2 className={styles.modalTitle}>{detailModalDoc.title}</h2>
                <div className={styles.docMeta}>
                  <span>上传者: {detailModalDoc.author_name}</span>
                  <span>·</span>
                  <span>切片数: {detailModalDoc.chunk_count}</span>
                  <span>·</span>
                  <span>
                    更新: {new Date(detailModalDoc.updated_at).toLocaleString()}
                  </span>
                </div>
              </div>

              <button
                type="button"
                className={styles.modalClose}
                onClick={() => setDetailModalDoc(null)}
              >
                ×
              </button>
            </div>

            <div className={styles.docBody}>
              <div className={styles.bodyScroll}>
                <pre className={styles.contentPre}>{detailModalDoc.content}</pre>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* 全局轻提示 Toast */}
      {toastMessage && <div className={styles.toast}>{toastMessage}</div>}
    </div>
  )
}
