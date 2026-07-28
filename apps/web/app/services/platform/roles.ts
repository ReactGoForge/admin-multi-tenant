import type { PlatformEmployeePage } from '@/services/platform/employees'
import type { PlatformMenuWire } from '@/services/platform/menus'
import type { WorkspaceType } from '@/types/auth'
import type { EntityStatus, MenuNode, RoleType } from '@/types/rbac'
import { requestAdminJSON } from '@/services/http'
import {
  normalizePlatformMenus,

} from '@/services/platform/menus'

/** 角色管理列表使用的角色摘要。 */
export interface PlatformRole {
  /** 角色唯一标识。 */
  id: string
  /** 角色显示名称。 */
  name: string
  /** 角色用途说明；未填写时为 null。 */
  description: string | null
  /** 角色为系统内置或业务自定义。 */
  type: RoleType
  /** 系统角色稳定键；自定义角色为 null。 */
  systemKey: string | null
  /** 角色当前是否可用。 */
  status: EntityStatus
  /** 当前关联该角色的员工数量。 */
  employeeCount: number
  /** 当前角色拥有的有效权限数量。 */
  permissionCount: number
  /** 当前操作者是否允许修改该角色的权限集合。 */
  permissionConfigurable: boolean
  /** 角色创建时间。 */
  createdAt: string
}

/** 角色列表分页和筛选条件。 */
export interface PlatformRoleQuery {
  /** 请求页码。 */
  page: number
  /** 每页记录数。 */
  pageSize: number
  /** 按角色名称模糊搜索。 */
  name?: string
  /** 筛选系统或自定义角色。 */
  type?: RoleType
  /** 筛选角色启停状态。 */
  status?: EntityStatus
}

/** 角色列表分页结果。 */
export interface PlatformRolePage {
  /** 当前页角色记录。 */
  items: PlatformRole[]
  /** 当前页码。 */
  page: number
  /** 每页记录数。 */
  pageSize: number
  /** 符合条件的角色总数。 */
  total: number
}

/** 角色编辑和权限抽屉使用的完整角色详情。 */
export interface PlatformRoleDetail {
  /** 角色基础信息。 */
  role: PlatformRole
  /** 普通工作空间角色已选权限标识。 */
  permissionIds?: string[]
  /** 普通工作空间角色可配置的权限节点。 */
  menus?: MenuNode[]
  /** 平台系统角色已选的平台端权限标识。 */
  platformPermissionIds?: string[]
  /** 平台系统角色已选的租户端权限标识。 */
  tenantPermissionIds?: string[]
  /** 平台系统角色可配置的平台端权限节点。 */
  platformMenus?: MenuNode[]
  /** 平台系统角色可配置的租户端权限节点。 */
  tenantMenus?: MenuNode[]
}

/** 新增角色或分步骤修改角色时提交的字段集合。 */
export interface RoleMutation {
  /** 角色显示名称。 */
  name: string
  /** 可选的角色用途说明。 */
  description?: string
  /** 普通工作空间角色的权限标识。 */
  permissionIds?: string[]
  /** 平台系统角色的平台端权限标识。 */
  platformPermissionIds?: string[]
  /** 平台系统角色的租户端权限标识。 */
  tenantPermissionIds?: string[]
  /** 角色启停状态。 */
  status: EntityStatus
}

/** 角色详情接口返回的未规范化菜单结构。 */
type PlatformRoleDetailWire = Omit<
  PlatformRoleDetail,
  'menus' | 'platformMenus' | 'tenantMenus'
> & {
  /** 普通角色的原始权限节点。 */
  menus?: PlatformMenuWire[]
  /** 平台角色的原始平台权限节点。 */
  platformMenus?: PlatformMenuWire[]
  /** 平台角色的原始租户权限节点。 */
  tenantMenus?: PlatformMenuWire[]
}

/** 将角色详情中的服务端菜单字段规范化为前端 MenuNode。 */
function normalizeRoleDetail(
  response: PlatformRoleDetailWire,
): PlatformRoleDetail {
  return {
    ...response,
    menus: response.menus ? normalizePlatformMenus(response.menus) : undefined,
    platformMenus: response.platformMenus
      ? normalizePlatformMenus(response.platformMenus)
      : undefined,
    tenantMenus: response.tenantMenus
      ? normalizePlatformMenus(response.tenantMenus)
      : undefined,
  }
}

/** 移除查询对象中的空值，避免后端收到无效筛选条件。 */
function compactParams<T extends object>(params: T): Partial<T> {
  return Object.fromEntries(
    Object.entries(params).filter(
      ([, value]) => value !== undefined && value !== '',
    ),
  ) as Partial<T>
}

/** fetchPlatformRoles 按服务端分页和筛选条件读取平台角色。 */
export function fetchPlatformRoles(
  query: PlatformRoleQuery,
  signal?: AbortSignal,
): Promise<PlatformRolePage> {
  return requestAdminJSON<PlatformRolePage>('/api/admin/platform/roles', {
    params: compactParams(query),
    signal,
  })
}

