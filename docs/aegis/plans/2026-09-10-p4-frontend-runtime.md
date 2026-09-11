# 实施计划 —— P4 前端骨架与练习运行时

- Date: `2026-09-10`
- Design Spec: [`docs/aegis/specs/2026-09-10-mental-math-app-design.md`](../specs/2026-09-10-mental-math-app-design.md)
- Baseline: [`docs/aegis/baseline/2026-09-10-initial-baseline.md`](../baseline/2026-09-10-initial-baseline.md)
- 上游：[`evidence/p3.5/ACCEPTANCE.md`](../work/2026-09-10-mental-math-app/evidence/p3.5/ACCEPTANCE.md)
- 覆盖阶段：规格 §16 的 **P4 前端骨架与练习运行时**
- 后续：P5 统计页 → P6 分享订阅 → P7 CI 与交付

---

## Goal

让用户在 Android / 浏览器上完整走通：

**登录 → 看到项目列表 → 进入练习（自带键盘、可暂停、打错即时反馈）→ 总结页 → 错题页 → 重练错题 / 再来一轮**

交付物覆盖功能 3（三 Tab）、6（自带键盘/暂停/不保存）、7（总结页与错题页）、9（打错即时反馈），
外加为走通上述流程所必需的登录/注册界面。

**本计划不含**：统计页的图表与指标（P5，本阶段仅放只读占位）、
分享链接与订阅导入/退订（P6）、签名与发布流水线（P7）。

## Architecture

```
app/lib/
  main.dart                 应用入口与路由
  api/
    client.dart             dio 封装：baseUrl、token 注入、统一错误映射
    models.dart             与后端契约对应的数据模型
    auth_api.dart           注册/登录/登出/me/公开设置
    project_api.dart        项目 CRUD
    round_api.dart          /rounds/start、/rounds/complete
  state/
    session.dart            登录态与 token 持久化
    projects.dart           项目列表状态
    practice.dart           练习运行时状态机（核心）
  scoring/scoring.dart      跨端判分（P3.5 已交付）
  ui/
    app.dart                CupertinoApp 与主题
    auth_page.dart          登录/注册
    shell.dart              三 Tab 骨架（规格 §4.3）
    practice_tab.dart       项目列表 + 新建项目（练习 Tab）
    stats_tab.dart          只读占位（P5 填充）
    profile_tab.dart        用户信息与登出（分享/退订留 P6）
    project_editor_page.dart 新建/编辑项目（含规则编辑与 JSON 配置）
    practice_page.dart      练习运行时（键盘/暂停/反馈）
    summary_page.dart       总结页
    wrong_answers_page.dart 错题页与重练
    widgets/
      keypad.dart           自带键盘
      metric_tile.dart      指标显示
```

依赖方向：`ui → state → api → (network)`；`state → scoring`（判分）。
`ui` 不直接调 `api`，一律经 `state`，以便状态与副作用集中可测。

## Tech Stack

| 项 | 选择 | 依据 |
|---|---|---|
| UI | Cupertino 组件 | D10：用户明确要求 iOS 风格 |
| HTTP | `dio` ^5.11.1 | 已有依赖；拦截器便于统一注入 token 与错误映射 |
| 状态 | `flutter_riverpod` ^3.4.3 | 已有依赖；不依赖 BuildContext，便于单测 |
| 存储 | `shared_preferences` ^2.5.5 | 已有依赖；Web 与 Android 均可用 |
| 判分 | `app/lib/scoring/scoring.dart` | P3.5 已交付，由 13872 条语料钉死 |

**不新增任何依赖。**

## Baseline / Authority Refs

- 规格 §4.3（三 Tab 信息架构与职责不重叠）
- 规格 §6.1(5)（`server_is_correct` 权威；客户端判定仅作即时反馈）
- 规格 §7（API 契约全表）
- 规格 §9（练习运行时状态机、交互细节、自带键盘）
- 规格 §3 / D6（关掉 app 即丢弃，不提供续做）
- 基线 §5.2(6)(7)(8)（判分无浮点、清洗表单一下发、判分权威）
- 基线 §9（兼容边界 1–6）
- 证据：`evidence/p3.5/ACCEPTANCE.md`（清洗表与信封的实际形状）

