# 项目记忆索引

## 使用规则

- 项目记忆只记录已经确认且长期有效的项目事实。
- 每次任务开始时，先读取本索引，再按任务需要读取相关记忆文件。
- 记忆与真实代码、数据库或已执行 Migration 冲突时，以真实实现为准，并及时标记过期记忆。
- 只有项目事实发生变化时才更新记忆，不记录临时分析、猜测或未确认方案。

## 当前记忆

- [architecture.md](./architecture.md)：项目结构、技术栈、运行边界、前后端接口边界。处理跨端架构、目录职责或本地运行约定时读取。
- [frontend.md](./frontend.md)：`apps/web` 已实现页面、组件、权限展示和前端验证约定。处理前端页面、组件、交互或请求封装时读取。
- [backend.md](./backend.md)：`apps/service` Go 服务、接口、中间件、认证上下文、路由和服务边界。处理后端接口、Go 代码或服务启动逻辑时读取。
- [database.md](./database.md)：数据库表、字段语义、Migration 事实和数据库约束。处理 Migration、字段语义、表结构、权限数据或数据库兼容性时必须读取。
- [auth-rbac.md](./auth-rbac.md)：多租户、后台认证、小程序认证、RBAC 和平台代管边界。处理登录、权限、角色、菜单、租户身份或 Token 时读取。
- [audit-log.md](./audit-log.md)：系统请求日志、运行事件、后台登录日志和操作审计事实。处理日志、审计、保留策略或日志权限时读取。
- [media-branding.md](./media-branding.md)：平台品牌、租户品牌、图片库、头像和 MinIO 能力。处理图片上传、品牌图标、图库、头像或对象存储时读取。

## 读取策略

- 前端任务读取 `frontend.md`；涉及权限、菜单、角色或登录时再读 `auth-rbac.md`。
- 后端任务读取 `backend.md`；涉及数据库表、字段或 Migration 时再读 `database.md`。
- Migration、字段语义、表结构和历史权限数据任务必须读取 `database.md`。
- 认证、RBAC、多租户、平台代管和 Token 任务读取 `auth-rbac.md`，必要时同时读取 `backend.md` 与 `database.md`。
- 日志、审计、登录记录和日志保留任务读取 `audit-log.md`。
- 品牌、图片库、头像、上传和对象存储任务读取 `media-branding.md`。
- 新功能根据用户当前需求单独规划；项目记忆不保存计划、完成清单或开发流水账。
