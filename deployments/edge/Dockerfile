# Edge Server Dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /build

# 安装依赖
RUN apk add --no-cache git

# 复制go mod文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 编译Edge Server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o edge-server cmd/edge-server/main.go

# 最终镜像
FROM alpine:latest

LABEL maintainer="IGH Silkroad Team"
LABEL description="IGH Edge Server - Production Line Edge Device Service"

# 安装运行时依赖
RUN apk --no-cache add ca-certificates tzdata

# 创建非root用户
RUN addgroup -g 1000 igh && \
    adduser -D -u 1000 -G igh igh

# 创建数据目录
RUN mkdir -p /data && chown igh:igh /data

WORKDIR /app

# 从builder复制二进制文件
COPY --from=builder /build/edge-server .

# 切换到非root用户
USER igh

# 数据卷
VOLUME ["/data"]

# 暴露端口
EXPOSE 8081

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8081/health || exit 1

# 启动命令
ENTRYPOINT ["/app/edge-server"]
