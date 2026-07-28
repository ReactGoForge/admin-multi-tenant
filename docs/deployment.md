# ReactGoForge Admin Multi-Tenant 服务器部署

本文说明如何在独立服务器上部署多租户项目。示例使用 1Panel OpenResty 托管 Web
静态文件和 HTTPS，Docker Compose 运行 MySQL、Redis、MinIO 与 Go Runtime。

所有域名、SSH 主机、目录和凭据都是示例，部署前必须按实际环境替换。

## 部署边界

- 测试与生产环境使用独立服务器、数据库、Redis、MinIO、密钥、数据卷和备份。
- 应用端口和基础设施端口默认只绑定 `127.0.0.1`。
- 数据库结构只通过 Goose SQL Migration 变更。
- 普通 Web 或 Go 发布不重启基础设施，也不自动执行 Migration。
- 密码、JWT 密钥、微信密钥和 MinIO 密钥只保存在服务器环境文件中。

## 目录与密钥

建议服务器目录：

```text
/opt/reactgoforge/admin-multi-tenant/
├── app/                     # Git 仓库
├── shared/
│   ├── .env                 # 服务器环境配置
│   ├── artifacts/           # 待部署产物
│   ├── service-releases/    # Go 服务版本
│   └── github_deploy        # GitHub Deploy Key
└── backups/                 # 部署前备份
```

初始化目录：

```bash
install -d -m 700 \
  /opt/reactgoforge/admin-multi-tenant/shared \
  /opt/reactgoforge/admin-multi-tenant/backups
```

创建只读 Deploy Key，并将公钥添加到
`ReactGoForge/admin-multi-tenant`：

```bash
ssh-keygen -t ed25519 -C admin-multi-tenant-deploy \
  -f /opt/reactgoforge/admin-multi-tenant/shared/github_deploy -N ''
chmod 600 /opt/reactgoforge/admin-multi-tenant/shared/github_deploy
```

克隆仓库并创建服务器配置：

```bash
GIT_SSH_COMMAND='ssh -i /opt/reactgoforge/admin-multi-tenant/shared/github_deploy -o IdentitiesOnly=yes' \
  git clone git@github.com:ReactGoForge/admin-multi-tenant.git \
  /opt/reactgoforge/admin-multi-tenant/app

cp /opt/reactgoforge/admin-multi-tenant/app/deploy/.env.example \
  /opt/reactgoforge/admin-multi-tenant/shared/.env
chmod 600 /opt/reactgoforge/admin-multi-tenant/shared/.env
```

## 环境配置

编辑 `/opt/reactgoforge/admin-multi-tenant/shared/.env`，至少替换：

- `APP_DOMAIN`、`MEDIA_DOMAIN`
- `GITHUB_REPOSITORY`、`GITHUB_DEPLOY_KEY`
- `MYSQL_ROOT_PASSWORD`、`MYSQL_PASSWORD`
- `REDIS_PASSWORD`
- `JWT_SECRET`
- `MINIO_ROOT_USER`、`MINIO_ROOT_PASSWORD`
- `MINIO_ACCESS_KEY`、`MINIO_SECRET_KEY`
- `WECHAT_MINIAPP_APP_SECRET`
- `TRUSTED_PROXIES`

默认数据库名和应用用户为 `admin_multi_tenant`，默认 MinIO Bucket 为
`admin-multi-tenant-images`。正式部署前可按环境调整，但脚本、Compose 和环境文件
必须保持一致。

## 基础设施

在服务器仓库目录执行：

```bash
cd /opt/reactgoforge/admin-multi-tenant/app
./deploy/artifact-deploy.sh infra up
```

Compose 创建以下持久资源：

- `admin-multi-tenant-mysql-data`
- `admin-multi-tenant-redis-data`
- `admin-multi-tenant-minio-data`
- `admin-multi-tenant-internal`

如果 OpenResty 运行在 Docker bridge 网络中，将其容器接入
`admin-multi-tenant-internal` 后再启用媒体反向代理。

## 1Panel OpenResty

1. 在 1Panel 中创建管理后台站点和媒体站点。
2. 将 `deploy/1panel/main.conf` 合并到管理后台站点配置。
3. 将 `deploy/1panel/media.conf` 合并到媒体站点配置。
4. 将示例站点目录 `admin-multi-tenant-test` 替换为实际 1Panel 站点目录。
5. 配置 HTTPS，并确认 `/api/` 代理到 `127.0.0.1:18080`。

Web 静态根目录指向 `<WEB_RELEASE_ROOT>/current`。不要把真实证书、密钥或完整
1Panel 配置提交到仓库。

## 离线镜像

无外网服务器可在有网络的机器生成镜像包：

```bash
./deploy/offline-images.sh export runtime
./deploy/offline-images.sh export infra
./deploy/offline-images.sh export all
```

将包上传到服务器后加载：

```bash
./deploy/offline-images.sh load /path/to/images.tar.gz
```

镜像代理可通过 `ADMIN_MULTI_TENANT_DOCKER_MIRROR`、
`ADMIN_MULTI_TENANT_QUAY_MIRROR`、`ADMIN_MULTI_TENANT_ALPINE_REPOSITORY` 和
`ADMIN_MULTI_TENANT_GO_PROXY` 覆盖。

## 首次发布

在开发机生成并上传完整产物：

```bash
./deploy/release.sh apps
```

在服务器仓库中部署基础设施和应用后，交互式创建唯一平台所有者：

```bash
./deploy/artifact-deploy.sh bootstrap-owner
```

只有全新环境执行该命令。密码通过终端无回显输入，不应写入命令历史或文档。

## 验证与回滚

验证当前部署：

```bash
./deploy/artifact-deploy.sh verify
```

恢复上一组 Web 与 Go 产物：

```bash
./deploy/artifact-deploy.sh rollback previous
```

回滚不会自动执行 Goose Down。涉及数据库变更时，必须在发布前单独审核兼容性和回滚
方案。
