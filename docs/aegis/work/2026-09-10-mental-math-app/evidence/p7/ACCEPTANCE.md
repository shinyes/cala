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

## 6. 未验证项

> 本节在收尾后持续更新，见 §6.1（已关闭项）、§6.2（首次发布的多轮迭代）、
> §6.3（最终成功与独立验签）、§6.4（v0.0.2 修复的部署缺陷）、
> §6.5（服务端地址可配置与 release APK 联网缺陷）、
> §6.6（键盘字母表收敛与文本答案缺陷）、§6.7（镜像内置版本号）、
> §6.8（v0.0.4 发布与独立验证）、§6.9（修改口令）、
> §6.10（v0.0.5 发布与独立验证）、§6.11（「再来一轮」漏实现与两个缺陷）、
> §6.12（v0.0.6 发布与独立验证）、§6.13（网络环境对推送的影响）。

| # | 项 | 状态 |
|---|---|---|
| 1 | `ci.yml` 首跑是否通过 | ✅ **已关闭**（§6.1a，公开 API 直接观察到 12+ 次全部 success） |
| 2 | Docker 镜像能否构建 | ✅ **已关闭**：run #8 的 Docker 镜像 job 成功构建并推送 ghcr（§6.3） |
| 3 | 基础镜像 tag 是否存在 | ✅ **已关闭**（§6.1b，实测 manifest HTTP 200） |
| 4 | ghcr 推送是否成功 | ✅ **已关闭**：run #8「登录 ghcr.io」与「构建并推送」均 success（§6.3） |
| 5 | `gh release create` 能否创建 Release | ✅ **已关闭**：run #8 创建 Release 成功，两个附件就位（§6.3） |
| 6 | 用户真实 keystore 的别名与 secret 是否一致 | ✅ **已关闭**：用户修正 secret 后 preflight 通过（§6.3） |
| 7 | APK 能否覆盖安装升级旧版本 | ⏳ 需真机与旧版本 APK（唯一剩余项） |
| 8 | `subosito/flutter-action@v2` + Flutter 3.44.9 组合 | ✅ **已关闭**（§6.1a） |

### 6.1 收尾时关闭的两项

**(a) `ci.yml` 持续通过 —— 用户报告**

用户确认 CI 一直正常。**证据等级说明**：这是用户报告而非我直接观察
（本环境不可达 github.com，我从未看到过 Actions 页面）。因此记为
「用户报告的事实」，而非「已验证」。

> **后续（仓库改为 public 后）**：已可通过公开 API 直接观察，不再依赖转述。
> 截至最后一次查询，`CI` 工作流共 12 次运行**全部 success**（含 `checkout@v6`
> 升级后的 run #12）——因此本项已从「用户报告」升级为**直接观察**。

由此关闭的连带项：第 8 项（`subosito/flutter-action@v2` + `flutter-version: '3.44.9'`
组合可用）也随之确认 —— `ci.yml` 的 frontend job 用的正是这一组合。
后端 job 的 `actions/setup-go@v5` + `go-version-file: backend/go.mod`
同样被确认，即 **Go 1.26.0 确实已发布且可被 setup-go 解析**。

**(b) 两个基础镜像 tag 均存在 —— 本次实际查询确认**

P-R6 原先记为「无法本地验证，若无会由 CI 立即显眼失败」。收尾时发现本环境
**能访问 Docker Hub 的国内镜像**（虽然 `hub.docker.com` 与 `registry-1.docker.io`
不可达，但 `docker.m.daocloud.io` 可达），于是直接查询了 manifest：

```
golang:1.26-alpine                        -> HTTP 200  存在
distroless/static-debian12:nonroot        -> HTTP 200  存在
distroless/static-debian12:latest         -> HTTP 200  存在
```

查询方式：Docker Registry v2 API，经 `WWW-Authenticate` 头取 realm 后走匿名
token 流程（`scope=repository:<repo>:pull`），再带 Bearer token 请求 manifest。

**这项原先是「假设」，现在是「已核实的事实」。** 这是本阶段唯一一次把
「无法验证」真正转成「已验证」的机会，值得单独记录 ——
原先我对它的判断是「一旦有误会立即失败」，但能验证就不该停留在判断。

**因此 release.yml 中「外部引用是否有效」这一整类风险已清除**；
剩余的第 2、4、5 项都是**工作流自身的执行逻辑**（Docker 构建、推送、创建 Release），
不再是外部依赖问题。


### 6.2 首次 tag 发布的四轮迭代（真实运行记录）

仓库改为 public 后，发布流程的状态已可通过公开 API 直接观察（不再依赖用户转述）。
`v0.0.1` 的发布经过四轮，每一轮都排除了一种假设或暴露一个问题：

#### 第 1 轮（run #34564664640，提交 142fef9）

| job | 结果 |
|---|---|
| Preflight | ✅ |
| Test | ✅ |
| Docker 镜像 | ✅ **构建并推送 ghcr 成功**，导出 tar.gz 成功 |
| **已签名 APK** | ❌ 失败于「解码 keystore 并生成 key.properties」 |
| 创建 Release | ⏭️ 跳过 |

失败信息是我自己加的那道检查报出的：`keystore 无法打开或别名不存在`。
而同一提交下 Preflight 的 keystore 检查**通过**（它只用 `-storepass`，不带 `-alias`）。
两者对比即可定位：**口令正确，是别名不匹配**。

同时从注解中取得两条弃用警告（Node.js 20），据此**实测**了 action 的
`runs.using` 字段（`actions/setup-java@v5` 与 `actions/checkout@v6` 均为 `node24`），
将 `checkout@v4→v6`、`setup-java@v4→v5`。

#### 第 2 轮（run #34566062017，提交 3cbc6ae）

`Preflight` 失败，注解为：

```
ANDROID_KEY_ALIAS='***' 在 keystore 中不存在。
```

（`***` 是 GitHub 对 secret 的自动遮蔽。）

**这一轮验证了前一轮所加诊断的价值**：问题在 **20 秒内**的 preflight 阶段被精确报出，
并带上「ANDROID_KEY_ALIAS」这一明确指向，而不是等到装完 Flutter（约 4 分钟）
才以晦涩的 Gradle keystore 异常失败。

