# 实施计划 —— P6 分享订阅与退订

- Date: `2026-09-10`
- Design Spec: [`docs/aegis/specs/2026-09-10-mental-math-app-design.md`](../specs/2026-09-10-mental-math-app-design.md) §4.4 / §6.1 / §7
- Baseline: [`docs/aegis/baseline/2026-09-10-initial-baseline.md`](../baseline/2026-09-10-initial-baseline.md)
- 上游：[`evidence/p5/ACCEPTANCE.md`](../work/2026-09-10-mental-math-app/evidence/p5/ACCEPTANCE.md)
- 覆盖阶段：规格 §16 的 **P6 分享订阅与退订**
- 后续：P7 CI 与交付（最后一个阶段）

---

## Goal

交付功能 5 的完整闭环：

1. 作者生成/重置/撤销分享链接
2. 其他用户通过链接订阅（**支持一次粘贴多行/多个链接**）
3. 订阅者纯只读跟随作者配置（作者改项目，订阅者同步改变）
4. 订阅者可随时退订，**退订即清空自己在该项目下的历史**（D4）

**本计划不含**：跨实例订阅（链接里的 host 与当前实例不同则拒绝）、
订阅通知、项目市场与审核。

## Architecture

```
backend/internal/store/subscription.go   订阅关系的读 + 一个原子退订操作
backend/internal/store/share.go          分享 token 的读写
backend/internal/service/subscription.go 导入解析、订阅、退订
backend/internal/service/share.go        分享 token 生成/重置/撤销
backend/internal/api/subscription.go     /api/subscriptions/*
backend/internal/api/share.go            /api/projects/:id/share

app/lib/api/subscription_api.dart        端点封装
app/lib/state/subscriptions.dart         导入/退订状态
app/lib/ui/import_subscriptions_page.dart 多链接导入页
app/lib/ui/share_page.dart               分享链接页
app/lib/ui/profile_tab.dart              接上退订入口（替换 P4 的「尚未提供」）
app/lib/ui/practice_tab.dart             接上导入入口（替换 P4 的「尚未提供」）
```

## Tech Stack

无新增依赖。

## Baseline / Authority Refs

- 规格 §4.4（链接格式 `cala://subscribe?h=<host>&t=<token>`——**因为后端不提供网页
  前端，`https://` 落地页会 404**，所以用应用内可粘贴的链接字符串）
- 规格 §6.1(2)（唯一级联规则：`(project, user)` 关系终止 ⇒ 该用户在该项目下的
  `practice_round`/`attempt` 消亡）
- 规格 §6.1(4)（订阅无需传播：订阅者读的就是 `project` 同一行）
- 规格 §7（`POST /api/subscriptions/import`、`DELETE /api/subscriptions/:projectId`、
  `POST/DELETE /api/projects/:id/share`）
- 规格 D4（纯只读跟随；退订即清空历史）
- 基线 §5.2(3)（级联只有一条规则，不得为两种情况各写一套）
- 基线 §9(3)

## Compatibility Boundary

1. **退订必须是一个原子操作**。这是本阶段最重要的一条。
   P3 已在 `store/subscription.go` 的注释中写明：**刻意不提供**「单独删除订阅行」的
   函数，就是为了让「删了订阅却忘了清历史」在结构上不可能发生。本阶段必须保持这一点：
   退订要在一个事务里同时完成三件事——
   - 删除该用户在该项目下的 `practice_round`（`attempt` 由外键级联）
   - 删除该用户的 `subscription` 行
   - 统计因此自然消失（统计是派生态）

2. **订阅是纯只读跟随**（D4）：订阅者不得修改项目。这条已由 P3 的
   `service.ErrNotOwner` 保证，本阶段需在**订阅导入**侧确认它仍然成立
   （即订阅不给任何写权限）。

3. **作者删除项目仍然级联删除所有人**（功能8）：这是与退订不同的操作，
   但由**同一条**外键规则覆盖。不得为两者各写一套删除逻辑。

4. **链接解析必须校验实例**：`h`（host）与当前实例不符时拒绝并给出明确原因，
   而不是静默订阅失败或订阅到错误的项目。

