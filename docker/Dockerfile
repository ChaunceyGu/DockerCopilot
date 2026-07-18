# syntax=docker/dockerfile:1

# ============================================================
# 阶段 1：构建前端（React + Vite）
# 在构建机本地架构上跑（--platform=$BUILDPLATFORM），不受目标架构影响，速度快
# ============================================================
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
WORKDIR /app/frontend

# 先装依赖（利用缓存层）
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund

# 再拷源码并构建，产物在 /app/frontend/dist
COPY frontend/ ./
RUN npm run build

# ============================================================
# 阶段 2：编译后端（Go），并把前端产物 embed 进二进制
# 同样在构建机本地架构上跑，通过 GOARCH 交叉编译到目标架构，避免 QEMU 模拟拖慢
# ============================================================
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS backend
WORKDIR /src

# buildx 自动注入的目标平台变量
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

# 先拉依赖（利用缓存层）
COPY go.mod go.sum ./
RUN go mod download

# 拷后端源码
COPY . .

# 用阶段 1 的前端产物替换 front/（//go:embed front/* 需要目录里有内容）
RUN rm -rf front && mkdir -p front
COPY --from=frontend /app/frontend/dist/ ./front/

# 交叉编译静态二进制
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -a --trimpath \
    -ldflags="-w -s -X 'github.com/ChaunceyGu/DockerCopilot/internal/config.Version=${VERSION}' -X 'github.com/ChaunceyGu/DockerCopilot/internal/config.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)'" \
    -o /out/dockerCopilot .

# ============================================================
# 阶段 3：运行镜像（alpine）
# ============================================================
FROM alpine
LABEL org.opencontainers.image.source="https://github.com/ChaunceyGu/DockerCopilot"
WORKDIR /app

COPY --from=backend /out/dockerCopilot ./dockerCopilot
COPY etc/. ./etc
COPY start.sh ./start.sh

RUN apk add --no-cache tzdata \
    && chmod +x start.sh dockerCopilot

ENV secretKey="" \
    DOCKER_HOST="unix:///var/run/docker.sock" \
    BACKUP_DIR="/data/backups" \
    TZ="Asia/Shanghai" \
    WORKDIR="/app"

VOLUME ["/data"]
EXPOSE 12712
CMD ["./start.sh"]