## Compatibility Boundary

1. **客户端不得自行硬编码清洗规则**（基线 §5.2(7)）：必须应用
   `/rounds/start` 返回的 `scoring.cleanupTable`。P3.5 已有专门测试守住这一点。
2. **客户端判定只用于即时反馈**（规格 §6.1(5)）：界面显示的对错可以与服务端不同，
   但**落库以服务端为准**。总结页优先展示服务端返回的权威结果。
3. **关闭 app 不保存**（D6）：运行时全程内存态，不写本地存储、不做断点续做。
4. **判分调用必须与语料一致**：一律经 `scoring.compare`，不得在 UI 里另写比较逻辑。
5. **三 Tab 职责不重叠**（规格 §4.3）：订阅导入在练习 Tab，退订在我的 Tab；
   统计 Tab 无任何编辑入口。

## TDD Route

```text
TDD Route:
- Mode: off
- Decision: skipped
- Strict authority: not applicable
- Test posture: post-change regression
- Reason: 项目配置 tdd_mode = "off"，用户未要求严格 TDD
- Verification: flutter analyze ; flutter test
```

> 本计划为**练习运行时状态机**与**即时反馈判分**编写 widget/单元测试：
> 前者是功能 6/7/9 的行为核心，后者直接决定用户看到的对错。
> 属 proportional regression。

## Verification（全局命令）

```powershell
cd D:\Desktop\Cala\app
flutter analyze
flutter test
flutter build web          # 开发期浏览器调试路径可用
```

后端联调（手动）：

```powershell
cd D:\Desktop\Cala\backend ; go run ./cmd/server
cd D:\Desktop\Cala\app     ; flutter run -d chrome --dart-define=CALA_API=http://127.0.0.1:8080
```

---

## Scope Check

**Aegis Visibility**：本阶段落地的是用户**直接感知**的行为（打错立即反馈、暂停、
总结数字、错题重练），且判分结论会经由既有跨端契约产生落库后果。因此把
「客户端只应用服务端下发的清洗表」「判分只走 scoring.compare」写成测试，
并在运行时状态机（暂停/计时/推进）上写回归测试，避免出现
「界面显示答对但落库为错」或「暂停了还在计时」这类静默错误。

```text
Requirement Ready Check:
- Requirement source refs: 用户功能 3/6/7/9；规格 §4.3、§7、§9
- Goals and scope refs: 规格 §16 P4 行
- User / scenario refs: 已登录用户在手机上完成一轮练习
- Requirement item refs: 3（三 Tab）、6（键盘/暂停/不保存）、7（总结/错题/重练）、9（即时反馈）
- Acceptance / verification criteria refs: A9（端到端可走通）、A3/A7 的前端侧体现
- Open blocker questions: 无
- Decision: ready
```

```text
Change Necessity:
- User-visible need: 当前只有后端与判分库，没有任何可交互界面
- No-change / non-code option: 不存在——功能 3/6/7/9 全部是前端行为
- Why code change is necessary: 无既有前端实现可复用（仅 Flutter 骨架）
- Minimum change boundary: 新增 app/lib 下的 api/state/ui 三层与对应测试；
  不改动 scoring.dart 与后端
- Decision: code-change
```

```text
Existence Check:
- Proposed new surface: ①api 层 ②state 层 ③ui 层各页面
- Existing owner / reuse candidate: scoring.dart（P3.5）与 main.dart（骨架）
- Why existing surface is insufficient:
  ① 无网络层，无法与后端通信；
  ② 状态若散在各页面 setState 中，练习运行时（暂停/计时/推进）无法被独立测试；
  ③ 功能 3/6/7/9 需要具体界面
- Creation proof: 规格 §4.3 与 §9 明确要求这些界面与行为；
  状态层是为「可测试的运行时状态机」而设，不是为抽象而抽象
- Entropy / retirement impact: 无旧路径退役。**明确拒绝**：不引入代码生成
  （riverpod_generator）、不引入路由框架（go_router）、不引入图表库（P5 再议）、
  不引入本地数据库（D6 明确不保存未完成练习）
- Decision: add-with-proof
```

