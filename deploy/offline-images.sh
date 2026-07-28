#!/usr/bin/env bash

set -Eeuo pipefail

umask 077

OFFLINE_SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
OFFLINE_REPOSITORY_DIR="$(cd -- "${OFFLINE_SCRIPT_DIR}/.." && pwd)"
OFFLINE_DEFAULT_OUTPUT_DIR="${OFFLINE_SCRIPT_DIR}/dist"
OFFLINE_MYSQL_IMAGE="mysql:8.4.10"
OFFLINE_REDIS_IMAGE="redis:7.2.14-alpine"
OFFLINE_MINIO_IMAGE="quay.io/minio/minio:RELEASE.2025-09-07T16-13-09Z"
OFFLINE_SERVICE_RUNTIME_IMAGE="admin-multi-tenant-service-runtime:1"
OFFLINE_DOCKER_MIRROR="${ADMIN_MULTI_TENANT_DOCKER_MIRROR:-m.daocloud.io/docker.io}"
OFFLINE_QUAY_MIRROR="${ADMIN_MULTI_TENANT_QUAY_MIRROR:-m.daocloud.io/quay.io}"
OFFLINE_ALPINE_REPOSITORY="${ADMIN_MULTI_TENANT_ALPINE_REPOSITORY:-https://mirrors.aliyun.com/alpine}"
OFFLINE_GO_PROXY="${ADMIN_MULTI_TENANT_GO_PROXY:-https://goproxy.cn,direct}"
OFFLINE_TEMP_DIR=""

offline_log() {
  printf '[admin-multi-tenant-offline] %s\n' "$*"
}

offline_die() {
  printf '[admin-multi-tenant-offline] 错误：%s\n' "$*" >&2
  exit 1
}

offline_usage() {
  cat <<'EOF'
用法：
  ./deploy/offline-images.sh export <runtime|infra|all> [linux/amd64|linux/arm64] [输出目录]
  ./deploy/offline-images.sh load <镜像包.tar.gz>

说明：
  export  在可联网的本机按目标服务器架构构建指定组件：
          runtime 仅包含固定 Go 运行环境；
          infra 包含 MySQL、Redis、MinIO；
          all 包含固定 Go 运行环境和全部基础设施镜像。
          输出离线镜像包及 SHA-256 校验文件，不修改本机已有基础设施镜像标签。
  load    在服务器校验镜像包、CPU 架构和 Commit SHA 后导入包内声明的镜像。
EOF
}

offline_cleanup() {
  if [[ -n "${OFFLINE_TEMP_DIR}" && -d "${OFFLINE_TEMP_DIR}" ]]; then
    rm -rf -- "${OFFLINE_TEMP_DIR}"
  fi
}

offline_require_command() {
  local command_name="$1"
  command -v "${command_name}" >/dev/null 2>&1 \
    || offline_die "缺少命令 ${command_name}"
}

offline_validate_platform() {
  local platform="$1"
  case "${platform}" in
    linux/amd64|linux/arm64)
      ;;
    *)
      offline_die "目标架构只允许 linux/amd64 或 linux/arm64"
      ;;
  esac
}

offline_validate_bundle_type() {
  local bundle_type="$1"
  case "${bundle_type}" in
    runtime|infra|all)
      ;;
    *)
      offline_die "包类型只允许 runtime、infra 或 all"
      ;;
  esac
}

offline_bundle_components() {
  local bundle_type="$1"
  case "${bundle_type}" in
    runtime)
      printf 'runtime\n'
      ;;
    infra)
      printf 'mysql redis minio\n'
      ;;
    all)
      printf 'runtime mysql redis minio\n'
      ;;
  esac
}

offline_has_component() {
  local components="$1"
  local component="$2"
  [[ " ${components} " == *" ${component} "* ]]
}

offline_validate_sha() {
  local commit_sha="$1"
  [[ "${commit_sha}" =~ ^[0-9a-f]{40}$ ]] \
    || offline_die "Commit SHA 必须是 40 位小写十六进制"
}

