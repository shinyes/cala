# 实施计划 —— P7 CI 与交付

- Date: `2026-09-10`
- Design Spec: [`docs/aegis/specs/2026-09-10-mental-math-app-design.md`](../specs/2026-09-10-mental-math-app-design.md) §10
- Baseline: [`docs/aegis/baseline/2026-09-10-initial-baseline.md`](../baseline/2026-09-10-initial-baseline.md)
- 上游：[`evidence/p6/ACCEPTANCE.md`](../work/2026-09-10-mental-math-app/evidence/p6/ACCEPTANCE.md)
- 覆盖阶段：规格 §16 的 **P7 CI 与交付**（**最后一个阶段**）
- 出口条件：**A10**（tag 触发、镜像推 ghcr、release 附 tar.gz）与 **A15**（已签名 APK）

---

## Goal

打一个 `v*` tag，自动产出两样可分发的产物：

1. **Docker 镜像** —— 推送到 `ghcr.io/shinyes/cala`，并以 `.tar.gz`（`docker save`）作为
   Release 附件（功能 10 原文要求「镜像文件上传到 release」）
2. **已签名的 release APK** —— 作为 Release 附件（D17/D18，用户明确要求发布 APK）

`ci.yml` 已在 P3.5 交付，本阶段只补 `release.yml`、Android 正式签名配置，
以及两者的本地可验证性。

## Architecture

```
.github/workflows/
  ci.yml              已有（P3.5）：build / vet / gofmt / go test / flutter analyze / test
  release.yml         新增：tag v* 触发

app/android/
  app/build.gradle.kts   改为从 key.properties 读取正式签名
  key.properties         **不入库**（由 CI 或本地生成）
backend/
  Dockerfile            已有（P0.6）；本阶段首次真实验证
  .dockerignore         已有
```

## Tech Stack

无新增依赖。

## Baseline / Authority Refs

- 规格 §10.1（两个工作流的触发与行为）
- 规格 §10.2（APK 纳入交付的理由）
- 规格 §10.3（**通用 APK 而非 split-per-abi；必须用正式 keystore**；
  keystore 遗失将导致已安装用户无法升级）
- 规格 D17/D18/D19/D20（APK 交付、正式签名、仓库地址、`cc.lcyk.cala`）
- 基线 §5.3、§9

## Compatibility Boundary

1. **keystore 与口令绝不入库**。`key.properties` 与 `*.jks`/`*.keystore`
   必须加入 `.gitignore`。这是不可协商项。
2. **不使用 debug 签名**（D18）。当前 `build.gradle.kts` 第 30-32 行签的是 debug，
   必须替换。debug 签名有效期短，且更换 keystore 会导致无法覆盖安装升级。
3. **applicationId 不得变更**（`cc.lcyk.cala`，D20）——发布后更改等于换了一个 App。
4. `ci.yml` 的既有行为不得削弱（语料门禁是跨端一致性的唯一防线）。
5. 不得把镜像构建挪出 CI（本机无 Docker，见规格 §14 V2）。

## 已知前置条件（用户已提供）

| 项 | 状态 |
|---|---|
| GitHub 仓库 `shinyes/cala` | ✅ 已创建，`main` 已推送 |
| Repository secrets（4 个） | ✅ 用户已设置**仓库级** secrets |
| `ANDROID_KEYSTORE_BASE64` | ✅ |
| `ANDROID_KEYSTORE_PASSWORD` | ✅ |
| `ANDROID_KEY_ALIAS` | ✅ |
| `ANDROID_KEY_PASSWORD` | ✅ |

> **仓库级**意味着 job 无需声明 `environment:`，secret 在所有 job 中可见。
> 若日后改为环境级，必须给 job 加 `environment:`，否则 secret 会解析为空字符串。

## TDD Route

```text
TDD Route:
- Mode: off
- Decision: skipped
- Strict authority: not applicable
- Test posture: post-change regression + 本地等效验证
- Reason: 项目配置 tdd_mode = "off"；且本阶段的产物是 CI 配置与构建脚本，
  没有「单元测试」这一层，验证方式是本地等效执行 + 静态校验
- Verification: 见下方 Verification
```

## Verification

本阶段的难点：**本机没有 Docker，也没有真实 keystore**。
因此不能靠「跑一遍 CI」来验证，必须把每一项拆成**本地可执行的等效验证**：

