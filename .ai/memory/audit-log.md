# 日志与审计记忆

本文件记录系统请求日志、运行事件、后台登录日志和操作审计事实。处理日志、审计、登录记录、日志保留策略或日志权限时读取；不得记录临时分析、猜测或未确认方案。

## 系统请求日志与运行事件

- 所有 `/api/**` 请求均由服务端生成请求 ID；系统请求日志通过 `SYSTEM_REQUEST_LOG_MODE` 支持 `mutation_and_error`、`all`、`off` 三档，默认仍只将查询失败和全部写请求的脱敏元数据写入 `system_logs`。
- 关闭入库不影响请求 ID 和基础结构化标准输出，公开图片二进制接口始终排除，`/ping` 不属于 API 日志范围。
- 请求日志不保存查询参数、请求体、响应体或 Authorization；保存路由模板、状态码、业务码、耗时、客户端信息和认证后的员工或小程序用户上下文。
- 运行事件默认同时写入 `system_logs` 和结构化标准输出；`SYSTEM_EVENT_DB_ENABLED=false` 时只关闭数据库写入，标准输出保持。数据库或日志写入失败时仅降级输出，不影响原业务响应。
- 纯请求日志属于 HTTP 基础设施能力，中间件通过最小 `RequestRecorder` 直接调用 Logging Store；运行事件由 Logging Service 编排。
- `APP_ENV=development` 时，Go 服务会在工作目录的 `logs/http-YYYY-MM-DD.jsonl` 记录全部 Gin 入站交换和微信客户端出站交换，按天切分并保留 7 天；文本、JSON、QueryString 和 Headers 原样保存且可能包含敏感凭证，单侧正文最多 8MB。JSON 中的图片 Data URL 和超过 4KB 的有效 Base64 字符串只保存 MIME、编码/解码大小与 SHA-256 摘要；multipart、图片和其他二进制只记录类型与大小。非精确 `development` 环境不会创建目录或包装请求，文件不写入数据库且已由 Git 忽略。

## 操作审计

- 后台成功写操作通过认证后的审计中间件写入 `operation_audit_logs`，且不提供关闭开关。
- 未登记的新模块从后台路由推导稳定编码并继续写入，只有中文模块名、特殊动作语义和操作前快照需要按需登记。
- JSON 字段只保存脱敏变更，密码和密钥只标记已修改。
- 审计中间件通过 `AuditRecorder` 调用 Logging Service；Service 负责审计路由识别、快照白名单和可信租户范围，Store 只执行 Service 生成的受控查询与审计写入。
- 平台系统日志使用 `platform:system-log:view`，平台操作日志使用 `platform:audit-log:view`，租户操作日志使用 `tenant:audit-log:view`；租户查询范围只取认证上下文，平台代管操作保留真实平台员工快照并归属当前租户。

## 登录日志与查询

- 后台每次登录结果始终以 `event` 写入 `system_logs`，不受普通运行事件入库开关影响，且不保存密码、验证码、JWT、Redis Key、请求体或响应体。
- 后台登录结果由 Auth Service 通过最小登录日志接口交给 Logging Service 持久化，不直接依赖 Logging Store。
- 登录日志记录成功、验证码错误、凭证错误、账号禁用、限流和安全服务不可用。
- 平台登录日志使用 `platform:login-log:view` 查看全局及未知账号尝试，租户登录日志使用 `tenant:login-log:view` 且仅查看已识别为当前租户员工的记录。
- 三类日志页面均有与列表相同权限和租户范围的筛选选项接口；租户按名称选择，操作者按日志历史快照的类型和稳定 ID 精确筛选，操作日志的模块和动作选项由服务端按可见历史日志去重返回。
- 系统日志支持按请求或运行事件类型筛选，列表统一展示请求/事件内容和 HTTP/业务结果。

## 保留策略

- 系统日志默认保留 30 天，可通过 `SYSTEM_LOG_RETENTION_DAYS` 配置 1 至 3650 天；服务启动后及每 24 小时分批清理过期记录，操作审计日志不自动删除。
- 可信代理通过可选 `TRUSTED_PROXIES` 配置，空值时不信任转发头。