#### 第 3 轮（run #34567019284，提交 b243a99）

把「keystore 中**实际存在**的别名」从日志移进失败注解后，一次拿到答案：

```
ANDROID_KEY_ALIAS 在 keystore 中不存在。keystore 里实际可用的别名: cala,my-key-alias
```

**该 keystore 含两个私钥条目**，而 secret 的值与两者都不匹配。

#### 第 4 轮（run #34568284818，提交 557620d）

把别名规范化改为「只保留 `[A-Za-z0-9_.-]`」后重跑，**报文完全相同**：

```
ANDROID_KEY_ALIAS 在 keystore 中不存在。keystore 里实际可用的别名: cala,my-key-alias
```

这一轮排除了一个假设：secret 的值**不是**「`cala` 加上空白或不可见字符」。
因为激进规范化会剥掉一切非 `[A-Za-z0-9_.-]` 字符 ——
若 secret 是 `cala` 加任何粘贴污染（含实测能穿过 ASCII 修剪的 U+200B 零宽空格），
规范化后都会得到 `cala` 并通过校验。

**结论**：`ANDROID_KEY_ALIAS` 的值是另一个字符串，与 keystore 中的两个别名都不匹配。
这属配置问题，须由仓库所有者修正 secret —— 工具不应代为猜测签名身份（见下节）。

> **为何不输出「收到的是什么」**：即便只输出长度或某种特征也有泄露风险 ——
> 无法排除用户把**口令误填进了别名的 secret**，那样任何派生信息都在披露凭据。
> 因此诊断只输出「keystore 里有什么」，绝不输出「你填了什么」。
> 这也是该检查只能报「不匹配」而不能报「你填的是 X」的原因。

#### 一个刻意的设计判断：不做别名自动回退

面对「secret 填错但 keystore 里有可用别名」，一个自然的想法是自动选用
keystore 中的别名。**本设计明确拒绝**，理由有两条：

1. **别名在此处不唯一**（有 `cala` 与 `my-key-alias` 两个），自动选择是任意的。
2. 更重要：**选择哪个密钥签名是一个持久且近乎不可逆的决定** ——
   Android 要求升级包的签名与已安装版本一致，换密钥会导致已安装用户无法覆盖升级
   （规格 §10.3 已记录该后果）。因此签名身份必须由**所有者有意选择**，
   工具不应代为猜测。

工具该做的是把事实摆清楚（列出可用别名），而不是替用户做这个决定。

#### 关于把别名放进公开注解的信息披露判断

仓库已改为 public，注解对外可见。把 keystore 中的别名列出，是否有安全影响？

判断为**无实质影响**，理由：签名需要同时具备
① keystore 文件、② store 口令、③ key 口令。别名单独存在没有任何用处，
且 Android 官方文档与大量公开仓库都把别名视为非敏感信息
（本项目自己的 `key.properties.example` 里也写着 `keyAlias=cala`）。
收益是让「secret 填错」一次定位，不必反复试错——因此采纳，并在工作流中写明该判断。

### 6.3 成功：run #34570826903（提交 c2ae8ac）—— A10 与 A15 达成

用户把 `ANDROID_KEY_ALIAS` 修正为 `cala` 后，第 5 次触发**全流程通过**：

| job | 结果 |
|---|---|
| Preflight | ✅ |
| Test | ✅ |
| Docker 镜像 | ✅ 构建 + 推送 ghcr + 导出 tar.gz + 上传 artifact |
| **已签名 APK** | ✅ 构建 + **验签并断言非 debug 证书** + 上传 artifact |
| 创建 Release | ✅ |

**Release 已发布**：`https://github.com/shinyes/cala/releases/tag/v0.0.1`

| 附件 | 大小 | GitHub 记录的 SHA-256 |
|---|---|---|
| `cala-0.0.1-linux-amd64.tar.gz` | 8,712,571 B | `8da305d7…` |
| `cala-0.0.1.apk` | 52,931,115 B | `10f25fb7…` |

#### 第 5 轮修的问题：`download-artifact` 把 buildx 的缓存条目也当成产物

第 4 次触发（run #34569828481）时四个 job 全部成功，只有最后的「创建 Release」
失败于 `actions/download-artifact@v4`：

```
Unable to download and extract artifact: Artifact download failed after 5 retries.
```

根因由 `runs/{id}/artifacts` 列表直接暴露 —— 该次运行的 artifact 有三项：

```
apk                                  26,755,376 B   <- 本工作流
shinyes~cala~AHTO4N.dockerbuild          51,391 B   <- docker/build-push-action 的 GHA 缓存
docker-image                          9,032,919 B   <- 本工作流
```

第三项是 `cache-to: type=gha` 创建的构建缓存条目，它以 artifact 形式出现在运行里。
原先的写法是不带 `name` 的 `download-artifact`（即「下载全部」），
于是它连带去取这个缓存 blob 并当作 zip 解压，必然失败。

**修复**：改为两个显式指定 `name` 的下载步骤（`docker-image` 与 `apk`）。
这也更符合意图 —— 只取自己要发布的产物，而不是「本次运行产生的一切」。
构建缓存保留（它对加速有价值），只是不再被误当作发布产物。

#### 独立验证（不依赖 CI 自述）

CI 的验签步骤成功本身已说明问题，但我进一步做了**独立验证**：
从 Release 页面下载已发布的 APK，在本机用 `apksigner` 亲自验签。

```
下载:              50.48 MB
大小与 Release 记录: ✓ 一致（52,931,115 B）
SHA-256 与记录:     ✓ 一致（10f25fb7…）
apksigner verify:   Verifies
  v2 scheme:        true
  signers:          1
  证书 DN:          CN=Unknown, OU=Unknown, O=Unknown, L=Unknown, ST=Unknown, C=Unknown
  非 debug 证书:     ✓
```

**关于证书 DN 全为 `Unknown`**：用户生成 keystore 时未填 distinguished name。
**不影响任何功能**：Android 对侧载应用不校验 DN 内容；
可覆盖升级取决于**签名密钥本身**而非 DN。故无需处理，仅记录以免日后困惑。

**这使 A15 从「CI 自述通过」升级为「已发布的产物经独立验签确认」。**

