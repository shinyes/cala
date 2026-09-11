import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/client.dart';
import '../state/session.dart';
import 'practice_tab.dart';
import 'profile_tab.dart';
import 'stats_tab.dart';

/// 底栏三 Tab 骨架（规格 §4.3）。
///
/// 职责边界（刻意不重叠，避免「同一件事两个入口」）：
///   - **练习**：做练习 + 管理项目。项目列表、新建项目、导入订阅链接。
///     点击项目直接进入练习。
///   - **统计**：只看数字。项目选择器 + 图表与指标卡。**不含任何编辑入口**。
///   - **我的**：账号与关系。用户信息、我创建的项目（管理/分享/删除）、
///     我订阅的项目（退订）、管理员可见的注册开关、退出登录。
class AppShell extends ConsumerStatefulWidget {
  const AppShell({super.key});

  @override
  ConsumerState<AppShell> createState() => _AppShellState();
}

class _AppShellState extends ConsumerState<AppShell> {
  int _tab = 0;

  @override
  Widget build(BuildContext context) {
    // 令牌中途失效（例如被管理员禁用）时退回登录页
    ref.listen(sessionProvider, (prev, next) {
      if (prev != null && prev.isLoggedIn && !next.isLoggedIn) {
        // 无需手动跳转：CalaApp 依据 isLoggedIn 切换根页面
      }
    });

    return CupertinoTabScaffold(
      tabBar: CupertinoTabBar(
        currentIndex: _tab,
        onTap: (i) => setState(() => _tab = i),
        items: const [
          BottomNavigationBarItem(
            icon: Icon(CupertinoIcons.pencil_outline),
            activeIcon: Icon(CupertinoIcons.pencil),
            label: '练习',
          ),
          BottomNavigationBarItem(
            icon: Icon(CupertinoIcons.chart_bar_alt_fill),
            label: '统计',
          ),
          BottomNavigationBarItem(
            icon: Icon(CupertinoIcons.person),
            activeIcon: Icon(CupertinoIcons.person_fill),
            label: '我的',
          ),
        ],
      ),
      tabBuilder: (context, index) {
        switch (index) {
          case 0:
            return const CupertinoTabView(builder: _practiceTabBuilder);
          case 1:
            return const CupertinoTabView(builder: _statsTabBuilder);
          default:
            return const CupertinoTabView(builder: _profileTabBuilder);
        }
      },
    );
  }
}

Widget _practiceTabBuilder(BuildContext context) => const PracticeTab();
Widget _statsTabBuilder(BuildContext context) => const StatsTab();
Widget _profileTabBuilder(BuildContext context) => const ProfileTab();

/// 把 ApiException 转成用户可读文案。
///
/// 后端已给出具体原因（例如规则错误指出第几题），此处**原样使用**，
/// 不替换成笼统的「操作失败」——那会让作者无从修正。
String describeError(Object error) {
  if (error is ApiException) return error.message;
  return '操作失败：$error';
}
