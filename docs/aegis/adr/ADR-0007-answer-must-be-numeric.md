# ADR-0007 - 答案域收窄为仅数值：拒绝文本答案

Status: `recorded-from-work`
Date: `2026-09-11`

## Source Evidence

- evidence/p7/ACCEPTANCE.md §6.7（真实后端端到端取证）；规格 §5.5.2 与 §9.3 的执行期修正
## Context

规格 §5.5.2 允许文本答案（含中文），并曾为此专门修过一次设计缺陷；而功能6 要求练习时使用自带键盘，键盘字母表为 0-9 . - 与退格且刻意不唤起系统键盘。两者从未对账，导致文本答案的规则能保存成功、服务端也正常下发 kind=text 的题目，但用户永远答不对，并会白做一整轮、把全错记录写进统计。

## Decision

答案收窄为仅数值：service 层在保存期与开轮时均拒绝文本答案，并给出可读理由（键盘打不出）与改法。拒绝逻辑收敛为单一 helper（classifyAnswerable），两处调用共用。**scoring.Classify 保持接受文本**——已落库的历史信封仍需据其判分。

## Alternatives Considered

- ① 给文本答案加输入途径（题型为 text 时改用系统键盘）——保留通用性，但需真机验证且与「自带键盘」的产品意图相悖。② 仅在编辑器给非阻断提示——提示易被忽略，项目仍然是坏的。③ 把拒绝下沉到 scoring.Classify——会让历史数据与错题重练立刻无法判定，明确排除。
## Consequences

- 作者无法再创建文本答案题，速算场景不受影响（本就只需数值）。已存在的文本答案项目在编辑或开轮时会收到明确报错，须改为数值答案——这是有意的：该项目本就无法作答。配套：输入字母表移除 /，故无限循环小数（如 1/3）形式的答案须配置容差。
## Compatibility Boundary

收窄仅发生在 service 层的产品校验，**不触及判分契约**：scoring.Classify 继续接受文本，历史信封（kind=text）与错题重练不受影响。该边界由 TestScoringStillClassifiesTextAnswer 锁定，防止拒绝逻辑被下沉到 scoring。分数答案（如 1/2）仍合法，因为它是数值。

## Retirement Impact

无退役项。若日后要恢复文本答案，应同时提供输入途径（系统键盘/文本框）并撤销 service 层的拒绝，而非放宽 scoring —— 后者会破坏历史判分。

## Baseline Sync

- Needed: needed
- Target: docs/aegis/baseline/2026-09-10-initial-baseline.md
- Action: update baseline
- Reason: 作者契约（规则返回值域）发生收窄，属产品/架构边界变化。基线 §5.2 的八条不可协商项不受影响，但「答案域」这一维度需记录。

## Evidence References

- docs/aegis/work/2026-09-10-mental-math-app/evidence/p7/ACCEPTANCE.md
## Boundary

This ADR is an advisory Aegis Method Pack record. It does not grant completion authority or replace project-authoritative architecture sources.
