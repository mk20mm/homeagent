import { Alert, Card, Col, Divider, Row, Table, Tag, Typography } from 'antd'

import { MEMBER_ROLE_LABEL, type MemberRole } from '@homeagent/shared'

const MATRIX: {
  role: MemberRole
  template: string
  expense: string
  task: string
  meal: string
  undo: string
  highRisk: string
  desc: string
}[] = [
  {
    role: 'parent',
    template: 'admin (全量管理)',
    expense: '读写 + 预算配置',
    task: '全量分配 / 指派 / 验收',
    meal: '读写 + 采购单决策',
    undo: '24h 全量撤销',
    highRisk: '允许执行',
    desc: '家庭超级管理员，具备系统最高控制权，可配置供应商、模型与家庭策略。',
  },
  {
    role: 'elder',
    template: 'limited (长辈适老)',
    expense: '日常记账（不可配预算）',
    task: '仅查看关联任务',
    meal: '上报用餐人数与偏好',
    undo: '仅限撤销本人操作',
    highRisk: '需二次显式确认',
    desc: '大字适老、语音友好，支持核心高频生活工具，防误触。',
  },
  {
    role: 'child',
    template: 'child (儿童安全)',
    expense: '只读零花钱账本',
    task: '打卡自己的家务/学习任务',
    meal: '点菜愿望单',
    undo: '不可撤销',
    highRisk: '严格禁止',
    desc: '保护性受限环境，仅开放自主学习生活打卡，隔离财务敏感操作。',
  },
]

const PERMISSION_DETAILS = [
  {
    module: '财务记账 (expense)',
    perms: [
      { key: 'expense.read', name: '查看流水与预算', roles: ['parent', 'elder', 'child'] },
      { key: 'expense.write', name: '记账 / 修正账单', roles: ['parent', 'elder'] },
      { key: 'expense.admin', name: '配置家庭预算与分类', roles: ['parent'] },
    ],
  },
  {
    module: '家务管理 (task)',
    perms: [
      { key: 'task.read', name: '查看家庭任务清单', roles: ['parent', 'elder', 'child'] },
      { key: 'task.write', name: '分配 / 认领 / 打卡', roles: ['parent', 'elder', 'child'] },
      { key: 'task.admin', name: '创建循环模板 / 审核', roles: ['parent'] },
    ],
  },
  {
    module: '用餐上报 (meal)',
    perms: [
      { key: 'meal.read', name: '查看用餐统计与菜谱', roles: ['parent', 'elder', 'child'] },
      { key: 'meal.write', name: '上报用餐 / 提出愿望', roles: ['parent', 'elder', 'child'] },
    ],
  },
  {
    module: '系统与模型 (system)',
    perms: [
      { key: 'system.admin', name: '供应商 / 模型 / 审计全权管理', roles: ['parent'] },
    ],
  },
]

/**
 * 权限与角色管理：
 * - 按照 ADR-005（权限双保险）设计：工具注册表按角色源头过滤（GET /tools 只下发允许的工具） + Runtime 运行时再次拦截。
 * - 明确家长（Parent）、老人（Elder）、小孩（Child）的角色安全边界与操作矩阵。
 */
export function PermissionPage() {
  return (
    <div>
      <Typography.Title level={4}>家庭成员权限管理</Typography.Title>
      <Typography.Paragraph type="secondary">
        基于 ADR-005「权限双保险」模型设计。权限源头过滤保证 Agent 无法获知未授权工具定义，运行时拦截杜绝越权伪造。
      </Typography.Paragraph>

      <Alert
        type="info"
        showIcon
        message="权限双保险机制说明"
        description={
          <div>
            1. <strong>下发过滤</strong>：Agent 启动及每轮会话下发 tools 时，服务端根据 JWT 关联成员的角色模板，在源头直接剔除无权工具。
            <br />
            2. <strong>运行时校验</strong>：工具执行器（Executor）在每次工具调用前再次严格核查上下文 MemberID 的对应权限，拦截越权与注入。
          </div>
        }
        style={{ marginBottom: 24 }}
      />

      <Typography.Title level={5} style={{ marginBottom: 12 }}>
        角色权限矩阵（Matrix）
      </Typography.Title>
      <Card size="small" style={{ marginBottom: 28 }}>
        <Table
          rowKey="role"
          size="middle"
          pagination={false}
          dataSource={MATRIX}
          columns={[
            {
              title: '成员角色',
              dataIndex: 'role',
              render: (v: MemberRole) => (
                <Tag color={v === 'parent' ? 'blue' : v === 'elder' ? 'purple' : 'green'}>
                  {MEMBER_ROLE_LABEL[v]}
                </Tag>
              ),
            },
            { title: '预设模板', dataIndex: 'template' },
            { title: '财务记账', dataIndex: 'expense' },
            { title: '家务管理', dataIndex: 'task' },
            { title: '用餐上报', dataIndex: 'meal' },
            { title: '撤销窗口', dataIndex: 'undo' },
            { title: '高危工具', dataIndex: 'highRisk' },
          ]}
        />
      </Card>

      <Typography.Title level={5} style={{ marginBottom: 12 }}>
        功能模块细粒度权限明细
      </Typography.Title>
      <Row gutter={[16, 16]}>
        {PERMISSION_DETAILS.map((mod) => (
          <Col span={12} key={mod.module}>
            <Card title={mod.module} size="small" style={{ height: '100%' }}>
              {mod.perms.map((p) => (
                <div
                  key={p.key}
                  style={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    padding: '6px 0',
                    borderBottom: '1px solid #f0f0f0',
                  }}
                >
                  <div>
                    <Typography.Text strong style={{ fontSize: 13 }}>
                      {p.name}
                    </Typography.Text>
                    <div style={{ color: '#8e8e93', fontSize: 12 }}>{p.key}</div>
                  </div>
                  <div>
                    {p.roles.map((r) => (
                      <Tag key={r} style={{ margin: 2 }}>
                        {MEMBER_ROLE_LABEL[r as MemberRole]}
                      </Tag>
                    ))}
                  </div>
                </div>
              ))}
            </Card>
          </Col>
        ))}
      </Row>

      <Divider />
      <Typography.Paragraph type="secondary" style={{ margin: 0 }}>
        提示：后续 C 阶段接入完整成员服务后，将开放成员账号邀请、独立令牌轮转与自定义权限策略分配写端点。
      </Typography.Paragraph>
    </div>
  )
}
