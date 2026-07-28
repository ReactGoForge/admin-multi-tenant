import { Button } from '@taroify/core'
import { Image, Text, View } from '@tarojs/components'
import Taro, { useLoad } from '@tarojs/taro'
import { useState } from 'react'

import { ApiError } from '../../api/errors'
import { uploadAvatar } from '../../api/upload'
import { parseTenantScene } from '../../schemas/session'
import { authStore, useAuthStore } from '../../stores/auth'

interface PhoneNumberEvent {
  detail: {
    code?: string
    errMsg: string
  }
}

interface ChooseAvatarEvent {
  detail: {
    avatarUrl?: string
  }
}

type LoginMode = 'wechat' | 'phone' | null

export default function Index() {
  const [loginMode, setLoginMode] = useState<LoginMode>(null)
  const [isAvatarUploading, setIsAvatarUploading] = useState(false)
  const accessToken = useAuthStore(state => state.accessToken)
  const tenantScene = useAuthStore(state => state.tenantScene)
  const currentTenant = useAuthStore(state => state.currentTenant)
  const currentUser = useAuthStore(state => state.currentUser)
  const phase = useAuthStore (state => state.phase)
  const syncSession = useAuthStore(state => state.syncSession)
  const loginTenant = useAuthStore(state => state.loginTenant)
  const invalidateToken = useAuthStore(state => state.invalidateToken)
  const updateUser = useAuthStore(state => state.updateUser)

  useLoad((options) => {
    void syncSession(parseTenantScene(options?.scene))
  })

  /** performLogin 执行微信登录，并可附带可信手机号 code。 */
  async function performLogin(mode: Exclude<LoginMode, null>, phoneCode?: string) {
    if (!tenantScene || loginMode || phase !== 'idle') {
      return
    }
    setLoginMode(mode)
    try {
      await loginTenant(tenantScene, phoneCode)
    }
    catch (error) {
      const title = error instanceof Error ? error.message : '登录失败，请稍后重试'
      await Taro.showToast({ title, icon: 'none' })
    }
    finally {
      setLoginMode(null)
    }
  }

  /** handlePhoneLogin 处理手机号授权，拒绝时保留当前会话。 */
  function handlePhoneLogin(event: PhoneNumberEvent) {
    const phoneCode = String(event.detail.code ?? '').trim()
    if (!phoneCode) {
      void Taro.showToast({ title: '未授权手机号，可使用微信登录', icon: 'none' })
      return
    }
    void performLogin('phone', phoneCode)
  }

  /** handleChooseAvatar 上传头像；401 时恢复登录但不重复上传。 */
  async function handleChooseAvatar(event: ChooseAvatarEvent) {
    const avatarPath = String(event.detail.avatarUrl ?? '').trim()
    if (!avatarPath) {
      void Taro.showToast({ title: '未选择头像', icon: 'none' })
      return
    }
    if (!accessToken || !currentUser || isAvatarUploading) {
      return
    }

    const uploadAccessToken = accessToken
    setIsAvatarUploading(true)
    try {
      const result = await uploadAvatar(avatarPath, uploadAccessToken)
      updateUser({ ...currentUser, avatarUrl: result.avatarUrl })
      await Taro.showToast({ title: '头像已更新', icon: 'success' })
    }
    catch (error) {
      if (error instanceof ApiError && error.isUnauthorized) {
        if (authStore.getState().accessToken === uploadAccessToken) {
          const recoveryTenantScene = authStore.getState().tenantScene
          invalidateToken(uploadAccessToken)
          if (recoveryTenantScene) {
            try {
              const restored = await loginTenant(recoveryTenantScene)
              if (restored) {
                await Taro.showToast({ title: '登录状态已恢复，请重新上传头像', icon: 'none' })
              }
            }
            catch (loginError) {
              const title = loginError instanceof Error
                ? loginError.message
                : '登录状态恢复失败，请稍后重试'
              await Taro.showToast({ title, icon: 'none' })
            }
          }
        }
        return
      }
      const title = error instanceof Error ? error.message : '头像上传失败，请稍后重试'
      await Taro.showToast({ title, icon: 'none' })
    }
    finally {
      setIsAvatarUploading(false)
    }
  }

  const isSwitchingTenant = phase === 'switching'
  const isSessionBusy = phase !== 'idle'

  if (isSessionBusy && !currentUser) {
    return (
      <View className="flex min-h-screen items-center justify-center bg-[#f7f8fa]">
        <Text className="text-[28rpx] text-[#969799]">
          {isSwitchingTenant ? '正在切换租户...' : '正在恢复登录状态...'}
        </Text>
      </View>
    )
  }

  if (isSwitchingTenant) {
    return (
      <View className="flex min-h-screen items-center justify-center bg-[#f7f8fa]">
        <Text className="text-[28rpx] text-[#969799]">正在切换租户...</Text>
      </View>
    )
  }

  if (currentTenant && currentUser) {
    return (
      <View className="flex min-h-screen items-center bg-[#f7f8fa] p-[48rpx]">
        <View className="w-full rounded-[20rpx] bg-white p-[48rpx]">
          <Text className="block text-[28rpx] text-[#969799]">当前租户</Text>
          <Text className="mt-[20rpx] block text-[40rpx] font-semibold text-[#323233]">
            {currentTenant.name}
          </Text>
          <View className="mt-[48rpx] flex items-center gap-[24rpx]">
            {currentUser.avatarUrl
              ? <Image className="h-[112rpx] w-[112rpx] rounded-full" mode="aspectFill" src={currentUser.avatarUrl} />
              : (
                  <View className="flex h-[112rpx] w-[112rpx] items-center justify-center rounded-full bg-[#f2f3f5]">
                    <Text className="text-[24rpx] text-[#969799]">暂无头像</Text>
                  </View>
                )}
            <View>
              <Text className="block text-[26rpx] text-[#969799]">手机号</Text>
              <Text className="mt-[8rpx] block text-[30rpx] text-[#323233]">
                {currentUser.phone || '未授权'}
              </Text>
            </View>
          </View>
          <View className="mt-[48rpx] flex flex-col gap-[24rpx]">
            {!currentUser.phone && (
              <Button
                block
                color="success"
                disabled={loginMode !== null || isAvatarUploading || isSessionBusy}
                loading={loginMode === 'phone'}
                loadingText="授权中"
                openType="getPhoneNumber"
                onGetPhoneNumber={handlePhoneLogin}
              >
                授权手机号
              </Button>
            )}
            <Button
              block
              disabled={loginMode !== null || isAvatarUploading || isSessionBusy}
              loading={isAvatarUploading}
              loadingText="上传中"
              openType="chooseAvatar"
              onChooseAvatar={handleChooseAvatar}
            >
              {currentUser.avatarUrl ? '更换头像' : '授权头像'}
            </Button>
            <Text className="text-center text-[24rpx] text-[#969799]">
              头像和手机号均可跳过，不影响查看当前租户
            </Text>
          </View>
        </View>
      </View>
    )
  }

  return (
    <View className="flex min-h-screen items-center bg-[#f7f8fa] p-[48rpx]">
      <View className="w-full rounded-[20rpx] bg-white p-[48rpx]">
        <Text className="block text-center text-[40rpx] font-semibold text-[#323233]">租户登录 Demo</Text>
        {tenantScene
          ? (
              <View className="mt-[48rpx] flex flex-col gap-[24rpx]">
                <Button
                  block
                  color="primary"
                  disabled={loginMode !== null || isSessionBusy}
                  loading={loginMode === 'wechat'}
                  loadingText="登录中"
                  onClick={() => void performLogin('wechat')}
                >
                  微信登录
                </Button>
                <Button
                  block
                  color="success"
                  disabled={loginMode !== null || isSessionBusy}
                  loading={loginMode === 'phone'}
                  loadingText="登录中"
                  openType="getPhoneNumber"
                  onGetPhoneNumber={handlePhoneLogin}
                >
                  一键手机号登录
                </Button>
              </View>
            )
          : (
              <Text className="mt-[32rpx] block text-center text-[28rpx] leading-[44rpx] text-[#ee0a24]">
                未获取到租户场景，请在开发者工具编译条件中传入 scene=租户ID
              </Text>
            )}
      </View>
    </View>
  )
}
