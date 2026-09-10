# 速算练习 App — 设计规格

- Date: `2026-09-10`
- Status: `draft-for-user-review`
- Work record: `docs/aegis/work/2026-09-10-mental-math-app/`
- 需求来源: 用户在会话中给出的十项功能描述（直接人类请求）

---

## 1. 背景与目标

为个人/小群体自部署场景构建一个速算练习应用。前后端分离：

- 后端 `Go + Fiber + SQLite`，负责账号、项目、出题规则执行、答题落库、统计与订阅。
- 前端 `Flutter`（目标平台 Android），开发阶段在浏览器（Chrome）中调试。
- UI 采用 **Cupertino 组件**，呈现 iOS 风格的简洁、现代、直观观感。

**成功证据**：本地 `go test ./...` 与 `flutter test` 通过；浏览器中可完整走通
`注册 → 建项目 → 练习 → 总结 → 错题重练 → 统计 → 分享订阅`；打 tag 后 CI 产出
Docker 镜像并推送 ghcr，且 GitHub Release 附有镜像 tar.gz。

---

## 2. 已确认决策（Decision Record）

以下决策由用户在本会话中逐项确认，是本规格的权威输入。

| # | 决策点 | 结论 | 理由/影响 |
|---|---|---|---|
| D1 | 规则函数契约 | **极简 `function generate(cfg)`** 返回 `{q, a}` | 最贴合「只需编写一个函数」的原始意图 |
| D2 | 一轮练习数据流 | **整轮预生成含答案 → 客户端本地即时判分** | 零网络往返，满足功能9「打错立即反馈」 |
| D3 | 判分方式 | **数值归一化比较 + 项目级可选容差**；非数值题退化为字符串精确匹配 | 避免「0.5 vs 1/2」「尾随空格」误判为错题 |
| D4 | 订阅关系边界 | **纯只读跟随；可随时退订，退订即清空本人历史** | 破坏性操作，UI 需二次确认并明示将删除的轮次数 |
| D5 | 统计存储 | **单事实源 + 查询时实时聚合，不建物化聚合表** | 删项目→级联删事实记录，聚合自然消失，物理上不可能不一致 |
| D6 | 「未完成练习」 | **不持久化未完成记录**（最小实现） | round 仅在整轮完成时写入；功能8 原文的「未完成练习」无对应实体 |
| D7 | 管理员权限 | **仅注册开关** | 严格按功能1 原文，不做用户管理后台 |
| D8 | 镜像分发 | **push ghcr.io + `docker save` 的 `.tar.gz` 作为 release 附件** | ghcr 用内置 `GITHUB_TOKEN`，无需额外密钥 |
| D9 | 前端交付 | **仅 APK；后端不内嵌前端产物** | 后端职责单一、镜像可极小；代价是开发期需显式配置 CORS |
| D10 | UI 组件库 | **直接使用 Cupertino 组件** | 用户明确要求 iOS 风格，此为字面实现 |
| D11 | 契约字段名 | `{q, a}` | 省字；保存期校验错误信息会明确提示字段名 |
| D12 | 每轮题数归属 | **由项目设置 `question_count` 决定，规则无权决定** | 与 D1 极简契约一致，避免配置分散两处 |
| D13 | 正确性来源 | **服务端用同一套归一化规则重算 `is_correct`** | 不盲信客户端上报，零成本换取一致性 |
| D14 | 项目配置编辑 | **JSON 文本框 + 实时校验** | 实现成本低；表单式编辑器列为后续增强 |

---

## 3. 需求冲突消解（显式记录，非遗漏）

**冲突**：功能6 声明「关闭 app 不保存」，即不应存在服务端未完成练习；功能8 却要求
删项目时清理「未完成练习」，意味着它存在。

**消解**：用户选择 D6（不持久化未完成记录）。因此：

- 服务端不存在 `in_progress` 状态的 round。
- 功能8 的级联删除范围为 `round` + `attempt` + `subscription`，不含未完成练习。
- 「关闭 app 不保存」与「重进不可续做」是**产品特性**，不是缺陷：本 App 不提供续做。

---

## 4. 架构

