APP := remotehelpdesk
MAIN := ./cmd/server
WEB_DIR := web
DIST_DIR := dist

GO ?= go
PNPM ?= pnpm
GOOS ?= $(shell $(GO) env GOOS)
GOARCH ?= $(shell $(GO) env GOARCH)
DEV_PORT ?= 8083
DEV_API_BASE_URL ?= http://127.0.0.1:$(DEV_PORT)
DEV_SERVER_URL ?= $(DEV_API_BASE_URL)/api/health
LANCEDB_VERSION ?= v0.1.2
LANCEDB_DOWNLOAD_SCRIPT ?= https://raw.githubusercontent.com/lancedb/lancedb-go/main/scripts/download-artifacts.sh
LANCEDB ?= 0

UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

ifeq ($(GOOS),windows)
	APP_EXT := .exe
else
	APP_EXT :=
endif

ifeq ($(LANCEDB),1)
	BUILD_NAME_SUFFIX := -lancedb
	BUILD_TAGS := -tags lancedb
	BUILD_CGO_ENABLED := 1
else
	BUILD_NAME_SUFFIX :=
	BUILD_TAGS :=
	BUILD_CGO_ENABLED :=
endif

BUILD_OUTPUT := $(DIST_DIR)/$(APP)$(BUILD_NAME_SUFFIX)$(APP_EXT)

ifeq ($(UNAME_M),x86_64)
	LANCEDB_ARCH := amd64
else ifeq ($(UNAME_M),amd64)
	LANCEDB_ARCH := amd64
else ifeq ($(UNAME_M),arm64)
	LANCEDB_ARCH := arm64
else ifeq ($(UNAME_M),aarch64)
	LANCEDB_ARCH := arm64
else
	LANCEDB_ARCH := unsupported
endif

ifeq ($(UNAME_S),Darwin)
	LANCEDB_PLATFORM := darwin
	LANCEDB_SYSTEM_LDFLAGS := -framework Security -framework CoreFoundation
else ifeq ($(UNAME_S),Linux)
	LANCEDB_PLATFORM := linux
	LANCEDB_SYSTEM_LDFLAGS := -lm -ldl -lpthread
else ifneq (,$(findstring MINGW,$(UNAME_S)))
	LANCEDB_PLATFORM := windows
	LANCEDB_ARCH := amd64
	LANCEDB_SYSTEM_LDFLAGS :=
else ifneq (,$(findstring MSYS,$(UNAME_S)))
	LANCEDB_PLATFORM := windows
	LANCEDB_ARCH := amd64
	LANCEDB_SYSTEM_LDFLAGS :=
else ifneq (,$(findstring CYGWIN,$(UNAME_S)))
	LANCEDB_PLATFORM := windows
	LANCEDB_ARCH := amd64
	LANCEDB_SYSTEM_LDFLAGS :=
else
	LANCEDB_PLATFORM := unsupported
	LANCEDB_SYSTEM_LDFLAGS :=
endif

LANCEDB_PLATFORM_ARCH := $(LANCEDB_PLATFORM)_$(LANCEDB_ARCH)
LANCEDB_NATIVE_LIB := $(CURDIR)/lib/$(LANCEDB_PLATFORM_ARCH)/liblancedb_go.a
LANCEDB_CGO_CFLAGS := -I$(CURDIR)/include
LANCEDB_CGO_LDFLAGS := $(LANCEDB_NATIVE_LIB) $(LANCEDB_SYSTEM_LDFLAGS)

.DEFAULT_GOAL := build

.PHONY: help dev build release generator enums \
	_web-build-spa _web-dev _prepare-dist _lancedb-artifacts _lancedb-check _lancedb-release-check

help:
	@echo "Available targets:"
	@echo "  make dev        Start backend and frontend development servers"
	@echo "  make build      Build the current system into dist/"
	@echo "  make build LANCEDB=1"
	@echo "                  Build the current-platform LanceDB binary into dist/"
	@echo "  make release    Build linux/darwin/windows release binaries into dist/"
	@echo "  make release LANCEDB=1"
	@echo "                  Build LanceDB release binaries into dist/"
	@echo "  make generator  Run code generation"
	@echo "  make enums      Generate frontend enums"
	@echo "  make help       Show this help"

