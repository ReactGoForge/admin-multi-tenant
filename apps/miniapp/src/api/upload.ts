import type { ApiResponse } from './types'

import Taro from '@tarojs/taro'

import { env } from '../config/env'
import { readApiResponse, readRequestID } from './client'
import { ApiError, createTransportError } from './errors'
import { apiSuccessCode, unauthorizedBusinessCode } from './types'

interface AvatarUploadResult {
  avatarUrl: string
}

const avatarMaxSize = 5 * 1024 * 1024

/** parseAvatarResponse 解析头像上传响应并映射为统一错误。 */
function parseAvatarResponse(response: Taro.uploadFile.SuccessCallbackResult) {
  const requestId = readRequestID(response.header)
  let rawResponse: unknown
  try {
    rawResponse = JSON.parse(response.data) as unknown
  }
  catch {
    throw new ApiError('服务响应格式异常', {
      httpStatus: response.statusCode,
      requestId,
    })
  }

  const apiResponse: ApiResponse<AvatarUploadResult> | null
    = readApiResponse<AvatarUploadResult>(rawResponse)
  const isUnauthorized = apiResponse?.code === unauthorizedBusinessCode
    || response.statusCode === 401
  if (response.statusCode < 200 || response.statusCode >= 300 || apiResponse?.code !== apiSuccessCode) {
    throw new ApiError(apiResponse?.message || '头像上传失败，请稍后重试', {
      httpStatus: response.statusCode,
      businessCode: apiResponse?.code,
      isUnauthorized,
      requestId,
    })
  }
  return apiResponse.data
}

/** prepareAvatarForUpload 将微信头像压缩为 512×512，并校验压缩后文件大小。 */
async function prepareAvatarForUpload(filePath: string) {
  let compressedFilePath: string
  try {
    const compressedImage = await Taro.compressImage({
      src: filePath,
      quality: 80,
      compressedWidth: 512,
      compressedHeight: 512,
    })
    compressedFilePath = compressedImage.tempFilePath
  }
  catch (error) {
    throw createTransportError(error, '头像处理失败，请重新选择')
  }

  try {
    const fileInfo = await Taro.getFileInfo({ filePath: compressedFilePath })
    if (!('size' in fileInfo) || fileInfo.size <= 0) {
      throw new ApiError('无法读取头像文件，请重新选择')
    }
    if (fileInfo.size > avatarMaxSize) {
      throw new ApiError('头像处理后仍超过 5MB，请重新选择')
    }
  }
  catch (error) {
    if (error instanceof ApiError)
      throw error
    throw createTransportError(error, '无法读取头像文件，请重新选择')
  }

  return compressedFilePath
}

/** uploadAvatar 压缩并校验当前用户头像后上传，服务端继续校验真实格式和正方形。 */
export async function uploadAvatar(filePath: string, accessToken: string) {
  const compressedFilePath = await prepareAvatarForUpload(filePath)
  let response: Taro.uploadFile.SuccessCallbackResult
  try {
    response = await Taro.uploadFile({
      url: `${env.apiBaseUrl}/profile/avatar`,
      filePath: compressedFilePath,
      name: 'file',
      header: {
        Authorization: `Bearer ${accessToken}`,
      },
    })
  }
  catch (error) {
    throw createTransportError(error, '头像上传失败，请稍后重试')
  }
  return parseAvatarResponse(response)
}
