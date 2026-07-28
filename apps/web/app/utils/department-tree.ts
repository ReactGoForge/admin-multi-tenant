/** buildDepartmentTree 按父节点和排序字段将部门平铺数据转换为树。 */
export function buildDepartmentTree<
  T extends {
    /** 节点唯一标识。 */
    id: string
    /** 上级节点标识；根节点不提供。 */
    parentId?: string
    /** 同级节点显示顺序。 */
    sort: number
    /** 组装后保存的子节点。 */
    children?: T[]
  },
>(items: T[]): T[] {
  const map = new Map(
    items.map(item => [item.id, { ...item, children: [] as T[] } as T]),
  )
  const roots: T[] = []
  for (const item of map.values()) {
    if (item.parentId && map.has(item.parentId)) {
      map.get(item.parentId)?.children?.push(item)
    }
    else {
      roots.push(item)
    }
  }

  /** 递归排序同级部门，并移除叶子节点上的空 children。 */
  const sort = (nodes: T[]) => {
    nodes.sort((left, right) => left.sort - right.sort)
    for (const node of nodes) {
      if (node.children?.length) {
        sort(node.children)
      }
      else {
        delete node.children
      }
    }
  }
  sort(roots)
  return roots
}
