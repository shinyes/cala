import 'package:flutter/cupertino.dart';

void main() {
  runApp(const CalaApp());
}

/// Cala 应用根。
///
/// UI 风格采用 Cupertino（D10）。此处仅为工程初始化阶段的最小可运行骨架；
/// 底栏三 Tab（练习 / 统计 / 我的）的信息架构见设计规格 §4.3，由后续阶段实现。
class CalaApp extends StatelessWidget {
  const CalaApp({super.key});

  @override
  Widget build(BuildContext context) {
    return const CupertinoApp(
      title: 'Cala',
      debugShowCheckedModeBanner: false,
      theme: CupertinoThemeData(brightness: Brightness.light),
      home: _PlaceholderPage(),
    );
  }
}

class _PlaceholderPage extends StatelessWidget {
  const _PlaceholderPage();

  @override
  Widget build(BuildContext context) {
    return const CupertinoPageScaffold(
      navigationBar: CupertinoNavigationBar(middle: Text('Cala')),
      child: Center(
        child: Padding(
          padding: EdgeInsets.all(24),
          child: Text(
            '速算练习',
            textAlign: TextAlign.center,
            style: TextStyle(fontSize: 20),
          ),
        ),
      ),
    );
  }
}
