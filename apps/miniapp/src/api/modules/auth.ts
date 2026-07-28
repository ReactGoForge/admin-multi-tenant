import type { components } from '../generated/schema'

import { currentSessionSchema, miniappSessionSchema } from '../../schemas/session'
import { request } from '../client'

/** MiniappUser 描述小程序当前用户公开信息。 */
export type MiniappUser = components['schemas']['MiniappUser']

/** MiniappTenant 描述小程序当前租户公开信息。 */
export type MiniappTenant = components['schemas']['MiniappTenant']

/** MiniappSession 描述登录接口返回的完整会话。 */
export type MiniappSession = components['schemas']['MiniappSession']

export type CurrentSession = components['schemas']['MiniappCurrentSession']

type LoginBody = components['schemas']['MiniappLoginRequest']

/** loginMiniapp 使用微信临时凭证登录指定 scene 对应的租户。 */
export function loginMiniapp(body: LoginBody) {
  return request<MiniappSession, LoginBody>({
    url: '/auth/login',
    method: 'POST',
    data: body,
  }).then(value => miniappSessionSchema.parse(value))
}

/** getCurrentSession 校验本地 Token 并返回当前用户和租户。 */
export function getCurrentSession(accessToken: string) {
  return request<CurrentSession>({
    url: '/me',
    method: 'GET',
    accessToken,
  }).then(value => currentSessionSchema.parse(value))
}
