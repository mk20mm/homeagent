import { Button, Input, Space, Switch, Table, Typography } from 'antd'
import dayjs from 'dayjs'
import { useCallback, useEffect, useState } from 'react'

import { api, unwrap } from '@/api/client'
import { RISK_LABEL, type RiskLevel } from '@homeagent/shared'

import type { paths } from '../api/schema'

type AuditList = paths['/audit']['get']['responses']['200']['content']['application/json']
type AuditItem = AuditList['items'][number]

/** 审计日志：游标分页 + 越权尝试过滤（只增不改，对齐 ADR-004） */
export function AuditPage() {
  const [items, setItems] = useState<AuditItem[]>([])
  const [cursor, setCursor] = useState<string>()
  const [toolName, setToolName] = useState('')
  const [deniedOnly, setDeniedOnly] = useState(false)
  const [loading, setLoading] = useState(false)

  const load = useCallback(
    async (reset: boolean) => {
      setLoading(true)
      try {
        const data = unwrap(
          await api.GET('/audit', {
            params: {
              query: {
                page_size: 20,
                cursor: reset ? undefined : cursor,
                tool_name: toolName || undefined,
                permission_denied: deniedOnly || undefined,
              },
            },
          }),
        )
        setItems((prev) => (reset ? data.items : [...prev, ...data.items]))
        setCursor(data.next_cursor)
      } finally {
        setLoading(false)
      }
    },
    [cursor, toolName, deniedOnly],
  )

  useEffect(() => {
    void load(true)
    // 依赖变化时重置：useCallback 闭包已含筛选条件
  }, [toolName, deniedOnly, load])

  return (
    <div>
      <Typography.Title level={4}>审计日志</Typography.Title>
      <Space style={{ marginBottom: 16 }}>
        <Input
          allowClear
          placeholder="按工具名过滤"
          style={{ width: 220 }}
          value={toolName}
          onChange={(e) => setToolName(e.target.value)}
        />
        <Switch
          checkedChildren="仅越权"
          unCheckedChildren="全部"
          checked={deniedOnly}
          onChange={setDeniedOnly}
        />
      </Space>

      <Table
        rowKey="id"
        size="small"
        loading={loading}
        pagination={false}
        dataSource={items}
        columns={[
          {
            title: '时间',
            dataIndex: 'created_at',
            render: (v: string) => dayjs(v).format('MM-DD HH:mm:ss'),
          },
          { title: '工具', dataIndex: 'tool_name' },
          {
            title: '风险',
            dataIndex: 'risk',
            render: (v?: RiskLevel) => (v ? RISK_LABEL[v] : '-'),
          },
          { title: '结果', dataIndex: 'result', ellipsis: true },
          { title: '耗时', dataIndex: 'latency_ms', render: (v?: number) => (v ? `${v}ms` : '-') },
          { title: '撤销', dataIndex: 'undone', render: (v?: boolean) => (v ? '已撤销' : '-') },
          {
            title: '越权',
            dataIndex: 'permission_denied',
            render: (v?: boolean) => (v ? '是' : '-'),
          },
          { title: 'trace_id', dataIndex: 'trace_id', ellipsis: true },
        ]}
      />

      <Button
        style={{ marginTop: 16 }}
        loading={loading}
        disabled={!cursor}
        onClick={() => void load(false)}
      >
        加载更多
      </Button>
    </div>
  )
}
