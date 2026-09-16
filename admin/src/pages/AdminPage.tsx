import { Card, Table, Typography } from 'antd'
import { useEffect, useState } from 'react'

import { api, unwrap } from '@/api/client'
import {
  MEMBER_ROLE_LABEL,
  PROVIDER_LABEL,
  type MemberRole,
  type ProviderName,
} from '@homeagent/shared'

import type { paths } from '../api/schema'

type ModelList = paths['/models']['get']['responses']['200']['content']['application/json']

/**
 * 模型供应商配置：当前启用模型清单（GET /models）。
 * 供应商/成员权限矩阵的写端点 V1 未开放，先展示只读视图（对齐 web-03 设计图）。
 */
export function AdminPage() {
  const [models, setModels] = useState<ModelList>()

  useEffect(() => {
    void (async () => {
      setModels(unwrap(await api.GET('/models')))
    })()
  }, [])

  return (
    <div>
      <Typography.Title level={4}>供应商与模型</Typography.Title>
      <Table
        rowKey="id"
        size="small"
        pagination={false}
        dataSource={models?.models ?? []}
        columns={[
          { title: '展示名', dataIndex: 'display_name' },
          { title: '模型名', dataIndex: 'model_name' },
          {
            title: '供应商',
            dataIndex: 'provider',
            render: (v: ProviderName) => PROVIDER_LABEL[v] ?? v,
          },
        ]}
      />

      <Typography.Title level={4} style={{ marginTop: 24 }}>
        成员权限矩阵
      </Typography.Title>
      <Card size="small">
        <Table
          rowKey="role"
          size="small"
          pagination={false}
          dataSource={MATRIX}
          columns={[
            {
              title: '角色',
              dataIndex: 'role',
              render: (v: MemberRole) => MEMBER_ROLE_LABEL[v],
            },
            { title: '记账', dataIndex: 'expense' },
            { title: '家务', dataIndex: 'task' },
            { title: '用餐上报', dataIndex: 'meal' },
            { title: '撤销', dataIndex: 'undo' },
            { title: '高危操作', dataIndex: 'highRisk' },
          ]}
        />
        <Typography.Paragraph type="secondary" style={{ marginTop: 12, marginBottom: 0 }}>
          只读占位：写端点在 C 阶段接入成员服务后开放（权限双保险源头见 ADR-005）。
        </Typography.Paragraph>
      </Card>
    </div>
  )
}

interface PermRow {
  role: MemberRole
  expense: string
  task: string
  meal: string
  undo: string
  highRisk: string
}

const MATRIX: PermRow[] = [
  { role: 'parent', expense: '读写', task: '读写', meal: '读写', undo: '允许', highRisk: '允许' },
  { role: 'adult', expense: '读写', task: '读写', meal: '读写', undo: '允许', highRisk: '需确认' },
  { role: 'elder', expense: '只读', task: '只读', meal: '读写', undo: '允许', highRisk: '禁止' },
  { role: 'child', expense: '只读', task: '认领', meal: '读写', undo: '禁止', highRisk: '禁止' },
  {
    role: 'roommate',
    expense: '读写',
    task: '读写',
    meal: '读写',
    undo: '允许',
    highRisk: '需确认',
  },
]
