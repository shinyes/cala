# 速算练习 App（Go+Fiber+SQLite 后端 / Flutter 安卓前端） - Evidence

No evidence has been recorded yet.

## EvidenceBundleDraft

- Artifact key: goja-spike-output
- Type: log
- Source: go run . (goja v0.0.0-20260906210903, go1.25.5)
- Summary: 8 组沙箱实验: ES 覆盖 28/30; 无宿主全局逃逸; while(true) 61ms 被杀且 VM 可复用; 未设 SetMaxCallStackSize 时无限递归挂死进程; SetRandSource 同种子输出逐字相同; 冷启动 VM 23.7us/题; 7 类错误源码全部被拒
- Verifier: aegis-js-spike/main.go (可复现)

## EvidenceBundleDraft

- Artifact key: spec-approved
- Type: artifact
- Source: 用户明确确认: 进入实施计划 + D6/D7 保持最小
- Summary: 设计规格获批; 用户另提供仓库地址 shinyes/cala 与 applicationId cc.lcyk.cala; 4 commits 已推送 main
- Verifier: 用户会话确认 + git ls-remote

## EvidenceBundleDraft

- Artifact key: sqlite-cascade-spike
- Type: log
- Source: go run . (modernc.org/sqlite v1.58.0, CGO_ENABLED=0, go1.25.5)
- Summary: V1 成立: 纯 Go 驱动可用, CGO_ENABLED=0 交叉编译 linux/amd64 产出 6.08MB 静态二进制; 5 场景全部 PASS(删项目级联/退订仅清本人/删用户/事务回滚/8并发读); 关键发现: PRAGMA foreign_keys 必须写在 DSN 中, 否则连接池下级联静默失效
- Verifier: evidence/sqlite-cascade-spike/main.go (可复现)

## EvidenceBundleDraft

- Artifact key: p0-p1-acceptance
- Type: test-report
- Source: go build/vet/test + flutter analyze/test/build + 手工端到端 HTTP + DB 字节取证
- Summary: P0/P1 出口条件满足: 5 个 Go 包测试全绿; A1 通过(首个用户 becameAdmin=true, 关注册后 403); 级联不变量经变异测试取证确有捕获能力; DB 中无明文令牌与明文口令; Flutter analyze 无 issue 且 apk/web 均构建成功
- Verifier: evidence/p0-p1/ACCEPTANCE.md

## EvidenceBundleDraft

- Artifact key: p2-acceptance
- Type: test-report
- Source: go test ./internal/rules/ -count=1 -v  + 两项变异测试
- Summary: A2/A3/A4 全通过: 69 项测试 PASS/0 FAIL; 编译期拒绝 7 类错误源码, 运行期拒绝 18 类; 同种子 50 题逐字段相同; 6 条恶意规则 30s 内全被拒; 变异测试证明可复现性与超时中断均为承重(移除后分别失败与挂起)
- Verifier: evidence/p2/ACCEPTANCE.md

## EvidenceBundleDraft

- Artifact key: p3-acceptance
- Type: test-report
- Source: go test ./... -count=1 (336 项) + 手工端到端 + 查库 + 浮点变异测试
- Summary: A11/A14 通过。336 测试 0 失败。查库确认 client_is_correct 6 行全 true(原样保存未覆盖)而 server_is_correct 仅 3 行 true(权威重算)，分歧计数 3。变异测试证明无浮点判分被真实守护。修复 3 处设计缺陷(ASCII 文本矛盾/容差无字段)与 3 处实现或测试缺陷(fail 吞错误/硬编码迁移数/逗号分组预期)
- Verifier: evidence/p3/ACCEPTANCE.md

## EvidenceBundleDraft

- Artifact key: p3.5-acceptance
- Type: test-report
- Source: flutter test (13872 条语料比对) + go test ./cmd/scorpus/ + 三项变异测试
- Summary: A12/A13 通过。13872 条语料逐例一致(分类628+比较13244)。变异测试证明门禁有捕获能力: Dart 改 double 仅 4/13244 命中(恰为植入陷阱, 说明用例针对性才是强度来源); 静默降级 278/628 命中; 改 Go 清洗表不重新生成则新鲜度护栏失败。已记录过程偏差: P3.5 未单独写计划文档
- Verifier: evidence/p3.5/ACCEPTANCE.md

## EvidenceBundleDraft

