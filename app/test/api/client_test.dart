import 'dart:convert';
import 'dart:typed_data';

import 'package:cala/api/client.dart';
import 'package:cala/api/models.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// 离线适配器：按预设脚本应答，不发真实请求。
///
/// 测试必须离线可跑——否则 CI 在无后端时会失败，
/// 而且「网络是否恰好可用」会变成测试结果的一部分。
class _StubAdapter implements HttpClientAdapter {
  _StubAdapter(this.handler);

  final ResponseBody Function(RequestOptions options) handler;

  /// 最近一次请求，供断言 token 注入与请求体。
  RequestOptions? lastRequest;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    lastRequest = options;
    return handler(options);
  }

  @override
  void close({bool force = false}) {}
}

ResponseBody _json(Object body, int status) => ResponseBody.fromString(
      jsonEncode(body),
      status,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );

ApiClient _clientWith(_StubAdapter stub) {
  final dio = Dio()..httpClientAdapter = stub;
  return ApiClient(baseUrl: 'http://test.local', dio: dio);
}

void main() {
  group('错误契约映射', () {
    test('后端 error.code / error.message 被正确提取', () async {
      final stub = _StubAdapter((_) => _json({
            'error': {'code': 'rule_invalid', 'message': '第 1 题的答案无法分类'}
          }, 400));
      final c = _clientWith(stub);

      try {
        await c.post('/api/projects', {'title': 'x'});
        fail('应当抛出 ApiException');
      } on ApiException catch (e) {
        expect(e.code, 'rule_invalid');
        expect(e.message, '第 1 题的答案无法分类');
        expect(e.status, 400);
        expect(e.isRuleInvalid, isTrue);
      }
    });

    test('401 被识别为未授权', () async {
      final stub = _StubAdapter((_) => _json({
            'error': {'code': 'unauthorized', 'message': '令牌无效'}
          }, 401));
      final c = _clientWith(stub);

      try {
        await c.get('/api/me');
        fail('应当抛出 ApiException');
      } on ApiException catch (e) {
        expect(e.isUnauthorized, isTrue);
      }
    });

    test('registration_closed 被识别', () async {
      final stub = _StubAdapter((_) => _json({
            'error': {'code': 'registration_closed', 'message': '管理员已关闭注册'}
          }, 403));
      final c = _clientWith(stub);

      try {
        await c.post('/api/auth/register');
        fail('应当抛出 ApiException');
      } on ApiException catch (e) {
        expect(e.isRegistrationClosed, isTrue);
      }
    });

    test('无 error 体的 500 也抛 ApiException 而非崩溃', () async {
      final stub = _StubAdapter((_) => _json({'unexpected': true}, 500));
      final c = _clientWith(stub);

      try {
        await c.get('/api/me');
        fail('应当抛出 ApiException');
      } on ApiException catch (e) {
        expect(e.code, 'internal');
        expect(e.status, 500);
        expect(e.message, isNotEmpty);
      }
    });

    test('反代返回 HTML 错误页时不因解析失败而掩盖问题', () async {
      final stub = _StubAdapter(
          (_) => ResponseBody.fromString('<html>502 Bad Gateway</html>', 502));
      final c = _clientWith(stub);

      try {
        await c.get('/api/me');
        fail('应当抛出 ApiException');
      } on ApiException catch (e) {
        expect(e.status, 502);
        expect(e.code, 'internal');
      }
    });

    test('连接失败映射为 network', () async {
      final stub = _StubAdapter((o) => throw DioException(
            requestOptions: o,
            type: DioExceptionType.connectionError,
          ));
      final c = _clientWith(stub);

      try {
        await c.get('/api/me');
        fail('应当抛出 ApiException');
      } on ApiException catch (e) {
        expect(e.code, 'network');
        expect(e.isNetwork, isTrue);
        expect(e.message, contains('无法连接'));
      }
    });

    test('超时映射为 network 且提示超时', () async {
      final stub = _StubAdapter((o) => throw DioException(
            requestOptions: o,
            type: DioExceptionType.connectionTimeout,
          ));
      final c = _clientWith(stub);

      try {
        await c.get('/api/me');
        fail('应当抛出 ApiException');
      } on ApiException catch (e) {
        expect(e.isNetwork, isTrue);
        expect(e.message, contains('超时'));
      }
    });
  });

  group('token 注入', () {
    test('无 token 时不带 Authorization 头', () async {
      final stub = _StubAdapter((_) => _json({'user': {}}, 200));
      final c = _clientWith(stub);
      await c.get('/api/me');

      expect(stub.lastRequest!.headers.containsKey('Authorization'), isFalse);
    });

    test('有 token 时注入 Bearer', () async {
      final stub = _StubAdapter((_) => _json({'user': {}}, 200));
      final c = _clientWith(stub)..token = 'abc123';
      await c.get('/api/me');

      expect(stub.lastRequest!.headers['Authorization'], 'Bearer abc123');
    });

    test('空字符串 token 视为未登录', () async {
      final stub = _StubAdapter((_) => _json({'user': {}}, 200));
      final c = _clientWith(stub)..token = '';
      await c.get('/api/me');

      expect(stub.lastRequest!.headers.containsKey('Authorization'), isFalse);
    });

    test('登出后清空 token，后续请求不带令牌', () async {
      final stub = _StubAdapter((_) => _json({'user': {}}, 200));
      final c = _clientWith(stub)..token = 'abc123';
      await c.get('/api/me');
      expect(stub.lastRequest!.headers['Authorization'], 'Bearer abc123');

      c.token = null;
      await c.get('/api/me');
      expect(stub.lastRequest!.headers.containsKey('Authorization'), isFalse);
    });
  });

  group('响应解析', () {
    test('204 空响应返回空对象而非崩溃', () async {
      final stub = _StubAdapter((_) => ResponseBody.fromString('', 204));
      final c = _clientWith(stub);

      final r = await c.delete('/api/projects/1');
      expect(r, isEmpty);
    });

    test('User 缺字段时给默认值', () {
      final u = User.fromJson(const {'id': 7});
      expect(u.id, 7);
      expect(u.username, '');
      expect(u.isAdmin, isFalse);
    });

    test('Project 容差两列可同时缺省', () {
      final p = Project.fromJson(const {'id': 1, 'title': 'x'});
      expect(p.hasTolerance, isFalse);
      expect(p.toleranceNum, isNull);
    });

    test('Project 容差两列同时存在时 hasTolerance 为 true', () {
      final p = Project.fromJson(const {
        'id': 1,
        'title': 'x',
        'toleranceNum': 1,
        'toleranceDen': 100,
      });
      expect(p.hasTolerance, isTrue);
    });

    test('ProjectList 分组解析，缺失字段返回空列表', () {
      final pl = ProjectList.fromJson(const {});
      expect(pl.owned, isEmpty);
      expect(pl.subscribed, isEmpty);
      expect(pl.isEmpty, isTrue);
    });

    test('ProjectList 正确区分 owned 与 subscribed', () {
      final pl = ProjectList.fromJson(const {
        'owned': [
          {'id': 1, 'title': 'mine', 'access': 'owner'}
        ],
        'subscribed': [
          {'id': 2, 'title': 'theirs', 'access': 'subscriber'}
        ],
      });
      expect(pl.owned.single.id, 1);
      expect(pl.owned.single.isOwner, isTrue);
      expect(pl.subscribed.single.isOwner, isFalse);
    });

    test('ScoringConfig 解析清洗表', () {
      final sc = ScoringConfig.fromJson(const {
        'version': 1,
        'cleanupTable': {'０': '0', ',': ''},
      });
      expect(sc.version, 1);
      expect(sc.cleanupTable['０'], '0');
      expect(sc.cleanupTable[','], '');
    });

    test('ScoringConfig 清洗表缺省为空表而非 null', () {
      final sc = ScoringConfig.fromJson(const {'version': 2});
      expect(sc.cleanupTable, isEmpty);
      expect(sc.version, 2);
    });

    test('Question 解析信封', () {
      final q = Question.fromJson(const {
        'idx': 0,
        'q': '1+1 = ?',
        'a': '2',
        'envelope': {'kind': 'rational', 'num': '2', 'den': '1'},
      });
      expect(q.index, 0);
      expect(q.envelope, isNotNull);
      expect(q.envelope!.num, '2');
    });

    test('Question 遇到未知信封种类时不抛异常，信封为 null', () {
      final q = Question.fromJson(const {
        'idx': 0,
        'q': 'x',
        'a': 'y',
        'envelope': {'kind': 'something-new'},
      });
      expect(q.envelope, isNull,
          reason: '未知种类不应让整个响应解析失败，否则无法开始练习');
    });

    test('CompleteRoundResult 派生指标', () {
      final r = CompleteRoundResult.fromJson(const {
        'roundId': 5,
        'totalMs': 30000,
        'questionCount': 10,
        'correctCount': 8,
        'discrepancies': 0,
        'staleProject': false,
      });
      expect(r.accuracy, closeTo(0.8, 1e-9));
      expect(r.avgMsPerQuestion, 3000);
    });

    test('CompleteRoundResult 题数为 0 时不除零', () {
      final r = CompleteRoundResult.fromJson(const {'questionCount': 0});
      expect(r.accuracy, 0);
      expect(r.avgMsPerQuestion, 0);
    });

    test('StoredAttempt 解析（错题页数据源）', () {
      final a = StoredAttempt.fromJson(const {
        'idx': 2,
        'qSnapshot': '3+4 = ?',
        'aSnapshot': '7',
        'envelopeJson': '{"kind":"rational","num":"7","den":"1"}',
        'userInput': '8',
        'clientIsCorrect': true,
        'serverIsCorrect': false,
        'elapsedMs': 1500,
      });
      expect(a.index, 2);
      expect(a.serverIsCorrect, isFalse);
      // 两个判定各自独立：这正是 D16 的客户端侧体现
      expect(a.clientIsCorrect, isTrue);
    });
  });
}
