# 速算练习 App（Go+Fiber+SQLite 后端 / Flutter 安卓前端） - Checkpoint

- Task ID: 2026-09-10-mental-math-app
- Current todo: 写并提交设计规格
- Active slice: 设计规格落盘与用户评审
- Blocked on: none
- Next step: 评审通过后进入 writing-plans 制定实施计划

## Checkpoint Update

- Current todo: 等待用户评审设计规格
- Active slice: 设计规格的用户评审门
- Completed todos:
- 探索项目上下文(空仓库)
- goja 沙箱可行性实测(8 组实验)
- 与用户确认 D1-D14 共 14 项决策
- 写设计规格并自审
- 落初始双基线
- Evidence refs:
- docs/aegis/work/2026-09-10-mental-math-app/evidence/goja-spike/OUTPUT.txt
- Blocked on: none
- Next step: 用户批准后进入 writing-plans 制定分阶段实施计划(P0-P7)

## DriftCheckDraft

- Scope status: status: 停留在设计阶段，未超出十项功能范围
- Compatibility status: status: 尚未写码，兼容边界无损；规格 §14 已登记 6 项风险
- Retirement status: status: 无需退役；已显式声明拒绝 VM 池与物化聚合表两个诱人的新增面
- New risk signals:
- R3 goja 无内存上限，以超时+递归上限代偿，列为已知残留风险
- Advisory decision: pause-for-user

## Checkpoint Update

- Current todo: 等待用户确认规格修订 + 提供 GitHub 仓库地址与签名密钥
- Active slice: 规格修订完成, 等待用户确认
- Completed todos:
- 按用户要求以最佳方案重新设计判分一致性(D15/D16, 六重措施)
- APK 纳入交付并改为正式签名(D17/D18)
- 自审修正: A1 编号冲突改为 V1, 新增 P3.5 阶段, 基线同步
- Evidence refs:
- docs/aegis/specs/2026-09-10-mental-math-app-design.md
- Blocked on: GitHub 仓库地址未知(CI 与 remote 需要)
- Next step: 用户确认后进入 writing-plans, 输出 P0-P7 分阶段实施计划

## DriftCheckDraft

- Scope status: status: 仍在十项功能范围内; 新增 D15/D16/D17/D18 为风险面彻底化与用户新要求
- Compatibility status: status: 未写码; 兼容边界新增判分无浮点与清洗表单一owner 两条(基线§5.2)
- Retirement status: status: 明确拒绝客户端JS引擎方案(§5.5.8)与split-per-abi, 并说明理由
- New risk signals:
- R7 前后端判分策略分歧已由六重措施约束; R8 keystore遗失属外部依赖
- Advisory decision: pause-for-user

## Checkpoint Update

- Current todo: 制定 P0-P7 实施计划
- Active slice: writing-plans: 输出分阶段实施计划
- Completed todos:
- 设计规格获用户批准(含 D15-D20 修订)
- 仓库接入 github.com/shinyes/cala, 主分支 main, 已推送
- 确认 D6/D7 保持最小实现
- Evidence refs:
- git log --oneline (4 commits, main=origin/main=283706b)
- Blocked on: none
- Next step: 计划落盘后从 P0 开始: 验证 V1(sqlite 静态编译)并搭仓库骨架

## DriftCheckDraft

- Scope status: status: 仍在 P0/P1 范围内; 计划明确拒绝迁移框架与额外抽象
- Compatibility status: status: 承载基线§9 的第1/2条级联与DSN外键, 已设 falsifier 测试
- Retirement status: status: 本计划无退役对象; 全部为新增, 无 fallback/兼容分支
- New risk signals:
- P-R1: 实现者用 db.Exec(PRAGMA) 替代 DSN pragma 会让级联静默失效, 已由 TestForeignKeysEnabledOnPooledConnections 设为 falsifier
- Advisory decision: continue

## Checkpoint Update

- Current todo: P0.1 仓库骨架与 Go module
- Active slice: 执行 P0.1: 创建 backend/go.mod 与 .dockerignore, 拉取依赖
- Completed todos:
- 设计规格与 P0/P1 实施计划获批
- V1 与级联不变量实测通过
- Evidence refs:
- docs/aegis/plans/2026-09-10-p0-p1-foundation-auth.md
- Blocked on: none
- Next step: P0.2 配置加载

## Checkpoint Update