- Artifact key: p4-acceptance
- Type: test-report
- Source: flutter analyze/test/build + go test ./... + 手工端到端 HTTP + 隐私边界验证
- Summary: A9 通过。前端 94 项测试、后端 8 包全绿、analyze 无 issue、apk 与 web 均构建成功。端到端走通注册->建项目->出题->交卷->错题并验证隐私(他人读轮次记录得 404)。发现并修复只在 Flutter Web 触发的种子精度 bug: 63 位种子经 JSON 数字(JS 只有 53 位尾数)往返后变值, 导致服务端重放出不同题目、每题判错; 修复为 2^53 上限并附两项测试(一项断言往返无损, 一项证明该断言的必要性)
- Verifier: evidence/p4/ACCEPTANCE.md

## EvidenceBundleDraft

- Artifact key: p5-acceptance
- Type: test-report
- Source: go test ./... (9 包) + flutter analyze/test (107 项) + 端到端三种粒度 + 中位数变异测试
- Summary: A8 通过。新增 internal/stats 36 测试、统计端点 20 测试、折线图 13 测试。端到端手工核对: 三轮 4000/8000/12000ms -> min4000 max12000 avg8000 median8000; 正确率 1.0/0.5/0.0 -> avg0.5 median0.5。隐私边界与派生态均有测试守护。变异测试: 中位数偶数分支改为取下中位数后偶数用例全失败而奇数用例仍通过
- Verifier: evidence/p5/ACCEPTANCE.md

## EvidenceBundleDraft

- Artifact key: p6-acceptance
- Type: test-report
- Source: go test ./... (9 包) + flutter analyze/test (117 项) + 端到端功能5闭环 + 退订变异测试
- Summary: A5/A6 通过。端到端走完分享->订阅->跟随->退订并查库确认: bob 导入 3 条(1 有效/1 令牌无效/1 跨实例)成功 1/3 且逐条有可读原因; bob 统计 3 轮而 alice 2 轮(隐私独立); alice 改项目后 bob 立即看到; bob 退订 deletedRounds=3 后再练习 403 且 alice 记录完好。变异测试: 退订只删订阅行不清轮次时 A6 测试立即失败
- Verifier: evidence/p6/ACCEPTANCE.md

## EvidenceBundleDraft

- Artifact key: p7-acceptance
- Type: test-report
- Source: 测试 keystore 真实构建 release APK + apksigner 验签 + Linux 交叉编译 + Dart YAML 结构校验
- Summary: A15 本地真实验证通过: 用一次性 keystore 走完与 CI 相同的配置路径, apksigner 显示证书 DN=CN=Cala Test Signing(非 debug)。同时修复三个真实问题: P-R8 修复不彻底(环境变量到不了已在运行的进程, 改为 kotlin.incremental=false)、preflight 用 keytool 却未准备 JDK、未加引号的 heredoc 会二次展开口令。A10(tag 触发/ghcr/release 附件)工作流已交付且结构合法但需 CI 才能验证
- Verifier: evidence/p7/ACCEPTANCE.md

## EvidenceBundleDraft

- Artifact key: p7-release-published
- Type: release
- Source: GitHub Actions run #34570826903 + 本机 apksigner 独立验签
- Summary: A10/A15 已验证。Release v0.0.1 已发布, 附件 cala-0.0.1-linux-amd64.tar.gz(8.7MB) 与 cala-0.0.1.apk(50.5MB)。独立验证: 下载已发布 APK, 大小与 SHA-256 与 GitHub 记录一致, apksigner 验签通过(v2 scheme, 单签名者, 非 debug 证书)。证书 DN 全为 Unknown(生成时未填), 不影响功能与可升级性
- Verifier: https://github.com/shinyes/cala/releases/tag/v0.0.1 + 本机 apksigner

## EvidenceBundleDraft

