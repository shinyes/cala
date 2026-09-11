# Cala

用于练习速算的自部署应用。前后端分离：Go + Fiber + SQLite 后端，Flutter（Android）前端。

> **当前状态：已实现并发布（v0.0.2）。**
> 设计与验收标准见 [`docs/aegis/specs/2026-09-10-mental-math-app-design.md`](docs/aegis/specs/2026-09-10-mental-math-app-design.md)。
> 本 README 不复述需求，以避免出现第二个需求事实源。

## 这是什么

- **练习**：创建或订阅速算项目，每轮自带键盘、可暂停继续、打错即时反馈。
- **可编程出题**：每个项目的题目由一个作者编写的 JS 函数 `generate(cfg)` 生成，
  在服务端 goja 沙箱中执行（超时 + 递归上限 + 确定性随机）。
- **统计**：按项目查看日 / 周 / 月折线，以及耗时与正确率的最高/最低/平均/中位。
- **分享订阅**：作者分享项目链接，他人订阅后跟随作者配置（非快照复制）。

## 仓库结构

```
backend/    Go + Fiber + SQLite（api / service / store / rules / auth）
app/        Flutter 应用（Cupertino 风格，Android 为目标平台）
docs/aegis/ 设计规格、基线与工作记录（权威文档）
```

## 本地开发

前置：Go 1.26+、Flutter 3.44+。

```bash
# 后端
cd backend && go run ./cmd/server

# 前端（开发期用浏览器调试）
cd app && flutter run -d chrome
```

### Windows 开发者必读：Kotlin 增量编译与跨盘符

若 pub cache 与项目位于**不同盘符**（例如项目在 `D:` 而 pub cache 在
`C:\Users\...\AppData\Local\Pub\Cache`），Android 构建会失败，报：
 
```
Could not close incremental caches in .../caches-jvm/jvm/kotlin
```

**这条信息具有误导性。** 真正的原因藏在 `--stacktrace` 的 `Suppressed` 链里：

```
IllegalArgumentException: this and base files have different roots:
  C:\Users\<user>\AppData\Local\Pub\Cache\...\SomePlugin.kt
  D:\path\to\project\android
```

Kotlin 增量编译器在 flush 缓存时对源文件调用 `Path.relativize()`，
而 Windows 下**跨盘符无法相对化**，于是抛异常并被包装成「无法关闭增量缓存」。

**本仓库的处理方式**：`app/android/gradle.properties` 里设了
`kotlin.incremental=false`，从根上避开这条代码路径。

之所以不依赖环境变量来修，是因为环境变量**只对设置之后启动的进程生效**：
已经开着的终端、IDE、构建服务看到的仍是旧环境，于是失败会以
「debug 构建成功但 release 构建失败」这类难以捉摸的形式回归。
仓库不应依赖 shell 是怎么启动的。

代价：本地重复构建失去 Kotlin 增量编译（数十秒）。
**CI 不受影响**——CI 每次都是冷构建，本就没有增量状态可复用。

若想恢复更快的本地构建，请修根因（让 pub cache 与项目同盘），
再把该行改回 `true`：

```powershell
[Environment]::SetEnvironmentVariable("PUB_CACHE", "D:\Programs\Pub\Cache", "User")
```

然后**重开终端**并 `flutter pub get`。

测试：

```bash
cd backend && go test ./...
cd app && flutter analyze && flutter test
```

## 安装 Android 客户端

从 [Releases](https://github.com/shinyes/cala/releases) 下载 `cala-<版本>.apk` 侧载安装
（已用正式密钥签名）。

**首次使用必须设置服务端地址**：手机上的 `127.0.0.1` 指向手机自身，
默认地址只在电脑上调试时可用。

1. 打开 App，在登录页底部点「服务器地址」
2. 填入你部署的后端地址，例如 `192.168.1.5:8080`
   （可省略 `http://`；只需填到端口，不要带路径）
3. 点「测试连接」确认能连上，再「保存」

手机与后端需在同一网络，且防火墙需放行该端口。

> **换服务器会退出登录**，这是有意为之：登录令牌属于原服务器，
> 带到新服务器只会得到一串 401。切换后需要重新登录。

## 部署后端

前置：只需 Docker。镜像已发布在 `ghcr.io/shinyes/cala`（**公开包，无需登录**）。

```bash
docker compose up -d
```

首次启动后打开 `http://<主机>:8080` 注册 ——
**第一个注册的用户会成为管理员**，随后建议在「我的」页关闭注册开关。

配置见 [`docker-compose.yml`](docker-compose.yml)（含环境变量说明、备份与恢复命令）。
从源码自建镜像用 [`docker-compose.build.yml`](docker-compose.build.yml)。

要点：

- **数据库在命名卷 `cala-data` 里，这是唯一需要备份的数据。**
- 升级请改 compose 中 `image` 的版本号后 `docker compose pull && docker compose up -d`；
  不建议跟随 `:latest`（会不经预期地应用 schema 迁移）。
- 生产环境应把 `CALA_DEV_CORS_ORIGINS` 留空以完全关闭 CORS：
  后端不提供网页前端，Android 原生客户端不受 CORS 约束。

## 发布

推送 `v*` 形式的 tag 会触发发布流水线：

1. **Preflight** —— 校验版本号、签名 secret、keystore 可用性与密码材料未入库
2. **Test** —— 后端测试（含跨端判分语料门禁）+ 前端 analyze/test
3. **Docker 镜像** —— 构建并推送至 `ghcr.io/shinyes/cala`，同时导出 `.tar.gz`
4. **已签名 APK** —— 用正式 keystore 签名，并断言证书不是 debug 证书
5. **容器冒烟测试** —— 在真实 Docker 中按 compose 的同一配置启动容器、注册一个用户、
   重启后确认数据持久化（这一步会拦下「镜像能构建但跑不起来」类问题）
6. **创建 Release** —— 汇总上述产物作为附件

## 权威文档

| 文档 | 用途 |
|---|---|
| [`specs/2026-09-10-mental-math-app-design.md`](docs/aegis/specs/2026-09-10-mental-math-app-design.md) | 设计规格：决策记录、数据模型、API 契约、验收标准 |
| [`baseline/2026-09-10-initial-baseline.md`](docs/aegis/baseline/2026-09-10-initial-baseline.md) | 产品与架构双基线、不可协商项、兼容边界 |
| [`BASELINE-GOVERNANCE.md`](docs/aegis/BASELINE-GOVERNANCE.md) | 基线治理规则 |
