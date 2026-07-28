#!/usr/bin/env bash

set -Eeuo pipefail

umask 077

ARTIFACT_SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ARTIFACT_REPOSITORY_DIR="$(cd -- "${ARTIFACT_SCRIPT_DIR}/.." && pwd)"
ARTIFACT_SCRIPT_PATH="${ARTIFACT_SCRIPT_DIR}/artifact-deploy.sh"
ARTIFACT_COMPOSE_FILE="${ARTIFACT_SCRIPT_DIR}/compose.artifact.yaml"
ARTIFACT_INFRA_COMPOSE_FILE="${ARTIFACT_SCRIPT_DIR}/compose.infra.yaml"
ARTIFACT_SHARED_DIR="${ADMIN_MULTI_TENANT_SHARED_DIR:-/opt/reactgoforge/admin-multi-tenant/shared}"
ARTIFACT_BACKUP_ROOT="${ADMIN_MULTI_TENANT_BACKUP_DIR:-/opt/reactgoforge/admin-multi-tenant/backups}"
ARTIFACT_ENV_FILE="${ADMIN_MULTI_TENANT_ENV_FILE:-${ARTIFACT_SHARED_DIR}/.env}"
ARTIFACT_STATE_FILE="${ARTIFACT_SHARED_DIR}/artifact-release.env"
ARTIFACT_LOCK_FILE="${ARTIFACT_SHARED_DIR}/deploy.lock"
ARTIFACT_SERVICE_RELEASE_ROOT="${SERVICE_RELEASE_ROOT:-${ARTIFACT_SHARED_DIR}/service-releases}"
ARTIFACT_WEB_RELEASE_ROOT="${WEB_RELEASE_ROOT:-/opt/1panel/www/sites/admin-multi-tenant-test/web}"
ARTIFACT_OPENRESTY_PROXY_CONFIG="${OPENRESTY_PROXY_CONFIG:-/opt/1panel/www/sites/admin-multi-tenant-test/proxy/root.conf}"
ARTIFACT_RUNTIME_IMAGE="admin-multi-tenant-service-runtime:1"
ARTIFACT_PROJECT_NAME="admin-multi-tenant"
ARTIFACT_TEMP_DIR=""
ARTIFACT_LOCK_ACQUIRED="false"
ARTIFACT_SERVICE_PAUSED="false"
ARTIFACT_ORIGINAL_ARGS=("$@")

ARTIFACT_CURRENT_SHA=""
ARTIFACT_CURRENT_SERVICE_SHA=""
ARTIFACT_CURRENT_WEB_SHA=""
ARTIFACT_PREVIOUS_SHA=""
ARTIFACT_PREVIOUS_SERVICE_SHA=""
ARTIFACT_PREVIOUS_WEB_SHA=""
ARTIFACT_RELEASE_TYPE=""
ARTIFACT_COMMIT_SHA=""

artifact_log() {
  printf '[admin-multi-tenant-artifact] %s\n' "$*"
}

artifact_error() {
  printf '[admin-multi-tenant-artifact] 错误：%s\n' "$*" >&2
}

artifact_die() {
  artifact_error "$*"
  exit 1
}

artifact_usage() {
  cat <<'EOF'
用法：
  ./deploy/artifact-deploy.sh deploy <产物包.tar.gz> [--migrate]
  ./deploy/artifact-deploy.sh stage <产物包.tar.gz>
  ./deploy/artifact-deploy.sh infra up
  ./deploy/artifact-deploy.sh initialize <apps产物包.tar.gz> [--migrate]
  ./deploy/artifact-deploy.sh bootstrap-owner
  ./deploy/artifact-deploy.sh verify
  ./deploy/artifact-deploy.sh rollback previous
  ./deploy/artifact-deploy.sh status
  ./deploy/artifact-deploy.sh backup

说明：
  deploy    校验并暂存产物，按 service → web 顺序原子切换和健康检查。
  stage     仅校验并写入版本目录，供首次运行方式切换使用。
  infra up  使用已加载的离线镜像启动 MySQL、Redis、MinIO。
  initialize 首次建立 Web/Go current；全新数据库必须显式使用 --migrate。
  bootstrap-owner 在交互式终端中使用当前 Go 产物创建唯一平台所有者。
  verify    验证固定 Go 容器、OpenResty 静态站点和当前版本目录。
  rollback  恢复上一组 Web 与 Go 产物，不执行 Goose Down。
  status    查看产物版本、固定 Go 容器和基础设施状态。
  backup    备份 MySQL、MinIO 和服务器环境文件。
EOF
}

artifact_cleanup() {
  if [[ -n "${ARTIFACT_TEMP_DIR}" && -d "${ARTIFACT_TEMP_DIR}" ]]; then
    rm -rf -- "${ARTIFACT_TEMP_DIR}"
  fi
}

artifact_resume_service() {
  if [[ "${ARTIFACT_SERVICE_PAUSED}" == "true" ]]; then
    artifact_compose unpause service >/dev/null 2>&1 || true
    ARTIFACT_SERVICE_PAUSED="false"
  fi
}

artifact_require_command() {
  local command_name="$1"
  command -v "${command_name}" >/dev/null 2>&1 \
    || artifact_die "缺少命令 ${command_name}"
}

