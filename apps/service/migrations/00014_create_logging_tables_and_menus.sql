-- 变更原因：为管理后台增加可查询的系统请求日志与操作审计日志，并注册平台端、租户端日志页面权限。
-- 影响范围：新增 system_logs、operation_audit_logs 两张表，并向 menus 新增 5 个固定系统节点。
-- 兼容性：只新增表、索引和菜单数据，不修改现有业务表、字段、角色关联或历史数据；旧版本服务可继续运行。
-- 数据处理：Up 不回填历史日志，不自动向现有普通角色授权；平台超级管理员和租户所有者仍由服务端动态放行。
-- 安全边界：日志表只保存脱敏元数据和变更摘要，不保存请求体、响应体、密码、Token、验证码、微信密钥、MinIO 对象键或原始 SQL。
-- 回滚说明：Down 先移除可能产生的角色菜单关联，再删除日志菜单和两张日志表；删除日志表会永久丢失全部日志，执行前必须再次批准并完成必要备份。
-- 执行约束：本文件仅为 Migration 草案，取得用户明确批准前不得执行。
-- 验证 SQL（执行获批并完成后，通过 DBHub 或 MySQL 客户端只读执行）：
--   SHOW CREATE TABLE system_logs;
--   SHOW CREATE TABLE operation_audit_logs;
--   SHOW INDEX FROM system_logs;
--   SHOW INDEX FROM operation_audit_logs;
--   SELECT id, parent_id, name, node_type, scope, path, component, permission_code,
--          tenant_assignable, sort, visible, status
--   FROM menus
--   WHERE id IN (1043, 1044, 1045, 2030, 2031)
--   ORDER BY scope, id;
--   SELECT COUNT(*) AS unexpected_role_grants
--   FROM role_menus
--   WHERE menu_id IN (1043, 1044, 1045, 2030, 2031);
--   预期 unexpected_role_grants 为 0。

-- +goose Up