- Artifact key: p7-deploy-defects
- Type: test
- Source: 本机逐层解包 ghcr 镜像 + CI smoke job(run #34587246298) + config 单元测试变异验证
- Summary: 应要求编写 compose 部署配置时，为写出准确配置而核实已发布镜像，发现两个真实缺陷并修复: (1) 镜像中不存在 /data 而容器以 nonroot 运行 -> 命名卷部署必然失败(unable to open database file, 实测 exit 1 硬失败)。已用 COPY --chown=65532 修复; 新增 smoke job 在真实 Docker 中验证, 并本机解包 0.0.2 确认 /data 属主为 65532。 (2) CALA_DEV_CORS_ORIGINS 显式设为空串不会关闭 CORS 而是回退到 localhost:3000, 与代码注释相反。已改用 os.LookupEnv; 新增回归测试并做变异验证(还原后测试如实失败并报出错误值)。v0.0.2 已发布
- Verifier: CI run #34587246298 (6 job 全 success, smoke 8 步全通过) + 本机镜像解包

## EvidenceBundleDraft

- Artifact key: p7-server-address
- Type: test
- Source: 已发布 APK 的 aapt2 解包取证 + 本地 release APK 解包复核 + 两个真实后端的端到端验证 + 42 项新测试(含变异验证)
- Summary: 新增手机端可配置服务端地址(ADR-0006)。实现中发现阻塞缺陷: release APK 缺少 INTERNET 权限(Flutter 只在 debug/profile 清单声明)且 targetSdk=36 禁止明文 HTTP, 即已发布版本无法发起任何网络请求。aapt2 取证确认。已修 main 清单 + networkSecurityConfig 放行明文, 并在新构建的 release APK 中复核生效。功能要点: 地址规范化(无 scheme/末尾斜杠/中间空白/裸 IPv6)、就地改 ApiClient.baseUrl 不重建(保 token)、单点写入且换服务器必清登录态、main() 启动前加载消除竞态、dataScopeProvider 使数据声明式作废、设置页含测试连接, 入口在登录页与我的 Tab。顺带修复 _restore 覆盖新登录态与异步写已销毁 notifier 两个真实缺陷。前端 159 项测试通过(原 117)
- Verifier: flutter analyze/test + aapt2 + 真实 HTTP 双后端端到端

## EvidenceBundleDraft

- Artifact key: p7-keypad-and-text-answer
- Type: test
- Source: 真实后端端到端取证(保存成功+kind=text) + 修复后的真实 HTTP 复验 + 新增 3 项测试
- Summary: 键盘字母表按用户要求由 0-9 . - / 收敛为 0-9 . -(3列5行布局), 去掉分数键保留负号。去掉 / 安全: 判分是有理数交叉相乘, 3/4 与 0.75 相等, 有限小数即可覆盖; 唯一例外是无限循环小数(如 1/3)须配容差, 该后果已写入 keypad.dart 与规格。核实过程中发现真实缺陷: 文本答案的规则能保存成功且服务端正常下发 kind=text, 但键盘打不出中文(练习页不唤起系统键盘), 该题永远答不对 —— 根因是规格 §5.5.2(允许文本)与功能6(自带键盘)从未对账。经用户确认收窄: service 层保存期与开轮时均拒绝文本答案并给出可读理由; 拒绝逻辑收敛为单一 helper。兼容边界: scoring.Classify 仍接受文本(历史信封与错题重练靠它判分), 由 TestScoringStillClassifiesTextAnswer 锁定。真实 HTTP 复验: 文本->400 且消息含题号与改法; 数值与分数->201 无误伤
- Verifier: 真实 HTTP API (127.0.0.1:8090) + 后端 9 包测试 + 前端 159 项

## EvidenceBundleDraft

- Artifact key: p7-answer-integer-decimal-only
- Type: test
- Source: 真实 HTTP 复验(5 种分数形态全部 400, 4 种整数小数全部 201) + 新增 3 项测试 + 语料门禁通过
- Summary: 产品要求收窄为「答案只需整数或小数」, 分数形式(如 a=1/2)在保存期与开轮时均被拒绝。与文本答案同源: 键盘没有除号, 分数形式里无限循环小数(如 1/3)永远敲不出来须靠容差 —— 那正是上一节刚记录的坑; 收窄后「作者写的答案一定能用键盘原样敲出」无条件成立。关键判据: 不能靠信封区分分数与小数(0.5 -> num=5 den=10, 1/2 -> num=1 den=2, 数值相等形状不同), 故取清洗后原始字面量是否含除号(Clean 后判断可覆盖全角斜杠)。刻意区分两类失败: 1/2 是合法但不允许的形式(提示改写小数), 1/0 与 1 空格 / 空格 2 是笔误(按文法畸形, 提示检查写法)。顺带修掉一处由本次收窄引入的不一致: scoring 曾建议改用文本答案而文本此时已被拒绝, 照做会撞上第二道拒绝 —— 该建议已移除(scoring 保持产品无关)。兼容边界: scoring.Classify 与 ParseRational 继续接受分数(小数同以 num/den 落库, 故分数分支一条都不能少), 由 TestScoringStillClassifiesTextAndFraction 锁定, 并验证用户敲 0.5 对 1/2 与 0.5 都判对
- Verifier: 真实 HTTP API 127.0.0.1:8090 + 后端 9 包测试 + 语料门禁

## EvidenceBundleDraft

- Artifact key: p7-image-version
- Type: test
- Source: 本机实测四条版本路径(含坏 DB 路径下的 --version) + 真实服务端契约验证 + 新增 10 项测试
- Summary: 用户要求镜像也要有版本号。核实: 镜像 tag 一直有版本号, 但镜像内部二进制完全没有版本信息(/api/healthz 只返回 status), 而 APK 内部是有真实版本号的(经 --build-name 注入)—— 这个不对称正是「也」字的由来。tag 是仓库侧元数据, 经 save/load 搬运或重新打标签后即与内容无关, 故版本号必须写进镜像。实现: internal/version.Version 默认 dev, 经 Dockerfile ARG VERSION + ldflags -X 注入; 四条读取路径(--version 子命令 / healthz 字段 / OCI 标签 / 启动日志)。刻意决定: --version 在处理配置与数据库之前(实测坏 DB 路径下仍返回 0.0.4 且 exit 0, 因为 distroless 无 shell 只能从外部调用, 若需连库才能回答则在最需要确认版本的场景里反而用不了); 默认值 dev 而非空串; 输出只有版本号本身; 版本号单独成包使 api 不依赖 main; 不做版本比较(用途是标识不是判断)。流水线新增 smoke 断言: tag/OCI 标签/服务端自报/--version 四者一致 —— 针对静默失败(ARG 名或 -X 路径写错、build-args 未传, 镜像都照常构建推送只是自报 dev)。客户端设置页测试连接顺带显示版本, 兼容旧服务端(7 项单测覆盖, 含类型异常不抛异常), 因此抽成纯函数而非内联 setState
- Verifier: 本机真实运行 + curl + App 解析函数对真实响应 + 后端/前端全量测试

## EvidenceBundleDraft

- Artifact key: p7-v004-released
- Type: release
- Source: ghcr registry API 直读标签 + 镜像层解包核对二进制 + Release 附件下载核对 + CI run #34616231075
- Summary: v0.0.4 发布成功(run #34616231075, 提交 b52cb6f, 六个 job 全 success, 含本轮新增的版本号三重一致断言)。独立验证(不依赖 CI 自述): ① 从 ghcr 直接读取已发布镜像的 OCI 标签 —— version=0.0.4, revision 与 tag 所指提交一致; ② **解包镜像层**取出 /cala 二进制, 确认内嵌字符串 0.0.4 存在(20.22MB ELF) —— 这条才是关键证据, 证明 ldflags 注入真的作用到已发布产物, 而非只是构建脚本参数写对了(若 ARG 名或 -X 路径写错, 镜像仍会构建成功且标签正确, 只有二进制里是 dev); ③ 下载 APK 核对: 大小/SHA256 与 GitHub 记录逐字一致, versionName=0.0.4, versionCode=4(比 v0.0.3 的 3 递增, 覆盖安装前提), INTERNET 权限与 networkSecurityConfig 均在(v0.0.3 修复的回归检查), 签名证书 SHA256 与 v0.0.1~v0.0.3 同一身份。compose 的 image 已同步升到 0.0.4
- Verifier: ghcr API + 镜像层解包 + aapt2/apksigner + GitHub Release API

## EvidenceBundleDraft

- Artifact key: p7-change-password
- Type: test
- Source: 真实 HTTP 端到端 20 项检查 + 变异验证(DELETE FROM session 置空后被 4 项测试捕获) + 后端 17 项与前端 13 项新测试
- Summary: 新增修改口令功能。关键设计决定: ① 必须提供当前口令(否则拿到令牌者可直接改密并永久占有账号, 把过期令牌延展为无限期访问); ② 成功后吊销该用户**全部**会话并为当前设备签发新令牌(改密最常见动机是怀疑账号被人登录, 旧会话若仍有效则补救完全落空; 吊销全部而非除当前外, 是为避免漏写排除条件而留下攻击者会话; 随即签发新令牌使当前设备无感续用); ③ 口令更新/会话吊销/新会话签发在同一事务内(分开会留下口令已改但旧会话仍有效或口令已改但客户端以为失败这两种自相矛盾状态); ④ 检查顺序为 长度->校验当前口令->新旧相同, 当前口令必须先于新旧判定(否则用户输错当前口令且新口令等于该错误输入时, 会收到新旧相同这一误导提示); ⑤ 当前口令错误返回 400 **刻意不用 401**(客户端把 401 解释为令牌失效并清除登录态, 会把仅输错一次的用户登出并弹回登录页, 而那正是他想避免的地方); ⑥ 拒绝新旧相同(否则返回成功却什么都没变, 用户以为换掉了泄露的口令); ⑦ 校验规则 owner 划分: 客户端只查服务端无法检查的两件事(非空、两次一致), 长度与新旧相同交给服务端并原样展示其消息, 避免同一条规则两个 owner。验证: 真实 HTTP 端到端 20 项检查全通过(含两设备会话轮换); 变异验证 —— 把事务里的 DELETE FROM session 替换为空操作后 4 项测试如实失败, 证明该不变式确实被守护。测试: 后端 +17, 前端 +13
- Verifier: 真实 HTTP API 127.0.0.1:8090 双会话 + go test + flutter test