artifact_prepare_directories() {
  mkdir -p \
    "${ARTIFACT_SHARED_DIR}" \
    "${ARTIFACT_BACKUP_ROOT}" \
    "${ARTIFACT_SERVICE_RELEASE_ROOT}/releases" \
    "${ARTIFACT_WEB_RELEASE_ROOT}/releases"
  chmod 700 "${ARTIFACT_SHARED_DIR}" "${ARTIFACT_BACKUP_ROOT}"
  chmod 755 "${ARTIFACT_SERVICE_RELEASE_ROOT}" "${ARTIFACT_SERVICE_RELEASE_ROOT}/releases"
  chmod 755 "${ARTIFACT_WEB_RELEASE_ROOT}" "${ARTIFACT_WEB_RELEASE_ROOT}/releases"
}

artifact_acquire_lock() {
  exec 9>"${ARTIFACT_LOCK_FILE}"
  if ! flock -n 9; then
    artifact_die "已有部署或备份任务正在运行"
  fi
  ARTIFACT_LOCK_ACQUIRED="true"
}

artifact_release_lock() {
  if [[ "${ARTIFACT_LOCK_ACQUIRED}" == "true" ]]; then
    flock -u 9
    ARTIFACT_LOCK_ACQUIRED="false"
  fi
}

artifact_load_environment() {
  [[ -f "${ARTIFACT_ENV_FILE}" ]] \
    || artifact_die "缺少服务器环境文件 ${ARTIFACT_ENV_FILE}"
  chmod 600 "${ARTIFACT_ENV_FILE}"
  set -a
  # shellcheck disable=SC1090
  source "${ARTIFACT_ENV_FILE}"
  set +a
  ARTIFACT_SERVICE_RELEASE_ROOT="${SERVICE_RELEASE_ROOT:-${ARTIFACT_SHARED_DIR}/service-releases}"
  ARTIFACT_WEB_RELEASE_ROOT="${WEB_RELEASE_ROOT:-/opt/1panel/www/sites/admin-multi-tenant-test/web}"
  ARTIFACT_OPENRESTY_PROXY_CONFIG="${OPENRESTY_PROXY_CONFIG:-/opt/1panel/www/sites/admin-multi-tenant-test/proxy/root.conf}"

  local required_names=(
    APP_DOMAIN
    GITHUB_REPOSITORY
    GITHUB_DEPLOY_KEY
    DEPLOY_BRANCH
    MYSQL_DATABASE
    MYSQL_USER
    MYSQL_PASSWORD
    MINIO_ROOT_USER
    MINIO_ROOT_PASSWORD
    MINIO_BUCKET
  )
  local required_name
  for required_name in "${required_names[@]}"; do
    [[ -n "${!required_name:-}" ]] \
      || artifact_die "${ARTIFACT_ENV_FILE} 中缺少 ${required_name}"
  done
  [[ "${MYSQL_USER}" != "root" ]] || artifact_die "MYSQL_USER 禁止使用 root"
}

artifact_require_runtime() {
  artifact_require_command curl
  artifact_require_command docker
  artifact_require_command flock
  artifact_require_command git
  artifact_require_command tar
  docker compose version >/dev/null 2>&1 \
    || artifact_die "Docker Compose 不可用"
}

artifact_compose() {
  SERVICE_RELEASE_ROOT="${ARTIFACT_SERVICE_RELEASE_ROOT}" \
    docker compose \
      --project-name "${ARTIFACT_PROJECT_NAME}" \
      --env-file "${ARTIFACT_ENV_FILE}" \
      --file "${ARTIFACT_COMPOSE_FILE}" \
      "$@"
}

artifact_infra_compose() {
  docker compose \
    --project-name "${ARTIFACT_PROJECT_NAME}" \
    --env-file "${ARTIFACT_ENV_FILE}" \
    --file "${ARTIFACT_INFRA_COMPOSE_FILE}" \
    "$@"
}

artifact_validate_sha() {
  local commit_sha="$1"
  [[ "${commit_sha}" =~ ^[0-9a-f]{40}$ ]] \
    || artifact_die "Commit SHA 必须是 40 位小写十六进制"
}

artifact_target_includes() {
  local release_type="$1"
  local component="$2"
  [[ "${release_type}" == "apps" || "${release_type}" == "${component}" ]]
}

artifact_expected_components() {
  local release_type="$1"
  case "${release_type}" in
    web)
      printf 'web\n'
      ;;
    service)
      printf 'service\n'
      ;;
    apps)
      printf 'service,web\n'
      ;;
    *)
      artifact_die "产物类型只允许 web、service 或 apps"
      ;;
  esac
}

artifact_sha256() {
  local file_path="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${file_path}" | awk '{print $1}'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "${file_path}" | awk '{print $1}'
    return
  fi
  artifact_die "缺少 sha256sum 或 shasum"
}

artifact_manifest_value() {
  local manifest_file="$1"
  local key="$2"
  local value
  value="$(grep -E "^${key}=" "${manifest_file}" | head -n 1 | cut -d= -f2-)" \
    || true
  [[ -n "${value}" ]] || artifact_die "产物清单缺少 ${key}"
  printf '%s\n' "${value}"
}

