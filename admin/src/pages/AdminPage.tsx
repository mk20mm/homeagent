import { Badge, Button, Card, Form, Input, Modal, Switch, Table, Typography, message } from 'antd'
import { useEffect, useState } from 'react'

import { api, ApiError, unwrap } from '@/api/client'
import {
  MEMBER_ROLE_LABEL,
  PROVIDER_LABEL,
  type MemberRole,
  type ProviderName,
} from '@homeagent/shared'

interface ProviderRow {
  id: string
  name: string
  base_url?: string
  api_key_masked?: string
  api_key_set?: boolean
  enabled?: boolean
}

interface ModelRow {
  id: string
  model_name: string
  display_name: string
  provider?: string
  is_default?: boolean
}

/**
 * 供应商与模型配置：密钥加密入库（脱敏回显）、启停模型、设默认模型。
 * 只有家长角色可写（后端 parent 权限双保险）。
 */
export function AdminPage() {
  const [providers, setProviders] = useState<ProviderRow[]>([])
  const [models, setModels] = useState<ModelRow[]>([])
  const [editing, setEditing] = useState<ProviderRow>()
  const [form] = Form.useForm()

  const load = async () => {
    try {
      const [pv, mv] = await Promise.all([
        unwrap(await api.GET('/admin/providers')),
        unwrap(await api.GET('/models')),
      ])
      setProviders(pv.providers)
      setModels(mv.models)
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '加载配置失败')
    }
  }

  useEffect(() => {
    void load()
  }, [])

  const openEdit = (p: ProviderRow) => {
    setEditing(p)
    form.setFieldsValue({ base_url: p.base_url ?? '', api_key: '' })
  }

  const saveProvider = async () => {
    if (!editing) return
    const values = await form.validateFields()
    try {
      // api_key 空串 = 不修改，只有填了才提交
      const body: Record<string, unknown> = { base_url: values.base_url ?? '' }
      if (values.api_key) body.api_key = values.api_key
      unwrap(await api.PUT('/admin/providers/{providerId}', {
        params: { path: { providerId: editing.id } },
        body,
      }))
      message.success('供应商配置已保存')
      setEditing(undefined)
      void load()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '保存失败')
    }
  }

  const toggleModel = async (m: ModelRow, field: 'enabled' | 'is_default', value: boolean) => {
    try {
      unwrap(await api.PUT('/admin/models/{modelId}', {
        params: { path: { modelId: m.id } },
        body: { [field]: value },
      }))
      message.success(field === 'is_default' ? '已设为默认模型' : value ? '模型已启用' : '模型已停用')
      void load()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '操作失败')
    }
  }

  return (
    <div>
      <Typography.Title level={4}>供应商与模型</Typography.Title>
      <Typography.Paragraph type="secondary">
        密钥加密存储，页面只显示脱敏值；修改密钥需重新填写完整密钥。
      </Typography.Paragraph>
      <Table
        rowKey="id"
        size="small"
        pagination={false}
        dataSource={providers}
        columns={[
          {
            title: '供应商',
            dataIndex: 'name',
            render: (v: ProviderName) => PROVIDER_LABEL[v] ?? v,
          },
          { title: 'API 地址', dataIndex: 'base_url' },
          {
            title: '密钥',
            render: (_, r) =>
              r.api_key_set ? (
                <Typography.Text code>{r.api_key_masked}</Typography.Text>
              ) : (
                <Badge status="warning" text="未配置" />
              ),
          },
          {
            title: '状态',
            dataIndex: 'enabled',
            render: (v?: boolean) =>
              v ? <Badge status="success" text="启用" /> : <Badge status="default" text="停用" />,
          },
          {
            title: '操作',
            render: (_, r) => (
              <Button size="small" onClick={() => openEdit(r)}>
                配置
              </Button>
            ),
          },
        ]}
      />

      <Typography.Title level={4} style={{ marginTop: 24 }}>
        模型
      </Typography.Title>
      <Table
        rowKey="id"
        size="small"
        pagination={false}
        dataSource={models}
        columns={[
          { title: '展示名', dataIndex: 'display_name' },
          { title: '模型名', dataIndex: 'model_name' },
          {
            title: '供应商',
            dataIndex: 'provider',
            render: (v?: ProviderName) => (v ? PROVIDER_LABEL[v] ?? v : '-'),
          },
          {
            title: '默认',
            dataIndex: 'is_default',
            render: (v?: boolean, r?: ModelRow) =>
              r ? (
                <Switch
                  checked={v ?? false}
                  onChange={(checked) => void toggleModel(r, 'is_default', checked)}
                />
              ) : null,
          },
          {
            title: '启用',
            dataIndex: 'enabled',
            render: (v?: boolean, r?: ModelRow) =>
              r ? (
                <Switch
                  checked={v ?? false}
                  onChange={(checked) => void toggleModel(r, 'enabled', checked)}
                />
              ) : null,
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

      <Modal
        title={`配置供应商${editing ? ' · ' + (PROVIDER_LABEL[editing.name as ProviderName] ?? editing.name) : ''}`}
        open={Boolean(editing)}
        onOk={() => void saveProvider()}
        onCancel={() => setEditing(undefined)}
        okText="保存"
        cancelText="取消"
      >
        <Form form={form} layout="vertical">
          <Form.Item name="base_url" label="API 地址">
            <Input placeholder="留空使用供应商默认地址" />
          </Form.Item>
          <Form.Item name="api_key" label="API 密钥">
            <Input.Password
              placeholder={
                editing?.api_key_set
                  ? `已配置（${editing.api_key_masked}），留空不修改`
                  : '粘贴完整密钥，加密存储'
              }
            />
          </Form.Item>
        </Form>
      </Modal>
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
