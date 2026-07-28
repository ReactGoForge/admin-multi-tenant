import type { EmployeeSelectProps } from './types'
import { entityLabel, EntitySelect } from './entity-select'

/** EmployeeSelect 统一展示并按员工姓名或登录账号搜索员工。 */
export function EmployeeSelect({
  options,
  showStatus = true,
  ...props
}: EmployeeSelectProps) {
  const choices = options.map((option) => {
    const account = option.loginAccount?.trim()
    const label = account
      ? `${entityLabel(option, showStatus)}（${account}）`
      : entityLabel(option, showStatus)
    return {
      label,
      value: option.id,
      keywords: [option.name, account].filter(Boolean).join(' '),
      disabled: option.disabled,
    }
  })

  return <EntitySelect options={choices} placeholder="请选择员工" {...props} />
}
