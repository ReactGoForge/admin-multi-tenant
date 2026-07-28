-- 变更原因：补齐字典表及既有配置、权限关联表遗漏的中文表注释和字段注释，便于通过 DBHub 等工具理解数据库结构。
-- 影响范围：仅修改 dictionary_types、dictionary_items、wechat_miniapp_settings、platform_role_tenant_menus、platform_settings 的注释元数据。
-- 兼容性：不修改字段类型、字符集、排序规则、NULL、默认值、索引、外键、检查约束或接口行为。
-- 数据处理：不插入、更新、删除或回填任何业务数据。
-- 风险说明：ALTER TABLE 执行时可能短暂持有元数据锁，应避开高并发写入时段。
-- 回滚说明：Down 仅清除本 Migration 新增的注释，恢复到执行前状态，不删除表、字段或数据。
-- 执行约束：本 Migration 只生成草案，必须取得用户明确批准后才能执行。
-- 验证 SQL（执行获批并完成后，通过 DBHub 只读执行）：
--   SELECT version_id, is_applied FROM goose_db_version ORDER BY id DESC LIMIT 1;
--   SELECT TABLE_NAME, TABLE_COMMENT
--   FROM information_schema.TABLES
--   WHERE TABLE_SCHEMA = DATABASE()
--     AND TABLE_NAME IN ('dictionary_types', 'dictionary_items', 'wechat_miniapp_settings', 'platform_role_tenant_menus', 'platform_settings')
--   ORDER BY TABLE_NAME;
--   SELECT TABLE_NAME, ORDINAL_POSITION, COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT, EXTRA, COLUMN_COMMENT
--   FROM information_schema.COLUMNS
--   WHERE TABLE_SCHEMA = DATABASE()
--     AND TABLE_NAME IN ('dictionary_types', 'dictionary_items', 'wechat_miniapp_settings', 'platform_role_tenant_menus', 'platform_settings')
--   ORDER BY TABLE_NAME, ORDINAL_POSITION;
--   SHOW CREATE TABLE dictionary_types;
--   SHOW CREATE TABLE dictionary_items;
--   SHOW CREATE TABLE wechat_miniapp_settings;
--   SHOW CREATE TABLE platform_role_tenant_menus;
--   SHOW CREATE TABLE platform_settings;

-- +goose Up

ALTER TABLE `dictionary_types`
  COMMENT = '全局字典字段',
  MODIFY COLUMN `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '字典字段ID',
  MODIFY COLUMN `code` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '字典字段编码',
  MODIFY COLUMN `name` VARCHAR(50) NOT NULL COMMENT '字典字段名称',
  MODIFY COLUMN `remark` VARCHAR(200) NULL DEFAULT NULL COMMENT '字典字段备注',
  MODIFY COLUMN `sort` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '排序值',
  MODIFY COLUMN `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态：1启用，0禁用',
  MODIFY COLUMN `is_system` TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '是否系统字典：1是，0否';

ALTER TABLE `dictionary_items`
  COMMENT = '全局字典项',
  MODIFY COLUMN `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '字典项ID',
  MODIFY COLUMN `dictionary_type_id` BIGINT UNSIGNED NOT NULL COMMENT '字典字段ID',
  MODIFY COLUMN `label` VARCHAR(50) NOT NULL COMMENT '字典项展示文案',
  MODIFY COLUMN `value` VARCHAR(100) NOT NULL COMMENT '字典项稳定值',
  MODIFY COLUMN `sort` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '排序值',
  MODIFY COLUMN `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态：1启用，0禁用';

ALTER TABLE `wechat_miniapp_settings`
  MODIFY COLUMN `id` TINYINT UNSIGNED NOT NULL COMMENT '单行配置ID，固定为1',
  MODIFY COLUMN `app_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '微信小程序AppID',
  MODIFY COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间';

ALTER TABLE `platform_role_tenant_menus`
  MODIFY COLUMN `role_id` BIGINT UNSIGNED NOT NULL COMMENT '平台角色ID',
  MODIFY COLUMN `menu_id` BIGINT UNSIGNED NOT NULL COMMENT '租户菜单或权限节点ID';

ALTER TABLE `platform_settings`
  MODIFY COLUMN `id` TINYINT UNSIGNED NOT NULL COMMENT '单行配置ID，固定为1';

-- +goose Down

ALTER TABLE `platform_settings`
  MODIFY COLUMN `id` TINYINT UNSIGNED NOT NULL COMMENT '';

ALTER TABLE `platform_role_tenant_menus`
  MODIFY COLUMN `role_id` BIGINT UNSIGNED NOT NULL COMMENT '',
  MODIFY COLUMN `menu_id` BIGINT UNSIGNED NOT NULL COMMENT '';

ALTER TABLE `wechat_miniapp_settings`
  MODIFY COLUMN `id` TINYINT UNSIGNED NOT NULL COMMENT '',
  MODIFY COLUMN `app_id` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '',
  MODIFY COLUMN `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '';

ALTER TABLE `dictionary_items`
  COMMENT = '',
  MODIFY COLUMN `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '',
  MODIFY COLUMN `dictionary_type_id` BIGINT UNSIGNED NOT NULL COMMENT '',
  MODIFY COLUMN `label` VARCHAR(50) NOT NULL COMMENT '',
  MODIFY COLUMN `value` VARCHAR(100) NOT NULL COMMENT '',
  MODIFY COLUMN `sort` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '',
  MODIFY COLUMN `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '';

ALTER TABLE `dictionary_types`
  COMMENT = '',
  MODIFY COLUMN `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '',
  MODIFY COLUMN `code` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '',
  MODIFY COLUMN `name` VARCHAR(50) NOT NULL COMMENT '',
  MODIFY COLUMN `remark` VARCHAR(200) NULL DEFAULT NULL COMMENT '',
  MODIFY COLUMN `sort` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '',
  MODIFY COLUMN `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '',
  MODIFY COLUMN `is_system` TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '';
