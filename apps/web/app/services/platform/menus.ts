import type { WorkspaceType } from '@/types/auth'
import type { EntityStatus, MenuNode, MenuNodeType } from '@/types/rbac'
import { requestAdminJSON } from '@/services/http'

/** 新增或修改平台统一菜单节点时提交的字段。 */
export interface MenuMutation {
  /** 菜单节点生效的平台端或租户端范围。 */
  scope: WorkspaceType
  /** 上级节点标识；根节点不提供。 */
  parentId?: string
  /** 节点显示名称。 */
  name: string
  /** 节点为目录、页面菜单或操作权限。 */
  type: MenuNodeType
  /** 页面菜单对应的前端路由。 */
  path?: string
  /** 页面菜单关联的组件标识。 */
  component?: string
  /** 菜单展示使用的 Ant Design 图标名称。 */
  icon?: string
  /** 页面或操作权限校验使用的稳定编码。 */
  permissionCode?: string
  /** 平台节点是否允许分配给租户角色。 */
  tenantAssignable: boolean
  /** 同级节点显示顺序。 */
  sort: number
  /** 节点是否在导航中显示。 */
  visible: boolean
  /** 节点启停状态。 */
  status: EntityStatus
}

/** 菜单接口直接返回的可空字段结构。 */
export interface PlatformMenuWire {
  /** 节点唯一标识。 */
  id: string
  /** 上级节点标识；根节点为 null。 */
  parentId: string | null
  /** 节点显示名称。 */
  name: string
  /** 节点业务类型。 */
  type: MenuNode['type']
  /** 节点所属工作空间范围。 */
  scope: WorkspaceType
  /** 页面路由；不适用时为 null。 */
  path: string | null
  /** 组件标识；不适用时为 null。 */
  component: string | null
  /** 图标名称；未配置时为 null。 */
  icon: string | null
  /** 权限编码；未配置时为 null。 */
  permissionCode: string | null
  /** 是否允许租户角色分配该节点。 */
  tenantAssignable: boolean
  /** 同级节点显示顺序。 */
  sort: number
  /** 是否显示在导航菜单中。 */
  visible: boolean
  /** 节点启停状态。 */
  status: MenuNode['status']
}

/** 菜单列表接口响应内容。 */
interface PlatformMenuListWire {
  /** 服务端返回的扁平菜单节点。 */
  items: PlatformMenuWire[]
}

/** normalizePlatformMenus 将接口 null 字段转换为现有菜单组件使用的可选字段。 */
export function normalizePlatformMenus(items: PlatformMenuWire[]): MenuNode[] {
  return items.map(item => ({
    id: item.id,
    parentId: item.parentId ?? undefined,
    name: item.name,
    type: item.type,
    scope: item.scope,
    path: item.path ?? undefined,
    component: item.component ?? undefined,
    icon: item.icon ?? undefined,
    permissionCode: item.permissionCode ?? undefined,
    tenantAssignable: item.tenantAssignable,
    sort: item.sort,
    visible: item.visible,
    status: item.status,
  }))
}

/** fetchPlatformMenus 读取平台统一维护的指定范围菜单节点。 */
export async function fetchPlatformMenus(
  scope: WorkspaceType,
  signal?: AbortSignal,
): Promise<MenuNode[]> {
  const response = await requestAdminJSON<PlatformMenuListWire>(
    '/api/admin/platform/menus',
    {
      params: { scope },
      signal,
    },
  )
  return normalizePlatformMenus(response.items)
}

/** fetchWorkspaceMenus 读取平台或当前租户可见的菜单定义。 */
export async function fetchWorkspaceMenus(
  workspace: WorkspaceType,
  signal?: AbortSignal,
): Promise<MenuNode[]> {
  if (workspace === 'platform')
    return fetchPlatformMenus('platform', signal)
  const response = await requestAdminJSON<PlatformMenuListWire>(
    '/api/admin/tenant/menus',
    { signal },
  )
  return normalizePlatformMenus(response.items)
}

/** createPlatformMenu 创建平台统一维护的菜单节点。 */
export function createPlatformMenu(input: MenuMutation) {
  return requestAdminJSON<null>('/api/admin/platform/menus', {
    method: 'POST',
    data: input,
  })
}

/** updatePlatformMenu 更新菜单节点的可编辑属性。 */
export function updatePlatformMenu(id: string, input: MenuMutation) {
  return requestAdminJSON<null>(`/api/admin/platform/menus/${id}`, {
    method: 'PATCH',
    data: input,
  })
}

/** setPlatformMenuStatus 启用或停用菜单节点。 */
export function setPlatformMenuStatus(id: string, status: EntityStatus) {
  return requestAdminJSON<null>(`/api/admin/platform/menus/${id}/status`, {
    method: 'PATCH',
    data: { status },
  })
}

/** deletePlatformMenu 保守删除无子节点且无角色关联的菜单节点。 */
export function deletePlatformMenu(id: string) {
  return requestAdminJSON<null>(`/api/admin/platform/menus/${id}`, {
    method: 'DELETE',
  })
}
