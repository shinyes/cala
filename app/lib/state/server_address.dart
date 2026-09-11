import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../api/client.dart';
import '../api/server_address.dart';
import 'session.dart';

/// 服务端地址在本地存储中的键。
const _serverUrlKey = 'cala.serverUrl';

/// 启动时读取已保存的服务端地址。
///
/// **在 `runApp` 之前调用**，这样应用起来时地址就已确定。
/// 否则会有一个真实的竞态：客户端先以默认地址发出启动请求
///（`SessionNotifier` 恢复登录态时会请求公开配置），
/// 那个请求可能打到错误的服务器，甚至把一个 401 归因到错误的对象上。
///
/// 存储值非法时回退到默认地址而不是让应用起不来：
/// 一个坏掉的偏好设置不应该把用户锁在应用之外。
Future<String> loadSavedServerAddress() async {
  try {
    final prefs = await SharedPreferences.getInstance();
    final saved = prefs.getString(_serverUrlKey);
    if (saved == null || saved.isEmpty) return ApiClient.defaultBaseUrl();
    return normalizeServerAddress(saved);
  } on Object {
    return ApiClient.defaultBaseUrl();
  }
}

/// 当前服务端地址。
///
/// 这是**唯一**的地址写入点：持久化、应用到 ApiClient、作废旧服务器的状态
/// 三件事都在 [ServerAddressNotifier.set] 里一次完成，
/// 避免出现「改了地址但客户端还在用旧地址」这类漂移。
final serverAddressProvider =
    NotifierProvider<ServerAddressNotifier, String>(ServerAddressNotifier.new);

class ServerAddressNotifier extends Notifier<String> {
  /// 初始值来自 [initialServerUrlProvider]，后者由 `main()` 用已保存的值覆盖。
  @override
  String build() => ref.read(initialServerUrlProvider);

  /// 修改服务端地址。
  ///
  /// 失败抛 [ServerAddressError]（消息面向用户），此时**不做任何改动**。
  Future<void> set(String raw) async {
    final normalized = normalizeServerAddress(raw);

    // 地址没变则什么都不做。
    // 这一点很重要：否则用户只是想「看一眼当前地址再保存」就会被登出。
    if (normalized == state) return;

    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_serverUrlKey, normalized);

    // 让客户端指向新地址。就地修改，token 与各 API 包装器都不受影响。
    ref.read(apiClientProvider).baseUrl = normalized;

    // 切换服务器必然作废登录态：token 属于旧服务器，带着它去新服务器
    // 只会得到一串 401。放在这里（而不是让调用方各自处理），
    // 是为了让「换地址 = 重新登录」成为不可绕过的事实。
    await ref.read(sessionProvider.notifier).forgetLocalSession();

    state = normalized;
  }
}

/// 数据作用域：当前「服务端地址 + 登录令牌」的组合。
///
/// 项目、统计、订阅数据都属于某个 (服务器, 用户) 组合。
/// 其中任一方变化时这些数据都必须整体作废，否则界面上会残留
/// 上一台服务器或上一个账号的数据 —— 这类串数据比报错更难察觉。
///
/// 各数据 provider 在 `build()` 里 `ref.watch(dataScopeProvider)` 即可获得
/// 「自动作废」：地址或令牌一变，它们就重建并重新拉取。
/// 这是声明式的，因此新增数据 provider 时不会「忘记作废」。
///
/// 返回记录类型而非拼接字符串：Dart 3 的记录有结构化相等性，
/// 因此令牌里含 `|` 之类的字符也不会造成误判。
final dataScopeProvider = Provider<({String server, String? token})>((ref) {
  return (
    server: ref.watch(serverAddressProvider),
    token: ref.watch(sessionProvider.select((s) => s.token)),
  );
});