### 6.4 v0.0.2：部署配置暴露的两个真实缺陷

用户要求给出 docker compose 部署配置。为写出**准确**的配置，我去核实了已发布镜像的
实际配置（而非照 Dockerfile 推断），由此发现两个缺陷。

#### 缺陷 1：镜像中不存在 `/data`，compose 部署必然失败

查询 v0.0.1 镜像的 config 得到：

```
Entrypoint  = [/cala]         User = nonroot:nonroot
Env         = CALA_DB=/data/cala.db, CALA_ADDR=:8080
Healthcheck = null
```

容器以 `nonroot`(uid 65532) 运行，而 `/data` 是数据库路径。
**逐层解包 v0.0.1 镜像的全部 13 层**核对，确认其中**不存在 `/data`**
（也不存在 `/home/nonroot`）。

Docker 挂载**命名卷**时会新建挂载点目录并归属 `root:root`，
于是 nonroot 无法创建 `cala.db`。实测该情形为**硬失败**：

```
数据库初始化失败: 连接数据库失败: unable to open database file (14)   （exit 1）
```

即容器会 crash-loop，不会静默降级。该缺陷在「构建成功 + 镜像已推送」时完全不可见。

**修复**：`COPY --chown=65532:65532` 在镜像中创建 `/data` 并归属 nonroot。
必须用 `COPY` 而非 `RUN mkdir/chown` —— distroless 内没有 shell，`RUN` 无法执行。
源目录放一个 `.keep` 占位文件：`COPY` 的语义是拷贝源目录的**内容**，
对**空**目录是否创建目标目录并应用 `--chown`，文档表述不够明确；放文件即消除歧义。

**双重验证**：

1. **运行时**（CI，真实 Docker）：`release.yml` 新增 `smoke` job，
   按 compose 的同一配置以命名卷启动容器。8 个步骤全部通过：
   等待 `/api/healthz` -> 断言未 crash-loop -> 注册首个用户并断言 `becameAdmin=true`
   -> 重启容器后确认仍能登录（证明数据落在卷中而非容器可写层）。
   注册会写 `user` 表，因此这就是「nonroot 能写入命名卷」的直接证据。
2. **镜像层**（本机独立解包 0.0.2 镜像）：

```
data          type=5 uid=65532 gid=65532 mode=755   <- 修复后存在且属主正确
data/.keep    type=0 uid=65532 gid=65532 mode=644
```

0.0.1 中不存在 `data`，0.0.2 中存在且属主为 65532 —— 修复在镜像层面得到确认。

> 这个 `smoke` job 本可以拦下缺陷 1。它的价值正在于此：
> 「镜像构建成功 + 推送成功」不等于「镜像能跑起来」。

#### 缺陷 2：`CALA_DEV_CORS_ORIGINS=""` 不会关闭 CORS，反而回退到默认来源

写部署配置时把该项设为空以关闭 CORS，随后核对代码发现该假设不成立：

```go
func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {   // 空字符串走 def
		return v
	}
	return def
}
```

显式设为空字符串时 `ok == true` 但 `v == ""`，于是走**默认值**分支，
得到 `localhost:3000` 与 `127.0.0.1:3000` ——
与 `config.go` 自己第 40 行的注释（「空字符串 -> 空列表，即完全不启用 CORS 放行」）
**正好相反**。生产部署若照此设置，会以为已关闭 CORS，实际仍放行开发期来源。

**修复**：该变量改用 `os.LookupEnv`，区分「未设置」（用开发期默认）与
「显式设为空」（空列表，不放行任何来源）。

**回归防护与变异验证**：新增两项测试。为确认测试真的能捕获该缺陷，
把修复**变异还原**后运行 —— 测试如实失败，并报出错误值：

```
--- FAIL: TestExplicitEmptyCORSDisablesIt
    config_test.go:74: 显式设为空时 DevCORSOrigins 应为空,
    得到 []string{"http://localhost:3000", "http://127.0.0.1:3000"}
```

这同时证明了缺陷真实存在、且测试能捕获它。恢复修复后文件 SHA-256 与原值一致，
全部 5 项 config 测试通过。

#### 一处刻意的取舍：不配 healthcheck

`docker-compose.yml` 中**刻意没有**配置 healthcheck。原因：镜像基于 distroless，
容器内没有 shell、`curl` 或 `wget`，无法编写探活命令。
可用性由 `restart: unless-stopped` 与宿主机端口探测保障。
若需要 Docker 级健康检查，需给服务端加一个自检子命令 —— 记录为后续增强，本次不做。

**另一处取舍**：compose 固定版本号而非 `:latest`。
理由：Cala 用迁移表按序升级 schema，跟随 `:latest` 可能在无预期的情况下应用一次迁移；
固定版本让升级成为显式动作（改一行 -> pull -> up -d）。

### 6.5 手机端可配置服务端地址；并发现 release APK 无法联网

用户要求「手机端可配置服务端地址」。实现时发现一个**阻塞性缺陷**：
已发布的 APK 根本无法发起网络请求 —— 若不修，该功能毫无意义。

#### 阻塞缺陷：release APK 没有 INTERNET 权限，明文 HTTP 也被禁

源码层面即已可确认：`app/src/main/AndroidManifest.xml` 中**没有任何
`<uses-permission>`**，而 `src/debug` 与 `src/profile` 的清单里都有 INTERNET。
Flutter 模板只在调试/性能构建中声明该权限。

对**已发布的 0.0.2 APK** 执行 `aapt2 dump xmltree` 取证：

```
targetSdkVersion      = 36        <- >= 28，明文 HTTP 默认被禁
INTERNET 权限         = 不存在
usesCleartextTraffic  = 不存在
networkSecurityConfig = 不存在
```

该 APK 声明的唯一权限是内部的 `DYNAMIC_RECEIVER_NOT_EXPORTED_PERMISSION`。

**为什么长期未被发现**：此前所有验证都在 debug 构建下进行
（浏览器调试、debug APK），而 release APK 只验证过**签名**，
从未真正运行过 —— 它恰好就是此前唯一被标为「未验证」的真机安装项。
这说明「构建成功 + 签名正确」与「应用能工作」之间可以差得很远。

