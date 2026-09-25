import {
  Badge,
  Button,
  Card,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Typography,
  message,
} from 'antd'
import { useEffect, useState } from 'react'

import { api, ApiError, unwrap } from '@/api/client'
import { PROVIDER_LABEL, type ProviderName } from '@homeagent/shared'

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
  enabled?: boolean
}

/**
 * 模型与供应商管理：
 * - 供应商密钥加密存储（脱敏回显）、自定义 API Base URL（支持 OpenAI 兼容第三方端点）、启停与连通性测试。
 * - 模型的增删、设为全局默认、启用/停用。
 * - 依赖 DynamicGateway 的缓存热重载（无需重启后端）。
 */
export function ModelPage() {
  const [providers, setProviders] = useState<ProviderRow[]>([])
  const [models, setModels] = useState<ModelRow[]>([])
  const [editing, setEditing] = useState<ProviderRow>()
  const [addingModel, setAddingModel] = useState(false)
  const [testingId, setTestingId] = useState<string>()
  const [form] = Form.useForm()
  const [modelForm] = Form.useForm()

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
      const body: Record<string, unknown> = { base_url: values.base_url?.trim() ?? '' }
      if (values.api_key) body.api_key = values.api_key.trim()
      unwrap(
        await api.PUT('/admin/providers/{providerId}', {
          params: { path: { providerId: editing.id } },
          body,
        }),
      )
      message.success('供应商配置已保存')
      setEditing(undefined)
      void load()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '保存失败')
    }
  }

  const testProvider = async (p: ProviderRow) => {
    setTestingId(p.id)
    try {
      const res = unwrap(
        await api.POST('/admin/providers/{providerId}/test', {
          params: { path: { providerId: p.id } },
        }),
      )
      if (res.success) {
        message.success(`连接测试成功（响应延迟 ${res.latency_ms}ms）`)
      } else {
        message.error(`连接测试失败：${res.error || '无法连通'}`)
      }
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '连接测试请求失败')
    } finally {
      setTestingId(undefined)
    }
  }

  const toggleModel = async (m: ModelRow, field: 'enabled' | 'is_default', value: boolean) => {
    try {
      unwrap(
        await api.PUT('/admin/models/{modelId}', {
          params: { path: { modelId: m.id } },
          body: { [field]: value },
        }),
      )
      message.success(field === 'is_default' ? '已设为默认模型' : value ? '模型已启用' : '模型已停用')
      void load()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '操作失败')
    }
  }

  const watchedProviderId = Form.useWatch('provider_id', modelForm)
  const selectedProvider = providers.find((p) => p.id === watchedProviderId)

  const handleAddModel = async () => {
    try {
      const values = await modelForm.validateFields()

      // 如果弹窗内输入或更新了 Base URL 或 API 密钥，直接联动更新对应供应商
      if (values.base_url !== undefined || values.api_key) {
        const provBody: Record<string, unknown> = {}
        if (values.base_url !== undefined) provBody.base_url = values.base_url.trim()
        if (values.api_key) provBody.api_key = values.api_key.trim()
        if (Object.keys(provBody).length > 0) {
          unwrap(
            await api.PUT('/admin/providers/{providerId}', {
              params: { path: { providerId: values.provider_id } },
              body: provBody,
            }),
          )
        }
      }

      unwrap(
        await api.POST('/admin/models', {
          body: {
            provider_id: values.provider_id,
            model_name: values.model_name.trim(),
            display_name: values.display_name.trim(),
            is_default: Boolean(values.is_default),
          },
        }),
      )
      message.success('模型新增成功，连接配置已热生效')
      setAddingModel(false)
      modelForm.resetFields()
      void load()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '新增模型失败')
    }
  }

  const deleteModel = async (m: ModelRow) => {
    try {
      unwrap(
        await api.DELETE('/admin/models/{modelId}', {
          params: { path: { modelId: m.id } },
        }),
      )
      message.success('模型已删除')
      void load()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '删除失败')
    }
  }

  return (
    <div>
      <Typography.Title level={4}>LLM 供应商管理</Typography.Title>
      <Typography.Paragraph type="secondary">
        配置基础供应商与第三方兼容中继端点（如 Atria ASI、OpenRouter、OneAPI 等）。API 密钥经 AES-GCM 加密存储，修改即时热生效。
      </Typography.Paragraph>
      <Table
        rowKey="id"
        size="small"
        pagination={false}
        dataSource={providers}
        columns={[
          {
            title: '供应商标识',
            dataIndex: 'name',
            render: (v: ProviderName) => PROVIDER_LABEL[v] ?? v,
          },
          {
            title: 'API 基础地址（Base URL）',
            dataIndex: 'base_url',
            render: (v?: string) => v || <span style={{ color: '#8e8e93' }}>使用官方默认地址</span>,
          },
          {
            title: 'API 密钥',
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
              <Space size="small">
                <Button size="small" onClick={() => openEdit(r)}>
                  配置
                </Button>
                <Button
                  size="small"
                  loading={testingId === r.id}
                  disabled={!r.api_key_set}
                  onClick={() => void testProvider(r)}
                >
                  测试连接
                </Button>
              </Space>
            ),
          },
        ]}
      />

      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          marginTop: 32,
          marginBottom: 10,
        }}
      >
        <div>
          <Typography.Title level={4} style={{ margin: 0 }}>
            模型配置列表
          </Typography.Title>
          <Typography.Paragraph type="secondary" style={{ margin: 0, marginTop: 4 }}>
            会话内切换清单来源。支持绑定供应商、自定义展示名，支持动态指定全局默认模型。
          </Typography.Paragraph>
        </div>
        <Button type="primary" size="small" onClick={() => setAddingModel(true)}>
          + 添加模型
        </Button>
      </div>

      <Table
        rowKey="id"
        size="small"
        pagination={false}
        dataSource={models}
        columns={[
          { title: '展示名', dataIndex: 'display_name' },
          { title: '供应商侧真实模型名', dataIndex: 'model_name' },
          {
            title: '所属供应商',
            dataIndex: 'provider',
            render: (v?: ProviderName) => (v ? PROVIDER_LABEL[v] ?? v : '-'),
          },
          {
            title: '全局默认',
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
          {
            title: '操作',
            render: (_, r) =>
              r && !r.is_default ? (
                <Popconfirm
                  title="确认删除该模型配置？"
                  onConfirm={() => void deleteModel(r)}
                  okText="删除"
                  cancelText="取消"
                >
                  <Button size="small" danger type="text">
                    删除
                  </Button>
                </Popconfirm>
              ) : null,
          },
        ]}
      />

      {/* 供应商编辑弹窗 */}
      <Modal
        title={`配置供应商${editing ? ' · ' + (PROVIDER_LABEL[editing.name as ProviderName] ?? editing.name) : ''}`}
        open={Boolean(editing)}
        onOk={() => void saveProvider()}
        onCancel={() => setEditing(undefined)}
        okText="保存"
        cancelText="取消"
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="base_url"
            label="API 地址（Base URL）"
            extra="支持填入自定义兼容中继地址，例如 https://api.atria-asi.ai/v1 或 https://api.openai.com/v1"
          >
            <Input placeholder="留空使用供应商默认地址" />
          </Form.Item>
          <Form.Item name="api_key" label="API 密钥（Key）">
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

      {/* 新增模型弹窗 */}
      <Modal
        title="添加模型"
        open={addingModel}
        onOk={() => void handleAddModel()}
        onCancel={() => {
          setAddingModel(false)
          modelForm.resetFields()
        }}
        okText="添加"
        cancelText="取消"
        width={560}
      >
        <Form form={modelForm} layout="vertical">
          <Form.Item
            name="provider_id"
            label="所属供应商通道"
            rules={[{ required: true, message: '请选择供应商' }]}
            extra="如接入 Atria ASI 等第三方端点，请选择 OpenAI（兼容任何标准协议服务）"
          >
            <Select
              placeholder="选择供应商"
              options={providers.map((p) => ({
                value: p.id,
                label:
                  p.name === 'openai'
                    ? 'OpenAI / 兼容第三方自定义中继（如 Atria ASI、OpenRouter）'
                    : PROVIDER_LABEL[p.name as ProviderName] ?? p.name,
              }))}
              onChange={(pId) => {
                const p = providers.find((item) => item.id === pId)
                if (p) {
                  modelForm.setFieldsValue({ base_url: p.base_url ?? '' })
                }
              }}
            />
          </Form.Item>

          <Card
            size="small"
            style={{
              background: '#f9f9fb',
              marginBottom: 20,
              borderRadius: 8,
              border: '1px solid #ebebf0',
            }}
          >
            <Typography.Text strong style={{ fontSize: 13, display: 'block', marginBottom: 8 }}>
              🔗 供应商连接参数（URL 与 密钥）
            </Typography.Text>
            <Form.Item
              name="base_url"
              label="API 基础地址（Base URL）"
              style={{ marginBottom: 12 }}
              extra="例如：https://api.atria-asi.ai/v1 或留空使用官方默认地址"
            >
              <Input placeholder="例如：https://api.atria-asi.ai/v1" />
            </Form.Item>
            <Form.Item
              name="api_key"
              label="API 密钥（Key）"
              style={{ marginBottom: 8 }}
              extra={
                selectedProvider?.api_key_set
                  ? `当前已配置密钥（${selectedProvider.api_key_masked}），留空沿用，输入即覆盖加密`
                  : '粘贴完整 API 密钥，AES-GCM 加密存储'
              }
            >
              <Input.Password placeholder="输入/修改 API 密钥（如 asi-xxx 或 sk-xxx）" />
            </Form.Item>
          </Card>

          <Form.Item
            name="model_name"
            label="供应商侧真实模型名"
            rules={[{ required: true, message: '请输入模型名' }]}
            extra="调用供应商 API 时实际传递的 model 字段，例如 deepseek-chat、gpt-4o 或 Atria ASI 上的模型 ID"
          >
            <Input placeholder="例如：deepseek-reasoner 或 meta-llama/Llama-3-70b-chat" />
          </Form.Item>
          <Form.Item
            name="display_name"
            label="界面展示名"
            rules={[{ required: true, message: '请输入展示名' }]}
            extra="在 Web 端模型胶囊切换器中展示给家庭成员的名称"
          >
            <Input placeholder="例如：Atria ASI 旗舰模型 或 DeepSeek R1" />
          </Form.Item>
          <Form.Item name="is_default" label="设为全局默认" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
