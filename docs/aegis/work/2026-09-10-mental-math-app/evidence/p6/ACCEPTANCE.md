# P6 分享订阅与退订 验收证据

- Date: `2026-09-10`
- Plan: [`docs/aegis/plans/2026-09-10-p6-share-subscription.md`](../../../plans/2026-09-10-p6-share-subscription.md)
- 范围：规格 §16 的 **P6 分享订阅与退订**
- 出口条件：**A5**（删项目级联全员）与 **A6**（退订只清自己）+ 功能 5

---

## 1. 全量命令结果

| 命令 | 结果 |
|---|---|
| `cd backend && go build ./...` | exit 0 |
| `cd backend && go vet ./...` | exit 0 |
| `cd backend && gofmt -l internal cmd` | 无输出 |
| `cd backend && go test ./... -count=1` | **9 个包全部 ok** |
| `cd app && flutter analyze` | `No issues found!` |
| `cd app && flutter test` | **117 项全通过** |

新增测试：store 订阅与分享 8 项、api 分享/导入/退订 32 项、前端解析 13 项。

---

## 2. 端到端验证（完整功能 5 闭环）

```
1. 注册 alice / bob / carol
2. alice 建项目 id=1
3. alice 做 2 轮
4. alice 生成分享链接:
   cala://subscribe?h=127.0.0.1:18100&t=8Ui8AM_dN47YH5AAyWbya9_Br8Mjn3tA195O5E9_Ikk
5. bob 导入 3 条: 成功 1/3
     ok=True  title=口算
     ok=False 链接无效或已被撤销
     ok=False 链接属于其他服务器（链接指向 other.host，当前为 127.0.0.1:18100）
6. bob 做 3 轮
7. bob 统计轮数=3 / alice 统计轮数=2   <- 隐私：各自只看自己
8. alice 改项目 -> bob 看到 questionCount=6   <- 订阅者跟随作者配置
9. bob 退订: 200 deletedRounds=3
10. bob 退订后:
    再练习 -> 403                    <- 访问权确实被终止
    alice 统计仍为 2 轮              <- 未被误删（A6 核心）
    项目仍在 -> 200
```

**编码确认**：错误文案按原始字节严格 UTF-8 解码通过，内容为
`链接无效或已被撤销` 与 `链接属于其他服务器（链接指向 other.host，当前为 127.0.0.1:18101）`
—— 跨实例错误同时指出链接指向哪、当前是哪，用户能自己判断问题。

---

## 3. 验收标准

### A6 退订后仅本人历史消失

**数据层**（`store` 包，直接验证语义）—— `TestUnsubscribeIsolation` PASS：

| 断言 | 结果 |
|---|---|
| `deletedRounds` = 3（界面据此提示代价） | ✅ |
| 本人的 `practice_round` 归零 | ✅ |
| 本人的 `attempt` 归零（**外键级联**，应用层未重复写删除） | ✅ |
| 作者的 2 轮完好 | ✅ |
| 其他订阅者的 4 轮完好 | ✅ |
| 只删本人那条订阅行，他人订阅不受影响 | ✅ |
| 项目本身仍在 | ✅ |

**HTTP 层** —— `TestUnsubscribeClearsOnlyOwnHistory` PASS（三人各做 2/3/4 轮，
bob 退订后断言同样七条），另有：

- `TestUnsubscribeDoesNotAffectOtherProjects`：退订项目一不影响项目二的 5 轮记录
- `TestUnsubscribeWithoutSubscriptionFails`：未订阅时返回 ErrNotFound
  （**静默成功会让界面显示「已退订」而实际什么都没发生**）
- `TestUnsubscribeTwiceFails`：重复退订明确报错
- `TestUnsubscribeDoesNotTouchProjectOwner`：作者对该项目仍有 owner 访问权
- `TestUnsubscribeThenResubscribe`：可重新订阅，但**不恢复已删除的历史**

### A5 作者删项目后所有用户记录消失

P3 已交付并由外键保证，P6 未改动该路径。回归确认：
`TestCascadeOnProjectDelete`（store）与 `TestDeleteProjectCascadesOverHTTP`（api）均 PASS。

**关键点**：删项目与退订是**两个不同的操作**，但由**同一条**外键规则覆盖
（规格 §6.1(2)）——应用层没有为两者各写一套删除逻辑。

---

## 4. 变异测试取证

把 `Unsubscribe` 改成「只删订阅行、不清轮次」后：

```
subscription_test.go:89: deletedRounds = 0, 期望 3（界面据此提示代价）
subscription_test.go:94: bob 的轮次残留 = 3, 期望 0
subscription_test.go:98: bob 的题目记录残留 = 3, 期望 0（外键级联）
--- FAIL: TestUnsubscribeIsolation
subscription_test.go:156: 项目一的记录应被清除, 残留 2
--- FAIL: TestUnsubscribeOnlyTargetProject
```

这正是本阶段要防的失败模式：**用户以为退订干净了，但数据仍在库里**，
且统计仍会把他们算进去。已还原（`grep MUTATION` 无残留）。

---

## 5. 端点的关键行为

### 分享（P6.1）