| 目标 | 本地等效验证 |
|---|---|
| Dockerfile 能构建 | `CGO_ENABLED=0 GOOS=linux go build` 交叉编译（P0.6 已做，本阶段复做）+ 静态检查 Dockerfile 指令 |
| 镜像能运行 | **无法本地验证**（无 Docker）→ 如实记录 |
| ghcr 推送 | **无法本地验证**（需 CI 与凭据）→ 如实记录 |
| APK 签名配置正确 | ⭐ **用临时 keystore 本地真实构建一次 release APK 并验签** |
| release.yml 语法正确 | 用 Dart 的 yaml 包解析 + 逐项检查引用的文件存在 |
| Shell 步骤正确 | 用 `bash -n`（Git Bash 或 WSL 若可用）做语法检查 |

第 4 项是关键：JDK 17 已安装（`D:\Programs\jdk-17`），可以用 `keytool` 生成一个
**一次性的测试 keystore**，走完与 CI 完全相同的配置路径，产出真实签名 APK，
再用 `apksigner`/`keytool` 验证签名。这样 CI 里唯一不确定的就只剩「用户的 keystore
格式是否正确」——而那是 preflight 校验要负责明确报错的。

```powershell
cd D:\Desktop\Cala\backend ; go build ./... ; go vet ./... ; go test ./... -count=1
cd D:\Desktop\Cala\app     ; flutter analyze ; flutter test
```

---

## Scope Check

**Aegis Visibility**：本阶段交付的是**发布产物**。它有两个特点：一是错误代价高
（签名错误的 APK 无法覆盖升级；keystore 泄露则是严重安全事故），二是**本机无法
完整验证**。因此策略是：把能本地验证的部分**全部真实验证**（尤其是签名配置，
用临时 keystore 走完整路径），把不能验证的部分**明确标注为未验证**，
并在工作流里加 **preflight 校验**——让缺失的 secret 以可读信息失败，
而不是以晦涩的 keytool 错误失败。

```text
Requirement Ready Check:
- Requirement source refs: 用户功能 10 + 后续要求「需要发布 apk」；规格 §10
- Goals and scope refs: 规格 §16 P7 行
- User / scenario refs: 自部署者拉镜像；使用者下载 APK 侧载
- Requirement item refs: 功能 10（tag 触发构建镜像并上传 release）、D17/D18（APK 与正式签名）
- Acceptance / verification criteria refs: A10、A15
- Open blocker questions: 无（secrets 层级已确认为仓库级）
- Decision: ready
```

```text
Change Necessity:
- User-visible need: 目前没有发布流程，也没有可分发的 APK
- No-change / non-code option: 不存在——功能 10 与 APK 交付都需要配置与脚本
- Why code change is necessary: 需要改 Android 签名配置并新增工作流
- Minimum change boundary: 新增 .github/workflows/release.yml、
  修改 app/android/app/build.gradle.kts 的 signingConfigs、扩充 .gitignore；
  不改动任何既有业务代码
- Decision: code-change
```

```text
Existence Check:
- Proposed new surface: ①release.yml ②Gradle 的 release signingConfig ③preflight 校验步骤
- Existing owner / reuse candidate: ci.yml 已存在但职责不同（校验 vs 发布）；
  Dockerfile 已存在（P0.6）
- Why existing surface is insufficient:
  ① ci.yml 不产出产物；
  ② 当前 release 用的是 debug 签名（D18 禁止）；
  ③ 缺少 secret 时若不显式校验，失败信息会指向 keytool 而非「缺少哪个 secret」
- Creation proof: 规格 §10.1/§10.3 明确要求
- Entropy / retirement impact: 无旧路径退役。**明确拒绝**：不做多架构镜像
  （arm64 需 QEMU，构建时间翻倍而目标场景是 x86 服务器）、不做 split-per-abi
  （§10.3）、不做 APK 自动更新机制
- Decision: add-with-proof
```

