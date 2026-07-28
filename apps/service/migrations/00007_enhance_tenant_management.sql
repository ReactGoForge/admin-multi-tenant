-- 变更原因：完善租户管理、租户基础设置、平台代管权限和微信小程序配置。
-- 影响范围：为 tenants 增加可空备注与图标字段；新增微信配置表、平台角色租户权限关联表及菜单权限定义。
-- 兼容性：旧租户的 remark、icon_url 保持 NULL；不自动向现有平台角色授予任何新增权限。
-- 数据处理：除插入系统菜单定义外，不回填、不修改、不删除现有业务数据。
-- 回滚说明：Down 会删除新增权限关联、微信配置及租户备注/图标数据，正式使用后回滚前必须备份并再次确认。
-- 验证 SQL（Up 获得执行批准并完成后，只读执行）：
--   SHOW CREATE TABLE tenants;
--   SHOW CREATE TABLE wechat_miniapp_settings;
--   SHOW CREATE TABLE platform_role_tenant_menus;
--   SELECT id, parent_id, name, scope, permission_code, tenant_assignable
--   FROM menus WHERE id BETWEEN 1030 AND 1036 OR id IN (2023, 2024) ORDER BY id;
--   SELECT COUNT(*) AS tenants_with_unexpected_values
--   FROM tenants WHERE remark IS NOT NULL OR icon_url IS NOT NULL;

-- +goose Up

ALTER TABLE `tenants`
  ADD COLUMN `remark` VARCHAR(500) NULL COMMENT '平台内部租户备注' AFTER `name`,
  ADD COLUMN `icon_url` VARCHAR(500) NULL COMMENT '租户图标访问地址' AFTER `remark`;

CREATE TABLE `wechat_miniapp_settings` (
  `id` TINYINT UNSIGNED NOT NULL,
  `app_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  CONSTRAINT `chk_wechat_miniapp_settings_singleton` CHECK (`id` = 1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='全平台唯一微信小程序公开配置';

CREATE TABLE `platform_role_tenant_menus` (
  `role_id` BIGINT UNSIGNED NOT NULL,
  `menu_id` BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (`role_id`, `menu_id`),
  KEY `idx_platform_role_tenant_menus_menu_role` (`menu_id`, `role_id`),
  CONSTRAINT `fk_platform_role_tenant_menus_role`
    FOREIGN KEY (`role_id`) REFERENCES `roles` (`id`)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT `fk_platform_role_tenant_menus_menu`
    FOREIGN KEY (`menu_id`) REFERENCES `menus` (`id`)
    ON UPDATE RESTRICT ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='平台角色进入租户后的可用菜单权限';

INSERT INTO `menus` (
  `id`, `parent_id`, `name`, `node_type`, `scope`, `path`, `component`, `icon`,
  `permission_code`, `tenant_assignable`, `sort`, `visible`, `status`
) VALUES
  (1030, 1001, '新增租户', 'permission', 'platform', NULL, NULL, NULL, 'platform:tenant:create', 0, 1, 0, 1),
  (1031, 1001, '编辑租户', 'permission', 'platform', NULL, NULL, NULL, 'platform:tenant:edit', 0, 2, 0, 1),
  (1032, 1001, '重置租户所有者密码', 'permission', 'platform', NULL, NULL, NULL, 'platform:tenant:reset-password', 0, 3, 0, 1),
  (1033, 1001, '启用或禁用租户', 'permission', 'platform', NULL, NULL, NULL, 'platform:tenant:status', 0, 4, 0, 1),
  (1034, 1001, '查看小程序码', 'permission', 'platform', NULL, NULL, NULL, 'platform:tenant:miniapp-code', 0, 5, 0, 1),
  (1035, 1001, '进入租户', 'permission', 'platform', NULL, NULL, NULL, 'platform:tenant:enter', 0, 6, 0, 1),
  (1036, 1002, '微信小程序配置', 'menu', 'platform', '/platform/system/miniapp', 'pages/platform/system/miniapp/index.tsx', 'WechatOutlined', 'platform:miniapp:view', 0, 70, 1, 1),
  (2023, 2001, '基础设置', 'menu', 'tenant', '/tenant/system/basic', 'pages/tenant/system/basic/index.tsx', 'ToolOutlined', 'tenant:basic:view', 1, 50, 1, 1),
  (2024, 2023, '编辑基础设置', 'permission', 'tenant', NULL, NULL, NULL, 'tenant:basic:edit', 1, 1, 0, 1);

-- +goose Down

DROP TABLE `platform_role_tenant_menus`;

DELETE FROM `role_menus`
WHERE `menu_id` BETWEEN 1030 AND 1036 OR `menu_id` IN (2023, 2024);

DELETE FROM `menus`
WHERE `id` BETWEEN 1030 AND 1036 OR `id` IN (2023, 2024);

DROP TABLE `wechat_miniapp_settings`;

ALTER TABLE `tenants`
  DROP COLUMN `icon_url`,
  DROP COLUMN `remark`;
