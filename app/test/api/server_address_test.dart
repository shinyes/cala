import 'package:cala/api/server_address.dart';
import 'package:flutter_test/flutter_test.dart';

/// 服务端地址规范化的边界用例。
///
/// 这些用例对应真实会发生的手误：用户从聊天工具、浏览器地址栏复制地址，
/// 或者直接手打 IP 与端口。规范化必须把它们都变成同一个可用地址，
/// 否则用户会拿到难以理解的连接失败。
void main() {
  group('规范化：接受常见写法', () {
    final cases = <String, String>{
      // 最常见：手打 IP:端口，没写 scheme
      '192.168.1.5:8080': 'http://192.168.1.5:8080',
      // 写全了
      'http://192.168.1.5:8080': 'http://192.168.1.5:8080',
      // 末尾斜杠（Dio 拼接时会变成 //api/...）
      'http://192.168.1.5:8080/': 'http://192.168.1.5:8080',
      // 多个末尾斜杠
      'http://192.168.1.5:8080///': 'http://192.168.1.5:8080',
      // 首尾空白
      '  http://192.168.1.5:8080  ': 'http://192.168.1.5:8080',
      // 夹在中间的空白（粘贴时常见，只 trim 两端会漏掉）
      'http://192.168.1.5 : 8080': 'http://192.168.1.5:8080',
      // 换行与制表符
      'http://192.168.1.5:8080\n': 'http://192.168.1.5:8080',
      '\thttp://192.168.1.5:8080\t': 'http://192.168.1.5:8080',
      // scheme 大小写
      'HTTP://192.168.1.5:8080': 'http://192.168.1.5:8080',
      // https
      'https://cala.example.com': 'https://cala.example.com',
      // https 带端口
      'https://cala.example.com:8443': 'https://cala.example.com:8443',
      // 主机名
      'localhost:8080': 'http://localhost:8080',
      // 域名，无端口
      'example.com': 'http://example.com',
      // IPv6
      'http://[::1]:8080': 'http://[::1]:8080',
      '::1': 'http://[::1]',
    };

    cases.forEach((input, expected) {
      test('"$input" -> "$expected"', () {
        expect(normalizeServerAddress(input), expected);
      });
    });
  });

  group('规范化：拒绝并给出可读消息', () {
    test('空字符串', () {
      expect(
        () => normalizeServerAddress(''),
        throwsA(isA<ServerAddressError>()),
      );
      expect(
        () => normalizeServerAddress('   '),
        throwsA(isA<ServerAddressError>()),
      );
    });

    test('不支持 scheme 时说明当前是什么', () {
      expect(
        () => normalizeServerAddress('ftp://192.168.1.5'),
        throwsA(
          isA<ServerAddressError>().having(
            (e) => e.message,
            'message',
            contains('ftp'),
          ),
        ),
      );
    });

    // 带路径的地址必须被拒绝，而不是拼出错误 URL 后报一个难懂的 404。
    // 客户端的请求路径以 / 开头，Dio 会用它替换掉 baseUrl 的路径部分，
    // 因此 http://host/api 不可能按用户预期工作。
    test('拒绝带路径的地址并说明原因', () {
      expect(
        () => normalizeServerAddress('http://192.168.1.5:8080/api'),
        throwsA(
          isA<ServerAddressError>().having(
            (e) => e.message,
            'message',
            contains('路径'),
          ),
        ),
      );
    });

    test('拒绝 query 与 fragment', () {
      expect(
        () => normalizeServerAddress('http://192.168.1.5:8080?a=1'),
        throwsA(isA<ServerAddressError>()),
      );
      expect(
        () => normalizeServerAddress('http://192.168.1.5:8080#x'),
        throwsA(isA<ServerAddressError>()),
      );
    });

    test('缺少主机', () {
      expect(
        () => normalizeServerAddress('http://'),
        throwsA(isA<ServerAddressError>()),
      );
    });

    test('错误消息面向用户且非空', () {
      for (final bad in ['', 'ftp://h', 'http://h/p', 'http://']) {
        try {
          normalizeServerAddress(bad);
          fail('应当抛出 ServerAddressError: "$bad"');
        } on ServerAddressError catch (e) {
          expect(e.message.trim(), isNotEmpty);
          // toString 直接用于展示，必须与 message 一致而不是 "Instance of ..."
          expect(e.toString(), e.message);
        }
      }
    });
  });

  group('等价比较', () {
    test('不同写法指向同一服务器时相等', () {
      expect(isSameServerAddress('192.168.1.5:8080', 'http://192.168.1.5:8080/'),
          isTrue);
      expect(isSameServerAddress('HTTP://h:8080', 'http://h:8080'), isTrue);
    });

    test('不同端口或主机不相等', () {
      expect(isSameServerAddress('http://h:8080', 'http://h:9090'), isFalse);
      expect(isSameServerAddress('http://a:8080', 'http://b:8080'), isFalse);
      expect(isSameServerAddress('http://h:8080', 'https://h:8080'), isFalse);
    });

    test('非法地址不相等（不抛异常）', () {
      expect(isSameServerAddress('', 'http://h:8080'), isFalse);
      expect(isSameServerAddress('ftp://h', 'http://h'), isFalse);
    });
  });

  test('幂等：规范化结果再规范化不变', () {
    for (final input in [
      '192.168.1.5:8080',
      'http://h:8080/',
      '  HTTP://h:8080  ',
      'https://cala.example.com',
      'http://[::1]:8080',
    ]) {
      final once = normalizeServerAddress(input);
      expect(normalizeServerAddress(once), once, reason: '输入 "$input"');
    }
  });

  test('默认地址本身是合法的', () {
    expect(normalizeServerAddress(defaultServerAddress), defaultServerAddress);
  });
}
