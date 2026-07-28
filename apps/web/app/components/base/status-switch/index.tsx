import type { EntityStatus } from '@/types/rbac'

import { Switch } from 'antd'
import { DICTIONARY_CODE, useDictionary } from '@/contexts/dictionary'

/** StatusSwitch 配置。 */
export interface StatusSwitchProps {
  /** 当前实体状态。 */
  value: EntityStatus
  /** 用户切换后触发，返回 enabled 或 disabled。 */
  onChange: (value: EntityStatus) => void
  /** 是否禁用状态切换，默认 false。 */
  disabled?: boolean
}

/** StatusSwitch 使用系统字典展示统一的启用和禁用文案。 */
export function StatusSwitch({
  value,
  onChange,
  disabled = false,
}: StatusSwitchProps) {
  const { getLabel } = useDictionary()
  return (
    <Switch
      checked={value === 'enabled'}
      checkedChildren={getLabel(
        DICTIONARY_CODE.entityStatus,
        'enabled',
        '启用',
      )}
      disabled={disabled}
      onChange={checked => onChange(checked ? 'enabled' : 'disabled')}
      unCheckedChildren={getLabel(
        DICTIONARY_CODE.entityStatus,
        'disabled',
        '禁用',
      )}
    />
  )
}
