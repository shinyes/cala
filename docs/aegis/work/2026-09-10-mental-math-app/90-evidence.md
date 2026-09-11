# 速算练习 App（Go+Fiber+SQLite 后端 / Flutter 安卓前端） - Evidence

No evidence has been recorded yet.

## EvidenceBundleDraft

- Artifact key: goja-spike-output
- Type: log
- Source: go run . (goja v0.0.0-20260906210903, go1.25.5)
- Summary: 8 组沙箱实验: ES 覆盖 28/30; 无宿主全局逃逸; while(true) 61ms 被杀且 VM 可复用; 未设 SetMaxCallStackSize 时无限递归挂死进程; SetRandSource 同种子输出逐字相同; 冷启动 VM 23.7us/题; 7 类错误源码全部被拒
- Verifier: aegis-js-spike/main.go (可复现)

## EvidenceBundleDraft

- Artifact key: spec-approved
- Type: artifact
- Source: 用户明确确认: 进入实施计划 + D6/D7 保持最小
- Summary: 设计规格获批; 用户另提供仓库地址 shinyes/cala 与 applicationId cc.lcyk.cala; 4 commits 已推送 main
- Verifier: 用户会话确认 + git ls-remote

## EvidenceBundleDraft

- Artifact key: sqlite-cascade-spike
- Type: log
- Source: go run . (modernc.org/sqlite v1.58.0, CGO_ENABLED=0, go1.25.5)
- Summary: V1 成立: 纯 Go 驱动可用, CGO_ENABLED=0 交叉编译 linux/amd64 产出 6.08MB 静态二进制; 5 场景全部 PASS(删项目级联/退订仅清本人/删用户/事务回滚/8并发读); 关键发现: PRAGMA foreign_keys 必须写在 DSN 中, 否则连接池下级联静默失效
- Verifier: evidence/sqlite-cascade-spike/main.go (可复现)

## EvidenceBundleDraft

- Artifact key: p0-p1-acceptance
- Type: test-report
- Source: go build/vet/test + flutter analyze/test/build + 手工端到端 HTTP + DB 字节取证
- Summary: P0/P1 出口条件满足: 5 个 Go 包测试全绿; A1 通过(首个用户 becameAdmin=true, 关注册后 403); 级联不变量经变异测试取证确有捕获能力; DB 中无明文令牌与明文口令; Flutter analyze 无 issue 且 apk/web 均构建成功
- Verifier: evidence/p0-p1/ACCEPTANCE.md

## EvidenceBundleDraft

- Artifact key: p2-acceptance
- Type: test-report
- Source: go test ./internal/rules/ -count=1 -v  + 两项变异测试
- Summary: A2/A3/A4 全通过: 69 项测试 PASS/0 FAIL; 编译期拒绝 7 类错误源码, 运行期拒绝 18 类; 同种子 50 题逐字段相同; 6 条恶意规则 30s 内全被拒; 变异测试证明可复现性与超时中断均为承重(移除后分别失败与挂起)
- Verifier: evidence/p2/ACCEPTANCE.md

## EvidenceBundleDraft

- Artifact key: p3-acceptance
- Type: test-report
- Source: go test ./... -count=1 (336 项) + 手工端到端 + 查库 + 浮点变异测试
- Summary: A11/A14 通过。336 测试 0 失败。查库确认 client_is_correct 6 行全 true(原样保存未覆盖)而 server_is_correct 仅 3 行 true(权威重算)，分歧计数 3。变异测试证明无浮点判分被真实守护。修复 3 处设计缺陷(ASCII 文本矛盾/容差无字段)与 3 处实现或测试缺陷(fail 吞错误/硬编码迁移数/逗号分组预期)
- Verifier: evidence/p3/ACCEPTANCE.md

## EvidenceBundleDraft

- Artifact key: p3.5-acceptance
- Type: test-report
- Source: flutter test (13872 条语料比对) + go test ./cmd/scorpus/ + 三项变异测试
- Summary: A12/A13 通过。13872 条语料逐例一致(分类628+比较13244)。变异测试证明门禁有捕获能力: Dart 改 double 仅 4/13244 命中(恰为植入陷阱, 说明用例针对性才是强度来源); 静默降级 278/628 命中; 改 Go 清洗表不重新生成则新鲜度护栏失败。已记录过程偏差: P3.5 未单独写计划文档
- Verifier: evidence/p3.5/ACCEPTANCE.md

## EvidenceBundleDraft

