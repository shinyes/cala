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
