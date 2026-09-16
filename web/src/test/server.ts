import { setupServer } from 'msw/node'

import { handlers } from './handlers'

/**
 * MSW 服务端：所有 API mock 在此注册（CONVENTIONS-frontend §7）。
 * 关键交互（撤销/危险确认/模型切换）的用例往 handlers 里加。
 */
export const server = setupServer(...handlers)
