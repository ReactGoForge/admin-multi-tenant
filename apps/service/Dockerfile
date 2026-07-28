ARG GO_BUILD_IMAGE="golang:1.26.5-alpine"
ARG GO_RUNTIME_IMAGE="alpine:3.22"
ARG ALPINE_REPOSITORY=""
ARG GOOSE_VERSION="v3.27.1"

FROM ${GO_BUILD_IMAGE} AS goose-build

ARG GOOSE_VERSION
ARG ALPINE_REPOSITORY
ARG GO_PROXY="https://proxy.golang.org,direct"

ENV GOPROXY="${GO_PROXY}"

RUN if [ -n "${ALPINE_REPOSITORY}" ]; then \
      sed -i "s#https://dl-cdn.alpinelinux.org/alpine#${ALPINE_REPOSITORY}#g" /etc/apk/repositories; \
    fi \
  && apk add --no-cache ca-certificates git

RUN GOBIN=/out go install \
    -tags="no_clickhouse no_libsql no_mssql no_postgres no_sqlite3 no_vertica no_ydb" \
    "github.com/pressly/goose/v3/cmd/goose@${GOOSE_VERSION}"

FROM ${GO_RUNTIME_IMAGE} AS runtime-base

ARG ALPINE_REPOSITORY

RUN if [ -n "${ALPINE_REPOSITORY}" ]; then \
      sed -i "s#https://dl-cdn.alpinelinux.org/alpine#${ALPINE_REPOSITORY}#g" /etc/apk/repositories; \
    fi \
  && apk add --no-cache ca-certificates tzdata \
  && addgroup -S app \
  && adduser -S -G app app

ENV TZ="Asia/Shanghai"

WORKDIR /app

COPY --from=goose-build /out/goose /app/goose

USER app

EXPOSE 8080

FROM runtime-base AS runtime

CMD ["/releases/current/server"]
