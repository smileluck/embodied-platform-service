# SmileX-Admin-Gin 多阶段构建：前端 dist → 后端二进制 → 精简运行时
# 构建上下文需包含 web/（前端源码）、Go 源码与 configs/config.yaml

# ---- 阶段 1：构建前端 ----
FROM node:20-alpine AS web
WORKDIR /build/web
# 先装依赖以利用层缓存
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
# changelog 脚本在 prebuild 钩子里生成版本文件，随构建打入产物
RUN npm run build

# ---- 阶段 2：构建后端（CGO 保留 sqlite 驱动） ----
FROM golang:1.26-alpine AS server
WORKDIR /build
RUN apk add --no-cache build-base
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /build/web/dist ./web/dist
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/server ./cmd/server

# ---- 阶段 3：运行时 ----
FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=server /out/server ./server
COPY --from=web /build/web/dist ./web/dist
COPY configs/config.yaml ./configs/config.yaml
# 容器默认不内置种子密码：未注入 APP_SEED_ADMIN_PASSWORD 时首启随机生成并打印一次
RUN sed -i 's/^  adminPassword:.*/  adminPassword: "" # 容器部署默认随机生成，可用 APP_SEED_ADMIN_PASSWORD 注入/' configs/config.yaml

# 容器内约定：可写数据（上传/导出/sqlite）落 /app/data，日志落 /app/logs，均建议挂载持久化
ENV TZ=Asia/Shanghai
EXPOSE 8080
ENTRYPOINT ["./server", "-conf", "configs/config.yaml"]