5. 不得改动既有的 `ProjectAccess` 语义（它是访问权判定的唯一入口）。

## TDD Route

```text
TDD Route:
- Mode: off
- Decision: skipped
- Strict authority: not applicable
- Test posture: post-change regression
- Reason: 项目配置 tdd_mode = "off"，用户未要求严格 TDD
- Verification: go test ./... -count=1 ; flutter analyze ; flutter test
```

> 退订是**唯一的跨用户破坏性操作**，故为其写重点回归测试：
> 「只清自己的、不碰他人的」必须被断言，而不是靠代码审查。

## Verification

```powershell
cd D:\Desktop\Cala\backend ; go build ./... ; go vet ./... ; go test ./... -count=1
cd D:\Desktop\Cala\app     ; flutter analyze ; flutter test
```

端到端：注册两个用户 → A 建项目并做一轮 → A 生成分享链接 →
B 导入链接订阅（含多链接）→ B 做一轮 → A 改项目 → B 看到变化 →
B 退订 → 查库确认 B 的历史被清空而 A 的完好。

---

## Scope Check

**Aegis Visibility**：本阶段落地的是**跨用户的数据所有权边界**。退订会不可逆地
删除用户自己的历史记录，而分享会让一个项目被多人使用。这两件事都必须由
**一条**规则覆盖（规格 §6.1(2)），否则会出现「某条路径漏删」的静默数据残留。
因此重点测试放在「退订只清自己」与「删项目清所有人」这两个方向，
以及「订阅不授予写权限」。

```text
Requirement Ready Check:
- Requirement source refs: 用户功能 5、功能 8；规格 §4.4、§6.1、§7、D4
- Goals and scope refs: 规格 §16 P6 行
- User / scenario refs: 作者分享项目；他人订阅并跟随；订阅者退订
- Requirement item refs: 功能 5（分享订阅 + 多链接导入）、功能 8（级联删除）
- Acceptance / verification criteria refs: A5（删项目级联）、A6（退订只清自己）
- Open blocker questions: 无
- Decision: ready
```

```text
Change Necessity:
- User-visible need: 分享与订阅入口在 P4 已存在但明确提示「尚未提供」
- No-change / non-code option: 不存在——功能 5 完全未实现
- Why code change is necessary: 无既有实现
- Minimum change boundary: 新增 share/subscription 的 store+service+api 三层与前端两页；
  不改动既有端点的行为语义
- Decision: code-change
```

```text
Existence Check:
- Proposed new surface: ①store 的退订原子操作 ②share token 的读写
  ③service 的链接解析 ④前端导入页与分享页
- Existing owner / reuse candidate: store.Subscribe / IsSubscribed / CountSubscriptions
  已存在（P3 交付）；project.share_token 列已存在（P0 建表）
- Why existing surface is insufficient:
  ① 需要**一个事务性**的退订操作（P3 刻意未提供可单独调用的删除原语）；
  ② share_token 列存在但无读写路径；
  ③ 链接解析是新逻辑；
  ④ 功能 5 需要界面
- Creation proof: 规格 §7 定义了这四个端点；§4.4 定义了链接格式
- Entropy / retirement impact: 无旧路径退役。**明确拒绝**：不引入短链接服务、
  不做二维码生成（可后续加）、不做订阅通知
- Decision: add-with-proof
```

```text
Architecture Integrity Lens:
- Invariant: `(project, user)` 关系终止 ⇒ 该用户在该项目下的记录消亡（规格 §6.1(2)）
- Canonical owner / contract: 数据库外键是级联的**唯一执行者**；
  应用层只负责在正确的时机终止关系，不写补偿删除逻辑
- Responsibility overlap: 退订（清自己）与删项目（清所有人）路径不同、规则相同；
  不得各写一套
- Higher-level simplification: 退订 = 「删自己的 round + 删 subscription 行」，
  由外键完成 attempt 的级联 —— 不需要在应用层遍历 attempt
- Retirement / falsifier: 若有人新增「删除订阅行」的独立函数，就会出现
  「删了订阅却忘了清历史」的用法 → 由 P3 留下的注释 + 本阶段的测试守护
- Verdict: 无阻塞问题
```

