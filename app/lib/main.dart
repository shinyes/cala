import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'state/server_address.dart';
import 'state/session.dart';
import 'ui/app.dart';

/// 应用入口。
///
/// 在 `runApp` 之前读取已保存的服务端地址，并用它覆盖
/// [initialServerUrlProvider]，使地址在应用起来时就已确定。
///
/// 也可以把读取放进某个 Notifier 里异步完成，但那会引入一个真实竞态：
/// 登录态恢复会立刻请求服务端（拉公开配置），若地址尚未加载，
/// 该请求就会打到默认地址（真机上 127.0.0.1 指向手机自身，必然失败），
/// 并且失败会被错误地归因。SharedPreferences 的读取只有毫秒级，
/// 放在这里换来确定性，是划算的。
Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();

  final serverUrl = await loadSavedServerAddress();

  runApp(
    ProviderScope(
      overrides: [
        initialServerUrlProvider.overrideWithValue(serverUrl),
      ],
      child: const CalaApp(),
    ),
  );
}