artifact_load_state() {
  ARTIFACT_CURRENT_SHA=""
  ARTIFACT_CURRENT_SERVICE_SHA=""
  ARTIFACT_CURRENT_WEB_SHA=""
  ARTIFACT_PREVIOUS_SHA=""
  ARTIFACT_PREVIOUS_SERVICE_SHA=""
  ARTIFACT_PREVIOUS_WEB_SHA=""
  if [[ -f "${ARTIFACT_STATE_FILE}" ]]; then
    # shellcheck disable=SC1090
    source "${ARTIFACT_STATE_FILE}"
    ARTIFACT_CURRENT_SHA="${CURRENT_SHA:-}"
    ARTIFACT_CURRENT_SERVICE_SHA="${CURRENT_SERVICE_SHA:-}"
    ARTIFACT_CURRENT_WEB_SHA="${CURRENT_WEB_SHA:-}"
    ARTIFACT_PREVIOUS_SHA="${PREVIOUS_SHA:-}"
    ARTIFACT_PREVIOUS_SERVICE_SHA="${PREVIOUS_SERVICE_SHA:-}"
    ARTIFACT_PREVIOUS_WEB_SHA="${PREVIOUS_WEB_SHA:-}"
  fi
}

artifact_write_state() {
  local current_sha="$1"
  local current_service_sha="$2"
  local current_web_sha="$3"
  local previous_sha="$4"
  local previous_service_sha="$5"
  local previous_web_sha="$6"
  local temporary_state="${ARTIFACT_STATE_FILE}.tmp"
  {
    printf 'CURRENT_SHA=%q\n' "${current_sha}"
    printf 'CURRENT_SERVICE_SHA=%q\n' "${current_service_sha}"
    printf 'CURRENT_WEB_SHA=%q\n' "${current_web_sha}"
    printf 'PREVIOUS_SHA=%q\n' "${previous_sha}"
    printf 'PREVIOUS_SERVICE_SHA=%q\n' "${previous_service_sha}"
    printf 'PREVIOUS_WEB_SHA=%q\n' "${previous_web_sha}"
    printf 'UPDATED_AT=%q\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  } >"${temporary_state}"
  chmod 600 "${temporary_state}"
  mv "${temporary_state}" "${ARTIFACT_STATE_FILE}"
}

artifact_refresh_source() {
  local expected_sha="$1"
  if [[ -n "${DEPLOY_SOURCE_READY:-}" ]]; then
    [[ "${DEPLOY_SOURCE_READY}" == "${expected_sha}" ]] \
      || artifact_die "已准备源码 ${DEPLOY_SOURCE_READY} 与产物 ${expected_sha} 不一致"
    return
  fi

  [[ -f "${GITHUB_DEPLOY_KEY}" ]] \
    || artifact_die "缺少 GitHub Deploy Key：${GITHUB_DEPLOY_KEY}"
  chmod 600 "${GITHUB_DEPLOY_KEY}"
  [[ -z "$(git -C "${ARTIFACT_REPOSITORY_DIR}" status --porcelain)" ]] \
    || artifact_die "服务器源码目录存在未提交变更，拒绝覆盖"

  GIT_SSH_COMMAND="ssh -i ${GITHUB_DEPLOY_KEY} -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes" \
    git -C "${ARTIFACT_REPOSITORY_DIR}" fetch --quiet "${GITHUB_REPOSITORY}" "${DEPLOY_BRANCH}"
  local remote_sha
  remote_sha="$(git -C "${ARTIFACT_REPOSITORY_DIR}" rev-parse FETCH_HEAD)"
  [[ "${remote_sha}" == "${expected_sha}" ]] \
    || artifact_die "远端 ${DEPLOY_BRANCH}=${remote_sha}，产物=${expected_sha}"

  if [[ "$(git -C "${ARTIFACT_REPOSITORY_DIR}" rev-parse HEAD)" != "${expected_sha}" ]]; then
    git -C "${ARTIFACT_REPOSITORY_DIR}" checkout --detach "${expected_sha}"
    artifact_release_lock
    DEPLOY_SOURCE_READY="${expected_sha}" \
      exec "${ARTIFACT_SCRIPT_PATH}" "${ARTIFACT_ORIGINAL_ARGS[@]}"
  fi
  export DEPLOY_SOURCE_READY="${expected_sha}"
}

artifact_verify_and_extract() {
  local bundle_path="$1"
  [[ -f "${bundle_path}" ]] || artifact_die "产物包不存在：${bundle_path}"
  [[ -f "${bundle_path}.sha256" ]] \
    || artifact_die "缺少校验文件：${bundle_path}.sha256"

  local expected_checksum
  local actual_checksum
  expected_checksum="$(awk 'NR == 1 {print $1}' "${bundle_path}.sha256")"
  [[ "${expected_checksum}" =~ ^[0-9a-f]{64}$ ]] \
    || artifact_die "校验文件中的 SHA-256 格式无效"
  actual_checksum="$(artifact_sha256 "${bundle_path}")"
  [[ "${actual_checksum}" == "${expected_checksum}" ]] \
    || artifact_die "产物包 SHA-256 校验失败"

  ARTIFACT_TEMP_DIR="$(mktemp -d)"
  local archive_listing="${ARTIFACT_TEMP_DIR}/archive-list.txt"
  local archive_details="${ARTIFACT_TEMP_DIR}/archive-details.txt"
  tar -tzf "${bundle_path}" >"${archive_listing}"
  tar -tvzf "${bundle_path}" >"${archive_details}"
  if grep -Eq '(^/|(^|/)\.\.(/|$))' "${archive_listing}"; then
    artifact_die "产物包包含不安全路径"
  fi
  if awk '$1 ~ /^[lh]/ {found=1} END {exit found ? 0 : 1}' "${archive_details}"; then
    artifact_die "产物包禁止包含符号链接或硬链接"
  fi
  tar -xzf "${bundle_path}" -C "${ARTIFACT_TEMP_DIR}"

  local manifest_file="${ARTIFACT_TEMP_DIR}/manifest.env"
  [[ -f "${manifest_file}" ]] || artifact_die "产物包缺少 manifest.env"
  local format_version
  local release_type
  local components
  local commit_sha
  local target_platform
  format_version="$(artifact_manifest_value "${manifest_file}" FORMAT_VERSION)"
  release_type="$(artifact_manifest_value "${manifest_file}" RELEASE_TYPE)"
  components="$(artifact_manifest_value "${manifest_file}" COMPONENTS)"
  commit_sha="$(artifact_manifest_value "${manifest_file}" COMMIT_SHA)"
  target_platform="$(artifact_manifest_value "${manifest_file}" TARGET_PLATFORM)"

  [[ "${format_version}" == "1" ]] || artifact_die "不支持的产物格式版本"
  [[ "${components}" == "$(artifact_expected_components "${release_type}")" ]] \
    || artifact_die "组件清单与产物类型不一致"
  artifact_validate_sha "${commit_sha}"
  [[ "${target_platform}" == "linux/amd64" ]] \
    || artifact_die "产物目标架构必须是 linux/amd64"
  case "$(uname -m)" in
    x86_64|amd64)
      ;;
    *)
      artifact_die "服务器 CPU 架构不是 linux/amd64"
      ;;
  esac

  if artifact_target_includes "${release_type}" service; then
    [[ -x "${ARTIFACT_TEMP_DIR}/service/server" ]] \
      || artifact_die "Go 产物缺少可执行 server"
    [[ -x "${ARTIFACT_TEMP_DIR}/service/bootstrap-owner" ]] \
      || artifact_die "Go 产物缺少可执行 bootstrap-owner"
    [[ -d "${ARTIFACT_TEMP_DIR}/service/migrations" ]] \
      || artifact_die "Go 产物缺少 migrations"
  fi
  if artifact_target_includes "${release_type}" web; then
    [[ -f "${ARTIFACT_TEMP_DIR}/web/index.html" ]] \
      || artifact_die "Web 产物缺少 index.html"
  fi

  ARTIFACT_RELEASE_TYPE="${release_type}"
  ARTIFACT_COMMIT_SHA="${commit_sha}"
}