```text
Architecture Integrity Lens:
- Invariant: 客户端判分只能经 scoring.compare 且只用服务端下发的清洗表
  （基线 §5.2(7)、§9(5)）
- Canonical owner / contract: `lib/scoring/scoring.dart` 是客户端判分唯一实现；
  `state/practice.dart` 是运行时状态的唯一 owner
- Responsibility overlap: 无——UI 不写判分逻辑，只读状态层结论
- Higher-level simplification: 把「题目 + 作答 + 计时 + 暂停」收敛为一个不可变快照状态，
  UI 只渲染快照；避免「多处 setState 各改一半」导致的计时/进度不一致
- Retirement / falsifier: 若有人在 UI 里另写一个字符串比较，会出现与语料不一致的判定
  → 由「练习页不包含判分实现」的结构约束 + 运行时测试守护
- Verdict: 无阻塞问题
```

```text
Plan Pressure Test:
- Owner / contract / retirement: 承载点已识别（判分调用、清洗表来源、运行时状态机）
- Architecture integrity / higher-level path: 快照式状态机
- Verification scope: 每个 Task 有 analyze + test；P4.8 端到端手工验证
- Task executability: 每步含完整代码与确切命令
- Pressure result: proceed
```

```text
Plan-Time Complexity Check:
- Target files: app/lib 下约 18 个新文件，最大者为 practice.dart（状态机）与 practice_page.dart
- Existing size / shape signals: 无既有压力（当前仅 main.dart 与 scoring.dart）
- Owner fit: 单文件职责清晰
- Add-in-place risk: 若把状态机与页面合在一个文件，会同时承担「逻辑」与「渲染」两项职责，
  且无法脱离 Widget 树测试
- Better file boundary: state/practice.dart 只放纯 Dart 状态机；
  ui/practice_page.dart 只做渲染与手势
- Recommendation: add owner file
```

---

## Task P4.1 —— API 客户端与数据模型

**Files**
- Create: `app/lib/api/models.dart`
- Create: `app/lib/api/client.dart`
- Create: `app/lib/api/auth_api.dart`
- Create: `app/lib/api/project_api.dart`
- Create: `app/lib/api/round_api.dart`
- Create: `app/test/api/client_test.dart`

**Why**：所有页面都要经此访问后端。统一处理 token 注入与错误映射，
避免每个页面各写一遍 `try/catch` 与状态码判断。

**Change Necessity**：`code-change`；最小边界 = 新增 `api` 层。

**Impact / Compatibility**：错误映射必须与后端 §7 的
`{"error":{"code","message"}}` 契约一致（P0.4 已确立）。

**Steps**

1. 写 `models.dart`：`User`、`Project`、`Question`、`Envelope`（复用 scoring 的）、
   `ScoringConfig`、`StartRoundResult`、`AttemptInput`、`CompleteRoundResult`。
   字段名与后端 JSON 严格对应（后端用 camelCase tag）。

2. 写 `client.dart`：

```dart
/// 统一 API 客户端。
///
/// 职责：baseUrl 解析、token 注入、后端错误契约 -> 异常映射。
/// 各页面不得自行拼 URL 或解析错误体。
class ApiClient {
  ApiClient({required this.baseUrl, Dio? dio}) : _dio = dio ?? Dio() { ... }

  /// 后端统一错误契约：{"error":{"code","message"}}
  static ApiException toException(DioException e) { ... }
}

class ApiException implements Exception {
  final String code;     // bad_request / unauthorized / forbidden / not_found /
                         // conflict / registration_closed / rule_invalid / internal
  final String message;
  final int? status;
  bool get isUnauthorized => status == 401;
  bool get isRuleInvalid => code == 'rule_invalid';
  ...
}
```

**baseUrl 来源**：`--dart-define=CALA_API=...`，默认 `http://127.0.0.1:8080`。
浏览器调试时后端已放行 localhost 来源（D9 的 CORS 配置）。

3. 测试（`client_test.dart`）——**必须覆盖**：

- 正常响应解析
- 后端错误体 → `ApiException` 且 `code`/`message` 正确
- 无 error 体的 500 → 仍抛出 `ApiException` 而非崩溃
- 网络超时 → `ApiException`（`code == 'network'`）
- token 存在时注入 `Authorization: Bearer <token>`；不存在时不注入

