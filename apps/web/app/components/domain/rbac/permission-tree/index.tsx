import type { TreeDataNode, TreeProps } from 'antd'
import type { WorkspaceType } from '@/types/auth'
import type { MenuNode } from '@/types/rbac'

import { Button, Input, Space, Tree } from 'antd'
import { useMemo, useState } from 'react'
import { buildMenuTree, normalizePermissionIds } from '@/auth/permission'

/** 权限树组件配置。 */
interface PermissionTreeProps {
  /** 权限树所属工作空间。 */
  workspace: WorkspaceType
  /** 租户上下文标识；由调用页面按当前会话提供。 */
  tenantId?: string
  /** 当前选中的菜单或权限节点 ID。 */
  value?: string[]
  /** 勾选结果变化时触发。 */
  onChange?: (value: string[]) => void
  /** 是否只读展示，默认 false。 */
  readonly?: boolean
  /** 当前会话可见的菜单节点。 */
  menus?: MenuNode[]
}

/** keepAssignable 过滤租户不可分配节点，同时保留包含可分配后代的父级路径。 */
function keepAssignable(
  nodes: MenuNode[],
  workspace: WorkspaceType,
): MenuNode[] {
  return nodes.flatMap((node) => {
    const children = node.children
      ? keepAssignable(node.children, workspace)
      : undefined
    const available = workspace === 'platform' || node.tenantAssignable
    if (!available && !children?.length) {
      return []
    }
    return [{ ...node, children }]
  })
}

/** filterByKeyword 按菜单名称筛选权限树，并保留命中节点的祖先路径。 */
function filterByKeyword(nodes: MenuNode[], keyword: string): MenuNode[] {
  if (!keyword) {
    return nodes
  }
  return nodes.flatMap((node) => {
    const children = node.children
      ? filterByKeyword(node.children, keyword)
      : undefined
    if (!node.name.includes(keyword) && !children?.length) {
      return []
    }
    return [{ ...node, children }]
  })
}

/** toTreeData 将业务菜单节点转换为 Ant Design Tree 数据，并标记不可勾选节点。 */
function toTreeData(
  nodes: MenuNode[],
  workspace: WorkspaceType,
): TreeDataNode[] {
  return nodes.map(node => ({
    key: node.id,
    title: node.name,
    disableCheckbox: workspace === 'tenant' && !node.tenantAssignable,
    children: node.children ? toTreeData(node.children, workspace) : undefined,
  }))
}

/** collectKeys 递归收集权限树中的全部节点 ID。 */
function collectKeys(nodes: MenuNode[]): string[] {
  return nodes.flatMap(node => [
    node.id,
    ...(node.children ? collectKeys(node.children) : []),
  ])
}

/** getCheckState 根据稳定权限 ID 计算 Ant Design Tree 的全选和半选状态。 */
function getCheckState(value: string[], nodes: MenuNode[]) {
  const selected = new Set(value)
  const checked: string[] = []
  const halfChecked: string[] = []
  /** visit 自底向上计算当前节点是否存在选中项以及整棵子树是否全选。 */
  const visit = (
    item: MenuNode,
  ): {
    /** 当前节点或任一后代是否被选中。 */
    any: boolean
    /** 当前节点和全部后代是否完整选中。 */
    full: boolean
  } => {
    if (!item.children?.length) {
      if (selected.has(item.id)) {
        checked.push(item.id)
        return { any: true, full: true }
      }
      return { any: false, full: false }
    }

    const childrenState = item.children.map(visit)
    const anyChild = childrenState.some(state => state.any)
    const allChildrenFull = childrenState.every(state => state.full)
    const selfSelected = selected.has(item.id)
    if (selfSelected && (!anyChild || allChildrenFull)) {
      checked.push(item.id)
    }
    else if (anyChild) {
      halfChecked.push(item.id)
    }
    return {
      any: selfSelected || anyChild,
      full: selfSelected && allChildrenFull,
    }
  }
  for (const node of nodes) {
    visit(node)
  }
  return { checked, halfChecked }
}