```text
Architecture Integrity Lens:
- Invariant: keystore 与口令绝不入库；applicationId 不得变更；不使用 debug 签名
- Canonical owner / contract: release.yml 是发布流程的唯一 owner；
  Gradle 的 signingConfigs.release 是签名的唯一实现点
- Responsibility overlap: ci.yml 与 release.yml 职责分离（前者校验、后者发布），
  release.yml **不重复** ci.yml 的测试步骤——它信任 tag 已经过 CI；
  但**会在构建前跑一次 go test 与 flutter test**（因为 tag 可能打在未过 CI 的提交上）
- Higher-level simplification: 用一个 `preflight` job 集中校验所有前置条件，
  使后续 job 的失败都指向真实构建问题而非缺失配置
- Retirement / falsifier: 若有人把口令写进 gradle 文件或提交 key.properties，
  → 由 .gitignore + 工作流里「断言 key.properties 不入库」的检查防住
- Verdict: 无阻塞问题
```

```text
Plan Pressure Test:
- Owner / contract / retirement: 承载点已识别（签名、secret、镜像分发）
- Architecture integrity / higher-level path: preflight job 集中校验
- Verification scope: 签名用临时 keystore 真实验证；其余明确标注未验证
- Task executability: 每步含完整配置与确切命令
- Pressure result: proceed
```

```text
Plan-Time Complexity Check:
- Target files: release.yml（新增）、build.gradle.kts（局部修改）、.gitignore（追加）
- Existing size / shape signals: build.gradle.kts 45 行，改动仅限 buildTypes 与新增 signingConfigs
- Owner fit: 签名配置属于 app 模块的构建配置，位置正确
- Add-in-place risk: 工作流若把「校验 + 构建镜像 + 构建 APK + 发布」全塞进一个 job，
  失败定位困难且无法并行
- Better file boundary: preflight（校验）→ build-image 与 build-apk（并行）→ release（汇总发布）
- Recommendation: add owner file + split task
```

---

## Task P7.1 —— Android 正式签名配置

**Files**
- Modify: `app/android/app/build.gradle.kts`
- Modify: `app/.gitignore`
- Create: `app/android/key.properties.example`

**Why**：D18 要求正式签名；当前配置签的是 debug（第 30-32 行）。

**Change Necessity**：`code-change`。

**Impact / Compatibility**：承载规格 §10.3 与 D18。**签名方式一旦发布即不可更改**
（换 keystore 会导致已安装用户无法覆盖升级）。

**Steps**

1. 修改 `app/android/app/build.gradle.kts`：

```kotlin
import java.util.Properties
import java.io.FileInputStream

plugins {
    id("com.android.application")
    id("dev.flutter.flutter-gradle-plugin")
}

// 从 key.properties 读取正式签名配置。
//
// 文件缺失时**不报错**，而是回退到 debug 签名 —— 这样 `flutter run --release`
// 与本地调试仍可用。CI 中由 release.yml 的 preflight 步骤保证 key.properties
// 一定存在，因此正式发布绝不会落到这个回退分支上。
//
// 注意：回退到 debug 签名只用于本地便利，绝不可用于发布（D18）。
val keystoreProperties = Properties()
val keystorePropertiesFile = rootProject.file("key.properties")
val hasReleaseKeystore = keystorePropertiesFile.exists()
if (hasReleaseKeystore) {
    keystoreProperties.load(FileInputStream(keystorePropertiesFile))
}

android {
    namespace = "cc.lcyk.cala"
    compileSdk = flutter.compileSdkVersion
    ndkVersion = flutter.ndkVersion

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    defaultConfig {
        applicationId = "cc.lcyk.cala"
        minSdk = flutter.minSdkVersion
        targetSdk = flutter.targetSdkVersion
        versionCode = flutter.versionCode
        versionName = flutter.versionName
    }

    signingConfigs {
        if (hasReleaseKeystore) {
            create("release") {
                keyAlias = keystoreProperties["keyAlias"] as String
                keyPassword = keystoreProperties["keyPassword"] as String
                storeFile = keystoreProperties["storeFile"]?.let { file(it) }
                storePassword = keystoreProperties["storePassword"] as String
            }
        }
    }

    buildTypes {
        release {
            signingConfig = if (hasReleaseKeystore) {
                signingConfigs.getByName("release")
            } else {
                // 仅本地便利；发布路径由 CI preflight 保证不会走到这里
                signingConfigs.getByName("debug")
            }
            // 不启用 minify：本项目无反射依赖，且 minify 会让崩溃栈难以阅读；
            // 体积收益（Flutter 应用主要体积在引擎）不足以抵消调试成本。
            isMinifyEnabled = false
            isShrinkResources = false
        }
    }
}

kotlin {
    compilerOptions {
        jvmTarget = org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17
    }
}

flutter {
    source = "../.."
}
```