用 `dio` 的 `HttpClientAdapter` 替身，不发真实请求（测试必须离线可跑）。

4. 验证：`flutter analyze ; flutter test test/api/`

5. **Commit**：`feat(app): API 客户端与数据模型`

---

## Task P4.2 —— 登录态与注册/登录页

**Files**
- Create: `app/lib/state/session.dart`
- Create: `app/lib/ui/auth_page.dart`
- Modify: `app/lib/main.dart`
- Create: `app/test/state/session_test.dart`

**Why**：没有 token 无法访问任何端点。

**Change Necessity**：`code-change`。

**Impact / Compatibility**：注册开关关闭时**首个用户仍应能注册**（规格 §6.2）。
界面需在开关关闭且系统已有用户时隐藏注册入口，但**不得**隐藏首个用户的入口——
否则全新部署会自我锁死（与服务端同一条规则，此处是它的界面侧体现）。

**Steps**

1. `session.dart`：`SessionNotifier`（Riverpod）持有 `token`/`user`，
   启动时从 `shared_preferences` 读取，登录/注册/登出时写入或清除。
   暴露 `registrationOpen` 供注册入口显隐。

2. `auth_page.dart`：`CupertinoTextField` 用户名/密码，登录与注册两个动作。
   错误以 `CupertinoAlertDialog` 展示后端返回的 `message`（而非「操作失败」）。

3. 测试：

- 未登录时 `me()` 失败 → token 被清除（避免卡在无效登录态）
- 登录成功 → token 持久化；登出 → token 清除
- **注册入口显隐**：`registrationOpen=false` 且已有用户时隐藏；
  `registrationOpen=false` 但系统无用户时**仍显示**（防锁死）

4. **Commit**：`feat(app): 登录态与注册登录页`

---

## Task P4.3 —— 三 Tab 骨架

**Files**
- Create: `app/lib/ui/app.dart`
- Create: `app/lib/ui/shell.dart`
- Modify: `app/lib/main.dart`

**Why**：功能 3。规格 §4.3 明确了三 Tab 的职责边界，本任务把它落成结构，
使后续页面各归其位而不是随手塞进「练习」Tab。

**Change Necessity**：`code-change`。

**Impact / Compatibility**：承载规格 §4.3 的职责不重叠约束（基线 §9(6) 的界面侧）。

**Steps**

1. `shell.dart`：`CupertinoTabScaffold` + `CupertinoTabBar`，三个 Tab：
   练习（`practice_tab.dart`）/ 统计（`stats_tab.dart`）/ 我的（`profile_tab.dart`）。
   未登录时显示 `auth_page.dart`（不显示 Tab 栏）。

2. `app.dart`：`CupertinoApp` + `CupertinoThemeData`（`brightness: light`），
   统一 `CupertinoPageScaffold` 的导航栏样式。

3. 测试：登录态下三个 Tab 都能渲染；未登录时显示登录页且**不显示** Tab 栏。

4. **Commit**：`feat(app): Cupertino 三 Tab 骨架`

---

## Task P4.4 —— 练习 Tab：项目列表与新建项目

**Files**
- Create: `app/lib/state/projects.dart`
- Create: `app/lib/ui/practice_tab.dart`
- Create: `app/lib/ui/project_editor_page.dart`
- Create: `app/test/state/projects_test.dart`

**Why**：功能 3 的练习 Tab；也是进入练习的唯一入口。

**Change Necessity**：`code-change`。

**Impact / Compatibility**：新建项目必须能给出**规则错误的可读原因**
（后端返回 `rule_invalid` 并带 detail），否则作者无法修正规则。

**Steps**

1. `projects.dart`：加载 `owned` 与 `subscribed`（后端已分组返回），
   暴露 `create` / `update` / `delete` / `refresh`。

2. `practice_tab.dart`：
   - 分组列表：我的项目 / 已订阅
   - 右上 `+` → `CupertinoActionSheet`：新建项目 / 导入订阅链接（**P6 前禁用并标注**）
   - 点击项目 → 进入练习

