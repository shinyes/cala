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
> §6.5（网络环境对推送的影响）。

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

### 6.5 网络环境对推送的影响

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
