import 'package:cala/ui/server_address_page.dart';
import 'package:flutter_test/flutter_test.dart';

/// `/api/healthz` 响应到用户文案的转换。
///
/// 重点是那条**兼容分支**：旧版本服务端不返回 `version`，
/// 这时必须报「连接成功」而不是「响应不符合预期」——
/// 否则升级客户端后会误报一个并不存在的问题。
void main() {
  group('describeHealthResponse', () {
    test('带版本号时显示版本', () {
      expect(
        describeHealthResponse({'status': 'ok', 'version': '0.0.4'}),
        '连接成功，服务端正常（版本 0.0.4）',
      );
    });

    test('无版本号字段时仍报连接成功（兼容旧服务端）', () {
      final s = describeHealthResponse({'status': 'ok'});
      expect(s, contains('连接成功'));
      expect(s, contains('未提供版本号'));
      expect(s, isNot(contains('不符合预期')));
    });

    test('版本号为空串时按「未提供」处理', () {
      final s = describeHealthResponse({'status': 'ok', 'version': ''});
      expect(s, contains('未提供版本号'));
    });

    test('版本号类型异常时按「未提供」处理，不抛异常', () {
      // 服务端返回了非字符串（例如数字或 null）时不应崩溃
      for (final bad in <Object?>[null, 4, 0.4, true, <String>[]]) {
        final s = describeHealthResponse({'status': 'ok', 'version': bad});
        expect(s, contains('未提供版本号'), reason: 'version=$bad');
      }
    });

    test('status 非 ok 时如实报告，不谎称成功', () {
      final s = describeHealthResponse({'status': 'degraded'});
      expect(s, contains('不符合预期'));
      expect(s, isNot(contains('连接成功')));
    });

    test('status 缺失时如实报告', () {
      final s = describeHealthResponse({});
      expect(s, contains('不符合预期'));
    });

    test('非 ok 响应里带版本号也不能报成功', () {
      // 顺序错误会让「已连上但不是我们期望的服务」被误判为正常
      final s = describeHealthResponse({'status': 'weird', 'version': '0.0.4'});
      expect(s, isNot(contains('连接成功')));
    });
  });
}
