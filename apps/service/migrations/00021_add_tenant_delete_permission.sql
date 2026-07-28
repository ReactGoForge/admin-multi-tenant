-- 变更原因：为平台租户管理补充“删除空租户”操作权限，支持前后端按现有 RBAC 体系控制删除入口。
-- 影响范围：仅向 menus 写入一个固定权限节点，不修改表结构、索引、外键或业务数据。
-- 兼容性：复用现有 platform:tenant:* 权限分组；平台超级管理员仍由服务端动态放行。
-- 数据处理：不自动授予 platform_admin、平台自定义角色或任何代管角色，普通角色需后续人工授权。
-- 固定标识：菜单权限 ID 为 1048，父节点为租户管理页面 1001，权限编码为 platform:tenant:delete。
-- 回滚说明：Down 会先删除该权限的 role_menus 关联，再删除菜单权限节点；若上线后人工授权过该权限，回滚会撤销这些授权。
-- 执行约束：本文件仅为 Migration 草案，取得用户明确批准前不得执行。
-- 验证 SQL（Up 获得执行批准并完成后，通过 DBHub 或 MySQL 客户端只读执行）：
--   SELECT version_id, is_applied FROM goose_db_version ORDER BY id DESC LIMIT 3;
--   SELECT id, parent_id, name, node_type, scope, path, component, icon,
--          permission_code, tenant_assignable, sort, visible, status
--   FROM menus
--   WHERE id = 1048 OR permission_code = 'platform:tenant:delete';
--   预期仅返回 1 行，id=1048，parent_id=1001，scope='platform'，tenant_assignable=0，sort=7，visible=0，status=1。
--   SELECT COUNT(*) AS role_grants
--   FROM role_menus
--   WHERE menu_id = 1048;
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
  (1048, 1001, '删除租户', 'permission', 'platform', NULL, NULL, NULL, 'platform:tenant:delete', 0, 7, 0, 1);

-- +goose Down

DELETE FROM `role_menus`
WHERE `menu_id` = 1048;

DELETE FROM `menus`
WHERE `id` = 1048
  AND `permission_code` = 'platform:tenant:delete';
