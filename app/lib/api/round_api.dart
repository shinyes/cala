import 'client.dart';
import 'models.dart';

/// 轮次相关端点（规格 §7）。
class RoundApi {
  final ApiClient _c;
  RoundApi(this._c);

  /// 取一轮题目。返回的 [ScoringConfig] 含服务端下发的清洗表，
  /// 客户端判分必须使用它（规格 §5.5.4）。
  Future<StartRoundResult> start(int projectId) async {
    final j = await _c.post('/api/rounds/start', {'projectId': projectId});
    return StartRoundResult.fromJson(j);
  }

  /// 交卷。
  ///
  /// [clientCorrectness] 是客户端判定，仅用于即时反馈与分歧告警；
  /// 后端会独立重算权威值，并以服务端派生值返回汇总（规格 §7）。
  Future<CompleteRoundResult> complete({
    required int projectId,
    required int seed,
    required DateTime startedAt,
    required DateTime finishedAt,
    required List<AttemptInput> attempts,
  }) async {
    final j = await _c.post('/api/rounds/complete', {
      'projectId': projectId,
      'seed': seed,
      'startedAt': _rfc3339(startedAt),
      'finishedAt': _rfc3339(finishedAt),
      'attempts': attempts.map((a) => a.toJson()).toList(growable: false),
    });
    return CompleteRoundResult.fromJson(j);
  }

  /// 读取某轮已落库的答题记录（错题页使用）。
  Future<List<StoredAttempt>> attempts(int roundId) async {
    final j = await _c.get('/api/rounds/$roundId/attempts');
    final raw = j['attempts'];
    if (raw is! List) return const [];
    return raw
        .whereType<Map<String, dynamic>>()
        .map(StoredAttempt.fromJson)
        .toList(growable: false);
  }

  /// RFC3339 UTC。后端用该格式解析；本地时间会被拒绝。
  static String _rfc3339(DateTime t) =>
      '${t.toUtc().toIso8601String().split('.').first}Z';
}
