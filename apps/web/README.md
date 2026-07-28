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

`VITE_API_PROXY_TARGET` 缺省为本机 Go 服务 `http://127.0.0.1:8080`，也可以改为其他
可访问的 API。所有 `VITE_` 变量都会进入浏览器产物，禁止填写密码、Token 或其他
服务端密钥。

## 开发与检查

```bash
pnpm install
pnpm dev
pnpm check
pnpm test
```

开发服务默认监听 `http://localhost:10001`，并将同源 `/api` 转发到配置的 API。
本地联调时应先通过 `make -C ../service run` 启动 Go 服务。

OpenAPI 类型来自 `../service/docs/openapi/openapi.yaml`：

```bash
pnpm contracts:generate
```

生成后应同时检查并提交 `app/services/generated/schema.d.ts`。
