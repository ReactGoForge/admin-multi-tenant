#!/usr/bin/env bash

set -Eeuo pipefail

umask 077

RELEASE_SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
RELEASE_REPOSITORY_DIR="$(cd -- "${RELEASE_SCRIPT_DIR}/.." && pwd)"
RELEASE_OUTPUT_DIR="${ADMIN_MULTI_TENANT_RELEASE_OUTPUT_DIR:-${RELEASE_SCRIPT_DIR}/dist}"
RELEASE_REMOTE_HOST="${ADMIN_MULTI_TENANT_DEPLOY_HOST:-admin-multi-tenant-test}"
RELEASE_REMOTE_APP_DIR="${ADMIN_MULTI_TENANT_REMOTE_APP_DIR:-/opt/reactgoforge/admin-multi-tenant/app}"
RELEASE_REMOTE_ARTIFACT_DIR="${ADMIN_MULTI_TENANT_REMOTE_ARTIFACT_DIR:-/opt/reactgoforge/admin-multi-tenant/shared/artifacts}"
RELEASE_BRANCH="${ADMIN_MULTI_TENANT_DEPLOY_BRANCH:-main}"
RELEASE_TARGET_PLATFORM="linux/amd64"
RELEASE_TEMP_DIR=""
RELEASE_BUNDLE_PATH=""

release_log() {
  printf '[admin-multi-tenant-release] %s\n' "$*"
}

release_die() {
  printf '[admin-multi-tenant-release] 错误：%s\n' "$*" >&2
  exit 1
}

release_usage() {
  cat <<'EOF'
用法：
  ./deploy/release.sh web
  ./deploy/release.sh service [--migrate]
  ./deploy/release.sh apps [--migrate]
  ./deploy/release.sh apps --initialize
  ./deploy/release.sh apps --initialize --migrate

说明：
  web       检查并构建 Web 静态产物，上传后由 OpenResty 直接提供。
  service   测试并交叉编译 linux/amd64 Go 二进制，连同 Migration 上传。
  apps      同时发布 Go 与 Web；先验证 Go，再切换 Web。
  --migrate 仅允许 service 或 apps，远端备份后执行待执行 Goose Migration。
  --initialize 仅用于首次建立产物发布状态；默认不执行 Migration。
  --initialize --migrate 仅用于全新数据库，显式执行全部 Goose Migration。
EOF
}

release_cleanup() {
  if [[ -n "${RELEASE_TEMP_DIR}" && -d "${RELEASE_TEMP_DIR}" ]]; then
    rm -rf -- "${RELEASE_TEMP_DIR}"
  fi
}

release_require_command() {
  local command_name="$1"
  command -v "${command_name}" >/dev/null 2>&1 \
    || release_die "缺少命令 ${command_name}"
}

release_validate_target() {
  local release_target="$1"
  case "${release_target}" in
    web|service|apps)
      ;;
    *)
      release_die "发布目标只允许 web、service 或 apps"
      ;;
  esac
}

release_target_includes() {
  local release_target="$1"
  local component="$2"
  [[ "${release_target}" == "apps" || "${release_target}" == "${component}" ]]
}

release_components() {
  local release_target="$1"
  case "${release_target}" in
    web)
      printf 'web\n'
      ;;
    service)
      printf 'service\n'
      ;;
    apps)
      printf 'service,web\n'
      ;;
  esac
}

release_sha256() {
  local file_path="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${file_path}" | awk '{print $1}'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "${file_path}" | awk '{print $1}'
    return
  fi
  release_die "缺少 sha256sum 或 shasum"
}

release_require_clean_pushed_commit() {
  [[ -z "$(git -C "${RELEASE_REPOSITORY_DIR}" status --porcelain)" ]] \
    || release_die "工作区存在未提交变更，请先提交或暂存后再发布"

  local current_branch
  current_branch="$(git -C "${RELEASE_REPOSITORY_DIR}" branch --show-current)"
  [[ "${current_branch}" == "${RELEASE_BRANCH}" ]] \
    || release_die "当前分支为 ${current_branch:-detached}，发布要求 ${RELEASE_BRANCH}"

  local commit_sha
  commit_sha="$(git -C "${RELEASE_REPOSITORY_DIR}" rev-parse HEAD)"
  [[ "${commit_sha}" =~ ^[0-9a-f]{40}$ ]] \
    || release_die "无法读取完整 Commit SHA"

  local remote_sha
  remote_sha="$(git -C "${RELEASE_REPOSITORY_DIR}" \
    ls-remote origin "refs/heads/${RELEASE_BRANCH}" | awk 'NR == 1 {print $1}')"
  [[ "${remote_sha}" == "${commit_sha}" ]] \
    || release_die "本地 HEAD 尚未推送到 origin/${RELEASE_BRANCH}"

  printf '%s\n' "${commit_sha}"
}

release_build_web() {
  local payload_root="$1"
  release_require_command pnpm
  release_log "检查并构建 Web"
  pnpm --dir "${RELEASE_REPOSITORY_DIR}/apps/web" run check
  pnpm --dir "${RELEASE_REPOSITORY_DIR}/apps/web" run build
  [[ -f "${RELEASE_REPOSITORY_DIR}/apps/web/build/client/index.html" ]] \
    || release_die "Web 构建产物缺少 build/client/index.html"
  mkdir -p "${payload_root}/web"
  cp -R "${RELEASE_REPOSITORY_DIR}/apps/web/build/client/." "${payload_root}/web/"
}