- Artifact key: p4-acceptance
- Type: test-report
- Source: flutter analyze/test/build + go test ./... + 手工端到端 HTTP + 隐私边界验证
- Summary: A9 通过。前端 94 项测试、后端 8 包全绿、analyze 无 issue、apk 与 web 均构建成功。端到端走通注册->建项目->出题->交卷->错题并验证隐私(他人读轮次记录得 404)。发现并修复只在 Flutter Web 触发的种子精度 bug: 63 位种子经 JSON 数字(JS 只有 53 位尾数)往返后变值, 导致服务端重放出不同题目、每题判错; 修复为 2^53 上限并附两项测试(一项断言往返无损, 一项证明该断言的必要性)
- Verifier: evidence/p4/ACCEPTANCE.md

## EvidenceBundleDraft

- Artifact key: p5-acceptance
- Type: test-report
- Source: go test ./... (9 包) + flutter analyze/test (107 项) + 端到端三种粒度 + 中位数变异测试
- Summary: A8 通过。新增 internal/stats 36 测试、统计端点 20 测试、折线图 13 测试。端到端手工核对: 三轮 4000/8000/12000ms -> min4000 max12000 avg8000 median8000; 正确率 1.0/0.5/0.0 -> avg0.5 median0.5。隐私边界与派生态均有测试守护。变异测试: 中位数偶数分支改为取下中位数后偶数用例全失败而奇数用例仍通过
- Verifier: evidence/p5/ACCEPTANCE.md

## EvidenceBundleDraft

- Artifact key: p6-acceptance
- Type: test-report
- Source: go test ./... (9 包) + flutter analyze/test (117 项) + 端到端功能5闭环 + 退订变异测试
- Summary: A5/A6 通过。端到端走完分享->订阅->跟随->退订并查库确认: bob 导入 3 条(1 有效/1 令牌无效/1 跨实例)成功 1/3 且逐条有可读原因; bob 统计 3 轮而 alice 2 轮(隐私独立); alice 改项目后 bob 立即看到; bob 退订 deletedRounds=3 后再练习 403 且 alice 记录完好。变异测试: 退订只删订阅行不清轮次时 A6 测试立即失败
- Verifier: evidence/p6/ACCEPTANCE.md

## EvidenceBundleDraft

- Artifact key: p7-acceptance
- Type: test-report
- Source: 测试 keystore 真实构建 release APK + apksigner 验签 + Linux 交叉编译 + Dart YAML 结构校验
- Summary: A15 本地真实验证通过: 用一次性 keystore 走完与 CI 相同的配置路径, apksigner 显示证书 DN=CN=Cala Test Signing(非 debug)。同时修复三个真实问题: P-R8 修复不彻底(环境变量到不了已在运行的进程, 改为 kotlin.incremental=false)、preflight 用 keytool 却未准备 JDK、未加引号的 heredoc 会二次展开口令。A10(tag 触发/ghcr/release 附件)工作流已交付且结构合法但需 CI 才能验证
- Verifier: evidence/p7/ACCEPTANCE.md

## EvidenceBundleDraft

- Artifact key: p7-release-published
- Type: release
- Source: GitHub Actions run #34570826903 + 本机 apksigner 独立验签
- Summary: A10/A15 已验证。Release v0.0.1 已发布, 附件 cala-0.0.1-linux-amd64.tar.gz(8.7MB) 与 cala-0.0.1.apk(50.5MB)。独立验证: 下载已发布 APK, 大小与 SHA-256 与 GitHub 记录一致, apksigner 验签通过(v2 scheme, 单签名者, 非 debug 证书)。证书 DN 全为 Unknown(生成时未填), 不影响功能与可升级性
- Verifier: https://github.com/shinyes/cala/releases/tag/v0.0.1 + 本机 apksigner

## EvidenceBundleDraft

- Artifact key: p7-deploy-defects
- Type: test
- Source: 本机逐层解包 ghcr 镜像 + CI smoke job(run #34587246298) + config 单元测试变异验证
- Summary: 应要求编写 compose 部署配置时，为写出准确配置而核实已发布镜像，发现两个真实缺陷并修复: (1) 镜像中不存在 /data 而容器以 nonroot 运行 -> 命名卷部署必然失败(unable to open database file, 实测 exit 1 硬失败)。已用 COPY --chown=65532 修复; 新增 smoke job 在真实 Docker 中验证, 并本机解包 0.0.2 确认 /data 属主为 65532。 (2) CALA_DEV_CORS_ORIGINS 显式设为空串不会关闭 CORS 而是回退到 localhost:3000, 与代码注释相反。已改用 os.LookupEnv; 新增回归测试并做变异验证(还原后测试如实失败并报出错误值)。v0.0.2 已发布
- Verifier: CI run #34587246298 (6 job 全 success, smoke 8 步全通过) + 本机镜像解包
