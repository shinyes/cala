import 'package:cala/api/subscription_api.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('ImportResultItem 解析', () {
    test('成功新增', () {
      final item = ImportResultItem.fromJson(const {
        'link': 'cala://subscribe?h=localhost&t=abc',
        'ok': true,
        'projectId': 7,
        'title': '口算',
      });

      expect(item.ok, isTrue);
      expect(item.projectId, 7);
      expect(item.title, '口算');
      expect(item.alreadySubscribed, isFalse);
      expect(item.status, ImportStatus.added);
    });

    test('已在订阅列表中', () {
      final item = ImportResultItem.fromJson(const {
        'link': 'cala://subscribe?h=localhost&t=abc',
        'ok': true,
        'alreadySubscribed': true,
        'title': '口算',
      });

      expect(item.status, ImportStatus.alreadySubscribed);
    });

    test('失败带可读原因', () {
      final item = ImportResultItem.fromJson(const {
        'link': 'cala://subscribe?t=bogus',
        'ok': false,
        'error': '链接无效或已被撤销',
      });

      expect(item.ok, isFalse);
      expect(item.error, '链接无效或已被撤销');
      expect(item.status, ImportStatus.failed);
    });

    test('跨实例失败的文案被保留', () {
      final item = ImportResultItem.fromJson(const {
        'link': 'cala://subscribe?h=other.host&t=abc',
        'ok': false,
        'error': '链接属于其他服务器（链接指向 other.host，当前为 example.com）',
      });

      expect(item.error, contains('其他服务器'));
      expect(item.error, contains('other.host'));
    });

    test('缺字段时给默认值而非抛异常', () {
      final item = ImportResultItem.fromJson(const {});
      expect(item.link, '');
      expect(item.ok, isFalse);
      expect(item.status, ImportStatus.failed);
    });
  });

  group('ImportOutcome 解析', () {
    test('混合结果逐条保留', () {
      final outcome = ImportOutcome.fromJson(const {
        'results': [
          {'link': 'a', 'ok': true, 'title': 'A'},
          {'link': 'b', 'ok': false, 'error': '链接无效或已被撤销'},
          {'link': 'c', 'ok': false, 'error': '链接属于其他服务器'},
        ],
        'succeeded': 1,
        'total': 3,
      });

      expect(outcome.results.length, 3);
      expect(outcome.succeeded, 1);
      expect(outcome.total, 3);
      // 关键：部分失败不应让其余结果丢失
      expect(outcome.results[0].status, ImportStatus.added);
      expect(outcome.results[1].status, ImportStatus.failed);
      expect(outcome.results[2].status, ImportStatus.failed);
    });

    test('results 缺省为空列表而非 null', () {
      final outcome = ImportOutcome.fromJson(const {});
      expect(outcome.results, isNotNull);
      expect(outcome.results, isEmpty);
      expect(outcome.succeeded, 0);
      expect(outcome.total, 0);
    });

    test('全部失败', () {
      final outcome = ImportOutcome.fromJson(const {
        'results': [
          {'link': 'a', 'ok': false, 'error': 'x'},
        ],
        'succeeded': 0,
        'total': 1,
      });
      expect(outcome.succeeded, 0);
      expect(outcome.results.single.status, ImportStatus.failed);
    });
  });

  group('ShareLink 解析', () {
    test('解析 token 与链接', () {
      final link = ShareLink.fromJson(const {
        'shareToken': 'tok123',
        'link': 'cala://subscribe?h=localhost:8080&t=tok123',
      });

      expect(link.token, 'tok123');
      expect(link.isEmpty, isFalse);
      expect(link.link, contains('cala://subscribe'));
      expect(link.link, contains('t=tok123'));
    });

    test('缺字段时 isEmpty 为 true', () {
      final link = ShareLink.fromJson(const {});
      expect(link.token, '');
      expect(link.link, '');
      expect(link.isEmpty, isTrue);
    });
  });
}
