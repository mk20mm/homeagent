import {
  useEffect,
  useRef,
  useState,
  useCallback,
  type MouseEvent,
  type TouchEvent,
  type WheelEvent,
  type FormEvent,
} from 'react'
import { api } from '../../api/client'
import type { CanvasNode, ExecutionTrajectory } from '../ThoughtProcess/traceGenerator'
import styles from './InfiniteCanvas.module.css'

export interface InfiniteCanvasProps {
  isOpen: boolean
  onClose: () => void
  trajectory: ExecutionTrajectory
  runId?: string
  isStreaming?: boolean
  onSteer?: (instruction: string) => void
  onCancel?: () => void
}

export function InfiniteCanvas({
  isOpen,
  onClose,
  trajectory,
  runId,
  isStreaming = false,
  onSteer,
  onCancel,
}: InfiniteCanvasProps) {
  const [scale, setScale] = useState(1)
  const [pan, setPan] = useState({ x: 80, y: 120 })
  const [isDragging, setIsDragging] = useState(false)
  const [dragStart, setDragStart] = useState({ x: 0, y: 0 })
  const [selectedNode, setSelectedNode] = useState<CanvasNode | null>(null)
  const [selectedTurn, setSelectedTurn] = useState<number | null>(null)
  const [steerInput, setSteerInput] = useState('')
  const viewportRef = useRef<HTMLDivElement>(null)

  // ESC 键关闭
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) {
        onClose()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isOpen, onClose])

  // 自适应居中
  const handleFitView = useCallback(() => {
    if (!viewportRef.current || trajectory.nodes.length === 0) return
    const rect = viewportRef.current.getBoundingClientRect()
    // 默认居中到第 2 个核心节点附近
    const targetNode = trajectory.nodes[1] || trajectory.nodes[0]
    const centerX = rect.width / 2 - targetNode.x - 120
    const centerY = rect.height / 2 - targetNode.y - 80
    setPan({ x: centerX, y: centerY })
    setScale(1)
  }, [trajectory.nodes])

  const handleSelectTurn = (turnNum: number | null) => {
    setSelectedTurn(turnNum)
    if (!viewportRef.current) return
    const rect = viewportRef.current.getBoundingClientRect()
    if (turnNum === null) {
      handleFitView()
      return
    }
    const targetGroup = trajectory.turnGroups?.find((g) => g.turn === turnNum)
    if (targetGroup) {
      const centerX = rect.width / 2 - targetGroup.x - targetGroup.width / 2
      const centerY = rect.height / 2 - targetGroup.y - targetGroup.height / 2
      setPan({ x: centerX, y: centerY })
      setScale(0.9)
    }
  }

  const handleStepPlay = () => {
    if (!viewportRef.current || trajectory.nodes.length === 0) return
    const rect = viewportRef.current.getBoundingClientRect()
    const currentIdx = selectedNode
      ? trajectory.nodes.findIndex((n) => n.id === selectedNode.id)
      : -1
    const nextIdx = (currentIdx + 1) % trajectory.nodes.length
    const nextNode = trajectory.nodes[nextIdx]
    setSelectedNode(nextNode)
    if (nextNode.turn) {
      setSelectedTurn(nextNode.turn)
    }
    const centerX = rect.width / 2 - nextNode.x - 120
    const centerY = rect.height / 2 - nextNode.y - 80
    setPan({ x: centerX, y: centerY })
  }

  useEffect(() => {
    if (isOpen) {
      handleFitView()
    }
  }, [isOpen, handleFitView])

  // 鼠标拖拽平移
  const handleMouseDown = (e: MouseEvent) => {
    // 避免拖拽卡片内部控件时触发平移
    if ((e.target as HTMLElement).closest(`.${styles.nodeCard}`)) return
    setIsDragging(true)
    setDragStart({ x: e.clientX - pan.x, y: e.clientY - pan.y })
  }

  const handleMouseMove = (e: MouseEvent) => {
    if (!isDragging) return
    setPan({
      x: e.clientX - dragStart.x,
      y: e.clientY - dragStart.y,
    })
  }

  const handleMouseUp = () => setIsDragging(false)

  // 滚轮缩放
  const handleWheel = (e: WheelEvent) => {
    e.preventDefault()
    const zoomFactor = e.deltaY < 0 ? 1.08 : 0.92
    setScale((prev) => Math.min(2.5, Math.max(0.45, prev * zoomFactor)))
  }

  // 触屏手势平移支持
  const handleTouchStart = (e: TouchEvent) => {
    if (e.touches.length === 1) {
      setIsDragging(true)
      const touch = e.touches[0]
      setDragStart({ x: touch.clientX - pan.x, y: touch.clientY - pan.y })
    }
  }

  const handleTouchMove = (e: TouchEvent) => {
    if (!isDragging || e.touches.length !== 1) return
    const touch = e.touches[0]
    setPan({
      x: touch.clientX - dragStart.x,
      y: touch.clientY - dragStart.y,
    })
  }

  const handleTouchEnd = () => setIsDragging(false)

  const handleSteerSubmit = (e: FormEvent) => {
    e.preventDefault()
    const instruction = steerInput.trim()
    if (!instruction) return
    if (runId) {
      void api.POST('/runs/{runId}/steer', {
        params: { path: { runId } },
        body: { instruction },
      })
    }
    if (onSteer) {
      onSteer(instruction)
    }
    setSteerInput('')
  }

  const handleCancelClick = () => {
    if (runId) {
      void api.POST('/runs/{runId}/cancel', {
        params: { path: { runId } },
        body: { reason: 'user_cancelled_from_infinite_canvas' },
      })
    }
    if (onCancel) {
      onCancel()
    }
  }

  if (!isOpen) return null

  const nodeMap = new Map<string, CanvasNode>()
  trajectory.nodes.forEach((n) => nodeMap.set(n.id, n))

  return (
    <div className={styles.overlay} role="dialog" aria-modal="true">
      {/* 顶部控制栏 */}
      <header className={styles.topBar}>
        <div className={styles.titleArea}>
          <h2>
            <span>✦</span>
            <span>AI 推演与执行轨迹 · 无限画布</span>
          </h2>
          <span className={styles.canvasPill}>Infinite Canvas DAG</span>
          {runId && (
            <span
              className={styles.canvasPill}
              style={{ background: 'rgba(59, 130, 246, 0.15)', color: '#60a5fa', borderColor: 'rgba(59, 130, 246, 0.3)' }}
              title={`调度运行事务 ID: ${runId}`}
            >
              Run: {runId.slice(0, 8)}
            </span>
          )}
          <div className={styles.statusIndicator}>
            <span className={styles.pulseDot} />
            <span>{isStreaming ? '实时调度监听中…' : '调度拓扑已归档'}</span>
          </div>
        </div>

        <div className={styles.controlsArea}>
          {/* 交互轮次与时序推演控制器 */}
          <div className={styles.turnNavGroup}>
            <button
              type="button"
              className={`${styles.turnNavBtn} ${selectedTurn === null ? styles.turnNavBtnActive : ''}`}
              onClick={() => handleSelectTurn(null)}
            >
              全部交互 ({trajectory.turnsCount || 1} 轮)
            </button>
            {trajectory.turnGroups?.map((g) => (
              <button
                key={g.turn}
                type="button"
                className={`${styles.turnNavBtn} ${selectedTurn === g.turn ? styles.turnNavBtnActive : ''}`}
                onClick={() => handleSelectTurn(g.turn)}
              >
                Turn {g.turn}
              </button>
            ))}
            <button
              type="button"
              className={styles.stepPlayBtn}
              onClick={handleStepPlay}
              title="单步推演：模拟 Agent 逐步产生 Ack 握手过程"
            >
              ▶ 单步推演
            </button>
          </div>

          <div className={styles.zoomButtonGroup}>
            <button
              type="button"
              className={styles.toolBtn}
              onClick={() => setScale((s) => Math.max(0.45, s * 0.85))}
              title="缩小 (Zoom Out)"
            >
              −
            </button>
            <button
              type="button"
              className={styles.toolBtn}
              onClick={() => setScale(1)}
              title="复位至 100%"
            >
              {Math.round(scale * 100)}%
            </button>
            <button
              type="button"
              className={styles.toolBtn}
              onClick={() => setScale((s) => Math.min(2.5, s * 1.15))}
              title="放大 (Zoom In)"
            >
              +
            </button>
            <button
              type="button"
              className={styles.toolBtn}
              onClick={handleFitView}
              title="自适应居中"
            >
              ⛶ 居中
            </button>
          </div>

          <button
            type="button"
            className={styles.closeBtn}
            onClick={onClose}
            aria-label="关闭无限画布"
          >
            ✕ 退出画布 (ESC)
          </button>
        </div>
      </header>

      {/* 画布主视口 */}
      <main
        ref={viewportRef}
        className={styles.canvasViewport}
        style={{
          backgroundPosition: `${pan.x}px ${pan.y}px, ${pan.x + 14 * scale}px ${pan.y + 14 * scale}px`,
          backgroundSize: `${28 * scale}px ${28 * scale}px, ${28 * scale}px ${28 * scale}px`,
        }}
        onMouseDown={handleMouseDown}
        onMouseMove={handleMouseMove}
        onMouseUp={handleMouseUp}
        onMouseLeave={handleMouseUp}
        onWheel={handleWheel}
        onTouchStart={handleTouchStart}
        onTouchMove={handleTouchMove}
        onTouchEnd={handleTouchEnd}
      >
        {/* 画布世界层（缩放与平移变换） */}
        <div
          className={styles.canvasWorld}
          style={{
            transform: `translate(${pan.x}px, ${pan.y}px) scale(${scale})`,
          }}
        >
          {/* 交互轮次集群泳道层 (Turn Swimlanes) */}
          <div className={styles.turnGroupsLayer}>
            {trajectory.turnGroups?.map((group) => (
              <div
                key={group.turn}
                className={`${styles.turnGroupContainer} ${
                  selectedTurn === group.turn ? styles.turnGroupFocused : ''
                }`}
                style={{
                  left: `${group.x}px`,
                  top: `${group.y}px`,
                  width: `${group.width}px`,
                  height: `${group.height}px`,
                }}
              >
                <div className={styles.turnGroupHeader}>
                  <div className={styles.turnBadge}>TURN {group.turn}</div>
                  <div className={styles.turnTitleArea}>
                    <span className={styles.turnTitle}>{group.title}</span>
                    <span className={styles.turnSubtitle}>{group.subtitle}</span>
                  </div>
                  <span
                    className={`${styles.turnStatusBadge} ${
                      group.status === 'running'
                        ? styles.turnStatusRunning
                        : styles.turnStatusCompleted
                    }`}
                  >
                    {group.status === 'running' ? '交互进行中…' : '本轮闭环'}
                  </span>
                </div>
              </div>
            ))}
          </div>

          {/* SVG 贝塞尔曲线连线层 */}
          <svg className={styles.svgOverlay}>
            <defs>
              <linearGradient id="lineGradient" x1="0%" y1="0%" x2="100%" y2="0%">
                <stop offset="0%" stopColor="#7c3aed" />
                <stop offset="50%" stopColor="#3b82f6" />
                <stop offset="100%" stopColor="#10b981" />
              </linearGradient>
            </defs>
            {trajectory.connections.map((conn, idx) => {
              const fromNode = nodeMap.get(conn.from)
              const toNode = nodeMap.get(conn.to)
              if (!fromNode || !toNode) return null

              const fromX = fromNode.x + 240
              const fromY = fromNode.y + 68
              const toX = toNode.x
              const toY = toNode.y + 68
              const c1X = fromX + Math.abs(toX - fromX) * 0.45
              const c2X = toX - Math.abs(toX - fromX) * 0.45

              const d = `M ${fromX} ${fromY} C ${c1X} ${fromY}, ${c2X} ${toY}, ${toX} ${toY}`
              const isConnActive = selectedNode?.id === conn.to || selectedNode?.id === conn.from
              return (
                <g key={idx}>
                  <path d={d} className={styles.connectionGlow} />
                  <path
                    d={d}
                    className={styles.connectionLine}
                    style={isConnActive ? { stroke: '#38bdf8', strokeWidth: 2.8 } : undefined}
                  />
                  {conn.label && (
                    <g
                      transform={`translate(${(fromX + toX) / 2}, ${(fromY + toY) / 2 - 12})`}
                      style={{ pointerEvents: 'none' }}
                    >
                      <rect
                        x={-Math.max(conn.label.length * 6 + 16, 44) / 2}
                        y={-11}
                        width={Math.max(conn.label.length * 6 + 16, 44)}
                        height={22}
                        rx={11}
                        className={styles.ackPillRect}
                      />
                      <text
                        x={0}
                        y={3}
                        className={styles.ackPillText}
                        textAnchor="middle"
                      >
                        {conn.label}
                      </text>
                    </g>
                  )}
                </g>
              )
            })}
          </svg>

          {/* 节点层 */}
          <div className={styles.nodesLayer}>
            {trajectory.nodes.map((node) => {
              const isSelected = selectedNode?.id === node.id
              const isGate = node.kind === 'gate'
              return (
                <div
                  key={node.id}
                  className={`${styles.nodeCard} ${isSelected ? styles.nodeCardSelected : ''} ${
                    isGate ? styles.nodeCardGate : ''
                  }`}
                  style={{ left: `${node.x}px`, top: `${node.y}px` }}
                  onClick={() => setSelectedNode(node)}
                >
                  <div className={styles.nodeHeader}>
                    <div className={styles.nodeIconArea}>
                      <span className={styles.nodeIcon}>{node.icon}</span>
                      <span className={styles.nodeKindTag}>{node.kind}</span>
                      {node.ackLabel && (
                        <span className={styles.nodeAckTag}>{node.ackLabel}</span>
                      )}
                    </div>
                    <span
                      className={`${styles.nodeStatusDot} ${
                        node.status === 'completed'
                          ? styles.statusCompleted
                          : node.status === 'running'
                          ? styles.statusRunning
                          : node.status === 'waiting'
                          ? styles.statusWaiting
                          : styles.statusPending
                      }`}
                      title={node.status}
                    />
                  </div>

                  <h4 className={styles.nodeTitle}>{node.title}</h4>
                  <p className={styles.nodeSubtitle}>{node.subtitle}</p>
                  <div className={styles.nodeDetails}>{node.details}</div>

                  {node.meta && (
                    <div className={styles.nodeMetaTags}>
                      {Object.entries(node.meta).map(([k, v]) => (
                        <span key={k} className={styles.nodeMetaBadge}>
                          {k}: {v}
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        </div>

        {/* 底部实时干预与驾驶条 (Steering Console) */}
        <div className={styles.steeringBar}>
          <span className={styles.steeringIcon}>⚡</span>
          <form
            onSubmit={handleSteerSubmit}
            style={{ flex: 1, display: 'flex', gap: '8px' }}
          >
            <input
              type="text"
              className={styles.steeringInput}
              placeholder="在此输入人类干预指令（例如：不要买青菜，改去买西红柿）…"
              value={steerInput}
              onChange={(e) => setSteerInput(e.target.value)}
            />
            <button
              type="submit"
              className={styles.steeringSendBtn}
              disabled={!steerInput.trim()}
            >
              注入干预 (Steer)
            </button>
          </form>
          {isStreaming && onCancel && (
            <button
              type="button"
              className={styles.steeringCancelBtn}
              onClick={handleCancelClick}
            >
              紧急中断
            </button>
          )}
        </div>

        {/* 侧边节点详情抽屉 */}
        {selectedNode && (
          <aside className={styles.detailDrawer}>
            <div className={styles.drawerHeader}>
              <h3>
                {selectedNode.icon} {selectedNode.title}
              </h3>
              <button
                type="button"
                className={styles.drawerClose}
                onClick={() => setSelectedNode(null)}
              >
                ✕
              </button>
            </div>
            <div className={styles.drawerContent}>
              <div className={styles.drawerField}>
                <div className={styles.drawerLabel}>交互轮次 / TURN & PHASE</div>
                <div className={styles.drawerValue}>
                  {selectedNode.turn ? `Turn ${selectedNode.turn}` : '初始上下文'} · {selectedNode.phase || selectedNode.kind}
                  {selectedNode.ackLabel ? ` (${selectedNode.ackLabel})` : ''}
                </div>
              </div>
              <div className={styles.drawerField}>
                <div className={styles.drawerLabel}>节点类型 / KIND</div>
                <div className={styles.drawerValue}>{selectedNode.kind}</div>
              </div>
              <div className={styles.drawerField}>
                <div className={styles.drawerLabel}>执行状态 / STATUS</div>
                <div className={styles.drawerValue}>{selectedNode.status}</div>
              </div>
              <div className={styles.drawerField}>
                <div className={styles.drawerLabel}>节点详细描述 / DETAILS</div>
                <div className={styles.drawerValue}>{selectedNode.details}</div>
              </div>
              {selectedNode.meta && (
                <div className={styles.drawerField}>
                  <div className={styles.drawerLabel}>元数据与凭证 / METADATA</div>
                  <div className={styles.drawerValue}>
                    {JSON.stringify(selectedNode.meta, null, 2)}
                  </div>
                </div>
              )}
            </div>
          </aside>
        )}
      </main>
    </div>
  )
}
