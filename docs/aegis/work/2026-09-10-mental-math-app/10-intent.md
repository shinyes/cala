# 速算练习 App（Go+Fiber+SQLite 后端 / Flutter 安卓前端） - Intent

## TaskIntentDraft

- Requested outcome: 交付可自部署的速算练习应用：多用户与管理、JS 自定义出题规则、三 Tab UI、按项目统计、分享订阅、练习运行时与总结/错题、级联删除、tag 触发 CI 出 Docker 镜像
- Goal: 交付可自部署的速算练习应用：多用户与管理、JS 自定义出题规则、三 Tab UI、按项目统计、分享订阅、练习运行时与总结/错题、级联删除、tag 触发 CI 出 Docker 镜像
- Success evidence:
- 本地 go test ./... 与 flutter test 通过；浏览器可完整走通 注册→建项目→练习→总结→错题重练→统计→分享订阅；打 tag 后 CI 产出镜像并推送 ghcr 且 release 附 tar.gz
- Stop condition: done=上述证据齐备；blocked=外部凭据/环境缺失；needs-verification=未跑通端到端；scope-exceeded=超出十项功能范围
- Non-goals:
- iOS 发布, 离线优先, 题目市场, 邮件找回密码, 管理员后台, 未完成练习续做
- Scope: backend/ (Go+Fiber+SQLite+goja) 与 app/ (Flutter) 全量实现，含 CI
- Change kinds:
- new-feature
- Risk hints:
- 规则契约发布后难改；跨用户级联删除不可逆；订阅者退订即清空历史

## BaselineReadSetHint

- none

## BaselineUsageDraft

- Required baseline refs:
- none
- Acknowledged before plan:
- none
- Cited in plan:
- none
- Missing refs:
- none
- Advisory decision: continue

## ImpactStatementDraft

- Compatibility boundary: Compatibility boundary not yet refined.
- Affected layers:
- backend,frontend,ci,datastore
- Owners:
- project-root
- Invariants:
- 答题记录生命周期绑定 (project,user) 关系；规则沙箱必须同时设置 maxCallStack 与 Interrupt
- Non-goals:
- iOS 发布, 离线优先, 题目市场, 邮件找回密码, 管理员后台, 未完成练习续做

These records are Method Pack drafts / hints, not authoritative runtime decisions.