**修复**（均写在 main 清单）：
1. `<uses-permission android:name="android.permission.INTERNET" />`
2. `android:networkSecurityConfig` 指向新增的 `network_security_config.xml`，
   其中 `cleartextTrafficPermitted="true"`。

必须放行明文：后端只支持 HTTP（Go 服务端未实现 TLS，配置项里没有任何
证书相关字段），用户自建部署通常在局域网内以 `http://192.168.x.x:8080` 访问。
无法把放行范围限制为私有网段 —— 该配置按域名匹配，不支持 IP 段。

**验证**：本地构建 release APK 后解包确认：

```
INTERNET 权限          : 存在 ✓
networkSecurityConfig  : 存在 ✓（资源已打包）
targetSdk              : 36
```

（本地构建用 debug 密钥签名属正常：`key.properties` 不存在时的回退，
CI 会用正式 keystore。）

#### 功能设计

详见 ADR-0006。要点与理由：

- **规范化**（`app/lib/api/server_address.dart`，纯函数、不依赖 Flutter）：
  处理用户真实会犯的手误 —— 无 scheme（`192.168.1.5:8080`）、末尾斜杠
  （会让 Dio 拼出 `//api/...`）、粘贴带入的空白（**含夹在中间的空白与不可见
  字符**，只 trim 两端会漏掉）、裸 IPv6（`::1` 需补方括号）。
  带路径的地址被拒绝并说明原因，而不是拼出错误 URL 后报难懂的 404。
- **就地修改 baseUrl，不重建 ApiClient**：token 存在实例上，且
  AuthApi / ProjectApi / StatsApi / SubscriptionApi 都持有同一实例；
  重建会丢登录态并让各包装器指向旧实例。
- **单点写入**：`ServerAddressNotifier.set()` 一次完成持久化、应用到客户端、
  清除登录态三件事。换服务器必然作废登录态，且**设为等价地址是空操作** ——
  否则用户只是想「看一眼当前地址再保存」就会被登出。
- **启动前加载**：`main()` 在 `runApp` 之前读取已保存地址，
  避免登录态恢复先以默认地址发出请求。存储值非法时回退默认值，
  不让坏掉的偏好设置把用户锁在应用外。
- **数据自动作废**：新增 `dataScopeProvider`（服务器地址 + 令牌），
  项目/统计/订阅 provider 在 `build()` 里 watch 它，地址或身份一变即重建。
  声明式，因此新增数据 provider 时不会「忘记作废」。
- **界面**：设置页含「测试连接」（探测 `/api/healthz`），把「地址错」与
  「服务端故障」区分开；入口在登录页与「我的」Tab。
  登录页的入口是必需的 —— 连不上服务器时登录框本身是死的。

#### 顺带修复的两个真实缺陷（由新测试暴露）

1. **`SessionNotifier._restore()` 会覆盖恢复期间已建立的登录态**：
   恢复是异步的（读存储 + 请求服务端），期间用户完全可能已注册成功，
   结果被恢复结果改回「未登录」，表现为「注册成功后又被弹回登录页」。
   现跳过该情形，并加 `ref.mounted` 守卫。
2. **异步加载可能在 provider 重建后才完成，向已销毁的 notifier 写 state 而抛错**：
   `ProjectsNotifier.refresh()` 等。加了 `ref.mounted` 守卫并丢弃属于旧作用域的
   结果。该路径因 provider 现随作用域重建而变得容易触发。

#### 验证

- 前端 `flutter analyze` 无问题；测试 **159 项全通过**（原 117，新增 42）
- 新增测试：地址规范化 26 项 + 状态/持久化/数据作废 16 项
- **变异验证**：移除 `ProjectsNotifier` 中的 `dataScopeProvider` watch 后，
  两项「数据重建」测试如实失败；文件 SHA-256 与还原前一致
- **真实 HTTP 端到端**：起两个真实后端（`127.0.0.1:18120` 与 `:18121`，
  不同 DB），用真实 `ApiClient` 验证：
  改地址后请求确实改道、两台服务器数据完全隔离（A 的账号在 B 上不存在，
  反之亦然）、切回后数据仍在、错误端口给出可读错误
- release APK 解包确认权限与明文配置生效
- **已发布产物复核**（不依赖 CI 自述）：`v0.0.3` 发布后下载 Release 中的
  `cala-0.0.3.apk` 并解包：

```
INTERNET 权限         : 存在 ✓
networkSecurityConfig : 存在 ✓
明文配置资源          : 已打包 ✓
签名                  : 正式密钥（证书 SHA-256 4e232cd4…，非 debug）
```

  该证书 SHA-256 与 `v0.0.1` 的 APK 完全一致 —— **签名身份跨版本稳定**，
  这正是覆盖安装能成功的前提（验收项 A15 的「可覆盖安装」部分因此有了
  除签名一致性之外的间接证据）。
- 后端未改动，9 包测试仍全通过

### 6.6 键盘字母表收敛与随之暴露的「文本答案」缺陷

用户要求：键盘只保留 `0-9`、小数点与确定，**不要分数键**，**保留负号**。
在核实「这样改会不会让某些答案无法输入」时，发现了一个此前长期存在的缺陷。

#### 改动：键盘字母表由 `0-9 . - /` 收敛为 `0-9 . -`

layout 由 4 列 4 行改为 **3 列 5 行**：

```
1 2 3
4 5 6
7 8 9
- 0 .
⌫ 确定(占两列)
```

**去掉 `/` 是安全的**：判分是有理数**交叉相乘**比较，`3/4` 与 `0.75` 判为相等，
因此凡能写成**有限小数**的分数都能直接输入。

唯一例外是**无限循环小数**（如答案 `1/3`）：没有有限小数写法，
去掉 `/` 后无法精确输入，此类题目**必须配置容差**（如 1/100 接受 0.33）。
该后果已写入 `keypad.dart`、规格 §5.5.4 与 §9.3 —— 因为它是作者会踩的坑。

同时确认清洗表与判分**仍然接受** `/`，移除的只是**输入途径**：
历史记录与错题重练中的分数答案不受影响。这是纯 UI 约束变更。

实现上把「确定占两列」从「按行长度猜」（`row.length == 2`）改为**显式声明**：
原先依赖行长度这一巧合，改布局时容易静默错位。

