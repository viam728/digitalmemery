# ============================================================
# JasperLee · 数字分身 —— 多阶段构建（BuildKit）
# 目标：尽量小的运行时镜像；后端 Go 零外部依赖，可在离线阶段编译。
# ============================================================

# ---- 阶段 1：构建前端（React + Vite）----
FROM node:20-alpine AS web
WORKDIR /app/frontend

# 先 COPY 清单（利用层缓存；重复构建不改清单时 npm ci 不重跑）
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund

# 再 COPY 源码，构建静态产物到 /app/frontend/dist
COPY frontend/ ./
RUN npm run build

# ---- 阶段 2：构建后端 Go 二进制 ----
FROM golang:1.26 AS go-build
WORKDIR /app/backend

# COPY backend 源码（.dockerignore 已排除 backend/.env 与 *.exe）
COPY backend/ ./

# CGO_ENABLED=0 -> 静态链接，可直接跑在 alpine 上
# 代码零外部依赖，go build 不需要 go get / 联网拉模块
RUN CGO_ENABLED=0 go build -o /out/jasperlee .

# ---- 阶段 3：运行时镜像（alpine，尽量小）----
FROM alpine:3.20

# ca-certificates：后端要 HTTPS 调用 GLM / 阿里云，容器内需 CA 证书验证
RUN apk add --no-cache ca-certificates

WORKDIR /app

# 可执行文件放 /app/backend/jasperlee
COPY --from=go-build /out/jasperlee /app/backend/jasperlee

# 前端构建产物放 /app/frontend/dist
# main.go 的 serveStatic 按 exe 目录解析: exeDir=../frontend/dist
#   /app/backend/jasperlee -> /app/frontend/dist
COPY --from=web /app/frontend/dist /app/frontend/dist

EXPOSE 8080

CMD ["/app/backend/jasperlee"]