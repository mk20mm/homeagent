import {
  AlertOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons'
import {
  Alert,
  Badge,
  Button,
  Card,
  Divider,
  Form,
  Input,
  Modal,
  Popconfirm,
  Space,
  Switch,
  Table,
  Tag,
  Tooltip,
  Typography,
  message,
} from 'antd'
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
  enabled: boolean
}

interface TestResultState {
  loading: boolean
  ok?: boolean
  latency_ms?: number
  error?: string
}

interface Preset {
  name: string
  providerType: ProviderName
  label: string
  defaultBaseUrl: string
  defaultModelName: string
  defaultDisplayName: string
  description: string
}

const PRESETS: Preset[] = [
  {
    name: 'atria',
    providerType: 'openai',
    label: 'Atria ASI',
    defaultBaseUrl: 'https://api.atria-asi.ai/v1',
    defaultModelName: 'Atria-Dawn-Preview',
    defaultDisplayName: 'Atria Dawn Preview',
    description: 'Atria 深度推理引擎（兼容 OpenAI 协议）',
  },
  {
    name: 'deepseek',
    providerType: 'deepseek',
    label: 'DeepSeek',
    defaultBaseUrl: 'https://api.deepseek.com',
    defaultModelName: 'deepseek-chat',
    defaultDisplayName: 'DeepSeek V3',
    description: 'DeepSeek 官方平台',
  },
  {
    name: 'openai',
    providerType: 'openai',
    label: 'OpenAI',
    defaultBaseUrl: 'https://api.openai.com/v1',
    defaultModelName: 'gpt-4o-mini',
    defaultDisplayName: 'GPT-4o mini',
    description: 'OpenAI 官方 API',
  },
  {
    name: 'siliconflow',
    providerType: 'openai',
    label: '硅基流动',
    defaultBaseUrl: 'https://api.siliconflow.cn/v1',
    defaultModelName: 'deepseek-ai/DeepSeek-V3',
    defaultDisplayName: 'DeepSeek V3 (硅基流动)',
    description: '国内高并发推理平台',
  },
  {
    name: 'ollama',
    providerType: 'ollama',
    label: 'Ollama 本地',
    defaultBaseUrl: 'http://localhost:11434/v1',
    defaultModelName: 'llama3',
    defaultDisplayName: 'Llama 3 (本地)',
    description: '本地私有化部署模型',
  },
]

// 检查 model_name 是否疑似填错了 API 密钥（如 atr_ 或 sk- 开头且长度较长）
function isLikelyApiKey(modelName: string): boolean {
  if (!modelName) return false
  const trimmed = modelName.trim()
  return (
    (trimmed.startsWith('sk-') || trimmed.startsWith('atr_')) &&
    trimmed.length > 20
  )
}

function maskLikelyKey(str: string): string {
  if (str.length <= 8) return str
  return `${str.slice(0, 6)}...${str.slice(-4)}`
}