- Current todo: P2 规则引擎
- Active slice: P0/P1 全部完成, 准备进入 P2 规则引擎
- Completed todos:
- P0.1 仓库骨架与 Go module
- P0.2 配置加载(3 测试)
- P0.3 存储层/迁移/级联不变量(7 测试, 含变异测试取证)
- P0.4 Fiber 路由/统一错误契约/健康检查(端到端验证)
- P0.5 Flutter 工程初始化(analyze/test/apk/web 全通过)
- P0.6 多阶段 Dockerfile 与 compose(Linux 静态交叉编译验证)
- P1.1/P1.2 bcrypt 口令哈希与不透明会话令牌(8 测试)
- P1.3 用户/会话/设置数据访问(8 测试)
- P1.4 认证服务与引导管理员规则(9 测试)
- P1.5 认证 API 与鉴权中间件(11 测试)
- P1.6 验收: A1 通过 + 手工端到端 + 数据库安全取证
- Evidence refs:
- docs/aegis/work/2026-09-10-mental-math-app/evidence/p0-p1/ACCEPTANCE.md
- Blocked on: 无(keystore 与 GitHub Secrets 属 P7 前置, 当前不阻塞)
- Next step: P2: internal/rules 的 goja 沙箱、generate(cfg) 契约、保存期校验

## DriftCheckDraft

- Scope status: status: 完成规格§16 的 P0/P1, 未越界到 P2 及以后
- Compatibility status: status: 兼容边界 1/2 已由测试守护(含变异取证); 统一错误契约已确立并复用
- Retirement status: status: 无退役对象; 明确拒绝迁移框架与冗余抽象; 被证伪的 -Dfile.encoding 已移除而非保留
- New risk signals:
- P-R8 跨盘符(已根因修复+隔离验证); P-R9 PS5.1 写非法 UTF-8(已修复+全仓校验); 新增未验证项: Docker 镜像构建与签名 APK 留待 P7
- Advisory decision: continue

## Checkpoint Update

- Current todo: P3 项目与练习闭环
- Active slice: P2 完成, 准备进入 P3 项目与练习闭环
- Completed todos:
- P2.1 沙箱配置(集中一处, 含递归与超时)
- P2.2 规则编译(7 类错误源码被拒, 18 项语法支持)
- P2.3 保存期校验(18 类行为错误被拒, 恶意规则 30s 内全拒)
- P2.4 整轮出题与可复现性(A3 通过)
- P2.5 验收(A2/A3/A4 全通过, 含两项变异测试取证)
- Evidence refs:
- docs/aegis/work/2026-09-10-mental-math-app/evidence/p2/ACCEPTANCE.md
- Blocked on: none
- Next step: 写 P3 计划: 项目 CRUD、/rounds/start、/rounds/complete、答案信封分类、服务端重算

## DriftCheckDraft

- Scope status: status: 完成规格§16 的 P2, 未越界到 P3
- Compatibility status: status: 作者契约未扩张(D1/D11); 沙箱四配置集中一处无第二份拷贝; 无 fallback/兼容分支
- Retirement status: status: 无退役对象; 明确拒绝 VM 池与多函数契约; 变异测试用的临时代码已全部还原(grep MUTATION 无残留)
- New risk signals:
- P2 新增局限: goja 无内存上限仍未解决(仅以超时+递归+输出长度上限收窄); 模块级初始化按题重复执行; 校验只跑 20 次属概率性防护
- Advisory decision: continue

## Checkpoint Update

- Current todo: P3.5 判分单一性与语料门禁
- Active slice: P3 完成, 准备进入 P3.5 跨端判分一致性
- Completed todos:
- P3.1 判分核心 internal/scoring(114 测试, 含浮点变异取证)
- P3.2 容差迁移 0002 与项目数据访问
- P3.3 项目 CRUD 与规则校验(含 fail() 吞错误缺陷修复)
- P3.4 轮次出题与交卷(服务端重算+分歧告警)
- P3.5 验收(A11/A14 通过, 336 测试全绿, 手工端到端+查库取证)
- Evidence refs:
- docs/aegis/work/2026-09-10-mental-math-app/evidence/p3/ACCEPTANCE.md
- Blocked on: 无
- Next step: P3.5: 由 scoring 生成 >=10000 条语料 + Dart 镜像实现 + CI 一致性门禁

## DriftCheckDraft

- Scope status: status: 完成规格§16 的 P3, 未越界到 P4/P5/P6
- Compatibility status: status: 判分权威与分歧留痕已由解码取证; 清洗表单一下发(32 项+版本号); 快照不可变已验证; 作者契约未扩张
- Retirement status: status: 无退役对象; 变异测试代码已还原; 明确拒绝 ORM/有理数库/metrics 库; 刻意不提供可单独调用的退订原语
- New risk signals:
- P3 遗留: A14 的统计部分待 P5; 前端判分一致性待 P3.5; 订阅导入退订待 P6
- Advisory decision: continue

## Checkpoint Update

