import { z } from 'zod'

const platformUserSchema = z.object({
  id: z.string().regex(/^[1-9]\d*$/),
  nickname: z.string().nullable(),
  avatarUrl: z.string().nullable(),
  phone: z.string().nullable(),
  status: z.enum(['enabled', 'disabled']),
  tenantCount: z.number().int().nonnegative(),
  createdAt: z.string(),
})

const platformUserPageSchema = z.object({
  items: z.array(platformUserSchema),
  page: z.number().int().positive(),
  pageSize: z.number().int().min(1).max(100),
  total: z.number().int().nonnegative(),
})

/** parsePlatformUserPage 校验平台用户试点接口的关键运行时响应。 */
export function parsePlatformUserPage(value: unknown) {
  return platformUserPageSchema.parse(value)
}