```text
Plan Pressure Test:
- Owner / contract / retirement: 承载点已识别（退订原子性、级联单一规则、链接校验）
- Architecture integrity / higher-level path: 已确认
- Verification scope: A5/A6 各有测试；并有「订阅不授予写权限」的断言
- Task executability: 每步含完整代码与确切命令
- Pressure result: proceed
```

```text
Plan-Time Complexity Check:
- Target files: store 2、service 2、api 2、前端 4
- Existing size / shape signals: subscription.go 已有 3 个函数，本阶段追加退订
- Owner fit: store/subscription.go 是订阅关系的 owner，退订放这里合适
- Add-in-place risk: 若把 share token 逻辑也塞进 project.go，会让项目 CRUD 与
  分享关注点混杂
- Better file boundary: 新建 store/share.go
- Recommendation: add owner file
```

---

## Task P6.1 —— 分享 token

**Files**
- Create: `backend/internal/store/share.go`
- Create: `backend/internal/service/share.go`
- Create: `backend/internal/api/share.go`
- Modify: `backend/internal/api/project.go`（接线）
- Create: `backend/internal/api/share_test.go`

**Why**：功能 5 的入口。作者需要一条可发给别人的链接。

**Change Necessity**：`code-change`。

**Steps**

1. `store/share.go`：

```go
// SetShareToken 写入或替换项目的分享 token。
//
// 重置（替换）会使旧链接立即失效 —— 这是「撤销分享」的实现方式，
// 因此不需要单独的 revoked 标记：token 不匹配即无法订阅。
func (s *Store) SetShareToken(projectID int64, token string) error

// ClearShareToken 撤销分享（置空）。
func (s *Store) ClearShareToken(projectID int64) error

// ProjectByShareToken 按 token 查项目。订阅时用它解析链接。
func (s *Store) ProjectByShareToken(token string) (Project, error)
```

要点：
- `share_token` 列已有 `UNIQUE` 约束（P0 建表），无需额外索引
- `share_token_updated_at` 列同时更新：便于日后排查「链接何时被重置」
- token 生成复用 `auth.NewToken()`（32 字节 base64url，SHA-256 不入库——
  这里的 token 是**分享凭据**而非会话令牌，需要能反查明文，故直接存明文；
  **必须在代码注释中说明这一区别**，否则会被误认为安全缺陷）

2. `service/share.go`：生成/重置/撤销，仅作者可操作（复用 `ProjectAccess`）。

3. `api/share.go`：

```
POST   /api/projects/:id/share   -> { shareToken, link }
DELETE /api/projects/:id/share   -> 204
```

`link` 由服务端拼好：`cala://subscribe?h=<host>&t=<token>`。
**host 的来源**：请求的 `Host` 头。这样同一份代码在 localhost、局域网 IP、
域名下都能给出可用的链接。

4. 测试**必须覆盖**：
- 作者可生成；生成的 token 非空且两次生成不同
- 非作者（订阅者/无关用户）-> 403
- 未登录 -> 401
- 重置后旧 token 不再能订阅（用 P6.2 的导入端点验证）
- 撤销后 token 为空且无法订阅
- `link` 包含当前 Host

5. **Commit**：`feat(share): 分享 token 的生成、重置与撤销`

---

## Task P6.2 —— 订阅导入（多链接）

**Files**
- Create: `backend/internal/service/subscription.go`
- Create: `backend/internal/api/subscription.go`
- Create: `backend/internal/api/subscription_test.go`
- Modify: `backend/internal/store/subscription.go`（追加原子退订，见 P6.3）

**Why**：功能 5 的核心：一次粘贴多个链接完成订阅。

**Change Necessity**：`code-change`。

**Impact / Compatibility**：订阅**不授予任何写权限**（D4）。

**Steps**

1. 链接解析（`service/subscription.go`）：

```go
// ParseSubscribeLink 解析分享链接。
//
// 格式（规格 §4.4）：cala://subscribe?h=<host>&t=<token>
//
// 为什么不用 https:// 链接：后端不提供网页前端（D9），
// 形如 https://host/s/<token> 的链接在浏览器中会 404，无法作为「点开即订阅」
// 的落地页。因此采用应用内可粘贴的链接字符串。
func ParseSubscribeLink(raw string) (host, token string, err error)
```