- Current todo: P4 前端骨架与练习运行时
- Active slice: P3.5 完成并推送, 准备进入 P4 前端
- Completed todos:
- P3.5 语料生成器(13872 条, 由 Go 侧计算)
- P3.5 Dart 镜像实现(零依赖, BigInt, 清洗表不由客户端定义)
- P3.5 生成端护栏(新鲜度/纯ASCII/规模下限)
- P3.5 CI 工作流 ci.yml(门禁执行点)
- P3.5 验收(A12/A13 通过, 三项变异测试取证)
- Evidence refs:
- docs/aegis/work/2026-09-10-mental-math-app/evidence/p3.5/ACCEPTANCE.md
- Blocked on: CI 无法本地验证(本环境不可达 github.com API 与页面); Actions 运行状态需用户在 GitHub 上确认
- Next step: P4: Cupertino 三 Tab、项目列表、练习运行时(自带键盘/暂停/即时反馈)、总结页与错题重练

## DriftCheckDraft

- Scope status: status: 完成规格§16 的 P3.5; 过程偏差(缺计划文档)已如实记录于 evidence §6
- Compatibility status: status: 清洗表单一下发(客户端只应用数据, 已由空表测试证明未内置映射); 判分无浮点由双侧守护; 单向对齐(Go 生成->Dart 对齐)
- Retirement status: status: 无退役对象; 三项变异测试代码已全部还原(grep MUTATION 无残留); 明确拒绝客户端 JS 引擎与人工同步两方案并记录理由
- New risk signals:
- CI 无法本地验证(本环境不可达 github.com); goja 无内存上限仍未解决; subosito/flutter-action 固定版本组合待 CI 首跑确认
- Advisory decision: continue

## Checkpoint Update

- Current todo: P5 统计
- Active slice: P4 完成并提交, 准备进入 P5 统计
- Completed todos:
- P4.1 API 客户端与数据模型(24 测试, 离线替身)
- P4.2 登录态与注册登录页(11 测试)
- P4.3 三 Tab 骨架(职责不重叠)
- P4.4 项目列表与项目编辑(规则错误原样展示)
- P4.5 练习运行时状态机(39 测试)
- P4.6 练习页/自带键盘/总结页
- P4.7 错题页与重练(快照不重新出题) + 后端 attempts 端点
- P4.8 我的 Tab/统计占位/验收
- 修复种子精度跨端 bug(2^53 上限 + 两项测试取证)
- Evidence refs:
- docs/aegis/work/2026-09-10-mental-math-app/evidence/p4/ACCEPTANCE.md
- Blocked on: CI 首跑结果需用户在 GitHub Actions 页确认; 真机交互与浏览器人工点击未验证(无交互会话)
- Next step: P5: /stats 端点、日/周/月分桶、8 项指标(含中位数)、统计页 UI

## DriftCheckDraft

- Scope status: status: 完成规格§16 的 P4; 未越界到 P5/P6
- Compatibility status: status: 判分只经 scoring.compare 且只用服务端清洗表(有全角数字测试守护); 错题以 serverIsCorrect 筛选; 重练用快照; 三 Tab 职责不重叠; 作者契约未扩张
- Retirement status: status: 无退役对象; 明确拒绝 Material 组件(自绘进度条)、代码生成、路由框架、图表库、本地库; 移除了一处我自写的多层 JSON 解码包装
- New risk signals:
- 新增: 种子精度已修复; 遗留: 真机交互与浏览器人工点击未验证、统计图表待 P5、分享退订待 P6、CI 首跑待用户确认
- Advisory decision: continue

## Checkpoint Update

- Current todo: P6 分享订阅与退订
- Active slice: P5 完成并提交, 准备进入 P6
- Completed todos:
- P5.1 三种粒度分桶(含时区与跨年, 11 测试)
- P5.2 8 项指标与中位数(奇偶分支, 含语义固定测试)
- P5.3 统计端点(tzOffsetMinutes, 20 测试含隐私边界)
- P5.4 统计页与自绘折线图(零新增依赖)
- P5.5 验收(A8 通过, 变异测试取证, 端到端数字手工核对)
- Evidence refs:
- docs/aegis/work/2026-09-10-mental-math-app/evidence/p5/ACCEPTANCE.md
- Blocked on: CI 首跑结果需用户在 GitHub Actions 页确认
- Next step: P6: 分享 token、多链接导入订阅、退订清历史(事务性整体操作)

## DriftCheckDraft

- Scope status: status: 完成规格§16 的 P5; 未越界到 P6
- Compatibility status: status: 无新增聚合表(统计为派生态); 未改动 round 写入语义; 统计 Tab 只读; 隐私按 (user,project) 过滤且无跨用户查询路径; 零新增依赖
- Retirement status: status: 无退役对象; 明确拒绝图表库/物化表/时间范围选择器/跨项目汇总; 变异测试代码已还原
- New risk signals:
- 新增记录: 统计允许 float64 而判分不允许, 该边界已写入包注释以免后续误判; 遗留: 真机浏览未验证、CI 首跑待确认、P6 分享退订待做
- Advisory decision: continue
