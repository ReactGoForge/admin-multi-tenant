# 项目架构记忆

本文件只记录当前有效的技术架构、运行边界和版本演进决定。

## 版本定位

- 当前仓库是 ReactGoForge Admin 的多租户版本，默认分支为 `main`。
- 单租户版本由独立仓库维护；本仓库不增加单租户运行模式，也不删除
  `tenant_id`、租户表、租户中间件或历史 Migration。
- 新功能按具体业务需求单独规划，不长期维护项目执行计划或完成流水账。

## 客户端

- `apps/web` 是 React 19 + React Router 8 + Ant Design + Vite 管理后台，包含平台端
  `/platform/*` 与租户端 `/tenant/*` 两个工作空间。
- Web 使用 TanStack Query 管理服务端状态、Zustand 管理认证状态、Zod 校验不可信边界，
  并通过 Vitest 测试关键请求与状态逻辑。
- `apps/miniapp` 是 React 18 + Taro 4.2 + Taroify 1.0.1 微信小程序，当前范围是租户
  登录 Demo：scene 入口、微信登录、可选手机号、头像、会话恢复和跨租户切换。
- 小程序使用显式 Token 的 `Taro.request`、Zustand 认证 Store、Zod `jitless` 校验和
  类型安全 Storage；全局样式入口固定为 `src/app.css`。
- Web 与小程序分别从 `apps/service/docs/openapi/openapi.yaml` 生成本地类型，不建立根
  pnpm workspace。
- 两端使用 Antfu ESLint 和 Stylelint，不使用 Biome 或独立 Prettier 保存动作。

## Go 服务

- `apps/service` 使用 Go、Gin、GORM、MySQL、Redis、MinIO 和 JWT，入口是
  `cmd/server/main.go`，默认监听 `:8080`。
- 业务代码按 `auth`、`rbac`、`media`、`user`、`dictionary`、`logging` 和
  `logquery` 等模块组织，不建立全局 MVC、DDD、CQRS 或通用 CRUD 基类。
- 除健康检查和纯 HTTP 基础设施外，调用链固定为
  Handler → Service → Store/Adapter；`cmd/server` 显式组装依赖。
- Service 负责业务规则、权限、租户范围、事务和外部能力编排；Store 负责 GORM 与
  数据库错误归一化；Adapter 负责 Redis、MinIO、微信和 JWT。
- 数据库结构只由 Goose SQL Migration 管理，服务、命令和测试均禁止 AutoMigrate。

## 接口边界

- 后台 API 使用 `/api/admin`，平台与租户资源分别位于 `/platform/*` 和 `/tenant/*`；
  小程序 API 使用 `/api/miniapp`。
- 所有应用 API 使用 `{code,message,data}`；成功业务码为 `0`，失败保持五位数字业务码。
- BIGINT ID 对外返回字符串；分页参数为 `page`、`pageSize`，默认 10、最大 100。
- Request ID 通过 `X-Request-ID` 响应头传递，Web 与小程序保存到
  `ApiError.requestId`，不扩展响应体。
- `GET /ping`、`GET /healthz` 和 `GET /readyz` 使用统一响应；`readyz` 检查 MySQL
  以及配置中实际启用的认证 Redis，可选 MinIO 不阻断就绪。

## 开发与环境边界

- 本地开发同时运行 Web 与 Go 服务；Web `/api` 缺省代理到本机
  `http://127.0.0.1:8080`。
- Go 由 `make -C apps/service run` 从 `apps/service/.env` 读取开发 MySQL、Redis、
  MinIO 和 JWT 配置；真实环境文件不提交。
- DBHub 是可选的本地只读开发工具，通过 SSH 连接测试服务器数据库；配置和凭据保存在
  仓库外，禁止通过 DBHub 执行写入或 DDL。
- 本地 Web 保持同源 `/api`，Vite 可通过 `VITE_API_PROXY_TARGET` 覆盖代理目标，不启用
  CORS；本地页面端口保持 `10001`。
- 小程序各环境必须通过对应环境文件提供 API 地址，仓库中的 `test.example.com` 只是
  示例占位值。
- Go 与 Web 先在本地联调，再通过 `release.sh service` 或 `release.sh apps` 发布到共享
  环境验证。
- 本地真实环境文件不提交；`VITE_` 和小程序构建变量只能保存客户端可公开信息。

## 部署边界

- 测试与生产环境必须使用独立服务器、域名、数据库、Redis、MinIO、密钥、数据卷和
  备份目录。
- 示例部署由 1Panel OpenResty 管理 80/443、HTTPS 和 Web SPA 静态产物，`/api/`
  代理到只绑定本机的 Go 服务端口。
- Web 与 Go 日常发布使用 `deploy/release.sh` 生成带 SHA-256、Commit、架构和组件清单
  的产物包；服务器校验后原子切换 `current`。
- Go 使用固定 `admin-multi-tenant-service-runtime:1` 容器挂载只读业务二进制。该镜像只包含
  CA、时区、非 root 用户和 Goose v3.27.1。
- `deploy/compose.infra.yaml` 管理 MySQL、Redis、MinIO；
  `deploy/compose.artifact.yaml` 只管理 Go Runtime。
- Docker 离线包只用于 `runtime`、`infra` 和新服务器首次准备的 `all`；普通 Web/Go
  发布不构建业务镜像，不重启基础设施。
- 建议服务器根目录为 `/opt/reactgoforge/admin-multi-tenant`，密钥保存在
  `shared/.env`；应用端口和基础设施端口只绑定 `127.0.0.1`。
- 回滚只恢复上一组 Web/Go 产物，不自动执行 Goose Down。
