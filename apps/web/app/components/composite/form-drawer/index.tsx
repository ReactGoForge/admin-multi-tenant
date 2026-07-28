import type { ReactNode } from 'react'
import { Button, Drawer, Space } from 'antd'

/** FormDrawer 配置。 */
export interface FormDrawerProps {
  /** 抽屉标题。 */
  title: ReactNode
  /** 是否显示抽屉。 */
  open: boolean
  /** 抽屉内的表单或只读内容。 */
  children: ReactNode
  /** 关闭或取消时触发。 */
  onClose: () => void
  /** 点击保存时触发；只读模式下不会展示保存按钮。 */
  onSubmit?: () => void
  /** 保存按钮加载状态，默认 false。 */
  loading?: boolean
  /** 是否为只读模式，默认 false。 */
  readonly?: boolean
  /** 抽屉宽度，默认 560。 */
  width?: number
}

/** FormDrawer 统一表单抽屉的底部取消、关闭和保存操作。 */
export function FormDrawer({
  title,
  open,
  children,
  onClose,
  onSubmit,
  loading = false,
  readonly = false,
  width = 560,
}: FormDrawerProps) {
  return (
    <Drawer
      destroyOnHidden
      footer={(
        <div className="flex justify-end">
          <Space>
            <Button onClick={onClose}>{readonly ? '关闭' : '取消'}</Button>
            {!readonly && onSubmit
              ? (
                  <Button loading={loading} onClick={onSubmit} type="primary">
                    保存
                  </Button>
                )
              : null}
          </Space>
        </div>
      )}
      onClose={onClose}
      open={open}
      size={width}
      title={title}
    >
      {children}
    </Drawer>
  )
}
