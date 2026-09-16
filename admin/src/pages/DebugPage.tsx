import { Input, message, Select, Space, Typography } from 'antd'
import { useEffect, useState } from 'react'

import { api, unwrap } from '@/api/client'
import { useSSE, type SSEEvent } from '@/hooks/useSSE'
import { ERROR_MESSAGE, RISK_LABEL, type RiskLevel } from '@homeagent/shared'

import type { paths } from '../api/schema'

type ToolList = paths['/tools']['get']['responses']['200']['content']['application/json']
type Tool = ToolList['tools'][number]

/**
 * 工具调试台：选工具 → 按其 input_schema 填参 → 走 /chat SSE 实测回执。
 * 对齐 web-02 设计图；直接调工具的端点 V1 没有，统一经对话触发。
 */
export function DebugPage() {
  const [tools, setTools] = useState<Tool[]>([])
  const [toolName, setToolName] = useState<string>()
  const [params, setParams] = useState('{}')
  const [output, setOutput] = useState<string[]>([])

  const { status, connect, abort } = useSSE({
    url: '/api/v1/chat',
    onEvent: (e: SSEEvent) => {
      if (e.type === 'token') setOutput((p) => [...p, e.content])
      else if (e.type === 'tool_call')
        setOutput((p) => [...p, `[工具回执] ${e.tool}: ${JSON.stringify(e.card)}`])
      else if (e.type === 'done') setOutput((p) => [...p, '--- done ---'])
    },
    onError: (err) => message.error(`${ERROR_MESSAGE.internal}：${err.message}`),
  })

  useEffect(() => {
    void (async () => {
      const data = unwrap(await api.GET('/tools'))
      setTools(data.tools)
      if (data.tools[0]) setToolName(data.tools[0].name)
    })()
  }, [])

  const tool = tools.find((t) => t.name === toolName)

  const send = () => {
    if (!tool) return
    let parsed: unknown
    try {
      parsed = JSON.parse(params)
    } catch {
      message.error('参数不是合法 JSON')
      return
    }
    setOutput([])
    connect({ content: `调用工具 ${tool.name}，参数：${JSON.stringify(parsed)}` })
  }

  return (
    <div>
      <Typography.Title level={4}>工具调试台</Typography.Title>
      <Space direction="vertical" style={{ width: '100%' }} size="large">
        <Select
          style={{ width: 320 }}
          placeholder="选择工具"
          value={toolName}
          onChange={setToolName}
          options={tools.map((t) => ({
            value: t.name,
            label: `${t.name}（${RISK_LABEL[t.risk as RiskLevel]}风险）`,
          }))}
        />

        {tool ? (
          <Typography.Paragraph type="secondary">
            {tool.description} · 权限码 {tool.permission}
          </Typography.Paragraph>
        ) : null}

        <Input.TextArea
          rows={8}
          value={params}
          onChange={(e) => setParams(e.target.value)}
          placeholder="按工具 input_schema 填写 JSON 参数"
          style={{ fontFamily: 'monospace' }}
        />

        <Space>
          <button
            type="button"
            onClick={send}
            disabled={status === 'streaming' || status === 'tool_running'}
          >
            发送调试
          </button>
          <button type="button" onClick={abort} disabled={status === 'idle'}>
            中断
          </button>
          {status !== 'idle' ? <span>状态：{status}</span> : null}
        </Space>

        <pre style={{ background: '#f6f7f9', padding: 16, borderRadius: 8, minHeight: 120 }}>
          {output.join('') || '回执将在此流式显示'}
        </pre>
      </Space>
    </div>
  )
}
