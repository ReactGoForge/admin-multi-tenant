import type { Area } from 'react-easy-crop'
import { CameraOutlined } from '@ant-design/icons'
import { App, Avatar, Button, Modal, Slider, Upload } from 'antd'
import { useEffect, useState } from 'react'
import Cropper from 'react-easy-crop'
import styles from './index.module.scss'

const inputMaxBytes = 5 * 1024 * 1024
const outputMaxBytes = 2 * 1024 * 1024
const outputSize = 512
const allowedTypes = new Set(['image/png', 'image/jpeg', 'image/webp'])

/** AvatarUploaderProps 描述当前头像、回退文字和上传回调。 */
interface AvatarUploaderProps {
  /** 当前头像临时访问地址。 */
  avatarUrl: string | null
  /** 头像不可用时展示的姓名首字。 */
  fallbackText: string
  /** 接收裁剪完成的 512×512 WebP 文件。 */
  onUpload: (file: File) => Promise<void>
}

/** AvatarUploader 独立完成头像选择、1:1 裁剪、缩放和上传。 */
export function AvatarUploader({
  avatarUrl,
  fallbackText,
  onUpload,
}: AvatarUploaderProps) {
  const { message } = App.useApp()
  // 裁剪状态：sourceUrl 保存本地原图，crop/zoom 保存交互位置，croppedAreaPixels 保存最终像素区域。
  const [sourceUrl, setSourceUrl] = useState<string | null>(null)
  const [crop, setCrop] = useState({ x: 0, y: 0 })
  const [zoom, setZoom] = useState(1)
  const [croppedAreaPixels, setCroppedAreaPixels] = useState<Area | null>(null)
  const [uploading, setUploading] = useState(false)

  useEffect(
    () => () => {
      if (sourceUrl)
        URL.revokeObjectURL(sourceUrl)
    },
    [sourceUrl],
  )

  /** closeCropper 关闭裁剪窗口并释放浏览器中的原图对象地址。 */
  const closeCropper = () => {
    if (sourceUrl)
      URL.revokeObjectURL(sourceUrl)
    setSourceUrl(null)
    setCrop({ x: 0, y: 0 })
    setZoom(1)
    setCroppedAreaPixels(null)
  }

  /** selectAvatar 校验原图类型和 5MB 上限后打开裁剪窗口。 */
  const selectAvatar = (file: File) => {
    if (!allowedTypes.has(file.type)) {
      void message.error('请选择 PNG、JPEG 或 WebP 图片')
      return Upload.LIST_IGNORE
    }
    if (file.size > inputMaxBytes) {
      void message.error('原图大小不能超过 5MB')
      return Upload.LIST_IGNORE
    }
    if (sourceUrl)
      URL.revokeObjectURL(sourceUrl)
    setSourceUrl(URL.createObjectURL(file))
    return Upload.LIST_IGNORE
  }

  /** submitAvatar 输出 512×512 WebP，校验 2MB 上限后交给页面上传。 */
  const submitAvatar = async () => {
    if (!sourceUrl || !croppedAreaPixels)
      return
    setUploading(true)
    try {
      const avatarBlob = await cropImage(sourceUrl, croppedAreaPixels)
      if (avatarBlob.size > outputMaxBytes) {
        throw new Error('裁剪后的头像超过 2MB，请选择更简单的图片')
      }
      await onUpload(
        new File([avatarBlob], 'avatar.webp', { type: 'image/webp' }),
      )
      closeCropper()
    }
    catch (error) {
      void message.error(
        error instanceof Error ? error.message : '头像处理失败，请重试',
      )
    }
    finally {
      setUploading(false)
    }
  }

  return (
    <>
      <div className={styles.avatarUploader}>
        <Avatar size={96} src={avatarUrl}>
          {fallbackText}
        </Avatar>
        <Upload
          accept="image/png,image/jpeg,image/webp"
          beforeUpload={selectAvatar}
          disabled={uploading}
          showUploadList={false}
        >
          <Button icon={<CameraOutlined />} loading={uploading}>
            更换头像
          </Button>
        </Upload>
        <div className={styles.hint}>支持 PNG、JPEG、WebP，原图不超过 5MB</div>
      </div>
      <Modal
        cancelButtonProps={{ disabled: uploading }}
        cancelText="取消"
        closable={!uploading}
        mask={{ closable: !uploading }}
        okButtonProps={{ disabled: !croppedAreaPixels, loading: uploading }}
        okText="确认上传"
        onCancel={closeCropper}
        onOk={submitAvatar}
        open={Boolean(sourceUrl)}
        title="裁剪头像"
      >
        <div className={styles.cropArea}>
          {sourceUrl
            ? (
                <Cropper
                  aspect={1}
                  crop={crop}
                  cropShape="round"
                  image={sourceUrl}
                  onCropChange={setCrop}
                  onCropComplete={(_, pixels) => setCroppedAreaPixels(pixels)}
                  onZoomChange={setZoom}
                  showGrid={false}
                  zoom={zoom}
                />
              )
            : null}
        </div>
        <div className={styles.zoomControl}>
          <span>缩放</span>
          <Slider
            disabled={uploading}
            max={3}
            min={1}
            onChange={setZoom}
            step={0.1}
            value={zoom}
          />
        </div>
      </Modal>
    </>
  )
}

/** cropImage 将选定像素区域缩放绘制为 512×512 WebP Blob。 */
async function cropImage(sourceUrl: string, pixelCrop: Area): Promise<Blob> {
  const image = await loadImage(sourceUrl)
  const canvas = document.createElement('canvas')
  canvas.width = outputSize
  canvas.height = outputSize
  const context = canvas.getContext('2d')
  if (!context)
    throw new Error('当前浏览器无法处理头像')
  context.drawImage(
    image,
    pixelCrop.x,
    pixelCrop.y,
    pixelCrop.width,
    pixelCrop.height,
    0,
    0,
    outputSize,
    outputSize,
  )
  return new Promise((resolve, reject) => {
    canvas.toBlob(
      blob => (blob ? resolve(blob) : reject(new Error('头像转换失败'))),
      'image/webp',
      0.9,
    )
  })
}

/** loadImage 等待本地对象地址完成解码后返回可绘制图片。 */
function loadImage(sourceUrl: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image()
    image.onload = () => resolve(image)
    image.onerror = () => reject(new Error('图片无法读取，请重新选择'))
    image.src = sourceUrl
  })
}
