import { Card, Col, Row, Statistic, Table, Typography } from 'antd'
import dayjs from 'dayjs'
import { useEffect, useState } from 'react'

import { api, unwrap } from '@/api/client'
import { RISK_LABEL, type RiskLevel } from '@homeagent/shared'

import type { paths } from '../api/schema'

type Usage = paths['/usage']['get']['responses']['200']['content']['application/json']
type AuditList = paths['/audit']['get']['responses']['200']['content']['application/json']

/** 仪表盘：LLM 用量（7 日）+ 最近工具调用（对齐 web-01 设计图） */
export function DashboardPage() {
  const [usage, setUsage] = useState<Usage>()
  const [audit, setAudit] = useState<AuditList>()

  useEffect(() => {
    void (async () => {
      setUsage(unwrap(await api.GET('/usage', { params: { query: { days: 7 } } })))
      setAudit(unwrap(await api.GET('/audit', { params: { query: { page_size: 5 } } })))
    })()
  }, [])

  return (
    <div>
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={8}>
          <Card>
            <Statistic
              title="7 日花费（元）"
              value={usage?.total_cost ?? 0}
              precision={2}
              prefix="¥"
            />
          </Card>
        </Col>
        <Col span={8}>
          <Card>
            <Statistic title="7 日 Token 总量" value={usage?.total_tokens ?? 0} />
          </Card>
        </Col>
        <Col span={8}>
          <Card>
            <Statistic title="近 5 次工具调用" value={audit?.items.length ?? 0} suffix="条" />
          </Card>
        </Col>
      </Row>

      <Typography.Title level={4}>每日用量</Typography.Title>
      <Table
        rowKey="date"
        size="small"
        pagination={false}
        dataSource={usage?.items ?? []}
        columns={[
          { title: '日期', dataIndex: 'date' },
          { title: 'Token', dataIndex: 'tokens' },
          { title: '花费（元）', dataIndex: 'cost', render: (v: number) => `¥${v.toFixed(2)}` },
        ]}
      />

      <Typography.Title level={4} style={{ marginTop: 24 }}>
        最近工具调用
      </Typography.Title>
      <Table
        rowKey="id"
        size="small"
        pagination={false}
        dataSource={audit?.items ?? []}
        columns={[
          { title: '工具', dataIndex: 'tool_name' },
          {
            title: '风险',
            dataIndex: 'risk',
            render: (v?: RiskLevel) => (v ? RISK_LABEL[v] : '-'),
          },
          { title: '结果', dataIndex: 'result', ellipsis: true },
          {
            title: '时间',
            dataIndex: 'created_at',
            render: (v: string) => dayjs(v).format('MM-DD HH:mm'),
          },
        ]}
      />
    </div>
  )
}
