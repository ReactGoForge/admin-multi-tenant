import type { RoleSelectProps } from './types'
import { EntitySelect, namedChoices } from './entity-select'

/** RoleSelect 统一展示并按角色名称搜索单选或多选角色。 */
export function RoleSelect({
  options,
  showStatus = true,
  ...props
}: RoleSelectProps) {
  return (
    <EntitySelect
      options={namedChoices(options, showStatus)}
      placeholder="请选择角色"
      {...props}
    />
  )
}
