import type { WorkspaceType } from '@/types/auth'
import type { EntityStatus } from '@/types/rbac'
import { requestAdminJSON } from '@/services/http'

/** 员工列表中展示的角色摘要。 */
export interface PlatformEmployeeRole {
  /** 角色唯一标识。 */
  id: string
  /** 角色显示名称。 */
  name: string
  /** 当前操作者是否允许分配或移除该角色。 */
  assignable: boolean
}

/** 员工管理列表使用的员工详情。 */
export interface PlatformEmployee {
  /** 员工唯一标识。 */
  id: string
  /** 员工显示名称。 */
  name: string
  /** 员工登录账号。 */
  loginAccount: string
  /** 员工所属部门；未分配时为 null。 */
  department: {
    /** 部门唯一标识。 */
    id: string
    /** 部门显示名称。 */
    name: string
  } | null
  /** 员工当前关联的角色摘要。 */
  roles: PlatformEmployeeRole[]
  /** 员工联系电话；未填写时为 null。 */
  phone: string | null
  /** 员工账号当前是否可用。 */
  status: EntityStatus
  /** 员工创建时间。 */
  createdAt: string
}

/** 员工筛选和编辑表单使用的轻量选项。 */
export interface PlatformEmployeeOption {
  /** 角色或部门唯一标识。 */
  id: string
  /** 角色或部门显示名称。 */
  name: string
  /** 选项对应实体当前是否启用。 */
  status: EntityStatus
  /** 当前操作者是否允许把该角色分配给员工；部门选项固定为 false。 */
  assignable: boolean
}

/** 员工页面一次性加载的角色和部门选项。 */
export interface PlatformEmployeeOptions {
  /** 可供筛选或分配的角色。 */
  roles: PlatformEmployeeOption[]
  /** 可供筛选或归属的部门。 */
  departments: PlatformEmployeeOption[]
}

/** 员工列表分页与筛选条件。 */
export interface PlatformEmployeeQuery {
  /** 请求页码。 */
  page: number
  /** 每页记录数。 */
  pageSize: number
  /** 按员工名称模糊搜索。 */
  name?: string
  /** 按登录账号模糊搜索。 */
  loginAccount?: string
  /** 筛选指定部门员工。 */
  departmentId?: string
  /** 筛选关联指定角色的员工。 */
  roleId?: string
  /** 筛选员工启停状态。 */
  status?: EntityStatus
}

/** 员工列表分页结果。 */
export interface PlatformEmployeePage {
  /** 当前页员工记录。 */
  items: PlatformEmployee[]
  /** 当前页码。 */
  page: number
  /** 每页记录数。 */
  pageSize: number
  /** 符合条件的员工总数。 */
  total: number
}

/** 新增或修改员工时提交的业务字段。 */
export interface EmployeeMutation {
  /** 员工显示名称。 */
  name: string
  /** 员工登录账号。 */
  loginAccount: string
  /** 新建或重置时提交的密码；普通编辑可不提供。 */
  password?: string
  /** 可选联系电话。 */
  phone?: string
  /** 所属部门标识；未分配部门时不提供。 */
  departmentId?: string
  /** 需要关联的角色标识。 */
  roleIds?: string[]
  /** 员工启停状态。 */
  status?: EntityStatus
}

/** 移除查询对象中的空值，避免后端将空字符串解释为有效筛选条件。 */
function compactParams<T extends object>(params: T): Partial<T> {
  return Object.fromEntries(
    Object.entries(params).filter(
      ([, value]) => value !== undefined && value !== '',
    ),
  ) as Partial<T>
}

/** fetchPlatformEmployees 按服务端分页和筛选条件读取平台员工。 */
export function fetchPlatformEmployees(
  query: PlatformEmployeeQuery,
  signal?: AbortSignal,
): Promise<PlatformEmployeePage> {
  return requestAdminJSON<PlatformEmployeePage>(
    '/api/admin/platform/employees',
    {
      params: compactParams(query),
      signal,
    },
  )
}

/** fetchWorkspaceEmployees 读取平台或当前租户员工列表。 */
export function fetchWorkspaceEmployees(
  workspace: WorkspaceType,
  query: PlatformEmployeeQuery,
  signal?: AbortSignal,
): Promise<PlatformEmployeePage> {
  return requestAdminJSON<PlatformEmployeePage>(
    `/api/admin/${workspace}/employees`,
    {
      params: compactParams(query),
      signal,
    },
  )
}

/** fetchPlatformEmployeeOptions 读取平台员工筛选用的角色与部门。 */
export function fetchPlatformEmployeeOptions(
  signal?: AbortSignal,
): Promise<PlatformEmployeeOptions> {
  return requestAdminJSON<PlatformEmployeeOptions>(
    '/api/admin/platform/employees/options',
    {
      signal,
    },
  )
}

/** fetchWorkspaceEmployeeOptions 读取平台或当前租户员工选项。 */
export function fetchWorkspaceEmployeeOptions(
  workspace: WorkspaceType,
  signal?: AbortSignal,
): Promise<PlatformEmployeeOptions> {
  return requestAdminJSON<PlatformEmployeeOptions>(
    `/api/admin/${workspace}/employees/options`,
    { signal },
  )
}

/** createWorkspaceEmployee 创建平台或当前租户员工。 */
export function createWorkspaceEmployee(
  workspace: WorkspaceType,
  input: EmployeeMutation,
) {
  return requestAdminJSON<null>(`/api/admin/${workspace}/employees`, {
    method: 'POST',
    data: input,
  })
}

/** updateWorkspaceEmployee 更新员工基本资料。 */
export function updateWorkspaceEmployee(
  workspace: WorkspaceType,
  employeeId: string,
  input: EmployeeMutation,
) {
  return requestAdminJSON<null>(
    `/api/admin/${workspace}/employees/${employeeId}`,
    { method: 'PATCH', data: input },
  )
}

/** assignWorkspaceEmployeeRoles 替换员工角色。 */
export function assignWorkspaceEmployeeRoles(
  workspace: WorkspaceType,
  employeeId: string,
  roleIds: string[],
) {
  return requestAdminJSON<null>(
    `/api/admin/${workspace}/employees/${employeeId}/roles`,
    { method: 'PUT', data: { roleIds } },
  )
}

/** resetWorkspaceEmployeePassword 重置员工密码。 */
export function resetWorkspaceEmployeePassword(
  workspace: WorkspaceType,
  employeeId: string,
  password: string,
) {
  return requestAdminJSON<null>(
    `/api/admin/${workspace}/employees/${employeeId}/password`,
    { method: 'PUT', data: { password } },
  )
}

/** setWorkspaceEmployeeStatus 更新员工状态。 */
export function setWorkspaceEmployeeStatus(
  workspace: WorkspaceType,
  employeeId: string,
  status: EntityStatus,
) {
  return requestAdminJSON<null>(
    `/api/admin/${workspace}/employees/${employeeId}/status`,
    { method: 'PATCH', data: { status } },
  )
}

/** deleteWorkspaceEmployee 删除已停用且无业务引用的普通员工。 */
export function deleteWorkspaceEmployee(
  workspace: WorkspaceType,
  employeeId: string,
) {
  return requestAdminJSON<null>(
    `/api/admin/${workspace}/employees/${employeeId}`,
    { method: 'DELETE' },
  )
}
