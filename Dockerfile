# 多阶段构建：builder 里编译，运行镜像只带二进制。
# 好处是运行镜像不含 Go 工具链，体积小、攻击面小。

# ---------- 构建阶段 ----------
FROM golang:1.26-alpine AS builder

# 静态编译，不依赖 glibc，才能跑在 alpine/scratch 上。
ENV CGO_ENABLED=0 GOOS=linux

WORKDIR /src

# 先只拷依赖清单再下载，这样改代码不会让依赖缓存失效。
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 两个服务都编出来，运行时用同一个镜像、换 command 即可。
RUN go build -trimpath -ldflags="-s -w" -o /out/charon-api ./cmd/api && \
    go build -trimpath -ldflags="-s -w" -o /out/charon-rpc ./cmd/rpc

# ---------- 运行阶段 ----------
FROM alpine:3.21

# 时区数据：日志时间戳与业务时间依赖它。
RUN apk add --no-cache tzdata ca-certificates && \
    adduser -D -u 10001 app

WORKDIR /app

COPY --from=builder /out/charon-api /app/charon-api
COPY --from=builder /out/charon-rpc /app/charon-rpc
COPY etc/ /app/etc/

# 用非 root 跑，降低容器被攻破后的影响面。
USER app

EXPOSE 8888 9090

# 默认起 api；起 rpc 用 docker run 覆盖 command。
CMD ["/app/charon-api", "-f", "/app/etc/charon-api.yaml"]
