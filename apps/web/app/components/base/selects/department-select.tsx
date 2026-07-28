import type { DepartmentSelectProps } from './types'
import { EntitySelect, namedChoices } from './entity-select'

/** DepartmentSelect 统一展示并按部门名称搜索部门选项。 */
export function DepartmentSelect({
  options,
  showStatus = true,
  ...props
}: DepartmentSelectProps) {
  return (
    <EntitySelect
      options={namedChoices(options, showStatus)}
      placeholder="请选择部门"
      {...props}
    />
  )
}
