# ADR-0001 - 规则引擎契约与沙箱边界

Status: `recorded-from-work`
Date: `2026-09-10`

## Source Evidence

- evidence/goja-spike/OUTPUT.txt（8 组沙箱实验）+ evidence/p2/ACCEPTANCE.md
## Context

功能2 要求用户用一段 JS 定义出题规则。规则是**不可信代码**；且作者改规则会同步影响所有订阅者（功能5），因此沙箱边界与保存期校验是安全性与正确性的共同基础。

## Decision

作者只写 function generate(cfg) 返回 {q,a}（D1/D11）。沙箱四项配置为**强制项**：SetRandSource（每轮种子，保证可复现）、SetTimeSource（冻结时刻）、SetMaxCallStackSize(200)、单次调用 Interrupt 超时 50ms。保存前必须编译 + 连续出题 20 次 + 校验返回形状，任一步失败即拒绝保存。每道题使用**独立的 VM**。

## Alternatives Considered

- ① 复用单个 VM + 逐题 Interrupt：实测冷启动仅 23.7µs/题、复用 8.8µs/题，差异不足以支撑管理 AfterFunc/Interrupt/ClearInterrupt 竞态的复杂度；该竞态会表现为「上一次超时的中断误杀下一题」的间歇性失败。② VM 池：Existence Check 判定为过度设计。③ 跨端共享 JS 源码判分：见 ADR-4。
## Consequences

- 得到显式作者契约：generate 必须是 cfg 与随机数的纯函数，模块级状态不跨题保留（有专门测试固化）。代价是模块级初始化按题重复执行——对纯计算型速算规则可忽略。
## Compatibility Boundary

契约一旦发布即难变更（影响所有已分享项目）；沙箱四配置不得缺项——不设递归上限实测会**挂死整个进程**（跑满 300s 未返回）。

## Retirement Impact

无退役对象。明确拒绝：外部规则市场、多函数契约、规则热加载。

## Baseline Sync

- Needed: not-needed
- Target: docs/aegis/baseline/2026-09-10-initial-baseline.md
- Action: cite unchanged
- Reason: 该决策在规格 §5.2/§5.3 已完整记录，初始基线 §5.2(1)(2) 已固化，无需更新基线。

## Evidence References

- docs/aegis/work/2026-09-10-mental-math-app/evidence/p2/ACCEPTANCE.md
## Boundary

This ADR is an advisory Aegis Method Pack record. It does not grant completion authority or replace project-authoritative architecture sources.
