-- 变更原因：初始化第一版 RBAC 所需的系统菜单定义、平台内置角色和平台管理员权限。
-- 影响范围：仅向 menus、roles、role_menus 写入系统定义数据，不修改表结构、索引、外键或检查约束。
-- 兼容性：依赖 00001、00002、00003 已创建并补充注释的 RBAC 表；面向当前 MySQL 8.4。
-- 数据处理：新增 49 个菜单节点、2 个平台内置角色和 28 条平台管理员权限关联。
-- 固定标识：平台菜单使用 1001-1028，租户菜单使用 2001-2021；平台角色使用 1001-1002。
-- 不包含：租户、部门、员工、员工角色、租户所有者、演示账号、密码或其他业务数据。
-- 回滚说明：Down 按 role_menus、roles、menus 的依赖逆序删除本 Migration 的固定 ID 数据，不使用级联删除。
-- 回滚保护：若后续业务数据已引用这些角色或菜单，外键会阻止回滚，届时必须先评估并人工解除引用。
-- 验证 SQL（Up 获得执行批准并完成后，通过 DBHub 只读执行）：
--   SELECT version_id, is_applied FROM goose_db_version ORDER BY id;
--   SELECT scope, COUNT(*) FROM menus GROUP BY scope ORDER BY scope;
--   SELECT node_type, COUNT(*) FROM menus GROUP BY node_type ORDER BY node_type;
--   SELECT id, name, scope, system_key, status FROM roles ORDER BY id;
--   SELECT role_id, scope, COUNT(*) FROM role_menus GROUP BY role_id, scope ORDER BY role_id, scope;
--   SELECT COUNT(*) FROM role_menus WHERE role_id = 1001;
--   SELECT COUNT(*) FROM role_menus WHERE role_id = 1002;
--   SELECT COUNT(*) FROM tenants;
--   SELECT COUNT(*) FROM employees;
--   SELECT COUNT(*) FROM departments;
--   SELECT COUNT(*) FROM employee_roles;

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
  (1001, NULL, '租户管理', 'menu', 'platform', '/platform/tenants', 'pages/platform/tenants/index.tsx', 'ApartmentOutlined', 'platform:tenant:view', 0, 10, 1, 1),
  (1002, NULL, '系统设置', 'directory', 'platform', NULL, NULL, 'SettingOutlined', NULL, 0, 20, 1, 1),
  (1003, 1002, '员工管理', 'menu', 'platform', '/platform/system/employees', 'pages/platform/system/employees/index.tsx', 'TeamOutlined', 'platform:employee:view', 0, 10, 1, 1),
  (1004, 1003, '新增员工', 'permission', 'platform', NULL, NULL, NULL, 'platform:employee:create', 0, 1, 0, 1),
  (1005, 1003, '编辑员工', 'permission', 'platform', NULL, NULL, NULL, 'platform:employee:edit', 0, 2, 0, 1),
  (1006, 1003, '分配角色', 'permission', 'platform', NULL, NULL, NULL, 'platform:employee:assign-role', 0, 3, 0, 1),
  (1007, 1003, '重置密码', 'permission', 'platform', NULL, NULL, NULL, 'platform:employee:reset-password', 0, 4, 0, 1),
  (1008, 1003, '启用或禁用员工', 'permission', 'platform', NULL, NULL, NULL, 'platform:employee:status', 0, 5, 0, 1),
  (1009, 1003, '删除员工', 'permission', 'platform', NULL, NULL, NULL, 'platform:employee:delete', 0, 6, 0, 1),
  (1010, 1002, '角色管理', 'menu', 'platform', '/platform/system/roles', 'pages/platform/system/roles/index.tsx', 'SafetyCertificateOutlined', 'platform:role:view', 0, 20, 1, 1),
  (1011, 1010, '新增角色', 'permission', 'platform', NULL, NULL, NULL, 'platform:role:create', 0, 1, 0, 1),
  (1012, 1010, '编辑角色', 'permission', 'platform', NULL, NULL, NULL, 'platform:role:edit', 0, 2, 0, 1),
  (1013, 1010, '配置权限', 'permission', 'platform', NULL, NULL, NULL, 'platform:role:permission', 0, 3, 0, 1),
  (1014, 1010, '查看员工', 'permission', 'platform', NULL, NULL, NULL, 'platform:role:employees', 0, 4, 0, 1),
  (1015, 1010, '启用或禁用角色', 'permission', 'platform', NULL, NULL, NULL, 'platform:role:status', 0, 5, 0, 1),
  (1016, 1010, '删除角色', 'permission', 'platform', NULL, NULL, NULL, 'platform:role:delete', 0, 6, 0, 1),
  (1017, 1002, '菜单管理', 'menu', 'platform', '/platform/system/menus', 'pages/platform/system/menus/index.tsx', 'MenuOutlined', 'platform:menu:view', 0, 30, 1, 1),
  (1018, 1017, '新增菜单', 'permission', 'platform', NULL, NULL, NULL, 'platform:menu:create', 0, 1, 0, 1),
  (1019, 1017, '编辑菜单', 'permission', 'platform', NULL, NULL, NULL, 'platform:menu:edit', 0, 2, 0, 1),
  (1020, 1017, '启用或禁用菜单', 'permission', 'platform', NULL, NULL, NULL, 'platform:menu:status', 0, 3, 0, 1),
  (1021, 1017, '删除菜单', 'permission', 'platform', NULL, NULL, NULL, 'platform:menu:delete', 0, 4, 0, 1),
  (1022, 1002, '部门管理', 'menu', 'platform', '/platform/system/departments', 'pages/platform/system/departments/index.tsx', 'ClusterOutlined', 'platform:department:view', 0, 40, 1, 1),
  (1023, 1022, '新增部门', 'permission', 'platform', NULL, NULL, NULL, 'platform:department:create', 0, 1, 0, 1),
  (1024, 1022, '编辑部门', 'permission', 'platform', NULL, NULL, NULL, 'platform:department:edit', 0, 2, 0, 1),
  (1025, 1022, '删除部门', 'permission', 'platform', NULL, NULL, NULL, 'platform:department:delete', 0, 3, 0, 1),
  (1026, 1002, '基础设置', 'menu', 'platform', '/platform/system/basic', 'pages/platform/system/basic/index.tsx', 'ToolOutlined', 'platform:basic:view', 0, 50, 1, 1),
  (1027, 1002, '字段管理', 'menu', 'platform', '/platform/system/fields', 'pages/platform/system/fields/index.tsx', 'FormOutlined', 'platform:field:view', 0, 60, 1, 1),
  (1028, NULL, '用户', 'menu', 'platform', '/platform/users', 'pages/platform/users/index.tsx', 'UserOutlined', 'platform:user:view', 0, 30, 1, 1),
  (2001, NULL, '系统设置', 'directory', 'tenant', NULL, NULL, 'SettingOutlined', NULL, 1, 10, 1, 1),
  (2002, 2001, '员工管理', 'menu', 'tenant', '/tenant/system/employees', 'pages/tenant/system/employees/index.tsx', 'TeamOutlined', 'tenant:employee:view', 1, 10, 1, 1),
  (2003, 2002, '新增员工', 'permission', 'tenant', NULL, NULL, NULL, 'tenant:employee:create', 1, 1, 0, 1),
  (2004, 2002, '编辑员工', 'permission', 'tenant', NULL, NULL, NULL, 'tenant:employee:edit', 1, 2, 0, 1),
  (2005, 2002, '分配角色', 'permission', 'tenant', NULL, NULL, NULL, 'tenant:employee:assign-role', 1, 3, 0, 1),
  (2006, 2002, '重置密码', 'permission', 'tenant', NULL, NULL, NULL, 'tenant:employee:reset-password', 1, 4, 0, 1),
  (2007, 2002, '启用或禁用员工', 'permission', 'tenant', NULL, NULL, NULL, 'tenant:employee:status', 1, 5, 0, 1),
  (2008, 2002, '删除员工', 'permission', 'tenant', NULL, NULL, NULL, 'tenant:employee:delete', 1, 6, 0, 1),
  (2009, 2001, '角色管理', 'menu', 'tenant', '/tenant/system/roles', 'pages/tenant/system/roles/index.tsx', 'SafetyCertificateOutlined', 'tenant:role:view', 1, 20, 1, 1),
  (2010, 2009, '新增角色', 'permission', 'tenant', NULL, NULL, NULL, 'tenant:role:create', 1, 1, 0, 1),
  (2011, 2009, '编辑角色', 'permission', 'tenant', NULL, NULL, NULL, 'tenant:role:edit', 1, 2, 0, 1),
  (2012, 2009, '配置权限', 'permission', 'tenant', NULL, NULL, NULL, 'tenant:role:permission', 1, 3, 0, 1),
  (2013, 2009, '查看员工', 'permission', 'tenant', NULL, NULL, NULL, 'tenant:role:employees', 1, 4, 0, 1),
  (2014, 2009, '启用或禁用角色', 'permission', 'tenant', NULL, NULL, NULL, 'tenant:role:status', 1, 5, 0, 1),
  (2015, 2009, '删除角色', 'permission', 'tenant', NULL, NULL, NULL, 'tenant:role:delete', 1, 6, 0, 1),
  (2016, 2001, '菜单', 'menu', 'tenant', '/tenant/system/menus', 'pages/tenant/system/menus/index.tsx', 'MenuOutlined', 'tenant:menu:view', 1, 30, 1, 1),
  (2017, 2001, '部门管理', 'menu', 'tenant', '/tenant/system/departments', 'pages/tenant/system/departments/index.tsx', 'ClusterOutlined', 'tenant:department:view', 1, 40, 1, 1),
  (2018, 2017, '新增部门', 'permission', 'tenant', NULL, NULL, NULL, 'tenant:department:create', 1, 1, 0, 1),
  (2019, 2017, '编辑部门', 'permission', 'tenant', NULL, NULL, NULL, 'tenant:department:edit', 1, 2, 0, 1),
  (2020, 2017, '删除部门', 'permission', 'tenant', NULL, NULL, NULL, 'tenant:department:delete', 1, 3, 0, 1),
  (2021, NULL, '用户', 'menu', 'tenant', '/tenant/users', 'pages/tenant/users/index.tsx', 'UserOutlined', 'tenant:user:view', 1, 20, 1, 1);