3. `project_editor_page.dart`：
   - 标题、描述、每轮题数（`CupertinoSlider` 或步进器）
   - 规则源码：`CupertinoTextField`（多行、等宽字体）
   - 配置 JSON：`CupertinoTextField`
   - 容差：开关 + 分子/分母输入（对应迁移 0002 的两列）
   - 保存时把 `rule_invalid` 的 message **原样展示**，便于作者定位

4. 测试：项目列表分组渲染；创建成功后列表刷新；
   规则错误时展示后端 message 且不跳转。

5. **Commit**：`feat(app): 练习 Tab 项目列表与项目编辑`

---

## Task P4.5 —— 练习运行时状态机（纯 Dart，可单测）

**Files**
- Create: `app/lib/state/practice.dart`
- Create: `app/test/state/practice_test.dart`

**Why**：功能 6/7/9 的行为核心。把「题目序列 + 作答 + 计时 + 暂停 + 推进」
收敛为一个纯 Dart 状态机，使暂停是否真的停表、答错是否停住等待确认、
最后一题何时结束，都能在不启动 Widget 树的情况下断言。

**Change Necessity**：`code-change`。

**Impact / Compatibility**：判分**必须**调用 `scoring.compare`（基线 §9(5)）。

**Steps**

1. 状态机设计要求（实现时逐条落实，并有对应测试）：

```dart
/// 一道题的作答结果。
class AnswerRecord {
  final int index;
  final String input;        // 用户原始输入（落库用，不规范化）
  final bool clientCorrect;  // 客户端判定，仅用于即时反馈
  final int elapsedMs;
}

/// 练习运行时状态（不可变快照）。
class PracticeState {
  final List<Question> questions;
  final Map<String, String> cleanupTable;  // 来自 /rounds/start
  final int seed;
  final int current;                       // 当前题号
  final List<AnswerRecord> answers;
  final bool paused;
  final bool awaitingNext;   // 答错后等待「下一题」
  final bool finished;
  final int questionElapsedMs;             // 当前题已耗时
  final int totalElapsedMs;
}
```

行为约定（每条都要有测试）：

| 行为 | 约定 |
|---|---|
| 输入数字/./-/`/` | 追加到当前输入，长度上限 20 |
| 退格 | 删除末位；空输入时为 no-op |
| 提交 | 用 `scoring.compare` 判分；答对**不停留**，直接进入下一题；答错则置 `awaitingNext=true` 并停住 |
| 答错后「下一题」 | 清除 `awaitingNext`，推进到下一题 |
| 最后一题结束 | 置 `finished=true`，不再接受输入 |
| 暂停 | `paused=true`：**停止计时**、清空当前输入、遮蔽题面 |
| 继续 | `paused=false`：从暂停处继续计时，用户需重新输入（暂停时输入已清空） |
| 关闭 app | 无任何持久化（D6）——状态机不引用 `shared_preferences` |

**暂停为什么清空输入**：避免「暂停时偷看题目再继续」毫无代价；
更实际的是避免暂停期间输入与计时状态不一致。

2. 计时实现：状态机不自己持有 `Timer`（那会使其难以测试）。
   改为由调用方驱动：`tick(int deltaMs)` 累加时间，暂停时**忽略** tick。
   这样「暂停是否真的停表」可以用纯函数断言，无需等待真实时钟。

3. 测试必须覆盖：
   - 答对自动推进；答错停住并置 `awaitingNext`
   - 答错后按「下一题」才推进
   - 暂停期间 `tick` 不累加计时；继续后恢复累加
   - 暂停清空当前输入
   - 最后一题后 `finished=true` 且拒绝后续输入
   - 判分确实走 `scoring.compare`（用「全角数字 + 服务端清洗表」验证：
     输入 `１２３` 对答案 `123` 应判对——若 UI 自己写比较就会失败）
   - 输入长度上限

4. 验证：`flutter test test/state/practice_test.dart`

5. **Commit**：`feat(app): 练习运行时状态机`

---

## Task P4.6 —— 练习页、自带键盘与总结页

**Files**
- Create: `app/lib/ui/widgets/keypad.dart`
- Create: `app/lib/ui/practice_page.dart`
- Create: `app/lib/ui/summary_page.dart`
- Create: `app/lib/api/round_api.dart`（若 P4.1 未含则在此补全）
- Create: `app/test/ui/practice_page_test.dart`