dev: _lancedb-check
	@RHD_SERVER_PORT="$(DEV_PORT)" CGO_ENABLED=1 CGO_CFLAGS="$(LANCEDB_CGO_CFLAGS)" CGO_LDFLAGS="$(LANCEDB_CGO_LDFLAGS)" \
		$(GO) run -tags "dev lancedb" $(MAIN) & \
	server_pid=$$!; \
	trap 'kill $$server_pid 2>/dev/null || true' EXIT INT TERM; \
	echo "Waiting for server at $(DEV_SERVER_URL)..."; \
	until curl -fsS "$(DEV_SERVER_URL)" >/dev/null 2>&1; do \
		if ! kill -0 $$server_pid 2>/dev/null; then \
			wait $$server_pid; \
			exit $$?; \
		fi; \
		sleep 1; \
	done; \
	echo "Server is ready; starting web dev server..."; \
	$(MAKE) _web-dev

ifeq ($(LANCEDB),1)
build: _lancedb-check _prepare-dist _web-build-spa
else
build: _prepare-dist _web-build-spa
endif
	@echo "Building $(BUILD_OUTPUT)..."
ifeq ($(LANCEDB),1)
	@CGO_ENABLED=$(BUILD_CGO_ENABLED) CGO_CFLAGS="$(LANCEDB_CGO_CFLAGS)" CGO_LDFLAGS="$(LANCEDB_CGO_LDFLAGS)" \
		$(GO) build $(BUILD_TAGS) -v -o $(BUILD_OUTPUT) $(MAIN)
else
	@$(GO) build -v -o $(BUILD_OUTPUT) $(MAIN)
endif

ifeq ($(LANCEDB),1)
release: _lancedb-release-check _prepare-dist _web-build-spa
else
release: _prepare-dist _web-build-spa
endif
	@echo "Building release binaries in $(DIST_DIR)..."
	@if [ "$(LANCEDB)" = "1" ]; then \
		set -e; \
		if [ ! -f "$(CURDIR)/include/lancedb.h" ]; then \
			echo "Missing LanceDB header: $(CURDIR)/include/lancedb.h"; \
			exit 1; \
		fi; \
		build_lancedb() { \
			platform="$$1"; \
			arch="$$2"; \
			ext="$$3"; \
			system_ldflags="$$4"; \
			native_lib="$(CURDIR)/lib/$${platform}_$${arch}/liblancedb_go.a"; \
			output="$(DIST_DIR)/$(APP)-lancedb-$${platform}-$${arch}$${ext}"; \
			if [ ! -f "$$native_lib" ]; then \
				echo "Missing LanceDB native library for $${platform}/$${arch}: $$native_lib"; \
				echo "LanceDB release builds require matching native artifacts and a CGO-capable toolchain for each target platform."; \
				exit 1; \
			fi; \
			echo "Building $$output..."; \
			CGO_ENABLED=1 CGO_CFLAGS="-I$(CURDIR)/include" CGO_LDFLAGS="$$native_lib $$system_ldflags" \
				GOOS="$$platform" GOARCH="$$arch" $(GO) build -tags lancedb -v -o "$$output" $(MAIN); \
		}; \
		build_lancedb linux amd64 "" "-lm -ldl -lpthread"; \
		build_lancedb linux arm64 "" "-lm -ldl -lpthread"; \
		build_lancedb darwin amd64 "" "-framework Security -framework CoreFoundation"; \
		build_lancedb darwin arm64 "" "-framework Security -framework CoreFoundation"; \
		build_lancedb windows amd64 ".exe" ""; \
	else \
		GOOS=linux GOARCH=amd64 $(GO) build -v -o $(DIST_DIR)/$(APP)-linux-amd64 $(MAIN); \
		GOOS=linux GOARCH=arm64 $(GO) build -v -o $(DIST_DIR)/$(APP)-linux-arm64 $(MAIN); \
		GOOS=darwin GOARCH=amd64 $(GO) build -v -o $(DIST_DIR)/$(APP)-darwin-amd64 $(MAIN); \
		GOOS=darwin GOARCH=arm64 $(GO) build -v -o $(DIST_DIR)/$(APP)-darwin-arm64 $(MAIN); \
		GOOS=windows GOARCH=amd64 $(GO) build -v -o $(DIST_DIR)/$(APP)-windows-amd64.exe $(MAIN); \
	fi