```
┌─────────────────────────────┐         ┌──────────────────────────────────┐
│  Flutter App (Android 为主)  │  HTTP   │  Go + Fiber                       │
│  Cupertino UI               │ ──────▶ │  ├─ internal/api     handler       │
│  内存态练习运行时             │  JSON   │  ├─ internal/service 业务逻辑       │
│  (暂停/继续、自带键盘)        │         │  ├─ internal/store   SQLite        │
└─────────────────────────────┘         │  ├─ internal/rules   goja 沙箱      │
                                         │  └─ internal/auth    会话与口令      │
       开发期: flutter run -d chrome      └──────────────────────────────────┘
       → 后端需放行 localhost 开发源 (CORS)
```

### 4.1 仓库布局

```
Cala/
├─ .github/workflows/{ci.yml, release.yml}
├─ backend/
│  ├─ cmd/server/main.go
│  ├─ internal/{api,service,store,rules,auth}/
│  ├─ migrations/*.sql          # 内嵌（embed）启动时按序应用
│  ├─ go.mod
│  └─ Dockerfile                # 多阶段 → distroless/static
├─ app/                         # Flutter 工程
├─ docker-compose.yml           # 备选自部署路径
└─ docs/aegis/
```

### 4.2 关键技术选型

| 项 | 选择 | 说明 |
|---|---|---|
| SQLite 驱动 | `modernc.org/sqlite`（纯 Go） | ⚠️ **待验证假设 A1**：需本地确认该驱动可用且 `CGO_ENABLED=0` 可静态编译 |
| JS 引擎 | `github.com/dop251/goja` | 纯 Go，已由实测验证（见 §5） |
| 口令哈希 | `golang.org/x/crypto/bcrypt` | 成熟、无额外依赖 |
| 会话 | 不透明随机 token（32B，base64url），**哈希后入库** | 可吊销，无需 JWT 密钥管理 |
| 前端状态管理 | Riverpod | 可测试、不依赖 `BuildContext` |
| HTTP 客户端 | `dio` | 拦截器统一注入 token 与错误映射 |
| Token 本地存储 | `shared_preferences` | Web 与 Android 均可用，开发期无需额外平台配置 |

### 4.3 前端信息架构（功能3）

底栏固定三 Tab，用 `CupertinoTabScaffold`。**每个 Tab 的职责必须互不重叠**，
否则会出现「同一件事两个入口」的可读性问题：

| Tab | 职责 | 内容 |
|---|---|---|
| **练习** | 做练习 + 管理项目 | 项目列表（拥有 / 已订阅，分组显示）；右上 `+` → 新建项目 / **导入订阅链接**；点击项目直接进入练习 |
| **统计** | 只看数字 | 项目选择器 + 粒度/指标分段控件 + 折线图 + 指标卡（§8.2）。**不含任何编辑入口** |
| **我的** | 账号与关系 | 用户信息；我创建的项目（管理/分享/删除）；我订阅的项目（**退订**入口）；管理员可见的注册开关；退出登录 |

设计取舍（避免 UI 复杂可读性差）：

- **订阅导入**入口放在「练习」Tab 的 `+` 菜单，因为它是「获得一个可练习的项目」；
  「我的」Tab 只做**已订阅项目的管理（退订）**，不做导入。二者职责不重叠。
- **项目配置编辑**只在项目详情页，且仅对作者可见。订阅者进入同一页面时该入口隐藏
  （D4 纯只读跟随），提示「该项目由 @作者 维护」。
- 统计 Tab 无编辑能力，保证「看数」与「改配置」在空间上分离。

### 4.4 分享链接格式（功能5）

由于 D9 确定后端**不提供网页前端**，形如 `https://host/s/<token>` 的链接在浏览器中
会 404，无法作为「点开即订阅」的落地页。因此采用**应用内可粘贴的链接字符串**：

```
cala://subscribe?h=<host>&t=<share_token>
```

- 由 App 生成并通过系统分享面板发出（功能5 的「发送链接」）。
- 接收方在「练习」Tab → `+` → 导入订阅 中粘贴，**支持一次粘贴多行/多个链接**（功能5）。
- `h` 携带服务端地址，使得跨实例分享可行；导入时若与当前登录实例不同则拒绝并提示。
- 分享链接为纯文本，便于通过任意 IM 传递，不依赖服务端提供落地页。

---

## 5. 规则引擎（功能2）

### 5.1 作者可见契约