**Why**：功能 6/9 的界面；功能 7 的总结页。

**Change Necessity**：`code-change`。

**Impact / Compatibility**：总结页优先展示**服务端返回的权威结果**
（规格 §6.1(5)）；若客户端与服务端判定不一致，需如实提示而非静默采用其一。

**Steps**

1. `keypad.dart`：`1-9` `0` `.` `-` `/` `⌫` `确定`。
   字母表与规格 §5.5.4 一致（`0-9 . - /`）。开发期同时接受物理键盘
   （`Focus` + `onKeyEvent`，便于浏览器调试）。

2. `practice_page.dart`：
   - 顶部：进度 `3/20`、已用时、暂停按钮
   - 中部：题面（大字号、居中）
   - 下部：当前输入回显
   - 答错：显示正确答案 + 「下一题」按钮（功能 9 原文）
   - 答对：短暂高亮（约 200ms）后自动推进
   - 暂停：`CupertinoAlertDialog` 或遮蔽层，含「继续」与「放弃本轮」
   - 计时：`Timer.periodic(100ms)` 驱动状态机 `tick`

3. 完成一轮 → `POST /rounds/complete` → `summary_page.dart`：
   - 总耗时、平均每题耗时、正确率（功能 7）
   - 点击错题数 → 错题页
   - 「再来一轮」→ 同项目新 seed（重新 `/rounds/start`）
   - 若 `staleProject` 为 true，提示「项目在本次练习期间被作者修改」

4. 测试（widget test）：
   - 键盘输入 `12` 后题面回显 `12`；退格后为 `1`
   - 答错显示正确答案与「下一题」
   - 暂停后计时数字不再变化

5. **Commit**：`feat(app): 练习页、自带键盘与总结页`

---

## Task P4.7 —— 错题页与重练

**Files**
- Create: `app/lib/ui/wrong_answers_page.dart`
- Modify: `app/lib/ui/summary_page.dart`

**Why**：功能 7 的错题页与「重练错题」「再来一轮」。

**Change Necessity**：`code-change`。

**Impact / Compatibility**：**重练错题读的是本题已落库的快照**
（`q_snapshot`/`a_snapshot`），不再调用规则重新出题。
这保证作者事后改规则不会改变历史错题（规格 §6.1(3)）。

**Steps**

1. 错题来源：本轮 `attempts` 中 `server_is_correct=false` 的记录。
   接口：`GET /api/rounds/:id/attempts`（**若后端尚未提供，本任务需补**——
   见下方「后端补充」）。

2. 错题页内容：列出题面、你的作答、正确答案。
   两个按钮：
   - **重练错题**：用**快照**构造一轮新练习（不调用 `/rounds/start`），
     题序为错题原序。判分仍用 `scoring.compare` + 原信封。
   - **再来一轮**：回到总结页同项目开新一轮（调用 `/rounds/start`）。

3. 测试：只列出判错的题；重练时题目内容与快照逐字一致。

4. **后端补充（本任务内含）**：新增
   `GET /api/rounds/:id/attempts`（仅本人可读），返回该轮全部 attempt。
   属小改：`store.ListAttemptsByRound` 已存在（P3.4 交付），只需加 handler 与授权。

5. **Commit**：`feat(app): 错题页与重练错题`

---

## Task P4.8 —— 我的 Tab、统计占位与验收

**Files**
- Create: `app/lib/ui/profile_tab.dart`
- Create: `app/lib/ui/stats_tab.dart`
- Modify: `app/lib/ui/shell.dart`

**Why**：功能 3 的第三个 Tab；统计页在 P5 填充，本阶段只放只读占位，
以固化「统计 Tab 无编辑入口」这一职责边界（规格 §4.3）。

**Steps**

1. `profile_tab.dart`：用户名、是否管理员、登出按钮；
   我创建的项目（管理/删除）；我订阅的项目（退订入口标注「P6」）。
   管理员可见注册开关（`PUT /api/admin/settings`）。
2. `stats_tab.dart`：项目选择器 + 明确的「统计将在后续版本提供」占位，
   **不放置任何编辑入口**。
