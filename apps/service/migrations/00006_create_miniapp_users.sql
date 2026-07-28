-- 变更原因：建立小程序平台唯一用户、用户与多租户归属关系，以及平台端和租户端用户启停权限。
-- 影响范围：新增 users、tenant_users 两张表，并在现有平台用户和租户用户菜单下新增两个操作权限节点。
-- 兼容性：依赖 00001 至 00005 已创建 tenants、menus、roles 和 role_menus；面向当前 MySQL 8.4。
-- 数据处理：Up 不创建、更新或删除用户业务数据，不自动扩大任何已有角色的权限。
-- 身份边界：wechat_openid 是当前单一小程序 AppID 下的平台用户唯一身份；phone 仅预留且不参与唯一身份判断。
-- 租户边界：tenant_users 使用 tenant_id 与 user_id 联合主键，同一平台用户可以归属多个租户。
-- 回滚说明：Down 先清理可能引用新增权限的 role_menus，再删除权限节点，最后按依赖顺序删除 tenant_users 和 users。
-- 回滚风险：如果执行 Up 后已经产生小程序用户数据，Down 会永久删除这些用户和租户归属数据，执行前必须再次确认并备份。
-- 执行约束：本 Migration 只生成草案，必须取得用户明确批准后才能执行。
-- 验证 SQL（执行获批并完成后，通过 DBHub 只读执行）：
--   SHOW CREATE TABLE users;
--   SHOW CREATE TABLE tenant_users;
--   SHOW INDEX FROM users;
--   SHOW INDEX FROM tenant_users;
--   SELECT id, parent_id, name, node_type, scope, permission_code, tenant_assignable, visible, status
--   FROM menus
--   WHERE id IN (1029, 2022)
--   ORDER BY id;
--   SELECT COUNT(*) FROM users;
--   SELECT COUNT(*) FROM tenant_users;
--   SELECT COUNT(*)
--   FROM role_menus
--   WHERE menu_id IN (1029, 2022);
--   预期：两张新表存在；两个权限节点分别挂在 1028、2021 下；三项 COUNT 均为 0。

-- +goose Up

CREATE TABLE `users` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '平台用户ID',
  `wechat_openid` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT '当前小程序AppID下的微信OpenID',
  `wechat_unionid` VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL COMMENT '满足微信UnionID下发条件时的跨应用用户标识',
  `phone` VARCHAR(20) NULL COMMENT '用户主动授权后保存的手机号，第一阶段仅预留',
  `nickname` VARCHAR(64) NULL COMMENT '用户主动填写或选择的昵称',
  `avatar_url` VARCHAR(500) NULL COMMENT '用户头像的持久化访问地址',
  `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '平台用户状态：1启用，0禁用',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_users_wechat_openid` (`wechat_openid`),
  UNIQUE KEY `uk_users_wechat_unionid` (`wechat_unionid`),
  KEY `idx_users_status_created` (`status`, `created_at`, `id`),
  CONSTRAINT `chk_users_status` CHECK (`status` IN (0, 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='小程序平台用户';

CREATE TABLE `tenant_users` (
  `tenant_id` BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '平台用户ID',
  `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '当前租户内的用户状态：1启用，0禁用',
  `joined_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '首次归属当前租户的时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`tenant_id`, `user_id`),
  KEY `idx_tenant_users_tenant_status_joined` (`tenant_id`, `status`, `joined_at`, `user_id`),
  KEY `idx_tenant_users_user_status_tenant` (`user_id`, `status`, `tenant_id`),
  CONSTRAINT `fk_tenant_users_tenant`
    FOREIGN KEY (`tenant_id`) REFERENCES `tenants` (`id`)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT `fk_tenant_users_user`
    FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT `chk_tenant_users_status` CHECK (`status` IN (0, 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='小程序用户租户归属';

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
  (1029, 1028, '启用或禁用用户', 'permission', 'platform', NULL, NULL, NULL, 'platform:user:status', 0, 1, 0, 1),
  (2022, 2021, '启用或禁用用户', 'permission', 'tenant', NULL, NULL, NULL, 'tenant:user:status', 1, 1, 0, 1);

-- +goose Down

DELETE FROM `role_menus`
WHERE `menu_id` IN (1029, 2022);

DELETE FROM `menus`
WHERE (`id` = 1029 AND `scope` = 'platform' AND `permission_code` = 'platform:user:status')
   OR (`id` = 2022 AND `scope` = 'tenant' AND `permission_code` = 'tenant:user:status');

DROP TABLE `tenant_users`;
DROP TABLE `users`;
