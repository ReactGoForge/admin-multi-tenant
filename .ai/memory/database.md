# 数据库记忆

本文件记录当前数据库结构语义、Migration 事实和高风险边界。详细演进历史以 SQL
Migration 和 Git 为准。

## 当前版本

- 数据库使用 MySQL 8.4。
- Goose `00001_create_rbac_core_tables.sql` 至
  `00022_add_miniapp_edit_permission.sql` 构成当前全新数据库基线。
- 新环境首次初始化后的 Migration 版本为 22；仓库不记录任一外部数据库的实时版本。
- Migration 文件必须保持五位连续编号；已经执行或进入共享环境的文件禁止修改。
- 服务、命令和测试均禁止 GORM AutoMigrate。

## 核心数据

- `tenants` 保存租户及所有者引用；租户停用会使旧租户会话在下一次认证时失效。
- `employees` 保存平台和租户后台员工；登录账号全局唯一，`active_session_id` 用于唯一
  会话，`avatar_image_id` 可空并引用私有员工头像。
- `roles`、`employee_roles`、`menus`、`role_menus` 保存后台 RBAC；
  `platform_role_tenant_menus` 保存平台角色的租户代管权限。
- `departments` 保存平台或租户范围内的部门树和可空负责人。
- `users` 保存平台唯一小程序用户、微信身份、可选手机号和私有头像对象键；
  `tenant_users` 使用租户 ID 与用户 ID 联合主键保存多租户归属。
- `users.avatar_url` 保存 MinIO 私有对象键，HTTP 响应按需签发临时地址。
- `users.phone` 当前没有唯一索引；正式手机号身份的一对一并发约束仍需未来独立
  Migration，当前不得依赖该字段实现强唯一身份。

## 配置、媒体与日志

- `platform_settings` 和租户字段保存平台/租户品牌；平台图标只能引用平台图片，租户图标
  可以引用本租户图片或平台共享图片。
- `image_categories`、`image_assets` 保存图片分类和元数据，二进制位于私有 MinIO；
  平台“共享图片”分类受保护。
- `dictionary_types`、`dictionary_items` 保存全局字典；系统字典的编码、稳定值和状态
  受保护。
- `wechat_miniapp_settings` 保存小程序公开配置；AppSecret 只存在服务端环境文件。
- `system_logs` 保存请求、运行事件和登录事件；`operation_audit_logs` 保存后台成功写操作
  审计。

## 权限数据边界

- 数据库包含平台超级管理员、平台管理员和租户企业管理员等内置角色；名称、状态和删除
  受保护。
- 企业管理员权限使用 `role_menus` 快照；新增可分配租户菜单时自动补给企业管理员，
  `tenant:menu:*` 默认排除。
- 普通角色权限不会因新增 Migration 自动获得，除非对应 Migration 明确写入授权关系。
- 平台与租户的首页、图片库、权限管理、日志和微信小程序配置均由真实菜单权限控制。
- 业务数据数量会持续变化，项目记忆不保存员工、租户、用户或图片的实时数量和 ID。

## 数据库管理约束

- 修改数据库前必须比较真实结构、已有 Migration、相关 Go Model 和当前数据。
- 所有结构变化创建新的 Goose SQL Migration，并提供原因、Up、回滚、影响、兼容性、
  数据处理和验证 SQL。
- DBHub 只允许 SELECT、SHOW、DESCRIBE 和 EXPLAIN，不执行写入或 DDL。
- DROP、删除字段、字段缩短、类型修改、数据回填及不可逆 Down 必须单独说明风险并再次
  获得明确批准。
- 生产数据库禁止 AI 直接执行 INSERT、UPDATE、DELETE、ALTER、CREATE、DROP 等写操作。
