-- 变更原因：允许租户角色和平台代管角色按需获得租户菜单查看权限。
-- 影响范围：仅更新 menus 中既有 tenant:menu:view 节点的 tenant_assignable 标记。
-- 兼容性：不修改表结构、字段、索引、外键、权限编码或接口；现有角色不会自动新增权限关联。
-- 数据处理：Up 不写入 role_menus 或 platform_role_tenant_menus，不回填任何角色权限。
-- 回滚说明：Down 会先删除 tenant:menu:view 已产生的租户角色和平台代管角色关联，再恢复不可分配标记。
-- 回滚风险：删除权限关联会撤销已经人工分配的租户菜单权限，执行 Down 前必须再次确认。
-- 执行约束：本 Migration 只生成草案，必须取得用户明确批准后才能执行。
-- 验证 SQL（Up 获得执行批准并完成后，通过 DBHub 或 MySQL 客户端只读执行）：
--   SELECT version_id, is_applied FROM goose_db_version ORDER BY id DESC LIMIT 3;
--   SELECT id, name, scope, permission_code, tenant_assignable, status
--   FROM menus WHERE scope = 'tenant' AND permission_code = 'tenant:menu:view';
--   SELECT COUNT(*) AS tenant_role_grants
--   FROM role_menus AS rm JOIN menus AS m ON m.id = rm.menu_id
--   WHERE m.scope = 'tenant' AND m.permission_code = 'tenant:menu:view';
--   SELECT COUNT(*) AS managed_role_grants
--   FROM platform_role_tenant_menus AS prtm JOIN menus AS m ON m.id = prtm.menu_id
--   WHERE m.scope = 'tenant' AND m.permission_code = 'tenant:menu:view';
--   预期刚执行 Up 时 tenant_assignable = 1，两个权限关联数量均未被本 Migration 增加。

-- +goose Up

UPDATE `menus`
SET `tenant_assignable` = 1
WHERE `scope` = 'tenant'
  AND `permission_code` = 'tenant:menu:view';

-- +goose Down

DELETE prtm
FROM `platform_role_tenant_menus` AS prtm
JOIN `menus` AS m ON m.`id` = prtm.`menu_id`
WHERE m.`scope` = 'tenant'
  AND m.`permission_code` = 'tenant:menu:view';

DELETE rm
FROM `role_menus` AS rm
JOIN `menus` AS m ON m.`id` = rm.`menu_id`
WHERE m.`scope` = 'tenant'
  AND m.`permission_code` = 'tenant:menu:view';

UPDATE `menus`
SET `tenant_assignable` = 0
WHERE `scope` = 'tenant'
  AND `permission_code` = 'tenant:menu:view';