3. 验收：
   - `flutter analyze` 无 issue
   - `flutter test` 全通过
   - `flutter build web` 成功
   - **手工端到端**：起后端 → 浏览器中注册 → 建项目 → 练习 → 暂停/继续 →
     答错见反馈 → 总结 → 错题 → 重练 → 再来一轮
   - 结果写入 `docs/aegis/work/.../evidence/p4/ACCEPTANCE.md`
4. **Commit**：`test(app): P4 验收记录`

---

## Risks

| # | 风险 | 处置 |
|---|---|---|
| R-P4-1 | UI 里另写判分逻辑，与服务端漂移 | 判分只经 `scoring.compare`；用「全角数字 + 服务端清洗表」用例守护（UI 自写比较会失败） |
| R-P4-2 | 暂停后计时仍在走 | 状态机不持有 `Timer`，由 `tick` 驱动且暂停时忽略；可用纯函数断言 |
| R-P4-3 | 关闭 app 后残留状态（违反 D6） | 运行时状态全在内存；状态机不引用 `shared_preferences`；仅 token 持久化 |
| R-P4-4 | 总结页采用客户端判定而与服务端不符 | 总结页以 `/rounds/complete` 返回的权威值为准；不一致时如实提示 |
| R-P4-5 | 浏览器 CORS 未放行导致调试受阻 | 后端已按 D9 放行 localhost 来源（`CALA_DEV_CORS_ORIGINS`）；默认含 3000 端口，需按实际端口调整 |
| R-P4-6 | widget test 与真实手势差异 | 关键行为由纯 Dart 状态机测试覆盖，widget test 只验证渲染与输入回显 |

## Retirement

- **Old owner / fallback**：无。全部新增；无兼容分支。
- **Deletion trigger**：无。
- 明确拒绝：代码生成、路由框架、图表库、本地数据库。

## ADR Signals

P4 不产生新 ADR。它是 §4.3（三 Tab 职责）与 §9（运行时行为）的落地。

---

## Execution Readiness View

```text
Execution Readiness View:
- Intent Lock: 交付可走通的练习闭环界面（功能 3/6/7/9）+ 登录注册
- Scope Fence: 不含统计图表（P5）、分享订阅（P6）、发布流水线（P7）
- Baseline Lock: 规格 §4.3/§6.1(5)/§7/§9、§3(D6)；基线 §5.2(6)(7)(8)、§9
- Approved Behavior: 判分只经 scoring.compare 且只用服务端清洗表；
  暂停停表；答错停住等确认；关闭 app 不保存；总结以服务端权威值为准
- Owner / Contract Constraints: scoring.dart 是客户端判分唯一实现；
  state/practice.dart 是运行时唯一 owner；ui 不直接调 api
- Compatibility Boundary: 基线 §9(1)(5)(6) 的界面侧体现
- Retirement Boundary: 无退役对象；拒绝代码生成/路由框架/图表库/本地库
- Task Batches: P4.1(api) → P4.2(session) → P4.3(shell) → P4.4(列表/编辑)
  → P4.5(状态机) → P4.6(练习页/总结) → P4.7(错题/重练) → P4.8(我的/验收)
- Test Obligations: 客户端错误映射、token 注入、注册入口显隐（含防锁死）、
  状态机 8 项行为、键盘输入回显、错题筛选、重练快照一致性
- Review Gates: 每 Task 后 analyze + test；P4.8 端到端手工验证
- Drift / Rewind Rules: 若需在 UI 内实现判分或另存状态，停止并回到本计划的所有权约束
- Evidence Required Before Completion: analyze 无 issue、test 全通过、
  build web 成功、端到端全流程截图或命令输出
- Advisory Boundary: method-pack execution guidance only; not GateDecision,
  PolicySnapshot, or completion authority
```

## Execution Route

```text
Execution Route:
- Decision: inline
- Evidence: P4.1→P4.2→P4.3 为严格前置；P4.4/P4.5 可并行但共享 state 目录，
  且 P4.6 同时依赖两者；串行成本低于协调成本
- Fallback: 无
- User confirmation required: no
```

**REQUIRED SUB-SKILL**：aegis:executing-plans
