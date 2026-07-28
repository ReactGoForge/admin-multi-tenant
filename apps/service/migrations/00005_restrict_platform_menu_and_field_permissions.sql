-- 变更原因：菜单管理和字典管理只允许平台超级管理员使用，同时收敛租户菜单分配能力并更新展示名称。
-- 影响范围：调整 platform_admin 的六条 role_menus 关联，以及 menus 中两个既有系统节点的配置。
-- 兼容性：不修改表结构、字段、索引、路由或权限编码；平台超级管理员仍由服务动态放行，不依赖 role_menus。
-- 数据处理：Up 删除六条内置角色权限关联、关闭租户菜单可分配标记，并将字段管理更名为字典管理。
-- 回滚说明：Down 恢复当前 00004 定义的六条关联、租户菜单可分配标记和字段管理原名称。
-- 执行约束：本 Migration 只生成草案，必须取得用户明确批准后才能执行。
-- 验证 SQL（执行获批并完成后，通过 DBHub 只读执行）：
--   SELECT r.system_key, m.permission_code
--   FROM role_menus AS rm
--   JOIN roles AS r ON r.id = rm.role_id
--   JOIN menus AS m ON m.id = rm.menu_id
--   WHERE r.scope = 'platform'
--     AND r.system_key = 'platform_admin'
--     AND (m.permission_code LIKE 'platform:menu:%' OR m.permission_code LIKE 'platform:field:%');
--   预期 Up 后返回 0 行，Down 后返回 6 行。
--   SELECT permission_code, name, tenant_assignable
--   FROM menus
--   WHERE permission_code IN ('platform:field:view', 'tenant:menu:view');
--   预期 Up 后 platform:field:view 名称为“字典管理”，tenant:menu:view 的 tenant_assignable 为 0；Down 后恢复。

-- +goose Up

DELETE rm
FROM `role_menus` AS rm
JOIN `roles` AS r ON r.`id` = rm.`role_id`
JOIN `menus` AS m ON m.`id` = rm.`menu_id`
WHERE rm.`scope` = 'platform'
  AND rm.`tenant_id` IS NULL
  AND r.`scope` = 'platform'
  AND r.`tenant_id` IS NULL
  AND r.`system_key` = 'platform_admin'
  AND m.`scope` = 'platform'
  AND m.`permission_code` IN (
    'platform:menu:view',
    'platform:menu:create',
    'platform:menu:edit',
    'platform:menu:status',
    'platform:menu:delete',
    'platform:field:view'
  );

UPDATE `menus`
SET `tenant_assignable` = 0
WHERE `scope` = 'tenant'
  AND `permission_code` = 'tenant:menu:view';

UPDATE `menus`
SET `name` = '字典管理'
WHERE `scope` = 'platform'
  AND `permission_code` = 'platform:field:view';

-- +goose Down

UPDATE `menus`
SET `name` = '字段管理'
WHERE `scope` = 'platform'
  AND `permission_code` = 'platform:field:view';

UPDATE `menus`
SET `tenant_assignable` = 1
WHERE `scope` = 'tenant'
  AND `permission_code` = 'tenant:menu:view';

INSERT INTO `role_menus` (`scope`, `tenant_id`, `role_id`, `menu_id`)
SELECT 'platform', NULL, r.`id`, m.`id`
FROM `roles` AS r
JOIN `menus` AS m
  ON m.`scope` = 'platform'
  AND m.`permission_code` IN (
    'platform:menu:view',
    'platform:menu:create',
    'platform:menu:edit',
    'platform:menu:status',
    'platform:menu:delete',
    'platform:field:view'
  )
WHERE r.`scope` = 'platform'
  AND r.`tenant_id` IS NULL
  AND r.`system_key` = 'platform_admin'
  AND NOT EXISTS (
    SELECT 1
    FROM `role_menus` AS existing_rm
    WHERE existing_rm.`role_id` = r.`id`
      AND existing_rm.`menu_id` = m.`id`
  );
