AIR ?= $(shell go env GOPATH)/bin/air

.PHONY: build run dev wire tidy test web web-dev web-build docker-up docker-down docker-logs docker-rebuild clean

build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server -conf configs/config.yaml

# 一键开发：后端热加载（air :28180）+ 前端热更新（Vite :28170，/api 代理到 28180）
# 需先: go install github.com/air-verse/air@latest
dev:
	@trap 'kill 0' INT TERM EXIT; \
	$(AIR) & \
	cd web && npm run dev & \
	wait

# 重新生成依赖注入代码（需: go install github.com/google/wire/cmd/wire@latest）
wire:
	wire ./cmd/server

tidy:
	go mod tidy

test:
	go test ./...

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

clean:
	rm -rf bin data web/dist
