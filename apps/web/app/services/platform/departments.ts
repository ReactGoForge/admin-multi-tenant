import type { WorkspaceType } from '@/types/auth'
import type { Department, EntityStatus } from '@/types/rbac'
import { requestAdminJSON } from '@/services/http'

/** 部门接口直接返回的传输结构。 */
interface PlatformDepartmentWire {
  /** 部门唯一标识。 */
  id: string
  /** 上级部门标识；根部门为 null。 */
  parentId: string | null
  /** 部门显示名称。 */
  name: string
  /** 部门负责人摘要；未设置时为 null。 */
  leader: {
    /** 负责人员工标识。 */
    id: string
    /** 负责人显示名称。 */
    name: string
  } | null
  /** 部门员工数量。 */
  employeeCount: number
  /** 同级部门显示顺序。 */
  sort: number
  /** 部门启停状态。 */
  status: EntityStatus
}

/** 前端部门组件统一使用的部门实体别名。 */
export type PlatformDepartment = Department

/** 部门列表接口响应内容。 */
interface PlatformDepartmentListWire {
  /** 服务端返回的扁平部门记录。 */
  items: PlatformDepartmentWire[]
}

/** fetchPlatformDepartments 读取平台部门平铺数据并规范化可选父节点。 */
export async function fetchPlatformDepartments(
  signal?: AbortSignal,
): Promise<PlatformDepartment[]> {
  const response = await requestAdminJSON<PlatformDepartmentListWire>(
    '/api/admin/platform/departments',
    {
      signal,
    },
  )
  return response.items.map(item => ({
    id: item.id,
    parentId: item.parentId ?? undefined,
    name: item.name,
    workspace: 'platform',
    leaderEmployeeId: item.leader?.id,
    leaderName: item.leader?.name,
    employeeCount: item.employeeCount,
    sort: item.sort,
    status: item.status,
  }))
}

/** 新增或修改部门时提交的字段。 */
export interface DepartmentMutation {
  /** 上级部门标识；根部门不提供。 */
  parentId?: string
  /** 部门显示名称。 */
  name: string
  /** 部门负责人对应员工标识；未设置时不提供。 */
  leaderEmployeeId?: string
  /** 同级部门显示顺序。 */
  sort: number
  /** 部门启停状态。 */
  status: EntityStatus
}

/** fetchWorkspaceDepartments 读取平台或当前租户部门。 */
export async function fetchWorkspaceDepartments(
  workspace: WorkspaceType,
  signal?: AbortSignal,
): Promise<PlatformDepartment[]> {
  const response = await requestAdminJSON<PlatformDepartmentListWire>(
    `/api/admin/${workspace}/departments`,
    { signal },
  )
  return response.items.map(item => ({
    id: item.id,
    parentId: item.parentId ?? undefined,
    name: item.name,
    workspace,
    leaderEmployeeId: item.leader?.id,
    leaderName: item.leader?.name,
    employeeCount: item.employeeCount,
    sort: item.sort,
    status: item.status,
  }))
}

/** createWorkspaceDepartment 创建部门。 */
export function createWorkspaceDepartment(
  workspace: WorkspaceType,
  input: DepartmentMutation,
) {
  return requestAdminJSON<null>(`/api/admin/${workspace}/departments`, {
    method: 'POST',
    data: input,
  })
}
/** updateWorkspaceDepartment 更新部门。 */
export function updateWorkspaceDepartment(
  workspace: WorkspaceType,
  id: string,
  input: DepartmentMutation,
) {
  return requestAdminJSON<null>(`/api/admin/${workspace}/departments/${id}`, {
    method: 'PATCH',
    data: input,
  })
}
/** deleteWorkspaceDepartment 删除空部门。 */
export function deleteWorkspaceDepartment(
  workspace: WorkspaceType,
  id: string,
) {
  return requestAdminJSON<null>(`/api/admin/${workspace}/departments/${id}`, {
    method: 'DELETE',
  })
}
