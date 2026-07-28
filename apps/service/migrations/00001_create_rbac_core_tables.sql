-- 变更原因：建立第一版多租户 RBAC 所需的租户、员工、部门、角色和菜单结构。
-- 影响范围：仅新增表、字段、索引、检查约束和租户根外键，不修改现有表。
-- 兼容性：面向当前 MySQL 8.4，使用 InnoDB、utf8mb4 和 CHECK 约束。
-- 数据处理：本 Migration 不插入、更新、删除或回填任何业务数据。
-- 回滚说明：Down 会删除本 Migration 创建的五张空表；执行前必须先回滚依赖它们的关联表 Migration。
-- 验证 SQL（Up 获得执行批准并完成后，通过 DBHub 只读执行）：
--   SHOW CREATE TABLE tenants;
--   SHOW CREATE TABLE employees;
--   SHOW CREATE TABLE departments;
--   SHOW CREATE TABLE roles;
--   SHOW CREATE TABLE menus;
--   SHOW INDEX FROM employees;
--   SHOW INDEX FROM roles;
--   SHOW INDEX FROM menus;

-- +goose Up

CREATE TABLE `tenants` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(100) NOT NULL,
  `status` TINYINT UNSIGNED NOT NULL DEFAULT 1,
  `owner_employee_id` BIGINT UNSIGNED NULL,
  PRIMARY KEY (`id`),
  KEY `idx_tenants_owner_employee` (`owner_employee_id`),
  CONSTRAINT `chk_tenants_status` CHECK (`status` IN (0, 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `employees` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `scope` VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `tenant_id` BIGINT UNSIGNED NULL,
  `department_id` BIGINT UNSIGNED NULL,
  `name` VARCHAR(30) NOT NULL,
  `login_account` VARCHAR(40) NOT NULL,
  `password_hash` VARCHAR(255) NOT NULL,
  `phone` VARCHAR(20) NULL,
  `status` TINYINT UNSIGNED NOT NULL DEFAULT 1,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_employees_login_account` (`login_account`),
  UNIQUE KEY `uk_employees_id_tenant` (`id`, `tenant_id`),
  KEY `idx_employees_tenant_scope_status` (`tenant_id`, `scope`, `status`, `id`),
  KEY `idx_employees_tenant_scope_department` (`tenant_id`, `scope`, `department_id`),
  CONSTRAINT `fk_employees_tenant`
    FOREIGN KEY (`tenant_id`) REFERENCES `tenants` (`id`)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT `chk_employees_scope_tenant` CHECK (
    (`scope` = 'platform' AND `tenant_id` IS NULL)
    OR (`scope` = 'tenant' AND `tenant_id` IS NOT NULL)
  ),
  CONSTRAINT `chk_employees_status` CHECK (`status` IN (0, 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `departments` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `scope` VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `tenant_id` BIGINT UNSIGNED NULL,
  `parent_id` BIGINT UNSIGNED NULL,
  `name` VARCHAR(40) NOT NULL,
  `leader_employee_id` BIGINT UNSIGNED NULL,
  `sort` INT UNSIGNED NOT NULL,
  `status` TINYINT UNSIGNED NOT NULL DEFAULT 1,
  PRIMARY KEY (`id`),
  KEY `idx_departments_tenant_scope_tree` (`tenant_id`, `scope`, `parent_id`, `sort`, `id`),
  CONSTRAINT `fk_departments_tenant`
    FOREIGN KEY (`tenant_id`) REFERENCES `tenants` (`id`)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT `chk_departments_scope_tenant` CHECK (
    (`scope` = 'platform' AND `tenant_id` IS NULL)
    OR (`scope` = 'tenant' AND `tenant_id` IS NOT NULL)
  ),
  CONSTRAINT `chk_departments_status` CHECK (`status` IN (0, 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `roles` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `scope` VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `tenant_id` BIGINT UNSIGNED NULL,
  `name` VARCHAR(30) NOT NULL,
  `description` VARCHAR(200) NULL,
  `system_key` VARCHAR(40) CHARACTER SET ascii COLLATE ascii_bin NULL,
  `status` TINYINT UNSIGNED NOT NULL DEFAULT 1,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_roles_tenant_system_key` (`tenant_id`, `system_key`),
  UNIQUE KEY `uk_roles_id_tenant` (`id`, `tenant_id`),
  KEY `idx_roles_tenant_scope_status` (`tenant_id`, `scope`, `status`, `id`),
  CONSTRAINT `fk_roles_tenant`
    FOREIGN KEY (`tenant_id`) REFERENCES `tenants` (`id`)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT `chk_roles_scope_tenant` CHECK (
    (`scope` = 'platform' AND `tenant_id` IS NULL)
    OR (`scope` = 'tenant' AND `tenant_id` IS NOT NULL)
  ),
  CONSTRAINT `chk_roles_system_key` CHECK (
    `system_key` IS NULL
    OR (
      `scope` = 'platform'
      AND `tenant_id` IS NULL
      AND `system_key` IN ('platform_super_admin', 'platform_admin')
    )
    OR (
      `scope` = 'tenant'
      AND `tenant_id` IS NOT NULL
      AND `system_key` = 'tenant_owner'
    )
  ),
  CONSTRAINT `chk_roles_status` CHECK (`status` IN (0, 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `menus` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `parent_id` BIGINT UNSIGNED NULL,
  `name` VARCHAR(40) NOT NULL,
  `node_type` VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `scope` VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `path` VARCHAR(255) NULL,
  `component` VARCHAR(255) NULL,
  `icon` VARCHAR(64) NULL,
  `permission_code` VARCHAR(100) CHARACTER SET ascii COLLATE ascii_bin NULL,
  `tenant_assignable` TINYINT UNSIGNED NOT NULL,
  `sort` INT UNSIGNED NOT NULL,
  `visible` TINYINT UNSIGNED NOT NULL,
  `status` TINYINT UNSIGNED NOT NULL DEFAULT 1,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_menus_permission_code` (`permission_code`),
  UNIQUE KEY `uk_menus_scope_path` (`scope`, `path`),
  UNIQUE KEY `uk_menus_id_scope` (`id`, `scope`),
  KEY `idx_menus_scope_tree` (`scope`, `parent_id`, `sort`, `id`),
  CONSTRAINT `chk_menus_node_type` CHECK (
    `node_type` IN ('directory', 'menu', 'permission')
  ),
  CONSTRAINT `chk_menus_scope` CHECK (`scope` IN ('platform', 'tenant')),
  CONSTRAINT `chk_menus_tenant_assignable` CHECK (`tenant_assignable` IN (0, 1)),
  CONSTRAINT `chk_menus_visible` CHECK (`visible` IN (0, 1)),
  CONSTRAINT `chk_menus_status` CHECK (`status` IN (0, 1)),
  CONSTRAINT `chk_menus_platform_assignable` CHECK (
    `scope` = 'tenant' OR `tenant_assignable` = 0
  ),
  CONSTRAINT `chk_menus_node_fields` CHECK (
    (
      `node_type` = 'directory'
      AND `component` IS NULL
      AND `permission_code` IS NULL
    )
    OR (
      `node_type` = 'menu'
      AND `path` IS NOT NULL
      AND `component` IS NOT NULL
      AND `permission_code` IS NOT NULL
    )
    OR (
      `node_type` = 'permission'
      AND `path` IS NULL
      AND `component` IS NULL
      AND `permission_code` IS NOT NULL
      AND `visible` = 0
    )
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down

DROP TABLE `menus`;
DROP TABLE `roles`;
DROP TABLE `departments`;
DROP TABLE `employees`;
DROP TABLE `tenants`;