artifact_stage_component() {
  local component="$1"
  local commit_sha="$2"
  local release_root
  if [[ "${component}" == "service" ]]; then
    release_root="${ARTIFACT_SERVICE_RELEASE_ROOT}"
  else
    release_root="${ARTIFACT_WEB_RELEASE_ROOT}"
  fi

  local release_path="${release_root}/releases/${commit_sha}"
  if [[ -d "${release_path}" ]]; then
    artifact_log "${component} 版本目录已存在，复用 ${commit_sha}"
    return
  fi

  local temporary_release="${release_root}/releases/.${commit_sha}.$$"
  mkdir -p "${temporary_release}"
  cp -R "${ARTIFACT_TEMP_DIR}/${component}/." "${temporary_release}/"
  find "${temporary_release}" -type d -exec chmod 755 {} +
  find "${temporary_release}" -type f -exec chmod 644 {} +
  if [[ "${component}" == "service" ]]; then
    chmod 755 \
      "${temporary_release}/server" \
      "${temporary_release}/bootstrap-owner"
  fi
  mv "${temporary_release}" "${release_path}"
  artifact_log "已暂存 ${component} 产物：${commit_sha}"
}

artifact_switch_release() {
  local release_root="$1"
  local commit_sha="$2"
  [[ -d "${release_root}/releases/${commit_sha}" ]] \
    || artifact_die "版本目录不存在：${release_root}/releases/${commit_sha}"
  local temporary_link="${release_root}/.current.$$"
  ln -s "releases/${commit_sha}" "${temporary_link}"
  mv -Tf "${temporary_link}" "${release_root}/current"
}

artifact_require_infrastructure() {
  local container_name
  for container_name in admin-multi-tenant-mysql-1 admin-multi-tenant-redis-1 admin-multi-tenant-minio-1; do
    [[ "$(docker inspect --format '{{.State.Status}}' "${container_name}" 2>/dev/null || true)" == "running" ]] \
      || artifact_die "基础设施容器未运行：${container_name}"
  done
  docker network inspect admin-multi-tenant-internal >/dev/null 2>&1 \
    || artifact_die "缺少 Docker 网络 admin-multi-tenant-internal"
}

artifact_wait_for_url() {
  local url="$1"
  local description="$2"
  local attempts=30
  local attempt
  for ((attempt = 1; attempt <= attempts; attempt++)); do
    if curl --fail --silent --show-error --max-time 5 "${url}" >/dev/null; then
      return
    fi
    sleep 2
  done
  artifact_error "${description} 健康检查失败：${url}"
  return 1
}

artifact_start_service() {
  docker image inspect "${ARTIFACT_RUNTIME_IMAGE}" >/dev/null 2>&1 \
    || artifact_die "缺少固定 Go 运行镜像 ${ARTIFACT_RUNTIME_IMAGE}"
  artifact_require_infrastructure
  artifact_compose up --pull never -d --no-deps --force-recreate \
    --wait --wait-timeout 180 service
  artifact_wait_for_url "http://127.0.0.1:18080/healthz" "Go 存活探针" \
    && artifact_wait_for_url "http://127.0.0.1:18080/readyz" "Go 就绪探针"
}