| 行为 | 测试 |
|---|---|
| 仅作者可生成/重置/撤销，订阅者与无关用户 403，未登录 401 | `TestShareRequiresOwnership` |
| 两次生成得到不同 token（第二次即重置） | `TestShareGeneratesDistinctTokensAndLinkFormat` |
| 链接格式为 `cala://subscribe?h=...&t=...` | 同上 |
| **重置后旧链接立即失效**，新链接可用 | `TestResetInvalidatesOldLink` |
| 撤销后链接失效 | `TestUnshareThenLinkInvalid` |

链接由**服务端**拼装（host 取自请求的 `Host` 头），不由客户端拼——
链接格式是服务端契约，两个实现会漂移。

### 导入（P6.2）

`TestImportMixedLinks` 是「逐条返回」的核心断言：5 条链接中混入有效、
令牌无效、格式错误、跨实例、空字符串，只有有效那条成功，其余各自给出可读原因。

```
第 2 条失败原因: 链接属于其他服务器（链接指向 127.0.0.1:9999，当前为 example.com）
第 3 条失败原因: 分享链接格式不正确: 应以 cala://subscribe? 开头，实际为 "https://example.com/whatever"
第 4 条失败原因: 链接属于其他服务器（链接指向 other.host，当前为 example.com）
第 5 条失败原因: 分享链接格式不正确: 链接为空
```

**为什么逐条返回**：用户粘贴 5 个链接时，其中一个失效不应让其余 4 个也失败。
整批回滚会让用户不知道哪个是坏的，只能反复试错。

其他：`TestImportIsIdempotent`（重复导入标记 `alreadySubscribed` 且不产生重复行）、
`TestImportOwnProject`（订阅自己的项目提示而成功数为 0）、
`TestImportValidation`（空数组 400、未登录 401、超过 50 条 400）。

### 订阅是纯只读跟随（D4）

`TestSubscriberHasNoWriteAccess` PASS：订阅者**不能**修改、删除、分享该项目
（均 403），但**可以**练习（200）。

`TestSubscriberSeesAuthorUpdates` PASS：作者改标题与题数后，订阅者立即看到新值
—— 因为订阅者读的就是 `project` 同一行（规格 §6.1(4)），没有快照复制。

---

## 6. 设计要点：为什么退订必须是一个原子操作

`store/subscription.go` 的顶部注释与实现共同保证：**刻意不提供**可单独调用的
「删除订阅行」函数。文件里只有：

```go
func (s *Store) Subscribe(userID, projectID int64) error
func (s *Store) IsSubscribed(userID, projectID int64) (bool, error)
func (s *Store) CountSubscriptions(projectID int64) (int, error)
func (s *Store) Unsubscribe(userID, projectID int64) (deletedRounds int, err error)  // 唯一入口
```

若退订被拆成两个原语，就会出现「删了订阅却忘了清历史」的用法，
而那正是本项目一直在消除的「同一规则两处实现」。现在这种错误用法
在**结构上不可能发生**。

`attempt` 的删除由 `practice_round` 的外键级联完成，应用层**不再写一遍**
——否则级联规则就有了第二个执行者，两者迟早不一致。

---

## 7. 前端

- `import_subscriptions_page.dart`：多行输入（一次粘贴多条），
  逐条结果展示（已订阅 / 已在列表中 / 失败 + 可读原因 + 链接原文）
- `share_page.dart`：展示链接、复制、生成、重置（二次确认说明旧链接失效）、撤销
- `profile_tab.dart`：退订入口接上真实功能，确认对话框明示
  「会删除你在该项目下的全部答题记录与统计，且无法恢复」
- `practice_tab.dart`：导入入口接上真实页面
- P4 中的「尚未提供」提示已全部移除

**不使用 Material 组件**（D10）：`SelectableText` / `SelectionArea` 都是 Material，
分享页改用普通 `Text` 配合「复制链接」按钮。

---

## 8. 未验证项

| # | 项 | 原因 | 何时可验 |
|---|---|---|---|
| 1 | 真机/浏览器上的人工分享与粘贴流程 | 需要交互会话；已用端到端 HTTP 验证同一契约 | 用户可 `flutter run -d chrome` |
| 2 | 链接在真实聊天软件中复制粘贴的表现 | 需要真实 IM 环境；解析器已测「混杂文本中提取链接」 | 用户实际使用时 |
| 3 | 跨实例订阅（真正连到另一台服务器） | 本阶段明确不支持，且已拒绝并给出可读错误 | 若日后需要 |
| 4 | CI 首跑结果 | 本环境不可达 github.com | 需用户在 Actions 页确认 |

---

## 9. 结论

规格 §16 的 **P6 出口条件已满足**：A5 与 A6 通过，
端到端走完了「分享 → 订阅 → 跟随 → 退订」完整闭环并查库确认了数据边界。

**新增规模**：store 2 文件（share.go 新增、subscription.go 扩充）、
service 2、api 2；前端 4 文件。零新增第三方依赖。

**下一步**：P7 CI 与交付（release.yml、ghcr 推送、**签名 APK**）——
这是最后一个阶段，且需要你在 GitHub 上确认 CI 首跑结果。
