# ReactGoForge Admin Multi-Tenant 操作指南

本文记录开发检查、数据库 Migration、发布和常见排障流程。部署准备与服务器初始化见
[服务器部署指南](./deployment.md)。

## 本地配置

安装前端依赖并复制本地配置模板：

```bash
pnpm --dir apps/web install
pnpm --dir apps/miniapp install
cp apps/service/.env.example apps/service/.env
cp apps/web/.env.example apps/web/.env.local
```

填写 `apps/service/.env` 中的数据库凭据和 `JWT_SECRET`。真实 `.env` 已被 Git 忽略，
不得提交密码或密钥。Web 默认将 `/api` 代理到本机 `127.0.0.1:8080`。

## Go 服务

Go module 为：

```text
github.com/ReactGoForge/admin-multi-tenant/apps/service
```

启动本地服务：

```bash
make -C apps/service run
```

检查服务状态：

```bash
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/readyz
```

运行测试：

```bash
cd apps/service
go test ./...
```

`make run` 只负责从 `apps/service/.env` 导出环境变量并启动服务，不会执行 Migration。

## Web 管理后台

先启动本机 Go 服务，再启动 Web：

```bash
pnpm --dir apps/web dev
```

默认地址为 `http://localhost:10001`。检查与测试：

```bash
pnpm --dir apps/web check
pnpm --dir apps/web test
```

## 微信小程序

安装依赖：

```bash
pnpm --dir apps/miniapp install
```

在 `apps/miniapp/.env.development` 中配置开发 API 后启动微信构建：

```bash
TARO_APP_ENV=development pnpm --dir apps/miniapp dev:weapp
```

将 `apps/miniapp/dist` 导入微信开发者工具。检查与测试：

```bash
pnpm --dir apps/miniapp check
pnpm --dir apps/miniapp test
```

## OpenAPI 契约

契约文件位于 `apps/service/docs/openapi/openapi.yaml`。更新契约后生成两端类型：

```bash
pnpm --dir apps/web contracts:generate
pnpm --dir apps/miniapp contracts:generate
```

生成文件需要与契约变更一起检查和提交。

## 数据库与 Migration

数据库结构的唯一来源是 `apps/service/migrations`，工具固定为 Goose v3 SQL
Migration。禁止使用 GORM AutoMigrate。

变更流程：

1. 检查当前真实表结构、已有 Migration 和相关 Go Model。
2. 创建新的连续编号 SQL Migration。
3. 写明原因、Up、Down 或不可逆风险、兼容性和验证 SQL。
4. 审核通过后才执行 Migration。
5. 执行后只读验证结构和版本。

生产数据库禁止通过 DBHub 直接执行写入或 DDL。

## 可选 DBHub

DBHub 配置默认保存在仓库外：

```text
~/.config/reactgoforge/admin-multi-tenant/
```

常用命令：

```bash
./tools/dbhub.sh start
./tools/dbhub.sh status
./tools/dbhub.sh stop
```

DBHub 只用于 `SELECT`、`SHOW`、`DESCRIBE` 和 `EXPLAIN`。

## 发布

发布脚本按组件生成带 Commit、架构、清单和 SHA-256 的产物：

```bash
./deploy/release.sh web
./deploy/release.sh service
./deploy/release.sh apps
```

默认 SSH 主机为示例别名 `admin-multi-tenant-test`，可通过
`ADMIN_MULTI_TENANT_DEPLOY_HOST` 覆盖。服务器仓库默认位于
`/opt/reactgoforge/admin-multi-tenant/app`。

只有包含已审核 Migration 的 `service` 或 `apps` 发布才允许显式增加 `--migrate`。
普通发布不执行 Migration，不重启 MySQL、Redis 或 MinIO。

服务器常用操作：

```bash
./deploy/artifact-deploy.sh infra up
./deploy/artifact-deploy.sh deploy /path/to/artifact.tar.gz
./deploy/artifact-deploy.sh verify
./deploy/artifact-deploy.sh rollback previous
```

具体参数以脚本 `--help` 输出为准。

## 排障顺序

1. 检查域名和 HTTPS。
2. 检查 OpenResty `/api/` 代理与静态目录。
3. 检查 Go Runtime 的 `/healthz` 与 `/readyz`。
4. 检查 MySQL、Redis 和 MinIO 健康状态。
5. 使用响应头 `X-Request-ID` 关联服务端日志。
6. 检查当前发布目录、Commit 和产物校验信息。

不要通过重启基础设施、修改数据库或跳过权限校验来掩盖应用问题。
