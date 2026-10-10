import React, { useEffect, useState } from 'react'
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
  { id: 'insurance', label: '家庭保单', icon: '🛡️' },
  { id: 'contract', label: '合同档案', icon: '📜' },
  { id: 'health', label: '健康档案', icon: '🩺' },
  { id: 'memo', label: '生活备忘', icon: '📝' },
  { id: 'recipe', label: '私房菜谱', icon: '🍳' },
] as const

export function KnowledgePage() {
  const navigate = useNavigate()
  const [spaces, setSpaces] = useState<SpaceItem[]>([])
  const [currentSpace, setCurrentSpace] = useState<'family' | 'private'>('family')
  const [currentCategory, setCurrentCategory] = useState<string>('all')
  const [searchKeyword, setSearchKeyword] = useState<string>('')
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

  const showToast = (msg: string) => {
    setToastMessage(msg)
    setTimeout(() => setToastMessage(null), 3000)
  }

  // 加载空间概览
  const loadSpaces = async () => {
    try {
      const res = unwrap(await api.GET('/knowledge/spaces'))
      setSpaces(res.spaces)
    } catch {
      // 容错兜底
    }
  }

  // 加载文档列表
  const loadDocuments = async () => {
    setIsLoading(true)
    try {
      const res = unwrap(
        await api.GET('/knowledge/documents', {
          params: {
            query: {
              visibility: currentSpace,
              category: currentCategory === 'all' ? undefined : (currentCategory as any),
              keyword: searchKeyword.trim() || undefined,
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
  }

  useEffect(() => {
    void loadSpaces()
  }, [])

  useEffect(() => {
    void loadDocuments()
  }, [currentSpace, currentCategory, searchKeyword])

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

  return (
    <div className={styles.container}>
      {/* 顶部导航 */}
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
          <div>
            <h1 className={styles.title}>家庭知识库</h1>
            <p className={styles.subtitle}>
              飞书风私域 Wiki：家电说明书、保单合同、健康就医与生活备忘
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
          <span>➕</span>
          <span>新建文档</span>
        </button>
      </header>

      {/* 飞书风双空间切换栏 */}
      <div className={styles.spaceTabs}>
        <button
          type="button"
          className={`${styles.spaceTab} ${currentSpace === 'family' ? styles.spaceTabActive : ''}`}
          onClick={() => setCurrentSpace('family')}
        >
          <span className={styles.spaceIcon}>🌐</span>
          <span className={styles.spaceName}>家庭共享空间</span>
          <span className={styles.spaceCount}>
            {familySpace ? familySpace.document_count : 0} 篇
          </span>
        </button>

        <button
          type="button"
          className={`${styles.spaceTab} ${currentSpace === 'private' ? styles.spaceTabActivePrivate : ''}`}
          onClick={() => setCurrentSpace('private')}
        >
          <span className={styles.spaceIcon}>🔒</span>
          <span className={styles.spaceName}>个人私密空间</span>
          <span className={styles.spaceCount}>
            {privateSpace ? privateSpace.document_count : 0} 篇
          </span>
        </button>
      </div>

      {/* 搜索与分类筛选栏 */}
      <div className={styles.filterSection}>
        <div className={styles.searchBox}>
          <span className={styles.searchIcon}>🔍</span>
          <input
            type="text"
            className={styles.searchInput}
            placeholder={`搜索${currentSpace === 'family' ? '家庭共享' : '个人私密'}文档、说明书或正文关键词…`}
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

      {/* 文档列表区 */}
      <main className={styles.docGrid}>
        {isLoading ? (
          <div className={styles.loadingBox}>
            <span className={styles.spinner} />
            <span>正在加载知识库文档…</span>
          </div>
        ) : documents.length === 0 ? (
          <div className={styles.emptyState}>
            <div className={styles.emptyIcon}>📚</div>
            <h3 className={styles.emptyTitle}>暂无文档</h3>
            <p className={styles.emptyDesc}>
              {currentSpace === 'family'
                ? '暂无家庭共享文档，点击右上角【新建文档】收录第一份家电说明书或保单'
                : '个人私密空间为空，仅您本人可见，适合记录个人体检、私房账单或私密备忘'}
            </p>
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
          </div>
        ) : (
          documents.map((doc) => {
            const cat = CATEGORIES.find((c) => c.id === doc.category) || {
              label: doc.category,
              icon: '📄',
            }
            return (
              <div
                key={doc.id}
                className={`${styles.docCard} ${doc.visibility === 'private' ? styles.docCardPrivate : styles.docCardFamily}`}
                onClick={() => void handleOpenDetail(doc.id)}
              >
                <div className={styles.docCardHeader}>
                  <div className={styles.badgesRow}>
                    <span
                      className={
                        doc.visibility === 'private'
                          ? styles.badgePrivate
                          : styles.badgeFamily
                      }
                    >
                      {doc.visibility === 'private' ? '🔒 个人私密' : '🌐 家庭共享'}
                    </span>
                    <span className={styles.badgeCategory}>
                      {cat.icon} {cat.label}
                    </span>
                  </div>

                  <button
                    type="button"
                    className={styles.deleteBtn}
                    onClick={(e) => void handleDeleteDoc(doc.id, doc.title, e)}
                    title="删除文档"
                  >
                    🗑️
                  </button>
                </div>

                <h3 className={styles.docTitle} title={doc.title}>
                  {doc.title}
                </h3>

                <div className={styles.docMeta}>
                  <span>上传者: {doc.author_name}</span>
                  <span>·</span>
                  <span>{doc.chunk_count} 个切片</span>
                  <span>·</span>
                  <span>{new Date(doc.updated_at).toLocaleDateString()}</span>
                </div>
              </div>
            )
          })
        )}
      </main>

      {/* 新建文档弹窗 */}
      {createModalOpen && (
        <div className={styles.modalOverlay} onClick={() => setCreateModalOpen(false)}>
          <div className={styles.modalContent} onClick={(e) => e.stopPropagation()}>
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
                  placeholder="如：美的净水器RO-800G使用说明书 / 个人体检档案"
                  value={formTitle}
                  onChange={(e) => setFormTitle(e.target.value)}
                  required
                />
              </div>

              <div className={styles.formRow}>
                <div className={styles.formGroup}>
                  <label className={styles.label}>所属空间（可见范围）</label>
                  <select
                    className={styles.select}
                    value={formVisibility}
                    onChange={(e) => setFormVisibility(e.target.value as any)}
                  >
                    <option value="family">🌐 家庭共享空间（全家可见）</option>
                    <option value="private">🔒 个人私密空间（仅本人可见）</option>
                  </select>
                </div>

                <div className={styles.formGroup}>
                  <label className={styles.label}>分类属性</label>
                  <select
                    className={styles.select}
                    value={formCategory}
                    onChange={(e) => setFormCategory(e.target.value)}
                  >
                    <option value="manual">🔌 家电设备与说明书</option>
                    <option value="insurance">🛡️ 家庭保单</option>
                    <option value="contract">📜 重要合同</option>
                    <option value="health">🩺 健康与就医备忘</option>
                    <option value="memo">📝 生活资产与备忘</option>
                    <option value="recipe">🍳 私房菜谱与生活指南</option>
                  </select>
                </div>
              </div>

              <div className={styles.formGroup}>
                <label className={styles.label}>
                  正文内容（支持 Markdown / 纯文本，将自动进行滑动窗口切片）
                </label>
                <textarea
                  className={styles.textarea}
                  rows={8}
                  placeholder="在此粘贴说明书文本、滤芯型号、密码、保单条款或备忘录正文…"
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
                  {isSubmitting ? '正在切片建库…' : '创建并切片索引'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* 查看详情抽屉 */}
      {detailModalDoc && (
        <div className={styles.modalOverlay} onClick={() => setDetailModalDoc(null)}>
          <div className={styles.drawerContent} onClick={(e) => e.stopPropagation()}>
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
                    更新时间: {new Date(detailModalDoc.updated_at).toLocaleString()}
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
