import type { FormFieldConfig } from '@/components/composite/schema-form'
import type { DepartmentMutation, PlatformDepartment } from '@/services/platform/departments'
import type { PlatformEmployee } from '@/services/platform/employees'
import type { WorkspaceType } from '@/types/auth'
import type { Employee } from '@/types/rbac'
import { App, Button, Form } from 'antd'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { PageContainer } from '@/components/base/page-container'
import { DepartmentSelect, EmployeeSelect } from '@/components/base/selects'
import { FormDrawer } from '@/components/composite/form-drawer'
import {

  SchemaForm,
} from '@/components/composite/schema-form'
import { SearchTable } from '@/components/composite/search-table'
import { Permission } from '@/components/domain/auth/permission'
import { useDepartmentTreeTableProps } from '@/components/domain/rbac/department-management/department-tree-table'
import { DICTIONARY_CODE, useDictionary } from '@/contexts/dictionary'
import { getErrorMessage } from '@/services/errors'
import { isSilentRequestError } from '@/services/http'
import {
  createWorkspaceDepartment,
  deleteWorkspaceDepartment,
  fetchWorkspaceDepartments,
  updateWorkspaceDepartment,

} from '@/services/platform/departments'
import {
  fetchWorkspaceEmployees,

} from '@/services/platform/employees'
import { buildDepartmentTree } from '@/utils/department-tree'

/** collectDepartmentKeys 递归收集部门树节点 ID，用于展开全部部门。 */
function collectDepartmentKeys(nodes: PlatformDepartment[]): string[] {
  return nodes.flatMap(node => [
    node.id,
    ...(node.children ? collectDepartmentKeys(node.children) : []),
  ])
}

/** 部门管理组件配置。 */
interface PlatformDepartmentManagementProps {
  /** 管理的平台或租户工作空间，默认 platform。 */
  workspace?: WorkspaceType
}

