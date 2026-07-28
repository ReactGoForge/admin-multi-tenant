-- 变更原因：将租户内置最高权限角色展示为“企业管理员”，并把原动态全权限改为可配置权限快照。
-- 影响范围：更新 tenant_owner 角色名称，并向 role_menus 回填当前可分配租户节点；不修改表结构、字段、索引或约束。
-- 兼容性：服务端部署前必须先执行 Up，避免企业管理员因缺少 role_menus 关联而失去全部权限；tenant:menu:* 默认不授予但可后续手动配置。
-- 数据处理：现有企业管理员会获得全部 tenant_assignable=1 的租户节点，但排除 permission_code 以 tenant:menu: 开头的菜单管理节点。
-- 回滚说明：Down 会恢复旧角色名并删除企业管理员的全部权限关联；这会丢失上线后人工配置的权限快照，且必须与旧版动态授权服务同时回滚。
-- 验证 SQL（Up 获得批准并执行后，通过 DBHub 或 MySQL 客户端只读执行）：
--   SELECT id, tenant_id, name, system_key FROM roles WHERE scope = 'tenant' AND system_key = 'tenant_owner' ORDER BY id;
--   SELECT r.tenant_id, r.id AS role_id, COUNT(rm.menu_id) AS permission_count
--   FROM roles r LEFT JOIN role_menus rm ON rm.role_id = r.id
--   WHERE r.scope = 'tenant' AND r.system_key = 'tenant_owner'
--   GROUP BY r.tenant_id, r.id ORDER BY r.tenant_id;
--   SELECT r.tenant_id, m.permission_code
--   FROM roles r JOIN role_menus rm ON rm.role_id = r.id JOIN menus m ON m.id = rm.menu_id
--   WHERE r.scope = 'tenant' AND r.system_key = 'tenant_owner' AND m.permission_code LIKE 'tenant:menu:%';

-- +goose Up

UPDATE roles
SET name = '企业管理员'
WHERE scope = 'tenant'
  AND system_key = 'tenant_owner';

INSERT INTO role_menus (scope, tenant_id, role_id, menu_id)
SELECT 'tenant', r.tenant_id, r.id, m.id
FROM roles AS r
JOIN menus AS m
  ON m.scope = 'tenant'
 AND m.tenant_assignable = 1
 AND (m.permission_code IS NULL OR m.permission_code NOT LIKE 'tenant:menu:%')
LEFT JOIN role_menus AS rm
  ON rm.role_id = r.id
 AND rm.menu_id = m.id
WHERE r.scope = 'tenant'
  AND r.system_key = 'tenant_owner'
  AND rm.role_id IS NULL;

-- +goose Down

DELETE rm
FROM role_menus AS rm
JOIN roles AS r ON r.id = rm.role_id
WHERE r.scope = 'tenant'
  AND r.system_key = 'tenant_owner';

UPDATE roles
SET name = '租户所有者'
WHERE scope = 'tenant'
  AND system_key = 'tenant_owner';