```js
// 唯一需要编写的函数。cfg 为项目配置对象（作者自定义结构，见 D14）
function generate(cfg) {
  const a = cfg.min + Math.floor(Math.random() * (cfg.max - cfg.min + 1));
  const b = cfg.min + Math.floor(Math.random() * (cfg.max - cfg.min + 1));
  return { q: a + " + " + b + " = ?", a: String(a + b) };
}
```

平台承担：每轮题数（D12）、随机种子、确定性、判分（D3）、超时与递归保护。

### 5.2 沙箱配置 —— 全部为强制项

以下配置依据**本地实测证据**（见 `evidence/goja-spike/`），非推测：

| 配置 | 值 | 实测依据 |
|---|---|---|
| `SetRandSource` | 每轮种子 | 同种子两次生成题目序列**逐字相同**；异种子不同 |
| `SetTimeSource` | 冻结时刻 | 防止 `new Date()` 破坏可复现性 |
| `SetMaxCallStackSize` | 200 | ⚠️ **不设置则无限递归挂死整个进程**（实测跑满 300s 未返回） |
| `Interrupt` 超时 | 单题 50ms | 实测（计时器设为 60ms）`while(true)` 于 61ms 被击杀；`ClearInterrupt` 后 VM 可复用。生产预算取 50ms |

### 5.3 保存期校验（强制，不可跳过）

功能5 规定作者修改规则会**同步影响所有订阅者**，因此坏规则会连坐全员。项目配置
保存前必须执行：

1. 编译源码（语法错误即拒绝）
2. 断言 `generate` 存在且可调用
3. 以当前 `cfg` 连续调用 20 次，每次施加 50ms 超时
4. 校验返回值为对象且含字符串 `q` 与 `a`
5. 任一步失败 → 拒绝保存并回传具体错误

实测可拦截的 7 类错误源码：缺 `generate`、语法错误、未终止注释、顶层 throw、
调用期未定义引用、返回数字、返回 null。**全部被成功拒绝。**

### 5.4 引擎实现注意（实测踩坑，已记录避免重犯）

- `goja.AssertFunction` 在当前版本返回 `(Callable, bool)`，**不是** `(Callable, error)`。
- `RunProgram` 返回**脚本完成值**（函数声明语句 → `undefined`），必须用
  `vm.Get("generate")` 取函数。
- **不需要 VM 池**：冷启动建 VM 实测 23.7µs/题（一轮 50 题约 1.2ms），复用 VM 8.8µs/题。
  差异不足以支撑一个池化子系统的复杂度（Existence Check 结论：`reject`）。

---

## 6. 数据模型

```sql
user(id, username, password_hash, is_admin, disabled, created_at)

session(id, token_hash, user_id, expires_at, created_at)

setting(key, value)                    -- registration_open

project(id, owner_id, title, description, question_count,
        cfg_json, rule_source, share_token, share_token_updated_at,
        created_at, updated_at)

subscription(user_id, project_id, created_at)
        -- UNIQUE(user_id, project_id)

round(id, project_id, user_id, seed, started_at, finished_at,
      total_ms, question_count, correct_count)

attempt(id, round_id, idx, q_snapshot, a_snapshot,
        user_input, is_correct, elapsed_ms)
        -- UNIQUE(round_id, idx)
```

### 6.1 核心不变量

1. **访问权** = `project.owner_id == me` **或** 存在 `subscription(user_id=me, project_id)`。
2. **唯一级联规则**：`(project, user)` 关系终止 ⇒ 该用户在该项目下的 `round`/`attempt` 消亡。
   - 作者删除项目 ⇒ 所有用户关系终止 ⇒ 删除 `round`/`attempt`/`subscription`（功能8）。
   - 订阅者退订 ⇒ 仅自己关系终止 ⇒ 删除自己的 `round`/`attempt` 与 `subscription` 行（D4）。
   - 两者由同一规则覆盖，无需两套逻辑。
3. **题面/答案以快照落库**（`q_snapshot`/`a_snapshot`），不靠种子重放。这保证：
   - 功能7「重练错题」直接读快照即可，无需重新出题；
   - 功能5 作者事后修改规则**不会篡改历史错题**。
4. **订阅无需传播**：订阅者读的就是 `project` 同一行，作者修改即时可见（功能5 天然满足）。

### 6.2 引导管理员

`registration_open = false` 时，**首个用户仍允许注册**并置 `is_admin = true`，否则全新部
署将自我锁死、无法开启注册。

---

