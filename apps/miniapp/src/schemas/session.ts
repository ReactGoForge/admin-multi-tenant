import { z } from 'zod'

// 微信小程序不支持 Zod 对象解析器使用的运行时代码生成，统一改用无 JIT 解析路径。
z.config({ jitless: true })

const identifierSchema = z.string().regex(/^[1-9]\d*$/)

const miniappUserSchema = z.object({
  id: identifierSchema,
  phone: z.string().nullable(),
  nickname: z.string().nullable(),
  avatarUrl: z.string().nullable(),
  status: z.enum(['enabled', 'disabled']),
})

const miniappTenantSchema = z.object({
  id: identifierSchema,
  name: z.string(),
})

export const miniappSessionSchema = z.object({
  accessToken: z.string().min(1),
  expiresAt: z.string().min(1),
  user: miniappUserSchema,
  tenant: miniappTenantSchema,
})

export const currentSessionSchema = z.object({
  user: miniappUserSchema,
  tenant: miniappTenantSchema,
})

export const storedAccessTokenSchema = z.string().min(1)

/** parseTenantScene 将启动参数收敛为可信十进制租户 ID。 */
export function parseTenantScene(value: unknown) {
  const result = identifierSchema.safeParse(String(value ?? '').trim())
  return result.success ? result.data : ''
}
