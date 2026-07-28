-- +goose Up
-- +goose StatementBegin
-- 修改原因：后台员工账号需要保持“最后一次登录有效”，认证时按员工主键比对当前会话标识。
-- 影响范围：仅 employees 新增一个可空认证字段；不影响微信小程序用户、业务数据、接口或索引。
-- 兼容性：不回填旧数据，新认证代码要求 JWT 必须包含 jti，因此发布后现有后台 Token 会统一失效一次。
ALTER TABLE employees
    ADD COLUMN active_session_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL COMMENT '当前有效后台会话标识' AFTER password_hash;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- 回滚说明：删除的仅为随机会话标识，不包含业务数据；回滚后恢复同一员工多 Token 并存行为。
ALTER TABLE employees
    DROP COLUMN active_session_id;
-- +goose StatementEnd

-- 执行约束：本文件仅为 Migration 草案，取得明确批准前不得执行。
-- 验证 SQL：
-- SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, CHARACTER_SET_NAME, COLLATION_NAME, COLUMN_COMMENT
-- FROM information_schema.COLUMNS
-- WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'employees' AND COLUMN_NAME = 'active_session_id';
-- SELECT id, login_account, active_session_id FROM employees ORDER BY id;