2. 追加到 `app/.gitignore`：

```
# 正式签名材料：绝不入库
key.properties
*.jks
*.keystore
```

3. 创建 `app/android/key.properties.example`（**示例文件入库**，便于他人自建）：

```properties
# 复制为 android/key.properties 后填入真实值。
# 该文件已被 .gitignore 排除，不会入库。
#
# 生成 keystore：
#   keytool -genkey -v -keystore ~/cala-release.jks -keyalg RSA -keysize 2048 \
#     -validity 10000 -alias cala
#
# 转成 base64（用于 CI 的 ANDROID_KEYSTORE_BASE64）：
#   PowerShell: [Convert]::ToBase64String([IO.File]::ReadAllBytes("cala-release.jks"))
#   注意：必须是单行、不含换行的 base64。不要用 certutil（它会加 header 并折行）。
storePassword=你的 keystore 口令
keyPassword=你的密钥口令
keyAlias=cala
storeFile=/绝对路径/cala-release.jks
```

4. **本地真实验证（本任务的核心）**：

```powershell
# 用一次性测试 keystore 走完整签名路径
$tmp = "$env:TEMP\cala-signing-test"
Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $tmp | Out-Null

& "D:\Programs\jdk-17\bin\keytool.exe" -genkeypair -v `
  -keystore "$tmp\test.jks" -storetype JKS `
  -keyalg RSA -keysize 2048 -validity 10000 -alias calatest `
  -storepass testpass123 -keypass testpass123 `
  -dname "CN=Cala Test, OU=Dev, O=Cala, L=X, S=X, C=CN"

# 写入 key.properties（指向测试 keystore）
@"
storePassword=testpass123
keyPassword=testpass123
keyAlias=calatest
storeFile=$($tmp -replace '\\','/')/test.jks
"@ | Set-Content -Path "D:\Desktop\Cala\app\android\key.properties" -Encoding utf8

# 构建 release APK（必须用 --release，否则不会走 signingConfigs.release）
cd D:\Desktop\Cala\app
flutter build apk --release --build-name=0.0.1 --build-number=1
```

5. **验签**（证明用的是我们的 keystore 而非 debug key）：

```powershell
& "D:\Programs\Android\Sdk\build-tools\36.0.0\apksigner.bat" verify --print-certs `
  "D:\Desktop\Cala\app\build\app\outputs\flutter-apk\app-release.apk"
