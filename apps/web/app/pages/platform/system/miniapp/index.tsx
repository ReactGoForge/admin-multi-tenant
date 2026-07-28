import type { FormContentConfig } from '@/components/composite/schema-form'
import { Alert, App, Button, Card, Form, Tag } from 'antd'
import { useEffect, useMemo, useState } from 'react'
import { PageContainer } from '@/components/base/page-container'
import {

  SchemaForm,
} from '@/components/composite/schema-form'
import { Permission } from '@/components/domain/auth/permission'
import { getErrorMessage } from '@/services/errors'
import { isSilentRequestError } from '@/services/http'
import {
  fetchMiniappSettings,
  updateMiniappSettings,
} from '@/services/settings'

/** 小程序设置表单允许提交的公开字段。 */
interface MiniappSettingsFormValues {
  /** 全平台唯一的微信小程序 AppID。 */
  appId: string
}

/** PlatformMiniappSettingsPage 维护公开 AppID，并只展示服务器密钥配置状态。 */
export default function PlatformMiniappSettingsPage() {
  const { message } = App.useApp()

  // 小程序配置状态：表单只维护公开 AppID，密钥状态只读展示且不会取得服务端密钥内容。
  const [miniappSettingsForm] = Form.useForm<MiniappSettingsFormValues>()
  const [miniappSettingsLoading, setMiniappSettingsLoading] = useState(true)
  const [miniappSettingsSaving, setMiniappSettingsSaving] = useState(false)
  const [secretConfigured, setSecretConfigured] = useState(false)
  // 配置项包含可编辑 AppID 和由服务端返回的 AppSecret 配置状态。
  const miniappSettingsFields = useMemo<
    Array<FormContentConfig<MiniappSettingsFormValues>>
  >(
    () => [
      {
        name: 'appId',
        label: '微信小程序 AppID',
        rules: [{ required: true, message: '请输入 AppID' }],
        componentProps: { maxLength: 64 },
      },
      {
        key: 'secret-status',
        renderItem: () => (
          <Form.Item label="AppSecret 状态">
            <Tag color={secretConfigured ? 'success' : 'error'}>
              {secretConfigured ? '服务器已配置' : '服务器未配置'}
            </Tag>
          </Form.Item>
        ),
      },
    ],
    [secretConfigured],
  )
  // 页面挂载后加载公开配置和密钥状态，接口始终不会返回 AppSecret 明文。
  useEffect(() => {
    const controller = new AbortController()
    void fetchMiniappSettings(controller.signal)
      .then((settings) => {
        miniappSettingsForm.setFieldsValue({ appId: settings.appId })
        setSecretConfigured(settings.secretConfigured)
      })
      .catch((error) => {
        if (!isSilentRequestError(error)) {
          void message.error(getErrorMessage(error, '微信配置加载失败'))
        }
      })
      .finally(() => {
        if (!controller.signal.aborted)
          setMiniappSettingsLoading(false)
      })
    return () => controller.abort()
  }, [message, miniappSettingsForm])
  return (
    <PageContainer>
      <Card
        loading={miniappSettingsLoading}
        size="medium"
        title="微信小程序配置"
        variant="borderless"
      >
        <Alert
          description="AppSecret 仅由服务器环境变量 WECHAT_MINIAPP_APP_SECRET 注入，页面和接口不会读取或返回密钥内容。"
          showIcon
          style={{ marginBottom: 24 }}
          title="密钥安全说明"
          type="info"
        />
        <div style={{ maxWidth: 560 }}>
          <SchemaForm
            columns={1}
            fields={miniappSettingsFields}
            form={miniappSettingsForm}
            onFinish={async ({ appId }) => {
              setMiniappSettingsSaving(true)
              try {
                await updateMiniappSettings(appId)
                void message.success('微信小程序配置已更新')
              }
              catch (error) {
                void message.error(getErrorMessage(error, '保存失败'))
              }
              finally {
                setMiniappSettingsSaving(false)
              }
            }}
            showActions={false}
          />
          <Permission code="platform:miniapp:edit">
            <Button
              loading={miniappSettingsSaving}
              onClick={() => miniappSettingsForm.submit()}
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
