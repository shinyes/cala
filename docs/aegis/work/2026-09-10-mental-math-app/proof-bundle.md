# Proof Bundle - 2026-09-10-mental-math-app

## Method Pack Boundary

This proof bundle is an advisory Aegis Method Pack record. It does not determine evidence sufficiency, produce authoritative `GateDecision`, or grant `completion authority`.

## Task Intent

- Requested outcome: 交付可自部署的速算练习应用：多用户与管理、JS 自定义出题规则、三 Tab UI、按项目统计、分享订阅、练习运行时与总结/错题、级联删除、tag 触发 CI 出 Docker 镜像
- Scope: backend/ (Go+Fiber+SQLite+goja) 与 app/ (Flutter) 全量实现，含 CI

## Impact

- Compatibility boundary: Compatibility boundary not yet refined.
- Non-goals:
- iOS 发布, 离线优先, 题目市场, 邮件找回密码, 管理员后台, 未完成练习续做

## Evidence Bundle Refs

- docs/aegis/work/2026-09-10-mental-math-app/evidence-bundle-draft-goja-spike-output.json
- docs/aegis/work/2026-09-10-mental-math-app/evidence-bundle-draft-p0-p1-acceptance.json
- docs/aegis/work/2026-09-10-mental-math-app/evidence-bundle-draft-p2-acceptance.json
- docs/aegis/work/2026-09-10-mental-math-app/evidence-bundle-draft-p3-5-acceptance.json
- docs/aegis/work/2026-09-10-mental-math-app/evidence-bundle-draft-p3-acceptance.json
- docs/aegis/work/2026-09-10-mental-math-app/evidence-bundle-draft-p4-acceptance.json
- docs/aegis/work/2026-09-10-mental-math-app/evidence-bundle-draft-p5-acceptance.json
- docs/aegis/work/2026-09-10-mental-math-app/evidence-bundle-draft-p6-acceptance.json
- docs/aegis/work/2026-09-10-mental-math-app/evidence-bundle-draft-spec-approved.json
- docs/aegis/work/2026-09-10-mental-math-app/evidence-bundle-draft-sqlite-cascade-spike.json

## Drift Check

- Scope status: status: 完成规格§16 的 P6; 未越界到 P7
- Compatibility status: status: (project,user) 单一级联不变量保持(退订与删项目共用同一条外键规则); 订阅不授予写权限已验证; 未提供可单独调用的删订阅行原语
- Retirement status: status: 无退役对象; P4 的两处『尚未提供』提示已随真实功能移除; 明确拒绝短链接/二维码/订阅通知/跨实例订阅; 变异测试代码已还原
- Advisory decision: continue
