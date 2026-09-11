import 'package:flutter/cupertino.dart';

import '../api/models.dart';
import '../scoring/scoring.dart' as scoring;
import '../state/practice.dart';
import 'wrong_answers_page.dart';

/// 总结页（功能 7）：总耗时、平均每题耗时、正确率，点击错题数进入错题页。
///
/// 数字以 **服务端返回的权威值**为准（规格 §6.1(5)）。
/// 若客户端判定与服务端不一致（`discrepancies > 0`），如实提示而非静默采用其一——
/// 那正是跨端判分漂移的信号，用户与开发者都应当看见。
class SummaryPage extends StatelessWidget {
  const SummaryPage({
    super.key,
    required this.project,
    required this.result,
    required this.state,
    required this.cleanupTable,
    required this.tolerance,
    this.startNewRound,
  });

  final Project project;
  final CompleteRoundResult result;
  final PracticeState state;
  final Map<String, String> cleanupTable;
  final scoring.Tolerance tolerance;

  /// 「再来一轮」。重练错题进入本页时为 null。
  final Future<void> Function()? startNewRound;

  int get _wrongCount => result.questionCount - result.correctCount;

  @override
  Widget build(BuildContext context) {
    return CupertinoPageScaffold(
      navigationBar: CupertinoNavigationBar(
        middle: const Text('本轮总结'),
        // 不允许返回上一题：本轮已落库，回到答题状态没有意义
        automaticallyImplyLeading: false,
      ),
      child: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(20),
          children: [
            const SizedBox(height: 12),
            _AccuracyRing(accuracy: result.accuracy),
            const SizedBox(height: 28),

            if (result.staleProject)
              const _Notice(
                icon: CupertinoIcons.exclamationmark_triangle,
                color: CupertinoColors.systemOrange,
                text: '本项目在练习期间被作者修改，题目可能已与开始时不同。'
                    '本轮记录已按修改前的规则重放。',
              ),

            if (result.discrepancies > 0)
              _Notice(
                icon: CupertinoIcons.arrow_left_right_circle,
                color: CupertinoColors.systemOrange,
                text: '有 ${result.discrepancies} 题的本地判定与服务端不一致。'
                    '统计以服务端判定为准。',
              ),

            _MetricRow(
              items: [
                _Metric('总耗时', _fmtDuration(result.totalMs)),
                _Metric('平均每题', _fmtDuration(result.avgMsPerQuestion)),
              ],
            ),
            const SizedBox(height: 12),
            _MetricRow(
              items: [
                _Metric('正确', '${result.correctCount} / ${result.questionCount}'),
                _Metric('正确率',
                    '${(result.accuracy * 100).toStringAsFixed(1)}%'),
              ],
            ),

            const SizedBox(height: 24),
            CupertinoListTile(
              title: Text(_wrongCount == 0 ? '没有错题' : '错题 $_wrongCount 道'),
              subtitle: Text(_wrongCount == 0
                  ? '本轮全部答对'
                  : '查看错题、重练错题'),
              trailing: const CupertinoListTileChevron(),
              onTap: _wrongCount == 0
                  ? null
                  : () => Navigator.of(context).push(
                        CupertinoPageRoute<void>(
                          builder: (_) => WrongAnswersPage(
                            roundResult: result,
                            state: state,
                            cleanupTable: cleanupTable,
                            tolerance: tolerance,
                            project: project,
                          ),
                        ),
                      ),
            ),

            const SizedBox(height: 20),
            if (startNewRound != null)
              CupertinoButton.filled(
                onPressed: () async {
                  // 回到练习页并开新一轮（新种子）
                  Navigator.of(context).pop();
                  await startNewRound!();
                },
                child: const Text('再来一轮'),
              ),
            const SizedBox(height: 10),
            CupertinoButton(
              onPressed: () => Navigator.of(context).pop(),
              child: const Text('返回'),
            ),
          ],
        ),
      ),
    );
  }

  static String _fmtDuration(int ms) {
    final totalSeconds = ms / 1000;
    if (totalSeconds < 60) return '${totalSeconds.toStringAsFixed(1)} 秒';
    final m = totalSeconds ~/ 60;
    final s = (totalSeconds % 60).round();
    return '$m 分 $s 秒';
  }
}

class _AccuracyRing extends StatelessWidget {
  const _AccuracyRing({required this.accuracy});
  final double accuracy;

  @override
  Widget build(BuildContext context) {
    final pct = (accuracy * 100).round();
    final color = accuracy >= 0.8
        ? CupertinoColors.systemGreen
        : accuracy >= 0.5
            ? CupertinoColors.systemOrange
            : CupertinoColors.systemRed;

    return Column(
      children: [
        SizedBox(
          width: 150,
          height: 150,
          child: Stack(
            alignment: Alignment.center,
            children: [
              SizedBox(
                width: 150,
                height: 150,
                child: CupertinoActivityIndicator.partiallyRevealed(
                  radius: 62,
                  progress: accuracy,
                  color: color,
                ),
              ),
              Text(
                '$pct%',
                style: TextStyle(
                  fontSize: 34,
                  fontWeight: FontWeight.w600,
                  color: color,
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 8),
        const Text('正确率', style: TextStyle(color: CupertinoColors.secondaryLabel)),
      ],
    );
  }
}

class _Metric {
  const _Metric(this.label, this.value);
  final String label;
  final String value;
}

class _MetricRow extends StatelessWidget {
  const _MetricRow({required this.items});
  final List<_Metric> items;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        for (final m in items)
          Expanded(
            child: Container(
              margin: const EdgeInsets.symmetric(horizontal: 4),
              padding: const EdgeInsets.symmetric(vertical: 16),
              decoration: BoxDecoration(
                color: CupertinoColors.tertiarySystemFill.resolveFrom(context),
                borderRadius: BorderRadius.circular(12),
              ),
              child: Column(
                children: [
                  Text(
                    m.value,
                    style: const TextStyle(
                        fontSize: 20, fontWeight: FontWeight.w600),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    m.label,
                    style: const TextStyle(
                        fontSize: 13, color: CupertinoColors.secondaryLabel),
                  ),
                ],
              ),
            ),
          ),
      ],
    );
  }
}

class _Notice extends StatelessWidget {
  const _Notice({
    required this.icon,
    required this.color,
    required this.text,
  });

  final IconData icon;
  final Color color;
  final String text;

  @override
  Widget build(BuildContext context) => Container(
        margin: const EdgeInsets.only(bottom: 12),
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(10),
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Icon(icon, color: color, size: 18),
            const SizedBox(width: 8),
            Expanded(
              child: Text(text, style: TextStyle(color: color, fontSize: 13)),
            ),
          ],
        ),
      );
}
