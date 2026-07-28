-- 变更原因：将平台端和租户端首页登记为可由角色分配的正式菜单节点，并统一首页名称与权限编码。
-- 影响范围：仅向 menus 新增两个固定节点，并向 role_menus 写入内置角色的初始关联；不修改表结构、字段、索引、外键或接口。
-- 兼容性：前端在本 Migration 执行前仍兼容既有首页路由；执行后首页按普通页面权限控制，未授权角色不会显示且直达返回 403。
-- 数据处理：Up 仅把平台首页授予 platform_admin，把租户首页授予所有 tenant_owner；不回填自定义角色或 platform_role_tenant_menus 代管权限。
-- 回滚说明：Down 会先删除 role_menus 和 platform_role_tenant_menus 中的首页关联，再删除首页菜单节点。
-- 回滚风险：Down 会丢失上线后人工配置的首页角色关联，执行前必须再次确认并记录或备份相关配置。
-- 执行约束：本文件仅为 Migration 草案，取得用户明确批准前不得执行。
-- 验证 SQL（Up 获得执行批准并完成后，通过 DBHub 或 MySQL 客户端只读执行）：
--   SELECT version_id, is_applied FROM goose_db_version ORDER BY id DESC LIMIT 3;
--   SELECT id, parent_id, name, node_type, scope, path, component, icon,
--          permission_code, tenant_assignable, sort, visible, status
--   FROM menus WHERE id IN (1046, 2032) ORDER BY id;
--   SELECT r.scope, r.tenant_id, r.id AS role_id, r.system_key, rm.menu_id
--   FROM role_menus AS rm JOIN roles AS r ON r.id = rm.role_id
--   WHERE rm.menu_id IN (1046, 2032) ORDER BY rm.menu_id, r.id;
--   SELECT COUNT(*) AS unexpected_custom_role_grants
--   FROM role_menus AS rm JOIN roles AS r ON r.id = rm.role_id
--   WHERE rm.menu_id IN (1046, 2032)
--     AND (r.system_key IS NULL OR r.system_key NOT IN ('platform_admin', 'tenant_owner'));
--   SELECT COUNT(*) AS unexpected_managed_role_grants
--   FROM platform_role_tenant_menus WHERE menu_id IN (1046, 2032);
--   预期两个 unexpected 计数均为 0；平台首页仅关联 platform_admin，租户首页仅关联各 tenant_owner。

-- +goose Up

INSERT INTO `menus` (
  `id`, `parent_id`, `name`, `node_type`, `scope`, `path`, `component`, `icon`,
  `permission_code`, `tenant_assignable`, `sort`, `visible`, `status`
) VALUES
  (1046, NULL, '首页', 'menu', 'platform', '/platform', 'router/modules/platform-index.tsx', 'HomeOutlined', 'platform:home:view', 0, 0, 1, 1),
  (2032, NULL, '首页', 'menu', 'tenant', '/tenant', 'router/modules/tenant-index.tsx', 'HomeOutlined', 'tenant:home:view', 1, 0, 1, 1);

INSERT INTO `role_menus` (`scope`, `tenant_id`, `role_id`, `menu_id`)
SELECT 'platform', NULL, r.`id`, 1046
FROM `roles` AS r
LEFT JOIN `role_menus` AS rm
  ON rm.`role_id` = r.`id`
 AND rm.`menu_id` = 1046
WHERE r.`scope` = 'platform'
  AND r.`tenant_id` IS NULL
  AND r.`system_key` = 'platform_admin'
  AND rm.`role_id` IS NULL;

INSERT INTO `role_menus` (`scope`, `tenant_id`, `role_id`, `menu_id`)
SELECT 'tenant', r.`tenant_id`, r.`id`, 2032
FROM `roles` AS r
LEFT JOIN `role_menus` AS rm
  ON rm.`role_id` = r.`id`
 AND rm.`menu_id` = 2032
WHERE r.`scope` = 'tenant'
  AND r.`tenant_id` IS NOT NULL
  AND r.`system_key` = 'tenant_owner'
  AND rm.`role_id` IS NULL;

-- +goose Down

DELETE FROM `platform_role_tenant_menus`
WHERE `menu_id` IN (1046, 2032);

DELETE FROM `role_menus`
WHERE `menu_id` IN (1046, 2032);

DELETE FROM `menus`
WHERE (`id` = 1046 AND `scope` = 'platform' AND `permission_code` = 'platform:home:view')
   OR (`id` = 2032 AND `scope` = 'tenant' AND `permission_code` = 'tenant:home:view');
