# ---- 构建阶段 ----
# 必须 >= 1.26: golang.org/x/crypto v0.57.0 要求 go 1.26.0（见计划 Tech Stack）
FROM golang:1.26-alpine AS build
WORKDIR /src

# 先只拷贝依赖描述，利用层缓存
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO_ENABLED=0：modernc.org/sqlite 是纯 Go 实现，无需 cgo，可静态链接（V1 已实测）
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/cala ./cmd/server

# ---- 运行阶段 ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/cala /cala
# 数据库文件默认落在 /data，部署时挂载卷
ENV CALA_DB=/data/cala.db
ENV CALA_ADDR=:8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/cala"]