export function AdminPage() {
  const [providers, setProviders] = useState<ProviderRow[]>([])
  const [models, setModels] = useState<ModelRow[]>([])
  const [loading, setLoading] = useState(false)

  // 编辑供应商状态
  const [editingProvider, setEditingProvider] = useState<ProviderRow>()
  const [providerForm] = Form.useForm()

  // 添加/编辑模型状态
  const [modelModalVisible, setModelModalVisible] = useState(false)
  const [modelModalMode, setModelModalMode] = useState<'create' | 'edit'>('create')
  const [targetProviderId, setTargetProviderId] = useState<string>('')
  const [editingModelId, setEditingModelId] = useState<string>('')
  const [modelForm] = Form.useForm()
  const [currentModelNameInput, setCurrentModelNameInput] = useState('')

  // 连通性测试状态 (provider_id -> state)
  const [testResults, setTestResults] = useState<Record<string, TestResultState>>({})
  const [modalTestResult, setModalTestResult] = useState<TestResultState>()

  const load = async () => {
    setLoading(true)
    try {
      const [pv, mv] = await Promise.all([
        unwrap(await api.GET('/admin/providers')),
        unwrap(await api.GET('/admin/models')),
      ])
      setProviders(pv.providers)
      setModels(mv.models)
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '加载配置失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  // 打开编辑供应商弹窗
  const openEditProvider = (p: ProviderRow) => {
    setEditingProvider(p)
    setModalTestResult(undefined)
    providerForm.setFieldsValue({
      base_url: p.base_url ?? '',
      api_key: '',
      enabled: p.enabled ?? true,
    })
  }

  // 套用预设到编辑表单
  const applyPresetToForm = (preset: Preset) => {
    providerForm.setFieldsValue({
      base_url: preset.defaultBaseUrl,
    })
    message.info(`已填入 ${preset.label} 默认地址 (${preset.defaultBaseUrl})`)
  }

  // 测试单条供应商连通性（页面卡片触发）
  const runTestConnection = async (p: ProviderRow) => {
    setTestResults((prev) => ({
      ...prev,
      [p.id]: { loading: true },
    }))

    try {
      const res = unwrap(
        await api.POST('/admin/providers/test', {
          body: {
            provider_id: p.id,
          },
        }),
      )

      setTestResults((prev) => ({
        ...prev,
        [p.id]: {
          loading: false,
          ok: res.ok,
          latency_ms: res.latency_ms,
          error: res.error ?? undefined,
        },
      }))

      if (res.ok) {
        message.success(
          `${PROVIDER_LABEL[p.name as ProviderName] ?? p.name} 连通正常 (${res.latency_ms}ms)`,
        )
      } else {
        message.error(`连接失败: ${res.error ?? '未知错误'}`)
      }
    } catch (e) {
      const errMsg = e instanceof ApiError ? e.message : '测试请求异常'
      setTestResults((prev) => ({
        ...prev,
        [p.id]: { loading: false, ok: false, error: errMsg },
      }))
      message.error(errMsg)
    }
  }

  // 在弹窗中测试配置（包含未保存的表单字段）
  const runModalTest = async () => {
    if (!editingProvider) return
    const values = providerForm.getFieldsValue()
    setModalTestResult({ loading: true })

    try {
      const res = unwrap(
        await api.POST('/admin/providers/test', {
          body: {
            provider_id: editingProvider.id,
            base_url: values.base_url || undefined,
            api_key: values.api_key || undefined,
          },
        }),
      )

      setModalTestResult({
        loading: false,
        ok: res.ok,
        latency_ms: res.latency_ms,
        error: res.error ?? undefined,
      })

      if (res.ok) {
        message.success(`探测成功！往返延迟: ${res.latency_ms}ms`)
      } else {
        message.error(`探测失败: ${res.error}`)
      }
    } catch (e) {
      const errMsg = e instanceof ApiError ? e.message : '探测异常'
      setModalTestResult({ loading: false, ok: false, error: errMsg })
      message.error(errMsg)
    }
  }

  // 保存供应商配置
  const saveProvider = async () => {
    if (!editingProvider) return
    const values = await providerForm.validateFields()
    try {
      const body: { base_url?: string; api_key?: string; enabled?: boolean } = {
        base_url: values.base_url ?? '',
        enabled: values.enabled ?? true,
      }
      if (values.api_key && values.api_key.trim() !== '') {
        body.api_key = values.api_key.trim()
      }

      unwrap(
        await api.PUT('/admin/providers/{providerId}', {
          params: { path: { providerId: editingProvider.id } },
          body,
        }),
      )
      message.success('供应商配置已保存')
      setEditingProvider(undefined)
      void load()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '保存失败')
    }
  }

  // 打开添加模型弹窗
  const openAddModel = (p: ProviderRow) => {
    setModelModalMode('create')
    setTargetProviderId(p.id)
    setEditingModelId('')
    setCurrentModelNameInput('')
    modelForm.resetFields()
    modelForm.setFieldsValue({
      model_name: '',
      display_name: '',
      is_default: false,
    })
    setModelModalVisible(true)
  }

  // 打开编辑模型弹窗
  const openEditModel = (m: ModelRow) => {
    setModelModalMode('edit')
    setEditingModelId(m.id)
    setTargetProviderId('')
    setCurrentModelNameInput(m.model_name)
    modelForm.resetFields()
    modelForm.setFieldsValue({
      model_name: m.model_name,
      display_name: m.display_name,
      is_default: m.is_default ?? false,
    })
    setModelModalVisible(true)
  }

  // 套用推荐模型到表单
  const applyPresetModel = (preset: Preset) => {
    modelForm.setFieldsValue({
      model_name: preset.defaultModelName,
      display_name: preset.defaultDisplayName,
    })
    setCurrentModelNameInput(preset.defaultModelName)
  }

  // 保存模型（新增或编辑）
  const handleSaveModel = async () => {
    const values = await modelForm.validateFields()
    try {
      if (modelModalMode === 'create') {
        unwrap(
          await api.POST('/admin/models', {
            body: {
              provider_id: targetProviderId,
              model_name: values.model_name.trim(),
              display_name: values.display_name?.trim() || values.model_name.trim(),
              context_window: 131072,
              is_default: values.is_default ?? false,
            },
          }),
        )
        message.success(`模型 ${values.model_name} 添加成功`)
      } else {
        unwrap(
          await api.PUT('/admin/models/{modelId}', {
            params: { path: { modelId: editingModelId } },
            body: {
              model_name: values.model_name.trim(),
              display_name: values.display_name?.trim() || values.model_name.trim(),
              is_default: values.is_default ?? false,
            },
          }),
        )
        message.success(`模型信息已更新`)
      }
      setModelModalVisible(false)
      void load()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '保存模型失败')
    }
  }

  // 删除模型
  const deleteModel = async (modelId: string) => {
    try {
      unwrap(
        await api.DELETE('/admin/models/{modelId}', {
          params: { path: { modelId } },
        }),
      )
      message.success('模型已删除')
      void load()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '删除失败')
    }
  }

  // 启停或设默认模型
  const toggleModel = async (m: ModelRow, field: 'enabled' | 'is_default', value: boolean) => {
    // 乐观更新前端状态
    setModels((prev) =>
      prev.map((item) => {
        if (field === 'is_default') {
          return {
            ...item,
            is_default: item.id === m.id ? value : false,
          }
        }
        if (item.id === m.id) {
          return { ...item, [field]: value }
        }
        return item
      }),
    )

    try {
      unwrap(
        await api.PUT('/admin/models/{modelId}', {
          params: { path: { modelId: m.id } },
          body: { [field]: value },
        }),
      )
      message.success(
        field === 'is_default' ? '已将该模型设为全局默认' : value ? '模型已启用' : '模型已停用',
      )
      void load()
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '操作失败')
      void load() // 失败回滚
    }
  }

  return (
    <div style={{ maxWidth: 1080, margin: '0 auto', paddingBottom: 48 }}>
      <div style={{ marginBottom: 20 }}>
        <Typography.Title level={3} style={{ marginBottom: 4 }}>
          AI 供应商与模型配置
        </Typography.Title>
        <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
          参考 CC Switch / OpenCode 极简管理交互：配置供应商端点与密钥，测试连通性，增删改模型与切换默认模型。
        </Typography.Paragraph>
      </div>

      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        {providers.map((p) => {
          const pModels = models.filter((m) => m.provider === p.name)
          const testState = testResults[p.id]

          return (
            <Card
              key={p.id}
              size="small"
              title={
                <Space align="center">
                  <Typography.Text strong style={{ fontSize: 16 }}>
                    {PROVIDER_LABEL[p.name as ProviderName] ?? p.name}
                  </Typography.Text>
                  {p.enabled ? (
                    <Badge status="success" text="已启用" />
                  ) : (
                    <Badge status="default" text="已停用" />
                  )}
                  {p.api_key_set ? (
                    <Tag color="green">密钥已配置 ({p.api_key_masked})</Tag>
                  ) : (
                    <Tag color="orange">未配置密钥</Tag>
                  )}
                </Space>
              }
              extra={
                <Space>
                  {testState && !testState.loading && (
                    testState.ok ? (
                      <Tag icon={<CheckCircleOutlined />} color="success">
                        连通正常 ({testState.latency_ms}ms)
                      </Tag>
                    ) : (
                      <Tooltip title={testState.error}>
                        <Tag icon={<CloseCircleOutlined />} color="error">
                          连接异常
                        </Tag>
                      </Tooltip>
                    )
                  )}
                  <Button
                    icon={<ThunderboltOutlined />}
                    size="small"
                    loading={testState?.loading}
                    onClick={() => void runTestConnection(p)}
                  >
                    测试连接
                  </Button>
                  <Button
                    icon={<EditOutlined />}
                    size="small"
                    type="primary"
                    ghost
                    onClick={() => openEditProvider(p)}
                  >
                    配置凭据
                  </Button>
                  <Button
                    icon={<PlusOutlined />}
                    size="small"
                    onClick={() => openAddModel(p)}
                  >
                    添加模型
                  </Button>
                </Space>
              }
            >
              <div style={{ marginBottom: 12 }}>
                <Typography.Text type="secondary" style={{ fontSize: 13, marginRight: 8 }}>
                  API 端点:
                </Typography.Text>
                <Typography.Text code style={{ fontSize: 13 }}>
                  {p.base_url || '使用官方默认地址'}
                </Typography.Text>
              </div>

              <Table
                rowKey="id"
                size="small"
                pagination={false}
                loading={loading}
                dataSource={pModels}
                locale={{ emptyText: '暂无模型，点击右上角「添加模型」新建' }}
                columns={[
                  {
                    title: '展示名称',
                    dataIndex: 'display_name',
                    width: '24%',
                    render: (v: string, r?: ModelRow) => (
                      <Space>
                        <Typography.Text strong={r?.is_default}>{v}</Typography.Text>
                        {r?.is_default && <Tag color="gold">默认模型</Tag>}
                      </Space>
                    ),
                  },
                  {
                    title: '模型标识 (model_name)',
                    dataIndex: 'model_name',
                    width: '32%',
                    render: (v: string) => {
                      if (isLikelyApiKey(v)) {
                        return (
                          <Space>
                            <Typography.Text code type="danger">
                              {maskLikelyKey(v)}
                            </Typography.Text>
                            <Tooltip title="此处检测到疑似 API 密钥！请点击右侧「编辑」更正为实际模型名称（如 Atria-Dawn-Preview），并在「配置凭据」中填写密钥。">
                              <Tag color="red" icon={<AlertOutlined />}>
                                疑似填错密钥
                              </Tag>
                            </Tooltip>
                          </Space>
                        )
                      }
                      return <Typography.Text code>{v}</Typography.Text>
                    },
                  },
                  {
                    title: '设为默认',
                    dataIndex: 'is_default',
                    width: '14%',
                    render: (v?: boolean, r?: ModelRow) =>
                      r ? (
                        <Switch
                          checked={v ?? false}
                          disabled={v}
                          checkedChildren="默认"
                          unCheckedChildren="设为默认"
                          onChange={(checked) => void toggleModel(r, 'is_default', checked)}
                        />
                      ) : null,
                  },
                  {
                    title: '启用状态',
                    dataIndex: 'enabled',
                    width: '14%',
                    render: (v?: boolean, r?: ModelRow) =>
                      r ? (
                        <Switch
                          checked={v ?? false}
                          checkedChildren="已启用"
                          unCheckedChildren="已停用"
                          onChange={(checked) => void toggleModel(r, 'enabled', checked)}
                        />
                      ) : null,
                  },
                  {
                    title: '操作',
                    width: '16%',
                    render: (_: unknown, r?: ModelRow) =>
                      r ? (
                        <Space>
                          <Button
                            type="link"
                            size="small"
                            icon={<EditOutlined />}
                            onClick={() => openEditModel(r)}
                          >
                            编辑
                          </Button>
                          <Popconfirm
                            title="确定删除该模型？"
                            description={
                              r.is_default ? '默认模型不可删除，请先将其他模型设为默认' : undefined
                            }
                            disabled={r.is_default}
                            onConfirm={() => void deleteModel(r.id)}
                            okText="删除"
                            cancelText="取消"
                            okButtonProps={{ danger: true }}
                          >
                            <Button
                              type="text"
                              danger
                              size="small"
                              icon={<DeleteOutlined />}
                              disabled={r.is_default}
                            >
                              删除
                            </Button>
                          </Popconfirm>
                        </Space>
                      ) : null,
                  },
                ]}
              />
            </Card>
          )
        })}

        <Card size="small" title="家庭成员权限矩阵">
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
              { title: '配置与高危操作', dataIndex: 'highRisk' },
            ]}
          />
          <Typography.Paragraph type="secondary" style={{ marginTop: 12, marginBottom: 0 }}>
            只读视图：只有家长（parent）角色可配置 AI 供应商与模型凭据（ADR-005 权限双保险）。
          </Typography.Paragraph>
        </Card>
      </Space>

      {/* 编辑供应商凭据弹窗 */}
      <Modal
        title={
          <Space>
            <span>配置供应商凭据</span>
            {editingProvider && (
              <Tag color="blue">
                {PROVIDER_LABEL[editingProvider.name as ProviderName] ?? editingProvider.name}
              </Tag>
            )}
          </Space>
        }
        open={Boolean(editingProvider)}
        onOk={() => void saveProvider()}
        onCancel={() => setEditingProvider(undefined)}
        okText="保存配置"
        cancelText="取消"
        width={560}
      >
        <div style={{ marginBottom: 16 }}>
          <Typography.Text type="secondary" style={{ fontSize: 13, display: 'block', marginBottom: 8 }}>
            快捷预设（点击一键填入地址）：
          </Typography.Text>
          <Space wrap size={[6, 8]}>
            {PRESETS.map((preset) => (
              <Button
                key={preset.name}
                size="small"
                onClick={() => applyPresetToForm(preset)}
              >
                {preset.label}
              </Button>
            ))}
          </Space>
        </div>

        <Divider style={{ margin: '12px 0' }} />

        {/* 隐藏假凭据输入框阻断浏览器密码管理器乱填 */}
        <input type="text" style={{ display: 'none' }} tabIndex={-1} autoComplete="off" />
        <input type="password" style={{ display: 'none' }} tabIndex={-1} autoComplete="off" />

        <Form form={providerForm} layout="vertical" autoComplete="off">
          <Form.Item
            name="base_url"
            label="API Base URL (端点地址)"
            help="兼容 OpenAI 协议的端点（如 https://api.atria-asi.ai/v1 或 https://api.deepseek.com）"
          >
            <Input
              autoComplete="off"
              name="ha_field_api_endpoint"
              placeholder="如 https://api.atria-asi.ai/v1"
            />
          </Form.Item>

          <Form.Item
            name="api_key"
            label="API Key (密钥)"
            help="服务端 AES-256 加密落库，绝不在页面明文回显；留空则保持原有密钥不修改"
          >
            <Input.Password
              autoComplete="new-password"
              name="ha_field_secret_token"
              placeholder={
                editingProvider?.api_key_set
                  ? `已配置密钥 (${editingProvider.api_key_masked})，如需更换请在此输入新密钥`
                  : '粘贴完整 API Key（以 sk- 或 atr_ 开头）'
              }
            />
          </Form.Item>

          <Form.Item name="enabled" label="启用状态" valuePropName="checked">
            <Switch checkedChildren="已启用" unCheckedChildren="已停用" />
          </Form.Item>
        </Form>

        {modalTestResult && (
          <div style={{ marginTop: 12, padding: 8, background: '#f5f5f5', borderRadius: 6 }}>
            {modalTestResult.ok ? (
              <Space>
                <CheckCircleOutlined style={{ color: '#52c41a' }} />
                <Typography.Text type="success">
                  连通成功！往返延迟: {modalTestResult.latency_ms}ms
                </Typography.Text>
              </Space>
            ) : (
              <Space align="start">
                <CloseCircleOutlined style={{ color: '#ff4d4f', marginTop: 3 }} />
                <Typography.Text type="danger">
                  连通失败: {modalTestResult.error}
                </Typography.Text>
              </Space>
            )}
          </div>
        )}

        <div style={{ marginTop: 16, display: 'flex', justifyContent: 'flex-start' }}>
          <Button
            icon={<ThunderboltOutlined />}
            loading={modalTestResult?.loading}
            onClick={() => void runModalTest()}
          >
            ⚡ 测试当前填写的配置
          </Button>
        </div>
      </Modal>

      {/* 添加 / 编辑模型弹窗 */}
      <Modal
        title={modelModalMode === 'create' ? '添加模型' : '编辑模型'}
        open={modelModalVisible}
        onOk={() => void handleSaveModel()}
        onCancel={() => setModelModalVisible(false)}
        okText="保存"
        cancelText="取消"
        width={520}
      >
        {modelModalMode === 'create' && (
          <div style={{ marginBottom: 16 }}>
            <Typography.Text type="secondary" style={{ fontSize: 13, display: 'block', marginBottom: 8 }}>
              常用推荐模型（点击一键填入）：
            </Typography.Text>
            <Space wrap size={[6, 8]}>
              {PRESETS.map((preset) => (
                <Button
                  key={preset.name}
                  size="small"
                  onClick={() => applyPresetModel(preset)}
                >
                  {preset.defaultDisplayName}
                </Button>
              ))}
            </Space>
          </div>
        )}

        <Divider style={{ margin: '12px 0' }} />

        <Form form={modelForm} layout="vertical" autoComplete="off">
          <Form.Item
            name="model_name"
            label="模型标识 (Model Identifier)"
            rules={[{ required: true, message: '请输入模型标识' }]}
            help="请求 API 时传递的实际模型名称，如 Atria-Dawn-Preview / gpt-4o / deepseek-chat"
          >
            <Input
              autoComplete="off"
              placeholder="如 Atria-Dawn-Preview"
              onChange={(e) => setCurrentModelNameInput(e.target.value)}
            />
          </Form.Item>

          {isLikelyApiKey(currentModelNameInput) && (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 16 }}
              message="注意：您输入的内容似乎是 API 密钥！"
              description="模型标识是告诉接口要调用的模型名称（例如 Atria-Dawn-Preview），而不是密钥。API 密钥请在供应商的「配置凭据」中填写。"
            />
          )}

          <Form.Item
            name="display_name"
            label="展示名称 (Display Name)"
            rules={[{ required: true, message: '请输入展示名称' }]}
            help="界面中显示的友好名称，如 Atria Dawn Preview"
          >
            <Input autoComplete="off" placeholder="如 Atria Dawn Preview" />
          </Form.Item>

          <Form.Item
            name="is_default"
            label="设为全局默认模型"
            valuePropName="checked"
            help="启用后将自动作为家庭所有成员的主对话模型"
          >
            <Switch checkedChildren="默认" unCheckedChildren="普通" />
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