```

期望：证书 CN 为 `Cala Test`（而非 `Android Debug`）。

6. **清理**：删除测试 keystore 与 `key.properties`（不能留在工作区）。

7. 确认 `git status` 中**没有** `key.properties`：
   `git check-ignore -v app/android/key.properties` 应命中。

8. **Commit**：`feat(app): Android 正式签名配置（从 key.properties 读取）`

---

## Task P7.2 —— release.yml

**Files**
- Create: `.github/workflows/release.yml`

**Why**：功能 10 与 APK 交付。

**Change Necessity**：`code-change`。

**Steps**

1. 写 `.github/workflows/release.yml`：

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write   # 创建 Release 并上传附件
  packages: write   # 推送 ghcr.io

env:
  IMAGE: ghcr.io/${{ github.repository }}

jobs:
  # 前置校验：把所有「配置缺失」类问题集中在这里失败，
  # 并给出可读信息。否则它们会在后续 job 里以晦涩的 keytool / gradle 错误暴露。
  preflight:
    name: Preflight
    runs-on: ubuntu-latest
    outputs:
      version: ${{ steps.version.outputs.version }}
      version_code: ${{ steps.version.outputs.version_code }}
    steps:
      - uses: actions/checkout@v4

      - name: 从 tag 推导版本号
        id: version
        run: |
          set -euo pipefail
          tag="${GITHUB_REF_NAME}"
          # v1.2.3 -> 1.2.3
          version="${tag#v}"
          if ! printf '%s' "$version" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+'; then
            echo "::error::tag '$tag' 不是 v<major>.<minor>.<patch> 形式"
            exit 1
          fi
          # versionCode 用 major*1000000 + minor*1000 + patch，保证单调递增
          IFS='.' read -r major minor patch <<< "${version%%-*}"
          code=$(( major * 1000000 + minor * 1000 + patch ))
          echo "version=$version" >> "$GITHUB_OUTPUT"
          echo "version_code=$code" >> "$GITHUB_OUTPUT"
          echo "版本: $version (code=$code)"

      - name: 校验签名所需的 4 个 secret 均已配置
        env:
          KS_B64: ${{ secrets.ANDROID_KEYSTORE_BASE64 }}
          KS_PASS: ${{ secrets.ANDROID_KEYSTORE_PASSWORD }}
          KEY_ALIAS: ${{ secrets.ANDROID_KEY_ALIAS }}
          KEY_PASS: ${{ secrets.ANDROID_KEY_PASSWORD }}
        run: |
          set -euo pipefail
          missing=""
          [ -n "${KS_B64:-}" ]    || missing="$missing ANDROID_KEYSTORE_BASE64"
          [ -n "${KS_PASS:-}" ]   || missing="$missing ANDROID_KEYSTORE_PASSWORD"
          [ -n "${KEY_ALIAS:-}" ] || missing="$missing ANDROID_KEY_ALIAS"
          [ -n "${KEY_PASS:-}" ]  || missing="$missing ANDROID_KEY_PASSWORD"
          if [ -n "$missing" ]; then
            echo "::error::缺少以下 repository secret:$missing"
            echo "请在 Settings > Secrets and variables > Actions 中配置（需为仓库级）。"
            exit 1
          fi
          echo "4 个签名 secret 均已配置。"

      - name: 校验 keystore base64 可解码且是合法 JKS
        env:
          KS_B64: ${{ secrets.ANDROID_KEYSTORE_BASE64 }}
        run: |
          set -euo pipefail
          # 去掉可能的换行与空白（有人用 certutil 生成过就会带 header 与折行）
          printf '%s' "$KS_B64" | tr -d ' \t\r\n' | base64 -d > /tmp/release.jks \
            || { echo "::error::ANDROID_KEYSTORE_BASE64 不是合法 base64"; exit 1; }
          size=$(stat -c%s /tmp/release.jks)
          if [ "$size" -lt 100 ]; then
            echo "::error::解码后的 keystore 仅 $size 字节，疑似内容不正确"
            exit 1
          fi
          # JKS 的魔数，或 PKCS12 的 ASN.1 起始；两者都接受
          if ! keytool -list -keystore /tmp/release.jks \
                 -storepass "${{ secrets.ANDROID_KEYSTORE_PASSWORD }}" > /dev/null 2>&1; then
            echo "::error::keystore 无法打开：口令错误或文件损坏"
            exit 1
          fi
          echo "keystore 校验通过（$size 字节）"

      - name: 断言签名材料未被提交入库
        run: |
          set -euo pipefail
          if git ls-files --error-unmatch app/android/key.properties 2>/dev/null; then
            echo "::error::app/android/key.properties 已被提交入库，必须立即移除并更换 keystore"
            exit 1
          fi
          if git ls-files | grep -Ei '\.(jks|keystore)$'; then
            echo "::error::仓库中存在 keystore 文件，必须立即移除并更换"
            exit 1
          fi
          echo "签名材料未入库。"

  test:
    name: Test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: backend/go.mod
          cache-dependency-path: backend/go.sum

      - uses: subosito/flutter-action@v2
        with:
          flutter-version: '3.44.9'
          channel: stable
          cache: true

      # tag 可能打在未经 CI 的提交上，因此发布前必须自己跑一遍测试。
      # 跨端判分语料门禁包含在内（13872 条），这是 A12 的唯一防线。
      - name: Backend tests
        working-directory: backend
        env:
          CGO_ENABLED: '0'
        run: go test ./... -count=1 -timeout 600s

      - name: Frontend tests
        working-directory: app
        run: |
          flutter pub get
          flutter analyze
          flutter test

  image:
    name: Docker image
    runs-on: ubuntu-latest
    needs: [preflight, test]
    steps:
      - uses: actions/checkout@v4

      - uses: docker/setup-buildx-action@v3

      - name: 登录 ghcr.io
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      # 只构建 amd64：目标场景是 x86 服务器，arm64 需要 QEMU，
      # 构建时间翻倍而收益不明。若日后需要，加 platforms 即可。
      - name: 构建并推送
        uses: docker/build-push-action@v6
        with:
          context: ./backend
          push: true
          platforms: linux/amd64
          tags: |
            ${{ env.IMAGE }}:${{ needs.preflight.outputs.version }}
            ${{ env.IMAGE }}:latest
            ${{ env.IMAGE }}:sha-${{ github.sha }}
          cache-from: type=gha
          cache-to: type=gha,mode=max

      # 功能 10 原文要求「镜像文件上传到 release」，故同时产出 tar.gz 附件
      - name: 导出镜像为 tar.gz
        run: |
          set -euo pipefail
          version="${{ needs.preflight.outputs.version }}"
          docker pull "${IMAGE}:${version}"
          docker save "${IMAGE}:${version}" | gzip -9 > "cala-${version}-linux-amd64.tar.gz"
          ls -lh "cala-${version}-linux-amd64.tar.gz"

      - uses: actions/upload-artifact@v4
        with:
          name: docker-image
          path: cala-*-linux-amd64.tar.gz
          retention-days: 7

  apk:
    name: Signed APK
    runs-on: ubuntu-latest
    needs: [preflight, test]
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-java@v4
        with:
          distribution: temurin
          java-version: '17'

      - uses: subosito/flutter-action@v2
        with:
          flutter-version: '3.44.9'
          channel: stable
          cache: true

      - name: 解码 keystore 并生成 key.properties
        env:
          KS_B64: ${{ secrets.ANDROID_KEYSTORE_BASE64 }}
        run: |
          set -euo pipefail
          # base64 可能带换行（有人用 certutil 生成就会有），先清理
          printf '%s' "$KS_B64" | tr -d ' \t\r\n' | base64 -d > "$RUNNER_TEMP/release.jks"
          cat > app/android/key.properties <<EOF
          storePassword=${{ secrets.ANDROID_KEYSTORE_PASSWORD }}
          keyPassword=${{ secrets.ANDROID_KEY_PASSWORD }}
          keyAlias=${{ secrets.ANDROID_KEY_ALIAS }}
          storeFile=$RUNNER_TEMP/release.jks
          EOF
          # 不打印内容（含口令），只确认文件已就位
          test -s app/android/key.properties && echo "key.properties 已生成"
          test -s "$RUNNER_TEMP/release.jks" && echo "keystore 已就位"

      - name: 构建 release APK
        working-directory: app
        run: |
          set -euo pipefail
          flutter pub get
          # 通用 APK（非 split-per-abi）：侧载场景下「下载哪个文件」的选错风险
          # 大于十几 MB 的体积收益（规格 §10.3）
          flutter build apk --release \
            --build-name="${{ needs.preflight.outputs.version }}" \
            --build-number="${{ needs.preflight.outputs.version_code }}"

      - name: 验签并断言不是 debug 证书
        run: |
          set -euo pipefail
          apk="app/build/app/outputs/flutter-apk/app-release.apk"
          test -f "$apk"
          sdk_root="${ANDROID_SDK_ROOT:-${ANDROID_HOME:-}}"
          apksigner="$(find "$sdk_root/build-tools" -name apksigner -type f | sort -V | tail -1)"
          echo "使用 $apksigner"
          "$apksigner" verify --print-certs "$apk" | tee /tmp/certs.txt
          if grep -qi 'Android Debug' /tmp/certs.txt; then
            echo "::error::APK 使用了 debug 证书，绝不可发布（规格 D18）"
            exit 1
          fi
          "$apksigner" verify "$apk"
          echo "签名校验通过，且不是 debug 证书。"

      - name: 重命名产物
        run: |
          set -euo pipefail
          version="${{ needs.preflight.outputs.version }}"
          mkdir -p out
          cp app/build/app/outputs/flutter-apk/app-release.apk \
             "out/cala-${version}.apk"
          ls -lh out/

      - uses: actions/upload-artifact@v4
        with:
          name: apk
          path: out/*.apk
          retention-days: 7

      - name: 清理签名材料
        if: always()
        run: rm -f app/android/key.properties "$RUNNER_TEMP/release.jks"

  release:
    name: GitHub Release
    runs-on: ubuntu-latest
    needs: [preflight, image, apk]
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/download-artifact@v4
        with:
          path: dist
          merge-multiple: true

      - name: 列出待发布文件
        run: find dist -type f | sort

      - name: 生成 Release 说明
        run: |
          set -euo pipefail
          version="${{ needs.preflight.outputs.version }}"
          cat > notes.md <<EOF
          ## Cala ${version}

          ### 部署（Docker）

          \`\`\`bash
          docker pull ${IMAGE}:${version}
          docker run -d --name cala -p 8080:8080 \\
            -v cala-data:/data \\
            -e CALA_REGISTRATION_OPEN=true \\
            ${IMAGE}:${version}
          \`\`\`

          首次启动后打开 http://localhost:8080 注册：**第一个注册的用户会成为管理员**。
          注册完成后建议在「我的」中关闭注册开关。

          离线环境可用附件里的 \`cala-${version}-linux-amd64.tar.gz\`：
          \`\`\`bash
          docker load < cala-${version}-linux-amd64.tar.gz
          \`\`\`

          ### Android

          下载 \`cala-${version}.apk\` 侧载安装（已用正式密钥签名）。

          ### 说明

          - 镜像仅 linux/amd64
          - APK 为通用包（含全部 ABI），未按 ABI 拆分
          EOF
          cat notes.md

      - name: 创建 Release 并上传附件
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          set -euo pipefail
          version="${{ needs.preflight.outputs.version }}"
          gh release create "v${version}" \
            --title "Cala ${version}" \
            --notes-file notes.md \
            dist/cala-*.tar.gz dist/cala-*.apk
```