## 7. API 契约

```
POST   /api/auth/register            # 受开关限制；首个用户例外并成为管理员
POST   /api/auth/login               → { token, user }
POST   /api/auth/logout
GET    /api/me

GET    /api/settings/public          → { registration_open }
PUT    /api/admin/settings           # 仅管理员

GET    /api/projects                 # 拥有 + 已订阅
POST   /api/projects                 # 创建（含规则校验 §5.3）
GET    /api/projects/:id
PUT    /api/projects/:id             # 仅作者；改后订阅者即时可见（含规则校验）
DELETE /api/projects/:id             # 仅作者；级联删除全员记录（功能8）
POST   /api/projects/:id/share       → { share_token }   # 生成/重置
DELETE /api/projects/:id/share       # 撤销分享

POST   /api/subscriptions/import     { links: [ "...", ... ] }   # 多链接导入（功能5）
DELETE /api/subscriptions/:projectId # 退订；清空本人历史（D4）

POST   /api/rounds/start             { projectId } → { seed, questions: [{q, a}] }
POST   /api/rounds/complete          { projectId, seed, startedAt, finishedAt,
                                       attempts: [{ q, a, input, elapsedMs }] }

GET    /api/projects/:id/stats?grain=day|week|month
```

**`/rounds/complete` 的服务端行为**（D13）：以归一化规则重算 `is_correct`，忽略客户端
上报的 `correct` 字段；`total_ms` 与 `correct_count` 由服务端从 attempts 派生，不采信
客户端汇总值。

---

## 8. 统计设计（功能4）

### 8.1 计算方式

按 D5，不做物化聚合。因为 SQLite 无内置中位数，且个人练习数据量小：
**一次查询取回区间内全部 `round` 行，在 Go 内分桶并计算全部 8 个指标。**
这比分桶 SQL + 中位数近似更简单，且不可能算错。

时间粒度：
- `day` —— 按自然日分桶
- `week` —— 按星期几（周一…周日）跨日期聚合
- `month` —— 按自然月分桶

每桶指标（8 项）＝ 对桶内每一轮先取「轮耗时」与「轮正确率」，再聚合：
`最高/最低/平均/中位` × `耗时/正确率`。

### 8.2 UI 分组（避免可读性下降）

8 个指标 + 3 种粒度 = 24 个数字，必须拆维度呈现，任一时刻屏幕上只有「一条线 + 8 个数字」：

```
┌ 项目选择器 ───────────────────────────┐
│  粒度 [日] [周] [月]                     │
│  指标 [耗时] [正确率]                    │  ← 两个分段控件，单一激活值
├───────────────────────────────────────┤
│  折线图（单条线，随上方切换）              │  ← 绝不画双 Y 轴
├───────────────────────────────────────┤
│  耗时指标          │  正确率指标          │
│  最高 / 最低       │  最高 / 最低         │  ← 两张卡，各 4 行
│  平均 / 中位       │  平均 / 中位         │
└───────────────────────────────────────┘
```

---

## 9. 练习运行时（功能6 / 7 / 9）

### 9.1 状态机

```
进入项目 → POST /rounds/start → 整轮题目抵达
   ↓
[答题中] ──暂停──▶ [已暂停] ──继续──▶ [答题中]      # 全程内存态
   │                    │
   │                    └── 关闭 app ⇒ 一切丢弃（功能6，D6）
   ↓ 最后一题完成
POST /rounds/complete → 总结页
   ↓ 点击错题数
错题页 ──「重练错题」──▶ 以快照重开一轮（不再调用规则）
      ──「再来一轮」──▶ 同项目、新种子
```

### 9.2 交互细节（功能9）

- **答对**：短暂高亮反馈后自动进入下一题（不打断节奏）。
- **答错**：立即显示正确答案 + 出现「下一题」按钮（原文要求）。
- 暂停：停表并遮蔽题面，避免暂停期间偷看题目。

### 9.3 自带键盘（功能6）

- 数字键盘：`1-9` `0` `.` `-` `⌫` + 提交键。
- 负号与小数点按需可用（速算含负数/小数题）。
- 开发期同时接受物理键盘输入，便于浏览器调试。

---

## 10. 交付与 CI（功能10）

### 10.1 工作流