INSERT INTO `roles` (
  `id`,
  `scope`,
  `tenant_id`,
  `name`,
  `description`,
  `system_key`,
  `status`
) VALUES
  (1001, 'platform', NULL, '平台超级管理员', '系统所有者唯一账号', 'platform_super_admin', 1),
  (1002, 'platform', NULL, '平台管理员', '负责平台日常管理', 'platform_admin', 1);

INSERT INTO `role_menus` (`scope`, `tenant_id`, `role_id`, `menu_id`) VALUES
  ('platform', NULL, 1002, 1001),
  ('platform', NULL, 1002, 1002),
  ('platform', NULL, 1002, 1003),
  ('platform', NULL, 1002, 1004),
  ('platform', NULL, 1002, 1005),
  ('platform', NULL, 1002, 1006),
  ('platform', NULL, 1002, 1007),
  ('platform', NULL, 1002, 1008),
  ('platform', NULL, 1002, 1009),
  ('platform', NULL, 1002, 1010),
  ('platform', NULL, 1002, 1011),
  ('platform', NULL, 1002, 1012),
  ('platform', NULL, 1002, 1013),
  ('platform', NULL, 1002, 1014),
  ('platform', NULL, 1002, 1015),
  ('platform', NULL, 1002, 1016),
  ('platform', NULL, 1002, 1017),
  ('platform', NULL, 1002, 1018),
  ('platform', NULL, 1002, 1019),
  ('platform', NULL, 1002, 1020),
  ('platform', NULL, 1002, 1021),
  ('platform', NULL, 1002, 1022),
  ('platform', NULL, 1002, 1023),
  ('platform', NULL, 1002, 1024),
  ('platform', NULL, 1002, 1025),
  ('platform', NULL, 1002, 1026),
  ('platform', NULL, 1002, 1027),
  ('platform', NULL, 1002, 1028);

-- +goose Down

DELETE FROM `role_menus`
WHERE `scope` = 'platform'
  AND `tenant_id` IS NULL
  AND `role_id` = 1002
  AND `menu_id` BETWEEN 1001 AND 1028;

DELETE FROM `roles`
WHERE `scope` = 'platform'
  AND `tenant_id` IS NULL
  AND `id` IN (1001, 1002)
  AND `system_key` IN ('platform_super_admin', 'platform_admin');

DELETE FROM `menus`
WHERE (`scope` = 'platform' AND `id` BETWEEN 1001 AND 1028)
   OR (`scope` = 'tenant' AND `id` BETWEEN 2001 AND 2021);
