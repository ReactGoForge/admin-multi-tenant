GOOSE_VERSION := v3.27.1
GOOSE := go run github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION)
MIGRATION_DIR := migrations
SERVICE_ENV := .env

.PHONY: run migrate-create

run:
	@test -f "$(SERVICE_ENV)" || { echo "缺少 $(SERVICE_ENV)，请先根据 apps/service/.env.example 创建本地配置。" >&2; exit 1; }
	@set -a; \
	. "$(SERVICE_ENV)"; \
	set +a; \
	go run ./cmd/server

migrate-create:
	@case "$${name:-}" in \
		''|[!a-z]*|*[!a-z0-9_]*) \
			echo "用法：make migrate-create name=create_xxx（name 必须为 snake_case 且以小写字母开头）" >&2; \
			exit 1; \
			;; \
	esac
	@$(GOOSE) -env=none -s -dir "$(MIGRATION_DIR)" create "$$name" sql
