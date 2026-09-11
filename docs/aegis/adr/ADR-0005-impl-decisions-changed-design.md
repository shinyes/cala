# ADR-0005 - 实施期确立的四项工程约定（规格未涵盖）

Status: `recorded-from-work`
Date: `2026-09-10`

## Source Evidence

- evidence/p4/ACCEPTANCE.md（种子精度）+ evidence/p7/ACCEPTANCE.md（构建与 CI）+ P2/P4 计划文档
## Context

规格是设计期产物，实施过程中出现了四项**规格未指定、但后继维护者会质疑**的约定。它们不改变产品行为，却决定构建与运行是否可靠，因此单独记录。

## Decision

① **种子上限 2^53**：种子经 JSON 以数字往返，而 JavaScript（含 Flutter Web 的 dart2js，其 int 即 double）只有 53 位尾数精度；超过 2^53 的值往返后变值，导致服务端重放出不同题目、每题判错。② **每问新建 VM**（见 ADR-1）。③ **kotlin.incremental=false 为仓库默认**：Windows 跨盘符时 Kotlin 增量编译器 flush 缓存的 Path.relativize() 会抛异常（外层报错「Could not close incremental caches」具误导性）。不用环境变量修，因为环境变量只对设置之后启动的进程生效，已在运行的父进程派生的 shell 看不到它。④ **CI 中 secret 一律经环境变量传入脚本**，不直接插值进脚本文本：未加引号的 heredoc 会二次展开，含 $ 的口令被静默改写。

## Alternatives Considered

- ① 种子：不做上限，改为把种子以字符串传输——需改契约且客户端也要改解析。② 跨盘符：只写文档要求「pub cache 与项目同盘」——实测不可靠（见上）。③ 口令：用带引号的 heredoc 并预先转义——比环境变量方案更易出错。
## Consequences

- ① 的取证方式值得记录：新增两项测试，一项断言 50 次取样经 float64 往返无损，另一项**证明该断言的必要性**（2^53+1 经 float64 确实变为 2^53）。③ 的代价：本地重复构建失去 Kotlin 增量编译（数十秒）；**CI 零成本**，因为 CI 每次都是冷构建。④ 已额外加 keytool 试开以使口令/别名错误提前显眼失败。
## Compatibility Boundary

种子上限与「判分无浮点」同源，都属跨端数值精度边界：任何经由 JSON 数字往返的 64 位整数都必须 ≤ 2^53。签名材料（key.properties / *.jks）绝不入库，CI preflight 会断言。

## Retirement Impact

③ 的退役触发条件：当 pub cache 与项目同盘后，可把 kotlin.incremental 改回 true 以恢复更快的本地构建。

## Baseline Sync

- Needed: needed
- Target: docs/aegis/baseline/2026-09-10-initial-baseline.md
- Action: cite unchanged
- Reason: 这些是实施期约定，不改变已批准的产品或架构边界；基线 §5.2 的八条不可协商项仍然成立。若日后调整 seed 上限或恢复增量编译，应更新基线。

## Evidence References

- docs/aegis/work/2026-09-10-mental-math-app/evidence/p7/ACCEPTANCE.md
## Boundary

This ADR is an advisory Aegis Method Pack record. It does not grant completion authority or replace project-authoritative architecture sources.
