-- 变更原因：将平台微信小程序配置的查看与保存权限拆开，避免只有查看菜单权限的角色也能修改 AppID。
-- 影响范围：仅向 menus 写入一个固定权限节点，不修改表结构、索引、外键或业务数据。
-- 兼容性：复用现有 platform:miniapp:* 权限分组；平台超级管理员仍由服务端动态放行。
-- 数据处理：不自动授予 platform_admin、平台自定义角色或任何代管角色，普通角色需后续人工授权。
-- 固定标识：菜单权限 ID 为 1049，父节点为微信小程序配置页面 1036，权限编码为 platform:miniapp:edit。
-- 回滚说明：Down 会先删除该权限的 role_menus 关联，再删除菜单权限节点；若上线后人工授权过该权限，回滚会撤销这些授权。
-- 执行约束：本文件仅为 Migration 草案，取得用户明确批准前不得执行。
-- 验证 SQL（Up 获得执行批准并完成后，通过 DBHub 或 MySQL 客户端只读执行）：
--   SELECT version_id, is_applied FROM goose_db_version ORDER BY id DESC LIMIT 3;
--   SELECT id, parent_id, name, node_type, scope, path, component, icon,
--          permission_code, tenant_assignable, sort, visible, status
--   FROM menus
--   WHERE id = 1049 OR permission_code = 'platform:miniapp:edit';
--   预期仅返回 1 行，id=1049，parent_id=1036，scope='platform'，tenant_assignable=0，sort=1，visible=0，status=1。
--   SELECT COUNT(*) AS role_grants
--   FROM role_menus
--   WHERE menu_id = 1049;
--   预期 role_grants = 0。

-- +goose Up

INSERT INTO `menus` (
  `id`,
  `parent_id`,
  `name`,
  `node_type`,
  `scope`,
  `path`,
  `component`,
  `icon`,
  `permission_code`,
  `tenant_assignable`,
  `sort`,
  `visible`,
  `status`
) VALUES
  (1049, 1036, '编辑微信小程序配置', 'permission', 'platform', NULL, NULL, NULL, 'platform:miniapp:edit', 0, 1, 0, 1);

-- +goose Down

DELETE FROM `role_menus`
WHERE `menu_id` = 1049;

DELETE FROM `menus`
WHERE `id` = 1049
  AND `permission_code` = 'platform:miniapp:edit';