| 文件 | 触发 | 行为 |
|---|---|---|
| `ci.yml` | push / PR | `go test ./...`、`flutter analyze`、`flutter test` |
| `release.yml` | tag `v*` | 构建静态 Go 二进制 → 多阶段 Docker 构建 → push ghcr（`latest`/版本号/`sha`）→ `docker save \| gzip` → 创建 GitHub Release 并上传 `.tar.gz`（APK 见 §10.2） |

### 10.2 超出原文的小增补（请评审时确认）

原文功能10 只要求构建 Docker 镜像。但由于 D9 确定前端**仅以 APK 交付**，
若不构建 APK，则没有任何可分发的 App 产物。因此建议 `release.yml` **额外构建 APK
并作为 release 附件**。

**APK 签名不在范围内**：CI 产出未签名（或 debug 签名）APK 供侧载；正式签名由使用者
本地完成。

### 10.3 本地测试

- 主路径（本机未安装 Docker）：`cd backend && go run ./cmd/server` + `cd app && flutter run -d chrome`。
- 备选：`docker-compose.yml`。
- 开发期 CORS：后端放行 `localhost` 来源的开发地址（D9 的代价）。

---

## 11. 非目标

- iOS 发布
- 离线优先 / 断网可用
- 题目云端市场、审核、评分
- 邮件找回密码
- 管理员后台（用户管理、项目审查、系统统计）—— D7
- 未完成练习的续做/恢复 —— D6
- APK 正式签名
- 规则 VM 池化 —— Existence Check 判定 `reject`
- 物化统计聚合表 —— D5
- VM 内存上限（goja 无内置支持，以超时 + 递归上限代偿，见 §14 R3）

---

## 12. ADR 信号（实施后回填，现在不创建已接受决策）

| # | 主题 | 为何是 durable 决策 |
|---|---|---|
| ADR-1 | 规则引擎契约与沙箱边界 | 契约一旦发布即难变更；影响所有已分享项目 |
| ADR-2 | `(project, user)` 单一级联不变量 | 跨用户不可逆删除语义，是数据所有权的权威定义 |
| ADR-3 | 统计为派生态而非物化态 | 决定事实源边界与一致性策略，影响后续所有统计功能 |

> 依 Aegis 规则，**不为未执行的设想创建已接受的架构记忆**。以上在对应实现完成后回填。

---

## 13. 验收标准

| # | 标准 | 验证方式 |
|---|---|---|
| A1 | 首个注册用户自动成为管理员；开关关闭后新注册被拒且首个用户仍可注册 | Go 测试 |
| A2 | 保存规则时 7 类错误源码全部被拒并返回可读错误 | Go 测试（表驱动） |
| A3 | 同种子两次生成题目序列逐字相同 | Go 测试 |
| A4 | 死循环规则在超时内被击杀且不影响后续请求 | Go 测试 |
| A5 | 作者删除项目后，订阅者与作者在该项目下的 round/attempt 全部消失 | Go 测试 |
| A6 | 订阅者退订后，仅其本人历史消失，作者与他人记录不受影响 | Go 测试 |
| A7 | 作者修改规则后，历史错题的 `q_snapshot`/`a_snapshot` 不变 | Go 测试 |
| A8 | 统计 8 个指标（含中位数，奇偶长度）计算结果正确 | Go 测试（含边界用例） |
| A9 | 浏览器中完整走通 注册→建项目→练习→总结→错题重练→统计→分享订阅 | 手动端到端 |
| A10 | 打 tag 后镜像推送 ghcr 且 release 附 `.tar.gz` | CI 运行记录 |

---

## 14. 兼容边界与风险

| # | 风险 | 处置 |
|---|---|---|
| R1 | 规则契约发布后变更成本高（影响所有已分享项目） | D1/D11 已刻意取最小；ADR-1 记录边界 |
| R2 | 作者保存坏规则会连坐所有订阅者 | §5.3 保存期校验为强制项，不可跳过 |
| R3 | goja 无内存上限，恶意规则可耗尽内存 | 以 50ms 超时 + `SetMaxCallStackSize(200)` 代偿；单进程部署下风险有限，列为已知残留风险 |
| R4 | 退订即清空历史不可逆 | UI 二次确认并**明示将删除的轮次数** |
| R5 | 客户端持有答案（D2 的固有代价） | 无作弊动机场景；服务端重算 `is_correct`（D13）保证落库一致性 |
| R6 | 开发期 CORS 配置不当 | 仅放行显式配置的开发源，不使用通配符 + 凭据 |

