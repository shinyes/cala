import 'package:flutter/cupertino.dart';

/// 统计 Tab（功能 4 的界面位置）。
///
/// **本阶段仅为占位**：图表与 8 项指标在 P5 交付。
///
/// 之所以现在就建这个文件而不是等 P5：规格 §4.3 要求三 Tab 的职责互不重叠，
/// 其中「统计 Tab 不含任何编辑入口」是一条结构性约束。先把它立起来，
/// 后续填充时就不会顺手把编辑功能塞进来。
class StatsTab extends StatelessWidget {
  const StatsTab({super.key});

  @override
  Widget build(BuildContext context) {
    return const CupertinoPageScaffold(
      navigationBar: CupertinoNavigationBar(middle: Text('统计')),
      child: SafeArea(
        child: Center(
          child: Padding(
            padding: EdgeInsets.all(32),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Icon(CupertinoIcons.chart_bar_square,
                    size: 48, color: CupertinoColors.systemGrey3),
                SizedBox(height: 16),
                Text('统计', style: TextStyle(fontSize: 20)),
                SizedBox(height: 8),
                Text(
                  '按项目的日 / 周 / 月折线，以及耗时与正确率的\n'
                  '最高 / 最低 / 平均 / 中位，将在后续版本提供。',
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    fontSize: 13,
                    color: CupertinoColors.secondaryLabel,
                    height: 1.5,
                  ),
                ),
                SizedBox(height: 20),
                Text(
                  '本页只读：不提供任何编辑入口。',
                  style: TextStyle(
                      fontSize: 12, color: CupertinoColors.tertiaryLabel),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
