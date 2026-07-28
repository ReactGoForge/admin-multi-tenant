-- 变更原因：为平台端和租户端增加独立登录日志查询入口，并登记对应页面查看权限。
-- 影响范围：仅向 menus 新增两个固定页面节点，并把租户登录日志初始授予全部 tenant_owner；不新增日志表、不修改 system_logs 或历史日志。
-- 兼容性：登录日志继续复用 system_logs 的三十天清理机制；旧版服务会忽略新菜单，升级后的前端和服务按新权限控制页面及接口。
-- 数据处理：Up 不向平台普通角色、自定义租户角色或 platform_role_tenant_menus 代管角色授权；仅为全部现有 tenant_owner 写入租户登录日志关联。
-- 安全边界：菜单权限只控制查询入口，租户接口仍必须从认证上下文强制取得 tenant_id，不能接受客户端指定其他租户范围。
-- 回滚说明：Down 会先删除 role_menus 和 platform_role_tenant_menus 中的两个登录日志关联，再删除菜单节点；system_logs 中已经产生的登录事件继续按原保留规则存在。
-- 回滚风险：Down 会丢失上线后人工配置的登录日志角色和代管权限关联，执行前必须再次批准并记录或备份相关配置。
-- 执行约束：本文件仅为 Migration 草案，取得用户明确批准前不得执行。
-- 验证 SQL（Up 获得执行批准并完成后，通过 DBHub 或 MySQL 客户端只读执行）：
--   SELECT version_id, is_applied FROM goose_db_version ORDER BY id DESC LIMIT 4;
--   SELECT id, parent_id, name, node_type, scope, path, component, icon,
--          permission_code, tenant_assignable, sort, visible, status
--   FROM menus WHERE id IN (1047, 2033) ORDER BY id;
--   SELECT r.scope, r.tenant_id, r.id AS role_id, r.system_key, rm.menu_id
--   FROM role_menus AS rm JOIN roles AS r ON r.id = rm.role_id
--   WHERE rm.menu_id IN (1047, 2033) ORDER BY rm.menu_id, r.id;
--   SELECT COUNT(*) AS unexpected_platform_role_grants
--   FROM role_menus AS rm JOIN roles AS r ON r.id = rm.role_id
--   WHERE rm.menu_id = 1047;
--   SELECT COUNT(*) AS unexpected_tenant_role_grants
--   FROM role_menus AS rm JOIN roles AS r ON r.id = rm.role_id
--   WHERE rm.menu_id = 2033
--     AND (r.scope <> 'tenant' OR r.system_key <> 'tenant_owner');
--   SELECT COUNT(*) AS unexpected_managed_role_grants
--   FROM platform_role_tenant_menus WHERE menu_id IN (1047, 2033);
--   预期平台普通角色关联为 0，租户关联仅属于 tenant_owner，代管角色关联为 0。

-- +goose Up

INSERT INTO `menus` (
  `id`, `parent_id`, `name`, `node_type`, `scope`, `path`, `component`, `icon`,
  `permission_code`, `tenant_assignable`, `sort`, `visible`, `status`
) VALUES
  (1047, 1043, '登录日志', 'menu', 'platform', '/platform/logs/login', 'pages/platform/logs/login/index.tsx', 'LoginOutlined', 'platform:login-log:view', 0, 30, 1, 1),
  (2033, 2030, '登录日志', 'menu', 'tenant', '/tenant/logs/login', 'pages/tenant/logs/login/index.tsx', 'LoginOutlined', 'tenant:login-log:view', 1, 20, 1, 1);

INSERT INTO `role_menus` (`scope`, `tenant_id`, `role_id`, `menu_id`)
SELECT 'tenant', r.`tenant_id`, r.`id`, 2033
FROM `roles` AS r
LEFT JOIN `role_menus` AS rm
  ON rm.`role_id` = r.`id`
 AND rm.`menu_id` = 2033
WHERE r.`scope` = 'tenant'
  AND r.`tenant_id` IS NOT NULL
  AND r.`system_key` = 'tenant_owner'
  AND rm.`role_id` IS NULL;

-- +goose Down

DELETE FROM `platform_role_tenant_menus`
WHERE `menu_id` IN (1047, 2033);

DELETE FROM `role_menus`
WHERE `menu_id` IN (1047, 2033);

DELETE FROM `menus`
WHERE (`id` = 1047 AND `scope` = 'platform' AND `permission_code` = 'platform:login-log:view')
   OR (`id` = 2033 AND `scope` = 'tenant' AND `permission_code` = 'tenant:login-log:view');
