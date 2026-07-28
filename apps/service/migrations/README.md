# 数据库 Migration

本目录是项目数据库结构变化历史的唯一来源，统一使用固定版本的 Goose v3 执行 SQL Migration。

## 文件规范

- 文件名使用五位连续编号和 snake_case 描述，例如 `00001_create_xxx.sql`。
- 一个 Migration 只处理一个明确主题，并随代码提交 Git。
- 已执行或已进入共享环境的 Migration 禁止修改；后续调整必须新增文件。
- 不得为未确认的未来需求提前增加表、字段、索引或外键。

SQL Migration 使用以下格式：

```sql
-- +goose Up

-- 正向变更 SQL

-- +goose Down

-- 回滚 SQL
```

每个文件必须包含 `-- +goose Up`。无法安全回滚时可以不提供 Down SQL，但必须在文件中说明原因、数据丢失风险和人工恢复方式。

## 执行流程

1. 阅读已有 Migration。
2. 通过 DBHub 只读检查当前真实数据库结构。
3. 对照相关 Go Model，提出当前需求所需的最小方案。
4. 使用 `make migrate-create name=xxx` 创建新的 SQL Migration。
5. 完成 SQL 和风险说明后停止，等待用户审核。
6. 取得用户明确批准后，才能执行 `migrate-up`、`migrate-up-one` 或 `migrate-down`。
7. 执行后通过 DBHub 只读验证结果。

`migrate-status` 和 `migrate-version` 在 `goose_db_version` 不存在时也可能创建并初始化该表。执行前必须先通过 DBHub 确认元数据表已经存在，或者取得用户对初始化该表的明确批准。

禁止通过数据库客户端或 DBHub 直接修改数据库结构，禁止使用 GORM AutoMigrate，禁止在 Go 服务启动时自动执行 Migration。

DROP、删除字段、字段缩短、修改字段类型、数据回填，以及可能丢失数据的 Down 操作，必须单独说明影响和回滚方案，并再次取得用户明确批准。