2. **本地静态验证**：

```powershell
# YAML 可解析
dart run <yaml 校验脚本> .github/workflows/release.yml

# 引用的文件都存在
Test-Path backend/Dockerfile, backend/.dockerignore, app/pubspec.yaml

# shell 片段语法（若有 bash）
bash -n <每个 run 块>
```

3. **Commit**：`ci: tag 触发的 release 工作流（ghcr 镜像 + 签名 APK）`

---

## Task P7.3 —— 本地等效验证与 no-Docker 说明

**Files**：无新增

**Why**：本机无 Docker，必须把可验证的部分验证到位，并明确记录不可验证的部分。

**Steps**

1. 复做 Linux 静态交叉编译（等价于 Dockerfile 的构建阶段）：

```powershell
cd D:\Desktop\Cala\backend
$env:CGO_ENABLED="0"; $env:GOOS="linux"; $env:GOARCH="amd64"
go build -trimpath -ldflags="-s -w" -o $env:TEMP\cala-linux ./cmd/server
```

断言：exit 0、产出二进制、无动态库引用。

2. 用 `bash -n` 检查工作流里的每个 `run` 片段（若 Git Bash 不可用，则逐段人工核对
   并如实记录）。

3. 用 Dart 的 yaml 包解析 `release.yml` 与 `ci.yml`，确认结构合法、
   job 依赖关系可解析。