### 待验证假设

| # | 假设 | 验证时机 |
|---|---|---|
| A1 | `modernc.org/sqlite` 可用且 `CGO_ENABLED=0` 静态编译通过 | 实施第一步 |
| A2 | 本机未安装 Docker，故镜像构建只能在 CI 验证 | 已验证（`docker` 不存在） |
| A3 | Cupertino 组件在 Android 上的观感符合预期 | 前端首个可运行里程碑 |

---

## 15. 证据引用

- `docs/aegis/work/2026-09-10-mental-math-app/evidence/goja-spike/` —— goja 沙箱可行性
  实测程序与输出。覆盖：ES 语法覆盖（28/30）、沙箱逃逸面、死循环与递归containment、
  确定性验证、返回契约、性能、校验拦截。
- 决策 D1–D14 来源：本会话用户逐项确认。

---

## 16. 分阶段交付（实施范围分解）

本规格覆盖十项功能、七个子系统。**单份实施计划无法安全承载**，故按下列阶段分解。
每阶段必须独立可验证（有对应验收标准与可执行命令）后才进入下一阶段。

| 阶段 | 范围 | 功能映射 | 出口条件 |
|---|---|---|---|
| **P0 骨架与风险先行** | 仓库布局、`go.mod`、**验证 A1（`modernc.org/sqlite` 纯 Go + `CGO_ENABLED=0`）**、完整 schema 迁移（§6 全部表）、配置、健康检查、Flutter 工程初始化、多阶段 Dockerfile | — | `go build` 通过；容器可启动；A1 得结论 |
| **P1 认证与引导** | `user`/`session`/`setting`、bcrypt、register/login/logout/me、首个用户即管理员、注册开关 | 1 | A1 通过 |
| **P2 规则引擎** | `internal/rules`、契约、沙箱四配置、保存期校验、表驱动单测 | 2（后端半） | A2/A3/A4 通过 |
| **P3 项目与练习闭环（后端）** | 项目 CRUD、`/rounds/start`、`/rounds/complete`、判分归一化、服务端重算 `is_correct` | 2、9（后端） | 报告与落库一致 |
| **P4 前端骨架与练习运行时** | Cupertino 三 Tab（§4.3）、项目列表、练习运行时（自带键盘/暂停/即时反馈）、总结页、错题页与重练 | 3、6、7、9 | A9 前半段可走通 |
| **P5 统计** | `/stats` 端点、日/周/月分桶、8 指标计算、统计页 UI（§8.2） | 4 | A8 通过；A9 统计段可走通 |
| **P6 分享订阅与级联删除** | `share_token`、`/subscriptions/import`（多链接）、退订清历史、项目删除级联 | 5、8 | A5/A6/A7 通过 |
| **P7 CI 与交付** | `ci.yml`、`release.yml`、ghcr 推送、release 附件 | 10 | A10 通过 |

**顺序约束**：P1 → P3 依赖 P1；P4 依赖 P1+P3；P5/P6 依赖 P3；P7 依赖可构建产物。

**P0 一次性建全 schema 的理由**：§6 的数据模型在设计阶段已完整确定，分阶段加表只会
产生无谓的迁移噪声。一次性落全表，后续阶段只加查询与写入逻辑。

**明确不做的事**：不为阶段划分创建并行的 `docs/aegis/specs/` 文档（本规格是该表面的
唯一 owner）；阶段进度记录写入 `docs/aegis/work/2026-09-10-mental-math-app/20-checkpoint.md`。

---

## 17. 需求可追溯性

| 功能 | 覆盖章节 |
|---|---|
| 1 多用户与管理员 | §6.2, §7, D7 |
| 2 JS 自定义出题规则 | §5, D1, D11, D12 |
| 3 底栏三 Tab | §4.3 |
| 4 统计页 | §8 |
| 5 分享订阅 | §4.4（链接格式）, §6.1(4), §7, D4 |
| 6 自带键盘/暂停/不保存 | §9.1, §9.3, D6 |
| 7 总结页与错题页 | §9.1, §6.1(3) |
| 8 删除项目级联 | §6.1(2), §3 |
| 9 打错立即反馈 | §9.2, D2 |
| 10 tag 触发 CI 出镜像 | §10 |

阶段分解见 §16。
