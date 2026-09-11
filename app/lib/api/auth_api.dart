import 'client.dart';
import 'models.dart';

/// 认证相关端点（规格 §7）。
class AuthApi {
  final ApiClient _c;
  AuthApi(this._c);

  /// 注册结果。[becameAdmin] 为 true 表示这是引导管理员（首个用户）。
  Future<({User user, String token, bool becameAdmin})> register(
    String username,
    String password,
  ) async {
    final j = await _c.post('/api/auth/register', {
      'username': username,
      'password': password,
    });
    return (
      user: User.fromJson((j['user'] as Map<String, dynamic>?) ?? const {}),
      token: j['token'] as String? ?? '',
      becameAdmin: j['becameAdmin'] as bool? ?? false,
    );
  }

  Future<({User user, String token})> login(
    String username,
    String password,
  ) async {
    final j = await _c.post('/api/auth/login', {
      'username': username,
      'password': password,
    });
    return (
      user: User.fromJson((j['user'] as Map<String, dynamic>?) ?? const {}),
      token: j['token'] as String? ?? '',
    );
  }

  Future<void> logout() => _c.post('/api/auth/logout');

  Future<User> me() async {
    final j = await _c.get('/api/me');
    return User.fromJson((j['user'] as Map<String, dynamic>?) ?? const {});
  }

  /// 公开配置。无需登录。
  ///
  /// [registrationOpen] 是**服务端算好的有效值**——已计入「系统无用户时无视
  /// 开关」的引导管理员规则（规格 §6.2）。客户端不得自行组合判断：
  /// 那会让同一条规则出现两个 owner，且客户端根本不知道用户数。
  ///
  /// [bootstrap] 为 true 表示注册者将成为管理员，界面据此给出提示。
  Future<({bool registrationOpen, bool bootstrap})> publicSettings() async {
    final j = await _c.get('/api/settings/public');
    return (
      registrationOpen: j['registrationOpen'] as bool? ?? false,
      bootstrap: j['bootstrap'] as bool? ?? false,
    );
  }

  /// 修改注册开关（仅管理员）。返回写入后的原始开关值。
  Future<bool> setRegistrationOpen(bool open) async {
    final j = await _c.put('/api/admin/settings', {'open': open});
    return j['registrationOpen'] as bool? ?? open;
  }
}