artifact_require_static_web_config() {
  [[ -f "${ARTIFACT_OPENRESTY_PROXY_CONFIG}" ]] \
    || artifact_die "缺少 OpenResty 站点配置 ${ARTIFACT_OPENRESTY_PROXY_CONFIG}"
  grep -Fq "/www/sites/admin-multi-tenant-test/web/current" "${ARTIFACT_OPENRESTY_PROXY_CONFIG}" \
    || artifact_die "OpenResty 尚未切换到 Web 静态产物目录"
}

artifact_check_web() {
  artifact_require_static_web_config
  curl --fail --silent --show-error --max-time 10 \
    --resolve "${APP_DOMAIN}:443:127.0.0.1" \
    "https://${APP_DOMAIN}/" >/dev/null \
    && curl --fail --silent --show-error --max-time 10 \
      --resolve "${APP_DOMAIN}:443:127.0.0.1" \
      "https://${APP_DOMAIN}/platform/login" >/dev/null
}

artifact_run_goose() {
  local service_sha="$1"
  shift
  artifact_compose run --pull never --rm -T --no-deps service \
    sh -ceu '
      export GOOSE_DRIVER=mysql
      export GOOSE_DBSTRING="${MYSQL_USER}:${MYSQL_PASSWORD}@tcp(${MYSQL_HOST}:${MYSQL_PORT})/${MYSQL_DATABASE}?parseTime=true&multiStatements=true"
      export GOOSE_MIGRATION_DIR="$1"
      shift
      exec /app/goose -env=none "$@"
    ' sh "/releases/releases/${service_sha}/migrations" "$@"
}

artifact_max_migration_version() {
  local service_sha="$1"
  local latest_file
  latest_file="$(find "${ARTIFACT_SERVICE_RELEASE_ROOT}/releases/${service_sha}/migrations" \
    -maxdepth 1 -type f -name '[0-9][0-9][0-9][0-9][0-9]_*.sql' \
    -print | sort | tail -n 1)"
  [[ -n "${latest_file}" ]] || artifact_die "目标 Go 产物没有 SQL Migration"
  basename "${latest_file}" | cut -d_ -f1 | sed 's/^0*//'
}

artifact_database_version() {
  local service_sha="$1"
  local version_output
  local parsed_version
  version_output="$(artifact_run_goose "${service_sha}" version 2>&1)" \
    || artifact_die "读取 Goose 版本失败：${version_output}"
  parsed_version="$(printf '%s\n' "${version_output}" \
    | grep -Eo '[0-9]+[[:space:]]*$' | tail -n 1 | tr -d '[:space:]')" || true
  [[ -n "${parsed_version}" ]] \
    || artifact_die "无法解析 Goose 版本：${version_output}"
  printf '%s\n' "${parsed_version}"
}

artifact_migration_status() {
  local service_sha="$1"
  local status_output
  status_output="$(artifact_run_goose "${service_sha}" status 2>&1)" \
    || artifact_die "读取 Goose 状态失败：${status_output}"
  printf '%s\n' "${status_output}"
}

artifact_backup_data() {
  local commit_sha="$1"
  local timestamp
  timestamp="$(date +%Y%m%d-%H%M%S)"
  local backup_directory="${ARTIFACT_BACKUP_ROOT}/${timestamp}-${commit_sha:0:12}"
  local mysql_temporary="${backup_directory}/mysql.sql.tmp"
  local mysql_backup="${backup_directory}/mysql.sql"
  mkdir -p "${backup_directory}/minio"
  chmod 700 "${backup_directory}" "${backup_directory}/minio"

  if [[ -n "$(artifact_compose ps --status running --quiet service)" ]]; then
    artifact_compose pause service
    ARTIFACT_SERVICE_PAUSED="true"
  fi

  artifact_log "备份 MySQL 到 ${backup_directory}"
  docker exec -e MYSQL_PWD="${MYSQL_PASSWORD}" admin-multi-tenant-mysql-1 \
    mysqldump \
      --user="${MYSQL_USER}" \
      --single-transaction \
      --routines \
      --triggers \
      --events \
      --hex-blob \
      --set-gtid-purged=OFF \
      "${MYSQL_DATABASE}" >"${mysql_temporary}"
  mv "${mysql_temporary}" "${mysql_backup}"

  artifact_log "备份 MinIO 数据"
  if ! MC_HOST_source="http://${MINIO_ROOT_USER}:${MINIO_ROOT_PASSWORD}@minio:9000" \
    docker run --rm \
    --network admin-multi-tenant-internal \
    --env MC_HOST_source \
    --volume "${backup_directory}/minio:/backup" \
    --entrypoint mc \
    quay.io/minio/minio:RELEASE.2025-09-07T16-13-09Z \
    mirror --overwrite "source/${MINIO_BUCKET}" /backup; then
    artifact_resume_service
    artifact_die "MinIO 备份失败"
  fi

  artifact_resume_service
  cp "${ARTIFACT_ENV_FILE}" "${backup_directory}/server.env"
  chmod 600 "${backup_directory}/server.env" "${mysql_backup}"
  printf '%s\n' "${commit_sha}" >"${backup_directory}/commit-sha.txt"
  artifact_log "备份完成：${backup_directory}"
}

