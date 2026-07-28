-- 变更原因：为平台品牌设置和 MinIO 多租户图片库提供持久化元数据、引用关系与独立权限。
-- 影响范围：新增平台设置、图片分类、图片资产三张表；为 tenants 增加可空图片引用；新增平台及租户图片权限。
-- 兼容性：保留 tenants.icon_url，不修改或回填现有租户数据；平台名称初始化为 ReactGoForge Admin；不自动向现有角色授予新权限。
-- 数据处理：插入一条平台默认设置和九条系统权限定义，不上传、移动或删除任何 MinIO 对象。
-- 回滚说明：Down 会丢失平台品牌、图片元数据、分类及租户图片引用，但不会删除 MinIO 对象；正式回滚前必须备份并单独清理对象存储。
-- 验证 SQL（Up 获得执行批准并完成后，只读执行）：
--   SELECT version_id, is_applied FROM goose_db_version ORDER BY id DESC LIMIT 1;
--   SHOW CREATE TABLE platform_settings;
--   SHOW CREATE TABLE image_categories;
--   SHOW CREATE TABLE image_assets;
--   SHOW CREATE TABLE tenants;
--   SELECT id, name, icon_image_id FROM platform_settings;
--   SELECT id, parent_id, name, scope, permission_code, tenant_assignable
--   FROM menus WHERE id BETWEEN 1037 AND 1041 OR id BETWEEN 2025 AND 2028 ORDER BY id;
--   SELECT COUNT(*) AS unexpected_platform_role_grants
--   FROM role_menus WHERE menu_id BETWEEN 1037 AND 1041;
--   SELECT COUNT(*) AS unexpected_tenant_role_grants
--   FROM role_menus WHERE menu_id BETWEEN 2025 AND 2028;
--   SELECT COUNT(*) AS unexpected_managed_role_grants
--   FROM platform_role_tenant_menus WHERE menu_id BETWEEN 2025 AND 2028;
--   SELECT COUNT(*) AS tenants_with_new_icon_reference FROM tenants WHERE icon_image_id IS NOT NULL;
--   SELECT rc.CONSTRAINT_NAME, rc.TABLE_NAME, rc.REFERENCED_TABLE_NAME, rc.DELETE_RULE, rc.UPDATE_RULE
--   FROM information_schema.REFERENTIAL_CONSTRAINTS AS rc
--   WHERE rc.CONSTRAINT_SCHEMA = DATABASE()
--     AND rc.TABLE_NAME IN ('platform_settings', 'image_categories', 'image_assets', 'tenants')
--   ORDER BY rc.TABLE_NAME, rc.CONSTRAINT_NAME;

-- +goose Up

CREATE TABLE `image_categories` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '图片分类ID',
  `tenant_id` BIGINT UNSIGNED NULL COMMENT '所属租户ID，NULL表示平台公共分类',
  `owner_tenant_id` BIGINT UNSIGNED GENERATED ALWAYS AS (COALESCE(`tenant_id`, 0)) STORED COMMENT '分类所有者唯一键，平台为0',
  `name` VARCHAR(40) NOT NULL COMMENT '分类名称',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_image_categories_owner_name` (`owner_tenant_id`, `name`),
  KEY `idx_image_categories_tenant_id` (`tenant_id`, `id`),
  CONSTRAINT `fk_image_categories_tenant`
    FOREIGN KEY (`tenant_id`) REFERENCES `tenants` (`id`)
    ON UPDATE RESTRICT ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='平台及租户图片自定义分类';

CREATE TABLE `image_assets` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '图片资产ID',
  `tenant_id` BIGINT UNSIGNED NULL COMMENT '所属租户ID，NULL表示平台公共图片',
  `category_id` BIGINT UNSIGNED NULL COMMENT '图片分类ID，NULL表示未分类',
  `original_name` VARCHAR(255) NOT NULL COMMENT '上传时的原始文件名',
  `object_key` VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT 'MinIO对象键',
  `mime_type` VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '服务端识别的图片MIME',
  `size_bytes` BIGINT UNSIGNED NOT NULL COMMENT '图片文件字节数',
  `uploaded_by_employee_id` BIGINT UNSIGNED NOT NULL COMMENT '上传员工ID',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_image_assets_object_key` (`object_key`),
  KEY `idx_image_assets_owner_category_created` (`tenant_id`, `category_id`, `created_at`, `id`),
  KEY `idx_image_assets_category_id` (`category_id`),
  KEY `idx_image_assets_uploader` (`uploaded_by_employee_id`),
  CONSTRAINT `fk_image_assets_tenant`
    FOREIGN KEY (`tenant_id`) REFERENCES `tenants` (`id`)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT `fk_image_assets_category`
    FOREIGN KEY (`category_id`) REFERENCES `image_categories` (`id`)
    ON UPDATE RESTRICT ON DELETE SET NULL,
  CONSTRAINT `fk_image_assets_uploader`
    FOREIGN KEY (`uploaded_by_employee_id`) REFERENCES `employees` (`id`)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT `chk_image_assets_mime_type`
    CHECK (`mime_type` IN ('image/png', 'image/jpeg', 'image/webp')),
  CONSTRAINT `chk_image_assets_size`
    CHECK (`size_bytes` > 0 AND `size_bytes` <= 5242880)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='MinIO图片资产元数据';

