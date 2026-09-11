import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/models.dart';
import '../api/stats_api.dart';
import '../state/projects.dart';
import '../state/stats.dart';
import 'widgets/line_chart.dart';

/// 统计 Tab（功能 4）。
///
/// **本页只读**：不提供任何编辑入口（规格 §4.3）。
///
/// 信息架构（规格 §8.2）：8 个指标 + 3 种粒度 = 24 个数字，必须拆维度呈现，
/// 任一时刻屏幕上只有「一条线 + 8 个数字」：
///
/// ```
/// 项目选择器
/// 粒度 [日][周][月]      指标 [耗时][正确率]
/// 折线图（单条线，绝不双 Y 轴）
/// 耗时指标卡          正确率指标卡
/// ```
class StatsTab extends ConsumerStatefulWidget {
  const StatsTab({super.key});

  @override
  ConsumerState<StatsTab> createState() => _StatsTabState();
}

/// 折线图当前展示的指标。
enum _Metric { time, accuracy }

class _StatsTabState extends ConsumerState<StatsTab> {
  _Metric _metric = _Metric.time;

  @override
  Widget build(BuildContext context) {
    final stats = ref.watch(statsProvider);
    final projects = ref.watch(projectsProvider);

    final all = [...projects.owned, ...projects.subscribed];

    return CupertinoPageScaffold(
      navigationBar: CupertinoNavigationBar(
        middle: const Text('统计'),
        trailing: CupertinoButton(
          padding: EdgeInsets.zero,
          onPressed: stats.projectId == null
              ? null
              : () => ref.read(statsProvider.notifier).refresh(),
          child: const Icon(CupertinoIcons.refresh),
        ),
      ),
      child: SafeArea(
        child: all.isEmpty
            ? const _Empty(message: '还没有可统计的项目')
            : ListView(
                children: [
                  _ProjectPicker(
                    projects: all,
                    selectedId: stats.projectId,
                    onSelect: (id) =>
                        ref.read(statsProvider.notifier).select(id),
                  ),
                  if (stats.projectId == null)
                    const _Empty(message: '选择上方项目以查看统计')
                  else ...[
                    _Controls(
                      grain: stats.grain,
                      metric: _metric,
                      onGrain: (g) =>
                          ref.read(statsProvider.notifier).setGrain(g),
                      onMetric: (m) => setState(() => _metric = m),
                    ),
                    if (stats.error != null)
                      _ErrorBanner(
                        message: stats.error!,
                        onRetry: () =>
                            ref.read(statsProvider.notifier).refresh(),
                      ),
                    _ChartArea(
                      result: stats.result,
                      metric: _metric,
                      loading: stats.loading,
                    ),
                    _MetricsCards(result: stats.result),
                    const SizedBox(height: 32),
                  ],
                ],
              ),
      ),
    );
  }
}

class _Empty extends StatelessWidget {
  const _Empty({required this.message});
  final String message;

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.all(48),
        child: Column(
          children: [
            const Icon(CupertinoIcons.chart_bar_square,
                size: 44, color: CupertinoColors.systemGrey3),
            const SizedBox(height: 14),
            Text(
              message,
              textAlign: TextAlign.center,
              style: const TextStyle(color: CupertinoColors.secondaryLabel),
            ),
          ],
        ),
      );
}

class _ErrorBanner extends StatelessWidget {
  const _ErrorBanner({required this.message, required this.onRetry});
  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) => Container(
        margin: const EdgeInsets.fromLTRB(16, 4, 16, 4),
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: CupertinoColors.systemOrange.withValues(alpha: 0.12),
          borderRadius: BorderRadius.circular(10),
        ),
        child: Row(
          children: [
            const Icon(CupertinoIcons.exclamationmark_triangle,
                size: 16, color: CupertinoColors.systemOrange),
            const SizedBox(width: 8),
            Expanded(
              child: Text(message,
                  style: const TextStyle(fontSize: 13)),
            ),
            CupertinoButton(
              padding: EdgeInsets.zero,
              onPressed: onRetry,
              child: const Text('重试', style: TextStyle(fontSize: 13)),
            ),
          ],
        ),
      );
}

/// 项目选择器：横向滚动的胶囊列表。
class _ProjectPicker extends StatelessWidget {
  const _ProjectPicker({
    required this.projects,
    required this.selectedId,
    required this.onSelect,
  });

  final List<Project> projects;
  final int? selectedId;
  final ValueChanged<int> onSelect;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: 52,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
        itemCount: projects.length,
        separatorBuilder: (_, _) => const SizedBox(width: 8),
        itemBuilder: (_, i) {
          final p = projects[i];
          final selected = p.id == selectedId;
          return CupertinoButton(
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 6),
            borderRadius: BorderRadius.circular(16),
            color: selected
                ? CupertinoColors.systemBlue
                : CupertinoColors.tertiarySystemFill.resolveFrom(context),
            onPressed: () => onSelect(p.id),
            child: Text(
              p.title,
              style: TextStyle(
                fontSize: 14,
                color: selected
                    ? CupertinoColors.white
                    : CupertinoColors.label.resolveFrom(context),
              ),
            ),
          );
        },
      ),
    );
  }
}