artifact_check_migrations() {
  local service_sha="$1"
  local allow_migration="$2"
  local expected_version
  local database_version
  local migration_status
  expected_version="$(artifact_max_migration_version "${service_sha}")"
  migration_status="$(artifact_migration_status "${service_sha}")"
  database_version="$(artifact_database_version "${service_sha}")"

  if ((database_version > expected_version)); then
    artifact_die "数据库版本 ${database_version} 高于代码版本 ${expected_version}"
  fi
  if grep -Fq "Pending" <<<"${migration_status}"; then
    [[ "${allow_migration}" == "true" ]] \
      || artifact_die "存在待执行 Migration；审核后使用 --migrate"
    artifact_backup_data "${service_sha}"
    artifact_run_goose "${service_sha}" up
    database_version="$(artifact_database_version "${service_sha}")"
    migration_status="$(artifact_migration_status "${service_sha}")"
    [[ "${database_version}" == "${expected_version}" ]] \
      || artifact_die "Migration 后数据库版本 ${database_version}，期望 ${expected_version}"
    grep -Fq "Pending" <<<"${migration_status}" \
      && artifact_die "Migration 后仍存在 Pending 状态"
  elif ((database_version < expected_version)); then
    artifact_die "数据库版本低于目标产物，但 Goose 未报告 Pending"
  fi
}

artifact_restore_components() {
  local release_type="$1"
  local service_sha="$2"
  local web_sha="$3"
  local restored="true"
  if artifact_target_includes "${release_type}" service; then
    if [[ -n "${service_sha}" && -d "${ARTIFACT_SERVICE_RELEASE_ROOT}/releases/${service_sha}" ]]; then
      artifact_switch_release "${ARTIFACT_SERVICE_RELEASE_ROOT}" "${service_sha}"
      artifact_start_service || restored="false"
    else
      restored="false"
    fi
  fi
  if artifact_target_includes "${release_type}" web; then
    if [[ -n "${web_sha}" && -d "${ARTIFACT_WEB_RELEASE_ROOT}/releases/${web_sha}" ]]; then
      artifact_switch_release "${ARTIFACT_WEB_RELEASE_ROOT}" "${web_sha}"
      artifact_check_web || restored="false"
    else
      restored="false"
    fi
  fi
  [[ "${restored}" == "true" ]]
}

artifact_stage_bundle() {
  local release_type="$1"
  local commit_sha="$2"
  if artifact_target_includes "${release_type}" service; then
    artifact_stage_component service "${commit_sha}"
  fi
  if artifact_target_includes "${release_type}" web; then
    artifact_stage_component web "${commit_sha}"
  fi
}

artifact_deploy_bundle() {
  local bundle_path="$1"
  local allow_migration="$2"
  artifact_load_environment
  artifact_prepare_directories
  artifact_acquire_lock
  artifact_require_runtime

  artifact_verify_and_extract "${bundle_path}"
  local release_type="${ARTIFACT_RELEASE_TYPE}"
  local commit_sha="${ARTIFACT_COMMIT_SHA}"
  artifact_refresh_source "${commit_sha}"
  artifact_load_state
  artifact_stage_bundle "${release_type}" "${commit_sha}"

  if [[ "${release_type}" != "apps" ]]; then
    [[ -n "${ARTIFACT_CURRENT_SERVICE_SHA}" && -n "${ARTIFACT_CURRENT_WEB_SHA}" ]] \
      || artifact_die "首次产物发布必须使用 apps 完成运行方式切换"
  fi

  local next_service_sha="${ARTIFACT_CURRENT_SERVICE_SHA}"
  local next_web_sha="${ARTIFACT_CURRENT_WEB_SHA}"
  if artifact_target_includes "${release_type}" service; then
    next_service_sha="${commit_sha}"
    artifact_check_migrations "${commit_sha}" "${allow_migration}"
    artifact_switch_release "${ARTIFACT_SERVICE_RELEASE_ROOT}" "${commit_sha}"
    if ! artifact_start_service; then
      artifact_error "新 Go 产物健康检查失败"
      artifact_restore_components service \
        "${ARTIFACT_CURRENT_SERVICE_SHA}" "${ARTIFACT_CURRENT_WEB_SHA}" || true
      artifact_die "Go 发布失败，数据库未执行 Down"
    fi
  fi

  if artifact_target_includes "${release_type}" web; then
    next_web_sha="${commit_sha}"
    artifact_require_static_web_config
    artifact_switch_release "${ARTIFACT_WEB_RELEASE_ROOT}" "${commit_sha}"
    if ! artifact_check_web; then
      artifact_error "新 Web 产物健康检查失败"
      artifact_restore_components "${release_type}" \
        "${ARTIFACT_CURRENT_SERVICE_SHA}" "${ARTIFACT_CURRENT_WEB_SHA}" || true
      artifact_die "Web 发布失败，已尝试恢复发布前产物"
    fi
  fi

  artifact_write_state \
    "${commit_sha}" "${next_service_sha}" "${next_web_sha}" \
    "${ARTIFACT_CURRENT_SHA}" "${ARTIFACT_CURRENT_SERVICE_SHA}" "${ARTIFACT_CURRENT_WEB_SHA}"
  artifact_log "发布完成：版本=${commit_sha}，service=${next_service_sha}，web=${next_web_sha}"
}