4. 检查 `release.yml` 引用的所有路径与文件均存在。

5. **Commit**：无（验证步骤，结果写入证据）

---

## Task P7.4 —— P7 验收

**Files**：无新增（验证与证据）

1. 全量验证并写入
   `docs/aegis/work/2026-09-10-mental-math-app/evidence/p7/ACCEPTANCE.md`：

| 标准 | 证明方式 | 本地可否验证 |
|---|---|---|
| A15 release 产出**已签名** APK 且可覆盖升级 | 用临时 keystore 构建 release APK 并验签 | ✅ **本地真实验证** |
| A15 不是 debug 证书 | `apksigner verify --print-certs` 证书 CN | ✅ |
| A10 tag 触发构建 | release.yml 的 `on.push.tags` | ✅ 静态 |
| A10 镜像推 ghcr | 工作流 step | ❌ 需 CI |
| A10 release 附 tar.gz | 工作流 step | ❌ 需 CI |
| Dockerfile 可构建 | Linux 交叉编译（等价验证） | ⚠️ 部分 |
| 镜像 tag 存在性（`golang:1.26-alpine` 等） | —— | ❌ 需 CI |

2. **手工检查清单**（供用户执行）：
   - 在 Actions 页确认 `ci.yml` 首跑通过
   - 打一个测试 tag（如 `v0.0.1`）并观察 `release.yml`
   - 确认 Release 页出现 `.tar.gz` 与 `.apk` 两个附件
   - 下载 APK 安装，确认可覆盖安装（若已有旧版本）

