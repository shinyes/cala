import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../api/auth_api.dart';
import '../api/client.dart';
import '../api/models.dart';

/// token 在本地存储中的键。
///
/// 只存 token，不存用户资料：用户资料每次启动重新拉取，
/// 避免本地缓存与后端不一致（例如账号被禁用后本地仍显示已登录）。
const _tokenKey = 'cala.token';

/// 启动时的服务端地址。
///
/// 默认值只适合桌面/浏览器调试；`main()` 会用本地保存的地址覆盖它，
/// 因此应用起来时地址已经确定，不存在「先用默认地址发一个请求」的竞态。
///
/// 放在本文件是因为它服务于 [apiClientProvider] 的构造；
/// 运行期的地址变更由 `server_address.dart` 的 `serverAddressProvider` 负责。
final initialServerUrlProvider =
    Provider<String>((ref) => ApiClient.defaultBaseUrl());

/// 全局 API 客户端。
///
/// 只创建一次。地址变更通过 `ApiClient.baseUrl` 就地修改（见其 setter 注释），
/// 不重建实例 —— 重建会丢失 token，且各 API 包装器仍持有旧实例。
final apiClientProvider = Provider<ApiClient>((ref) {
  return ApiClient(baseUrl: ref.watch(initialServerUrlProvider));
});

/// 认证接口。
final authApiProvider = Provider<AuthApi>((ref) {
  return AuthApi(ref.watch(apiClientProvider));
});

/// 登录态。
@immutable
class SessionState {
  /// 是否已完成启动时的登录态恢复（避免启动瞬间闪烁登录页）。
  final bool initialized;

  final String? token;
  final User? user;

  /// 注册开关的**有效值**（服务端算好，已计入引导管理员规则）。
  final bool registrationOpen;

  /// 系统尚无用户：注册者将成为管理员。
  final bool bootstrap;

  const SessionState({
    this.initialized = false,
    this.token,
    this.user,
    this.registrationOpen = true,
    this.bootstrap = false,
  });

  bool get isLoggedIn => token != null && token!.isNotEmpty && user != null;

  /// 是否应显示注册入口。
  ///
  /// 直接采用服务端的有效值，**不**在客户端重新组合规则：
  /// 「开关关闭但无用户时仍可注册」这条规则只有一个 owner（服务端），
  /// 客户端既不知道用户数，也不该猜。
  bool get showRegistration => registrationOpen;

  SessionState copyWith({
    bool? initialized,
    String? token,
    bool clearToken = false,
    User? user,
    bool clearUser = false,
    bool? registrationOpen,
    bool? bootstrap,
  }) {
    return SessionState(
      initialized: initialized ?? this.initialized,
      token: clearToken ? null : (token ?? this.token),
      user: clearUser ? null : (user ?? this.user),
      registrationOpen: registrationOpen ?? this.registrationOpen,
      bootstrap: bootstrap ?? this.bootstrap,
    );
  }
}

/// 登录态与认证动作。
class SessionNotifier extends Notifier<SessionState> {
  @override
  SessionState build() {
    // 启动时异步恢复；此处同步返回初始值。
    Future.microtask(_restore);
    return const SessionState();
  }

  ApiClient get _client => ref.read(apiClientProvider);
  AuthApi get _auth => ref.read(authApiProvider);

  /// 启动时恢复 token，并校验它是否仍然有效。
  ///
  /// 无效即清除：否则界面会停在「看起来已登录但每个请求都 401」的状态。
  Future<void> _restore() async {
    final prefs = await SharedPreferences.getInstance();
    final saved = prefs.getString(_tokenKey);

    // 公开配置在未登录时也可读；失败时保守地允许显示注册入口，
    // 否则后端不可达会让用户以为注册被关闭了。
    var regOpen = true;
    var bootstrap = false;
    try {
      final pub = await _auth.publicSettings();
      regOpen = pub.registrationOpen;
      bootstrap = pub.bootstrap;
    } on Object {
      regOpen = true;
      bootstrap = false;
    }

    if (saved == null || saved.isEmpty) {
      _applyRestored(SessionState(
        initialized: true,
        registrationOpen: regOpen,
        bootstrap: bootstrap,
      ));
      return;
    }

    _client.token = saved;
    try {
      final user = await _auth.me();
      _applyRestored(SessionState(
        initialized: true,
        token: saved,
        user: user,
        registrationOpen: regOpen,
        bootstrap: bootstrap,
      ));
    } on ApiException catch (e) {
      if (e.isUnauthorized) {
        // 令牌失效：清除本地 token
        await prefs.remove(_tokenKey);
        _client.token = null;
        _applyRestored(SessionState(
          initialized: true,
          registrationOpen: regOpen,
          bootstrap: bootstrap,
        ));
        return;
      }
      // 网络等其它错误：保留 token，让用户重试而不是被迫重新登录
      _applyRestored(SessionState(
        initialized: true,
        token: saved,
        registrationOpen: regOpen,
        bootstrap: bootstrap,
      ));
    }
  }

