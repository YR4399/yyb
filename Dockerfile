# 第一阶段：使用 Go 运行时编译程序
FROM golang:1.23-alpine AS builder

# 安装 git（拉取模块用）
RUN apk add --no-cache git ca-certificates

# Go 模块代理：默认 proxy.golang.org 被墙，改用国内镜像
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=off

WORKDIR /build

# 复制构建所需全部文件（含嵌入资源 assets.go 与地区表.txt）
COPY go.mod go.sum ./
COPY assets.go ./
COPY 地区表.txt ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY resource/ ./resource/

# 下载 Go 依赖
RUN go mod download

# 编译 Go 程序（静态链接，适用于 Alpine）
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /build/yyb-go ./cmd/yyb-go

# 第二阶段：最小化运行时镜像
FROM alpine:3.18

# 安装基础依赖（ca-certificates 用于 HTTPS；tzdata 用于时区）
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# 从构建阶段复制编译好的二进制文件
COPY --from=builder /build/yyb-go /app/yyb-go
# 复制地区表（供 -pinzan-regions 默认值从磁盘读取时回退使用）
COPY --from=builder /build/地区表.txt /app/地区表.txt

# 创建运行时数据目录（db/qr/avatars/static 由程序自动生成）
RUN mkdir -p /app/resource && chmod 755 /app/resource

# 暴露 HTTP 端口（程序监听 8000）
EXPOSE 8000

# 设置时区
ENV TZ=Asia/Shanghai

# 启动程序（监听所有接口，便于 Docker 端口映射）
ENTRYPOINT ["/app/yyb-go", "-host=0.0.0.0", "-port=8000"]