/// 粒度与指标两个分段控件。
///
/// 两个控件各自只有一个激活值，避免 8 指标 × 3 粒度同时呈现（规格 §8.2）。
class _Controls extends StatelessWidget {
  const _Controls({
    required this.grain,
    required this.metric,
    required this.onGrain,
    required this.onMetric,
  });

  final StatsGrain grain;
  final _Metric metric;
  final ValueChanged<StatsGrain> onGrain;
  final ValueChanged<_Metric> onMetric;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 4, 16, 8),
      child: Column(
        children: [
          CupertinoSlidingSegmentedControl<StatsGrain>(
            groupValue: grain,
            children: {
              for (final g in StatsGrain.values)
                g: Padding(
                  padding: const EdgeInsets.symmetric(vertical: 6),
                  child: Text(g.label),
                ),
            },
            onValueChanged: (v) {
              if (v != null) onGrain(v);
            },
          ),
          const SizedBox(height: 8),
          CupertinoSlidingSegmentedControl<_Metric>(
            groupValue: metric,
            children: const {
              _Metric.time: Padding(
                padding: EdgeInsets.symmetric(vertical: 6),
                child: Text('耗时'),
              ),
              _Metric.accuracy: Padding(
                padding: EdgeInsets.symmetric(vertical: 6),
                child: Text('正确率'),
              ),
            },
            onValueChanged: (v) {
              if (v != null) onMetric(v);
            },
          ),
        ],
      ),
    );
  }
}

class _ChartArea extends StatelessWidget {
  const _ChartArea({
    required this.result,
    required this.metric,
    required this.loading,
  });

  final StatsResult? result;
  final _Metric metric;
  final bool loading;

  @override
  Widget build(BuildContext context) {
    if (loading && result == null) {
      return const SizedBox(
        height: 180,
        child: Center(child: CupertinoActivityIndicator()),
      );
    }

    final res = result;
    if (res == null || res.buckets.isEmpty) {
      return const SizedBox(
        height: 180,
        child: Center(
          child: Text('还没有练习记录',
              style: TextStyle(color: CupertinoColors.tertiaryLabel)),
        ),
      );
    }

    final isTime = metric == _Metric.time;
    final points = res.buckets
        .map((b) => ChartPoint(
              b.label,
              isTime
                  ? b.metrics.timeAvgMs
                  : b.metrics.accuracyAvg * 100,
            ))
        .toList(growable: false);

    return Padding(
      padding: const EdgeInsets.fromLTRB(8, 4, 8, 4),
      child: LineChart(
        points: points,
        color: isTime
            ? CupertinoColors.systemBlue
            : CupertinoColors.systemGreen,
        valueFormatter: isTime
            ? (v) => '${(v / 1000).toStringAsFixed(v >= 10000 ? 0 : 1)}s'
            : (v) => '${v.toStringAsFixed(0)}%',
      ),
    );
  }
}

/// 两张指标卡，各 4 行。
///
/// 数字取 `overall`——即所选范围内**所有轮次**的聚合，
/// 与粒度无关：粒度只决定折线图怎么分桶。
class _MetricsCards extends StatelessWidget {
  const _MetricsCards({required this.result});
  final StatsResult? result;

  @override
  Widget build(BuildContext context) {
    final m = result?.overall;
    if (m == null || m.isEmpty) {
      return const SizedBox.shrink();
    }

    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: _MetricCard(
              title: '耗时',
              rows: [
                ('最高', _fmtMs(m.timeMaxMs.toDouble())),
                ('最低', _fmtMs(m.timeMinMs.toDouble())),
                ('平均', _fmtMs(m.timeAvgMs)),
                ('中位', _fmtMs(m.timeMedianMs)),
              ],
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: _MetricCard(
              title: '正确率',
              rows: [
                ('最高', _fmtPct(m.accuracyMax)),
                ('最低', _fmtPct(m.accuracyMin)),
                ('平均', _fmtPct(m.accuracyAvg)),
                ('中位', _fmtPct(m.accuracyMedian)),
              ],
            ),
          ),
        ],
      ),
    );
  }

  static String _fmtMs(double ms) {
    if (ms < 1000) return '${ms.round()}ms';
    return '${(ms / 1000).toStringAsFixed(1)}s';
  }

  static String _fmtPct(double v) => '${(v * 100).toStringAsFixed(1)}%';
}

class _MetricCard extends StatelessWidget {
  const _MetricCard({required this.title, required this.rows});
  final String title;
  final List<(String, String)> rows;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: CupertinoColors.tertiarySystemFill.resolveFrom(context),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: const TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w600,
                color: CupertinoColors.secondaryLabel),
          ),
          const SizedBox(height: 10),
          for (final (label, value) in rows)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 3),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text(label,
                      style: const TextStyle(
                          fontSize: 13, color: CupertinoColors.secondaryLabel)),
                  Text(
                    value,
                    style: const TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.w500,
                      fontFeatures: [FontFeature.tabularFigures()],
                    ),
                  ),
                ],
              ),
            ),
        ],
      ),
    );
  }
}
