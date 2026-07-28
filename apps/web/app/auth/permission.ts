import type {
  AuthContext,
  CurrentSessionUser,
  WorkspaceType,
} from '@/types/auth'
import type { MenuNode } from '@/types/rbac'

/**
 * 判断当前权限上下文是否拥有指定权限编码。
 * 平台超级管理员直接通过；无上下文或普通用户必须在权限集合中精确匹配。
 */
export function hasPermission(context: AuthContext | null, code: string) {
  if (!context) {
    return false
  }

  if (context.workspace === 'platform' && context.isSuperAdmin) {
    return true
  }

  return context.permissions.includes(code)
}

/** 判断业务实体是否属于指定工作空间，并在租户端同时核对租户标识。 */
export function matchesWorkspace(
  item: {
    /** 实体直接声明的工作空间。 */
    workspace?: WorkspaceType
    /** 角色、菜单等实体使用的工作空间范围。 */
    scope?: WorkspaceType
    /** 租户端实体所属的租户标识。 */
    tenantId?: string
  },
  workspace: WorkspaceType,
  tenantId?: string,
) {
  const itemWorkspace = item.workspace ?? item.scope
  return (
    itemWorkspace === workspace
    && (workspace === 'platform' || item.tenantId === tenantId)
  )
}

/**
 * 将服务端返回的扁平菜单节点组装为有序树。
 * 函数复制每个节点，不修改调用方传入的数据；失去父节点的记录按根节点处理。
 */
export function buildMenuTree(nodes: MenuNode[]): MenuNode[] {
  const nodeMap = new Map<string, MenuNode>()
  for (const node of nodes) {
    nodeMap.set(node.id, { ...node, children: [] })
  }

  const roots: MenuNode[] = []
  for (const node of nodeMap.values()) {
    if (node.parentId && nodeMap.has(node.parentId)) {
      nodeMap.get(node.parentId)?.children?.push(node)
    }
    else {
      roots.push(node)
    }
  }

  /** 递归按 sort 排列同级节点，并移除叶子节点上的空 children。 */
  const sortNodes = (items: MenuNode[]) => {
    items.sort((left, right) => left.sort - right.sort)
    for (const item of items) {
      if (item.children?.length) {
        sortNodes(item.children)
      }
      else {
        delete item.children
      }
    }
  }

  sortNodes(roots)
  return roots
}

/**
 * 补齐已选权限节点的全部祖先标识，保证权限树回显时父级目录处于选中状态。
 * 返回去重后的新数组，不修改原始选择集合。
 */
export function normalizePermissionIds(ids: string[], menus: MenuNode[]) {
  const selected = new Set(ids)
  const nodeMap = new Map(menus.map(node => [node.id, node]))

  for (const id of ids) {
    let current = nodeMap.get(id)
    while (current?.parentId) {
      selected.add(current.parentId)
      current = nodeMap.get(current.parentId)
    }
  }

  return [...selected]
}

/** 从当前会话身份提取页面权限判断所需的最小上下文；未登录时返回 null。 */
export function resolveAuthContext(
  session: CurrentSessionUser | null,
): AuthContext | null {
  if (!session) {
    return null
  }

  return {
    workspace: session.workspace,
    mode: session.mode,
    tenantId: session.tenantId ?? undefined,
    isSuperAdmin: session.isSuperAdmin,
    roleIds: session.roles.map(role => role.id),
    permissions: session.permissions,
  }
}

/**
 * 查找当前工作空间中排序最靠前且已授权的可见菜单路由。
 * 未登录时返回登录页，已登录但没有可访问菜单时返回无权限页。
 */
export function getFirstAuthorizedPath(
  context: AuthContext | null,
  menus: MenuNode[],
) {
  if (!context) {
    return '/login'
  }

  const candidates = menus
    .filter(
      node =>
        node.scope === context.workspace
        && node.type === 'menu'
        && node.status === 'enabled'
        && node.visible
        && node.path
        && node.permissionCode
        && hasPermission(context, node.permissionCode),
    )
    .sort((left, right) => left.sort - right.sort)

  return candidates[0]?.path ?? '/403'
}
