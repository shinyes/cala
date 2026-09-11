import 'client.dart';

/// 时间粒度。取值与后端 `stats.Grain` 一致。
enum StatsGrain {
  day('day', '日'),
  week('week', '周'),
  month('month', '月');

  const StatsGrain(this.wire, this.label);

  /// 传给后端的值。
  final String wire;

  /// 界面显示用的中文标签。
  final String label;
}

/// 一组轮次的 8 项指标。字段名与后端 `stats.Metrics` 一致。
class StatsMetrics {
  final int roundCount;

  final int timeMaxMs;
  final int timeMinMs;
  final double timeAvgMs;
  final double timeMedianMs;

  final double accuracyMax;
  final double accuracyMin;
  final double accuracyAvg;
  final double accuracyMedian;

  const StatsMetrics({
    required this.roundCount,
    required this.timeMaxMs,
    required this.timeMinMs,
    required this.timeAvgMs,
    required this.timeMedianMs,
    required this.accuracyMax,
    required this.accuracyMin,
    required this.accuracyAvg,
    required this.accuracyMedian,
  });

  static const empty = StatsMetrics(
    roundCount: 0,
    timeMaxMs: 0,
    timeMinMs: 0,
    timeAvgMs: 0,
    timeMedianMs: 0,
    accuracyMax: 0,
    accuracyMin: 0,
    accuracyAvg: 0,
    accuracyMedian: 0,
  );

  factory StatsMetrics.fromJson(Map<String, dynamic> j) => StatsMetrics(
        roundCount: (j['roundCount'] as num?)?.toInt() ?? 0,
        timeMaxMs: (j['timeMaxMs'] as num?)?.toInt() ?? 0,
        timeMinMs: (j['timeMinMs'] as num?)?.toInt() ?? 0,
        timeAvgMs: (j['timeAvgMs'] as num?)?.toDouble() ?? 0,
        timeMedianMs: (j['timeMedianMs'] as num?)?.toDouble() ?? 0,
        accuracyMax: (j['accuracyMax'] as num?)?.toDouble() ?? 0,
        accuracyMin: (j['accuracyMin'] as num?)?.toDouble() ?? 0,
        accuracyAvg: (j['accuracyAvg'] as num?)?.toDouble() ?? 0,
        accuracyMedian: (j['accuracyMedian'] as num?)?.toDouble() ?? 0,
      );

  bool get isEmpty => roundCount == 0;
}

/// 一个时间桶。
class StatsBucket {
  final String key;
  final String label;
  final StatsMetrics metrics;

  const StatsBucket({
    required this.key,
    required this.label,
    required this.metrics,
  });

  factory StatsBucket.fromJson(Map<String, dynamic> j) => StatsBucket(
        key: j['key'] as String? ?? '',
        label: j['label'] as String? ?? '',
        metrics: StatsMetrics.fromJson(
            (j['metrics'] as Map<String, dynamic>?) ?? const {}),
      );
}

/// 一次统计查询的结果。
class StatsResult {
  final String grain;

  /// 整个范围内的 8 项指标（总结卡片使用）。
  final StatsMetrics overall;

  /// 按时间排序的桶（折线图使用）。
  final List<StatsBucket> buckets;

  const StatsResult({
    required this.grain,
    required this.overall,
    required this.buckets,
  });

  static const empty = StatsResult(
    grain: 'day',
    overall: StatsMetrics.empty,
    buckets: [],
  );

  factory StatsResult.fromJson(Map<String, dynamic> j) {
    final raw = j['buckets'];
    return StatsResult(
      grain: j['grain'] as String? ?? 'day',
      overall: StatsMetrics.fromJson(
          (j['overall'] as Map<String, dynamic>?) ?? const {}),
      buckets: raw is List
          ? raw
              .whereType<Map<String, dynamic>>()
              .map(StatsBucket.fromJson)
              .toList(growable: false)
          : const [],
    );
  }
}

/// 统计端点。
class StatsApi {
  final ApiClient _c;
  StatsApi(this._c);

  /// 取某项目的统计。
  ///
  /// [tzOffsetMinutes] 必须是调用方的本地时区偏移：分桶按用户的自然日/月进行，
  /// 若按 UTC 分桶，当地 00:30 的练习会被算到前一天。
  Future<StatsResult> forProject(
    int projectId, {
    required StatsGrain grain,
    required int tzOffsetMinutes,
  }) async {
    final j = await _c.get(
      '/api/projects/$projectId/stats?grain=${grain.wire}'
      '&tzOffsetMinutes=$tzOffsetMinutes',
    );
    return StatsResult.fromJson(j);
  }
}
