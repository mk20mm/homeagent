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

  // 若关联真实 runId，拉取持久化 AgentRun 记录（包含真实步骤、工具参数与事件日志）
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

    // 若当前正在流式交互或无限画布正处于打开状态，以 1.2 秒节流轮询最新步骤与事件
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

  // 动态生成符合当前具体 prompt 与工具执行的真实轨迹与拓扑（优先使用后端持久化真数据）
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
      <div className={`${styles.container} ${isOpen ? styles.open : ''}`}>
        <div
          className={styles.header}
          onClick={() => setIsOpen((prev) => !prev)}
          role="button"
          tabIndex={0}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              setIsOpen((prev) => !prev)
            }
          }}
          aria-expanded={isOpen}
        >
          <div className={styles.headerLeft}>
            <span className={`${styles.boltIcon} ${isStreaming ? styles.pulsing : ''}`}>⚡</span>
            <span className={styles.title}>
              {isStreaming ? '正在多维推理与调度分析…' : `深度思考过程 · 耗时 ${trajectory.duration}`}
            </span>
          </div>

          <div className={styles.headerRight}>
            <button
              type="button"
              className={styles.canvasBtn}
              onClick={(e) => {
                e.stopPropagation()
                setIsCanvasOpen(true)
              }}
              title="打开全屏无限画布，查看执行 DAG 与节点"
            >
              <span>🌐</span>
              <span>无线画布</span>
            </button>
            <span className={`${styles.chevron} ${isOpen ? styles.chevronOpen : ''}`}>▾</span>
          </div>
        </div>

        {isOpen && (
          <div className={styles.body}>
            {trajectory.traces.map((trace, idx) => (
              <div key={idx} className={styles.traceRow}>
                <span className={styles.traceDot}>◆</span>
                <span className={styles.traceText}>{trace}</span>
              </div>
            ))}
            {isStreaming && (
              <div className={styles.liveIndicator}>
                <span className={styles.liveDot} />
                <span>协同调度与意图推演中…</span>
              </div>
            )}

            <button
              type="button"
              className={styles.openCanvasBanner}
              onClick={() => setIsCanvasOpen(true)}
            >
              <span style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                <span className={styles.canvasIcon}>🌐</span>
                <span>展开至无线画布拓扑视图 · 自由缩放平移探索 Agent 决策流</span>
              </span>
              <span className={styles.canvasArrow}>➔</span>
            </button>
          </div>
        )}
      </div>

      {/* 弹出式全屏无限画布 */}
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
