import { App, Button, Card, Form, Input, Tabs } from 'antd'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import { PageContainer } from '@/components/base/page-container'
import { AvatarUploader } from '@/components/domain/profile/avatar-uploader'
import {
  changeCurrentPassword,
  updateCurrentBasicProfile,
  uploadCurrentAvatar,
} from '@/services/auth'
import { getErrorMessage } from '@/services/errors'
import { useAuthStore } from '@/stores/auth'
import styles from './index.module.scss'

/** 修改密码表单只在前端保留确认密码字段。 */
interface PasswordFormValues {
  /** 当前正在使用的原密码。 */
  currentPassword: string
  /** 准备设置的新密码。 */
  newPassword: string
  /** 仅用于前端一致性校验的新密码确认值。 */
  confirmPassword: string
}

/** 基本资料表单只允许修改手机号，并以空字符串表达尚未填写。 */
interface BasicProfileFormValues {
  /** 当前员工手机号，允许留空。 */
  phone: string
}

const mainlandPhonePattern = /^(?:(?:\+|00)86)?1\d{10}$/

/** ProfilePage 展示并维护当前账号的基本资料、头像和密码。 */
export function ProfilePage() {
  const { message } = App.useApp()
  const navigate = useNavigate()
  const currentUser = useAuthStore(state => state.currentUser)
  const refreshCurrentUser = useAuthStore(state => state.refreshCurrentUser)
  const logout = useAuthStore(state => state.logout)
  const [basicProfileForm] = Form.useForm<BasicProfileFormValues>()
  const [passwordForm] = Form.useForm<PasswordFormValues>()
  const [basicProfileSaving, setBasicProfileSaving] = useState(false)
  const [passwordSaving, setPasswordSaving] = useState(false)

  useEffect(() => {
    if (!currentUser)
      return
    basicProfileForm.setFieldsValue({
      phone: currentUser.phone ?? '',
    })
  }, [basicProfileForm, currentUser])

  if (!currentUser)
    return null

  /** saveAvatar 上传裁剪头像并重新获取当前用户，使顶栏同步最新头像。 */
  const saveAvatar = async (file: File) => {
    try {
      await uploadCurrentAvatar(file)
      await refreshCurrentUser()
      void message.success('头像已更新')
    }
    catch (error) {
      throw new Error(getErrorMessage(error, '头像上传失败'))
    }
  }

  /** saveBasicProfile 提交当前员工手机号，并刷新认证 Store 中的展示资料。 */
  const saveBasicProfile = async (values: BasicProfileFormValues) => {
    setBasicProfileSaving(true)
    try {
      const phone = values.phone.trim() || null
      await updateCurrentBasicProfile(phone)
      await refreshCurrentUser()
      void message.success('基本资料已更新')
    }
    catch (error) {
      void message.error(getErrorMessage(error, '基本资料更新失败'))
    }
    finally {
      setBasicProfileSaving(false)
    }
  }

  /** savePassword 修改密码，成功后清除当前及保留的平台会话并返回登录页。 */
  const savePassword = async (values: PasswordFormValues) => {
    setPasswordSaving(true)
    try {
      await changeCurrentPassword(values.currentPassword, values.newPassword)
      void message.success('密码已修改，请重新登录')
      logout()
      navigate('/login', { replace: true })
    }
    catch (error) {
      void message.error(getErrorMessage(error, '密码修改失败'))
    }
    finally {
      setPasswordSaving(false)
    }
  }

  const basicProfile = (
    <Card className={styles.profileCard} variant="borderless">
      <div className={styles.profileOverview}>
        <AvatarUploader
          avatarUrl={currentUser.avatarUrl}
          fallbackText={currentUser.avatarText}
          onUpload={saveAvatar}
        />
        <Form<BasicProfileFormValues>
          className={styles.basicProfileForm}
          disabled={basicProfileSaving}
          form={basicProfileForm}
          layout="vertical"
          onFinish={saveBasicProfile}
        >
          <div className={styles.profileFieldGrid}>
            <Form.Item label="姓名">
              <Input readOnly value={currentUser.name} variant="filled" />
            </Form.Item>
            <Form.Item
              label="手机号"
              name="phone"
              rules={[
                {
                  validator: (_, value?: string) => {
                    const phone = value?.trim() ?? ''
                    return !phone || mainlandPhonePattern.test(phone)
                      ? Promise.resolve()
                      : Promise.reject(new Error('请输入正确的手机号'))
                  },
                },
              ]}
            >
              <Input allowClear maxLength={20} placeholder="请输入手机号" />
            </Form.Item>
            <Form.Item label="登录账号">
              <Input readOnly value={currentUser.loginAccount} variant="filled" />
            </Form.Item>
            <Form.Item label="角色">
              <Input
                readOnly
                value={
                  currentUser.roles.map(role => role.name).join('、') || '-'
                }
                variant="filled"
              />
            </Form.Item>
          </div>
          <Button htmlType="submit" loading={basicProfileSaving} type="primary">
            保存修改
          </Button>
        </Form>
      </div>
    </Card>
  )

  const passwordPanel = (
    <Card className={styles.passwordCard} variant="borderless">
      <Form<PasswordFormValues>
        form={passwordForm}
        layout="vertical"
        onFinish={savePassword}
        className={styles.passwordForm}
      >
        <Form.Item
          label="原密码"
          name="currentPassword"
          rules={[{ required: true, message: '请输入原密码' }]}
        >
          <Input.Password autoComplete="current-password" maxLength={18} />
        </Form.Item>
        <Form.Item
          label="新密码"
          name="newPassword"
          rules={[
            { required: true, message: '请输入新密码' },
            {
              validator: (_, value?: string) => {
                const length = Array.from(value ?? '').length
                return !value || (length >= 6 && length <= 18)
                  ? Promise.resolve()
                  : Promise.reject(new Error('新密码长度为 6～18 位'))
              },
            },
          ]}
        >
          <Input.Password autoComplete="new-password" />
        </Form.Item>
        <Form.Item
          dependencies={['newPassword']}
          label="确认新密码"
          name="confirmPassword"
          rules={[
            { required: true, message: '请再次输入新密码' },
            ({ getFieldValue }) => ({
              validator(_, value) {
                return !value || getFieldValue('newPassword') === value
                  ? Promise.resolve()
                  : Promise.reject(new Error('两次输入的新密码不一致'))
              },
            }),
          ]}
        >
          <Input.Password autoComplete="new-password" />
        </Form.Item>
        <Button htmlType="submit" loading={passwordSaving} type="primary">
          修改密码
        </Button>
      </Form>
    </Card>
  )

  return (
    <PageContainer>
      <Tabs
        items={[
          { key: 'basic', label: '基本资料', children: basicProfile },
          { key: 'password', label: '修改密码', children: passwordPanel },
        ]}
      />
    </PageContainer>
  )
}
