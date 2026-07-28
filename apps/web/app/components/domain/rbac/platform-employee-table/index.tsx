import type { TableColumnsType, TablePaginationConfig, TableProps } from 'antd'
import type { PlatformEmployee } from '@/services/platform/employees'
import type { WorkspaceType } from '@/types/auth'
import type { EntityStatus } from '@/types/rbac'

import { DeleteOutlined } from '@ant-design/icons'
import { Button, Space, Table, Tag, Tooltip } from 'antd'
import { useMemo } from 'react'
import { ConfirmDelete } from '@/components/base/confirm-delete'
import { StatusSwitch } from '@/components/base/status-switch'
import { Permission } from '@/components/domain/auth/permission'
import { DICTIONARY_CODE, useDictionary } from '@/contexts/dictionary'

/** 员工表格及可选行操作配置。 */
interface PlatformEmployeeTableProps {
  /** 当前页员工数据。 */
  data: PlatformEmployee[]
  /** 表格加载状态。 */
  loading: boolean
  /** 当前页码。 */
  page: number
  /** 每页条数。 */
  pageSize: number
  /** 员工总数。 */
  total: number
  /** 页码或每页条数变化时触发。 */
  onPaginationChange: (page: number, pageSize: number) => void
  /** 操作权限所属工作空间，默认 platform。 */
  workspace?: WorkspaceType
  /** 进入员工编辑时触发；不传则不展示编辑入口。 */
  onEdit?: (employee: PlatformEmployee) => void
  /** 进入角色分配时触发；不传则不展示分配入口。 */
  onAssignRole?: (employee: PlatformEmployee) => void
  /** 进入密码重置时触发；不传则不展示重置入口。 */
  onResetPassword?: (employee: PlatformEmployee) => void
  /** 员工状态切换时触发；不传则不展示状态操作。 */
  onStatusChange?: (employee: PlatformEmployee, status: EntityStatus) => void
  /** 删除已停用且无业务引用的员工时触发；不传则不展示删除入口。 */
  onDelete?: (employee: PlatformEmployee) => void
  /** 当前登录员工 ID；命中后允许编辑基础资料，但不提供其他敏感管理操作。 */
  currentEmployeeId?: string
}

