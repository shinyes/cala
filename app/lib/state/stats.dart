import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/stats_api.dart';
import 'session.dart';

/// 统计端点。
final statsApiProvider = Provider<StatsApi>((ref) {
  return StatsApi(ref.watch(apiClientProvider));
});

/// 统计页状态。
@immutable
class StatsState {
  /// 当前选中的项目 ID；null 表示尚未选择。
  final int? projectId;

  final StatsGrain grain;
  final StatsResult? result;
  final bool loading;
  final String? error;

  const StatsState({
    this.projectId,
    this.grain = StatsGrain.day,
    this.result,
    this.loading = false,
    this.error,
  });

  StatsState copyWith({
    int? projectId,
    StatsGrain? grain,
    StatsResult? result,
    bool? loading,
    String? error,
    bool clearError = false,
    bool clearResult = false,
  }) {
    return StatsState(
      projectId: projectId ?? this.projectId,
      grain: grain ?? this.grain,
      result: clearResult ? null : (result ?? this.result),
      loading: loading ?? this.loading,
      error: clearError ? null : (error ?? this.error),
    );
  }
}

/// 统计页状态与动作。
///
/// 刻意只做只读操作：统计 Tab 不提供任何编辑入口（规格 §4.3）。
class StatsNotifier extends Notifier<StatsState> {
  @override
  StatsState build() => const StatsState();

  StatsApi get _api => ref.read(statsApiProvider);

  /// 选择项目并加载。切换粒度时复用当前项目。
  Future<void> select(int projectId, {StatsGrain? grain}) async {
    final g = grain ?? state.grain;
    state = state.copyWith(projectId: projectId, grain: g, loading: true, clearError: true);
    await _load(projectId, g);
  }

  /// 切换粒度。
  Future<void> setGrain(StatsGrain grain) async {
    final id = state.projectId;
    if (id == null) {
      state = state.copyWith(grain: grain);
      return;
    }
    state = state.copyWith(grain: grain, loading: true, clearError: true);
    await _load(id, grain);
  }

  Future<void> refresh() async {
    final id = state.projectId;
    if (id == null) return;
    state = state.copyWith(loading: true, clearError: true);
    await _load(id, state.grain);
  }

  Future<void> _load(int projectId, StatsGrain grain) async {
    try {
      final res = await _api.forProject(
        projectId,
        grain: grain,
        // 分桶按用户本地时间进行
        tzOffsetMinutes: DateTime.now().timeZoneOffset.inMinutes,
      );
      state = state.copyWith(result: res, loading: false, clearError: true);
    } on Object catch (e) {
      state = state.copyWith(
        loading: false,
        error: e.toString(),
        // 保留上一次结果：网络抖动时不该把已有图表清空
        result: state.result,
      );
    }
  }
}

final statsProvider =
    NotifierProvider<StatsNotifier, StatsState>(StatsNotifier.new);