generator:
	@$(GO) run ./cmd/generator/generator.go

enums:
	@$(GO) run ./cmd/enums/generator.go

# 单二进制内嵌静态包：使用静态导出构建（NEXT_STATIC_EXPORT=1）生成 web/out。
# SaaS 管理后台动态路由构建使用默认的 `pnpm build`（见 web/next.config.ts）。
_web-build-spa:
	@cd $(WEB_DIR) && $(PNPM) build:sdk && $(PNPM) build:static

_web-dev:
	@cd $(WEB_DIR) && NEXT_PUBLIC_API_BASE_URL="" NEXT_API_BASE_URL="$(DEV_API_BASE_URL)" $(PNPM) dev

_prepare-dist:
	@mkdir -p $(DIST_DIR)

_lancedb-artifacts:
	@if [ "$(LANCEDB_PLATFORM)" = "unsupported" ] || [ "$(LANCEDB_ARCH)" = "unsupported" ]; then \
		echo "Unsupported LanceDB platform: $(UNAME_S)/$(UNAME_M)"; \
		exit 1; \
	fi
	@if [ -f "$(LANCEDB_NATIVE_LIB)" ] && [ -f "$(CURDIR)/include/lancedb.h" ]; then \
		echo "LanceDB native artifacts already exist for $(LANCEDB_PLATFORM_ARCH)."; \
	else \
		echo "Downloading LanceDB native artifacts $(LANCEDB_VERSION) for $(LANCEDB_PLATFORM_ARCH)..."; \
		curl -sSL "$(LANCEDB_DOWNLOAD_SCRIPT)" | bash -s "$(LANCEDB_VERSION)"; \
	fi

_lancedb-check: _lancedb-artifacts
	@if [ ! -f "$(LANCEDB_NATIVE_LIB)" ]; then \
		echo "Missing LanceDB native library: $(LANCEDB_NATIVE_LIB)"; \
		exit 1; \
	fi
	@if [ ! -f "$(CURDIR)/include/lancedb.h" ]; then \
		echo "Missing LanceDB header: $(CURDIR)/include/lancedb.h"; \
		exit 1; \
	fi

_lancedb-release-check:
	@if [ ! -f "$(CURDIR)/include/lancedb.h" ]; then \
		echo "Missing LanceDB header: $(CURDIR)/include/lancedb.h"; \
		exit 1; \
	fi
	@missing=0; \
	for target in linux_amd64 linux_arm64 darwin_amd64 darwin_arm64 windows_amd64; do \
		native_lib="$(CURDIR)/lib/$${target}/liblancedb_go.a"; \
		if [ ! -f "$$native_lib" ]; then \
			echo "Missing LanceDB native library: $$native_lib"; \
			missing=1; \
		fi; \
	done; \
	if [ "$$missing" = "1" ]; then \
		echo "LanceDB release builds require matching native artifacts and a CGO-capable toolchain for each target platform."; \
		exit 1; \
	fi

# 数据库备份
.PHONY: backup-db
backup-db:
	./scripts/backup-db.sh

# 数据库恢复
.PHONY: restore-db
restore-db:
	./scripts/restore-db.sh $(FILE)

# 健康检查
.PHONY: health
health:
	curl -s http://localhost:8083/api/health | jq

# 就绪检查
.PHONY: ready
ready:
	curl -s http://localhost:8083/api/health/ready | jq

# =============================================================================
# CI / Code Quality Targets
# =============================================================================

# 代码检查（Go + 前端）
.PHONY: lint
lint:
	@echo "Running Go linter..."
	golangci-lint run ./...
	@echo "Running frontend linter..."
	cd web && pnpm lint

