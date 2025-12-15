# Worker 服务 Dockerfile

# docker file 的构建总流程， cli 客户端会将 dockerfile 和上下文文件，通常是当前目录发送给 docker 的守护进程 daemon

# 构建阶段
FROM docker.1ms.run/golang:1.25-alpine AS builder

# 使用阿里云镜像源加速
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

# 安装构建依赖
RUN apk add --no-cache git make

# 设置工作目录
WORKDIR /app

# 设置 Go 代理加速
ENV GOPROXY=https://goproxy.cn,direct

# 复制 go.mod 和 go.sum
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建 Worker 服务
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/bin/worker ./cmd/worker

# 运行阶段
FROM docker.1ms.run/alpine:latest

# 使用阿里云镜像源加速
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

# 安装必要的运行时依赖（包括 procps 用于 pgrep 命令）
RUN apk --no-cache add ca-certificates tzdata procps

# 设置时区
ENV TZ=Asia/Shanghai

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/bin/worker /app/worker

# 创建非 root 用户
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

USER appuser


# 健康检查（通过检查进程是否存在）
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD pgrep -f /app/worker || exit 1

# 启动服务， CMD 是容器启动的入口程序，是 docker run 的默认部分，与之相对的是 entrypoint 标志，它可以设置环境变量
CMD ["/app/worker"] 

