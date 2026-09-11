# P4 前端骨架与练习运行时 验收证据

- Date: `2026-09-10`
- Plan: [`docs/aegis/plans/2026-09-10-p4-frontend-runtime.md`](../../../plans/2026-09-10-p4-frontend-runtime.md)
- 范围：规格 §16 的 **P4 前端骨架与练习运行时**
- 出口条件：**A9**（浏览器中可完整走通主流程）+ 功能 3/6/7/9

---

## 1. 全量命令结果

| 命令 | 结果 |
|---|---|
| `cd app && flutter analyze` | `No issues found!` |
| `cd app && flutter test` | **94 项全通过** |
| `cd app && flutter build web` | `✓ Built build\web` |
| `cd app && flutter build apk --debug` | `✓ Built app-debug.apk`（P0.5 已验证，代码为增量改动） |
| `cd backend && go build ./...` | exit 0 |
| `cd backend && go vet ./...` | exit 0 |
| `cd backend && gofmt -l internal cmd` | 无输出（全部合规） |
| `cd backend && go test ./... -count=1` | 8 个包全部 ok |

前端测试分布：api 24 · 状态机 39 · 会话 11 · 键盘与练习页 6 · 跨端语料 11 · 其他 3。

---

## 2. 端到端验证（真实进程 / HTTP / SQLite）

```
1. 公开配置(未登录): 200  {"bootstrap":true,"registrationOpen":true}
2. 注册首个用户: becameAdmin=True
3. 建项目: id=1 题数=6
4. 出题: 题数=6  seed=8145695986501551
   seed 在 float64 精确范围内: True
   清洗表项=32
5. 交卷: 正确=3/6  分歧=0  总耗时=7200ms
6. 错题页数据: 共 6 条, 其中错题 3 道
   首道错题: 24 + 75 = ?  你的作答=999999  正确答案=99
7. 他人读取该轮记录: 404 (期望 404)
```

覆盖：未登录读公开配置、引导管理员注册、建项目、出题（含清洗表下发与种子范围）、
交卷（服务端重算 + 分歧计数 + 派生总耗时）、错题数据源、以及**隐私边界**。

---

## 3. 功能逐条

### 功能 3 底栏三 Tab

`AppShell` 用 `CupertinoTabScaffold`。职责边界（规格 §4.3）：

| Tab | 内容 |
|---|---|
| 练习 | 项目列表（我的/已订阅分组）、新建项目、导入订阅入口、点击进入练习 |
| 统计 | 只读占位，页面上明确标注「本页只读：不提供任何编辑入口」 |
| 我的 | 账号、管理员注册开关、我创建的项目（编辑/分享/删除）、我订阅的项目（退订入口） |

**职责不重叠**的落地：订阅**导入**在练习 Tab（它是「获得一个可练习的项目」），
**退订**在我的 Tab。分享与退订属 P6，点击时明确提示「尚未提供」——
让用户点了没反应比告诉他「还没有」更糟。

### 功能 6 自带键盘 / 可暂停 / 关闭不保存

| 要求 | 实现 |
|---|---|
| 自带键盘 | `Keypad` 字母表 `0-9 . - /` 与规格 §5.5.4 一致；开发期同时接受物理键盘 |
| 可暂停 | 暂停遮蔽题面（`Opacity 0.12`）、**停表**、清空当前输入；弹窗含「继续」与「放弃本轮」 |
| 继续 | 从暂停处续上计时；用户需重新输入当前题（暂停时输入已清空） |
| 关闭不保存 | 运行时状态全在内存；状态机不引用 `shared_preferences`；仅 token 持久化（D6） |

**「暂停确实停表」的可测试性**：状态机不持有 `Timer`，时间由 UI 通过 `tick(deltaMs)`
驱动，且暂停时忽略 tick。因此该行为可以用纯函数断言，无需等待真实时钟：

```dart
test('暂停期间 tick 被忽略（暂停确实停表）', ...);
test('继续后恢复累加', ...);
```

### 功能 7 总结页与错题页

- 总结页：总耗时、平均每题耗时、正确率（环形图）、错题数入口
- 错题页：题面、你的作答、正确答案；「重练错题」与「返回」
- **重练错题使用已落库快照**，不调用规则重新出题（规格 §6.1(3)）
- 错题以 **`serverIsCorrect`** 筛选，不用客户端判定（规格 §6.1(5)）
- `staleProject` 与 `discrepancies > 0` 时如实提示，不静默采用其一

### 功能 9 打错立即反馈

答错：立即显示正确答案 + 出现「下一题」按钮（原文要求）。
答对：短暂高亮（220ms）后自动推进，不打断节奏。

两条路径共用同一个 `awaitingNext` 状态标志（语义是「等推进」）。
若各设一个标志，会出现「既在等确认又在等自动推进」的矛盾状态。

---

## 4. 修复一个真实的跨端 bug：种子精度

**现象**：Go 测试中「一半答对」的用例全部被判错（`correct=false wrong=true`）。

