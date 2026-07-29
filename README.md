# ReactGoForge Admin Multi-Tenant

ReactGoForge Admin Multi-Tenant 是一个基于 React 和 Go 的多租户后台基准项目，提供后台系统常用的工程结构、认证授权、多租户隔离和基础设施接入能力，适合作为独立项目的开发起点。

本仓库是独立维护的多租户基准版，不包含特定行业或具体业务系统的数据与流程。使用时可在现有边界内按实际需求扩展业务模块。

## 基准能力

- 多租户管理：区分平台与租户工作空间，提供租户数据隔离和平台代管能力。
- 认证与权限：提供后台登录、JWT 会话、数据库实时 RBAC、菜单和操作权限控制。
- 基础管理：提供员工、部门、角色、菜单、用户、字典和基础设置等通用模块。
- 媒体能力：提供图片库、头像、品牌图片及私有 MinIO 对象存储接入。
- 日志能力：提供系统请求日志、操作日志和后台登录日志。
- 小程序示例：提供基于 Taro 的微信小程序登录示例，用于演示租户入口、会话和用户资料流程。
- 数据库管理：使用 Goose SQL Migration 管理数据库结构，不在应用启动时自动修改数据库。
- 部署支持：提供 Web 静态产物、Go 二进制及基础设施的部署配置与发布脚本。

以上内容是基准项目当前已经具备的通用能力，不代表具体业务系统的完整解决方案。

## 技术架构

| 模块 | 主要技术 | 职责 |
| --- | --- | --- |
| `apps/web` | React 19、React Router 8、Ant Design、Vite、TanStack Query | 平台与租户管理后台 |
| `apps/service` | Go 1.26、Gin、GORM、Goose | HTTP API、认证授权与基础业务服务 |
| `apps/miniapp` | React 18、Taro 4.2、Taroify、Zustand | 微信小程序登录示例 |
| `deploy` | Shell、Docker Compose、OpenResty | 基础设施、产物发布与服务器运行配置 |

各 JavaScript 应用独立管理依赖，不使用根目录 pnpm workspace。数据库结构统一由 `apps/service/migrations` 中的 Goose SQL Migration 管理。

## 目录结构

```text
admin-multi-tenant/
├── apps/
│   ├── web/                 # React 管理后台
│   ├── service/             # Go API、OpenAPI 和 Goose Migration
│   └── miniapp/             # Taro 微信小程序示例
├── deploy/                  # 部署配置与产物发布脚本
├── docs/                    # 操作与部署文档
└── tools/                   # 可选开发工具
```

## 开发环境

开始前需要安装：

- Go `1.26.5`
- Node.js
- pnpm `10.34.5`
- 微信开发者工具（仅开发小程序时需要）

安装前端依赖并创建本地配置：

```bash
pnpm --dir apps/web install
pnpm --dir apps/miniapp install
cp apps/service/.env.example apps/service/.env
cp apps/web/.env.example apps/web/.env.local
```

填写 `apps/service/.env` 中的开发数据库凭据和 `JWT_SECRET`。真实配置不得提交到 Git。

`VITE_API_PROXY_TARGET` 默认指向本机 Go 服务。所有 `VITE_` 变量都会暴露给浏览器，禁止在其中填写密码、Token 或密钥。

## 本地启动

启动 Go 服务：

```bash
make -C apps/service run
```

Go 服务默认监听 `http://127.0.0.1:8080`，可以通过以下接口检查运行状态：

```bash
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/readyz
```

启动 Web 管理后台：

```bash
pnpm --dir apps/web dev
```

Web 默认访问地址为 `http://localhost:10001`，同源 `/api` 请求由 Vite 转发到本机 Go 服务。

启动微信小程序开发构建：

```bash
TARO_APP_ENV=development pnpm --dir apps/miniapp dev:weapp
```

开发小程序前，需要将 `apps/miniapp/.env.development` 中的示例 API 地址替换为自己的开发环境地址。

## 检查与契约生成

常用检查命令：

```bash
pnpm --dir apps/web check
pnpm --dir apps/web test
pnpm --dir apps/miniapp check
pnpm --dir apps/miniapp test
(cd apps/service && go test ./...)
```

根据 OpenAPI 契约更新客户端类型：

```bash
pnpm --dir apps/web contracts:generate
pnpm --dir apps/miniapp contracts:generate
```

## 配置与数据安全

- 服务端本地配置模板位于 `apps/service/.env.example`。
- 服务器配置模板位于 `deploy/.env.example`。
- 密码、JWT 密钥、微信密钥、MinIO 密钥和真实服务器信息只能保存在未提交的环境文件中。
- 生产数据库禁止通过开发工具直接写入或执行 DDL。
- 数据库结构变化必须创建新的 Goose SQL Migration，普通应用启动和发布不会自动执行 Migration。
- 测试与生产环境应使用相互独立的数据库、Redis、对象存储、密钥、数据卷和备份目录。

## 二次开发边界

本项目提供通用后台基座，不预设具体行业模型。开始二次开发前，建议先确认需要保留的基准模块，再按真实业务需求增加页面、接口和数据库结构，避免把示例能力直接当作业务规则。

平台与租户数据必须保持隔离。调整认证、权限、租户范围或数据库结构时，应同时检查现有接口、Migration 和历史数据兼容性。

## 部署与更多文档

仓库提供基于 Docker Compose、1Panel OpenResty 和发布产物的部署方案。部署前必须将示例域名、SSH 主机、服务器目录、凭据和代理配置替换为自己的环境值。

- [项目操作指南](./docs/operations.md)
- [服务器部署指南](./docs/deployment.md)
- [数据库 Migration 规范](./apps/service/migrations/README.md)

## License

[Apache License 2.0](./LICENSE)
