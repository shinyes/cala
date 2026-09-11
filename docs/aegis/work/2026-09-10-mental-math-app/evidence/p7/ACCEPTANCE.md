# P7 CI 与交付 验收证据

- Date: `2026-09-10`
- Plan: [`docs/aegis/plans/2026-09-10-p7-ci-release.md`](../../../plans/2026-09-10-p7-ci-release.md)
- 范围：规格 §16 的 **P7 CI 与交付**（最后一个阶段）
- 出口条件：**A10**（tag 触发、ghcr 推送、release 附 tar.gz）与 **A15**（已签名 APK）

---

## 1. 全量命令结果

| 命令 | 结果 |
|---|---|
| `cd backend && go build ./...` | exit 0 |
| `cd backend && go vet ./...` | exit 0 |
| `cd backend && gofmt -l internal cmd` | 无输出 |
| `cd backend && go test ./... -count=1` | **9 个包全部 ok** |
| `cd app && flutter analyze` | `No issues found!` |
| `cd app && flutter test` | **117 项全通过** |
| `cd app && flutter build apk --release` | `✓ Built app-release.apk (50.5MB)` |
| `apksigner verify` | 通过，证书 DN 为测试 keystore |

---

## 2. A15 已签名 APK —— 本地真实验证

本机没有用户的 keystore，但 **JDK 17 已安装**，因此可以生成一次性测试 keystore，
走完与 CI **完全相同**的配置路径，验证签名接线是否正确。

### 验证步骤

1. `keytool -genkeypair` 生成测试 keystore（CN=Cala Test Signing）
2. 写 `app/android/key.properties` 指向它
3. `flutter build apk --release --build-name=0.0.1 --build-number=1`
4. `apksigner verify --print-certs`

### 结果

```
Signer #1 certificate DN: CN=Cala Test Signing, OU=Dev, O=Cala, L=City, ST=State, C=CN
Signer #1 certificate SHA-256 digest: 71c4c33f941656d6f7161600b8a37d0a7642050885f1ed2aa90a0810ce43315e
```

**证书是我们自己的 keystore，不是 debug 证书** —— 这证明：

- `build.gradle.kts` 正确从 `key.properties` 读取并建立 `signingConfigs.release`
- release 构建确实使用了该签名配置（而非模板默认的 debug 签名）
- `versionCode` / `versionName` 能由 `--build-number` / `--build-name` 注入

### 断言脚本演练

工作流里的「不是 debug 证书」检查已就地演练：

```
$ apksigner verify --print-certs app-release.apk | grep -qi 'Android Debug' && exit 1
→ 未匹配，断言通过
$ apksigner verify app-release.apk
→ 通过
```

### 清理

测试 keystore 与 `key.properties` 已删除；`git status` 中不含任何签名材料。

---

## 3. 执行期发现并修复的两个真实问题

### 3.1 P-R8 修复不彻底：环境变量到不了已在运行的进程

上一阶段我把 `PUB_CACHE` 设为用户级环境变量（指向 D 盘），并在当时验证通过。
但本阶段构建 **release** APK 时同一个错误又出现了。

原因：

```
当前进程看到的 PUB_CACHE = ''          <- 空
用户级环境变量（注册表）  = D:\Programs\Pub\Cache
```

**环境变量只对「设置之后启动的进程」生效。** DSH 服务进程在我设置该变量之前就已启动，
它派生的 shell 继承的是旧环境。于是：

- 我在设置变量**之后**手动验证的那次构建成功了（`$env:PUB_CACHE` 在同一个 shell 里被显式设置）
- 但后续由旧父进程派生的 shell 看不到它，失败回归

**这是我上一次验证的漏洞**：我验证的是「设置了变量后能成功」，而没验证
「不设置变量时是否也能成功」。前者掩盖了后者的脆弱性。

**修复**：在 `app/android/gradle.properties` 设置 `kotlin.incremental=false`，
从根上避开那条代码路径。

