# 后端记忆

本文件记录 Go 服务当前接口、模块职责、启动和安全边界。

## 服务基础

- `apps/service` 入口是 `cmd/server/main.go`，使用 Gin 和标准库 `http.Server` 监听
  `:8080`，支持超时配置和最多 10 秒优雅退出。
- 本地通过 `make run` 启动，Makefile 只负责导出 `apps/service/.env` 并运行
  `go run ./cmd/server`，不会自动执行 Migration。
- 测试服务器通过环境文件为 Go Runtime 注入 MySQL、Redis 和 MinIO 连接配置；GORM
  使用 MySQL 应用账号，禁止 root，连接使用 `utf8mb4`、`parseTime=true`、`loc=Local`。
- 本地 Go 服务使用开发数据库、Redis 和 MinIO；共享环境仍通过 `release.sh service`
  或 `release.sh apps` 发布产物。
- `healthz` 只判断进程存活；`readyz` 在三秒内检查 MySQL 和配置中实际启用的验证码、
  登录限流 Redis，失败返回 HTTP 503 与业务码 `50004`。
- `apps/service/docs/openapi/openapi.yaml` 是覆盖完整 Gin 路由表的 OpenAPI 3.1 唯一入口，`paths/` 与 `components/` 分别按业务模块和领域拆分；不安装 Swaggo，
  不注册 Swagger UI。
- 数据库结构只由 `apps/service/migrations` 的 Goose SQL Migration 管理，不使用
  AutoMigrate。

## 路由与调用链

- 路由分为公开接口、小程序、后台公共接口、平台工作空间和租户工作空间。
- `internal/httpapi/router.go` 管理 Engine、全局中间件、错误入口和注册顺序；
  `cmd/server/routes.go` 组装 Handler 与路由。
- 后台中间件顺序固定为认证、审计、资源权限。
- 除健康检查、Recovery、404/405、统一响应和纯 HTTP 请求日志外，业务调用统一经过
  Handler → Service → Store/Adapter。
- Handler 只处理 HTTP DTO、可信身份提取、响应和审计上下文；Service 负责业务规则、
  权限、租户范围、事务和外部能力编排；Store 负责 GORM；Adapter 负责 Redis、MinIO、
  微信和 JWT。

## 业务模块

- `internal/auth`：验证码、登录限流、凭证、唯一会话、Token、当前用户、平台代管、个人
  资料、密码和实时授权。
- `internal/rbac`：平台与租户的员工、角色、菜单、部门和租户生命周期；资源继续在同一
  业务包内按文件分组。
- `internal/media`：图片、分类、品牌、员工头像、小程序头像、预签名地址和 MinIO 补偿。
- `internal/user`：微信登录、小程序会话、平台/租户用户管理、微信配置和租户小程序码。
- `internal/dictionary`：全局字典消费和管理，保持系统字典保护。
- `internal/logquery`：平台与租户日志列表、分页和筛选选项。
- `internal/logging`：登录日志、操作审计、运行事件和清理；纯 HTTP 请求日志中间件是
  明确的基础设施例外。
- `cmd/server` 按 Store/Adapter → Service → Handler 显式组装全部依赖，业务 Handler
  不持有 GORM、具体 Store、Redis、MinIO、微信客户端或 TokenManager。

## 主要接口范围

- 后台认证：验证码、登录、`/me`、个人手机号、头像和密码。
- 平台与租户：员工、角色、菜单、部门、用户、图片、基础设置和日志。
- 平台专属：租户生命周期、全局字典、微信小程序配置、平台用户租户归属和小程序码。
- 小程序：微信登录、`/me` 和头像上传；登录支持可选可信 `phoneCode`。
- 图片和头像只接受真实 PNG、JPEG、WebP；通用图片和小程序头像最大 5MB，员工头像由
  Web 裁剪为 512×512 WebP 后上传。

## 认证与数据边界

- 后台 JWT 和小程序 JWT 使用不同受众，不能互用；认证与权限详细规则以
  `auth-rbac.md` 为准。
- 所有租户接口只从认证上下文读取租户 ID，不接受客户端指定数据范围。
- BIGINT ID 对外序列化为字符串；所有应用响应使用 `{code,message,data}`。
- Service 决定事务边界；创建租户与所有者、员工与角色等关联写入保持原子性。
- MinIO、Redis 和微信不能加入 MySQL 事务，由 Service 保持补偿语义。

## 运行日志与初始化

- `APP_ENV=development` 时，服务在 `logs/http-YYYY-MM-DD.jsonl` 保存开发排障所需的
  HTTP 交换，按天切分并保留 7 天；其他环境不创建该日志。该能力保留为代码行为，不作为
  当前本地运行入口。
- 开发 HTTP 文件可能包含敏感文本，禁止提交 Git。
- 服务器 `artifact-deploy.sh bootstrap-owner` 只用于全新环境创建唯一平台所有者，密码
  通过交互终端无回显输入；已有平台所有者时拒绝重复创建。