要求：
- 宽容处理首尾空白与常见前后缀（用户可能从聊天软件里复制到带说明的文字）
- 拒绝 `t` 缺失或为空
- **返回错误时给出可读原因**（哪一段不对），因为用户是手工粘贴

2. 导入端点：

```
POST /api/subscriptions/import   { links: ["...", "..."] }
  -> { results: [ { link, ok, projectId?, title?, alreadySubscribed?, error? } ] }
```

**逐条返回结果而非整批失败**：用户粘贴 5 个链接，其中一个失效时，
其余 4 个应当成功。整批回滚会让用户不知道哪个是坏的，反复试错。

3. 校验（导入时）：
- host 与当前实例不符 -> 该条 `error: "链接属于其他服务器"`
- token 查不到项目 -> 该条 `error: "链接无效或已被撤销"`
- 订阅自己的项目 -> 该条标记为「这是你自己的项目」（不报错，也不建订阅行）
- 已订阅 -> `alreadySubscribed: true`（幂等，不算失败）

4. 测试**必须覆盖**：
- 单链接订阅成功
- **多链接混合**：1 个有效 + 1 个无效 + 1 个属于他人服务器 -> 只有有效的那条成功，
  其余各自给出可读错误（这是「逐条返回」的核心断言）
- 重复订阅同一链接 -> `alreadySubscribed`，不报错，不产生重复行
- 订阅自己的项目 -> 不建订阅行
- 订阅后**不能修改该项目** -> 403（验证 D4：订阅不授予写权限）
- 订阅后**能练习**该项目 -> 200
- 未登录 -> 401
- `links` 为空 -> 400

5. **Commit**：`feat(share): 多链接导入订阅`

---

## Task P6.3 —— 原子退订

**Files**
- Modify: `backend/internal/store/subscription.go`
- Create: `backend/internal/service/subscription.go`（追加）
- Modify: `backend/internal/api/subscription.go`
- Create: `backend/internal/api/unsubscribe_test.go`

**Why**：功能 5 的终点，也是本项目**唯一的跨用户破坏性操作**。

**Change Necessity**：`code-change`。

**Impact / Compatibility**：承载基线 §5.2(3) 与规格 §6.1(2)。

**Steps**

1. `store/subscription.go` 追加**唯一**的退订入口：

```go
// Unsubscribe 终止用户与项目的订阅关系，并清除该用户在该项目下的做题记录。
//
// 这是一个**单一的原子操作**，而不是「删订阅行」+「删记录」两个可分别调用的函数。
// 原因见本文件顶部的说明：拆成两个原语会留下「删了订阅却忘了清历史」的错误用法，
// 而那正是本项目一直在消除的「同一规则两处实现」。
//
// 规则（规格 §6.1(2)）：(project, user) 关系终止 ⇒ 该用户在该项目下的
// practice_round / attempt 消亡。attempt 由 practice_round 的外键级联删除。
//
// 返回被删除的轮次数，供界面在确认时明示代价。
func (s *Store) Unsubscribe(userID, projectID int64) (deletedRounds int, err error)
```

实现要求：
- **在一个事务内**完成：删除 `practice_round`（限定 user+project）→ 删除 `subscription` 行
- 不删除 `attempt`（外键自动级联）——**不得**在应用层再写一遍 attempt 的删除，
  那会让级联规则出现第二个执行者
- 若订阅行不存在，返回 `ErrNotFound`
- **不得删除项目本身**，也不得影响其他用户的记录

2. `service` 层：退订前先确认调用者不是作者（作者退订自己的项目没有意义，
   应提示删除项目）。作者的 `subscription` 行本就不存在，但显式检查能给出更好的提示。

3. 端点：

```
DELETE /api/subscriptions/:projectId  -> { deletedRounds }
```

返回删除的轮次数，使客户端能在确认对话框里**事先**告知代价。

4. 测试**必须覆盖**（这是本阶段最重要的部分）：

