import { expect, test } from '@playwright/test'
import { login, randomAmount, waitForStreamDone } from './helpers'

test.describe('模型管理与 Grok x Apple 胶囊切换', () => {
  test('模型胶囊下拉切换与流式对话', async ({ page }) => {
    await login(page)

    // 1. 验证模型胶囊按钮存在并展示默认模型
    const modelPill = page.getByRole('button', { name: '切换模型' })
    await expect(modelPill).toBeVisible()
    await expect(modelPill).toContainText('DeepSeek 对话')

    // 2. 点击模型胶囊展开下拉菜单
    await modelPill.click()
    const dropdown = page.getByRole('listbox')
    await expect(dropdown).toBeVisible()

    // 列表中应包含已启用的模型
    const gpt4Option = dropdown.getByRole('option', { name: /GPT-4o/ })
    await expect(gpt4Option).toBeVisible()

    // 3. 点击切换到 GPT-4o
    await gpt4Option.click()
    await expect(dropdown).toBeHidden()
    await expect(modelPill).toContainText('GPT-4o')

    // 4. 发送一条消息，验证选定模型下的对话流闭环
    const amount = randomAmount()
    await page.getByPlaceholder('输入消息…').fill(`e2e测试动态模型买咖啡花了 ${amount}`)
    await page.getByRole('button', { name: '发送' }).click()

    await waitForStreamDone(page, 60_000)
    await expect(page.getByPlaceholder('输入消息…')).toHaveValue('')
  })

  test('通过管理端接口测试探针、新增与删除模型联动', async ({ request }) => {
    // 1. 爸爸身份获取 JWT
    const tokenRes = await request.post('/api/v1/auth/token', {
      data: { name: '爸爸', auth_token: 'dev-baba' },
    })
    expect(tokenRes.ok()).toBeTruthy()
    const { token } = await tokenRes.json()
    const headers = { Authorization: `Bearer ${token}` }

    // 2. 查询供应商列表获取 DeepSeek 供应商 ID
    const provRes = await request.get('/api/v1/admin/providers', { headers })
    expect(provRes.ok()).toBeTruthy()
    const { providers } = await provRes.json()
    const dsProvider = providers.find((p: { name: string }) => p.name === 'deepseek')
    expect(dsProvider).toBeDefined()

    // 3. 测试探针端点 POST /admin/providers/{id}/test
    const testRes = await request.post(`/api/v1/admin/providers/${dsProvider.id}/test`, { headers })
    expect(testRes.ok()).toBeTruthy()
    const testResult = await testRes.json()
    expect(typeof testResult.success).toBe('boolean')
    expect(typeof testResult.latency_ms).toBe('number')

    // 4. 添加新模型 POST /admin/models
    const createRes = await request.post('/api/v1/admin/models', {
      headers,
      data: {
        provider_id: dsProvider.id,
        model_name: 'deepseek-coder-test',
        display_name: 'DeepSeek 代码测试版',
        is_default: false,
      },
    })
    expect(createRes.ok()).toBeTruthy()
    const createdModel = await createRes.json()
    expect(createdModel.id).toBeDefined()
    expect(createdModel.model_name).toBe('deepseek-coder-test')

    // 5. 验证模型在 GET /models 中可用
    const listRes = await request.get('/api/v1/models', { headers })
    expect(listRes.ok()).toBeTruthy()
    const { models } = await listRes.json()
    const found = models.find((m: { id: string }) => m.id === createdModel.id)
    expect(found).toBeDefined()
    expect(found.display_name).toBe('DeepSeek 代码测试版')

    // 6. 删除刚创建的模型 DELETE /admin/models/{id}
    const delRes = await request.delete(`/api/v1/admin/models/${createdModel.id}`, { headers })
    expect(delRes.ok()).toBeTruthy()

    // 7. 再次查询确认已删除
    const listAfterDel = await request.get('/api/v1/models', { headers })
    const { models: modelsAfter } = await listAfterDel.json()
    expect(modelsAfter.find((m: { id: string }) => m.id === createdModel.id)).toBeUndefined()
  })
})