物理键盘同步收敛（`_allowed` 去掉 `/`）：界面上打不出的字符，
物理键盘也不应能输入，否则「开发期能答、手机上答不了」会掩盖问题。

#### 缺陷：文本答案能保存，但永远答不对

**真实后端端到端取证**（`127.0.0.1:8090`）：

```
2. **项目保存成功** id=2  <- 文本答案通过了保存期校验
3. 题面 q      = 7 是质数吗？(填 质数 或 合数)
   答案 a      = 质数
   信封 kind   = text
```

即：规则 `a: "质数"` **保存成功**，服务端也正常下发 `kind=text` 的题目，
但练习页只有数字键盘（`0-9 . -`）且刻意不唤起系统键盘（功能6）——
**该题永远答不对**，用户还会白做一整轮并把全错记录写进统计。

**根因是两处规格从未对账**：§5.5.2 明确允许文本答案（且曾专门为「允许中文」
修过一次设计缺陷），而功能6 要求自带键盘。二者各自都自洽，放在一起才矛盾。

**修复**：按速算的实际用途收窄 —— service 层在**保存期与开轮时**均拒绝文本答案，
并给出可读理由与改法。拒绝逻辑收敛为单一 helper（`classifyAnswerable`），
两处调用共用，避免同一规则两份实现而漂移。

**兼容边界（关键）**：收窄只发生在 **service 层的产品校验**，
**`scoring.Classify` 必须继续接受文本** —— 已落库的历史信封（`kind=text`）
仍要靠它判分。该边界由 `TestScoringStillClassifiesTextAnswer` 锁定，
防止日后有人把拒绝逻辑「顺手」下沉到 `scoring`。

**真实 HTTP 复验**：

```
1. 文本答案 a="质数"  -> HTTP 400  code=bad_request
   message = 项目配置不合法: 第 1 题：答案是文本 "质数"，无法作答：
             练习页只有数字键盘（可输入 0-9、小数点与负号），且不唤起系统键盘。
             请让 generate 返回数值答案
2. 数值答案 a="2"     -> HTTP 201   （无误伤）
3. 分数答案 a="1/2"   -> HTTP 201   （分数仍是合法数值答案，用户输 0.5）
```

**意外收获（测试反向纠正了我）**：我改写错误消息时丢掉了「无法分类」这一措辞，
导致既有测试 `TestCreateProjectRejectsMalformedAnswer` 失败。
该测试断言的是**真实意图**（作者需一眼看出问题出在答案形态上），
因此正确的动作是**改回我的代码**而不是放宽测试 —— 已在 helper 中保留该措辞。

#### 同日第二次收窄：答案只接受整数或小数

产品要求「答案不需要是分数，目前只需要整数或小数即可」。分数形式
（如 `a: "1/2"`）同样被拒绝，与文本答案**同源**：
都遵循「作者写的答案必须能用练习键盘原样敲出」。

**为什么这比只拒绝文本更强**：键盘没有除号，因此分数形式里
**无限循环小数**（如 `1/3`）永远敲不出来，作者必须自行配容差 ——
那正是上一节刚记录下来的坑。收窄到整数/小数后，
**「作者写的答案一定能用键盘原样敲出」无条件成立**，该坑被彻底消除。

**关键实现判据**：不能靠信封区分分数与小数 ——

```
实测：a = "0.5"  ->  信封 {kind: rational, num: 5, den: 10}
      a = "1/2"  ->  信封 {kind: rational, num: 1, den: 2}
```

两者数值相等、形状不同，且 `0.5` 同样以 num/den 落库 ——
**这正是 `ParseRational` 的分数分支一条都不能少的证据**
（它不是只为历史分数服务的，小数也走它）。
因此判据取**清洗后的原始字面量是否含 `/`**；用 `Clean` 之后的结果判断，
可一并覆盖全角 `／`（清洗表把它映射为 `/`）。

**真实 HTTP 复验**：

```
=== 应被拒绝 ===
a="1/2"     -> 400  答案是分数形式 "1/2"，但本题型只接受整数或小数：
                    练习键盘没有除号，请改写为等值小数（如 1/2 写作 0.5）；
                    若该值无法写成有限小数（如 1/3），请改用近似小数（如 0.33）并配置容差
a="7/4"     -> 400  同上
a="-1/2"    -> 400  同上
a="１／２"   -> 400  同上（全角亦被覆盖）
a="1/0"     -> 400  无法分类：看起来是数值但无法解析（分母不能为零）

=== 应被接受 ===
a="42" / "-7" / "0.5" / "-1.25"  -> 全部 201
```

**分类的刻意区分**：`1/2`（合法但已不允许的形式）与 `1/0`、`1 / 2`
（**笔误**，按文法 `整数/整数` 本身就畸形）走**不同**分支、给不同提示。
前者提示「改写为小数」，后者提示「检查该数值的写法」。
这不是实现巧合而是有意的：把笔误说成「形态受限」会让作者以为改写小数即可，
而实际问题是他把分数写错了。
（测试上，`1 / 2` 因此归入 malformed 组而非 fraction 组。）

**顺带修掉一处由本次收窄引入的不一致**：`scoring` 在「数值无法解析」时
曾建议作者「若要作为文本答案，请包含数值以外的字符」——
而文本答案此时已被拒绝，照做会撞上第二道拒绝。
该建议已从 `scoring` 移除（本包保持产品无关），改为中性的「请检查该数值的写法」；
产品层指引由 service 负责。复验确认该案例的消息已是修正后的版本。

**兼容边界**：与文本答案一致 —— 拒绝只在 service 层，
`scoring.Classify` 与 `ParseRational` 继续接受分数。
`TestScoringStillClassifiesTextAndFraction` 同时锁定两点：
文本/分数仍可分类，且用户敲 `0.5` 对作者写的 `1/2` 与 `0.5` **都判对**
（这正是「把分数改写成小数不改变题目对错」的依据）。

### 6.7 镜像内置版本号

用户要求「镜像也要有版本号」。核实后发现：镜像的 **tag** 一直有版本号
（`:0.0.3`、`:latest`、`:sha-…`），但**镜像内部的二进制完全没有版本信息** ——
`/api/healthz` 只返回 `{"status":"ok"}`，跑起来的容器无法自报版本。