/** usePlatformEmployeeTableProps 生成员工表格的公共列、分页与操作配置。 */
export function usePlatformEmployeeTableProps({
  data,
  loading,
  page,
  pageSize,
  total,
  onPaginationChange,
  workspace,
  onEdit,
  onAssignRole,
  onResetPassword,
  onStatusChange,
  onDelete,
  currentEmployeeId,
}: PlatformEmployeeTableProps): TableProps<PlatformEmployee> {
  const { getLabel } = useDictionary()
  // 根据工作空间和可选回调生成员工资料列、权限操作列及状态切换能力。
  const employeeTableColumns = useMemo<TableColumnsType<PlatformEmployee>>(
    () => [
      { title: '员工姓名', dataIndex: 'name', width: 140 },
      { title: '登录账号', dataIndex: 'loginAccount', width: 150 },
      {
        title: '所属部门',
        dataIndex: 'department',
        width: 140,
        render: (department: PlatformEmployee['department']) =>
          department?.name ?? '-',
      },
      {
        title: '所属角色',
        dataIndex: 'roles',
        width: 220,
        render: (roles: PlatformEmployee['roles']) =>
          roles.length > 0
            ? (
                <Space size={[0, 4]} wrap>
                  {roles.map(role => (
                    <Tag color="blue" key={role.id}>
                      {role.name}
                    </Tag>
                  ))}
                </Space>
              )
            : (
                '-'
              ),
      },
      {
        title: '手机号',
        dataIndex: 'phone',
        width: 130,
        render: (phone: string | null) => phone || '-',
      },
      {
        title: '状态',
        dataIndex: 'status',
        width: 110,
        render: (status: EntityStatus, employee) =>
          workspace && onStatusChange && employee.id !== currentEmployeeId
            ? (
                <Permission
                  code={`${workspace}:employee:status`}
                  fallback={(
                    <Tag color={status === 'enabled' ? 'success' : 'default'}>
                      {getLabel(DICTIONARY_CODE.entityStatus, status)}
                    </Tag>
                  )}
                >
                  <StatusSwitch
                    onChange={value => onStatusChange(employee, value)}
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
      { title: '创建时间', dataIndex: 'createdAt', width: 170 },
      ...(workspace
        ? [
            {
              title: '操作',
              key: 'actions',
              fixed: 'right' as const,
              width: 330,
              render: (_: unknown, employee: PlatformEmployee) => {
                const roleOutOfAuthority = employee.roles.some(
                  role => !role.assignable,
                )
                const deleteDisabledReason
                  = employee.status === 'enabled'
                    ? '请先停用员工后再删除'
                    : roleOutOfAuthority
                      ? '员工角色超出当前授权范围'
                      : undefined
                const isCurrentEmployee = employee.id === currentEmployeeId
                return (
                  <Space size="small" wrap>
                    <Permission code={`${workspace}:employee:edit`}>
                      <Button
                        onClick={() => onEdit?.(employee)}
                        size="small"
                        type="link"
                      >
                        编辑
                      </Button>
                    </Permission>
                    {!isCurrentEmployee && employee.roles.every(role => role.assignable)
                      ? (
                          <Permission code={`${workspace}:employee:assign-role`}>
                            <Button
                              onClick={() => onAssignRole?.(employee)}
                              size="small"
                              type="link"
                            >
                              分配角色
                            </Button>
                          </Permission>
                        )
                      : null}
                    {!isCurrentEmployee
                      ? (
                          <Permission code={`${workspace}:employee:reset-password`}>
                            <Button
                              onClick={() => onResetPassword?.(employee)}
                              size="small"
                              type="link"
                            >
                              重置密码
                            </Button>
                          </Permission>
                        )
                      : null}
                    {!isCurrentEmployee && onDelete
                      ? (
                          <Permission code={`${workspace}:employee:delete`}>
                            <Tooltip title={deleteDisabledReason}>
                              <span>
                                <ConfirmDelete
                                  description="仅会删除已停用且无业务引用的普通员工，历史日志和图片资产会保留。"
                                  disabled={Boolean(deleteDisabledReason)}
                                  onConfirm={() => onDelete(employee)}
                                  title="确认删除员工？"
                                >
                                  <Button
                                    danger
                                    disabled={Boolean(deleteDisabledReason)}
                                    icon={<DeleteOutlined />}
                                    size="small"
                                    type="link"
                                  >
                                    删除
                                  </Button>
                                </ConfirmDelete>
                              </span>
                            </Tooltip>
                          </Permission>
                        )
                      : null}
                  </Space>
                )
              },
            },
          ]
        : []),
    ],
    [
      getLabel,
      onAssignRole,
      onEdit,
      onDelete,
      onResetPassword,
      onStatusChange,
      currentEmployeeId,
      workspace,
    ],
  )

  /** handleChange 将 Ant Design 分页对象转换为页面使用的页码和每页数量。 */
  const handleChange = (pagination: TablePaginationConfig) => {
    onPaginationChange(pagination.current ?? 1, pagination.pageSize ?? 10)
  }

  return {
    columns: employeeTableColumns,
    dataSource: data,
    loading,
    onChange: handleChange,
    pagination: {
      current: page,
      pageSize,
      pageSizeOptions: [10, 20, 50, 100],
      showSizeChanger: true,
      showTotal: count => `共 ${count} 条`,
      total,
    },
    rowKey: 'id',
    scroll: { x: 'max-content' },
  }
}

/** PlatformEmployeeTable 渲染员工列表及受权限控制的行操作。 */
export function PlatformEmployeeTable(props: PlatformEmployeeTableProps) {
  const tableProps = usePlatformEmployeeTableProps(props)
  return <Table<PlatformEmployee> {...tableProps} />
}
