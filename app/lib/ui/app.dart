import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../state/session.dart';
import 'auth_page.dart';
import 'shell.dart';

/// 应用根。
///
/// UI 风格采用 Cupertino（D10）。本组件只做「登录态分流」：
/// 未登录显示登录页，已登录显示三 Tab 骨架。
class CalaApp extends ConsumerWidget {
  const CalaApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final session = ref.watch(sessionProvider);

    return CupertinoApp(
      title: 'Cala',
      debugShowCheckedModeBanner: false,
      theme: const CupertinoThemeData(
        brightness: Brightness.light,
        primaryColor: CupertinoColors.systemBlue,
      ),
      // 启动恢复登录态期间显示空白页，避免先闪一下登录页再跳走
      home: !session.initialized
          ? const CupertinoPageScaffold(
              child: Center(child: CupertinoActivityIndicator()),
            )
          : session.isLoggedIn
              ? const AppShell()
              : const AuthPage(),
    );
  }
}
