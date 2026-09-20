# Windows 兼容：GNU make 在 Windows 下默认可能选到 sh.exe（Git Bash 在 PATH 时），
# 强制 cmd.exe 保证 CMD / PowerShell / Git Bash 三种终端行为一致
ifeq ($(OS),Windows_NT)
SHELL := cmd.exe
BIN := bin/server.exe
AIR ?= $(shell go env GOPATH)/bin/air.exe
AIR_CONF := .air.windows.toml
else
BIN := bin/server
AIR ?= $(shell go env GOPATH)/bin/air
AIR_CONF := .air.toml
endif

.PHONY: build run dev wire tidy test lint web-dev web-build web-install docker-up docker-down docker-logs docker-rebuild clean

build:
	go build -o $(BIN) ./cmd/server

run:
	go run ./cmd/server -conf configs/config.yaml

# 一键开发：后端热加载（air :28180）+ 前端热更新（Vite :28170，/api 代理到 28180）
# 需先: go install github.com/air-verse/air@latest
ifeq ($(OS),Windows_NT)
dev:
	@start /b "" "$(AIR)" -c $(AIR_CONF)
	@cd web && npm run dev
else
dev:
	@trap 'kill 0' INT TERM EXIT; \
	$(AIR) -c $(AIR_CONF) & \
	cd web && npm run dev & \
	wait
endif

# 重新生成依赖注入代码（需: go install github.com/google/wire/cmd/wire@latest）
wire:
	wire ./cmd/server

tidy:
	go mod tidy

test:
	go test ./...

# 后端静态检查（vet 始终执行；golangci-lint 已安装时追加深度检查）
lint:
	go vet ./...
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "[lint] golangci-lint 未安装，仅执行 go vet"

# 前端开发（Vite 热更新，/api 代理到 localhost:28180）
web-dev:
	cd web && npm run dev

# 前端构建（产物 web/dist，由后端静态托管）
web-build:
	cd web && npm run build

web-install:
	cd web && npm install

# ---- Docker 部署（app + MySQL + Redis，见 docker-compose.yml）----
docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f app

docker-rebuild:
	docker compose build --no-cache app

ifeq ($(OS),Windows_NT)
clean:
	if exist bin rmdir /s /q bin
	if exist data rmdir /s /q data
	if exist web/dist rmdir /s /q web/dist
else
clean:
	rm -rf bin data web/dist
endif
