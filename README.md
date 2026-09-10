# Cala

用于练习速算的自部署应用。前后端分离：Go + Fiber + SQLite 后端，Flutter（Android）前端。

> **当前状态：设计阶段，零行产品代码。**
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

### Windows 开发者必读：pub cache 必须与项目同盘

若 pub cache 与项目位于**不同盘符**（例如项目在 `D:` 而 pub cache 在 `C:\Users\...\AppData\Local\Pub\Cache`），
Android 构建会失败于 `:shared_preferences_android:compileDebugKotlin`，报
`Could not close incremental caches`。

原因：Kotlin 增量编译器在 flush 缓存时对源文件做 `Path.relativize()`，
而 Windows 下**跨盘符无法相对化**，抛 `IllegalArgumentException`。
外层「增量缓存」错误信息具有误导性，真正的原因只出现在 `--stacktrace` 的 `Suppressed` 链里。

修复（令两者同盘），例如把 pub cache 放到与项目相同的盘：

```powershell
[Environment]::SetEnvironmentVariable("PUB_CACHE", "D:\Programs\Pub\Cache", "User")
```

然后重开终端并 `flutter pub get`。

CI 不受影响（Linux 单文件系统）。仅在无法同盘时，才退而设置
`kotlin.incremental=false`——它会掩盖成因并使构建变慢。

测试：

```bash
cd backend && go test ./...
cd app && flutter analyze && flutter test
```

## 发布

推送 `v*` 形式的 tag 会触发 CI：构建 Docker 镜像推送至 `ghcr.io/shinyes/cala`，
并创建 GitHub Release，附带镜像 `.tar.gz` 与已签名 APK。

## 权威文档

| 文档 | 用途 |
|---|---|
| [`specs/2026-09-10-mental-math-app-design.md`](docs/aegis/specs/2026-09-10-mental-math-app-design.md) | 设计规格：决策记录、数据模型、API 契约、验收标准 |
| [`baseline/2026-09-10-initial-baseline.md`](docs/aegis/baseline/2026-09-10-initial-baseline.md) | 产品与架构双基线、不可协商项、兼容边界 |
| [`BASELINE-GOVERNANCE.md`](docs/aegis/BASELINE-GOVERNANCE.md) | 基线治理规则 |