CREATE TABLE `platform_settings` (
  `id` TINYINT UNSIGNED NOT NULL,
  `name` VARCHAR(100) NOT NULL COMMENT '平台品牌名称',
  `icon_image_id` BIGINT UNSIGNED NULL COMMENT '平台品牌图标图片ID',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_platform_settings_icon_image` (`icon_image_id`),
  CONSTRAINT `fk_platform_settings_icon_image`
    FOREIGN KEY (`icon_image_id`) REFERENCES `image_assets` (`id`)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT `chk_platform_settings_singleton` CHECK (`id` = 1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='全平台唯一基础设置';

INSERT INTO `platform_settings` (`id`, `name`, `icon_image_id`)
VALUES (1, 'ReactGoForge Admin', NULL);

ALTER TABLE `tenants`
  ADD COLUMN `icon_image_id` BIGINT UNSIGNED NULL COMMENT '租户图标图片ID' AFTER `icon_url`,
  ADD KEY `idx_tenants_icon_image` (`icon_image_id`),
  ADD CONSTRAINT `fk_tenants_icon_image`
    FOREIGN KEY (`icon_image_id`) REFERENCES `image_assets` (`id`)
    ON UPDATE RESTRICT ON DELETE RESTRICT;

INSERT INTO `menus` (
  `id`, `parent_id`, `name`, `node_type`, `scope`, `path`, `component`, `icon`,
  `permission_code`, `tenant_assignable`, `sort`, `visible`, `status`
) VALUES
  (1037, 1026, '编辑基础设置', 'permission', 'platform', NULL, NULL, NULL, 'platform:basic:edit', 0, 1, 0, 1),
  (1038, 1026, '查看图片', 'permission', 'platform', NULL, NULL, NULL, 'platform:image:view', 0, 2, 0, 1),
  (1039, 1026, '上传图片', 'permission', 'platform', NULL, NULL, NULL, 'platform:image:upload', 0, 3, 0, 1),
  (1040, 1026, '编辑图片分类', 'permission', 'platform', NULL, NULL, NULL, 'platform:image:edit', 0, 4, 0, 1),
  (1041, 1026, '删除图片', 'permission', 'platform', NULL, NULL, NULL, 'platform:image:delete', 0, 5, 0, 1),
  (2025, 2023, '查看图片', 'permission', 'tenant', NULL, NULL, NULL, 'tenant:image:view', 1, 2, 0, 1),
  (2026, 2023, '上传图片', 'permission', 'tenant', NULL, NULL, NULL, 'tenant:image:upload', 1, 3, 0, 1),
  (2027, 2023, '编辑图片分类', 'permission', 'tenant', NULL, NULL, NULL, 'tenant:image:edit', 1, 4, 0, 1),
  (2028, 2023, '删除图片', 'permission', 'tenant', NULL, NULL, NULL, 'tenant:image:delete', 1, 5, 0, 1);

-- +goose Down

DELETE FROM `platform_role_tenant_menus`
WHERE `menu_id` BETWEEN 2025 AND 2028;

DELETE FROM `role_menus`
WHERE `menu_id` BETWEEN 1037 AND 1041 OR `menu_id` BETWEEN 2025 AND 2028;

DELETE FROM `menus`
WHERE `id` BETWEEN 1037 AND 1041 OR `id` BETWEEN 2025 AND 2028;

ALTER TABLE `tenants`
  DROP FOREIGN KEY `fk_tenants_icon_image`,
  DROP INDEX `idx_tenants_icon_image`,
  DROP COLUMN `icon_image_id`;

DROP TABLE `platform_settings`;
DROP TABLE `image_assets`;
DROP TABLE `image_categories`;
