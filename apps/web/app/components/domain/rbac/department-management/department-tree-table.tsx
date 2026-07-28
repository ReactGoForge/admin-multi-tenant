import type { TableColumnsType, TableProps } from 'antd'
import type { Department, Employee } from '@/types/rbac'

import { Button, Space, Tag } from 'antd'
import { ConfirmDelete } from '@/components/base/confirm-delete'
import { Permission } from '@/components/domain/auth/permission'
import { DICTIONARY_CODE, useDictionary } from '@/contexts/dictionary'

/** 部门树表格数据、展开状态和可选操作配置。 */
interface DepartmentTreeTableOptions {
  /** 已经构建好父子关系的部门树。 */
  data: Department[]
  /** 用于解析部门负责人姓名的员工数据。 */
  employees: Employee[]
  /** 行操作权限编码的平台或租户前缀。 */
  permissionPrefix: 'platform' | 'tenant'
  /** 当前展开的部门节点 ID。 */
  expandedRowKeys: React.Key[]
  /** 展开节点变化时同步给上层组件。 */
  onExpandedRowsChange: (keys: readonly React.Key[]) => void
  /** 点击新增子部门时触发。 */
  onAddChild?: (department: Department) => void
  /** 点击编辑部门时触发。 */
  onEdit?: (department: Department) => void
  /** 用户确认删除部门时触发。 */
  onDelete?: (department: Department) => void
  /** 是否隐藏全部写操作，默认 false。 */
  readonly?: boolean
  /** 部门数据加载状态。 */
  loading?: boolean
}

/** useDepartmentTreeTableProps 生成部门树表格的列、展开与操作配置。 */
export function useDepartmentTreeTableProps({
  data,
  employees,
  permissionPrefix,
  expandedRowKeys,
  onExpandedRowsChange,
  onAddChild,
  onEdit,
  onDelete,
  readonly = false,
  loading = false,
}: DepartmentTreeTableOptions): TableProps<Department> {
  // employeeMap 将负责人 ID 转为姓名；找不到时回退部门接口中的负责人快照。
  const { getLabel } = useDictionary()
  const employeeMap = new Map(
    employees.map(employee => [employee.id, employee.name]),
  )
  const columns: TableColumnsType<Department> = [
    { title: '部门名称', dataIndex: 'name', width: 220 },
    {
      title: '部门负责人',
      dataIndex: 'leaderEmployeeId',
      width: 160,
      render: (value: string | undefined, department) =>
        value ? (employeeMap.get(value) ?? department.leaderName ?? '-') : '-',
    },
    { title: '员工数量', dataIndex: 'employeeCount', width: 100 },
    { title: '排序', dataIndex: 'sort', width: 90 },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (status: Department['status']) => (
        <Tag color={status === 'enabled' ? 'success' : 'default'}>
          {getLabel(DICTIONARY_CODE.entityStatus, status)}
        </Tag>
      ),
    },
  ]
  if (!readonly) {
    columns.push({
      title: '操作',
      key: 'actions',
      fixed: 'right',
      width: 230,
      render: (_, department) => (
        <Space size="small">
          <Permission code={`${permissionPrefix}:department:create`}>
            <Button
              onClick={() => onAddChild?.(department)}
              size="small"
              type="link"
            >
              新增子部门
            </Button>
          </Permission>
          <Permission code={`${permissionPrefix}:department:edit`}>
            <Button
              onClick={() => onEdit?.(department)}
              size="small"
              type="link"
            >
              编辑
            </Button>
          </Permission>
          <Permission code={`${permissionPrefix}:department:delete`}>
            <ConfirmDelete onConfirm={() => onDelete?.(department)}>
              <Button danger size="small" type="link">
                删除
              </Button>
            </ConfirmDelete>
          </Permission>
        </Space>
      ),
    })
  }

  return {
    columns,
    dataSource: data,
    expandable: { expandedRowKeys, onExpandedRowsChange },
    loading,
    pagination: false,
    rowKey: 'id',
    scroll: { x: 'max-content' },
  }
}
