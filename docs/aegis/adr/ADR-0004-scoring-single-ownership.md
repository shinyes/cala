# ADR-0004 - 判分单一性策略（跨端一致性）

Status: `recorded-from-work`
Date: `2026-09-10`

## Source Evidence

- evidence/p3/ACCEPTANCE.md + evidence/p3.5/ACCEPTANCE.md（13872 条语料 + 三项变异测试）
## Context

功能9 要求「打错立即反馈」，即客户端必须在本地判分（零网络往返）；D13 又要求服务端重算以保证落库权威性。于是同一判分语义必须存在于两端，而任一处语义漂移都会造成「界面显示答对、落库记为错」并**静默污染历史统计**。

## Decision

结构性收敛而非缓解：① 答案域收敛为「有理数信封 / 规范文本信封」两类，其他形态在保存期报错；② 判分路径**无浮点**——有理数一律 big.Int / Dart BigInt 交叉相乘；③ 输入清洗表由**服务端单一下发**，客户端只应用数据，不内置任何映射；④ 语料由 Go 侧生成（13872 条）并作为 Dart 源码常量入库，Dart 必须逐例对齐，CI 门禁使分歧即失败；⑤ attempt 同时记录 client_is_correct 与 server_is_correct，不一致即计数并留痕；⑥ 保存 user_input 与信封，可全量重算修复历史。

## Alternatives Considered

- ① 「共享 JS 源码」跨端判分：共享了源码却未消除**引擎语义差异**（浮点格式化、正则、Unicode 行为），且客户端要引入原生 JS 引擎依赖，Web 与 Android 反而形成两条路径。② 仅靠「共享测试向量人工同步」：只能提高发现概率，不能消除分歧可能。
## Consequences

- 残余诚实记录：技术上仍是两份实现，其语义由六重措施共同约束（规格 §5.5.8）。关键取证：把 Dart 精确比较改为 double 后，**仅 4/13244 条语料命中**——恰好是刻意植入的浮点陷阱（2^53 vs 2^53+1、1/3 vs 0.333…）。这说明语料规模本身不保证强度，**用例的针对性**才是。
## Compatibility Boundary

server_is_correct 是唯一权威判分值，统计一律基于它；清洗表只有服务端一个 owner；判分路径不得出现浮点。语料变更必须由 Go 侧重新生成（有新鲜度护栏：改了 Go 实现却不重新生成会使 CI 失败）。

## Retirement Impact

无退役对象。明确拒绝：客户端 JS 引擎、第三方有理数库。

## Baseline Sync

- Needed: not-needed
- Target: docs/aegis/baseline/2026-09-10-initial-baseline.md
- Action: cite unchanged
- Reason: 规格 §5.5 与基线 §5.2(6)(7)(8) 已固化；基线 §6 的「边界缺口」已标记为 D15 关闭。

## Evidence References

- docs/aegis/work/2026-09-10-mental-math-app/evidence/p3.5/ACCEPTANCE.md
## Boundary

This ADR is an advisory Aegis Method Pack record. It does not grant completion authority or replace project-authoritative architecture sources.
