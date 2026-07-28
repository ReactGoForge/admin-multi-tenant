import type { EntityStatus } from '@/types/rbac'
import { requestAdminJSON } from '@/services/http'

/** 平台租户管理列表使用的租户信息。 */
export interface PlatformTenant {
  /** 租户唯一标识。 */
  id: string
  /** 租户显示名称。 */
  name: string
  /** 平台内部备注；未填写时为 null。 */
  remark: string | null
  /** 租户品牌图标地址；未配置时为 null。 */
  iconUrl: string | null
  /** 租户当前是否可用。 */
  status: EntityStatus
  /** 租户所有者员工标识；历史异常数据可能为 null。 */
  ownerEmployeeId: string | null
  /** 租户所有者名称；未取得时为 null。 */
  ownerName: string | null
  /** 租户所有者登录账号；未取得时为 null。 */
  loginAccount: string | null
}
/** 租户列表分页结果。 */
export interface PlatformTenantPage {
  /** 当前页租户记录。 */
  items: PlatformTenant[]
  /** 当前页码。 */
  page: number
  /** 每页记录数。 */
  pageSize: number
  /** 符合条件的租户总数。 */
  total: number
}
/** 创建租户及首位所有者时提交的字段。 */
export interface TenantCreateInput {
  /** 租户显示名称。 */
  name: string
  /** 初始所有者姓名。 */
  ownerName: string
  /** 初始所有者登录账号。 */
  loginAccount: string
  /** 初始所有者登录密码。 */
  password: string
}
/** 修改租户基础信息时提交的字段。 */
export interface TenantUpdateInput {
  /** 租户显示名称。 */
  name: string
  /** 租户所有者登录账号。 */
  loginAccount: string
  /** 可选的平台内部备注。 */
  remark?: string
}
/** 平台进入租户后获得的代管会话凭据。 */
export interface TenantEnterResult {
  /** 仅用于指定租户的代管访问令牌。 */
  accessToken: string
  /** 代管令牌过期时间。 */
  expiresAt: string
}

/** fetchPlatformTenants 分页读取真实租户。 */
export function fetchPlatformTenants(
  query: {
    /** 请求页码。 */
    page: number
    /** 每页记录数。 */
    pageSize: number
    /** 按租户名称模糊搜索。 */
    name?: string
    /** 筛选租户启停状态。 */
    status?: EntityStatus
  },
  signal?: AbortSignal,
) {
  return requestAdminJSON<PlatformTenantPage>('/api/admin/platform/tenants', {
    params: query,
    signal,
  })
}
/** createPlatformTenant 创建租户及所有者。 */
export function createPlatformTenant(input: TenantCreateInput) {
  return requestAdminJSON<null>('/api/admin/platform/tenants', {
    method: 'POST',
    data: input,
  })
}
/** setPlatformTenantStatus 更新租户状态。 */
export function setPlatformTenantStatus(id: string, status: EntityStatus) {
  return requestAdminJSON<null>(`/api/admin/platform/tenants/${id}/status`, {
    method: 'PATCH',
    data: { status },
  })
}

/** updatePlatformTenant 更新租户名称、所有者登录账号和内部备注。 */
export function updatePlatformTenant(id: string, input: TenantUpdateInput) {
  return requestAdminJSON<null>(`/api/admin/platform/tenants/${id}`, {
    method: 'PATCH',
    data: input,
  })
}

/** resetPlatformTenantOwnerPassword 重置租户所有者密码。 */
export function resetPlatformTenantOwnerPassword(id: string, password: string) {
  return requestAdminJSON<null>(
    `/api/admin/platform/tenants/${id}/owner-password`,
    { method: 'PUT', data: { password } },
  )
}

/** fetchPlatformTenantMiniappCode 读取租户小程序码，缓存未命中时生成。 */
export function fetchPlatformTenantMiniappCode(id: string) {
  return requestAdminJSON<{
    /** 下载小程序码时使用的真实图片扩展名。 */
    extension: 'jpg' | 'png'
    /** 可直接展示的小程序码图片内容。 */
    image: string
  }>(`/api/admin/platform/tenants/${id}/miniapp-code`)
}

/** regeneratePlatformTenantMiniappCode 强制重新生成并覆盖租户小程序码缓存。 */
export function regeneratePlatformTenantMiniappCode(id: string) {
  return requestAdminJSON<{
    /** 下载小程序码时使用的真实图片扩展名。 */
    extension: 'jpg' | 'png'
    /** 可直接展示的小程序码图片内容。 */
    image: string
  }>(`/api/admin/platform/tenants/${id}/miniapp-code`, {
    method: 'POST',
  })
}

/** enterPlatformTenant 换取指定租户的代管 Token。 */
export function enterPlatformTenant(id: string) {
  return requestAdminJSON<TenantEnterResult>(
    `/api/admin/platform/tenants/${id}/enter`,
    { method: 'POST' },
  )
}

/** deletePlatformTenant 删除已停用且未产生业务数据的空租户。 */
export function deletePlatformTenant(id: string) {
  return requestAdminJSON<null>(`/api/admin/platform/tenants/${id}`, {
    method: 'DELETE',
  })
}