offline_sha256() {
  local file_path="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${file_path}" | awk '{print $1}'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "${file_path}" | awk '{print $1}'
    return
  fi
  offline_die "缺少 sha256sum 或 shasum"
}

offline_manifest_value() {
  local manifest_file="$1"
  local key="$2"
  local value
  value="$(grep -E "^${key}=" "${manifest_file}" | head -n 1 | cut -d= -f2-)" \
    || true
  [[ -n "${value}" ]] || offline_die "镜像包清单缺少 ${key}"
  printf '%s\n' "${value}"
}

offline_build_base_archive() {
  local platform="$1"
  local source_image="$2"
  local archive_image="$3"
  local archive_path="$4"
  local dockerfile_path="$5"
  local empty_context="$6"

  printf 'FROM %s\n' "${source_image}" >"${dockerfile_path}"
  docker buildx build \
    --pull \
    --platform "${platform}" \
    --file "${dockerfile_path}" \
    --tag "${archive_image}" \
    --load \
    "${empty_context}"
  docker image save --output "${archive_path}" "${archive_image}"
}

offline_export() {
  local bundle_type="${1:-}"
  local platform="${2:-linux/amd64}"
  local output_dir="${3:-${OFFLINE_DEFAULT_OUTPUT_DIR}}"
  [[ -n "${bundle_type}" ]] \
    || offline_die "export 需要包类型：runtime、infra 或 all"
  offline_validate_bundle_type "${bundle_type}"
  offline_validate_platform "${platform}"

  offline_require_command docker
  offline_require_command git
  offline_require_command tar
  docker buildx version >/dev/null 2>&1 \
    || offline_die "Docker Buildx 不可用"

  [[ -z "$(git -C "${OFFLINE_REPOSITORY_DIR}" status --porcelain)" ]] \
    || offline_die "工作区存在未提交变更，请先提交并推送同一 Commit"

  local commit_sha
  commit_sha="$(git -C "${OFFLINE_REPOSITORY_DIR}" rev-parse HEAD)"
  offline_validate_sha "${commit_sha}"

  mkdir -p "${output_dir}"
  output_dir="$(cd -- "${output_dir}" && pwd)"
  OFFLINE_TEMP_DIR="$(mktemp -d)"
  local images_dir="${OFFLINE_TEMP_DIR}/images"
  local empty_context="${OFFLINE_TEMP_DIR}/empty-context"
  mkdir -p "${images_dir}" "${empty_context}"

  local platform_name="${platform/\//-}"
  local bundle_name="admin-multi-tenant-${bundle_type}-images-${commit_sha:0:12}-${platform_name}.tar.gz"
  local bundle_path="${output_dir}/${bundle_name}"
  local temporary_bundle="${bundle_path}.tmp"
  local components
  components="$(offline_bundle_components "${bundle_type}")"
  local format_version="3"
  local mysql_archive_image="admin-multi-tenant-offline/mysql:8.4.10-${platform_name}-${commit_sha:0:12}"
  local redis_archive_image="admin-multi-tenant-offline/redis:7.2.14-${platform_name}-${commit_sha:0:12}"
  local minio_archive_image="admin-multi-tenant-offline/minio:2025-09-07-${platform_name}-${commit_sha:0:12}"
  local mysql_source_image="${OFFLINE_DOCKER_MIRROR}/library/mysql:8.4.10"
  local redis_source_image="${OFFLINE_DOCKER_MIRROR}/library/redis:7.2.14-alpine"
  local minio_source_image="${OFFLINE_QUAY_MIRROR}/minio/minio:RELEASE.2025-09-07T16-13-09Z"

  if offline_has_component "${components}" runtime; then
    offline_log "构建固定 Go 运行镜像（${platform}）"
    docker buildx build \
      --pull \
      --platform "${platform}" \
      --target runtime \
      --file "${OFFLINE_REPOSITORY_DIR}/apps/service/Dockerfile" \
      --build-arg "GO_BUILD_IMAGE=${OFFLINE_DOCKER_MIRROR}/library/golang:1.26.5-alpine" \
      --build-arg "GO_RUNTIME_IMAGE=${OFFLINE_DOCKER_MIRROR}/library/alpine:3.22" \
      --build-arg "ALPINE_REPOSITORY=${OFFLINE_ALPINE_REPOSITORY}" \
      --build-arg "GO_PROXY=${OFFLINE_GO_PROXY}" \
      --tag "${OFFLINE_SERVICE_RUNTIME_IMAGE}" \
      --load \
      "${OFFLINE_REPOSITORY_DIR}/apps/service"
    docker image save --output "${images_dir}/runtime.tar" \
      "${OFFLINE_SERVICE_RUNTIME_IMAGE}"
  fi

  if offline_has_component "${components}" mysql; then
    offline_log "下载并封装 MySQL、Redis、MinIO 镜像（${platform}）"
    offline_build_base_archive "${platform}" "${mysql_source_image}" "${mysql_archive_image}" \
      "${images_dir}/mysql.tar" "${OFFLINE_TEMP_DIR}/mysql.Dockerfile" "${empty_context}"
    offline_build_base_archive "${platform}" "${redis_source_image}" "${redis_archive_image}" \
      "${images_dir}/redis.tar" "${OFFLINE_TEMP_DIR}/redis.Dockerfile" "${empty_context}"
    offline_build_base_archive "${platform}" "${minio_source_image}" "${minio_archive_image}" \
      "${images_dir}/minio.tar" "${OFFLINE_TEMP_DIR}/minio.Dockerfile" "${empty_context}"
  fi

  {
    printf 'FORMAT_VERSION=%s\n' "${format_version}"
    printf 'BUNDLE_TYPE=%s\n' "${bundle_type}"
    printf 'COMPONENTS=%s\n' "${components}"
    printf 'COMMIT_SHA=%s\n' "${commit_sha}"
    printf 'TARGET_PLATFORM=%s\n' "${platform}"
    printf 'MYSQL_IMAGE=%s\n' "${OFFLINE_MYSQL_IMAGE}"
    printf 'REDIS_IMAGE=%s\n' "${OFFLINE_REDIS_IMAGE}"
    printf 'MINIO_IMAGE=%s\n' "${OFFLINE_MINIO_IMAGE}"
    printf 'SERVICE_RUNTIME_IMAGE=%s\n' "${OFFLINE_SERVICE_RUNTIME_IMAGE}"
    printf 'MYSQL_ARCHIVE_IMAGE=%s\n' "${mysql_archive_image}"
    printf 'REDIS_ARCHIVE_IMAGE=%s\n' "${redis_archive_image}"
    printf 'MINIO_ARCHIVE_IMAGE=%s\n' "${minio_archive_image}"
  } >"${OFFLINE_TEMP_DIR}/manifest.env"

  tar -czf "${temporary_bundle}" \
    -C "${OFFLINE_TEMP_DIR}" manifest.env images
  mv "${temporary_bundle}" "${bundle_path}"

  local checksum
  checksum="$(offline_sha256 "${bundle_path}")"
  printf '%s  %s\n' "${checksum}" "${bundle_name}" >"${bundle_path}.sha256"

  offline_log "离线镜像包已生成：${bundle_path}"
  offline_log "校验文件已生成：${bundle_path}.sha256"
  offline_log "包类型：${bundle_type}（${components}）"
  offline_log "Commit SHA：${commit_sha}"
}

