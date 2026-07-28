import type { ReactElement } from 'react'
import type {
  ImagePickerProps,
  MultipleImagePickerProps,
  SingleImagePickerProps,
} from './types'

import type { ImageValue } from '@/services/media'
import { Button, Modal } from 'antd'
import { useCallback, useState } from 'react'
import { useAuthStore } from '@/stores/auth'
import { ImageLibrary } from './image-library'

export { ImageLibrary } from './image-library'
export type {
  ImageLibraryProps,
  ImagePickerBaseProps,
  ImagePickerProps,
  ImagePickerSlotContext,
  ImageSource,
  ManageImageLibraryProps,
  MultipleImagePickerProps,
  SelectImageLibraryProps,
  SingleImagePickerProps,
} from './types'

/** ImagePicker 单选模式签名，向表单返回一张图片摘要或 null。 */
export function ImagePicker(props: SingleImagePickerProps): ReactElement
/** ImagePicker 多选模式签名，向表单返回图片摘要数组。 */
export function ImagePicker(props: MultipleImagePickerProps): ReactElement
/** ImagePicker 维护图片选择弹窗和待确认值，图库能力统一交给 ImageLibrary。 */
export function ImagePicker(props: ImagePickerProps) {
  const { workspace, selectionOwner, disabled } = props
  const currentUser = useAuthStore(state => state.currentUser)
  const [open, setOpen] = useState(false)
  const [draftSelection, setDraftSelection] = useState<ImageValue[]>([])
  const [bulkMode, setBulkMode] = useState(false)

  const permissionPrefix = workspace === 'platform' ? 'platform' : 'tenant'
  const pickerDisabled = Boolean(
    disabled
    || (!currentUser?.isSuperAdmin
      && !currentUser?.permissions.includes(`${permissionPrefix}:image:view`)),
  )
  const multiple = props.multiple === true

  /** openPicker 使用当前受控值初始化待确认选择，并打开图片库弹窗。 */
  const openPicker = useCallback(() => {
    if (pickerDisabled)
      return
    setDraftSelection(
      props.multiple
        ? [...(props.value ?? [])]
        : props.value
          ? [props.value]
          : [],
    )
    setOpen(true)
    setBulkMode(false)
  }, [pickerDisabled, props.multiple, props.value])

  /** closePicker 关闭弹窗并丢弃尚未确认的选择。 */
  const closePicker = () => {
    setOpen(false)
    setDraftSelection([])
    setBulkMode(false)
  }

  /** confirmSelection 将弹窗中的待确认选择提交给外部受控字段。 */
  const confirmSelection = () => {
    if (props.multiple)
      props.onChange?.(draftSelection)
    else props.onChange?.(draftSelection[0] ?? null)
    setOpen(false)
    setBulkMode(false)
  }

  const trigger = props.multiple
    ? props.children?.({
        value: props.value ?? [],
        openPicker,
        clear: () => {
          if (!pickerDisabled)
            props.onChange?.([])
        },
        disabled: pickerDisabled,
      })
    : props.children?.({
        value: props.value ?? null,
        openPicker,
        clear: () => {
          if (!pickerDisabled)
            props.onChange?.(null)
        },
        disabled: pickerDisabled,
      })

  return (
    <>
      {trigger ?? (
        <Button disabled={pickerDisabled} onClick={openPicker}>
          选择图片
        </Button>
      )}
      <Modal
        cancelText="取消"
        destroyOnHidden
        onCancel={closePicker}
        onOk={confirmSelection}
        okButtonProps={{ disabled: bulkMode || !draftSelection.length }}
        okText={multiple ? `确定（${draftSelection.length}）` : '确定'}
        open={open}
        title="选择图片"
        width={1040}
      >
        <ImageLibrary
          active={open}
          maxCount={props.multiple ? props.maxCount : undefined}
          mode="select"
          multiple={multiple}
          onChange={setDraftSelection}
          onBulkModeChange={setBulkMode}
          selectionOwner={selectionOwner}
          value={draftSelection}
          workspace={workspace}
        />
      </Modal>
    </>
  )
}
