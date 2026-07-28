import type { ReactElement } from 'react'
import { Popconfirm } from 'antd'

/** ConfirmDelete 配置。 */
export interface ConfirmDeleteProps {
  /** 触发确认框的单个 React 元素。 */
  children: ReactElement
  /** 用户确认删除时触发。 */
  onConfirm: () => void
  /** 确认框标题，默认“确认删除？”。 */
  title?: string
  /** 风险说明，默认提示删除后无法恢复。 */
  description?: string
  /** 是否禁用确认交互，默认 false。 */
  disabled?: boolean
}

/** ConfirmDelete 为删除入口提供统一的二次确认文案。 */
export function ConfirmDelete({
  children,
  onConfirm,
  title = '确认删除？',
  description = '删除后无法恢复，请谨慎操作。',
  disabled = false,
}: ConfirmDeleteProps) {
  return (
    <Popconfirm
      cancelText="取消"
      description={description}
      disabled={disabled}
      okText="删除"
      onConfirm={onConfirm}
      title={title}
    >
      {children}
    </Popconfirm>
  )
}
