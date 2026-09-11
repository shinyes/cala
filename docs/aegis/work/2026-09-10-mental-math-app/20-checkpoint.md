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

## Checkpoint Update

- Current todo: P7 CI 与交付(最后阶段)
- Active slice: P6 完成并提交, 准备进入 P7 最后一个阶段
- Completed todos:
- P6.1 分享 token(生成/重置/撤销, 链接由服务端拼装)
- P6.2 多链接导入(逐条返回结果, 跨实例校验)
- P6.3 原子退订(单一入口, 外键级联 attempt)
- P6.4 导入页/分享页/退订入口(移除 P4 的尚未提供提示)
- P6.5 验收(A5/A6 通过, 变异测试取证, 端到端闭环)
- Evidence refs:
- docs/aegis/work/2026-09-10-mental-math-app/evidence/p6/ACCEPTANCE.md
- Blocked on: P7 的签名 APK 需要 keystore 与 4 个 GitHub Secret(用户已创建); CI 首跑结果仍需用户在 Actions 页确认
- Next step: P7: release.yml(tag 触发 -> ghcr 推送 + 签名 APK + release 附件); 先校验 secrets 非空并明确报出缺哪个

## DriftCheckDraft

- Scope status: status: 完成规格§16 的 P6; 未越界到 P7
- Compatibility status: status: (project,user) 单一级联不变量保持(退订与删项目共用同一条外键规则); 订阅不授予写权限已验证; 未提供可单独调用的删订阅行原语
- Retirement status: status: 无退役对象; P4 的两处『尚未提供』提示已随真实功能移除; 明确拒绝短链接/二维码/订阅通知/跨实例订阅; 变异测试代码已还原
- New risk signals:
- 遗留: 真机人工分享粘贴未验证、跨实例订阅明确不支持、CI 首跑待用户确认、P7 签名 APK 依赖 keystore 与 secrets
- Advisory decision: continue

## Checkpoint Update

- Current todo: 全部阶段完成；等待用户在 GitHub Actions 页确认 CI/release 首跑
- Active slice: 全部阶段完成，进入项目收尾
- Completed todos:
- P7.1 Android 正式签名配置(用一次性 keystore 真实构建并验签)
- P7.2 release.yml(5 job, preflight 集中校验)
- P7.3 本地等效验证(Linux 交叉编译 + YAML 结构校验)
- P7.4 验收(A15 本地真实验证; A10 待 CI)
- 回填 5 份 ADR(规格 §12 承诺)
- Evidence refs:
- docs/aegis/work/2026-09-10-mental-math-app/evidence/p7/ACCEPTANCE.md
- Blocked on: A10 需 CI 真实执行才能确认(本环境不可达 github.com 且无 Docker); 真机/浏览器人工交互未验证
- Next step: 用户在 Actions 页确认 ci.yml 首跑; 打一个 v0.0.1 tag 观察 release.yml; 确认 Release 附件含 .tar.gz 与 .apk

## DriftCheckDraft

- Scope status: status: 规格 §16 的 P0-P7 全部交付，无范围外扩张；最终 10 个 Go 包 + 前端 4 层
- Compatibility status: status: 八条架构不可协商项全部由测试守护(沙箱四配置/保存期校验/单一级联/统计派生/无浮点判分/清洗表单一owner/判分权威); 签名材料未入库
- Retirement status: status: 退役项 = 模板默认的『release 用 debug 签名』已替换; kotlin.incremental 的退役触发条件已写明(pub cache 同盘后恢复); 变异测试代码已全部还原
- New risk signals:
- A10 未验证(需 CI): Docker 镜像构建/ghcr 推送/release 创建/镜像 tag 存在性; 真机与浏览器人工交互未验证; 用户真实 keystore 与 secret 的一致性由 preflight 的 keytool 试开负责报错
- Advisory decision: pause-for-user

## Checkpoint Update

- Current todo: 项目全部完成；v0.0.1 已发布
- Active slice: 项目收尾（唯一剩余: 真机覆盖安装验证）
- Completed todos:
- P7 验收: A10 与 A15 均已验证(CI 全流程通过 + 下载已发布 APK 独立验签)
- v0.0.1 发布成功: ghcr 镜像 + tar.gz 附件 + 已签名 APK 附件
- Evidence refs:
- https://github.com/shinyes/cala/releases/tag/v0.0.1
- Blocked on: 无
- Next step: 交付总结；如需继续可做真机验证或功能增强

## DriftCheckDraft

- Scope status: status: 规格 §16 的 P0-P7 全部交付并验证; 无范围外扩张
- Compatibility status: status: 八条架构不可协商项全部由测试守护; 签名材料未入库; ci.yml 的语料门禁未削弱
- Retirement status: status: 退役项 = 模板默认 debug 签名(已替换为正式签名)。variants/变异测试代码已全部还原
- New risk signals:
- 唯一剩余未验证项: APK 能否在真机覆盖安装升级旧版本(需真机与旧版本)。其余外部依赖已全部核实
- Advisory decision: continue

## Checkpoint Update

- Current todo: v0.0.2 已发布; 部署配置完成
- Active slice: 交付总结
- Completed todos:
- docker-compose 部署配置(部署/自建两份) + README 部署章节
- 修复镜像 /data 缺失与 CORS 空值语义两个缺陷, 并加 smoke 守护
- Evidence refs:
- §6.4
- Blocked on: 无
- Next step: 如需可做真机覆盖安装验证

## Checkpoint Update

- Current todo: v0.0.3 待发布
- Active slice: 发布 v0.0.3
- Completed todos:
- 手机端可配置服务端地址 + 修复 release APK 无 INTERNET 权限与明文限制
- ADR-0006 记录该决策; 基线 §5.1/§7 同步
- docker-compose.yml 升到 0.0.3
- Evidence refs:
- §6.5; ADR-0006
- Blocked on: 无
- Next step: 打 v0.0.3 tag; 真机安装验证联网与服务器地址配置

## Checkpoint Update

- Current todo: 全部完成; v0.0.3 已发布
- Active slice: 交付
- Completed todos:
- v0.0.3 发布成功(run #34590282901, 6 job 全 success)
- 已发布 APK 复核: INTERNET 权限与明文配置随签名产物生效; 签名身份跨版本稳定
- Evidence refs:
- §6.5
- Blocked on: 无
- Next step: 真机安装并验证联网 + 配置服务器地址 + 覆盖安装

## DriftCheckDraft

- Scope status: status: 键盘字母表为用户明确要求的收窄; 答案域收窄为 ADR-0007。无范围外扩张
- Compatibility status: status: scoring.Classify 保持接受文本(历史判分不受影响); 清洗表与判分仍接受 /; 分数答案仍合法。拒绝仅发生在 service 层产品校验
- Retirement status: status: 退役项 = 键盘的 / 键与文本答案这一作者能力。均有记录与理由; 无遗留回退路径
- New risk signals:
- 无限循环小数形式的答案须配容差, 已写入规格与 keypad.dart 提示; 作者仍可能忽略
- Advisory decision: continue

## Checkpoint Update

- Current todo: 收窄已完成; v0.0.4 待发布
- Active slice: 发布 v0.0.4
- Completed todos:
- 键盘字母表收敛为 0-9 . -
- 答案域收窄: 拒绝文本答案与分数形式(ADR-0007 含修订)
- 修掉收窄引入的死提示(scoring 建议改用文本)
- 规格/基线/证据/ADR 同步
- Evidence refs:
- §6.6; ADR-0007
- Blocked on: 无
- Next step: 打 v0.0.4 tag
