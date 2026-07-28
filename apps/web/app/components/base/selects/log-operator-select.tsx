import type { LogOperatorSelectProps } from './types'
import { EntitySelect } from './entity-select'

/** LogOperatorSelect 按历史姓名、账号和操作者类型搜索日志操作者。 */
export function LogOperatorSelect({
  options,
  ...props
}: LogOperatorSelectProps) {
  const choices = options.map((option) => {
    const typeLabel
      = option.actorType === 'miniapp_user' ? '小程序用户' : '后台员工'
    const identity = option.account
      ? `${option.name}（${option.account}）`
      : option.name
    return {
      label: `${identity} · ${typeLabel}`,
      value: option.key,
      keywords: [option.name, option.account, typeLabel]
        .filter(Boolean)
        .join(' '),
    }
  })

  return (
    <EntitySelect options={choices} placeholder="请选择操作者" {...props} />
  )
}
