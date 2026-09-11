# ADR-0007 - 答案域收窄为整数或小数

Status: `recorded-from-work`
Date: `2026-09-11`（同日两次收窄，见「修订记录」）
Supersedes: 无；本 ADR 自身经修订，未拆分新 ADR

## Source Evidence

- evidence/p7/ACCEPTANCE.md §6.6（真实后端端到端取证）；规格 §5.5.2 与 §9.3 的执行期修正
## Context

规格 §5.5.2 允许文本答案（含中文），并曾为此专门修过一次设计缺陷；而功能6 要求练习时使用自带键盘，键盘字母表为 `0-9` `.` `-` 与退格且刻意不唤起系统键盘。两者从未对账，导致文本答案的规则能保存成功、服务端也正常下发 `kind=text` 的题目，但用户永远答不对，并会白做一整轮、把全错记录写进统计。

同一原则还牵出第二处：键盘没有除号，因此**分数形式**答案里
**无限循环小数**（如 `1/3`）永远敲不出来，作者必须自行配容差才行 ——
那是个容易忽略的坑。经产品确认，速算场景只需整数与小数。

## Decision

答案域为 **整数或小数**。service 层在保存期与开轮时拒绝两类答案：

1. **文本答案**（如 `"质数"`）—— 键盘打不出中文。
2. **分数形式**（如 `"1/2"`）—— 键盘没有除号；提示改写为等值小数。

两者同源，都是同一条原则的推论：**作者写的答案必须能用练习键盘原样敲出**。

拒绝逻辑收敛为单一 helper（`classifyAnswerable`），保存期与开轮共用。
**`scoring.Classify` 与 `ParseRational` 保持接受文本与分数** ——
已落库的历史信封仍需据其判分。

判据：分数形式看**清洗后的原始字面量是否含 `/`**，而不是看信封 ——
`0.5` 与 `1/2` 生成数值相等但形状不同的信封（`5/10` 与 `1/2`），
按信封无法区分「作者写小数」与「作者写分数」。

## Alternatives Considered

- ① 给文本答案/分数加输入途径（题型为 text 时改用系统键盘）——保留通用性，但需真机验证且与「自带键盘」的产品意图相悖。② 仅在编辑器给非阻断提示——提示易被忽略，项目仍然是坏的。③ 把拒绝下沉到 scoring.Classify —— 会让历史数据与错题重练立刻无法判定，明确排除。④ 只拒绝文本、保留分数 —— 会留下「无限循环小数需容差」这个坑，而收窄到整数/小数能**彻底消除**它。
## Consequences

- 作者无法再创建文本答案题与分数答案题。速算场景不受影响：整数与小数足够，且**分数可改写为等值小数**（`1/2` → `0.5`），因判分是交叉相乘，改写不改变题目对错。
- 已存在的文本答案项目在编辑或开轮时会收到明确报错，须改为数值答案 —— 这是有意的：该项目本就无法作答。
- 收窄后**「作者写的答案一定能用键盘原样敲出」无条件成立**，这是收窄的主要收益：不再有「看起来合法但用户答不出」的题目。
- 修复过程中发现并修掉一处由本次收窄**引入**的不一致：`scoring` 在「数值无法解析」时曾建议作者「改用文本答案」，而文本答案此时已被拒绝 —— 那是个把人引向第二个失败的死提示。该建议已从 `scoring` 移除（本包保持产品无关），产品层指引由 service 负责。
## Compatibility Boundary

收窄仅发生在 service 层的产品校验，**不触及判分契约**：`scoring.Classify` 与 `ParseRational` 继续接受文本与分数，历史信封（`kind=text` 与 num/den 形状）与错题重练不受影响。该边界由 `TestScoringStillClassifiesTextAndFraction` 锁定，防止拒绝逻辑被下沉到 scoring。

注意：**小数同样以 num/den 形状落库**（`0.5` → `5/10`），
因此 `ParseRational` 的分数分支一条都不能少 —— 它不是只为历史分数服务的。

## Retirement Impact

无退役项。若日后要恢复文本答案或分数答案，应同时提供输入途径（系统键盘/文本框、
或恢复键盘除号）并撤销 service 层的拒绝，而非放宽 scoring —— 后者会破坏历史判分。

## Baseline Sync

- Needed: needed
- Target: docs/aegis/baseline/2026-09-10-initial-baseline.md
- Action: update baseline
- Reason: 作者契约（规则返回值域）发生收窄，属产品/架构边界变化。基线 §5.2 的八条不可协商项不受影响，但「答案域」这一维度需记录（§4.2 第 8 条）。

## Evidence References

- docs/aegis/work/2026-09-10-mental-math-app/evidence/p7/ACCEPTANCE.md
- 真实 HTTP 复验：文本与分数 → 400 且消息含题号与改法；整数与小数 → 201

## 修订记录

- 2026-09-11 初版：拒绝文本答案（`TestScoringStillClassifiesTextAnswer`）。
- 2026-09-11 同日修订：产品要求「答案只需整数或小数」，遂进一步拒绝**分数形式**。
  这不是新增决定，而是把初版已声明的原则（答案必须能用键盘敲出）
  贯彻到底 —— 故修订本 ADR 而非另立新 ADR，避免「答案域」这一决定出现两个 owner。
  原「分数答案仍合法」一句已删除。

## Boundary

This ADR is an advisory Aegis Method Pack record. It does not grant completion authority or replace project-authoritative architecture sources.
