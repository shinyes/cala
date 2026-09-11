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

/// 全局 API 客户端。
final apiClientProvider = Provider<ApiClient>((ref) {
  return ApiClient(baseUrl: ApiClient.defaultBaseUrl());
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
      state = SessionState(
        initialized: true,
        registrationOpen: regOpen,
        bootstrap: bootstrap,
      );
      return;
    }

    _client.token = saved;
    try {
      final user = await _auth.me();
      state = SessionState(
        initialized: true,
        token: saved,
        user: user,
        registrationOpen: regOpen,
        bootstrap: bootstrap,
      );
    } on ApiException catch (e) {
      if (e.isUnauthorized) {
        // 令牌失效：清除本地 token
        await prefs.remove(_tokenKey);
        _client.token = null;
        state = SessionState(
          initialized: true,
          registrationOpen: regOpen,
          bootstrap: bootstrap,
        );
        return;
      }
      // 网络等其它错误：保留 token，让用户重试而不是被迫重新登录
      state = SessionState(
        initialized: true,
        token: saved,
        registrationOpen: regOpen,
        bootstrap: bootstrap,
      );
    }
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
