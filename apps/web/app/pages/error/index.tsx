import { Button, Result } from 'antd'
import { isRouteErrorResponse, useNavigate } from 'react-router'

/** 独立错误页接收的 React Router 原始异常。 */
interface ErrorPageProps {
  /** 路由加载、渲染或显式抛出的未知错误对象。 */
  error: unknown
}

/** 从 React Router 错误对象中解析展示文案和开发环境堆栈。 */
function resolveErrorInfo(error: unknown) {
  let status: '404' | 'error' = 'error'
  let title = '页面出错'
  let description = '发生了未预期的错误。'
  let stack: string | undefined

  if (isRouteErrorResponse(error)) {
    status = error.status === 404 ? '404' : 'error'
    title = error.status === 404 ? '404' : '页面出错'
    description
      = error.status === 404
        ? '当前页面不存在。'
        : error.statusText || description
  }
  else if (import.meta.env.DEV && error && error instanceof Error) {
    description = error.message
    stack = error.stack
  }

  return { description, stack, status, title }
}

/** 渲染独立错误页，统一承接 404、普通异常和开发环境堆栈展示。 */
export default function ErrorPage({ error }: ErrorPageProps) {
  const navigate = useNavigate()
  const { description, stack, status, title } = resolveErrorInfo(error)

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
        status={status}
        subTitle={description}
        title={title}
      />
      {stack
        ? (
            <pre className="mx-auto mt-6 max-w-5xl overflow-x-auto rounded-lg border border-slate-200 bg-white p-4 text-xs text-slate-700">
              <code>{stack}</code>
            </pre>
          )
        : null}
    </main>
  )
}
