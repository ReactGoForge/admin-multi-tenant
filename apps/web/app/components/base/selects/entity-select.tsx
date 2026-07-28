import type { EntitySelectProps, NamedEntityOption } from './types'

import { Select } from 'antd'

/** 业务选项转换后的内部 Select 选项。 */
export interface SelectChoice {
  /** 选择器展示给用户的文案。 */
  label: string
  /** 选择器提交的实体 ID。 */
  value: string
  /** 参与本地搜索匹配的业务关键字。 */
  keywords: string
  /** 是否禁止选择当前实体。 */
  disabled?: boolean
}

/** entityLabel 统一生成具名实体的展示文案。 */
export function entityLabel(option: NamedEntityOption, showStatus: boolean) {
  return showStatus && option.status === 'disabled'
    ? `${option.name}（已禁用）`
    : option.name
}

/** namedChoices 将具名业务选项转换成内部 Select 选项。 */
export function namedChoices(
  options: NamedEntityOption[],
  showStatus: boolean,
) {
  return options.map(option => ({
    label: entityLabel(option, showStatus),
    value: option.id,
    keywords: option.name,
    disabled: option.disabled,
  }))
}

/** EntitySelect 统一实体选择器的清空、搜索和宽度行为。 */
export function EntitySelect<TValue extends string | string[]>({
  options,
  allowClear = true,
  style,
  ...props
}: Omit<EntitySelectProps<SelectChoice, TValue>, 'showStatus'>) {
  return (
    <Select<TValue, SelectChoice>
      allowClear={allowClear}
      options={options}
      showSearch={{ optionFilterProp: ['label', 'keywords'] }}
      style={{ width: '100%', ...style }}
      {...props}
    />
  )
}
