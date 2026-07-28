import type { components } from '@/services/generated/schema'
import type { EntityStatus } from '@/types/rbac'
import { parsePlatformUserPage } from '@/schemas/platform-users'
import { requestAdminJSON } from '@/services/http'

/** 平台用户管理列表中的全局用户。 */
export type PlatformUser = components['schemas']['PlatformUser']

/** 当前租户用户管理列表中的用户关系。 */
export interface TenantUser {
  /** 平台用户唯一标识。 */
  id: string
  /** 用户昵称；尚未设置时为 null。 */
  nickname: string | null
  /** 用户头像地址；尚未设置时为 null。 */
  avatarUrl: string | null
  /** 用户手机号；尚未绑定时为 null。 */
  phone: string | null
  /** 用户在整个平台范围内的全局状态。 */
  platformStatus: EntityStatus
  /** 用户在当前租户内的独立状态。 */
  tenantStatus: EntityStatus
  /** 用户加入当前租户的时间。 */
  joinedAt: string
}

/** 平台用户列表按租户筛选时使用的租户选项。 */
export interface PlatformUserTenantOption {
  /** 租户唯一标识。 */
  id: string
  /** 租户显示名称。 */
  name: string
  /** 租户当前启停状态。 */
  status: EntityStatus
}

/** 平台用户关联的一条租户归属。 */
export interface PlatformUserTenant {
  /** 租户唯一标识。 */
  tenantId: string
  /** 租户显示名称。 */
  tenantName: string
  /** 租户当前启停状态。 */
  tenantStatus: EntityStatus
  /** 用户在该租户内的启停状态。 */
  userStatus: EntityStatus
  /** 用户加入该租户的时间。 */
  joinedAt: string
}

/** 用户列表接口的通用分页结果。 */
interface PageResult<T> {
  /** 当前页用户记录。 */
  items: T[]
  /** 当前页码。 */
  page: number
  /** 每页记录数。 */
  pageSize: number
  /** 符合条件的用户总数。 */
  total: number
}

/** 平台用户列表查询条件。 */
export interface PlatformUserQuery {
  /** 请求页码。 */
  page: number
  /** 每页记录数。 */
  pageSize: number
  /** 按用户昵称模糊搜索。 */
  nickname?: string
  /** 按用户手机号模糊搜索。 */
  phone?: string
  /** 筛选已加入指定租户的用户。 */
  tenantId?: string
  /** 筛选用户全局启停状态。 */
  status?: EntityStatus
}

/** 当前租户用户列表查询条件。 */
export interface TenantUserQuery {
  /** 请求页码。 */
  page: number
  /** 每页记录数。 */
  pageSize: number
  /** 按用户昵称模糊搜索。 */
  nickname?: string
  /** 按用户手机号模糊搜索。 */
  phone?: string
  /** 筛选用户在当前租户内的启停状态。 */
  status?: EntityStatus
}

/** fetchPlatformUsers 分页查询平台唯一用户。 */
export function fetchPlatformUsers(
  query: PlatformUserQuery,
  signal?: AbortSignal,
) {
  return requestAdminJSON<components['schemas']['PlatformUserPage']>(
    '/api/admin/platform/users',
    { params: query, signal },
  ).then(parsePlatformUserPage)
}

/** fetchPlatformUserTenantOptions 查询平台用户筛选使用的全部租户选项。 */
export function fetchPlatformUserTenantOptions(signal?: AbortSignal) {
  return requestAdminJSON<PlatformUserTenantOption[]>(
    '/api/admin/platform/users/tenant-options',
    { signal },
  )
}

/** fetchPlatformUserTenants 查询指定平台用户关联的全部租户。 */
export function fetchPlatformUserTenants(id: string, signal?: AbortSignal) {
  return requestAdminJSON<PlatformUserTenant[]>(
    `/api/admin/platform/users/${id}/tenants`,
    { signal },
  )
}

/** setPlatformUserStatus 更新平台用户的全局状态。 */
export function setPlatformUserStatus(id: string, status: EntityStatus) {
  return requestAdminJSON<null>(`/api/admin/platform/users/${id}/status`, {
    method: 'PATCH',
    data: { status },
  })
}

/** fetchTenantUsers 分页查询当前租户的用户。 */
export function fetchTenantUsers(query: TenantUserQuery, signal?: AbortSignal) {
  return requestAdminJSON<PageResult<TenantUser>>('/api/admin/tenant/users', {
    params: query,
    signal,
  })
}

/** setTenantUserStatus 更新用户在当前租户内的状态。 */
export function setTenantUserStatus(id: string, status: EntityStatus) {
  return requestAdminJSON<null>(`/api/admin/tenant/users/${id}/status`, {
    method: 'PATCH',
    data: { status },
  })
}
