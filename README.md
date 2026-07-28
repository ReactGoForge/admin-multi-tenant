# ReactGoForge Admin Multi-Tenant

ReactGoForge Admin Multi-Tenant 是一个面向平台运营方与企业租户的多租户管理项目，
包含 React 管理后台、Go API 服务和微信小程序登录示例。项目提供租户、组织、权限、
用户、媒体、品牌配置与日志审计等基础能力，可作为多租户后台系统的开发起点。

本仓库只维护多租户版本。单租户版本位于独立仓库，两个版本不通过运行时开关混用。

## 主要能力

- 平台工作空间：租户、员工、部门、角色、菜单、用户、图片库、字典、基础设置、微信
  小程序配置及系统、操作、登录日志。
- 租户工作空间：员工、部门、角色、菜单、用户、图片库、基础设置及操作、登录日志。
- 权限体系：数据库实时 RBAC、平台与租户数据隔离、平台代管租户、页面和操作权限控制。
- 微信小程序：通过租户 `scene` 进入，支持微信登录、可选手机号授权、头像上传、
  会话恢复和租户切换。
- 基础设施：MySQL、Redis、私有 MinIO 和 Goose SQL Migration。
- 发布方式：Web 静态产物与 Go 二进制按 Commit 打包，服务器校验后切换发布版本。

## 技术架构

| 模块 | 主要技术 | 职责 |
| --- | --- | --- |
| `apps/web` | React 19、React Router 8、Ant Design、Vite、TanStack Query | 平台端和租户端管理后台 |
| `apps/service` | Go 1.26、Gin、GORM、Goose | HTTP API、认证授权和业务服务 |
| `apps/miniapp` | React 18、Taro 4.2、Taroify、Zustand | 微信小程序登录示例 |
| `deploy` | Shell、Docker Compose、OpenResty | 基础设施、产物发布和服务器运行配置 |

各 JavaScript 项目独立管理依赖，不使用根目录 pnpm workspace。数据库结构只由
`apps/service/migrations` 中的 Goose SQL Migration 管理。

## 目录结构

```text
admin-multi-tenant/
├── apps/
│   ├── web/                 # React 管理后台
│   ├── service/             # Go API、OpenAPI 和 Goose Migration
│   └── miniapp/             # Taro 微信小程序
├── deploy/                  # 部署配置与产物发布脚本
├── docs/                    # 操作和部署文档
└── tools/                   # 可选开发工具
```

## 开发准备

需要安装：

- Go `1.26.5`
- Node.js
- pnpm `10.34.5`
- 微信开发者工具（仅开发小程序时需要）

安装前端依赖并创建 Web 本地配置：

```bash
pnpm --dir apps/web install
pnpm --dir apps/miniapp install
cp apps/web/.env.example apps/web/.env.local
```

将 `apps/web/.env.local` 中的 `VITE_API_PROXY_TARGET` 设置为可访问的 API 地址。所有
`VITE_` 变量都会暴露给浏览器，禁止填写密码、Token 或密钥。

启动 Web：

```bash
pnpm --dir apps/web dev
```

默认地址为 `http://localhost:10001`，同源 `/api` 请求由 Vite 转发到配置的 API。

启动微信小程序开发构建：

```bash
TARO_APP_ENV=development pnpm --dir apps/miniapp dev:weapp
```

开发前应将 `apps/miniapp/.env.development` 中的示例 API 地址替换为自己的开发环境。

## 配置与检查

服务端通过环境变量连接 MySQL、Redis、MinIO 和微信小程序能力。服务器配置模板位于
`deploy/.env.example`，其中密码和密钥必须在仓库外保存。

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

## 部署

项目提供基于 Docker Compose、1Panel OpenResty 和发布产物的部署方案。部署前必须将
示例域名、SSH 主机、服务器目录、凭据和代理配置替换为自己的环境值。

```bash
./deploy/release.sh web
./deploy/release.sh service
./deploy/release.sh apps
```

完整流程见：

- [项目操作指南](./docs/operations.md)
- [服务器部署指南](./docs/deployment.md)
- [数据库 Migration 规范](./apps/service/migrations/README.md)

## 安全边界

- 不提交 `.env`、密码、JWT 密钥、微信密钥、MinIO 密钥或服务器备份。
- 客户端构建变量只能保存可公开信息。
- 生产数据库禁止通过开发工具直接写入或执行 DDL。
- 普通应用发布不重启 MySQL、Redis 或 MinIO，也不自动执行 Migration。
- 测试与生产环境必须使用独立数据库、对象存储、Redis、密钥、数据卷和备份目录。

## License

[Apache License 2.0](./LICENSE)
