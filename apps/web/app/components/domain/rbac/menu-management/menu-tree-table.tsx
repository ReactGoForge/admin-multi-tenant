import type { TableColumnsType, TableProps } from 'antd'
import type { MenuNode } from '@/types/rbac'

import { Button, Space, Tag } from 'antd'
import { ConfirmDelete } from '@/components/base/confirm-delete'
import { StatusSwitch } from '@/components/base/status-switch'
import { Permission } from '@/components/domain/auth/permission'
import { DICTIONARY_CODE, useDictionary } from '@/contexts/dictionary'

/** 菜单树表格数据、展开状态和可选操作配置。 */
interface MenuTreeTableOptions {
  /** 已经构建好父子关系的菜单树。 */
  data: MenuNode[]
  /** 是否允许展示平台菜单写操作。 */
  editable: boolean
  /** 当前展开的菜单节点 ID。 */
  expandedRowKeys: React.Key[]
  /** 展开节点变化时同步给上层组件。 */
  onExpandedRowsChange: (keys: readonly React.Key[]) => void
  /** 点击新增子节点时触发。 */
  onAddChild?: (node: MenuNode) => void
  /** 点击编辑节点时触发。 */
  onEdit?: (node: MenuNode) => void
  /** 用户确认删除节点时触发。 */
  onDelete?: (node: MenuNode) => void
  /** 菜单状态开关变化时触发，并返回目标启用状态。 */
  onStatusChange?: (node: MenuNode, enabled: boolean) => void
  /** 菜单数据加载状态。 */
  loading?: boolean
}

/** isProtectedPlatformMenu 判断节点是否属于防止超级管理员自锁的核心菜单权限。 */
function isProtectedPlatformMenu(node: MenuNode) {
  return (
    node.scope === 'platform'
    && node.permissionCode?.startsWith('platform:menu:')
  )
}

/** useMenuTreeTableProps 生成菜单树表格的列、展开与操作配置。 */
export function useMenuTreeTableProps({
  data,
  editable,
  expandedRowKeys,
  onExpandedRowsChange,
  onAddChild,
  onEdit,
  onDelete,
  onStatusChange,
  loading = false,
}: MenuTreeTableOptions): TableProps<MenuNode> {
  const { getLabel } = useDictionary()
  const columns: TableColumnsType<MenuNode> = [
    { title: '菜单名称', dataIndex: 'name', width: 200, fixed: 'left' },
    {
      title: '节点类型',
      dataIndex: 'type',
      width: 110,
      render: (type: MenuNode['type']) => {
        const colors = {
          directory: 'purple',
          menu: 'blue',
          permission: 'cyan',
        }
        return (
          <Tag color={colors[type]}>
            {getLabel(DICTIONARY_CODE.menuNodeType, type)}
          </Tag>
        )
      },
    },
    {
      title: '路由地址',
      dataIndex: 'path',
      width: 210,
      render: value => value || '-',
    },
    {
      title: '组件路径',
      dataIndex: 'component',
      width: 220,
      render: value => value || '-',
    },
    {
      title: '权限标识',
      dataIndex: 'permissionCode',
      width: 230,
      render: value => value || '-',
    },
    {
      title: '租户可分配',
      dataIndex: 'tenantAssignable',
      width: 110,
      render: (value: boolean) => (value ? '是' : '否'),
    },
    { title: '排序', dataIndex: 'sort', width: 80 },
    {
      title: '是否显示',
      dataIndex: 'visible',
      width: 100,
      render: (value: boolean, node) =>
        node.type === 'permission' ? '-' : value ? '是' : '否',
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 110,
      render: (status: MenuNode['status'], node) =>
        editable
          ? (
              <Permission
                code="platform:menu:status"
                fallback={(
                  <Tag color={status === 'enabled' ? 'success' : 'default'}>
                    {getLabel(DICTIONARY_CODE.entityStatus, status)}
                  </Tag>
                )}
              >
                <StatusSwitch
                  disabled={isProtectedPlatformMenu(node)}
                  onChange={value => onStatusChange?.(node, value === 'enabled')}
                  value={status}
                />
              </Permission>
            )
          : (
              <Tag color={status === 'enabled' ? 'success' : 'default'}>
                {getLabel(DICTIONARY_CODE.entityStatus, status)}
              </Tag>
            ),
    },
  ]

  if (editable) {
    columns.push({
      title: '操作',
      key: 'actions',
      fixed: 'right',
      width: 220,
      render: (_, node) => (
        <Space size="small" wrap>
          {node.type !== 'permission'
            ? (
                <Permission code="platform:menu:create">
                  <Button
                    onClick={() => onAddChild?.(node)}
                    size="small"
                    type="link"
                  >
                    新增子节点
                  </Button>
                </Permission>
              )
            : null}
          <Permission code="platform:menu:edit">
            <Button onClick={() => onEdit?.(node)} size="small" type="link">
              编辑
            </Button>
          </Permission>
          {!isProtectedPlatformMenu(node)
            ? (
                <Permission code="platform:menu:delete">
                  <ConfirmDelete onConfirm={() => onDelete?.(node)}>
                    <Button danger size="small" type="link">
                      删除
                    </Button>
                  </ConfirmDelete>
                </Permission>
              )
            : null}
        </Space>
      ),
    })
  }

  return {
    columns,
    dataSource: data,
    expandable: {
      expandedRowKeys,
      onExpandedRowsChange,
    },
    loading,
    pagination: false,
    rowKey: 'id',
    scroll: { x: 'max-content' },
  }
}
