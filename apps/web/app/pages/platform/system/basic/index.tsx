import type { FormContentConfig } from '@/components/composite/schema-form'
import type { PlatformBasicSettings } from '@/services/settings'
import { App, Button, Card, Form, Space } from 'antd'
import { useEffect, useMemo, useState } from 'react'
import { PageContainer } from '@/components/base/page-container'
import { ImagePicker } from '@/components/composite/image-picker'
import {

  SchemaForm,
} from '@/components/composite/schema-form'
import { Permission } from '@/components/domain/auth/permission'
import { getErrorMessage } from '@/services/errors'
import { isSilentRequestError } from '@/services/http'
import {
  fetchPlatformBasicSettings,

  updatePlatformBasicSettings,
} from '@/services/settings'
import { useAuthStore } from '@/stores/auth'
import { useBrandStore } from '@/stores/brand'

/** PlatformBasicPage 维护平台名称和图片库图标。 */
export default function PlatformBasicPage() {
  const { message } = App.useApp()
  // 平台基础设置表单状态：首次请求负责回填表单，保存状态只控制提交按钮防止重复操作。
  const [platformSettingsForm] = Form.useForm<PlatformBasicSettings>()
  const [platformSettingsLoading, setPlatformSettingsLoading] = useState(true)
  const [platformSettingsSaving, setPlatformSettingsSaving] = useState(false)
  const refreshCurrentUser = useAuthStore(state => state.refreshCurrentUser)
  const refreshBrand = useBrandStore(state => state.refresh)
  // 表单维护平台名称，并通过图片库选择品牌图标。
  const platformSettingsFields = useMemo<
    Array<FormContentConfig<PlatformBasicSettings>>
  >(
    () => [
      {
        name: 'name',
        label: '平台名称',
        rules: [{ required: true, message: '请输入平台名称' }],
        componentProps: { maxLength: 100 },
      },
      {
        key: 'platform-icon',
        renderItem: () => (
          <Form.Item label="平台图标" name="icon">
            <ImagePicker selectionOwner="platform" workspace="platform">
              {({ value, openPicker, clear, disabled }) => (
                <div className="flex items-center gap-3">
                  {value?.previewUrl
                    ? (
                        <img
                          alt={value.originalName || '平台图标'}
                          className="h-16 w-16 rounded-lg border border-slate-200 object-contain"
                          src={value.previewUrl}
                        />
                      )
                    : null}
                  <Space>
                    <Button disabled={disabled} onClick={openPicker}>
                      {value ? '更换图片' : '选择图片'}
                    </Button>
                    {value
                      ? (
                          <Button disabled={disabled} onClick={clear} type="link">
                            清除
                          </Button>
                        )
                      : null}
                  </Space>
                </div>
              )}
            </ImagePicker>
          </Form.Item>
        ),
      },
    ],
    [],
  )

  // 页面挂载后读取平台设置，清理阶段取消严格模式重放或页面离开产生的旧请求。
  useEffect(() => {
    const controller = new AbortController()
    void fetchPlatformBasicSettings(controller.signal)
      .then(settings => platformSettingsForm.setFieldsValue(settings))
      .catch((error) => {
        if (!isSilentRequestError(error)) {
          void message.error(getErrorMessage(error, '平台基础设置加载失败'))
        }
      })
      .finally(() => {
        if (!controller.signal.aborted)
          setPlatformSettingsLoading(false)
      })
    return () => controller.abort()
  }, [message, platformSettingsForm])

  return (
    <PageContainer>
      <Card
        loading={platformSettingsLoading}
        size="medium"
        title="基础设置"
        variant="borderless"
      >
        <div style={{ maxWidth: 560 }}>
          <SchemaForm
            columns={1}
            fields={platformSettingsFields}
            form={platformSettingsForm}
            onFinish={async (values) => {
              setPlatformSettingsSaving(true)
              try {
                await updatePlatformBasicSettings(values)
                await Promise.all([refreshCurrentUser(), refreshBrand()])
                void message.success('平台基础设置已更新')
              }
              catch (error) {
                void message.error(getErrorMessage(error, '保存失败'))
              }
              finally {
                setPlatformSettingsSaving(false)
              }
            }}
            showActions={false}
          />
          <Permission code="platform:basic:edit">
            <Button
              loading={platformSettingsSaving}
              onClick={() => platformSettingsForm.submit()}
              type="primary"
            >
              保存
            </Button>
          </Permission>
        </div>
      </Card>
    </PageContainer>
  )
}