| 断言 | 说明 |
|---|---|
| 退订后本人记录消失 | `practice_round` 与 `attempt` 都归零 |
| **他人记录完好** | 作者与其他订阅者的轮次数不变（A6 的核心） |
| 订阅行消失 | `ProjectAccess` 返回 `ErrNoAccess` |
| 项目本身仍在 | 作者仍能访问，其他人仍能订阅 |
| 返回的 `deletedRounds` 等于实际删除数 | 界面提示的代价必须准确 |
| 未订阅时退订 -> 404 | 幂等性与错误可读性 |
| 作者对自己项目退订 -> 400（提示应删除项目） | 避免误操作 |
| 退订后不再能练习该项目 -> 403 | 访问权确实被终止 |
| **退订一项不影响其他项目的记录** | 用户在有多个项目时，只清目标项目 |

5. **Commit**：`feat(share): 原子退订（清空本人历史）`

---

## Task P6.4 —— 前端导入页、分享页与退订入口

**Files**
- Create: `app/lib/api/subscription_api.dart`
- Create: `app/lib/state/subscriptions.dart`
- Create: `app/lib/ui/import_subscriptions_page.dart`
- Create: `app/lib/ui/share_page.dart`
- Modify: `app/lib/ui/practice_tab.dart`（接上导入入口）
- Modify: `app/lib/ui/profile_tab.dart`（接上退订入口）
- Modify: `app/lib/state/projects.dart`（退订后刷新列表）
- Create: `app/test/state/subscriptions_test.dart`

**Why**：功能 5 的界面。P4 已在两处放了明确提示「尚未提供」，本任务替换为真实功能。

**Change Necessity**：`code-change`。

**Impact / Compatibility**：
- **退订是破坏性操作**，确认对话框必须**明示将删除的轮次数**（后端在
  `DELETE` 前无法预知，故客户端先用 `GET` 拿到轮次数，或退订后展示结果——
  实现取前者：调用前用已有的统计/轮次信息无法取得，故在确认对话框中说明
  「将删除你在该项目的全部答题记录与统计，且无法恢复」，不承诺具体数字；
  若后端能提供则展示）
- 导入页必须**逐条展示结果**（成功/失败/已订阅/属于其他服务器）

**Steps**

1. `subscription_api.dart`：`import(List<String> links)`、`unsubscribe(int projectId)`、
   `share(int projectId)`、`unshare(int projectId)`。

返回值模型要能表达每条链接的独立结果：

```dart
class ImportResultItem {
  final String link;
  final bool ok;
  final int? projectId;
  final String? title;
  final bool alreadySubscribed;
  final String? error;
}
```

2. `import_subscriptions_page.dart`：
   - 多行输入框（`CupertinoTextField` + `maxLines`），支持一次粘贴多行
   - 「导入」按钮 → 逐条结果显示列表，每条带状态图标与说明
   - 成功后刷新项目列表

3. `share_page.dart`：
   - 显示当前分享链接（无则显示「尚未分享」）
   - 「生成链接」/「重置链接」（重置需二次确认，说明旧链接会失效）
   - 「复制」（`Clipboard.setData`）
   - 「撤销分享」

4. 接入点：
   - `practice_tab.dart` 的「导入订阅链接」→ 跳转到导入页（替换提示）
   - `profile_tab.dart` 的订阅项退订按钮 → 确认对话框 → 调用退订 → 刷新
   - 项目列表里的「分享订阅链接」→ 跳转到分享页（替换提示）

5. 测试：
   - `ImportResultItem` 的解析（成功/失败/已订阅）
   - 退订后项目列表刷新（订阅分组减少一项）

6. **Commit**：`feat(app): 订阅导入页、分享页与退订入口`

---

## Task P6.5 —— P6 验收

**Files**：无新增（验证与证据）

1. 全量验证并逐条核对 A5/A6，写入
   `docs/aegis/work/2026-09-10-mental-math-app/evidence/p6/ACCEPTANCE.md`：

| 标准 | 证明它的确切命令 |
|---|---|
| A5 作者删项目后所有用户记录消失 | `go test ./internal/api/ -run TestDeleteProjectCascades -v` |
| A6 退订后仅本人历史消失 | `go test ./internal/api/ -run TestUnsubscribe -v` |
| 多链接导入逐条返回结果 | `go test ./internal/api/ -run TestImportMixedLinks -v` |
| 订阅不授予写权限（D4） | `go test ./internal/api/ -run TestSubscriberCannotModify -v` |
| 分享链接可订阅；重置后旧链接失效 | `go test ./internal/api/ -run TestShare -v` |