offline_server_platform() {
  case "$(uname -m)" in
    x86_64|amd64)
      printf 'linux/amd64\n'
      ;;
    aarch64|arm64)
      printf 'linux/arm64\n'
      ;;
    *)
      offline_die "不支持的服务器 CPU 架构：$(uname -m)"
      ;;
  esac
}

offline_verify_loaded_image() {
  local image_name="$1"
  local expected_platform="$2"
  local image_platform
  image_platform="$(docker image inspect \
    --format '{{.Os}}/{{.Architecture}}' "${image_name}")" \
    || offline_die "镜像导入后不存在：${image_name}"
  [[ "${image_platform}" == "${expected_platform}" ]] \
    || offline_die "镜像 ${image_name} 架构为 ${image_platform}，期望 ${expected_platform}"
}

offline_load() {
  local bundle_path="${1:-}"
  [[ -n "${bundle_path}" ]] || offline_die "load 需要镜像包路径"
  [[ -f "${bundle_path}" ]] || offline_die "镜像包不存在：${bundle_path}"
  [[ -f "${bundle_path}.sha256" ]] \
    || offline_die "缺少校验文件：${bundle_path}.sha256"

  offline_require_command docker
  offline_require_command tar

  local expected_checksum
  local actual_checksum
  expected_checksum="$(awk 'NR == 1 {print $1}' "${bundle_path}.sha256")"
  [[ "${expected_checksum}" =~ ^[0-9a-f]{64}$ ]] \
    || offline_die "校验文件中的 SHA-256 格式无效"
  actual_checksum="$(offline_sha256 "${bundle_path}")"
  [[ "${actual_checksum}" == "${expected_checksum}" ]] \
    || offline_die "离线镜像包 SHA-256 校验失败"

  OFFLINE_TEMP_DIR="$(mktemp -d)"
  local archive_listing="${OFFLINE_TEMP_DIR}/archive-list.txt"
  tar -tzf "${bundle_path}" >"${archive_listing}"
  if grep -Eq '(^/|(^|/)\.\.(/|$))' "${archive_listing}"; then
    offline_die "镜像包包含不安全路径"
  fi
  tar -xzf "${bundle_path}" -C "${OFFLINE_TEMP_DIR}"
  local manifest_file="${OFFLINE_TEMP_DIR}/manifest.env"
  [[ -f "${manifest_file}" ]] || offline_die "镜像包缺少 manifest.env"

  local format_version
  local bundle_type
  local components
  local commit_sha
  local target_platform
  format_version="$(offline_manifest_value "${manifest_file}" FORMAT_VERSION)"
  case "${format_version}" in
    3)
      bundle_type="$(offline_manifest_value "${manifest_file}" BUNDLE_TYPE)"
      components="$(offline_manifest_value "${manifest_file}" COMPONENTS)"
      offline_validate_bundle_type "${bundle_type}"
      [[ "${components}" == "$(offline_bundle_components "${bundle_type}")" ]] \
        || offline_die "镜像包组件列表与包类型不一致"
      ;;
    *)
      offline_die "不支持的镜像包格式版本"
      ;;
  esac
  commit_sha="$(offline_manifest_value "${manifest_file}" COMMIT_SHA)"
  target_platform="$(offline_manifest_value "${manifest_file}" TARGET_PLATFORM)"
  offline_validate_sha "${commit_sha}"
  offline_validate_platform "${target_platform}"
  [[ "${target_platform}" == "$(offline_server_platform)" ]] \
    || offline_die "镜像包架构 ${target_platform} 与服务器 $(offline_server_platform) 不一致"

  local mysql_image
  local redis_image
  local minio_image
  local runtime_image
  local mysql_archive_image
  local redis_archive_image
  local minio_archive_image
  mysql_image="$(offline_manifest_value "${manifest_file}" MYSQL_IMAGE)"
  redis_image="$(offline_manifest_value "${manifest_file}" REDIS_IMAGE)"
  minio_image="$(offline_manifest_value "${manifest_file}" MINIO_IMAGE)"
  runtime_image="$(offline_manifest_value "${manifest_file}" SERVICE_RUNTIME_IMAGE)"
  mysql_archive_image="$(offline_manifest_value "${manifest_file}" MYSQL_ARCHIVE_IMAGE)"
  redis_archive_image="$(offline_manifest_value "${manifest_file}" REDIS_ARCHIVE_IMAGE)"
  minio_archive_image="$(offline_manifest_value "${manifest_file}" MINIO_ARCHIVE_IMAGE)"

  [[ "${mysql_image}" == "${OFFLINE_MYSQL_IMAGE}" ]] \
    || offline_die "镜像包 MySQL 版本不符合部署配置"
  [[ "${redis_image}" == "${OFFLINE_REDIS_IMAGE}" ]] \
    || offline_die "镜像包 Redis 版本不符合部署配置"
  [[ "${minio_image}" == "${OFFLINE_MINIO_IMAGE}" ]] \
    || offline_die "镜像包 MinIO 版本不符合部署配置"
  [[ "${runtime_image}" == "${OFFLINE_SERVICE_RUNTIME_IMAGE}" ]] \
    || offline_die "镜像包 Go 运行镜像版本不符合部署配置"
  local platform_name="${target_platform/\//-}"
  [[ "${mysql_archive_image}" == "admin-multi-tenant-offline/mysql:8.4.10-${platform_name}-${commit_sha:0:12}" ]] \
    || offline_die "镜像包 MySQL 归档标签无效"
  [[ "${redis_archive_image}" == "admin-multi-tenant-offline/redis:7.2.14-${platform_name}-${commit_sha:0:12}" ]] \
    || offline_die "镜像包 Redis 归档标签无效"
  [[ "${minio_archive_image}" == "admin-multi-tenant-offline/minio:2025-09-07-${platform_name}-${commit_sha:0:12}" ]] \
    || offline_die "镜像包 MinIO 归档标签无效"
  local component
  for component in ${components}; do
    [[ -f "${OFFLINE_TEMP_DIR}/images/${component}.tar" ]] \
      || offline_die "镜像包缺少 images/${component}.tar"
    docker load --input "${OFFLINE_TEMP_DIR}/images/${component}.tar"
  done

  if offline_has_component "${components}" mysql; then
    offline_verify_loaded_image "${mysql_archive_image}" "${target_platform}"
    offline_verify_loaded_image "${redis_archive_image}" "${target_platform}"
    offline_verify_loaded_image "${minio_archive_image}" "${target_platform}"
    docker image tag "${mysql_archive_image}" "${mysql_image}"
    docker image tag "${redis_archive_image}" "${redis_image}"
    docker image tag "${minio_archive_image}" "${minio_image}"
    offline_verify_loaded_image "${mysql_image}" "${target_platform}"
    offline_verify_loaded_image "${redis_image}" "${target_platform}"
    offline_verify_loaded_image "${minio_image}" "${target_platform}"
  fi
  if offline_has_component "${components}" runtime; then
    offline_verify_loaded_image "${runtime_image}" "${target_platform}"
  fi

  offline_log "${bundle_type} 镜像包导入并校验完成：${components}"
  offline_log "Commit SHA：${commit_sha}"
  offline_log "基础设施镜像加载后可执行 ./deploy/artifact-deploy.sh infra up"
}

offline_main() {
  local command_name="${1:-}"
  shift || true

  case "${command_name}" in
    export)
      (($# >= 1 && $# <= 3)) \
        || offline_die "export 需要包类型，可选架构和输出目录"
      offline_export "$1" "${2:-linux/amd64}" "${3:-${OFFLINE_DEFAULT_OUTPUT_DIR}}"
      ;;
    load)
      (($# == 1)) || offline_die "load 需要一个镜像包路径"
      offline_load "$1"
      ;;
    -h|--help|help|"")
      offline_usage
      ;;
    *)
      offline_usage
      offline_die "未知命令：${command_name}"
      ;;
  esac
}

trap offline_cleanup EXIT

offline_main "$@"
