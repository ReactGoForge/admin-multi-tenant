import Taro from '@tarojs/taro'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { uploadAvatar } from './upload'

vi.mock('@tarojs/taro', () => ({
  default: {
    compressImage: vi.fn(),
    getFileInfo: vi.fn(),
    uploadFile: vi.fn(),
  },
}))

const compressedImageResult = {
  errMsg: 'compressImage:ok',
  tempFilePath: 'wxfile://compressed-avatar.jpg',
}

const avatarFileInfo = {
  errMsg: 'getFileInfo:ok',
  size: 1024,
}

const successfulUploadResponse = {
  cookies: [],
  data: JSON.stringify({
    code: 0,
    message: '成功',
    data: { avatarUrl: 'https://media.example.com/avatar.jpg' },
  }),
  errMsg: 'uploadFile:ok',
  header: {},
  statusCode: 201,
}

describe('小程序头像上传', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(Taro.compressImage).mockResolvedValue(compressedImageResult)
    vi.mocked(Taro.getFileInfo).mockResolvedValue(avatarFileInfo)
    vi.mocked(Taro.uploadFile).mockResolvedValue(successfulUploadResponse)
  })

  it('压缩为 512×512 后使用新路径上传', async () => {
    await expect(uploadAvatar('wxfile://camera-avatar.jpg', 'access-token'))
      .resolves
      .toEqual({ avatarUrl: 'https://media.example.com/avatar.jpg' })

    expect(Taro.compressImage).toHaveBeenCalledWith({
      src: 'wxfile://camera-avatar.jpg',
      quality: 80,
      compressedWidth: 512,
      compressedHeight: 512,
    })
    expect(Taro.uploadFile).toHaveBeenCalledWith(expect.objectContaining({
      filePath: 'wxfile://compressed-avatar.jpg',
    }))
  })

  it('压缩失败时不上传原始文件', async () => {
    vi.mocked(Taro.compressImage).mockRejectedValue(new Error('compress failed'))

    await expect(uploadAvatar('wxfile://camera-avatar.jpg', 'access-token'))
      .rejects
      .toMatchObject({ message: '头像处理失败，请重新选择' })
    expect(Taro.getFileInfo).not.toHaveBeenCalled()
    expect(Taro.uploadFile).not.toHaveBeenCalled()
  })

  it('压缩后超过 5MB 时停止上传', async () => {
    vi.mocked(Taro.getFileInfo).mockResolvedValue({
      ...avatarFileInfo,
      size: 5 * 1024 * 1024 + 1,
    })

    await expect(uploadAvatar('wxfile://camera-avatar.jpg', 'access-token'))
      .rejects
      .toMatchObject({ message: '头像处理后仍超过 5MB，请重新选择' })
    expect(Taro.uploadFile).not.toHaveBeenCalled()
  })

  it('服务端返回 400 时保留 Request ID', async () => {
    vi.mocked(Taro.uploadFile).mockResolvedValue({
      ...successfulUploadResponse,
      data: JSON.stringify({
        code: 10001,
        message: '请求参数无效',
        data: null,
      }),
      header: { 'X-Request-ID': 'request-miniapp-avatar-400' },
      statusCode: 400,
    })

    await expect(uploadAvatar('wxfile://camera-avatar.jpg', 'access-token'))
      .rejects
      .toMatchObject({
        businessCode: 10001,
        httpStatus: 400,
        requestId: 'request-miniapp-avatar-400',
      })
  })
})