CREATE TABLE `system_logs` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '系统日志ID',
  `log_type` VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '日志类型：request请求，event运行事件',
  `level` VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '日志级别：info、warn、error',
  `request_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL COMMENT '服务端生成的请求标识，运行事件可为空',
  `occurred_at` DATETIME(3) NOT NULL COMMENT '日志发生时间',
  `method` VARCHAR(10) CHARACTER SET ascii COLLATE ascii_bin NULL COMMENT 'HTTP方法，运行事件可为空',
  `route` VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NULL COMMENT '不含参数值的路由模板',
  `path` VARCHAR(255) NULL COMMENT '不含查询参数的请求路径',
  `status_code` SMALLINT UNSIGNED NULL COMMENT 'HTTP状态码，运行事件可为空',
  `business_code` INT UNSIGNED NULL COMMENT '统一响应业务错误码，无法取得时为空',
  `duration_ms` BIGINT UNSIGNED NULL COMMENT '请求处理耗时毫秒，运行事件可为空',
  `client_ip` VARCHAR(45) CHARACTER SET ascii COLLATE ascii_bin NULL COMMENT '按可信代理配置解析的客户端IP',
  `user_agent` VARCHAR(512) NULL COMMENT '截断后的User-Agent',
  `actor_type` VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NULL COMMENT '操作者类型：employee或miniapp_user',
  `actor_id` BIGINT UNSIGNED NULL COMMENT '操作者ID快照，不设置业务外键',
  `actor_name` VARCHAR(100) NULL COMMENT '操作者名称快照',
  `actor_account` VARCHAR(100) NULL COMMENT '后台员工登录账号快照，小程序用户可为空',
  `workspace` VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NULL COMMENT '请求工作空间：platform、tenant或miniapp',
  `tenant_id` BIGINT UNSIGNED NULL COMMENT '请求所属租户ID快照',
  `auth_mode` VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NULL COMMENT '后台认证模式：normal或managed',
  `message` VARCHAR(500) NULL COMMENT '脱敏后的运行事件或请求结果摘要',
  `metadata` JSON NULL COMMENT '不含敏感信息的扩展元数据',
  PRIMARY KEY (`id`),
  KEY `idx_system_logs_occurred` (`occurred_at`, `id`),
  KEY `idx_system_logs_request` (`request_id`),
  KEY `idx_system_logs_tenant_occurred` (`tenant_id`, `occurred_at`, `id`),
  KEY `idx_system_logs_actor_occurred` (`actor_type`, `actor_id`, `occurred_at`, `id`),
  KEY `idx_system_logs_status_occurred` (`status_code`, `occurred_at`, `id`),
  KEY `idx_system_logs_route_occurred` (`route`, `occurred_at`, `id`),
  CONSTRAINT `chk_system_logs_type` CHECK (`log_type` IN ('request', 'event')),
  CONSTRAINT `chk_system_logs_level` CHECK (`level` IN ('info', 'warn', 'error')),
  CONSTRAINT `chk_system_logs_actor_type` CHECK (`actor_type` IS NULL OR `actor_type` IN ('employee', 'miniapp_user')),
  CONSTRAINT `chk_system_logs_workspace` CHECK (`workspace` IS NULL OR `workspace` IN ('platform', 'tenant', 'miniapp')),
  CONSTRAINT `chk_system_logs_auth_mode` CHECK (`auth_mode` IS NULL OR `auth_mode` IN ('normal', 'managed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='系统请求与运行事件日志';

CREATE TABLE `operation_audit_logs` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '操作审计日志ID',
  `request_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '关联的系统请求标识',
  `occurred_at` DATETIME(3) NOT NULL COMMENT '操作完成时间',
  `actor_employee_id` BIGINT UNSIGNED NOT NULL COMMENT '操作者员工ID快照，不设置业务外键',
  `actor_name` VARCHAR(30) NOT NULL COMMENT '操作者姓名快照',
  `actor_account` VARCHAR(40) NOT NULL COMMENT '操作者登录账号快照',
  `actor_scope` VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '操作者原始身份范围：platform或tenant',
  `workspace` VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '执行操作的工作空间：platform或tenant',
  `auth_mode` VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '认证模式：normal或managed',
  `tenant_id` BIGINT UNSIGNED NULL COMMENT '日志归属租户ID；平台全局操作为空',
  `module_code` VARCHAR(40) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '稳定模块编码',
  `action_code` VARCHAR(60) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '稳定动作编码',
  `action_name` VARCHAR(60) NOT NULL COMMENT '操作动作中文名称',
  `target_type` VARCHAR(40) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '目标资源类型',
  `target_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL COMMENT '目标资源ID快照',
  `target_name` VARCHAR(200) NULL COMMENT '目标资源名称快照',
  `summary` VARCHAR(500) NOT NULL COMMENT '脱敏后的操作摘要',
  `changes` JSON NULL COMMENT '关键字段脱敏前后值',
  `client_ip` VARCHAR(45) CHARACTER SET ascii COLLATE ascii_bin NULL COMMENT '按可信代理配置解析的客户端IP',
  `user_agent` VARCHAR(512) NULL COMMENT '截断后的User-Agent',
  PRIMARY KEY (`id`),
  KEY `idx_audit_logs_occurred` (`occurred_at`, `id`),
  KEY `idx_audit_logs_request` (`request_id`),
  KEY `idx_audit_logs_tenant_occurred` (`tenant_id`, `occurred_at`, `id`),
  KEY `idx_audit_logs_actor_occurred` (`actor_employee_id`, `occurred_at`, `id`),
  KEY `idx_audit_logs_module_action_occurred` (`module_code`, `action_code`, `occurred_at`, `id`),
  KEY `idx_audit_logs_target` (`target_type`, `target_id`, `occurred_at`, `id`),
  CONSTRAINT `chk_audit_logs_actor_scope` CHECK (`actor_scope` IN ('platform', 'tenant')),
  CONSTRAINT `chk_audit_logs_workspace` CHECK (`workspace` IN ('platform', 'tenant')),
  CONSTRAINT `chk_audit_logs_auth_mode` CHECK (`auth_mode` IN ('normal', 'managed')),
  CONSTRAINT `chk_audit_logs_scope_tenant` CHECK (
    (`workspace` = 'platform' AND `tenant_id` IS NULL)
    OR (`workspace` = 'tenant' AND `tenant_id` IS NOT NULL)
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='管理后台成功写操作审计日志';

INSERT INTO `menus` (
  `id`, `parent_id`, `name`, `node_type`, `scope`, `path`, `component`, `icon`,
  `permission_code`, `tenant_assignable`, `sort`, `visible`, `status`
) VALUES
  (1043, NULL, '日志管理', 'directory', 'platform', NULL, NULL, 'FileSearchOutlined', NULL, 0, 40, 1, 1),
  (1044, 1043, '系统日志', 'menu', 'platform', '/platform/logs/system', 'pages/platform/logs/system/index.tsx', 'FileTextOutlined', 'platform:system-log:view', 0, 10, 1, 1),
  (1045, 1043, '操作日志', 'menu', 'platform', '/platform/logs/operations', 'pages/platform/logs/operations/index.tsx', 'AuditOutlined', 'platform:audit-log:view', 0, 20, 1, 1),
  (2030, NULL, '日志管理', 'directory', 'tenant', NULL, NULL, 'FileSearchOutlined', NULL, 1, 30, 1, 1),
  (2031, 2030, '操作日志', 'menu', 'tenant', '/tenant/logs/operations', 'pages/tenant/logs/operations/index.tsx', 'AuditOutlined', 'tenant:audit-log:view', 1, 10, 1, 1);

-- +goose Down

DELETE FROM `platform_role_tenant_menus`
WHERE `menu_id` IN (2030, 2031);

DELETE FROM `role_menus`
WHERE `menu_id` IN (1043, 1044, 1045, 2030, 2031);

DELETE FROM `menus`
WHERE `id` IN (1044, 1045, 1043, 2031, 2030);

DROP TABLE `operation_audit_logs`;
DROP TABLE `system_logs`;
