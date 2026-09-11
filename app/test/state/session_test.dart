import 'package:cala/api/models.dart';
import 'package:cala/state/session.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  group('SessionState 派生属性', () {
    test('无 token 时为未登录', () {
      const s = SessionState();
      expect(s.isLoggedIn, isFalse);
    });

    test('空字符串 token 视为未登录', () {
      const s = SessionState(token: '');
      expect(s.isLoggedIn, isFalse);
    });

    test('有 token 但无 user 时仍视为未登录（避免半登录态）', () {
      const s = SessionState(token: 'abc');
      expect(s.isLoggedIn, isFalse);
    });

    test('有 token 且有 user 时为已登录', () {
      const u = User(
        id: 1,
        username: 'a',
        isAdmin: false,
        disabled: false,
        createdAt: '',
      );
      const s = SessionState(token: 'abc', user: u);
      expect(s.isLoggedIn, isTrue);
    });
  });

  group('注册入口显隐（防锁死）', () {
    // 这一组对应规格 §6.2 的界面侧体现。
    // 关键点：客户端直接采用服务端的**有效值**，不自行组合规则。

    test('服务端有效值为 true 时显示注册入口', () {
      const s = SessionState(registrationOpen: true);
      expect(s.showRegistration, isTrue);
    });

    test('服务端有效值为 false 时隐藏注册入口', () {
      const s = SessionState(registrationOpen: false);
      expect(s.showRegistration, isFalse);
    });

    test('无用户时服务端返回有效值 true，因此入口显示（不会自我锁死）', () {
      // 模拟全新部署 + 开关关闭：服务端会把有效值算成 true，
      // 客户端只需照此渲染。若客户端自行实现该规则，它无从得知用户数。
      const s = SessionState(registrationOpen: true, bootstrap: true);
      expect(s.showRegistration, isTrue);
      expect(s.bootstrap, isTrue, reason: '界面应据此提示将成为管理员');
    });

    test('bootstrap 标记与注册入口相互独立', () {
      // bootstrap=true 但入口被关（理论上不会同时出现，但派生属性不应耦合）
      const s = SessionState(registrationOpen: false, bootstrap: true);
      expect(s.showRegistration, isFalse);
    });
  });

  group('状态转移', () {
    test('clearToken 清除令牌与用户', () {
      const u = User(
        id: 1,
        username: 'a',
        isAdmin: false,
        disabled: false,
        createdAt: '',
      );
      const s = SessionState(token: 'abc', user: u);
      final cleared = s.copyWith(clearToken: true, clearUser: true);
      expect(cleared.token, isNull);
      expect(cleared.user, isNull);
      expect(cleared.isLoggedIn, isFalse);
    });

    test('copyWith 保留未指定字段', () {
      const s = SessionState(registrationOpen: false, bootstrap: true);
      final updated = s.copyWith(initialized: true);
      expect(updated.registrationOpen, isFalse);
      expect(updated.bootstrap, isTrue);
      expect(updated.initialized, isTrue);
    });
  });
}
