# ReactGoForge Admin Web

多租户管理后台使用 React 19、React Router 8、Ant Design、Vite、TanStack Query 和
Zustand，包含平台工作空间与租户工作空间。

## 环境配置

复制本地配置：

```bash
cp .env.example .env.local
```

配置 API 代理：

```dotenv
VITE_API_PROXY_TARGET=https://test.example.com
```

`VITE_API_PROXY_TARGET` 必须指向可访问的 API 服务。所有 `VITE_` 变量都会进入浏览器
产物，禁止填写密码、Token 或其他服务端密钥。

## 开发与检查

```bash
pnpm install
pnpm dev
pnpm check
pnpm test
```

开发服务默认监听 `http://localhost:10001`，并将同源 `/api` 转发到配置的 API。

OpenAPI 类型来自 `../service/docs/openapi/openapi.yaml`：

```bash
pnpm contracts:generate
```

生成后应同时检查并提交 `app/services/generated/schema.d.ts`。
