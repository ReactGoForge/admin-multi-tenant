import type { FormContentConfig } from '@/components/composite/schema-form'
import type { TenantBasicSettings } from '@/services/settings'
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
  fetchTenantBasicSettings,
  updateTenantBasicSettings,

} from '@/services/settings'
import { useAuthStore } from '@/stores/auth'

/** TenantBasicSettingsPage 维护当前租户名称和图片库图标。 */
export default function TenantBasicSettingsPage() {
  const { message } = App.useApp()
  // 租户基础设置表单状态：首次请求负责回填表单，保存状态只控制提交按钮防止重复操作。
  const [tenantSettingsForm] = Form.useForm<TenantBasicSettings>()
  const [tenantSettingsLoading, setTenantSettingsLoading] = useState(true)
  const [tenantSettingsSaving, setTenantSettingsSaving] = useState(false)
  const refreshCurrentUser = useAuthStore(state => state.refreshCurrentUser)
  // 表单维护当前租户名称，并通过租户图库选择品牌图标。
  const tenantSettingsFields = useMemo<
    Array<FormContentConfig<TenantBasicSettings>>
  >(
    () => [
      {
        name: 'name',
        label: '租户名称',
        rules: [{ required: true, message: '请输入租户名称' }],
        componentProps: { maxLength: 100 },
      },
      {
        key: 'tenant-icon',
        renderItem: () => (
          <Form.Item label="租户图标" name="icon">
            <ImagePicker selectionOwner="tenant" workspace="tenant">
              {({ value, openPicker, clear, disabled }) => (
                <div className="flex items-center gap-3">
                  {value?.previewUrl
                    ? (
                        <img
                          alt={value.originalName || '租户图标'}
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
  // 页面挂载后读取租户设置，清理阶段取消严格模式重放或页面离开产生的旧请求。
  useEffect(() => {
    const controller = new AbortController()
    void fetchTenantBasicSettings(controller.signal)
      .then(settings => tenantSettingsForm.setFieldsValue(settings))
      .catch((error) => {
        if (!isSilentRequestError(error)) {
          void message.error(getErrorMessage(error, '基础设置加载失败'))
        }
      })
      .finally(() => {
        if (!controller.signal.aborted)
          setTenantSettingsLoading(false)
      })
    return () => controller.abort()
  }, [message, tenantSettingsForm])
  return (
    <PageContainer>
      <Card
        loading={tenantSettingsLoading}
        size="medium"
        title="基础设置"
        variant="borderless"
      >
        <div style={{ maxWidth: 560 }}>
          <SchemaForm
            columns={1}
            fields={tenantSettingsFields}
            form={tenantSettingsForm}
            onFinish={async (values) => {
              setTenantSettingsSaving(true)
              try {
                await updateTenantBasicSettings(values)
                await refreshCurrentUser()
                void message.success('基础设置已更新')
              }
              catch (error) {
                void message.error(getErrorMessage(error, '保存失败'))
              }
              finally {
                setTenantSettingsSaving(false)
              }
            }}
            showActions={false}
          />
          <Permission code="tenant:basic:edit">
            <Button
              loading={tenantSettingsSaving}
              onClick={() => tenantSettingsForm.submit()}
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