对比之下，APK 内部是**有**真实版本号的（发布时经
`flutter build apk --build-name=<tag 版本>` 注入）。这个不对称正是「也」字的由来。

**为什么 tag 不够**：tag 是**仓库侧**的元数据。一旦镜像经 `docker save/load`
搬运、被重新打标签、或从别处拷贝，tag 就与内容无关了。要回答
「现在跑的是哪个版本」，必须让镜像自己知道 —— 这也是排查部署问题时第一个要问的。

**实现**（四条读取路径，一个来源）：

```
注入   Dockerfile 的 ARG VERSION -> -ldflags "-X .../version.Version=<版本>"
来源   backend/internal/version.Version（默认 "dev"）

① docker run --rm <镜像> --version   -> 0.0.4
② curl <地址>/api/healthz            -> {"status":"ok","version":"0.0.4"}
③ docker inspect <镜像> --format '{{index .Config.Labels "org.opencontainers.image.version"}}'
④ 容器启动日志首行
```

**几处刻意的决定**：

- **`--version` 在读配置、开数据库之前处理**。运行镜像是 distroless，
  里面没有 shell，检查版本只能从外部调用该子命令；若它要先连上数据库才能回答，
  那么在「数据库挂载有问题」这类**恰恰最需要确认版本**的场景里反而用不了。
- **默认值是 `dev` 而非空串**：本地 `go build` 出来的二进制应自报「开发构建」，
  既不伪装成某个发布版本，也不显示为空白。由 `TestVersionDefaultIsDev` 钉住。
- **输出只有版本号本身**（不含前缀），便于脚本直接取用。
- **版本号单独成包**：它有两个读者（启动日志与 HTTP 层），
  放在 `internal/version` 让两者读同一个值，且 **api 包不必依赖 main**
  （那会形成反向依赖）。
- **不做版本比较**：它的用途是**标识**（跑的是哪个），不是**判断**（是否该迁移）。
  schema 版本由迁移表自己的计数负责 —— 混用会造出第二个事实源。

**实测四条路径**（本机真实运行，非推理）：

```
cala.exe --version                          -> 0.0.4
CALA_DB=Z:\nonexistent cala.exe --version   -> 0.0.4   (exit 0，坏 DB 路径下仍可回答)
启动日志                                     -> Cala 后端 0.0.4 监听 0.0.0.0:8090，数据库 …
curl 127.0.0.1:8090/api/healthz             -> {"status":"ok","version":"0.0.4"}
curl 192.168.110.115:8090/api/healthz       -> {"status":"ok","version":"0.0.4"}
```

**契约验证**：用 App 的解析函数 `describeHealthResponse` 直接处理**真实服务端**响应，
确认客户端读取的字段名与服务端实际返回的一致（字段名写错是静默错误）：

```
服务端原始响应: {"status":"ok","version":"0.0.4"}
客户端解析结果: 连接成功，服务端正常（版本 0.0.4）
```

**流水线守护**：`release.yml` 的 smoke job 新增断言 ——
**tag、OCI 标签、服务端自报、`--version` 输出四者必须一致**。
这条断言针对的是一类**静默失败**：Dockerfile 里 ARG 名写错、`-X` 路径写错、
`build-args` 没传，镜像都会照常构建成功、照常推送，只是自报 `dev`。
只有真的跑起来对比才能发现 —— 与 §6.4 的 `/data` 缺陷同类。

**客户端**：服务端地址设置页的「测试连接」顺带显示版本号。
该页的用途正是「我连的是哪个服务器」，而版本是分辨服务器最直接的线索。
对不返回 `version` 的旧服务端保持兼容（报「未提供版本号」而非「不符合预期」），
该兼容分支由 `describeHealthResponse` 的 7 项单测覆盖，包括
`version` 类型异常（null/数字/布尔/数组）时不抛异常 ——
因此特意把它抽成**纯函数**而非内联在 `setState` 里，否则该分支需真实网络才能测。

### 6.8 v0.0.4 发布与独立验证

`v0.0.4`（run #34616231075，提交 b52cb6f）**六个 job 全部成功**，
其中包括本轮新增的「断言版本号三重一致」步骤。

**独立验证**（不依赖 CI 自述）：

从 ghcr **直接读取**已发布镜像的 OCI 标签：

```
org.opencontainers.image.version  = 0.0.4
org.opencontainers.image.revision = b52cb6f2980cdc5d391e34028077a2645a58dfd0   <- 与 tag 所指提交一致
org.opencontainers.image.source   = https://github.com/shinyes/cala
```

再**解包镜像层**取出 `/cala` 二进制核对：

```
/cala  大小=20.22 MB  ELF=true
内嵌字符串 "0.0.4" 存在: true
```

这一条才是关键证据：它证明 ldflags 注入**真的作用到了已发布产物**，
而不只是构建脚本里写了对的参数。若 ARG 名或 `-X` 路径写错，
镜像仍会构建成功、标签也仍正确，只有二进制里会是 `dev` ——
本检查正是为发现这类静默失败而做。

从 Release 下载 APK 并核对：

```
大小 / SHA-256        : 与 GitHub 记录逐字一致（d13f3485…）
versionName           : 0.0.4
versionCode           : 4          <- 比 v0.0.3 的 3 递增，Android 覆盖安装的前提
INTERNET 权限         : 存在 ✓      <- v0.0.3 修复的回归检查
networkSecurityConfig : 存在 ✓      <- 同上
签名证书 SHA-256      : 4e232cd4…  <- 与 v0.0.1~v0.0.3 同一身份，故可覆盖安装
```

`compose` 的 `image:` 已同步升到 `0.0.4`。

### 6.9 修改口令

用户要求增加改密功能。除基本路径外，重点处理了三个**安全**与**易错**点。

#### 关键设计决定

**① 必须提供当前口令。** 若只凭会话令牌放行，任何拿到令牌的人
（设备失窃、令牌泄露）都能改密并**永久占有**账号 ——
令牌本会过期，改密却把访问权延展到无限期。要求当前口令把这一步重新绑定到
「知道口令」上，令牌泄露不再等于账号失守。

