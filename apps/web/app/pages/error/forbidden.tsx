import { Button, Result } from 'antd'
import { useNavigate } from 'react-router'
import { useAuthStore } from '@/stores/auth'

/** 渲染无权限页面，并为正常会话或代管会话提供安全返回入口。 */
export default function ForbiddenPage() {
  const navigate = useNavigate()
  const { currentUser, leaveTenant, logout } = useAuthStore()
  const managed = currentUser?.mode === 'managed'

  /** 返回当前身份可访问的入口；代管模式需先恢复平台会话，失败时彻底退出。 */
  const returnToAccessiblePage = async () => {
    if (!managed) {
      navigate('/', { replace: true })
      return
    }
    try {
      await leaveTenant()
      navigate('/platform', { replace: true })
    }
    catch {
      logout()
      navigate('/login', { replace: true })
    }
  }

  return (
    <main className="min-h-screen bg-slate-50 px-4 py-16">
      <Result
        extra={(
          <Button onClick={() => void returnToAccessiblePage()} type="primary">
            {managed ? '返回平台端' : '返回可访问页面'}
          </Button>
        )}
        status="403"
        subTitle="当前账号没有访问该页面的权限。"
        title="403"
      />
    </main>
  )
}
