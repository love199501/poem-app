# 微信云托管 / 通用容器部署
# 不用 Docker Hub（会 429），也不用 ccr.ccs.tencentyun.com/library（没有这个标签）。
FROM docker.m.daocloud.io/library/golang:1.23 AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /server ./cmd/server

# 运行阶段必须带 /bin/sh。云托管启动时会执行 /bin/sh /app/cert/initenv.sh，空镜像没有 shell。
FROM docker.m.daocloud.io/library/alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && update-ca-certificates
WORKDIR /app
COPY --from=build /server /app/server
COPY web /app/web
# 把 .env 一并打进镜像，运行时从 .env 读取配置（无需在云托管控制台单独配环境变量）。
# 用 .env* 通配，确保即使没有 .env（仅 .env.example）也不会构建失败。
COPY .env* ./
ENV TZ=Asia/Shanghai \
    GIN_MODE=release \
    PORT=80
# 云托管会注入 PORT；应用读取 PORT 监听（见 config.go）
EXPOSE 80
CMD ["/app/server"]