**② 成功后吊销全部会话，并为当前设备签发新令牌。**
改密最常见的动机是「怀疑账号被人登录」。若旧会话继续有效，
攻击者仍能访问，用户的补救措施完全落空 —— 而那正是他做这件事的原因。
- 吊销**全部**而非「除当前外」：排除当前会话需要额外的 WHERE 条件，
  漏写就是把攻击者的会话留下。
- 随即签发新令牌，使当前设备**无感续用**，用户不会在改密后被踢下线。
- 因此该端点**返回新令牌**，客户端必须用它替换本地值。

**③ 口令更新、会话吊销、新会话签发在同一个事务内。**
分开执行会留下自相矛盾的状态：
- 口令已改但旧会话仍有效 → 攻击者继续访问，用户以为已经安全；
- 口令已改、会话已清、但新令牌签发失败 → 客户端收到错误以为改密失败，
  实际口令已经变了，用户用旧口令重试只会得到「当前口令不正确」，无从理解。

**④ 检查顺序：新口令长度 → 校验当前口令 → 新旧是否相同。**
当前口令必须**先于**「新旧相同」判定：若顺序反过来，用户把当前口令**输错**
且新口令恰好等于那个错误输入时，会收到「新旧相同」——
而真正的问题是当前口令不对，该提示会把人引向错误的修正方向。
由 `TestChangePasswordWrongCurrentBeatsSamePassword` 钉住。

**⑤ 状态码：当前口令不正确返回 400，刻意不用 401。**
客户端把 401 解释为「令牌失效」并清除登录态（`ApiException.isUnauthorized`）。
若这里回 401，用户仅仅输错一次当前口令就会被**登出**并弹回登录页 ——
而登录页正是他本想避免去的地方（他本来已经登录着）。
请求本身是已认证的，错的只是请求体里的一个字段。
由 `TestChangePasswordWrongCurrentIsNot401` 与前端
`当前口令错误不是 unauthorized` 两侧共同钉住。

**⑥ 拒绝「新旧相同」。** 否则接口返回成功却什么都没变，
用户以为换掉了泄露的口令而实际没有 —— 比直接报错危险得多。
明文直接比较即可（两者都在手上，且当前口令已在上一步验证过），无需再跑一次 bcrypt。

**⑦ 校验规则的 owner 划分。** 客户端只做服务端**无法**检查的两件事：
输入非空、两次新口令一致（确认框只存在于客户端）。
长度下限与「新旧相同」一律交服务端判断，客户端把服务端消息原样展示 ——
若在客户端再实现一遍长度规则，同一条规则就有两个 owner，日后必然漂移。
由 widget 测试
`口令长度规则由服务端判断，客户端不预先拦截` 钉住。

#### 验证

**真实 HTTP 端到端**（20 项检查全部通过，两个真实会话模拟两台设备）：

```
1. 当前口令错误        -> 400（且断言 **不是 401**）、消息含「当前口令」、
                          会话未被吊销、口令未被改动
2. 新旧相同            -> 400，消息含「相同」
3. 新口令过短          -> 400，消息含长度要求
4. 正常改密            -> 200，返回新令牌且与旧的不同
5. 会话轮换            -> 新令牌可用（设备 A 无感续用）
                          设备 A 旧令牌失效
                          **设备 B 的会话被吊销**（核心安全效果）
6. 口令确实换了        -> 旧口令 401、新口令 200
7. 未认证请求          -> 401，且未改动口令
8. 连续改密            -> 第二次改密成功，最终口令生效
```

**变异验证**：把事务里的 `DELETE FROM session` 替换为空操作后，
**4 项测试如实失败**（service 层 2 项、api 层 2 项）；文件 SHA-256 与还原前一致。
这证明「吊销会话」这一不变式确实被测试守护，而不是靠注释声称。

**测试计数**：后端新增 9 项（service）+ 8 项（api，含 3 个子用例）；
前端新增 8 项（状态与令牌替换）+ 5 项（页面）。

### 6.10 v0.0.5 发布与独立验证

`v0.0.5`（run #34616231075 之后的下一次，提交 d61286e）**六个 job 全部成功**。

**独立验证**（针对已发布产物本身，不读构建日志）：

从 ghcr 直读已发布镜像的 OCI 标签：

```
version  = 0.0.5
revision = d61286e7574503231d0528725aa5cc5b61253aaf   <- 与 tag 所指提交一致
source   = https://github.com/shinyes/cala
```

解包镜像层取出 `/cala` 二进制：

```
/cala  20.23 MB
内嵌版本 "0.0.5"          : true
含改密路由 "auth/password" : true      <- 本版新增端点确实在已发布产物中
```

下载 APK 核对：

```
大小 / SHA-256        : 与 GitHub 记录逐字一致（e1c1dd64…）
versionName           : 0.0.5
versionCode           : 5          <- 递增，可覆盖安装
INTERNET 权限         : 存在 ✓      <- v0.0.3 修复的回归检查
networkSecurityConfig : 存在 ✓      <- 同上
签名证书 SHA-256      : 4e232cd4…  <- 与 v0.0.1~v0.0.4 同一身份
```

> **一处检查方法的局限（记录以免误读）**：我曾尝试在二进制里搜索中文错误消息
> （如「当前口令不正确」）来佐证改密端点存在，结果为 false。
> 这是**检查方法**的问题而非镜像问题：Go 把字符串以 UTF-8 存储，
> 而我的过滤器只保留 ASCII 可打印字节，中文被滤掉。
> 该端点的实际行为已由后端 8 项 API 测试与 20 项真实 HTTP 端到端检查覆盖，
> 镜像层面则由路由字符串 `auth/password` 与内嵌版本号佐证。

`compose` 的 `image:` 已同步升到 `0.0.5`。

### 6.11 「再来一轮」漏实现，以及顺带发现的两个缺陷

用户指出「再来一轮的功能还未实现」。核实结果：**规格早有要求，是实现漏了** ——
规格 §9.1 的状态图写明

```
错题页 ──「重练错题」──▶ 以快照重开一轮（不再调用规则）
      ──「再来一轮」──▶ 同项目、新种子
```

而错题页只有「重练错题」与「返回」，**没有「再来一轮」**。

#### 缺陷 2（更严重）：总结页的「再来一轮」点了没有任何反应

