#!/usr/bin/env bash

set -Eeuo pipefail

readonly DBHUB_CONFIG_DIRECTORY="${ADMIN_MULTI_TENANT_DBHUB_CONFIG_DIR:-${XDG_CONFIG_HOME:-${HOME}/.config}/reactgoforge/admin-multi-tenant}"
readonly DBHUB_ENV_FILE="${ADMIN_MULTI_TENANT_DBHUB_ENV_FILE:-${DBHUB_CONFIG_DIRECTORY}/dbhub.env}"
readonly DBHUB_CONFIG_FILE="${ADMIN_MULTI_TENANT_DBHUB_CONFIG_FILE:-${DBHUB_CONFIG_DIRECTORY}/dbhub-test.toml}"
readonly DBHUB_LOG_FILE="${ADMIN_MULTI_TENANT_DBHUB_LOG_FILE:-${DBHUB_CONFIG_DIRECTORY}/dbhub.log}"
readonly DBHUB_PID_FILE="${ADMIN_MULTI_TENANT_DBHUB_PID_FILE:-${DBHUB_CONFIG_DIRECTORY}/dbhub.pid}"
readonly DBHUB_VERSION="${ADMIN_MULTI_TENANT_DBHUB_VERSION:-0.23.0}"
readonly DBHUB_HOST="${ADMIN_MULTI_TENANT_DBHUB_HOST:-127.0.0.1}"
readonly DBHUB_PORT="${ADMIN_MULTI_TENANT_DBHUB_PORT:-18080}"
readonly DBHUB_URL="http://${DBHUB_HOST}:${DBHUB_PORT}"

log() {
  printf '[admin-multi-tenant-dbhub] %s\n' "$*"
}

fail() {
  printf '[admin-multi-tenant-dbhub] 错误：%s\n' "$*" >&2
  exit 1
}

print_usage() {
  cat <<'EOF'
用法：
  ./tools/dbhub.sh start
  ./tools/dbhub.sh stop
  ./tools/dbhub.sh restart
  ./tools/dbhub.sh status
  ./tools/dbhub.sh logs

命令：
  start    后台启动本地 DBHub，通过 dbhub-test.toml 中的 SSH 配置连接测试库
  stop     停止本脚本管理的本地 DBHub
  restart  重启本地 DBHub
  status   查看本地 DBHub 状态
  logs     持续查看最近 100 行日志，按 Ctrl+C 退出
EOF
}

require_command() {
  local command_name="$1"

  command -v "${command_name}" >/dev/null 2>&1 ||
    fail "缺少命令 ${command_name}"
}

listener_pid() {
  lsof -nP -tiTCP:"${DBHUB_PORT}" -sTCP:LISTEN 2>/dev/null | head -n 1 || true
}

process_command() {
  local process_id="$1"

  lsof \
    -nP \
    -a \
    -p "${process_id}" \
    -iTCP:"${DBHUB_PORT}" \
    -sTCP:LISTEN \
    2>/dev/null |
    tail -n 1 ||
    true
}

is_dbhub_process() {
  curl \
    --silent \
    --show-error \
    --fail \
    --max-time 2 \
    "${DBHUB_URL}/" \
    2>/dev/null |
    grep -Fq '<title>DBHub - Minimal Database MCP Server</title>'
}

is_managed_pid() {
  local process_id="$1"
  local managed_pid

  [[ -r "${DBHUB_PID_FILE}" ]] || return 1
  managed_pid="$(<"${DBHUB_PID_FILE}")"
  [[ "${managed_pid}" == "${process_id}" ]]
}

is_ready() {
  curl \
    --silent \
    --show-error \
    --fail \
    --max-time 2 \
    "${DBHUB_URL}/" \
    >/dev/null 2>&1
}

validate_configuration() {
  [[ -r "${DBHUB_ENV_FILE}" ]] ||
    fail "找不到环境文件 ${DBHUB_ENV_FILE}"
  [[ -r "${DBHUB_CONFIG_FILE}" ]] ||
    fail "找不到配置文件 ${DBHUB_CONFIG_FILE}"

  set -a
  # shellcheck disable=SC1090
  source "${DBHUB_ENV_FILE}"
  set +a

  [[ -n "${DBHUB_MYSQL_USER:-}" ]] ||
    fail "${DBHUB_ENV_FILE} 缺少 DBHUB_MYSQL_USER"
  [[ -n "${DBHUB_MYSQL_PASSWORD:-}" ]] ||
    fail "${DBHUB_ENV_FILE} 缺少 DBHUB_MYSQL_PASSWORD"
}

