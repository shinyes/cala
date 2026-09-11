import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/subscription_api.dart';
import 'projects.dart';
import 'session.dart';

/// 订阅与分享端点。
final subscriptionApiProvider = Provider<SubscriptionApi>((ref) {
  return SubscriptionApi(ref.watch(apiClientProvider));
});

/// 导入订阅的状态。
@immutable
class SubscriptionState {
  final bool busy;
  final ImportOutcome? outcome;
  final String? error;

  const SubscriptionState({this.busy = false, this.outcome, this.error});

  SubscriptionState copyWith({
    bool? busy,
    ImportOutcome? outcome,
    String? error,
    bool clearError = false,
    bool clearOutcome = false,
  }) {
    return SubscriptionState(
      busy: busy ?? this.busy,
      outcome: clearOutcome ? null : (outcome ?? this.outcome),
      error: clearError ? null : (error ?? this.error),
    );
  }
}

/// 订阅动作。
class SubscriptionNotifier extends Notifier<SubscriptionState> {
  @override
  SubscriptionState build() => const SubscriptionState();

  SubscriptionApi get _api => ref.read(subscriptionApiProvider);

  /// 导入链接。成功后会刷新项目列表（订阅分组会出现新项目）。
  ///
  /// [raw] 是用户粘贴的原始文本，按行切分并去除空行。
  /// 支持一次粘贴多行——这是功能 5 的明确要求。
  Future<void> importFromText(String raw) async {
    final links = raw
        .split(RegExp(r'[\r\n]+'))
        .map((l) => l.trim())
        .where((l) => l.isNotEmpty)
        .toList(growable: false);

    if (links.isEmpty) {
      state = state.copyWith(error: '请至少粘贴一条分享链接', clearOutcome: true);
      return;
    }

    state = state.copyWith(busy: true, clearError: true, clearOutcome: true);
    try {
      final outcome = await _api.import(links);
      state = state.copyWith(busy: false, outcome: outcome);
      if (outcome.succeeded > 0) {
        await ref.read(projectsProvider.notifier).refresh();
      }
    } on Object catch (e) {
      state = state.copyWith(busy: false, error: e.toString());
    }
  }

  /// 退订：清空本人在该项目下的全部记录与统计。
  ///
  /// 这是不可逆的破坏性操作，调用方必须先向用户确认。
  /// 返回被删除的轮次数。
  Future<int> unsubscribe(int projectId) async {
    final deleted = await _api.unsubscribe(projectId);
    await ref.read(projectsProvider.notifier).refresh();
    return deleted;
  }

  /// 生成或重置分享链接。
  Future<ShareLink> share(int projectId) => _api.share(projectId);

  /// 撤销分享。
  Future<void> unshare(int projectId) => _api.unshare(projectId);
}

final subscriptionProvider =
    NotifierProvider<SubscriptionNotifier, SubscriptionState>(
        SubscriptionNotifier.new);
