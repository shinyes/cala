import 'dart:convert';
import 'dart:typed_data';

import 'package:cala/api/client.dart';
import 'package:cala/state/session.dart';
import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// 记录请求并可配置响应的 Dio 适配器。
class _StubAdapter implements HttpClientAdapter {
  /// `/api/auth/password` 的响应。为 null 时返回成功并下发新令牌。
  Response? passwordResponse;

  final List<RequestOptions> requests = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);

    if (options.path == '/api/auth/password' && passwordResponse != null) {
      final r = passwordResponse!;
      return ResponseBody.fromString(
        jsonEncode(r.data),
        r.statusCode ?? 200,
        headers: {
          Headers.contentTypeHeader: [Headers.jsonContentType],
        },
      );
    }

    final body = switch (options.path) {
      '/api/settings/public' => '{"registrationOpen":true,"bootstrap":false}',
      '/api/auth/register' =>
        '{"user":{"id":1,"username":"alice","isAdmin":false},'
            '"token":"token-old","becameAdmin":false}',
      '/api/auth/password' => '{"token":"token-new"}',
      '/api/me' => '{"user":{"id":1,"username":"alice","isAdmin":false}}',
      _ => '{"status":"ok"}',
    };

    return ResponseBody.fromString(
      body,
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late _StubAdapter adapter;
  late ApiClient client;
  late ProviderContainer container;

  /// 错误体：模拟服务端返回 bad_request
  Response errorResponse(String code, String message) => Response(
        requestOptions: RequestOptions(path: '/api/auth/password'),
        statusCode: 400,
        data: {
          'error': {'code': code, 'message': message},
        },
      );

  setUp(() async {
    SharedPreferences.setMockInitialValues({});
    adapter = _StubAdapter();
    final dio = Dio()..httpClientAdapter = adapter;
    client = ApiClient(baseUrl: 'http://127.0.0.1:8080', dio: dio);
    container = ProviderContainer(
      overrides: [apiClientProvider.overrideWithValue(client)],
    );
    // 建立登录态
    await container.read(sessionProvider.notifier).register('alice', 'password123');
  });

  tearDown(() => container.dispose());

  group('changePassword 成功路径', () {
    test('用返回的新令牌替换本地令牌（否则用户下一步会被登出）', () async {
      expect(container.read(sessionProvider).token, 'token-old');

      await container
          .read(sessionProvider.notifier)
          .changePassword('password123', 'newpassword456');

      // 内存中的客户端令牌
      expect(client.token, 'token-new');
      // 状态里的令牌
      expect(container.read(sessionProvider).token, 'token-new');
      // 持久化的令牌 —— 否则重启后会用已吊销的旧令牌
      final prefs = await SharedPreferences.getInstance();
      expect(prefs.getString('cala.token'), 'token-new');
    });

    test('登录态保持，用户资料不丢', () async {
      await container
          .read(sessionProvider.notifier)
          .changePassword('password123', 'newpassword456');

      final s = container.read(sessionProvider);
      expect(s.isLoggedIn, isTrue);
      expect(s.user?.username, 'alice');
      expect(s.initialized, isTrue);
    });

    test('请求体字段名与后端契约一致', () async {
      await container
          .read(sessionProvider.notifier)
          .changePassword('password123', 'newpassword456');

      final req = adapter.requests.lastWhere((r) => r.path == '/api/auth/password');
      expect(req.method, 'POST');
      final body = req.data as Map<String, dynamic>;
      // 字段名写错会静默变成空串，服务端只会说「当前口令不正确」
      expect(body['currentPassword'], 'password123');
      expect(body['newPassword'], 'newpassword456');
      expect(body.keys.toSet(), {'currentPassword', 'newPassword'});
    });

    test('改密后的请求带上新令牌', () async {
      await container
          .read(sessionProvider.notifier)
          .changePassword('password123', 'newpassword456');

      adapter.requests.clear();
      await container.read(sessionProvider.notifier).refreshPublicSettings();

      final req = adapter.requests.last;
      expect(req.headers['Authorization'], 'Bearer token-new');
    });
  });

  group('changePassword 失败路径', () {
    test('服务端拒绝时令牌与登录态都不变', () async {
      adapter.passwordResponse =
          errorResponse('bad_request', '当前口令不正确');

      await expectLater(
        container
            .read(sessionProvider.notifier)
            .changePassword('wrongpassword', 'newpassword456'),
        throwsA(isA<ApiException>()),
      );

      expect(client.token, 'token-old');
      expect(container.read(sessionProvider).token, 'token-old');
      expect(container.read(sessionProvider).isLoggedIn, isTrue);
      final prefs = await SharedPreferences.getInstance();
      expect(prefs.getString('cala.token'), 'token-old');
    });

    test('错误消息原样保留，供界面展示', () async {
      adapter.passwordResponse =
          errorResponse('bad_request', '口令至少需要 8 个字符');

      try {
        await container
            .read(sessionProvider.notifier)
            .changePassword('password123', 'short');
        fail('应当抛出 ApiException');
      } on ApiException catch (e) {
        expect(e.message, '口令至少需要 8 个字符');
        expect(e.code, 'bad_request');
      }
    });

    test('当前口令错误不是 unauthorized（否则界面会把用户登出）', () async {
      // 服务端契约：该情形回 400 bad_request。
      // 若某天改成 401，客户端的 isUnauthorized 会清除登录态，
      // 用户仅仅输错一次口令就被弹回登录页。
      adapter.passwordResponse =
          errorResponse('bad_request', '当前口令不正确');

      try {
        await container
            .read(sessionProvider.notifier)
            .changePassword('wrongpassword', 'newpassword456');
        fail('应当抛出 ApiException');
      } on ApiException catch (e) {
        expect(e.isUnauthorized, isFalse,
            reason: '当前口令错误绝不能被判为令牌失效');
      }
    });

    test('服务端未返回令牌时明确报错，且不沿用旧令牌', () async {
      adapter.passwordResponse = Response(
        requestOptions: RequestOptions(path: '/api/auth/password'),
        statusCode: 200,
        data: const <String, dynamic>{}, // 契约要求返回 token，这里刻意缺失
      );

      await expectLater(
        container
            .read(sessionProvider.notifier)
            .changePassword('password123', 'newpassword456'),
        throwsA(isA<ApiException>()),
      );

      // 静默沿用旧令牌会让用户下一步被登出且不知原因，因此必须报错
      expect(client.token, 'token-old');
    });
  });
}