代价评估：Kotlin 增量编译只对**本地重复构建**有效（节省数十秒）；
**CI 每次都是冷构建，本就没有增量状态可复用，因此 CI 零成本**。

**验证**（这次验证的是正确的命题）：

```
PUB_CACHE = ''  (故意不设置 —— 这正是之前失败的条件)
Running Gradle task 'assembleRelease'...  226.8s
✓ Built build\app\outputs\flutter-apk\app-release.apk (50.5MB)
exit=0
```

随后再次验签，证书仍为 `CN=Cala Test Signing`，证明签名接线不受该改动影响。

README 中相应章节已改写，说明根因、为何不依赖环境变量、以及想恢复增量编译该怎么做。

### 3.2 工作流里两个会静默失败的问题

**(a) preflight 用 `keytool` 但没有准备 JDK。**
原写法依赖 runner 镜像自带的 Java，其版本随镜像变动。已加
`actions/setup-java@v4`（temurin 17），使 keytool 行为确定。

**(b) 未加引号的 heredoc 会二次展开口令。**

原写法把 secret 直接插值进脚本文本：

```bash
cat > key.properties <<EOF
storePassword=${{ secrets.ANDROID_KEYSTORE_PASSWORD }}   # 危险
EOF
```

GitHub 先把口令的**字面文本**替换进来，随后 bash 对未加引号的 heredoc 内容
做参数展开、命令替换与算术展开。因此口令里若含 `$` 或反引号就会被**静默改写**，
表现为「口令明明是对的却签名失败」——极难排查。

已改为经环境变量传入：

```bash
env:
  KS_PASS: ${{ secrets.ANDROID_KEYSTORE_PASSWORD }}
run: |
  cat > key.properties <<EOF
  storePassword=${KS_PASS}
  EOF
```

参数展开只发生一次：`${KS_PASS}` 展开为口令值后，值中的 `$` 不会再次展开。

同时在 APK job 里增加了「用 keytool 试开并检查别名」，使口令或别名错误
在 Gradle 之前就以可读信息失败。

---

## 4. 工作流结构

```
preflight ──┬─► test ──┬─► image ──┐
            │          └─► apk ────┴─► release
            └──────────────────────────┘
```

| job | 职责 |
|---|---|
| `preflight` | 从 tag 推导版本与 versionCode；校验 4 个 secret 非空；解码并**试开** keystore；断言签名材料未入库 |
| `test` | 后端测试（含跨端语料门禁 13872 条）+ 前端 analyze/test |
| `image` | 多阶段构建 → push ghcr（版本号/`latest`/`sha`）→ `docker save \| gzip` → 上传 artifact |
| `apk` | 解码 keystore → 建 `key.properties` → `flutter build apk --release` → **验签并断言非 debug 证书** → 上传 artifact |
| `release` | 汇总两个 artifact → 生成说明 → `gh release create` 附 `.tar.gz` 与 `.apk` |

**为什么 release.yml 自带 `test` job**：tag 可能打在未经 CI 的提交上。
发布一个「界面显示答对、落库记为错」的版本，代价远高于重跑一次测试。

**为什么把校验集中到 preflight**：缺少 secret、base64 格式错误、口令错误、别名不存在
——这些都会在后续 job 里以晦涩的 gradle / keytool 错误暴露。集中校验后，
后续 job 的失败必然指向真实构建问题。

**版本号推导**：`v1.2.3` → `versionName=1.2.3`，`versionCode = major*1e6 + minor*1e3 + patch`。
`versionCode` 必须随版本单调递增，否则 Android 拒绝覆盖安装；
同时校验 minor/patch < 1000 以保证严格单调。

---

## 5. 本地等效验证

本机无 Docker，因此把可验证的部分逐一验证：

