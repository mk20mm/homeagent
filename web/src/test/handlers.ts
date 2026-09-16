import { http, HttpResponse } from 'msw'

import type { paths } from '../api/schema'

/**
 * MSW handlers：骨架期先放一个 /health 探活，真实接口 mock 随 C 阶段 handler 补齐。
 * 类型来自生成的 schema（paths），改契约后此处会随之更新。
 */
type Health = paths['/health']['get']['responses']['200']['content']['application/json']

export const handlers = [
  http.get('/api/v1/health', () => HttpResponse.json({ status: 'ok' } satisfies Health)),
]
