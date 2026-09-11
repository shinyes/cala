/// 服务端地址的规范化与校验。
///
/// 本文件**不依赖 Flutter**，只做纯字符串/URI 处理，因此可以在普通单元测试里
/// 覆盖全部边界（与 `scoring.dart` 的组织方式一致）。
///
/// 为什么要规范化而不是直接用用户输入：
/// Dio 把 `baseUrl` 与请求路径直接拼接，因此
///   - 末尾多一个 `/` 会拼出 `//api/healthz`，部分服务端不会路由到该路径；
///   - 缺少 scheme 的 `192.168.1.5:8080` 完全无法解析；
///   - 粘贴时带进的空白（含不可见字符）会让解析失败。
/// 这些都是用户最容易犯的错，且报错信息通常难以理解，因此在入口处一次处理掉。
library;

/// 规范化失败时抛出。`message` 可直接展示给用户。
class ServerAddressError implements Exception {
  final String message;

  const ServerAddressError(this.message);

  @override
  String toString() => message;
}

/// 默认地址。
///
/// 用于桌面/浏览器调试（后端在本机）。**在真机上它没有意义** ——
/// 127.0.0.1 指向手机自身，因此手机端必须在设置里填写实际地址。
const String defaultServerAddress = 'http://127.0.0.1:8080';

/// 把用户输入规范化成可用的 baseUrl。
///
/// 成功返回形如 `http://192.168.1.5:8080`（无末尾斜杠）；
/// 失败抛 [ServerAddressError]，其 message 面向用户。
String normalizeServerAddress(String raw) {
  // 1. 删除**所有**空白字符，而不只是首尾。
  //
  // URL 本身不允许空白，因此整体删除是安全的；而粘贴来源（聊天工具、
  // 浏览器地址栏）常常带上换行、普通空格，甚至不可见字符。
  // 只 trim 两端会漏掉夹在中间的这类字符，而它们造成的失败极难排查。
  // （同样的教训在 CI 的 keystore 别名处理上出现过一次。）
  var s = raw.replaceAll(RegExp(r'\s'), '');

  if (s.isEmpty) {
    throw const ServerAddressError('请填写服务器地址');
  }

  // 2. 没有 scheme 就补 http://。
  //    用户最常直接输入 `192.168.1.5:8080`，补全后即可用，
  //    比要求他们记住写 `http://` 友好得多。
  if (!s.contains('://')) {
    // 未加方括号的裸 IPv6（如 `::1`、`fe80::1`）需要补上括号，
    // 否则 Uri 无法解析。判据：含 2 个以上冒号 ——
    // `host:port` 最多只有 1 个冒号，因此不会误判。
    // 已带方括号的写法（`[::1]:8080`）不满足该条件，无需处理。
    final colons = ':'.allMatches(s).length;
    if (colons >= 2 && !s.startsWith('[')) {
      s = '[$s]';
    }
    s = 'http://$s';
  }

  final uri = Uri.tryParse(s);
  if (uri == null) {
    throw const ServerAddressError('地址格式不正确');
  }

  if (uri.scheme != 'http' && uri.scheme != 'https') {
    throw ServerAddressError(
      '只支持 http 与 https，当前是「${uri.scheme}」',
    );
  }

  if (uri.host.isEmpty) {
    throw const ServerAddressError('地址缺少主机名或 IP');
  }

  // 3. 拒绝带路径的地址。
  //
  // 客户端的请求路径都以 `/` 开头，Dio 会用它替换 baseUrl 的路径部分，
  // 因此 `http://host/api` 这种写法不可能按用户预期工作
  //（后端固定挂在 `/api` 下）。与其让它拼出错误地址后报一个难懂的 404，
  // 不如在这里直接说清楚。
  //
  // 判据是「路径里除斜杠外还有内容」，而不是 uri.path 非空：
  // 末尾多打几个斜杠（`http://host:8080///`）显然只是手滑，
  // 不是想指定路径，应当被容忍而不是报错。
  if (uri.path.replaceAll('/', '').isNotEmpty) {
    throw const ServerAddressError('只需填到端口即可，不要带路径');
  }

  if (uri.hasQuery || uri.hasFragment) {
    throw const ServerAddressError('地址不应包含 ? 或 #');
  }

  // 4. 重建为规范形式。
  //
  // 不复用原字符串：那样会保留末尾斜杠、大小写 scheme 等差异，
  // 导致「同一个地址」的字符串比较不等（本文件的比较语义依赖它）。
  final buffer = StringBuffer('${uri.scheme}://');
  // IPv6 主机在 Uri 中是去括号的，重建时必须补回，否则拼出非法地址。
  if (uri.host.contains(':')) {
    buffer.write('[${uri.host}]');
  } else {
    buffer.write(uri.host);
  }
  if (uri.hasPort) {
    buffer.write(':${uri.port}');
  }
  return buffer.toString();
}

/// 两个地址是否指向同一服务端。任一方非法时返回 false。
bool isSameServerAddress(String a, String b) {
  try {
    return normalizeServerAddress(a) == normalizeServerAddress(b);
  } on ServerAddressError {
    return false;
  }
}