3. 更新工作记录并提交。

4. **Commit**：`test(release): P7 验收记录`

---

## Risks

| # | 风险 | 处置 |
|---|---|---|
| R-P7-1 | 用 debug 签名发布 | preflight 之后 APK job 里 `apksigner` 断言证书不是 `Android Debug`，是则失败 |
| R-P7-2 | keystore 或口令入库 | `.gitignore` + preflight 里 `git ls-files` 断言；已入库则明确要求更换 |
| R-P7-3 | base64 带换行/header 导致解码失败 | preflight 与 APK job 都先 `tr -d ' \t\r\n'`；preflight 还会用 keytool 试开 |
| R-P7-4 | secret 缺失导致晦涩失败 | preflight 集中校验并**报出缺哪个** |
| R-P7-5 | `golang:1.26-alpine` 或 distroless tag 不存在 | ~~无法本地验证（P-R6）~~ → **收尾时已实际核实两者均存在**，见 evidence/p7 §6.1 |
| R-P7-6 | versionCode 不单调导致无法覆盖升级 | 由 tag 推导：`major*1e6 + minor*1e3 + patch`，天然单调 |
| R-P7-7 | tag 打在未过 CI 的提交上 | release.yml 自带 test job（含跨端语料门禁） |
| R-P7-8 | 本机无 Docker，镜像无法本地验证 | **如实记录为未验证**，不宣称已验证 |

## Retirement

- **Old owner / fallback**：`build.gradle.kts` 中「release 用 debug 签名」这一临时配置
  在本阶段被**替换**（不是新增分支）。保留 `key.properties` 缺失时的 debug 回退，
  理由是本地 `flutter run --release` 需要它，且 CI preflight 保证发布路径不会走到那里。
- **Deletion trigger**：无。
- 明确拒绝：多架构镜像、split-per-abi、APK 自动更新、镜像签名（cosign）。

## ADR Signals

P7 不产生新 ADR。它是规格 §10 的落地。

---

## Execution Readiness View

```text
Execution Readiness View:
- Intent Lock: 交付 tag 触发的发布流程，产出 ghcr 镜像 + tar.gz + 已签名 APK
- Scope Fence: 不含多架构镜像、split-per-abi、自动更新、镜像签名
- Baseline Lock: 规格 §10.1/§10.2/§10.3、D17/D18/D19/D20；基线 §5.3
- Approved Behavior: keystore 与口令绝不入库；使用正式签名；applicationId 不变；
  通用 APK；镜像仅 amd64
- Owner / Contract Constraints: release.yml 是发布流程唯一 owner；
  Gradle signingConfigs.release 是签名唯一实现点
- Compatibility Boundary: ci.yml 行为不得削弱；applicationId 不得变更
- Retirement Boundary: 替换「release 用 debug 签名」这一临时配置；保留本地回退
- Task Batches: P7.1(签名) → P7.2(release.yml) → P7.3(本地等效验证) → P7.4(验收)
- Test Obligations: 临时 keystore 真实构建 + 验签；YAML 解析；路径存在性；
  Linux 交叉编译；shell 片段语法
- Review Gates: P7.1 必须产出真实签名 APK 并通过验签；P7.4 逐条核对 A10/A15
- Drift / Rewind Rules: 若发现需要把口令写入仓库或改 applicationId，停止并回到规格
- Evidence Required Before Completion: 验签输出（证书 CN）；YAML 解析结果；
  交叉编译结果；**明确列出无法本地验证的项**
- Advisory Boundary: method-pack execution guidance only; not GateDecision,
  PolicySnapshot, or completion authority
```

## Execution Route

```text
Execution Route:
- Decision: inline
- Evidence: P7.1（签名配置）是 P7.2（APK job）的前提；P7.3/P7.4 依赖前两者
- Fallback: 无
- User confirmation required: no
```

**REQUIRED SUB-SKILL**：aegis:executing-plans
