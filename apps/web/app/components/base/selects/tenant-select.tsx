import type { TenantSelectProps } from './types'
import { EntitySelect, namedChoices } from './entity-select'

/** TenantSelect 统一展示并按租户名称搜索租户选项。 */
export function TenantSelect({
  options,
  showStatus = true,
  ...props
}: TenantSelectProps) {
  return (
    <EntitySelect
      options={namedChoices(options, showStatus)}
      placeholder="请选择租户"
      {...props}
    />
  )
}