# 测试 + 覆盖率报告
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
	@echo "Total coverage:"
	go tool cover -func=coverage.out | tail -1

# 生成 API 文档（Swagger/OpenAPI）
.PHONY: api-docs
api-docs:
	@echo "Generating API documentation..."
	swag init -g cmd/server/main.go -o docs/api
	@echo "API docs generated in docs/api/"

# =============================================================================
# Local Development / Deployment Targets
# =============================================================================

# 全量部署（开发环境）
.PHONY: dev-up
dev-up:
	@echo "Starting development environment..."
	docker compose up -d
	@echo "Waiting for services..."
	@until curl -s http://localhost:8083/api/health > /dev/null; do sleep 2; done
	@echo "All services ready!"

# 停止开发环境
.PHONY: dev-down
dev-down:
	docker compose down

# 重启开发环境
.PHONY: dev-restart
dev-restart: dev-down dev-up

# 查看开发环境日志
.PHONY: dev-logs
dev-logs:
	docker compose logs -f

# =============================================================================
# Production Targets
# =============================================================================

# 启动生产环境
.PHONY: prod-up
prod-up:
	@echo "Starting production environment..."
	docker compose -f docker-compose.prod.yml up -d
	@echo "Waiting for services..."
	@for i in $$(seq 1 30); do \
		if curl -sf http://localhost:8083/api/health > /dev/null; then \
			echo "All services ready!"; \
			exit 0; \
		fi; \
		echo "Waiting... $$i"; \
		sleep 2; \
	done; \
	echo "Timeout waiting for services"; \
	exit 1

# 停止生产环境
.PHONY: prod-down
prod-down:
	docker compose -f docker-compose.prod.yml down

# 拉起最新镜像并重启生产环境
.PHONY: prod-pull
prod-pull:
	docker compose -f docker-compose.prod.yml pull
	docker compose -f docker-compose.prod.yml up -d

# 构建生产 Docker 镜像
.PHONY: docker-build
docker-build:
	docker build -t remotehelpdesk:latest -f Dockerfile.prod .

# =============================================================================
# Database Targets
# =============================================================================

# 数据库备份
.PHONY: backup-db
backup-db:
	./scripts/backup-db.sh

# 数据库恢复
.PHONY: restore-db
restore-db:
	./scripts/restore-db.sh $(FILE)

# 单租户→多租户迁移
.PHONY: migrate-multi-tenant
migrate-multi-tenant:
	./scripts/migrate-multi-tenant.sh --dry-run
	@echo ""
	@echo "Run without --dry-run to execute the migration."

# =============================================================================
# Utility Targets
# =============================================================================

# 显示 Docker 容器状态
.PHONY: ps
ps:
	docker compose ps

# 清理未使用的 Docker 资源
.PHONY: docker-clean
docker-clean:
	docker system prune -f

# 显示所有可用目标帮助
.PHONY: help-ci
help-ci:
	@echo "CI / Quality targets:"
	@echo "  make lint             Run Go + frontend linters"
	@echo "  make test-coverage    Run tests and generate coverage report"
	@echo "  make api-docs         Generate Swagger API documentation"
	@echo ""
	@echo "Development targets:"
	@echo "  make dev-up           Start full dev environment (Docker Compose)"
	@echo "  make dev-down         Stop dev environment"
	@echo "  make dev-restart      Restart dev environment"
	@echo "  make dev-logs         Tail dev logs"
	@echo ""
	@echo "Production targets:"
	@echo "  make prod-up          Start production environment"
	@echo "  make prod-down        Stop production environment"
	@echo "  make prod-pull        Pull latest images and restart"
	@echo "  make docker-build     Build production Docker image"
	@echo ""
	@echo "Database targets:"
	@echo "  make backup-db        Backup PostgreSQL database"
	@echo "  make restore-db       Restore PostgreSQL database (FILE=path)"
	@echo "  make migrate-multi-tenant   Run multi-tenant migration (dry-run first)"
