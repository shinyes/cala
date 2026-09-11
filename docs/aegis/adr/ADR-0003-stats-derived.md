# ADR-0003 - 统计为派生态而非物化态

Status: `recorded-from-work`
Date: `2026-09-10`

## Source Evidence

- evidence/p5/ACCEPTANCE.md（含中位数变异测试）
## Context

功能4 需要日/周/月折线与 8 项指标（耗时与正确率的最高/最低/平均/中位）。它由答题事实推导而来。

## Decision

不建任何物化聚合表或缓存表（D5）。一次查询取回区间内全部 practice_round，在 Go 内分桶并计算全部 8 项指标。分桶在**用户本地时间**下进行（端点接受 tzOffsetMinutes）。周粒度按**星期几跨日期聚合**。平均正确率取「各轮正确率的平均」，而非总正确数/总题数。

## Alternatives Considered

- ① 物化日/周/月聚合表——查询更快，但删除需显式级联清理，且聚合值可能因 bug 或写入失败而与事实源不一致。② 分桶 SQL + 中位数近似——SQLite 无内置中位数；个人练习数据量下实时计算更简单且不可能算错。
## Consequences

- 删除 practice_round 后统计自然消失，**物理上不存在「记录删了但聚合还在」的可能**。代价是每次查询读取区间内全部轮次——个人数据量下可忽略；若日后成为瓶颈，应加时间范围参数而非物化表。
## Compatibility Boundary

不得新增聚合表；不得改动 practice_round 的写入语义；统计只计算本人数据（取数按 (user, project) 过滤，无跨用户查询路径）。

## Retirement Impact

无退役对象。明确拒绝：图表库（折线图用 CustomPainter 自绘）、时间范围选择器、跨项目汇总。

## Baseline Sync

- Needed: not-needed
- Target: docs/aegis/baseline/2026-09-10-initial-baseline.md
- Action: cite unchanged
- Reason: 规格 §8.1 与基线 §5.2(4) 已固化「统计不得物化」。

## Evidence References

- docs/aegis/work/2026-09-10-mental-math-app/evidence/p5/ACCEPTANCE.md
## Boundary

This ADR is an advisory Aegis Method Pack record. It does not grant completion authority or replace project-authoritative architecture sources.