/** fetchWorkspaceRoles 读取平台或当前租户角色列表。 */
export function fetchWorkspaceRoles(
  workspace: WorkspaceType,
  query: PlatformRoleQuery,
  signal?: AbortSignal,
): Promise<PlatformRolePage> {
  return requestAdminJSON<PlatformRolePage>(`/api/admin/${workspace}/roles`, {
    params: compactParams(query),
    signal,
  })
}

/** fetchPlatformRoleDetail 读取平台角色的有效权限和只读权限树节点。 */
export async function fetchPlatformRoleDetail(
  roleId: string,
  signal?: AbortSignal,
): Promise<PlatformRoleDetail> {
  const response = await requestAdminJSON<PlatformRoleDetailWire>(
    `/api/admin/platform/roles/${roleId}`,
    {
      signal,
    },
  )
  return normalizeRoleDetail(response)
}

/** fetchWorkspaceRoleDetail 读取平台或当前租户角色详情。 */
export async function fetchWorkspaceRoleDetail(
  workspace: WorkspaceType,
  roleId: string,
  signal?: AbortSignal,
): Promise<PlatformRoleDetail> {
  const response = await requestAdminJSON<PlatformRoleDetailWire>(
    `/api/admin/${workspace}/roles/${roleId}`,
    { signal },
  )
  return normalizeRoleDetail(response)
}

/** fetchPlatformRolePermissionOptions 读取新增平台角色使用的双权限树。 */
export async function fetchPlatformRolePermissionOptions() {
  const response = await requestAdminJSON<{
    /** 可分配的平台端原始权限节点。 */
    platformMenus: PlatformMenuWire[]
    /** 可分配的租户端原始权限节点。 */
    tenantMenus: PlatformMenuWire[]
  }>('/api/admin/platform/roles/permission-options')
  return {
    platformMenus: normalizePlatformMenus(response.platformMenus),
    tenantMenus: normalizePlatformMenus(response.tenantMenus),
  }
}

/** fetchPlatformRoleEmployees 分页读取指定平台角色的关联员工。 */
export function fetchPlatformRoleEmployees(
  roleId: string,
  page: number,
  pageSize: number,
  signal?: AbortSignal,
): Promise<PlatformEmployeePage> {
  return requestAdminJSON<PlatformEmployeePage>(
    `/api/admin/platform/roles/${roleId}/employees`,
    {
      params: { page, pageSize },
      signal,
    },
  )
}

/** fetchWorkspaceRoleEmployees 读取角色关联员工。 */
export function fetchWorkspaceRoleEmployees(
  workspace: WorkspaceType,
  roleId: string,
  page: number,
  pageSize: number,
  signal?: AbortSignal,
): Promise<PlatformEmployeePage> {
  return requestAdminJSON<PlatformEmployeePage>(
    `/api/admin/${workspace}/roles/${roleId}/employees`,
    { params: { page, pageSize }, signal },
  )
}

/** createWorkspaceRole 创建自定义角色。 */
export function createWorkspaceRole(
  workspace: WorkspaceType,
  input: RoleMutation,
) {
  return requestAdminJSON<null>(`/api/admin/${workspace}/roles`, {
    method: 'POST',
    data: input,
  })
}
/** updateWorkspaceRole 更新自定义角色基本信息。 */
export function updateWorkspaceRole(
  workspace: WorkspaceType,
  roleId: string,
  input: Pick<RoleMutation, 'name' | 'description'>,
) {
  return requestAdminJSON<null>(`/api/admin/${workspace}/roles/${roleId}`, {
    method: 'PATCH',
    data: input,
  })
}
/** assignWorkspaceRolePermissions 替换自定义角色权限。 */
export function assignWorkspaceRolePermissions(
  workspace: WorkspaceType,
  roleId: string,
  input: Pick<
    RoleMutation,
    'permissionIds' | 'platformPermissionIds' | 'tenantPermissionIds'
  >,
) {
  return requestAdminJSON<null>(
    `/api/admin/${workspace}/roles/${roleId}/permissions`,
    { method: 'PUT', data: input },
  )
}
/** setWorkspaceRoleStatus 更新自定义角色状态。 */
export function setWorkspaceRoleStatus(
  workspace: WorkspaceType,
  roleId: string,
  status: EntityStatus,
) {
  return requestAdminJSON<null>(
    `/api/admin/${workspace}/roles/${roleId}/status`,
    { method: 'PATCH', data: { status } },
  )
}
/** deleteWorkspaceRole 删除无员工关联的自定义角色。 */
export function deleteWorkspaceRole(workspace: WorkspaceType, roleId: string) {
  return requestAdminJSON<null>(`/api/admin/${workspace}/roles/${roleId}`, {
    method: 'DELETE',
  })
}
