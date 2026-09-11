import 'dart:convert';
import 'dart:typed_data';

import 'package:cala/api/client.dart';
import 'package:cala/state/session.dart';
import 'package:cala/ui/change_password_page.dart';
import 'package:dio/dio.dart';
import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

class _StubAdapter implements HttpClientAdapter {
  int status = 200;
  Object data = const {'token': 'token-new'};
  final List<RequestOptions> requests = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    if (options.path != '/api/auth/password') {
      return ResponseBody.fromString('{}', 200, headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      });
    }
    return ResponseBody.fromString(jsonEncode(data), status, headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    });
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late _StubAdapter adapter;

  setUp(() {
    SharedPreferences.setMockInitialValues({});
    adapter = _StubAdapter();
  });

  Future<void> pumpPage(WidgetTester tester) async {
    final dio = Dio()..httpClientAdapter = adapter;
    final client = ApiClient(baseUrl: 'http://127.0.0.1:8080', dio: dio);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [apiClientProvider.overrideWithValue(client)],
        child: const CupertinoApp(home: ChangePasswordPage()),
      ),
    );
    await tester.pump();
  }

  Future<void> fill(
    WidgetTester tester, {
    required String current,
    required String next,
    required String confirm,
  }) async {
    await tester.enterText(find.widgetWithText(CupertinoTextField, '当前口令'), current);
    await tester.enterText(find.widgetWithText(CupertinoTextField, '新口令'), next);
    await tester.enterText(find.widgetWithText(CupertinoTextField, '确认新口令'), confirm);
    await tester.pump();
  }

  int passwordRequests() =>
      adapter.requests.where((r) => r.path == '/api/auth/password').length;

  testWidgets('渲染三个口令输入框，且都是遮蔽输入', (tester) async {
    await pumpPage(tester);

    expect(find.widgetWithText(CupertinoTextField, '当前口令'), findsOneWidget);
    expect(find.widgetWithText(CupertinoTextField, '新口令'), findsOneWidget);
    expect(find.widgetWithText(CupertinoTextField, '确认新口令'), findsOneWidget);

    // 全部必须遮蔽：口令明文显示在屏幕上是最常见的低级泄露
    for (final f in tester.widgetList<CupertinoTextField>(
        find.byType(CupertinoTextField))) {
      expect(f.obscureText, isTrue, reason: '口令输入框必须遮蔽');
    }
  });

  testWidgets('空字段不发起请求，就地报错', (tester) async {
    await pumpPage(tester);

    await tester.tap(find.widgetWithText(CupertinoButton, '修改口令'));
    await tester.pump();

    expect(find.textContaining('请填写'), findsOneWidget);
    // 客户端能判断的事不必打扰服务端
    expect(passwordRequests(), 0);
  });

  testWidgets('两次新口令不一致时不发起请求，就地报错', (tester) async {
    await pumpPage(tester);
    await fill(tester, current: 'password123', next: 'newpassword456', confirm: 'newpassword457');

    await tester.tap(find.widgetWithText(CupertinoButton, '修改口令'));
    await tester.pump();

    expect(find.textContaining('不一致'), findsOneWidget);
    // 确认框只存在于客户端，服务端看不到它，因此这一检查必须在这里做
    expect(passwordRequests(), 0);
  });

  testWidgets('本地校验通过后请求服务端，并原样展示服务端的错误', (tester) async {
    adapter.status = 400;
    adapter.data = const {
      'error': {'code': 'bad_request', 'message': '当前口令不正确'},
    };

    await pumpPage(tester);
    await fill(tester, current: 'wrongpassword', next: 'newpassword456', confirm: 'newpassword456');

    await tester.tap(find.widgetWithText(CupertinoButton, '修改口令'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(passwordRequests(), 1);
    // 服务端是校验规则的 owner，消息原样展示（客户端不自己编一句）
    expect(find.text('当前口令不正确'), findsOneWidget);
    // 失败时不应导航走 —— 用户要能改完重试
    expect(find.byType(ChangePasswordPage), findsOneWidget);
  });

  testWidgets('口令长度规则由服务端判断，客户端不预先拦截', (tester) async {
    // 客户端刻意不做长度校验（那会让「至少 8 字符」这条规则有两个 owner）。
    // 因此过短的新口令应当**照常发往服务端**，由服务端拒绝并给出消息。
    adapter.status = 400;
    adapter.data = const {
      'error': {'code': 'bad_request', 'message': '口令至少需要 8 个字符'},
    };

    await pumpPage(tester);
    await fill(tester, current: 'password123', next: 'short', confirm: 'short');

    await tester.tap(find.widgetWithText(CupertinoButton, '修改口令'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(passwordRequests(), 1, reason: '长度规则归服务端，客户端不应拦截');
    expect(find.text('口令至少需要 8 个字符'), findsOneWidget);
  });
}