start_dbhub() {
  local current_pid
  local launcher_pid
  local attempt

  require_command curl
  require_command grep
  require_command lsof
  require_command npx

  current_pid="$(listener_pid)"
  if [[ -n "${current_pid}" ]]; then
    if is_dbhub_process && is_ready; then
      log "DBHub 已在运行，PID=${current_pid}"
      log "Workbench：${DBHUB_URL}/"
      return
    fi

    fail "端口 ${DBHUB_PORT} 已被占用：$(process_command "${current_pid}")"
  fi

  validate_configuration
  mkdir -p "${DBHUB_CONFIG_DIRECTORY}"
  umask 077
  touch "${DBHUB_LOG_FILE}"
  printf '\n[%s] 启动 DBHub %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "${DBHUB_VERSION}" \
    >>"${DBHUB_LOG_FILE}"

  nohup npx -y "@bytebase/dbhub@${DBHUB_VERSION}" \
    --config="${DBHUB_CONFIG_FILE}" \
    --transport=http \
    --host="${DBHUB_HOST}" \
    --port="${DBHUB_PORT}" \
    >>"${DBHUB_LOG_FILE}" 2>&1 </dev/null &
  launcher_pid=$!

  for attempt in {1..20}; do
    if is_ready; then
      current_pid="$(listener_pid)"
      printf '%s\n' "${current_pid}" >"${DBHUB_PID_FILE}"
      log "DBHub 启动成功，PID=${current_pid}"
      log "Workbench：${DBHUB_URL}/"
      log "MCP：${DBHUB_URL}/mcp"
      log "日志：${DBHUB_LOG_FILE}"
      return
    fi

    kill -0 "${launcher_pid}" 2>/dev/null ||
      break
    sleep 1
  done

  current_pid="$(listener_pid)"
  if [[ -n "${current_pid}" ]] && is_dbhub_process; then
    kill "${current_pid}" 2>/dev/null || true
  else
    kill "${launcher_pid}" 2>/dev/null || true
  fi

  tail -n 30 "${DBHUB_LOG_FILE}" >&2 || true
  fail "DBHub 启动失败，请根据以上日志排查"
}

stop_dbhub() {
  local current_pid
  local attempt

  require_command curl
  require_command grep
  require_command lsof

  current_pid="$(listener_pid)"
  if [[ -z "${current_pid}" ]]; then
    rm -f "${DBHUB_PID_FILE}"
    log "DBHub 已停止"
    return
  fi

  if ! is_managed_pid "${current_pid}" && ! is_dbhub_process; then
    fail "端口 ${DBHUB_PORT} 不是 DBHub，拒绝停止：$(process_command "${current_pid}")"
  fi

  kill "${current_pid}"
  for attempt in {1..10}; do
    if ! kill -0 "${current_pid}" 2>/dev/null; then
      rm -f "${DBHUB_PID_FILE}"
      log "DBHub 已停止"
      return
    fi
    sleep 1
  done

  fail "DBHub 未在 10 秒内退出，PID=${current_pid}，请先查看日志"
}

show_status() {
  local current_pid

  require_command curl
  require_command grep
  require_command lsof

  current_pid="$(listener_pid)"
  if [[ -z "${current_pid}" ]]; then
    log "状态：已停止"
    return
  fi

  if ! is_managed_pid "${current_pid}" && ! is_dbhub_process; then
    fail "端口 ${DBHUB_PORT} 被其他程序占用：$(process_command "${current_pid}")"
  fi

  if is_ready; then
    log "状态：运行中"
    log "PID：${current_pid}"
    log "Workbench：${DBHUB_URL}/"
    log "MCP：${DBHUB_URL}/mcp"
    return
  fi

  fail "DBHub 进程存在但暂不可用，PID=${current_pid}"
}

show_logs() {
  [[ -f "${DBHUB_LOG_FILE}" ]] ||
    fail "日志尚未生成：${DBHUB_LOG_FILE}"

  log "持续查看 ${DBHUB_LOG_FILE}，按 Ctrl+C 退出"
  tail -n 100 -f "${DBHUB_LOG_FILE}"
}

main() {
  local command_name="${1:-help}"

  case "${command_name}" in
    start)
      start_dbhub
      ;;
    stop)
      stop_dbhub
      ;;
    restart)
      stop_dbhub
      start_dbhub
      ;;
    status)
      show_status
      ;;
    logs)
      show_logs
      ;;
    help | -h | --help)
      print_usage
      ;;
    *)
      print_usage >&2
      fail "不支持的命令 ${command_name}"
      ;;
  esac
}

main "$@"
