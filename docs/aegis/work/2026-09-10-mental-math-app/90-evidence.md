# 速算练习 App（Go+Fiber+SQLite 后端 / Flutter 安卓前端） - Evidence

No evidence has been recorded yet.

## EvidenceBundleDraft

- Artifact key: goja-spike-output
- Type: log
- Source: go run . (goja v0.0.0-20260906210903, go1.25.5)
- Summary: 8 组沙箱实验: ES 覆盖 28/30; 无宿主全局逃逸; while(true) 61ms 被杀且 VM 可复用; 未设 SetMaxCallStackSize 时无限递归挂死进程; SetRandSource 同种子输出逐字相同; 冷启动 VM 23.7us/题; 7 类错误源码全部被拒
- Verifier: aegis-js-spike/main.go (可复现)
