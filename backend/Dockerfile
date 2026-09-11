# ---- 构建阶段 ----
# 必须 >= 1.26: golang.org/x/crypto v0.57.0 要求 go 1.26.0（见计划 Tech Stack）
FROM golang:1.26-alpine AS build
WORKDIR /src

# 构建期参数：版本号与提交号，由 release.yml 传入。
#
# 默认值刻意是 dev / unknown 而不是某个具体版本：不传参直接 docker build
# 得到的镜像应当自报「未知来源的开发构建」，而不是伪装成某个发布版本。
ARG VERSION=dev
ARG REVISION=unknown

# 先只拷贝依赖描述，利用层缓存
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO_ENABLED=0：modernc.org/sqlite 是纯 Go 实现，无需 cgo，可静态链接（V1 已实测）
#
# -X 把版本号写进 internal/version.Version。
# 已实测 -s -w（剥离符号表）与 -X 可共存：剥离不影响链接期变量赋值。
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
      -ldflags="-s -w -X github.com/shinyes/cala/backend/internal/version.Version=${VERSION}" \
      -o /out/cala ./cmd/server

# 准备一份「数据目录」，下一步带属主拷进运行镜像。
#
# 放一个 .keep 文件是有意为之：COPY 的语义是「拷贝源目录的**内容**」，
# 对**空**目录是否会创建目标目录并应用 --chown，文档表述不够明确。
# 放一个占位文件即可消除该歧义，代价是 /data 下多一个空文件（SQLite 不受影响）。
RUN mkdir -p /out/data && : > /out/data/.keep

# ---- 运行阶段 ----
FROM gcr.io/distroless/static-debian12:nonroot

# ARG 不跨阶段继承，需在此重新声明才能用于下方的 LABEL。
ARG VERSION=dev
ARG REVISION=unknown

# OCI 标准标签。
#
# 为什么值得加：镜像 tag 是**仓库侧**的元数据，一旦镜像被重新打标签、
# 经 docker save/load 搬运，或从别处拷贝，tag 就可能与内容不符。
# 标签写在**镜像内部**，随镜像一起搬运，因此 `docker inspect` 能给出可信答案。
#
# source 标签另有实际作用：它把包与仓库关联，ghcr 上会显示来源链接。
LABEL org.opencontainers.image.title="Cala" \
      org.opencontainers.image.description="速算练习后端（Go + Fiber + SQLite）" \
      org.opencontainers.image.source="https://github.com/shinyes/cala" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}"

COPY --from=build /out/cala /cala

# 创建 /data 并**归属于 nonroot**（65532 是 distroless 中 nonroot 的 uid/gid，
# 用数字而非用户名，避免依赖目标镜像 /etc/passwd 的解析行为）。
#
# 为什么必须在此创建：运行用户是 nonroot。
# 若镜像中不存在 /data，Docker 挂载**命名卷**时会新建该目录并归属 root:root，
# 于是 nonroot 无法创建 cala.db，服务启动即失败：
#   数据库初始化失败: 连接数据库失败: unable to open database file (14)
# 容器 crash-loop，且不会静默降级。
#
# 用 COPY --chown 而非 RUN mkdir/chown：distroless 镜像内**没有 shell**，RUN 无法执行。
# 而当目录已存在于镜像中时，Docker 初始化命名卷会沿用其属主，问题即被消除。
#
# 实测依据：v0.0.1 镜像的 13 层中不存在 /data（逐层解包核对）。
# 该缺陷由 release.yml 的 smoke job 守护 —— 它会在真实 Docker 中启动容器并注册一次用户。
COPY --from=build --chown=65532:65532 /out/data /data

# 数据库文件默认落在 /data，部署时挂载卷
ENV CALA_DB=/data/cala.db
ENV CALA_ADDR=:8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/cala"]

# 版本号的三种查法（distroless 内没有 shell，因此都靠外部命令）：
#   docker run --rm <镜像> --version          -> 只输出 0.0.4
#   curl <地址>/api/healthz                    -> {"status":"ok","version":"0.0.4"}
#   docker inspect <镜像> --format '{{index .Config.Labels "org.opencontainers.image.version"}}'
# 容器启动日志里也会打印版本。
