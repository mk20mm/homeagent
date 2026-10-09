import { useEffect, useMemo, useState } from 'react'
import { api } from '../../api/client'
import { generateExecutionTrajectory, buildTrajectoryFromRun, type RunDetail } from './traceGenerator'
import { InfiniteCanvas } from '../InfiniteCanvas'
import styles from './ThoughtProcess.module.css'

export interface ThoughtProcessProps {
  prompt?: string
  toolName?: string
  card?: Record<string, unknown>
  runId?: string
  duration?: string
  isStreaming?: boolean
  status?: 'streaming' | 'tool_running' | 'idle' | 'error'
  traces?: string[]
  defaultOpen?: boolean
  onSteer?: (instruction: string) => void
  onCancel?: () => void
}

export function ThoughtProcess({
  prompt = '',
  toolName,
  card,
  runId,
  duration,
  isStreaming = false,
  status = 'idle',
  traces,
  defaultOpen = false,
  onSteer,
  onCancel,
}: ThoughtProcessProps) {
  const [isOpen, setIsOpen] = useState(defaultOpen || isStreaming)
  const [isCanvasOpen, setIsCanvasOpen] = useState(false)
  const [realRunDetail, setRealRunDetail] = useState<RunDetail | null>(null)

  // 若关联真实 runId，拉取持久化 AgentRun 记录
  useEffect(() => {
    if (!runId) return

    let cancelled = false
    const fetchRun = async () => {
      try {
        const res = await api.GET('/runs/{runId}', {
          params: { path: { runId } },
        })
        if (res.data && !cancelled) {
          setRealRunDetail(res.data)
        }
      } catch {
        // 接口不可用或降级时沿用启发式生成
      }
    }

    void fetchRun()

    if (isStreaming || isCanvasOpen) {
      const timer = setInterval(() => {
        void fetchRun()
      }, 1200)
      return () => {
        cancelled = true
        clearInterval(timer)
      }
    }

    return () => {
      cancelled = true
    }
  }, [runId, isStreaming, isCanvasOpen])

  const trajectory = useMemo(() => {
    if (realRunDetail) {
      return buildTrajectoryFromRun(realRunDetail, isStreaming)
    }
    const base = generateExecutionTrajectory(prompt, toolName, card, isStreaming, status)
    if (traces && traces.length > 0) {
      return { ...base, traces, duration: duration || base.duration }
    }
    return { ...base, duration: duration || base.duration }
  }, [realRunDetail, prompt, toolName, card, isStreaming, status, traces, duration])

  return (
    <>
      <div className={`${styles.doubaoContainer} ${isOpen ? styles.open : ''}`}>
        {/* 顶部耗时与操作栏（对齐豆包“已处理 X 秒 + 虚线分隔”） */}
        <div className={styles.doubaoHeader}>
          <div className={styles.durationBox} onClick={() => setIsOpen((prev) => !prev)}>
            <span className={styles.durationText}>
              {isStreaming ? '正在思考与协同调度…' : `已处理 ${trajectory.duration}`}
            </span>
            {isStreaming && <span className={styles.livePulseDot} />}
          </div>

          <div className={styles.topRightActions}>
            <button
              type="button"
              className={styles.canvasBtn}
              onClick={(e) => {
                e.stopPropagation()
                setIsCanvasOpen(true)
              }}
              title="打开全屏无线画布，查看完整 DAG 与节点"
            >
              <span>🌐</span>
              <span>无线画布</span>
            </button>

            <button
              type="button"
              className={styles.toggleCollapseBtn}
              onClick={() => setIsOpen((prev) => !prev)}
            >
              <span>{isOpen ? '收起' : '展开过程'}</span>
              <span className={`${styles.chevron} ${isOpen ? styles.chevronOpen : ''}`}>▾</span>
            </button>
          </div>
        </div>

        <div className={styles.dashedDivider} />

        {/* 展开的交错思维流与动作小药丸（对齐豆包“思维断句 + 📖/🌐 动作小徽章”） */}
        {isOpen && (
          <div className={styles.doubaoBody}>
            {trajectory.traces.map((trace, idx) => {
              // 匹配包含 Emoji 的动作小胶囊行
              const isActionPill = /^[\u{1F300}-\u{1F9FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}]/u.test(trace)
              if (isActionPill) {
                return (
                  <div key={idx} className={styles.actionPillRow}>
                    <div className={styles.actionPill}>
                      <span className={styles.actionPillText}>{trace}</span>
                    </div>
                  </div>
                )
              }
              return (
                <div key={idx} className={styles.thoughtSentence}>
                  {trace}
                </div>
              )
            })}

            {isStreaming && (
              <div className={styles.streamingIndicator}>
                <span className={styles.streamingDot} />
                <span>正在执行与推演下一步…</span>
              </div>
            )}
          </div>
        )}
      </div>

      {/* 弹出式全屏无线画布 */}
      <InfiniteCanvas
        isOpen={isCanvasOpen}
        onClose={() => setIsCanvasOpen(false)}
        trajectory={trajectory}
        runId={runId}
        isStreaming={isStreaming}
        onSteer={onSteer}
        onCancel={onCancel}
      />
    </>
  )
}