/** PlatformDepartmentManagement 管理平台或当前租户的真实部门树。 */
export function PlatformDepartmentManagement({
  workspace = 'platform',
}: PlatformDepartmentManagementProps) {
  // 页面反馈和状态字典选项。
  const { message } = App.useApp()
  const { getOptions } = useDictionary()
  const statusOptions = getOptions(DICTIONARY_CODE.entityStatus)

  // 部门树和负责人选项状态：
  // - departments/employees：接口返回的部门平铺数据和可选负责人列表。
  // - departmentListLoading/departmentListRefreshVersion：联合请求状态和写操作刷新触发值。
  // - expandedRowKeys：部门树当前展开的节点 ID。
  const [departments, setDepartments] = useState<PlatformDepartment[]>([])
  const [employees, setEmployees] = useState<PlatformEmployee[]>([])
  const [departmentListLoading, setDepartmentListLoading] = useState(false)
  const [departmentListRefreshVersion, setDepartmentListRefreshVersion]
    = useState(0)
  const [expandedRowKeys, setExpandedRowKeys] = useState<React.Key[]>([])

  // 部门表单抽屉状态：editingDepartment 表示编辑目标，fixedParentDepartment 锁定新增子部门的父级。
  const [departmentForm] = Form.useForm<DepartmentMutation>()
  const [editingDepartment, setEditingDepartment]
    = useState<PlatformDepartment | null>(null)
  const [fixedParentDepartment, setFixedParentDepartment]
    = useState<PlatformDepartment | null>(null)
  const [departmentFormOpen, setDepartmentFormOpen] = useState(false)
  const [departmentMutationLoading, setDepartmentMutationLoading]
    = useState(false)

  /** handleDepartmentRequestError 统一展示部门请求错误，并忽略主动取消的请求。 */
  const handleDepartmentRequestError = useCallback(
    (error: unknown, fallback = '部门数据加载失败') => {
      if (!isSilentRequestError(error))
        void message.error(getErrorMessage(error, fallback))
    },
    [message],
  )
  // 部门表单支持父级、名称、负责人、排序和状态；新增子部门时父级不可修改。
  const departmentFormFields = useMemo<
    Array<FormFieldConfig<DepartmentMutation>>
  >(
    () => [
      {
        name: 'parentId',
        label: '上级部门',
        disabled: Boolean(fixedParentDepartment),
        render: () => (
          <DepartmentSelect
            disabled={Boolean(fixedParentDepartment)}
            options={departments.filter(
              item => item.id !== editingDepartment?.id,
            )}
            placeholder="请选择上级部门"
            showStatus={false}
          />
        ),
      },
      {
        name: 'name',
        label: '部门名称',
        rules: [{ required: true, message: '请输入部门名称' }],
        componentProps: { maxLength: 40 },
      },
      {
        name: 'leaderEmployeeId',
        label: '部门负责人',
        render: () => <EmployeeSelect options={employees} showStatus={false} />,
      },
      {
        name: 'sort',
        label: '排序',
        type: 'number',
        rules: [{ required: true, message: '请输入排序' }],
        componentProps: { min: 0 },
      },
      {
        name: 'status',
        label: '状态',
        type: 'select',
        rules: [{ required: true, message: '请选择状态' }],
        options: statusOptions,
      },
    ],
    [
      departments,
      editingDepartment?.id,
      employees,
      fixedParentDepartment,
      statusOptions,
    ],
  )

  // 工作空间或刷新版本变化时并行加载部门和负责人选项，并默认展开完整部门树。
  useEffect(() => {
    void departmentListRefreshVersion
    const controller = new AbortController()
    setDepartmentListLoading(true)
    void Promise.all([
      fetchWorkspaceDepartments(workspace, controller.signal),
      fetchWorkspaceEmployees(
        workspace,
        { page: 1, pageSize: 100 },
        controller.signal,
      ),
    ])
      .then(([departments, employeePage]) => {
        setDepartments(departments)
        setEmployees(employeePage.items)
        setExpandedRowKeys(
          collectDepartmentKeys(buildDepartmentTree(departments)),
        )
      })
      .catch(handleDepartmentRequestError)
      .finally(() => {
        if (!controller.signal.aborted)
          setDepartmentListLoading(false)
      })
    return () => controller.abort()
  }, [departmentListRefreshVersion, handleDepartmentRequestError, workspace])

  // 将平铺部门和员工接口数据转换为树表格需要的结构。
  const departmentTree = useMemo(
    () => buildDepartmentTree(departments),
    [departments],
  )
  const allDepartmentKeys = useMemo(
    () => collectDepartmentKeys(departmentTree),
    [departmentTree],
  )
  const employeeRows = useMemo<Employee[]>(
    () =>
      employees.map(item => ({
        id: item.id,
        userId: item.id,
        name: item.name,
        loginAccount: item.loginAccount,
        workspace,
        departmentId: item.department?.id,
        roleIds: item.roles.map(role => role.id),
        phone: item.phone ?? undefined,
        status: item.status,
        createdAt: item.createdAt,
      })),
    [employees, workspace],
  )
  /** openDepartmentCreateDrawer 根据可选父部门初始化默认值并打开新增抽屉。 */
  const openDepartmentCreateDrawer = (parent: PlatformDepartment | null) => {
    setEditingDepartment(null)
    setFixedParentDepartment(parent)
    departmentForm.resetFields()
    departmentForm.setFieldsValue({
      parentId: parent?.id,
      sort: 10,
      status: 'enabled',
    })
    setDepartmentFormOpen(true)
  }

  /** openDepartmentEditDrawer 回填所选部门字段并打开编辑抽屉。 */
  const openDepartmentEditDrawer = (department: PlatformDepartment) => {
    setEditingDepartment(department)
    setFixedParentDepartment(null)
    departmentForm.setFieldsValue({
      parentId: department.parentId,
      name: department.name,
      leaderEmployeeId: department.leaderEmployeeId,
      sort: department.sort,
      status: department.status,
    })
    setDepartmentFormOpen(true)
  }

  /** handleDepartmentSubmit 提交部门新增或编辑，并在成功后关闭抽屉、刷新部门树。 */
  const handleDepartmentSubmit = async (values: DepartmentMutation) => {
    setDepartmentMutationLoading(true)
    try {
      if (editingDepartment) {
        await updateWorkspaceDepartment(
          workspace,
          editingDepartment.id,
          values,
        )
      }
      else {
        await createWorkspaceDepartment(workspace, {
          ...values,
          parentId: fixedParentDepartment?.id,
        })
      }
      void message.success(editingDepartment ? '部门已更新' : '部门已新增')
      setDepartmentFormOpen(false)
      setDepartmentListRefreshVersion(value => value + 1)
    }
    catch (error) {
      handleDepartmentRequestError(error, '操作失败')
    }
    finally {
      setDepartmentMutationLoading(false)
    }
  }

  // 组装部门树表格的展开状态、负责人数据和受权限控制的行操作。
  const departmentTableProps = useDepartmentTreeTableProps({
    data: departmentTree,
    employees: employeeRows,
    expandedRowKeys,
    loading: departmentListLoading,
    onAddChild: openDepartmentCreateDrawer,
    onDelete: async (department) => {
      try {
        await deleteWorkspaceDepartment(workspace, department.id)
        void message.success('部门已删除')
        setDepartmentListRefreshVersion(value => value + 1)
      }
      catch (error) {
        handleDepartmentRequestError(error, '删除失败')
      }
    },
    onEdit: openDepartmentEditDrawer,
    onExpandedRowsChange: keys => setExpandedRowKeys([...keys]),
    permissionPrefix: workspace,
  })
  return (
    <PageContainer>
      <SearchTable<PlatformDepartment>
        actions={(
          <>
            <Permission code={`${workspace}:department:create`}>
              <Button
                onClick={() => openDepartmentCreateDrawer(null)}
                type="primary"
              >
                新增部门
              </Button>
            </Permission>
            <Button onClick={() => setExpandedRowKeys(allDepartmentKeys)}>
              展开全部
            </Button>
            <Button onClick={() => setExpandedRowKeys([])}>折叠全部</Button>
          </>
        )}
        {...departmentTableProps}
      />
      <FormDrawer
        loading={departmentMutationLoading}
        onClose={() => setDepartmentFormOpen(false)}
        onSubmit={() => departmentForm.submit()}
        open={departmentFormOpen}
        title={
          editingDepartment
            ? '编辑部门'
            : fixedParentDepartment
              ? '新增子部门'
              : '新增部门'
        }
      >
        <SchemaForm
          columns={1}
          fields={departmentFormFields}
          form={departmentForm}
          onFinish={handleDepartmentSubmit}
          showActions={false}
        />
      </FormDrawer>
    </PageContainer>
  )
}
