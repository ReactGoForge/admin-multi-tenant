-- 变更原因：将平台端和租户端图片权限从基础设置解耦，并把既有查看图片节点转换为独立图片库页面。
-- 影响范围：仅更新 menus 中固定 ID 为 1038-1041、2025-2028 的系统菜单定义；不修改表结构、图片数据或对象存储。
-- 兼容性：保留全部图片权限编码和菜单 ID，现有 role_menus 与 platform_role_tenant_menus 关联继续有效，不自动新增或撤销角色权限。
-- 数据处理：既有角色可能仍保留历史基础设置关联，本 Migration 不推测其授权来源，也不主动删除。
-- 回滚说明：Down 恢复图片权限原有父级和节点类型；对迁移后新配置图片权限的角色补齐旧基础设置父节点关联，避免回滚后权限树缺少祖先。
-- 回滚风险：Down 可能为迁移后新获得图片权限的角色补充基础设置查看节点，但不会删除任何既有角色权限。
-- 执行约束：本文件仅为 Migration 草案，取得用户明确批准前不得执行。
-- 验证 SQL（Up 获得执行批准并完成后，通过 DBHub 或 MySQL 客户端只读执行）：
--   SELECT version_id, is_applied FROM goose_db_version ORDER BY id DESC LIMIT 3;
--   SELECT id, parent_id, name, node_type, scope, path, component, icon,
--          permission_code, tenant_assignable, sort, visible, status
--   FROM menus
--   WHERE id IN (1038, 1039, 1040, 1041, 2025, 2026, 2027, 2028)
--   ORDER BY scope, id;
--   预期 1038、2025 分别为 /platform/images、/tenant/images 可见页面，其余图片权限挂在对应页面下。
--   SELECT menu_id, COUNT(*) AS role_count
--   FROM role_menus
--   WHERE menu_id IN (1038, 1039, 1040, 1041, 2025, 2026, 2027, 2028)
--   GROUP BY menu_id ORDER BY menu_id;
--   SELECT menu_id, COUNT(*) AS managed_role_count
--   FROM platform_role_tenant_menus
--   WHERE menu_id IN (2025, 2026, 2027, 2028)
--   GROUP BY menu_id ORDER BY menu_id;

-- +goose Up

UPDATE `menus`
SET `parent_id` = NULL,
    `name` = '图片库',
    `node_type` = 'menu',
    `path` = '/platform/images',
    `component` = 'pages/platform/images/index.tsx',
    `icon` = 'PictureOutlined',
    `sort` = 35,
    `visible` = 1
WHERE `id` = 1038
  AND `scope` = 'platform'
  AND `permission_code` = 'platform:image:view';

UPDATE `menus`
SET `parent_id` = 1038,
    `name` = CASE `id`
      WHEN 1039 THEN '上传图片'
      WHEN 1040 THEN '编辑图片和分类'
      WHEN 1041 THEN '删除图片'
    END,
    `sort` = CASE `id`
      WHEN 1039 THEN 1
      WHEN 1040 THEN 2
      WHEN 1041 THEN 3
    END
WHERE `id` IN (1039, 1040, 1041)
  AND `scope` = 'platform';

UPDATE `menus`
SET `parent_id` = NULL,
    `name` = '图片库',
    `node_type` = 'menu',
    `path` = '/tenant/images',
    `component` = 'pages/tenant/images/index.tsx',
    `icon` = 'PictureOutlined',
    `sort` = 25,
    `visible` = 1
WHERE `id` = 2025
  AND `scope` = 'tenant'
  AND `permission_code` = 'tenant:image:view';

UPDATE `menus`
SET `parent_id` = 2025,
    `name` = CASE `id`
      WHEN 2026 THEN '上传图片'
      WHEN 2027 THEN '编辑图片和分类'
      WHEN 2028 THEN '删除图片'
    END,
    `sort` = CASE `id`
      WHEN 2026 THEN 1
      WHEN 2027 THEN 2
      WHEN 2028 THEN 3
    END
WHERE `id` IN (2026, 2027, 2028)
  AND `scope` = 'tenant';

-- +goose Down

INSERT INTO `role_menus` (`scope`, `tenant_id`, `role_id`, `menu_id`)
SELECT 'platform', NULL, image_role.`role_id`, 1026
FROM (
  SELECT DISTINCT `role_id`
  FROM `role_menus`
  WHERE `scope` = 'platform'
    AND `tenant_id` IS NULL
    AND `menu_id` IN (1038, 1039, 1040, 1041)
) AS image_role
LEFT JOIN `role_menus` AS basic_role
  ON basic_role.`role_id` = image_role.`role_id`
 AND basic_role.`menu_id` = 1026
WHERE basic_role.`role_id` IS NULL;

INSERT INTO `role_menus` (`scope`, `tenant_id`, `role_id`, `menu_id`)
SELECT 'tenant', image_role.`tenant_id`, image_role.`role_id`, 2023
FROM (
  SELECT DISTINCT `tenant_id`, `role_id`
  FROM `role_menus`
  WHERE `scope` = 'tenant'
    AND `tenant_id` IS NOT NULL
    AND `menu_id` IN (2025, 2026, 2027, 2028)
) AS image_role
LEFT JOIN `role_menus` AS basic_role
  ON basic_role.`role_id` = image_role.`role_id`
 AND basic_role.`menu_id` = 2023
WHERE basic_role.`role_id` IS NULL;

INSERT INTO `platform_role_tenant_menus` (`role_id`, `menu_id`)
SELECT image_role.`role_id`, 2023
FROM (
  SELECT DISTINCT `role_id`
  FROM `platform_role_tenant_menus`
  WHERE `menu_id` IN (2025, 2026, 2027, 2028)
) AS image_role
LEFT JOIN `platform_role_tenant_menus` AS basic_role
  ON basic_role.`role_id` = image_role.`role_id`
 AND basic_role.`menu_id` = 2023
WHERE basic_role.`role_id` IS NULL;

UPDATE `menus`
SET `parent_id` = 1026,
    `name` = CASE `id`
      WHEN 1039 THEN '上传图片'
      WHEN 1040 THEN '编辑图片分类'
      WHEN 1041 THEN '删除图片'
    END,
    `sort` = CASE `id`
      WHEN 1039 THEN 3
      WHEN 1040 THEN 4
      WHEN 1041 THEN 5
    END
WHERE `id` IN (1039, 1040, 1041)
  AND `scope` = 'platform';

UPDATE `menus`
SET `parent_id` = 1026,
    `name` = '查看图片',
    `node_type` = 'permission',
    `path` = NULL,
    `component` = NULL,
    `icon` = NULL,
    `sort` = 2,
    `visible` = 0
WHERE `id` = 1038
  AND `scope` = 'platform'
  AND `permission_code` = 'platform:image:view';

UPDATE `menus`
SET `parent_id` = 2023,
    `name` = CASE `id`
      WHEN 2026 THEN '上传图片'
      WHEN 2027 THEN '编辑图片分类'
      WHEN 2028 THEN '删除图片'
    END,
    `sort` = CASE `id`
      WHEN 2026 THEN 3
      WHEN 2027 THEN 4
      WHEN 2028 THEN 5
    END
WHERE `id` IN (2026, 2027, 2028)
  AND `scope` = 'tenant';

UPDATE `menus`
SET `parent_id` = 2023,
    `name` = '查看图片',
    `node_type` = 'permission',
    `path` = NULL,
    `component` = NULL,
    `icon` = NULL,
    `sort` = 2,
    `visible` = 0
WHERE `id` = 2025
  AND `scope` = 'tenant'
  AND `permission_code` = 'tenant:image:view';