2. **手工端到端**（完整功能 5 闭环）：

```
注册 A、B → A 建项目 → A 做一轮 → A 生成分享链接
→ B 导入链接订阅 → B 做一轮 → A 改项目（B 应看到变化）
→ B 退订 → 查库确认：B 的 round/attempt 归零、A 的完好、项目仍在
```

3. **变异测试取证**：把退订实现改成「只删 subscription 行、不删 practice_round」，
   确认 A6 的测试失败；还原。

4. 更新工作记录并提交。

5. **Commit**：`test(share): P6 验收记录`

---

## Risks

| # | 风险 | 处置 |
|---|---|---|
| R-P6-1 | 退订漏删历史（只删了订阅行） | 单一原子操作 + 变异测试取证 |
| R-P6-2 | 退订误删他人记录 | 测试断言「他人轮次数不变」（A6 核心） |
| R-P6-3 | 多链接导入整批失败 | 逐条返回结果；测试混合有效/无效/跨实例 |
| R-P6-4 | 订阅意外授予写权限 | 测试断言订阅者修改项目得 403 |
| R-P6-5 | 分享 token 明文入库被误认为安全缺陷 | 代码注释说明：分享凭据需可反查，与会话令牌（哈希入库）的区别 |
| R-P6-6 | 链接解析过于严格导致用户粘贴失败 | 宽容处理空白与前后缀；错误信息指出具体哪一段不对 |
| R-P6-7 | 作者改项目后订阅者看到旧数据 | 订阅者读的就是 `project` 同一行（规格 §6.1(4)），无缓存；端到端验证 |

## Retirement

- **Old owner / fallback**：无。
- **Deletion trigger**：无。
- 明确拒绝：短链接服务、二维码生成、订阅通知、跨实例订阅。

## ADR Signals

本阶段是 **ADR-2「`(project, user)` 单一级联不变量」** 的落地点。
P6 完成后可回填：退订与删项目如何共用同一条外键规则、
为何不提供可单独调用的删除原语、分享 token 与会话令牌的存储差异。

---

## Execution Readiness View

```text
Execution Readiness View:
- Intent Lock: 交付功能 5 的完整闭环（分享 → 订阅 → 跟随 → 退订）
- Scope Fence: 不含跨实例订阅、二维码、订阅通知
- Baseline Lock: 规格 §4.4/§6.1/§7、D4；基线 §5.2(3)、§9(3)
- Approved Behavior: 退订是单一原子操作且只清本人；删项目仍清所有人；
  订阅不授予写权限；多链接导入逐条返回
- Owner / Contract Constraints: 数据库外键是级联唯一执行者；
  ProjectAccess 仍是访问权判定的唯一入口
- Compatibility Boundary: 不提供可单独调用的「删订阅行」；
  不改动既有端点语义
- Retirement Boundary: 无退役对象；拒绝短链接/二维码/通知/跨实例
- Task Batches: P6.1(分享) → P6.2(导入) → P6.3(退订) → P6.4(界面) → P6.5(验收)
- Test Obligations: 分享权限与重置失效、导入逐条结果、退订原子性与隔离性、
  订阅无写权限、前端解析
- Review Gates: 每 Task 后 go build/vet/test 或 analyze/test；P6.5 变异测试 + 端到端
- Drift / Rewind Rules: 若发现需要第二个删除入口，停止并回到规格 §6.1(2)
- Evidence Required Before Completion: 全量测试输出；A5/A6 专项；
  端到端查库结果；退订变异测试
- Advisory Boundary: method-pack execution guidance only; not GateDecision,
  PolicySnapshot, or completion authority
```

## Execution Route

```text
Execution Route:
- Decision: inline
- Evidence: P6.1→P6.2 依赖分享 token 存在；P6.3 独立于前两者但 P6.4 依赖全部；
  串行成本低于协调成本
- Fallback: 无
- User confirmation required: no
```

**REQUIRED SUB-SKILL**：aegis:executing-plans