  /// 应用恢复结果，但**不**覆盖在恢复期间已经建立的登录态。
  ///
  /// 恢复是异步的（要读存储并请求服务端），期间用户完全可能已经登录 ——
  /// 例如启动后立刻注册。若此时照常写入恢复结果，就会把刚建立起来的
  /// 登录态覆盖成「未登录」，表现为「注册成功后又被弹回登录页」。
  ///
  /// 同时跳过已销毁的情形：恢复的微任务可能在容器销毁之后才完成，
  /// 此时写 state 会抛错。
  void _applyRestored(SessionState restored) {
    if (!ref.mounted) return;
    if (state.isLoggedIn) return;
    state = restored;
  }

  Future<void> _persist(String token, User user) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_tokenKey, token);
    _client.token = token;
    state = state.copyWith(token: token, user: user, initialized: true);
  }

  /// 注册。返回是否为引导管理员。
  Future<bool> register(String username, String password) async {
    final res = await _auth.register(username, password);
    await _persist(res.token, res.user);
    // 注册成功后系统必有用户，bootstrap 不再成立
    state = state.copyWith(bootstrap: false);
    return res.becameAdmin;
  }

  Future<void> login(String username, String password) async {
    final res = await _auth.login(username, password);
    await _persist(res.token, res.user);
  }

  /// 登出。即使后端调用失败也要清除本地状态——
  /// 否则用户会卡在「点登出没反应」的状态。
  Future<void> logout() async {
    try {
      await _auth.logout();
    } on ApiException {
      // 忽略：本地登出必须生效
    } on Object {
      // 忽略
    }
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_tokenKey);
    _client.token = null;
    state = state.copyWith(clearToken: true, clearUser: true);
    // 登出后重新读取公开配置（可能已变化）
    await refreshPublicSettings();
  }

  /// 清除本地登录态，**不调用后端**。
  ///
  /// 用于切换服务端地址：旧服务器的 token 对新服务器无效，
  /// 而此刻旧服务器很可能正不可达（这常常正是用户切换地址的原因），
  /// 因此不能依赖一次网络登出 —— 那会让切换卡住或残留登录态。
  ///
  /// 与 [logout] 的区别：[logout] 会尽力通知后端吊销会话，用于正常登出。
  Future<void> forgetLocalSession() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_tokenKey);
    _client.token = null;
    // 重置为「未登录且已初始化」，并丢弃上一台服务器的公开配置 ——
    // 注册开关是**每台服务器各自的**，沿用旧值会误导用户。
    state = const SessionState(initialized: true);
    await refreshPublicSettings();
  }

  /// 修改口令。
  ///
  /// 失败时抛 [ApiException]，其 message 由服务端给出（例如「当前口令不正确」
  /// 「口令至少需要 8 个字符」），调用方应**原样**展示。
  ///
  /// 成功后本机必须立即换用新令牌：服务端已吊销该用户的全部会话
  /// （含本机原来那个），继续用旧令牌会让下一次请求 401。
  Future<void> changePassword(
    String currentPassword,
    String newPassword,
  ) async {
    final newToken = await _auth.changePassword(currentPassword, newPassword);
    if (newToken.isEmpty) {
      // 服务端契约保证返回令牌。真为空说明契约被破坏 ——
      // 明确报错而不是默默沿用旧令牌（那会让用户下一步被登出且不知原因）。
      throw const ApiException(
        code: 'internal',
        message: '服务端未返回新令牌，请重新登录',
      );
    }

    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_tokenKey, newToken);
    _client.token = newToken;
    // 用户资料未变，只换令牌
    state = state.copyWith(token: newToken);
  }

  /// 刷新公开配置（管理员改开关后调用）。
  Future<void> refreshPublicSettings() async {
    try {
      final pub = await _auth.publicSettings();
      state = state.copyWith(
        registrationOpen: pub.registrationOpen,
        bootstrap: pub.bootstrap,
      );
    } on Object {
      // 保持原值
    }
  }

  /// 管理员修改注册开关。
  Future<void> setRegistrationOpen(bool open) async {
    await _auth.setRegistrationOpen(open);
    await refreshPublicSettings();
  }
}

final sessionProvider =
    NotifierProvider<SessionNotifier, SessionState>(SessionNotifier.new);
