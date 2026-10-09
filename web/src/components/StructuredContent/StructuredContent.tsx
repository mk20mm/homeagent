import React from 'react'
import styles from './StructuredContent.module.css'

export interface StructuredContentProps {
  content: string
  onQuickAction?: (actionText: string) => void
}

interface StepItem {
  number: number
  title: string
  detail: string
  actionHint?: string
}

/**
 * 结构化消息排版组件（对齐豆包等新一代 AI 对话结构化呈现）：
 * 1. 自动解析步骤清单 (1. 2. 3.) -> 渲染为带数字徽章的操作卡片
 * 2. 自动解析结论建议 -> 渲染为浅色 Callout 重点提示盒
 * 3. 自动解析末尾提问 -> 渲染为一键快捷动作胶囊 (Quick Action Chips)
 * 4. 内联 Markdown 解析（加粗、行内代码、箭头导向）
 */
export const StructuredContent: React.FC<StructuredContentProps> = ({ content, onQuickAction }) => {
  if (!content) return null

  // 1. 提取末尾快捷建议提议（例如：“需要我帮你建个维修/清洁待办任务吗？”）
  let bodyText = content
  const actionChips: string[] = []

  if (bodyText.includes('待办') || bodyText.includes('任务') || bodyText.includes('建个')) {
    if (bodyText.includes('洗碗机') || bodyText.includes('维修') || bodyText.includes('清洁')) {
      actionChips.push('帮我建个洗碗机排查待办')
      actionChips.push('记一笔家电维修预算')
    }
  }
  if (bodyText.includes('记账') || bodyText.includes('支出') || bodyText.includes('买菜')) {
    if (!actionChips.includes('查看本月家庭支出')) {
      actionChips.push('查看本月家庭支出')
    }
  }

  // 2. 将文本按双换行切分成段落块
  const blocks = bodyText.split(/\n{2,}/).map((b) => b.trim()).filter(Boolean)

  const renderInline = (text: string) => {
    // 简易安全内联 Markdown 渲染器：支持 **加粗** 和 `代码`
    const parts = text.split(/(\*\*[^*]+\*\*|`[^`]+`)/g)
    return parts.map((part, i) => {
      if (part.startsWith('**') && part.endsWith('**')) {
        return <strong key={i} className={styles.strongText}>{part.slice(2, -2)}</strong>
      }
      if (part.startsWith('`') && part.endsWith('`')) {
        return <code key={i} className={styles.inlineCode}>{part.slice(1, -1)}</code>
      }
      return part
    })
  }

  // 3. 检查某一段是否为数字列表步骤（例如: 1. **进水问题**：... 2. **排水堵塞**：...）
  const parseSteps = (block: string): StepItem[] | null => {
    const lines = block.split('\n').map((l) => l.trim()).filter(Boolean)
    const stepRegex = /^(\d+)[.、]\s*(.*)$/
    const items: StepItem[] = []
    let currentStep: StepItem | null = null

    for (const line of lines) {
      const match = stepRegex.exec(line)
      if (match) {
        if (currentStep) items.push(currentStep)
        const num = parseInt(match[1], 10)
        let rawContent = match[2]
        let title = ''
        let detail = rawContent

        // 提取加粗标题或冒号标题（例如 **进水问题**：水龙头没开）
        const boldTitleMatch = /^\*\*([^*]+)\*\*[:：]?\s*(.*)$/.exec(rawContent)
        if (boldTitleMatch) {
          title = boldTitleMatch[1]
          detail = boldTitleMatch[2]
        } else if (rawContent.includes('：') || rawContent.includes(':')) {
          const colonIdx = rawContent.indexOf('：') !== -1 ? rawContent.indexOf('：') : rawContent.indexOf(':')
          title = rawContent.slice(0, colonIdx)
          detail = rawContent.slice(colonIdx + 1)
        }

        currentStep = {
          number: num,
          title,
          detail,
        }
      } else if (currentStep) {
        // 次级行动指导（如 → 先把洗碗机下面的水龙头拧开）
        if (line.startsWith('→') || line.startsWith('->') || line.startsWith('-')) {
          currentStep.actionHint = line.replace(/^[→\-]+>\s*|^[→\-]\s*/, '')
        } else {
          currentStep.detail += ' ' + line
        }
      }
    }
    if (currentStep) items.push(currentStep)

    return items.length >= 2 ? items : null
  }

  return (
    <div className={styles.structuredContainer}>
      {blocks.map((block, idx) => {
        // 步骤清单块
        const steps = parseSteps(block)
        if (steps) {
          return (
            <div key={idx} className={styles.stepListCard}>
              <div className={styles.stepListHeader}>
                <span className={styles.stepListIcon}>📋</span>
                <span className={styles.stepListTitle}>建议排查指引与步骤</span>
              </div>
              <div className={styles.stepsWrap}>
                {steps.map((st) => (
                  <div key={st.number} className={styles.stepItemRow}>
                    <div className={styles.stepNumBadge}>{st.number}</div>
                    <div className={styles.stepContentBox}>
                      {st.title && (
                        <div className={styles.stepItemTitle}>{renderInline(st.title)}</div>
                      )}
                      <div className={styles.stepItemDetail}>{renderInline(st.detail)}</div>
                      {st.actionHint && (
                        <div className={styles.stepActionHint}>
                          <span className={styles.actionArrow}>↳ 操作：</span>
                          <span>{renderInline(st.actionHint)}</span>
                        </div>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )
        }

        // 核心结论/重点提醒块（例如“建议按 1→2→3 顺序试...如果仍亮红灯需报修”）
        if (
          block.startsWith('建议') ||
          block.startsWith('注意') ||
          block.startsWith('⚠️') ||
          block.startsWith('💡') ||
          block.startsWith('总结') ||
          block.startsWith('>')
        ) {
          const cleanText = block.replace(/^[>⚠️💡]\s*/, '')
          return (
            <div key={idx} className={styles.calloutCard}>
              <span className={styles.calloutIcon}>💡</span>
              <div className={styles.calloutBody}>
                <span className={styles.calloutLabel}>核心建议：</span>
                <span>{renderInline(cleanText)}</span>
              </div>
            </div>
          )
        }

        // 提问收尾块（如：“需要我帮你建个待办吗？”）
        if (block.endsWith('吗？') || block.endsWith('吗?') || block.includes('需要我帮你')) {
          return (
            <div key={idx} className={styles.promptClosingRow}>
              <span className={styles.promptText}>{renderInline(block)}</span>
            </div>
          )
        }

        // 普通正文段落
        return (
          <p key={idx} className={styles.paragraph}>
            {renderInline(block)}
          </p>
        )
      })}

      {/* 底部一键闭环行动胶囊（快捷操作） */}
      {actionChips.length > 0 && (
        <div className={styles.chipsRow}>
          {actionChips.map((chip, cIdx) => (
            <button
              key={cIdx}
              type="button"
              className={styles.actionChip}
              onClick={() => onQuickAction?.(chip)}
              title={`点击发送: ${chip}`}
            >
              <span className={styles.chipPlus}>+</span>
              <span>{chip}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
