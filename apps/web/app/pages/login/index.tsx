import type { CaptchaResult, CurrentUser, LoginParams } from '@/services/auth'
import {
  ApartmentOutlined,
  ArrowRightOutlined,
  FileSearchOutlined,
  LockOutlined,
  SafetyCertificateOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { App, Button, Card, Checkbox, Form, Input } from 'antd'
import { useCallback, useEffect, useState } from 'react'
import { useLocation, useNavigate } from 'react-router'
import {

  fetchCaptcha,

} from '@/services/auth'
import { getErrorMessage } from '@/services/errors'
import { useAuthStore } from '@/stores/auth'
import { useBrandStore } from '@/stores/brand'
import styles from './index.module.scss'

/** 登录路由跳转到登录页时携带的来源状态。 */
interface LocationState {
  /** 登录成功后期望返回的原始路径。 */
  from?: string
}

/** getLoginTarget 只允许返回根路径或当前用户工作空间内的来源路径。 */
function getLoginTarget(from: string, workspace?: CurrentUser['workspace']) {
  if (from === '/' || (workspace && from.startsWith(`/${workspace}`))) {
    return from
  }
  return '/'
}

/** 渲染真实登录页，并在 Token 和当前用户校验成功后跳转到目标页面。 */
export default function LoginPage() {
  // 路由、全局反馈和登录表单实例。
  const navigate = useNavigate()
  const location = useLocation()
  const { message } = App.useApp()
  const [loginForm] = Form.useForm<LoginParams>()

  // 验证码状态：captcha 保存当前一次性验证码，加载失败时清空旧验证码并展示 captchaError。
  const [captcha, setCaptcha] = useState<CaptchaResult | null>(null)
  const [captchaLoading, setCaptchaLoading] = useState(false)
  const [captchaError, setCaptchaError] = useState<string | null>(null)

  // 认证状态：hydrate 负责从浏览器存储恢复会话，loginLoading 控制提交按钮。
  const {
    currentUser,
    hydrate,
    hydrated,
    isAuthenticated,
    loading: loginLoading,
    login,
  } = useAuthStore()
  // 平台品牌用于桌面端和移动端登录区域的名称、图标展示。
  const { iconUrl: brandIconUrl, name: brandName } = useBrandStore()
  const requestedPath = (location.state as LocationState | null)?.from ?? '/'

  /**
   * refreshCaptcha 重新获取一次性验证码，并丢弃当前输入的旧答案。
   * 请求失败时清空旧验证码，防止用户继续提交已经不可用的 captchaId。
   */
  const refreshCaptcha = useCallback(async () => {
    setCaptchaLoading(true)
    setCaptchaError(null)
    loginForm.setFieldValue('captchaCode', '')
    try {
      setCaptcha(await fetchCaptcha())
    }
    catch (error) {
      setCaptcha(null)
      setCaptchaError(getErrorMessage(error, '验证码加载失败，请重试'))
    }
    finally {
      setCaptchaLoading(false)
    }
  }, [loginForm])

  // 首次进入登录页时恢复浏览器中的有效认证会话。
  useEffect(() => {
    if (!hydrated) {
      void hydrate()
    }
  }, [hydrate, hydrated])

  // 登录页挂载后立即获取验证码；refreshCaptcha 引用变化时重新请求。
  useEffect(() => {
    void refreshCaptcha()
  }, [refreshCaptcha])

  // 会话恢复完成且已登录时，跳转到安全校验后的来源路径。
  useEffect(() => {
    if (hydrated && isAuthenticated) {
      navigate(getLoginTarget(requestedPath, currentUser?.workspace), {
        replace: true,
      })
    }
  }, [
    currentUser?.workspace,
    hydrated,
    isAuthenticated,
    navigate,
    requestedPath,
  ])

  /**
   * handleFinish 提交真实登录凭证，并按验证码启用状态补充一次性验证码参数。
   * 登录成功后读取 Store 中最新工作空间并跳转；失败时提示错误并刷新已消费的验证码。
   */
  const handleFinish = async (values: LoginParams) => {
    try {
      await login({
        ...values,
        captchaId: captcha?.enabled ? captcha.captchaId : undefined,
        captchaCode: captcha?.enabled ? values.captchaCode : undefined,
      })
      void message.success('登录成功')
      navigate(
        getLoginTarget(
          requestedPath,
          useAuthStore.getState().currentUser?.workspace,
        ),
        { replace: true },
      )
    }
    catch (error) {
      void message.error(getErrorMessage(error, '登录失败，请稍后重试'))
      if (captcha?.enabled) {
        void refreshCaptcha()
      }
    }
  }

  return (
    <main className={styles.loginPage}>
      <section className={styles.loginShowcase}>
        <div className={styles.loginBrand}>
          <div
            className={`${styles.loginLogo} ${
              brandIconUrl ? styles.hasImage : ''
            }`}
          >
            {brandIconUrl
              ? (
                  <img alt={`${brandName}图标`} src={brandIconUrl} />
                )
              : (
                  brandName.trim().charAt(0) || '云'
                )}
          </div>
          <div>
            <div className={styles.loginBrandTitle}>{brandName}</div>
            <div className={styles.loginBrandSubtitle}>统一业务管理平台</div>
          </div>
        </div>
        <div className={styles.loginShowcaseContent}>
          <div className={styles.loginShowcaseCopy}>
            <div className={styles.loginEyebrow}>
              <SafetyCertificateOutlined />
              统一业务管理平台
            </div>
            <h1>
              让平台运营与企业管理，
              <br />
              <span>保持清晰有序。</span>
            </h1>
            <p>
              统一管理租户、员工与业务权限，在清晰的工作空间边界内安全协作。
            </p>
          </div>
          <div className={styles.loginCapabilities}>
            <div>
              <span className={styles.loginCapabilityIcon}>
                <ApartmentOutlined />
              </span>
              <span>
                <strong>多租户统一管理</strong>
                <small>清晰区分平台端与租户工作空间</small>
              </span>
            </div>
            <div>
              <span className={styles.loginCapabilityIcon}>
                <SafetyCertificateOutlined />
              </span>
              <span>
                <strong>权限实时生效</strong>
                <small>按角色与职责控制页面和操作范围</small>
              </span>
            </div>
            <div>
              <span className={styles.loginCapabilityIcon}>
                <FileSearchOutlined />
              </span>
              <span>
                <strong>系统记录可追溯</strong>
                <small>集中查看系统运行与后台操作记录</small>
              </span>
            </div>
          </div>
        </div>
        <div className={styles.loginShowcaseFooter}>
          <span>
            <SafetyCertificateOutlined />
            {' '}
            安全登录
          </span>
          <span>
            <LockOutlined />
            {' '}
            权限实时校验
          </span>
        </div>
      </section>
      <section className={styles.loginPanel}>
        <div className={styles.loginPanelInner}>
          <div className={styles.loginMobileBrand}>
            <div
              className={`${styles.loginLogo} ${
                brandIconUrl ? styles.hasImage : ''
              }`}
            >
              {brandIconUrl
                ? (
                    <img alt={`${brandName}图标`} src={brandIconUrl} />
                  )
                : (
                    brandName.trim().charAt(0) || '云'
                  )}
            </div>
            <div>
              <div className={styles.loginBrandTitle}>{brandName}</div>
              <div className={styles.loginBrandSubtitle}>统一业务管理平台</div>
            </div>
          </div>
          <Card className={styles.loginCard} variant="borderless">
            <div className={styles.loginCardHeading}>
              <p className={styles.loginCardKicker}>账号登录</p>
              <h2>欢迎登录</h2>
              <p className={styles.loginCardDesc}>
                使用你的
                {' '}
                {brandName}
                {' '}
                工作账号进入管理空间。
              </p>
            </div>
            <Form<LoginParams>
              form={loginForm}
              initialValues={{ remember: true }}
              layout="vertical"
              name="admin-multi-tenant-login"
              onFinish={handleFinish}
              requiredMark={false}
            >
              <Form.Item
                label="账号"
                name="username"
                rules={[{ required: true, message: '请输入账号' }]}
              >
                <Input
                  autoComplete="username"
                  placeholder="请输入账号"
                  prefix={<UserOutlined />}
                  size="large"
                />
              </Form.Item>
              <Form.Item
                label="密码"
                name="password"
                rules={[{ required: true, message: '请输入密码' }]}
              >
                <Input.Password
                  autoComplete="current-password"
                  placeholder="请输入密码"
                  prefix={<LockOutlined />}
                  size="large"
                />
              </Form.Item>
              {captcha?.enabled
                ? (
                    <Form.Item label="验证码" required>
                      <div className={styles.loginCaptchaRow}>
                        <Form.Item
                          name="captchaCode"
                          noStyle
                          normalize={(value: string) =>
                            value.replace(/\D/g, '').slice(0, 4)}
                          rules={[
                            { required: true, message: '请输入验证码' },
                            { len: 4, message: '请输入 4 位数字验证码' },
                          ]}
                        >
                          <Input
                            autoComplete="off"
                            inputMode="numeric"
                            maxLength={4}
                            placeholder="4 位数字"
                            size="large"
                          />
                        </Form.Item>
                        <button
                          aria-label="刷新验证码"
                          className={styles.loginCaptchaButton}
                          disabled={captchaLoading}
                          onClick={() => void refreshCaptcha()}
                          title="看不清？点击刷新验证码"
                          type="button"
                        >
                          <img
                            alt="4 位数字验证码，点击可刷新"
                            height={42}
                            src={captcha.image}
                            width={120}
                          />
                        </button>
                      </div>
                    </Form.Item>
                  )
                : null}
              {captchaError
                ? (
                    <div className={styles.loginCaptchaError} role="alert">
                      <span>{captchaError}</span>
                      <Button
                        onClick={() => void refreshCaptcha()}
                        size="small"
                        type="link"
                      >
                        重新加载
                      </Button>
                    </div>
                  )
                : null}
              <div className={styles.loginFormOptions}>
                <Form.Item name="remember" noStyle valuePropName="checked">
                  <Checkbox>保持登录状态</Checkbox>
                </Form.Item>
              </div>
              <Button
                block
                className={styles.loginSubmit}
                htmlType="submit"
                icon={<ArrowRightOutlined />}
                iconPlacement="end"
                disabled={
                  captcha === null || captchaLoading || Boolean(captchaError)
                }
                loading={loginLoading}
                size="large"
                type="primary"
              >
                进入工作台
              </Button>
            </Form>
            <div className={styles.loginSecurityNote}>
              <SafetyCertificateOutlined />
              请妥善保管账号密码，退出后将清理本地登录状态
            </div>
          </Card>
          <p className={styles.loginCopyright}>
            © 2026
            {brandName}
          </p>
        </div>
      </section>
    </main>
  )
}
