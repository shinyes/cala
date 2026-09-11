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

  /// 修改当前用户的口令，返回**新**会话令牌。
  ///
  /// 服务端在改密成功后会吊销该用户的**全部**会话（含本机原来那个），
  /// 并为当前设备签发新令牌。调用方**必须**用返回值替换本地保存的令牌，
  /// 否则下一次请求就会 401，用户会被莫名弹回登录页 ——
  /// 而改密的人本来是想留在登录状态里的。
  ///
  /// 校验规则（口令长度、新旧是否相同）的 owner 是服务端，
  /// 因此这里不做本地判断，只把服务端的错误消息原样交给界面展示。
  Future<String> changePassword(
    String currentPassword,
    String newPassword,
  ) async {
    final j = await _c.post('/api/auth/password', {
      'currentPassword': currentPassword,
      'newPassword': newPassword,
    });
    return j['token'] as String? ?? '';
  }

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
