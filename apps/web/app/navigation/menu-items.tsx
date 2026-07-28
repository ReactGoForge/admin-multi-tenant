import type { MenuProps } from 'antd'

import type { AuthContext } from '@/types/auth'
import type { MenuNode } from '@/types/rbac'
import { buildMenuTree, hasPermission } from '@/auth/permission'
import { getMenuIcon } from './route-meta'

/**
 * 将菜单树转换为 Ant Design 菜单配置。
 * 过滤操作权限、禁用节点、隐藏节点和无权访问的页面，并移除没有可见子项的目录。
 */
function toMenuItems(
  nodes: MenuNode[],
  context: AuthContext,
): MenuProps['items'] {
  return nodes.flatMap((node) => {
    if (
      node.type === 'permission'
      || node.status !== 'enabled'
      || !node.visible
    ) {
      return []
    }

    if (node.type === 'menu') {
      if (
        !node.permissionCode
        || !hasPermission(context, node.permissionCode)
      ) {
        return []
      }
      return [
        {
          key: node.id,
          label: node.name,
          icon: getMenuIcon(node.icon),
        },
      ]
    }

    const children = node.children ? toMenuItems(node.children, context) : []
    if (!children?.length) {
      return []
    }
    return [
      {
        key: node.id,
        label: node.name,
        icon: getMenuIcon(node.icon),
        children,
      },
    ]
  })
}

/** 根据当前工作空间和权限上下文生成侧边栏可见菜单项。 */
export function getWorkspaceMenuItems(menus: MenuNode[], context: AuthContext) {
  return toMenuItems(
    buildMenuTree(menus.filter(node => node.scope === context.workspace)),
    context,
  )
}
