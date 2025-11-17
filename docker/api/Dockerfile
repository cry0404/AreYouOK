# API 服务 Dockerfile

# 构建阶段
FROM golang:1.25-alpine AS builder

# 安装构建依赖
RUN apk add --no-cache git make

# 设置工作目录
WORKDIR /app

# 复制 go.mod 和 go.sum
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建 API 服务
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/bin/api ./cmd/server

# 运行阶段
FROM alpine:latest

# 安装必要的运行时依赖（包括 procps 用于 pgrep 命令）
RUN apk --no-cache add ca-certificates tzdata procps

# 设置时区
ENV TZ=Asia/Shanghai

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/bin/api /app/api

# 创建非 root 用户
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

USER appuser

# 暴露端口
EXPOSE 8080

# 健康检查（检查进程是否存在，如果未来添加了 /health 端点可以改用 HTTP 检查）
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD pgrep -f /app/api || exit 1

# 启动服务
CMD ["/app/api"]