/** PermissionTree 按工作空间过滤并展示可分配的菜单权限树。 */
export function PermissionTree({
  workspace,
  value = [],
  onChange,
  readonly = false,
  menus: providedMenus,
}: PermissionTreeProps) {
  // 搜索与展开状态：keyword 只过滤展示树，expandedKeys 控制当前展开节点。
  const menus = providedMenus ?? []
  const [keyword, setKeyword] = useState('')

  // 权限树派生数据：按工作空间和启用状态过滤，再处理租户可分配规则及关键字搜索。
  const scopedMenus = useMemo(
    () =>
      menus.filter(
        node => node.scope === workspace && node.status === 'enabled',
      ),
    [menus, workspace],
  )
  const tree = useMemo(
    () => keepAssignable(buildMenuTree(scopedMenus), workspace),
    [scopedMenus, workspace],
  )
  const filteredTree = useMemo(
    () => filterByKeyword(tree, keyword.trim()),
    [keyword, tree],
  )
  const allKeys = useMemo(() => collectKeys(tree), [tree])
  const [expandedKeys, setExpandedKeys] = useState<React.Key[]>(allKeys)
  // checkState 计算受控值对应的全选、半选状态，treeNodes 提供按 ID 查找节点的平铺索引。
  const checkState = useMemo(() => getCheckState(value, tree), [tree, value])
  const treeNodes = useMemo(() => {
    const result: MenuNode[] = []
    /** visit 以深度优先顺序把菜单树展开为平铺节点。 */
    const visit = (nodes: MenuNode[]) => {
      for (const node of nodes) {
        result.push(node)
        if (node.children) {
          visit(node.children)
        }
      }
    }
    visit(tree)
    return result
  }, [tree])

  /**
   * handleCheck 同步当前节点及其后代权限，并移除只用于分组的目录节点。
   * 最终通过 normalizePermissionIds 补齐必要父级后再回传受控值。
   */
  const handleCheck: TreeProps['onCheck'] = (_checked, info) => {
    const nodeId = String(info.node.key)
    const currentNode = treeNodes.find(node => node.id === nodeId)
    if (!currentNode) {
      return
    }
    const descendantIds = currentNode.children
      ? collectKeys(currentNode.children)
      : []
    const nextValue = new Set(value)
    if (info.checked) {
      nextValue.add(nodeId)
      if (currentNode.type !== 'permission') {
        for (const id of descendantIds) {
          nextValue.add(id)
        }
      }
    }
    else {
      nextValue.delete(nodeId)
      if (currentNode.type !== 'permission') {
        for (const id of descendantIds) {
          nextValue.delete(id)
        }
      }
    }
    for (const node of treeNodes) {
      if (node.type === 'directory') {
        nextValue.delete(node.id)
      }
    }
    onChange?.(normalizePermissionIds([...nextValue], scopedMenus))
  }

  return (
    <div className="space-y-3 rounded-lg border border-slate-200 p-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <Input.Search
          allowClear
          className="max-w-72"
          onChange={event => setKeyword(event.target.value)}
          placeholder="搜索权限"
          value={keyword}
        />
        <Space wrap>
          {!readonly
            ? (
                <>
                  <Button
                    onClick={() =>
                      onChange?.(normalizePermissionIds(allKeys, scopedMenus))}
                    size="small"
                  >
                    全选
                  </Button>
                  <Button onClick={() => onChange?.([])} size="small">
                    取消全选
                  </Button>
                </>
              )
            : null}
          <Button onClick={() => setExpandedKeys(allKeys)} size="small">
            展开全部
          </Button>
          <Button onClick={() => setExpandedKeys([])} size="small">
            折叠全部
          </Button>
        </Space>
      </div>
      <Tree
        blockNode
        checkable
        checkedKeys={checkState}
        checkStrictly
        disabled={readonly}
        expandedKeys={expandedKeys}
        onCheck={handleCheck}
        onExpand={keys => setExpandedKeys(keys)}
        selectable={false}
        treeData={toTreeData(filteredTree, workspace)}
      />
    </div>
  )
}