| 目标 | 验证方式 | 结果 |
|---|---|---|
| Dockerfile 构建阶段 | `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build` | ✅ 20.2 MB，无动态库引用（静态） |
| YAML 结构 | Dart `yaml` 包解析两个工作流 | ✅ 结构合法（下方输出） |
| job 依赖图 | 同上 | ✅ preflight → test → {image, apk} → release |
| step 完整性 | 同上（每步有且仅有一个 `uses` 或 `run`） | ✅ |
| 工作流引用路径存在 | 逐一 `Test-Path` | ✅ 6/6 |
| APK 签名接线 | 测试 keystore 真实构建 + 验签 | ✅ 见 §2 |

YAML 校验输出：

```
=== release.yml ===
  on: [push]
  job preflight: needs=null steps=6
  job test: needs=null steps=5
  job image: needs=[preflight, test] steps=6
  job apk: needs=[preflight, test] steps=9
  job release: needs=[preflight, image, apk] steps=5
=== ci.yml ===
  on: [push, pull_request]
  job backend: needs=null steps=6
  job frontend: needs=null steps=5

全部结构校验通过
```

**shell 片段语法**：本机 `bash` 是未配置的 WSL（无发行版），无法用 `bash -n`。
已改为逐段人工核对，并针对 heredoc 展开语义做了专项修复（见 §3.2b）。
**这一项未被自动化验证**，如实记录。

---

## 6. 未验证项（本机无法验证，需 CI 或用户确认）

| # | 项 | 原因 | 如何确认 |
|---|---|---|---|
| 1 | `ci.yml` 首跑是否通过 | 本环境不可达 github.com | 用户查看 Actions 页 |
| 2 | Docker 镜像能否构建 | 本机无 Docker | 打 tag 后看 release 工作流 |
| 3 | `golang:1.26-alpine` / `distroless/static-debian12` tag 是否存在 | 无 Docker 且 Docker Hub 不可达（P-R6） | 同上；若不存在会立即显眼失败，修复仅改字符串 |
| 4 | ghcr 推送是否成功 | 需 CI 与 `GITHUB_TOKEN` | 同上 |
| 5 | `gh release create` 能否创建 Release | 需 CI | 同上 |
| 6 | 用户真实 keystore 的格式与口令是否与 secret 一致 | 不应读取密钥值 | preflight 的 `keytool -list -alias` 会明确报错 |
| 7 | APK 能否覆盖安装升级旧版本 | 需真机与旧版本 APK | 用户实际安装时 |
| 8 | `subosito/flutter-action@v2` + Flutter 3.44.9 组合 | 需 CI | 同上 |

> 第 1–5、8 项都属于「一旦有误会立即且显眼地失败」的类型，
> 修复代价通常是改一个字符串或一行配置。因此不为它们引入额外的本地验证机制。

---

## 7. 由本阶段产生的两个新的 ADR 级决策

两者都写入了代码注释，供后续维护者理解：

1. **`kotlin.incremental=false` 是本仓库的默认值**，而非临时绕过。
   理由是环境变量无法可靠传递给已在运行的进程，仓库不应依赖 shell 如何启动。
   恢复增量编译的前置条件是「pub cache 与项目同盘」。

2. **CI 中 secret 一律经环境变量传入脚本，不直接插值进脚本文本**。
   理由是未加引号的 heredoc 会二次展开，含 `$` 的口令会被静默改写。

---

## 8. 结论

规格 §16 的 **P7 出口条件**中：

- **A15（已签名 APK）已通过本地真实验证** —— 用测试 keystore 走完完整路径并验签
- **A10（tag 触发 / ghcr / release 附件）工作流已交付且结构合法**，
  但真实执行需 CI，**本机无法验证**

至此规格 §16 的 **P0 → P7 全部阶段完成**。

**新增规模**：`release.yml`（约 330 行）、`build.gradle.kts` 签名配置、
`key.properties.example`、`.gitignore` 与 `gradle.properties` 各一处修改。零新增依赖。
