# ADR-0006 - 服务端地址可配置与明文 HTTP 放行

Status: `recorded-from-work`
Date: `2026-09-11`

## Source Evidence

- evidence/p7/ACCEPTANCE.md §6.6; 已发布 APK 的 aapt2 取证; 两个真实后端的端到端验证
## Context

手机端必须能连接用户自建的后端，而默认地址 127.0.0.1 在手机上指向手机自身。实现该功能时发现 release APK 根本没有 INTERNET 权限（Flutter 模板只在 debug/profile 清单里声明它），且 targetSdk=36 使明文 HTTP 默认被禁；即已发布版本无法发起任何网络请求。后端只支持 HTTP，用户部署通常在局域网内。

## Decision

① 服务端地址由用户在应用内配置并持久化，设置入口同时放在登录页与我的 Tab —— 连不上服务器时登录框本身是死的，必须能在登录前修改。② 地址就地修改 ApiClient.baseUrl，不重建客户端：token 存在实例上且各 API 包装器持有同一实例。③ 修改地址必然清除登录态（token 属于旧服务器），设为等价地址则为空操作。④ 启动前在 main() 加载地址，消除用默认地址发首个请求的竞态。⑤ 数据 provider 通过 watch dataScopeProvider（地址+令牌）自动作废，声明式而非各自记得清理。⑥ main 清单声明 INTERNET 权限，并以 networkSecurityConfig 放行明文 HTTP。

## Alternatives Considered

- ① 重建 ApiClient 而非就地改 baseUrl——会丢 token 且包装器指向旧实例。② 强制首次启动向导填写地址——增加流程复杂度，改为在登录页显著展示当前地址。③ 用 android:usesCleartextTraffic=true——效果等价但不如 networkSecurityConfig 明确，且后者留有按域名细化的余地。④ 把明文限制在私有网段——该配置按域名匹配，不支持 IP 段，而用户地址任意。⑤ 要求 HTTPS——后端未实现 TLS，等于让功能不可用。
## Consequences

- 明文放行是安全取舍：局域网内可被嗅探。但这是后端只支持 HTTP 的必然结果，非本决定引入。日后后端支持 TLS 后应收紧为按域名放行。地址可配置使「换服务器=重新登录」成为不可绕过的事实，避免出现两个服务器的数据混在一起。
## Compatibility Boundary

服务端地址是客户端本地设置，不影响任何 API 契约、判分语义或数据库 schema。dataScopeProvider 的引入改变了数据 provider 的重建时机（登录/登出/换地址时重新拉取），这是修正而非破坏——此前换账号后可能残留旧数据。

## Retirement Impact

无退役项。若日后引入 HTTPS-only 部署模式，network_security_config 的 base-config 应改为按域名放行明文，本 ADR 需同步修订。

## Baseline Sync

- Needed: needed
- Target: docs/aegis/baseline/2026-09-10-initial-baseline.md
- Action: update baseline
- Reason: 产品的运行时配置面扩大（新增用户可配置的服务端地址），且新增一条网络层安全边界（放行明文 HTTP）。基线 §5.2 的八条不可协商项不受影响，但「部署与客户端连接」这一维度需要记录。

## Evidence References

- docs/aegis/work/2026-09-10-mental-math-app/evidence/p7/ACCEPTANCE.md
## Boundary

This ADR is an advisory Aegis Method Pack record. It does not grant completion authority or replace project-authoritative architecture sources.
