import { Button, Result } from 'antd'
import { useNavigate } from 'react-router'

/** 渲染未匹配路由的 404 页面，并允许用户返回授权首页。 */
export default function NotFoundPage() {
  const navigate = useNavigate()
  return (
    <main className="min-h-screen bg-slate-50 px-4 py-16">
      <Result
        extra={(
          <Button
            onClick={() => navigate('/', { replace: true })}
            type="primary"
          >
            返回首页
          </Button>
        )}
        status="404"
        subTitle="当前页面不存在。"
        title="404"
      />
    </main>
  )
}