artifact_stage_only() {
  local bundle_path="$1"
  artifact_load_environment
  artifact_prepare_directories
  artifact_acquire_lock
  artifact_require_runtime
  artifact_verify_and_extract "${bundle_path}"
  local release_type="${ARTIFACT_RELEASE_TYPE}"
  local commit_sha="${ARTIFACT_COMMIT_SHA}"
  artifact_refresh_source "${commit_sha}"
  artifact_stage_bundle "${release_type}" "${commit_sha}"
  artifact_log "产物暂存完成：${commit_sha}"
}

artifact_initialize_migrations() {
  local service_sha="$1"
  local allow_migration="$2"
  local expected_version
  local database_version
  local migration_status
  expected_version="$(artifact_max_migration_version "${service_sha}")"
  migration_status="$(artifact_migration_status "${service_sha}")"
  database_version="$(artifact_database_version "${service_sha}")"

  if ((database_version > expected_version)); then
    artifact_die "数据库版本 ${database_version} 高于代码版本 ${expected_version}"
  fi
  if grep -Fq "Pending" <<<"${migration_status}"; then
    [[ "${allow_migration}" == "true" ]] \
      || artifact_die "全新数据库存在待执行 Migration；请显式使用 initialize --migrate"
    artifact_run_goose "${service_sha}" up
    database_version="$(artifact_database_version "${service_sha}")"
    migration_status="$(artifact_migration_status "${service_sha}")"
    [[ "${database_version}" == "${expected_version}" ]] \
      || artifact_die "初始化 Migration 后数据库版本 ${database_version}，期望 ${expected_version}"
    grep -Fq "Pending" <<<"${migration_status}" \
      && artifact_die "初始化 Migration 后仍存在 Pending 状态"
  elif ((database_version < expected_version)); then
    artifact_die "数据库版本低于目标产物，但 Goose 未报告 Pending"
  fi
}

artifact_initialize() {
  local bundle_path="$1"
  local allow_migration="$2"
  artifact_load_environment
  artifact_prepare_directories
  artifact_acquire_lock
  artifact_require_runtime
  artifact_load_state
  [[ -z "${ARTIFACT_CURRENT_SHA}" ]] \
    || artifact_die "产物发布已经初始化，禁止重复执行 initialize"

  artifact_verify_and_extract "${bundle_path}"
  local release_type="${ARTIFACT_RELEASE_TYPE}"
  local commit_sha="${ARTIFACT_COMMIT_SHA}"
  [[ "${release_type}" == "apps" ]] \
    || artifact_die "首次初始化必须使用 apps 产物包"
  artifact_refresh_source "${commit_sha}"
  artifact_stage_bundle "${release_type}" "${commit_sha}"
  docker image inspect "${ARTIFACT_RUNTIME_IMAGE}" >/dev/null 2>&1 \
    || artifact_die "缺少固定 Go 运行镜像 ${ARTIFACT_RUNTIME_IMAGE}"
  artifact_require_infrastructure
  artifact_initialize_migrations "${commit_sha}" "${allow_migration}"

  artifact_switch_release "${ARTIFACT_SERVICE_RELEASE_ROOT}" "${commit_sha}"
  artifact_start_service \
    || artifact_die "固定 Go 运行容器启动失败；请使用旧镜像发布流程人工恢复"
  artifact_switch_release "${ARTIFACT_WEB_RELEASE_ROOT}" "${commit_sha}"
  artifact_write_state "${commit_sha}" "${commit_sha}" "${commit_sha}" "" "" ""
  artifact_log "初始化完成；下一步切换 OpenResty 静态配置并执行 verify"
}

artifact_infra_up() {
  artifact_load_environment
  artifact_prepare_directories
  artifact_acquire_lock
  artifact_require_runtime
  artifact_log "使用已加载镜像启动 MySQL、Redis、MinIO"
  artifact_infra_compose up --pull never -d --wait --wait-timeout 180
  artifact_require_infrastructure
  artifact_log "基础设施容器已启动并通过健康检查"
}

artifact_bootstrap_owner() {
  [[ -t 0 && -t 1 ]] \
    || artifact_die "bootstrap-owner 必须通过 ssh -t 在交互式终端运行"
  artifact_load_environment
  artifact_prepare_directories
  artifact_acquire_lock
  artifact_require_runtime
  artifact_load_state
  [[ -n "${ARTIFACT_CURRENT_SERVICE_SHA}" ]] \
    || artifact_die "产物发布尚未初始化"
  [[ -x "${ARTIFACT_SERVICE_RELEASE_ROOT}/current/bootstrap-owner" ]] \
    || artifact_die "当前 Go 产物缺少 bootstrap-owner"

  local owner_account
  local owner_name
  read -r -p "平台所有者登录账号：" owner_account
  read -r -p "平台所有者姓名：" owner_name
  artifact_compose run --pull never --rm --no-deps service \
    /releases/current/bootstrap-owner \
      -account "${owner_account}" \
      -name "${owner_name}"
}

