import type { SelectProps } from 'antd'

/** 具名实体选择器使用的通用选项。 */
export interface NamedEntityOption {
  /** 实体唯一标识，也是选择器提交的值。 */
  id: string
  /** 实体展示名称，也是默认搜索关键字。 */
  name: string
  /** 实体业务状态；禁用状态可选择是否展示在文案中。 */
  status?: 'enabled' | 'disabled'
  /** 是否禁止用户选择当前选项。 */
  disabled?: boolean
}

/** 员工选择器选项，在具名实体基础上补充登录账号。 */
export type EmployeeSelectOption = NamedEntityOption & {
  /** 员工登录账号；存在时会与姓名一起展示并参与搜索。 */
  loginAccount?: string | null
}

/** 日志操作者选择器选项，保留日志产生时的身份快照。 */
export interface LogOperatorSelectOption {
  /** 选择器使用的稳定复合键，也是提交值。 */
  key: string
  /** 操作者来源类型。 */
  actorType: 'employee' | 'miniapp_user'
  /** 操作者在对应来源中的实体 ID。 */
  actorId: string
  /** 日志记录的操作者姓名。 */
  name: string
  /** 日志记录的操作者账号；可能为空。 */
  account: string | null
}

/**
 * 具名实体选择器的通用配置。
 * 除 options 与 showSearch 外，其余属性透传给 Ant Design Select。
 */
export type EntitySelectProps<TOption, TValue = string> = Omit<
  SelectProps<TValue>,
  'options' | 'showSearch'
> & {
  /** 业务选项列表，由具体选择器转换成 Ant Design Select 选项。 */
  options: TOption[]
  /** 是否在名称后展示“已禁用”，默认 true。 */
  showStatus?: boolean
}

/** 租户选择器配置，选择值为租户 ID。 */
export type TenantSelectProps = EntitySelectProps<NamedEntityOption>

/** 部门选择器配置，选择值为部门 ID。 */
export type DepartmentSelectProps = EntitySelectProps<NamedEntityOption>

/** 角色选择器配置，兼容单选 ID 与多选 ID 数组。 */
export type RoleSelectProps = EntitySelectProps<
  NamedEntityOption,
  string | string[]
>

/** 员工选择器配置，选择值为员工 ID。 */
export type EmployeeSelectProps = EntitySelectProps<EmployeeSelectOption>

/** 日志操作者选择器配置，选择值为操作者稳定复合键。 */
export type LogOperatorSelectProps = Omit<
  EntitySelectProps<LogOperatorSelectOption>,
  'showStatus'
>
