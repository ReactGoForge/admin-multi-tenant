-- 变更原因：将平台图库中允许租户读取的图片限制到唯一且受保护的“共享图片”分类。
-- 影响范围：为 image_categories 增加共享标识、唯一生成键与平台归属约束，并新增一个平台系统分类。
-- 兼容性：不移动、不回填现有图片；现有平台图片默认不共享，租户已有品牌图片引用继续由公开图片接口读取。
-- 数据处理：仅插入空的“共享图片”分类，不更新 image_assets 或 tenants 数据。
-- 回滚说明：Down 会删除“共享图片”分类，数据库外键会将该分类下图片转入未分类；共享关系会丢失，执行前必须备份并再次确认。
-- 验证 SQL（Up 获得执行批准并完成后，只读执行）：
--   SELECT version_id, is_applied FROM goose_db_version ORDER BY id DESC LIMIT 1;
--   SHOW CREATE TABLE image_categories;
--   SELECT id, tenant_id, name, is_shared, shared_unique_key FROM image_categories WHERE is_shared = 1;
--   SELECT COUNT(*) AS unexpected_shared_categories
--   FROM image_categories WHERE is_shared = 1 AND tenant_id IS NOT NULL;
--   SELECT COUNT(*) AS existing_platform_images_moved
--   FROM image_assets ia
--   JOIN image_categories ic ON ic.id = ia.category_id
--   WHERE ic.is_shared = 1;

-- +goose Up

ALTER TABLE `image_categories`
  ADD COLUMN `is_shared` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否为租户可读的平台共享分类' AFTER `name`,
  ADD COLUMN `shared_unique_key` TINYINT UNSIGNED
    GENERATED ALWAYS AS (CASE WHEN `is_shared` = 1 THEN 1 ELSE NULL END) STORED
    COMMENT '确保全平台只有一个共享分类',
  ADD UNIQUE KEY `uk_image_categories_single_shared` (`shared_unique_key`),
  ADD CONSTRAINT `chk_image_categories_shared_owner`
    CHECK (`is_shared` = 0 OR `tenant_id` IS NULL);

INSERT INTO `image_categories` (`tenant_id`, `name`, `is_shared`)
VALUES (NULL, '共享图片', 1);

-- +goose Down

DELETE FROM `image_categories`
WHERE `is_shared` = 1;

ALTER TABLE `image_categories`
  DROP CHECK `chk_image_categories_shared_owner`,
  DROP INDEX `uk_image_categories_single_shared`,
  DROP COLUMN `shared_unique_key`,
  DROP COLUMN `is_shared`;