release_build_service() {
  local payload_root="$1"
  release_require_command go
  release_log "测试并交叉编译 Go"
  (
    cd "${RELEASE_REPOSITORY_DIR}/apps/service"
    go test ./...
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
      go build -trimpath -ldflags="-s -w" \
      -o "${payload_root}/service/server" ./cmd/server
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
      go build -trimpath -ldflags="-s -w" \
      -o "${payload_root}/service/bootstrap-owner" ./cmd/bootstrap-owner
  )
  chmod 755 \
    "${payload_root}/service/server" \
    "${payload_root}/service/bootstrap-owner"
  mkdir -p "${payload_root}/service/migrations"
  cp -R "${RELEASE_REPOSITORY_DIR}/apps/service/migrations/." \
    "${payload_root}/service/migrations/"
}

release_create_bundle() {
  local release_target="$1"
  local commit_sha="$2"
  local payload_root="${RELEASE_TEMP_DIR}/payload"
  mkdir -p "${payload_root}"

  if release_target_includes "${release_target}" service; then
    mkdir -p "${payload_root}/service"
    release_build_service "${payload_root}"
  fi
  if release_target_includes "${release_target}" web; then
    release_build_web "${payload_root}"
  fi

  {
    printf 'FORMAT_VERSION=1\n'
    printf 'RELEASE_TYPE=%s\n' "${release_target}"
    printf 'COMPONENTS=%s\n' "$(release_components "${release_target}")"
    printf 'COMMIT_SHA=%s\n' "${commit_sha}"
    printf 'TARGET_PLATFORM=%s\n' "${RELEASE_TARGET_PLATFORM}"
  } >"${payload_root}/manifest.env"

  mkdir -p "${RELEASE_OUTPUT_DIR}"
  local bundle_name="admin-multi-tenant-${release_target}-artifacts-${commit_sha:0:12}-linux-amd64.tar.gz"
  local bundle_path="${RELEASE_OUTPUT_DIR}/${bundle_name}"
  local temporary_bundle="${bundle_path}.tmp"
  tar -czf "${temporary_bundle}" -C "${payload_root}" .
  mv "${temporary_bundle}" "${bundle_path}"

  local checksum
  checksum="$(release_sha256 "${bundle_path}")"
  printf '%s  %s\n' "${checksum}" "${bundle_name}" >"${bundle_path}.sha256"
  RELEASE_BUNDLE_PATH="${bundle_path}"
}

release_upload_and_deploy() {
  local bundle_path="$1"
  local release_action="$2"
  local bundle_name
  bundle_name="$(basename "${bundle_path}")"
  local remote_bundle="${RELEASE_REMOTE_ARTIFACT_DIR}/${bundle_name}"

  release_log "上传产物包到 ${RELEASE_REMOTE_HOST}"
  ssh "${RELEASE_REMOTE_HOST}" \
    "install -d -m 700 '${RELEASE_REMOTE_ARTIFACT_DIR}'"
  scp "${bundle_path}" "${bundle_path}.sha256" \
    "${RELEASE_REMOTE_HOST}:${RELEASE_REMOTE_ARTIFACT_DIR}/"

  release_log "调用服务器原子发布"
  local remote_command="deploy"
  local remote_argument=""
  if [[ "${release_action}" == "migrate" ]]; then
    remote_argument=" --migrate"
  elif [[ "${release_action}" == "initialize" ]]; then
    remote_command="initialize"
  elif [[ "${release_action}" == "initialize-migrate" ]]; then
    remote_command="initialize"
    remote_argument=" --migrate"
  fi
  ssh "${RELEASE_REMOTE_HOST}" \
    "cd '${RELEASE_REMOTE_APP_DIR}' && ./deploy/artifact-deploy.sh ${remote_command} '${remote_bundle}'${remote_argument}"
}

release_main() {
  local release_target="${1:-}"
  shift || true
  if [[ "${release_target}" == "-h" || "${release_target}" == "--help" || "${release_target}" == "help" || -z "${release_target}" ]]; then
    release_usage
    return
  fi
  release_validate_target "${release_target}"

  local release_action="deploy"
  if (($# == 1)) && [[ "$1" == "--migrate" ]]; then
    [[ "${release_target}" != "web" ]] \
      || release_die "web 发布不允许使用 --migrate"
    release_action="migrate"
  elif (($# == 1)) && [[ "$1" == "--initialize" ]]; then
    [[ "${release_target}" == "apps" ]] \
      || release_die "--initialize 只允许用于 apps"
    release_action="initialize"
  elif (($# == 2)) \
    && [[ "${release_target}" == "apps" ]] \
    && { [[ "$1" == "--initialize" && "$2" == "--migrate" ]] \
      || [[ "$1" == "--migrate" && "$2" == "--initialize" ]]; }; then
    release_action="initialize-migrate"
  elif (($# != 0)); then
    release_die "只接受 --migrate、--initialize 或 apps --initialize --migrate"
  fi

  release_require_command git
  release_require_command scp
  release_require_command ssh
  release_require_command tar
  RELEASE_TEMP_DIR="$(mktemp -d)"

  local commit_sha
  commit_sha="$(release_require_clean_pushed_commit)"
  release_create_bundle "${release_target}" "${commit_sha}"
  local bundle_path="${RELEASE_BUNDLE_PATH}"
  release_log "产物包已生成：${bundle_path}"
  release_upload_and_deploy "${bundle_path}" "${release_action}"
  release_log "发布完成：${commit_sha}"
}

trap release_cleanup EXIT

release_main "$@"
