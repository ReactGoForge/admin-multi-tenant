# OpenAPI 契约

`openapi.yaml` 是服务端、Web 和小程序共用的 OpenAPI 3.1 唯一入口，覆盖 Gin 路由表中的全部接口。

## 目录职责

- `openapi.yaml`：项目信息、全部路径引用、公共参数、错误响应、安全方案和 Schema 索引。
- `paths/`：按业务模块保存 Path Item 与 Operation。
- `components/`：按领域保存请求和响应 Schema。

## 修改规则

1. 先检查真实 Gin 路由、Handler DTO、错误映射和响应转换。
2. 修改对应路径与组件文件，保持每个 `operationId` 全局唯一。
3. BIGINT ID 继续使用十进制字符串；应用响应保持 `{code,message,data}`。
4. 文件上传使用 `multipart/form-data`，公开图片响应声明实际图片类型。
5. 不在契约中保存域名、Token、密钥或真实账号。

## 验证

```bash
(cd apps/service && go test ./...)
pnpm --dir apps/web contracts:generate
pnpm --dir apps/miniapp contracts:generate
```

Go 契约测试会解析多文件引用，并将契约中的 HTTP 方法和路径与 Gin 路由表逐项比较。
生成类型后需要提交 Web 与小程序的生成文件。
