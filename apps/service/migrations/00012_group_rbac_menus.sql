-- 变更原因：在平台端和租户端的系统设置下增加“权限管理”目录，统一收纳 RBAC 菜单。
-- 影响范围：仅新增两个 menus 目录节点并调整八个既有菜单节点的 parent_id，不修改表结构、权限编码或角色有效权限。
-- 兼容性：前端菜单树、权限树和服务端菜单校验均已支持递归目录；现有页面路由和操作权限节点保持不变。
-- 数据处理：不修改业务数据；新增目录不需要角色关联，导航会按子菜单的实时权限决定是否显示。
-- 回滚说明：先恢复八个菜单的原父节点，再删除新目录产生的无权限角色关联并删除目录节点。
-- 执行约束：本 Migration 只生成草案，必须取得用户明确批准后才能执行。
-- 验证 SQL（执行获批并完成后，通过 DBHub 只读执行）：
--   SELECT id, parent_id, name, node_type, scope, icon, sort, visible, status
--   FROM menus
--   WHERE id IN (1042, 2029, 1003, 1010, 1017, 1022, 2002, 2009, 2016, 2017)
--   ORDER BY scope, parent_id, sort, id;

-- +goose Up

INSERT INTO `menus` (
  `id`, `parent_id`, `name`, `node_type`, `scope`, `path`, `component`, `icon`,
  `permission_code`, `tenant_assignable`, `sort`, `visible`, `status`
) VALUES
  (1042, 1002, '权限管理', 'directory', 'platform', NULL, NULL, 'SafetyCertificateOutlined', NULL, 0, 10, 1, 1),
  (2029, 2001, '权限管理', 'directory', 'tenant', NULL, NULL, 'SafetyCertificateOutlined', NULL, 1, 10, 1, 1);

UPDATE `menus`
SET `parent_id` = 1042
WHERE `id` IN (1003, 1010, 1017, 1022)
  AND `scope` = 'platform'
  AND `parent_id` = 1002;

UPDATE `menus`
SET `parent_id` = 2029
WHERE `id` IN (2002, 2009, 2016, 2017)
  AND `scope` = 'tenant'
  AND `parent_id` = 2001;

-- +goose Down

UPDATE `menus`
SET `parent_id` = 1002
WHERE `id` IN (1003, 1010, 1017, 1022)
  AND `scope` = 'platform'
  AND `parent_id` = 1042;

UPDATE `menus`
SET `parent_id` = 2001
WHERE `id` IN (2002, 2009, 2016, 2017)
  AND `scope` = 'tenant'
  AND `parent_id` = 2029;

DELETE FROM `platform_role_tenant_menus`
WHERE `menu_id` = 2029;

DELETE FROM `role_menus`
WHERE `menu_id` IN (1042, 2029);

DELETE FROM `menus`
WHERE `id` IN (1042, 2029);
