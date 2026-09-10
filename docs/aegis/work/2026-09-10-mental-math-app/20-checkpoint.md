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