写回归测试时发现：总结页**有**这个按钮，但它**静默失效**。

根因是导航结构：总结页由练习页 `pushReplacement` 换出，练习页**已被销毁**；
而按钮回调的是原练习页 State 上的 `_startRound()`，其开头即
`if (!mounted) return;` —— 于是它取回一轮题目后直接丢弃，界面上什么都不发生。

这类「静默失效」比「按钮缺失」更糟：按钮在那里、能点、有按下反馈，
用户只会以为是自己没点到或应用卡了。

**修复**：改为导航到一个**全新的** `PracticePage`（`pushNewRound` 辅助函数），
它自己会在 `initState` 里取新一轮题目，因此不依赖任何可能已销毁的 State。
用 `pushAndRemoveUntil(..., (r) => r.isFirst)` 清掉夹在中间的总结页与错题页 ——
否则从新一轮返回会回到上一轮的总结页，既无意义又容易误操作。

同时删除了 `SummaryPage.startNewRound` 这个回调参数：它正是缺陷的来源。

#### 缺陷 3：矮屏上答错时布局溢出，被裁掉的正是「下一题」按钮

写测试驱动「答错 → 下一题」时，测试框架报出：

```
A RenderFlex overflowed by 54 pixels on the bottom.
  creator: Column ← Padding ← _QuestionArea …
  constraints: BoxConstraints(0.0<=w<=760.0, h=195.0)
  size: Size(760.0, 195.0)
```

答错时会额外出现「答错了 / 正确答案 / 下一题」，内容明显变高；
键盘固定占约 286px，屏幕一矮，题目区就放不下 ——
而**被裁掉的恰是用户此刻唯一能推进的操作**。

**修复**：题目区改为可滚动（`LayoutBuilder` + `ConstrainedBox(minHeight)`
包 `SingleChildScrollView`）。用 `ConstrainedBox` 而非单纯滚动，是为了
空间充足时仍保持垂直居中，空间不足时才退化为滚动。

**变异验证（两次，第一次的结论本身有价值）**：

1. 往题目区塞 400px 额外内容 → 守卫**仍全通过**。
   这不是守卫失效，而是说明**滚动把内容增长吸收成了滚动**，
   溢出在结构上已不可能发生 —— 正是修复的目的。
2. 因此改测「回退修复本身」：还原为不可滚动的旧布局 → 守卫如实失败：

```
画布高 844px：通过      <- 内容本来就放得下
画布高 700px：通过
画布高 640px：**失败**
画布高 600px：**失败**
画布高 560px：**失败**
画布高 480px：**失败**
```

这既证明溢出真实存在，也给出了精确边界：**旧布局在画布高 ≤640px 时溢出**；
并证明守卫能捕获回退。文件 SHA-256 与还原前一致。

#### 另外两处「测试自身的教训」（记录以免误读）

- 首轮测试失败于「点确定后找不到『下一题』」——那是**测试写错了**：
  答对会在 220ms 后自动推进、不出现该按钮（功能 9 只要求答错时停住）。
  已改为如实反映两种路径，而不是绕过它。
- `find.textContaining('错题')` 在真实机型尺寸下匹配到两处
  （「错题 1 道」与「查看错题、重练错题」）而导致 tap 歧义。已改用精确文案。
- 测试画布原先沿用 flutter_test 默认的 800×600 —— 比手机**矮**得多，
  于是测的其实是矮屏分支。已改为真实机型尺寸 390×844，
  矮屏不变量另由专门的分档测试守护。

#### 验证

- 新增 9 项测试：3 项「再来一轮」行为 + 6 项矮屏不溢出分档守卫
- 前端 analyze 无问题、**188 项**全部通过（原 179）
- 缺陷 2 的回归测试在修复前确实失败（`Expected: <2>, Actual: <1>`），修复后通过
- 渲染确认错题页底部为「再来一轮」（主按钮）+「重练错题」+「返回」

### 6.12 v0.0.6 发布与独立验证

`v0.0.6`（run #13，提交 dbbef5a）**六个 job 全部成功**。

**独立验证**（针对已发布产物本身）：

从 ghcr 直读镜像 OCI 标签：

```
version  = 0.0.6
revision = dbbef5a31654f219621806d28d54eb61ffb689ec   <- 与 tag 所指提交一致
```

解包镜像层取出 `/cala` 二进制：

```
/cala  20.23 MB   内嵌 "0.0.6": true
```

下载 APK 核对：

```
大小 / SHA-256        : 与 GitHub 记录逐字一致（80b5706a…）
versionName           : 0.0.6
versionCode           : 6          <- 递增，可覆盖安装
INTERNET 权限         : 存在 ✓      <- v0.0.3 修复的回归检查
networkSecurityConfig : 存在 ✓      <- 同上
签名证书 SHA-256      : 4e232cd4…  <- 与 v0.0.1~v0.0.5 同一身份
```

`compose` 的 `image:` 已同步升到 `0.0.6`。

> 发布记录至此连续六版（v0.0.1~v0.0.6）签名身份一致、
> `versionCode` 严格递增，因此每一版都可直接覆盖安装上一版。

### 6.13 网络环境对推送的影响

第 3 轮的推送一度全部失败：

```
fatal: unable to access 'https://github.com/shinyes/cala.git/':
  schannel: failed to receive handshake, SSL/TLS connection failed
```

诊断结果：git 配置了本地代理 `http://127.0.0.1:7897`（clash-verge / verge-mihomo
在运行且端口监听），但**代理的海外节点失效**——
经代理访问 google / github / api.github.com 全部握手失败，而 baidu 正常
（Clash 按规则把国内流量直连）。直连 github.com 亦失败。

**绕行方案**：`github.com:22` 与 `ssh.github.com:443` 直连可达，且本机
`~/.ssh/id_rsa` 已注册到 GitHub（实测 `Hi shinyes! You've successfully authenticated`）。
因此改用 SSH 地址推送，**未修改用户既有的 `origin` 配置**（只对本次推送指定 SSH URL），
以最小侵入方式解除阻塞。

> 该绕行是环境层面的应对，不是项目配置的变更。若 HTTPS 代理恢复，现有
> `origin` 仍可正常工作。

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
