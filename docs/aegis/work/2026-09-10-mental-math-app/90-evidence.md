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