artifact_verify() {
  artifact_load_environment
  artifact_prepare_directories
  artifact_require_runtime
  artifact_load_state
  [[ -n "${ARTIFACT_CURRENT_SHA}" \
    && -n "${ARTIFACT_CURRENT_SERVICE_SHA}" \
    && -n "${ARTIFACT_CURRENT_WEB_SHA}" ]] \
    || artifact_die "产物发布尚未初始化"
  [[ -d "${ARTIFACT_SERVICE_RELEASE_ROOT}/releases/${ARTIFACT_CURRENT_SERVICE_SHA}" ]] \
    || artifact_die "当前 Go 版本目录不存在"
  [[ -d "${ARTIFACT_WEB_RELEASE_ROOT}/releases/${ARTIFACT_CURRENT_WEB_SHA}" ]] \
    || artifact_die "当前 Web 版本目录不存在"
  artifact_wait_for_url "http://127.0.0.1:18080/healthz" "Go 存活探针"
  artifact_wait_for_url "http://127.0.0.1:18080/readyz" "Go 就绪探针"
  artifact_check_web || artifact_die "OpenResty Web 静态站点检查失败"
  artifact_log "当前 Web、Go 产物验证通过"
}

artifact_rollback_previous() {
  artifact_load_environment
  artifact_prepare_directories
  artifact_acquire_lock
  artifact_require_runtime
  artifact_load_state
  [[ -n "${ARTIFACT_PREVIOUS_SHA}" \
    && -n "${ARTIFACT_PREVIOUS_SERVICE_SHA}" \
    && -n "${ARTIFACT_PREVIOUS_WEB_SHA}" ]] \
    || artifact_die "没有可用的上一组产物版本"

  local old_current_sha="${ARTIFACT_CURRENT_SHA}"
  local old_service_sha="${ARTIFACT_CURRENT_SERVICE_SHA}"
  local old_web_sha="${ARTIFACT_CURRENT_WEB_SHA}"
  artifact_restore_components apps \
    "${ARTIFACT_PREVIOUS_SERVICE_SHA}" "${ARTIFACT_PREVIOUS_WEB_SHA}" \
    || artifact_die "上一组产物回滚失败，当前状态未改写"
  artifact_write_state \
    "${ARTIFACT_PREVIOUS_SHA}" \
    "${ARTIFACT_PREVIOUS_SERVICE_SHA}" \
    "${ARTIFACT_PREVIOUS_WEB_SHA}" \
    "${old_current_sha}" "${old_service_sha}" "${old_web_sha}"
  artifact_log "已恢复上一组产物，数据库未执行 Down"
}

artifact_status() {
  artifact_load_environment
  artifact_prepare_directories
  artifact_require_runtime
  artifact_load_state
  printf '当前版本：%s\n' "${ARTIFACT_CURRENT_SHA:-尚未记录}"
  printf '当前 Go 产物：%s\n' "${ARTIFACT_CURRENT_SERVICE_SHA:-尚未记录}"
  printf '当前 Web 产物：%s\n' "${ARTIFACT_CURRENT_WEB_SHA:-尚未记录}"
  printf '上一版本：%s\n' "${ARTIFACT_PREVIOUS_SHA:-尚未记录}"
  printf '上一 Go 产物：%s\n' "${ARTIFACT_PREVIOUS_SERVICE_SHA:-尚未记录}"
  printf '上一 Web 产物：%s\n' "${ARTIFACT_PREVIOUS_WEB_SHA:-尚未记录}"
  artifact_infra_compose ps mysql redis minio
  artifact_compose ps service
}

artifact_backup() {
  artifact_load_environment
  artifact_prepare_directories
  artifact_acquire_lock
  artifact_require_runtime
  artifact_load_state
  local commit_sha="${ARTIFACT_CURRENT_SHA:-$(git -C "${ARTIFACT_REPOSITORY_DIR}" rev-parse HEAD)}"
  artifact_backup_data "${commit_sha}"
}

artifact_main() {
  local command_name="${1:-}"
  shift || true
  case "${command_name}" in
    deploy)
      if (($# == 1)); then
        artifact_deploy_bundle "$1" "false"
      elif (($# == 2)) && [[ "$2" == "--migrate" ]]; then
        artifact_deploy_bundle "$1" "true"
      else
        artifact_die "deploy 需要产物包路径和可选参数 --migrate"
      fi
      ;;
    stage)
      (($# == 1)) || artifact_die "stage 需要产物包路径"
      artifact_stage_only "$1"
      ;;
    infra)
      (($# == 1)) && [[ "$1" == "up" ]] \
        || artifact_die "infra 只支持 up"
      artifact_infra_up
      ;;
    initialize)
      if (($# == 1)); then
        artifact_initialize "$1" "false"
      elif (($# == 2)) && [[ "$2" == "--migrate" ]]; then
        artifact_initialize "$1" "true"
      else
        artifact_die "initialize 需要 apps 产物包路径和可选参数 --migrate"
      fi
      ;;
    bootstrap-owner)
      (($# == 0)) || artifact_die "bootstrap-owner 不接受参数"
      artifact_bootstrap_owner
      ;;
    verify)
      (($# == 0)) || artifact_die "verify 不接受参数"
      artifact_verify
      ;;
    rollback)
      (($# == 1)) && [[ "$1" == "previous" ]] \
        || artifact_die "rollback 只支持 previous"
      artifact_rollback_previous
      ;;
    status)
      (($# == 0)) || artifact_die "status 不接受参数"
      artifact_status
      ;;
    backup)
      (($# == 0)) || artifact_die "backup 不接受参数"
      artifact_backup
      ;;
    -h|--help|help|"")
      artifact_usage
      ;;
    *)
      artifact_usage
      artifact_die "未知命令：${command_name}"
      ;;
  esac
}

trap 'artifact_resume_service; artifact_cleanup' EXIT

artifact_main "$@"
