-- 变更原因：建立后台员工与角色、角色与菜单权限之间的多对多关系。
-- 影响范围：仅新增两张关联表及已审核的联合主键、索引、检查约束和核心外键。
-- 兼容性：依赖 00001 创建的 employees、roles、menus 及其组合唯一索引。
-- 数据处理：本 Migration 不插入、更新、删除或回填任何业务数据。
-- 回滚说明：Down 按依赖逆序删除两张关联表，不删除员工、角色或菜单数据。
-- 平台边界：tenant_id 为 NULL 时，MySQL 组合外键不会校验 tenant_id 部分，平台写入仍须由服务校验 scope。
-- 验证 SQL（Up 获得执行批准并完成后，通过 DBHub 只读执行）：
--   SHOW CREATE TABLE employee_roles;
--   SHOW CREATE TABLE role_menus;
--   SHOW INDEX FROM employee_roles;
--   SHOW INDEX FROM role_menus;
--   SELECT CONSTRAINT_NAME, TABLE_NAME, REFERENCED_TABLE_NAME
--   FROM information_schema.REFERENTIAL_CONSTRAINTS
--   WHERE CONSTRAINT_SCHEMA = DATABASE()
--     AND TABLE_NAME IN ('employee_roles', 'role_menus');

-- +goose Up

CREATE TABLE `employee_roles` (
  `scope` VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `tenant_id` BIGINT UNSIGNED NULL,
  `employee_id` BIGINT UNSIGNED NOT NULL,
  `role_id` BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (`employee_id`, `role_id`),
  KEY `idx_employee_roles_tenant_scope_role` (`tenant_id`, `scope`, `role_id`, `employee_id`),
  KEY `idx_employee_roles_employee_tenant` (`employee_id`, `tenant_id`),
  KEY `idx_employee_roles_role_tenant` (`role_id`, `tenant_id`),
  CONSTRAINT `fk_employee_roles_employee`
    FOREIGN KEY (`employee_id`) REFERENCES `employees` (`id`)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT `fk_employee_roles_role`
    FOREIGN KEY (`role_id`) REFERENCES `roles` (`id`)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT `fk_employee_roles_employee_tenant`
    FOREIGN KEY (`employee_id`, `tenant_id`)
    REFERENCES `employees` (`id`, `tenant_id`)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT `fk_employee_roles_role_tenant`
    FOREIGN KEY (`role_id`, `tenant_id`)
    REFERENCES `roles` (`id`, `tenant_id`)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT `chk_employee_roles_scope_tenant` CHECK (
    (`scope` = 'platform' AND `tenant_id` IS NULL)
    OR (`scope` = 'tenant' AND `tenant_id` IS NOT NULL)
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `role_menus` (
  `scope` VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `tenant_id` BIGINT UNSIGNED NULL,
  `role_id` BIGINT UNSIGNED NOT NULL,
  `menu_id` BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (`role_id`, `menu_id`),
  KEY `idx_role_menus_tenant_scope_role` (`tenant_id`, `scope`, `role_id`, `menu_id`),
  KEY `idx_role_menus_menu_role` (`menu_id`, `role_id`),
  KEY `idx_role_menus_role_tenant` (`role_id`, `tenant_id`),
  KEY `idx_role_menus_menu_scope` (`menu_id`, `scope`),
  CONSTRAINT `fk_role_menus_role`
    FOREIGN KEY (`role_id`) REFERENCES `roles` (`id`)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT `fk_role_menus_menu`
    FOREIGN KEY (`menu_id`) REFERENCES `menus` (`id`)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT `fk_role_menus_role_tenant`
    FOREIGN KEY (`role_id`, `tenant_id`)
    REFERENCES `roles` (`id`, `tenant_id`)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT `fk_role_menus_menu_scope`
    FOREIGN KEY (`menu_id`, `scope`)
    REFERENCES `menus` (`id`, `scope`)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT `chk_role_menus_scope_tenant` CHECK (
    (`scope` = 'platform' AND `tenant_id` IS NULL)
    OR (`scope` = 'tenant' AND `tenant_id` IS NOT NULL)
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down

DROP TABLE `role_menus`;
DROP TABLE `employee_roles`;
