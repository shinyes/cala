# ADR-0002 - 答题记录生命周期绑定 (project, user) 关系

Status: `recorded-from-work`
Date: `2026-09-10`

## Source Evidence

- evidence/p6/ACCEPTANCE.md（含退订变异测试）+ evidence/p0-p1/ACCEPTANCE.md（外键级联实测）
## Context

功能8 要求「删除项目时所有人的答题记录一并删除」，D4 又要求「订阅者退订即清空本人历史」。两者看似不同，本质是同一条关系终止。

## Decision

定义唯一不变量：(project, user) 关系终止 ⇒ 该用户在该项目下的 practice_round/attempt 消亡。由**数据库外键级联**执行，应用层不写补偿删除逻辑。退订实现为**单一原子操作**：store 层刻意不提供可单独调用的「删除订阅行」函数。attempt 由 practice_round 外键级联，应用层不再写一遍。

## Alternatives Considered

- ① 为「作者删项目」与「订阅者退订」各写一套删除逻辑——同一规则两个实现，迟早不一致。② 提供可单独调用的删订阅行原语——会留下「删了订阅却忘了清历史」的错误用法。
## Consequences

- 一条规则覆盖两种操作。变异测试证明「只删订阅行」会让 A6 测试立即失败（轮次残留 3），即用户以为退订干净了但数据仍在库里、统计仍会算进去。
## Compatibility Boundary

级联语义不可削弱。**PRAGMA foreign_keys 必须写在 DSN 中**：SQLite 该 pragma 每连接生效，而 database/sql 是连接池——写在 Open 之后会让级联**静默失效**并留下孤儿记录。已由专门的 falsifier 测试守护。

## Retirement Impact

无退役对象。

## Baseline Sync

- Needed: not-needed
- Target: docs/aegis/baseline/2026-09-10-initial-baseline.md
- Action: cite unchanged
- Reason: 规格 §6.1(2) 与基线 §5.2(3) 已固化该不变量。

## Evidence References

- docs/aegis/work/2026-09-10-mental-math-app/evidence/p6/ACCEPTANCE.md
## Boundary

This ADR is an advisory Aegis Method Pack record. It does not grant completion authority or replace project-authoritative architecture sources.
