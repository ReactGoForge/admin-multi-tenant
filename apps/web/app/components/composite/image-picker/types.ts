import type { ReactNode } from 'react'

import type { ImageValue } from '@/services/media'
import type { WorkspaceType } from '@/types/auth'

/** 图片库数据来源。 */
export type ImageSource = 'platform' | 'tenant'

/** 图片库管理模式配置，不提供业务表单选择能力。 */
export interface ManageImageLibraryProps {
  /** 当前工作空间，决定接口范围和权限前缀。 */
  workspace: WorkspaceType
  /** 独立图片库页面使用管理模式。 */
  mode: 'manage'
  /** 是否开始加载图库数据，默认 true。 */
  active?: boolean
}

/** 图片库选择模式配置，由 ImagePicker 维护待确认的图片摘要。 */
export interface SelectImageLibraryProps {
  /** 当前工作空间，决定接口范围和权限前缀。 */
  workspace: WorkspaceType
  /** 图片选择弹窗使用选择模式。 */
  mode: 'select'
  /** 最终所选图片的归属方，用于限制可选择的数据来源。 */
  selectionOwner: 'platform' | 'tenant'
  /** 是否允许选择多张图片。 */
  multiple: boolean
  /** 多选模式下的最大数量；不传时不限制。 */
  maxCount?: number
  /** 当前待确认的图片摘要列表。 */
  value: ImageValue[]
  /** 待确认图片变化时触发。 */
  onChange: (value: ImageValue[]) => void
  /** 批量管理模式变化时触发，供选择弹窗禁用确认操作。 */
  onBulkModeChange?: (bulkMode: boolean) => void
  /** 是否开始加载图库数据，默认 true。 */
  active?: boolean
}

/** ImageLibrary 根据 mode 区分独立管理与业务选择配置。 */
export type ImageLibraryProps
  = | ManageImageLibraryProps
    | SelectImageLibraryProps

/** ImagePicker 自定义触发器接收的上下文。 */
export interface ImagePickerSlotContext<TValue = ImageValue | null> {
  /** 当前受控选择值。 */
  value: TValue
  /** 打开图片选择弹窗。 */
  openPicker: () => void
  /** 清空当前受控选择值。 */
  clear: () => void
  /** 当前是否因显式禁用或缺少查看权限而不可操作。 */
  disabled: boolean
}

/** ImagePicker 单选与多选模式共享配置。 */
export interface ImagePickerBaseProps {
  /** 当前工作空间，决定接口范围和权限前缀。 */
  workspace: WorkspaceType
  /** 最终所选图片的归属方，用于限制可选择的数据来源。 */
  selectionOwner: 'platform' | 'tenant'
  /** 是否禁止打开、选择或清空图片，默认 false。 */
  disabled?: boolean
}

/** ImagePicker 单选模式配置。 */
export type SingleImagePickerProps = ImagePickerBaseProps & {
  /** 单选模式标识；不传时默认单选。 */
  multiple?: false
  /** 当前选中的图片摘要；未选择时为 null。 */
  value?: ImageValue | null
  /** 用户确认或清空选择时触发。 */
  onChange?: (value: ImageValue | null) => void
  /** 自定义触发器；不传时使用默认“选择图片”按钮。 */
  children?: (context: ImagePickerSlotContext<ImageValue | null>) => ReactNode
}

/** ImagePicker 多选模式配置。 */
export type MultipleImagePickerProps = ImagePickerBaseProps & {
  /** 开启多选模式，固定为 true。 */
  multiple: true
  /** 最多可选择的图片数量；不传时不限制。 */
  maxCount?: number
  /** 当前选中的图片摘要列表。 */
  value?: ImageValue[]
  /** 用户确认或清空选择时触发。 */
  onChange?: (value: ImageValue[]) => void
  /** 自定义触发器；不传时使用默认“选择图片”按钮。 */
  children?: (context: ImagePickerSlotContext<ImageValue[]>) => ReactNode
}

/** ImagePicker 根据 multiple 区分单选和多选配置。 */
export type ImagePickerProps
  = | SingleImagePickerProps
    | MultipleImagePickerProps