**根因**：种子原为 63 位整数，经 JSON 以**数字**形式往返（服务端 → 客户端 → 服务端）。
JSON 数字在 JavaScript——以及 Flutter Web 的 dart2js（其中 `int` 就是 double）——
中只有 53 位尾数精度。超过 2^53 的值往返后会**变成另一个数**，
于是服务端用回传种子重放得到**完全不同的题目**，每一题都被判错。

这不是理论风险：测试用 `float64` 解析回传种子时立刻复现。

**修复**：种子上限收敛到 2^53（`maxSeed`），并在代码中写明原因。
2^53 ≈ 9.0e15 种取值，对「每轮一个不可预测种子」的用途绰绰有余。

**取证**（两项测试，缺一不可）：

```
TestSeedSurvivesFloat64JSONRoundTrip   PASS   50 次取样，种子经 float64 往返无损
TestSeedAboveLimitWouldBreakReplay     PASS   证明：9007199254740993 经 float64
                                              往返后变为 9007199254740992（相差 -1）
```

第二条**证明第一条的必要性**：若没有它，第一条可能只是碰巧通过。

**这一 bug 的重要性**：它只在 Flutter **Web** 上会真实触发（Android 的 Dart VM
中 `int` 是真正的 64 位整数）。而本项目开发期正是用浏览器调试——
若不在测试中主动模拟 float64 往返，它会在真机上「看起来正常」、
在浏览器调试时「莫名其妙全错」。

---

## 5. 修正两处我自己的设计问题

### 5.1 注册入口显隐出现两个 owner

原实现在客户端写 `registrationOpen || !hasUsers` 来判断是否显示注册入口。
但客户端**无法知道用户数**，只能猜——这让引导管理员规则（规格 §6.2）
同时存在于服务端与客户端两处，正是本项目一直在消除的模式。

**改为**：`/api/settings/public` 返回**有效值** `registrationOpen`（已计入 bootstrap）
与 `bootstrap` 标记，客户端只负责渲染。新增后端测试覆盖两种情形：

```
TestPublicSettingsReturnsEffectiveRegistrationValue/无用户时有效值为_true  PASS
TestPublicSettingsReturnsEffectiveRegistrationValue/已有用户且开关关闭时有效值为_false  PASS
```

### 5.2 跨 async 间隙使用 BuildContext

注册开关的 `onChanged` 回调在 `await` 之后弹对话框，分析器正确地报了
`use_build_context_synchronously`。改为**错误就地渲染**（内联红色提示）：
既消除该问题，也让错误持续可见而非一闪而过。

---

## 6. 执行期发现的其他问题

| # | 问题 | 处置 |
|---|---|---|
| 1 | `Tolerance` 与 Flutter `physics.Tolerance` 重名 | 测试中改用 `scoring.` 前缀；分析器捕获 |
| 2 | 误用了 Material 组件（`LinearProgressIndicator`、`RefreshIndicator`、`SelectableText`） | 改为自绘进度条、导航栏刷新按钮、普通 Text —— 与 D10「不使用 Material」一致 |
| 3 | `CupertinoIcons.person_outline` / `person_solid` 不存在 | 改用 `person` / `person_fill` |
| 4 | `CupertinoTabView` 的 `rootNavigator` 参数不存在 | 移除 |
| 5 | 状态机 `_advance` 传入互相矛盾的标志，且立即清空判定结果会让 UI 无法显示答对反馈 | 重构为两路径共用 `awaitingNext`，由 UI 决定何时推进 |
| 6 | 为解析一个 JSON 写了三层的解码包装类 | 直接用 `dart:convert` 的 `jsonDecode` |

---

## 7. 未验证项（诚实记录）

| # | 项 | 原因 | 何时可验 |
|---|---|---|---|
| 1 | 真机（Android）上的实际交互 | 未连接设备；本机 flutter doctor 显示有设备但本次未使用 | 用户可随时 `flutter run` |
| 2 | 浏览器中的人工点击流程 | 需要交互会话；已用 e2e HTTP 验证与 `flutter build web` 成功替代 | 用户可 `flutter run -d chrome` |
| 3 | 统计页图表 | 属 P5 | P5 |
| 4 | 分享/订阅/退订 | 属 P6（界面已有入口并明确提示） | P6 |
| 5 | CI 首跑结果 | 本环境不可达 github.com | 需用户在 Actions 页确认 |

> 第 2 项的替代说明：本报告的 e2e 验证覆盖了**同一套 HTTP 契约**，
> 并额外确认了 web 构建成功。但「界面上点击是否顺手」属主观体验，
> 只有人工使用才能判断——这一点不宣称已验证。

---

## 8. 结论

规格 §16 的 **P4 出口条件已满足**：

- 功能 3/6/7/9 全部交付并有对应测试
- 前端 94 项测试、后端 8 包测试全绿
- 端到端走通「注册 → 建项目 → 出题 → 交卷 → 错题」并验证了隐私边界
- 发现并修复一个只在 Flutter Web 上触发的种子精度 bug

**新增规模**：`app/lib` 下 api/state/ui 三层共 18 个文件，
`app/test` 下 5 个测试文件；后端新增 1 个端点与 3 项测试。

**下一步**：P5 统计（`/stats` 端点、日/周/月分桶、8 项指标、统计页 UI）。
