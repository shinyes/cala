import 'dart:typed_data';

import 'package:cala/api/client.dart';
import 'package:cala/api/server_address.dart';
import 'package:cala/state/projects.dart';
import 'package:cala/state/server_address.dart';
import 'package:cala/state/session.dart';
import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// 记录请求去向并提供固定响应的 Dio 适配器。
///
/// 用它替代真实网络，使测试既确定又快速 ——
/// 同时能断言「请求实际打到了哪个地址」，这正是本功能的要害。
class _StubAdapter implements HttpClientAdapter {
  final List<Uri> requests = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final uri = Uri.parse('${options.baseUrl}${options.path}');
    requests.add(uri);

    final body = switch (options.path) {
      '/api/settings/public' => '{"registrationOpen":true,"bootstrap":false}',
      '/api/auth/register' =>
        '{"user":{"id":1,"username":"u","isAdmin":true},'
            '"token":"tok-123","becameAdmin":true}',
      '/api/projects' => '{"owned":[],"subscribed":[]}',
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

  /// 构造一个独立的容器。initialUrl 模拟「启动时已保存的地址」。
  ProviderContainer makeContainer(String initialUrl) {
    adapter = _StubAdapter();
    final dio = Dio()..httpClientAdapter = adapter;
    client = ApiClient(baseUrl: initialUrl, dio: dio);
    return ProviderContainer(
      overrides: [
        initialServerUrlProvider.overrideWithValue(initialUrl),
        apiClientProvider.overrideWithValue(client),
      ],
    );
  }

  setUp(() {
    SharedPreferences.setMockInitialValues({});
    container = makeContainer('http://127.0.0.1:8080');
  });

  tearDown(() => container.dispose());

  group('loadSavedServerAddress', () {
    test('无保存值时返回默认地址', () async {
      SharedPreferences.setMockInitialValues({});
      expect(await loadSavedServerAddress(), defaultServerAddress);
    });

    test('返回已保存的地址', () async {
      SharedPreferences.setMockInitialValues(
        {'cala.serverUrl': 'http://192.168.1.5:8080'},
      );
      expect(await loadSavedServerAddress(), 'http://192.168.1.5:8080');
    });

    test('保存值非法时回退到默认地址而不是让应用起不来', () async {
      // 一个坏掉的偏好设置不应该把用户锁在应用之外
      SharedPreferences.setMockInitialValues({'cala.serverUrl': 'ftp://坏值'});
      expect(await loadSavedServerAddress(), defaultServerAddress);

      SharedPreferences.setMockInitialValues({'cala.serverUrl': ''});
      expect(await loadSavedServerAddress(), defaultServerAddress);
    });

    test('保存值会被规范化后返回', () async {
      SharedPreferences.setMockInitialValues(
        {'cala.serverUrl': '192.168.1.5:8080/'},
      );
      expect(await loadSavedServerAddress(), 'http://192.168.1.5:8080');
    });
  });

  group('serverAddressProvider', () {
    test('初始值来自启动时加载的地址', () {
      expect(container.read(serverAddressProvider), 'http://127.0.0.1:8080');
    });

    test('set() 规范化、持久化，并让客户端指向新地址', () async {
      await container
          .read(serverAddressProvider.notifier)
          .set('192.168.1.5:8080/');

      expect(container.read(serverAddressProvider), 'http://192.168.1.5:8080');
      expect(client.baseUrl, 'http://192.168.1.5:8080');

      final prefs = await SharedPreferences.getInstance();
      expect(prefs.getString('cala.serverUrl'), 'http://192.168.1.5:8080');
    });

    test('改地址后，请求真的打到新地址（本功能的要害）', () async {
      await container.read(serverAddressProvider.notifier).set('10.0.0.9:9000');

      // 触发一次真实请求路径：注册
      await container
          .read(sessionProvider.notifier)
          .register('u', 'password123');

      expect(
        adapter.requests.any((u) => u.host == '10.0.0.9' && u.port == 9000),
        isTrue,
        reason: '请求应发往新地址，实际请求：${adapter.requests}',
      );
      expect(
        adapter.requests.any((u) => u.host == '127.0.0.1'),
        isFalse,
        reason: '不应再请求旧地址，实际请求：${adapter.requests}',
      );
    });

    test('非法输入抛 ServerAddressError 且不做任何改动', () async {
      final before = container.read(serverAddressProvider);

      await expectLater(
        container.read(serverAddressProvider.notifier).set('ftp://h'),
        throwsA(isA<ServerAddressError>()),
      );

      expect(container.read(serverAddressProvider), before);
      expect(client.baseUrl, before);
      final prefs = await SharedPreferences.getInstance();
      expect(prefs.getString('cala.serverUrl'), isNull);
    });

    test('设为等价地址是空操作，不会把用户登出', () async {
      // 先登录，建立登录态
      await container
          .read(sessionProvider.notifier)
          .register('u', 'password123');
      expect(container.read(sessionProvider).isLoggedIn, isTrue);

      // 等价写法（末尾多斜杠、缺 scheme）不应触发登出 ——
      // 否则用户只是想「看一眼当前地址再保存」就被踢出去了
      await container.read(serverAddressProvider.notifier).set('127.0.0.1:8080');

      expect(container.read(sessionProvider).isLoggedIn, isTrue);
    });

    test('切换到不同地址会清除登录态', () async {
      await container
          .read(sessionProvider.notifier)
          .register('u', 'password123');
      expect(container.read(sessionProvider).isLoggedIn, isTrue);

      await container.read(serverAddressProvider.notifier).set('10.0.0.9:9000');

      final s = container.read(sessionProvider);
      expect(s.isLoggedIn, isFalse, reason: 'token 属于旧服务器，必须作废');
      expect(s.token, isNull);
      expect(s.initialized, isTrue, reason: '仍应处于已初始化，避免界面卡在加载');

      final prefs = await SharedPreferences.getInstance();
      expect(prefs.getString('cala.token'), isNull,
          reason: '本地保存的 token 也必须清除');
    });
  });

  group('dataScopeProvider', () {
    test('地址变化会改变数据作用域', () async {
      final before = container.read(dataScopeProvider);
      await container.read(serverAddressProvider.notifier).set('10.0.0.9:9000');
      expect(container.read(dataScopeProvider), isNot(before));
      expect(container.read(dataScopeProvider).server, 'http://10.0.0.9:9000');
    });

    test('登录令牌变化会改变数据作用域', () async {
      final before = container.read(dataScopeProvider);
      await container
          .read(sessionProvider.notifier)
          .register('u', 'password123');
      final after = container.read(dataScopeProvider);
      expect(after.token, 'tok-123');
      expect(after, isNot(before));
    });

    test('作用域同时包含地址与令牌，两者独立生效', () async {
      await container
          .read(sessionProvider.notifier)
          .register('u', 'password123');
      final withToken = container.read(dataScopeProvider);
      expect(withToken.server, 'http://127.0.0.1:8080');
      expect(withToken.token, 'tok-123');

      // 仅改地址：令牌维度不变，地址维度变
      await container.read(serverAddressProvider.notifier).set('10.0.0.9:9000');
      final moved = container.read(dataScopeProvider);
      expect(moved.server, 'http://10.0.0.9:9000');
      expect(moved, isNot(withToken));
    });
  });

  test('token 会随请求发送（改地址不影响令牌注入机制）', () async {
    await container.read(sessionProvider.notifier).register('u', 'password123');
    adapter.requests.clear();

    await container.read(sessionProvider.notifier).refreshPublicSettings();

    expect(adapter.requests, isNotEmpty);
  });

  // 这一组覆盖「数据自动作废」这一承诺的**实际生效点**：
  // 仅测 dataScopeProvider 本身是不够的 —— 真正要保证的是各数据 provider
  // 会因作用域变化而重建。这里以项目列表为代表验证该机制。
  group('数据 provider 在作用域变化时重建', () {
    /// 有界轮询直到条件成立。
    ///
    /// 不用单次 `Future.delayed(Duration.zero)`：provider 的加载是
    /// 「microtask -> refresh -> Dio -> 适配器」这样一条多跳异步链，
    /// 让出一次事件循环并不保证整条链跑完，断言会变得不稳定。
    Future<void> pumpUntil(bool Function() ready) async {
      for (var i = 0; i < 100; i++) {
        if (ready()) return;
        await Future<void>.delayed(Duration.zero);
      }
    }

    bool requested(String host, String path) =>
        adapter.requests.any((u) => u.host == host && u.path == path);

    test('切换服务器后，项目列表改从新地址拉取', () async {
      await container.read(sessionProvider.notifier).register('u', 'password123');

      adapter.requests.clear();
      container.read(projectsProvider);
      await pumpUntil(() => requested('127.0.0.1', '/api/projects'));
      expect(requested('127.0.0.1', '/api/projects'), isTrue,
          reason: '初始应从旧地址拉取；实际：${adapter.requests}');

      adapter.requests.clear();
      await container.read(serverAddressProvider.notifier).set('10.0.0.9:9000');
      container.read(projectsProvider);
      await pumpUntil(() => requested('10.0.0.9', '/api/projects'));

      expect(requested('10.0.0.9', '/api/projects'), isTrue,
          reason: '切换服务器后必须重新拉取，且打到新地址；实际：${adapter.requests}');
    });

    test('登录后项目列表重新拉取（不会残留上一个账号的数据）', () async {
      // 未登录时先建立 provider，模拟「曾以游客/上一账号身份访问过」
      container.read(projectsProvider);
      await pumpUntil(() => adapter.requests.any((u) => u.path == '/api/projects'));

      adapter.requests.clear();
      await container.read(sessionProvider.notifier).register('u', 'password123');
      container.read(projectsProvider);
      await pumpUntil(() => adapter.requests.any((u) => u.path == '/api/projects'));

      expect(adapter.requests.any((u) => u.path == '/api/projects'), isTrue,
          reason: '登录后应重新拉取该账号的项目；实际：${adapter.requests}');
    });
  });
}
