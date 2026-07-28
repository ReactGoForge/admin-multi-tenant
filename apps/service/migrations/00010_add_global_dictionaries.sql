-- 变更原因：为平台字典管理和全后台统一展示选项提供全局字典类型与字典项。
-- 影响范围：新增 dictionary_types、dictionary_items 两张表，并初始化四组受保护的系统字典。
-- 兼容性：不修改现有业务表、字段、索引或接口内部值；现有 enabled、platform 等稳定值保持不变。
-- 数据处理：仅插入系统字典定义，不更新、删除或回填任何现有业务数据。
-- 回滚说明：Down 会删除本 Migration 创建的两张字典表及其中全部数据；执行前需确认没有后续表引用它们。
-- 执行约束：本 Migration 只生成草案，必须取得用户明确批准后才能执行。
-- 验证 SQL（执行获批并完成后，通过 DBHub 只读执行）：
--   SELECT version_id, is_applied FROM goose_db_version ORDER BY id DESC LIMIT 1;
--   SHOW CREATE TABLE dictionary_types;
--   SHOW CREATE TABLE dictionary_items;
--   SHOW INDEX FROM dictionary_types;
--   SHOW INDEX FROM dictionary_items;
--   SELECT code, name, status, is_system FROM dictionary_types ORDER BY sort, id;
--   SELECT dt.code, di.label, di.value, di.sort, di.status
--   FROM dictionary_items AS di
--   JOIN dictionary_types AS dt ON dt.id = di.dictionary_type_id
--   ORDER BY dt.sort, dt.id, di.sort, di.id;

-- +goose Up

CREATE TABLE `dictionary_types` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `code` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `name` VARCHAR(50) NOT NULL,
  `remark` VARCHAR(200) NULL,
  `sort` INT UNSIGNED NOT NULL DEFAULT 0,
  `status` TINYINT UNSIGNED NOT NULL DEFAULT 1,
  `is_system` TINYINT UNSIGNED NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_dictionary_types_code` (`code`),
  KEY `idx_dictionary_types_status_sort` (`status`, `sort`, `id`),
  CONSTRAINT `chk_dictionary_types_status` CHECK (`status` IN (0, 1)),
  CONSTRAINT `chk_dictionary_types_is_system` CHECK (`is_system` IN (0, 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `dictionary_items` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `dictionary_type_id` BIGINT UNSIGNED NOT NULL,
  `label` VARCHAR(50) NOT NULL,
  `value` VARCHAR(100) NOT NULL,
  `sort` INT UNSIGNED NOT NULL DEFAULT 0,
  `status` TINYINT UNSIGNED NOT NULL DEFAULT 1,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_dictionary_items_type_value` (`dictionary_type_id`, `value`),
  KEY `idx_dictionary_items_type_status_sort` (`dictionary_type_id`, `status`, `sort`, `id`),
  CONSTRAINT `fk_dictionary_items_type`
    FOREIGN KEY (`dictionary_type_id`) REFERENCES `dictionary_types` (`id`)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT `chk_dictionary_items_status` CHECK (`status` IN (0, 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO `dictionary_types` (`id`, `code`, `name`, `remark`, `sort`, `status`, `is_system`) VALUES
  (1, 'entity_status', '通用状态', '平台和租户业务对象的启用状态展示', 10, 1, 1),
  (2, 'role_type', '角色类型', '内置角色与自定义角色的展示', 20, 1, 1),
  (3, 'menu_node_type', '菜单节点类型', '目录、菜单与操作权限的展示', 30, 1, 1),
  (4, 'workspace_scope', '工作空间', '平台端与租户端的展示', 40, 1, 1);

INSERT INTO `dictionary_items` (`id`, `dictionary_type_id`, `label`, `value`, `sort`, `status`) VALUES
  (1, 1, '启用', 'enabled', 10, 1),
  (2, 1, '禁用', 'disabled', 20, 1),
  (3, 2, '内置角色', 'system', 10, 1),
  (4, 2, '自定义角色', 'custom', 20, 1),
  (5, 3, '目录', 'directory', 10, 1),
  (6, 3, '菜单', 'menu', 20, 1),
  (7, 3, '操作权限', 'permission', 30, 1),
  (8, 4, '平台端', 'platform', 10, 1),
  (9, 4, '租户端', 'tenant', 20, 1);

-- +goose Down

DROP TABLE `dictionary_items`;
DROP TABLE `dictionary_types`;
