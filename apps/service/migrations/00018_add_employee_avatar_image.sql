-- 变更原因：允许后台员工保存一个由本人上传的头像图片引用，并继续复用现有私有 MinIO 图片资产。
-- 影响范围：仅为 employees 新增可空头像图片字段、索引和指向 image_assets 的外键；不修改现有图片表结构、接口或权限。
-- 兼容性：旧版服务会忽略新增字段并继续使用姓名首字头像；现有员工的头像引用保持 NULL，不影响登录、认证或租户隔离。
-- 数据处理：Up 不上传图片、不创建 image_assets、不回填员工数据，全部现有员工继续显示姓名首字头像。
-- 安全边界：数据库外键只保证图片存在；图片与员工平台/租户归属一致性必须由头像上传事务按认证员工原始身份校验。
-- 回滚说明：Down 会移除员工头像引用字段，但不会删除 image_assets 元数据或 MinIO 对象。
-- 回滚风险：如果执行 Down 时已经有员工头像，头像引用会丢失并留下无法自动关联的图片元数据和 MinIO 对象；执行前必须再次批准并备份引用清单。
-- 执行约束：本文件仅为 Migration 草案，取得用户明确批准前不得执行。
-- 验证 SQL（Up 获得执行批准并完成后，通过 DBHub 或 MySQL 客户端只读执行）：
--   SELECT version_id, is_applied FROM goose_db_version ORDER BY id DESC LIMIT 3;
--   SHOW CREATE TABLE employees;
--   SHOW INDEX FROM employees WHERE Key_name = 'idx_employees_avatar_image';
--   SELECT COUNT(*) AS employee_count,
--          SUM(avatar_image_id IS NOT NULL) AS employees_with_avatar
--   FROM employees;
--   SELECT rc.CONSTRAINT_NAME, rc.TABLE_NAME, rc.REFERENCED_TABLE_NAME,
--          rc.DELETE_RULE, rc.UPDATE_RULE
--   FROM information_schema.REFERENTIAL_CONSTRAINTS AS rc
--   WHERE rc.CONSTRAINT_SCHEMA = DATABASE()
--     AND rc.CONSTRAINT_NAME = 'fk_employees_avatar_image';
--   预期全部现有员工 avatar_image_id 均为 NULL，外键删除和更新规则均为 RESTRICT。

-- +goose Up

ALTER TABLE `employees`
  ADD COLUMN `avatar_image_id` BIGINT UNSIGNED NULL COMMENT '当前员工头像图片ID' AFTER `active_session_id`,
  ADD KEY `idx_employees_avatar_image` (`avatar_image_id`),
  ADD CONSTRAINT `fk_employees_avatar_image`
    FOREIGN KEY (`avatar_image_id`) REFERENCES `image_assets` (`id`)
    ON UPDATE RESTRICT ON DELETE RESTRICT;

-- +goose Down

ALTER TABLE `employees`
  DROP FOREIGN KEY `fk_employees_avatar_image`,
  DROP INDEX `idx_employees_avatar_image`,
  DROP COLUMN `avatar_image_id`;
