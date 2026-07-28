import type { MenuNode } from '@/types/rbac'

/**
 * 根据当前路径查找菜单节点及其父级分区，生成页面标题和菜单选中信息。
 * 路由未登记在菜单中时返回安全的默认标题。
 */
export function getRouteMetaByPath(pathname: string, menus: MenuNode[]) {
  const routeNode = menus.find(
    node => node.type === 'menu' && node.path === pathname,
  )
  const parentNode = routeNode?.parentId
    ? menus.find(node => node.id === routeNode.parentId)
    : undefined

  return {
    key: routeNode?.id,
    title:
      routeNode?.name
      ?? (pathname === '/platform/profile' || pathname === '/tenant/profile'
        ? '个人信息'
        : undefined)
      ?? (pathname === '/platform' || pathname === '/tenant' ? '首页' : '页面'),
    section:
      parentNode?.name
      ?? (pathname === '/platform/profile' || pathname === '/tenant/profile'
        ? '账号'
        : undefined),
  }
}

/**
 * 根据当前路由菜单向上查找全部目录祖先，供侧栏在刷新后恢复展开状态。
 * 返回顺序从最外层目录到当前菜单的直接父目录。
 */
export function getMenuAncestorKeys(pathname: string, menus: MenuNode[]) {
  const nodeMap = new Map(menus.map(node => [node.id, node]))
  const ancestorKeys: string[] = []
  let currentNode = menus.find(
    node => node.type === 'menu' && node.path === pathname,
  )

  while (currentNode?.parentId) {
    const parentNode = nodeMap.get(currentNode.parentId)
    if (!parentNode)
      break
    ancestorKeys.unshift(parentNode.id)
    currentNode = parentNode
  }

  return ancestorKeys
}

/** 根据菜单标识从扁平菜单集合中查找对应节点，未找到时返回 undefined。 */
export function getMenuNodeById(id: string, menus: MenuNode[]) {
  return menus.find(node => node.id === id)
}
