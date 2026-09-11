import 'client.dart';

/// 一条链接的导入结果。
///
/// 逐条返回而非整批成败：用户粘贴 5 个链接时，其中一个失效不应让其余 4 个也失败。
class ImportResultItem {
  final String link;
  final bool ok;
  final int? projectId;
  final String? title;

  /// 已经在订阅列表中（不算失败）。
  final bool alreadySubscribed;

  /// 失败原因，后端已给出可读文案（哪一条、为什么）。
  final String? error;

  const ImportResultItem({
    required this.link,
    required this.ok,
    this.projectId,
    this.title,
    this.alreadySubscribed = false,
    this.error,
  });

  factory ImportResultItem.fromJson(Map<String, dynamic> j) => ImportResultItem(
        link: j['link'] as String? ?? '',
        ok: j['ok'] as bool? ?? false,
        projectId: (j['projectId'] as num?)?.toInt(),
        title: j['title'] as String?,
        alreadySubscribed: j['alreadySubscribed'] as bool? ?? false,
        error: j['error'] as String?,
      );

  /// 结果分类，便于界面选择图标与文案。
  ImportStatus get status {
    if (ok && alreadySubscribed) return ImportStatus.alreadySubscribed;
    if (ok) return ImportStatus.added;
    return ImportStatus.failed;
  }
}

enum ImportStatus { added, alreadySubscribed, failed }

/// 导入汇总。
class ImportOutcome {
  final List<ImportResultItem> results;
  final int succeeded;
  final int total;

  const ImportOutcome({
    required this.results,
    required this.succeeded,
    required this.total,
  });

  factory ImportOutcome.fromJson(Map<String, dynamic> j) {
    final raw = j['results'];
    return ImportOutcome(
      results: raw is List
          ? raw
              .whereType<Map<String, dynamic>>()
              .map(ImportResultItem.fromJson)
              .toList(growable: false)
          : const [],
      succeeded: (j['succeeded'] as num?)?.toInt() ?? 0,
      total: (j['total'] as num?)?.toInt() ?? 0,
    );
  }
}

/// 分享链接。
class ShareLink {
  final String token;
  final String link;

  const ShareLink({required this.token, required this.link});

  factory ShareLink.fromJson(Map<String, dynamic> j) => ShareLink(
        token: j['shareToken'] as String? ?? '',
        link: j['link'] as String? ?? '',
      );

  bool get isEmpty => token.isEmpty && link.isEmpty;
}

/// 订阅与分享端点。
class SubscriptionApi {
  final ApiClient _c;
  SubscriptionApi(this._c);

  /// 导入一组分享链接。
  Future<ImportOutcome> import(List<String> links) async {
    final j = await _c.post('/api/subscriptions/import', {'links': links});
    return ImportOutcome.fromJson(j);
  }

  /// 退订并清空本人历史。返回被删除的轮次数。
  Future<int> unsubscribe(int projectId) async {
    final j = await _c.delete('/api/subscriptions/$projectId');
    return (j['deletedRounds'] as num?)?.toInt() ?? 0;
  }

  /// 生成或重置分享链接。
  Future<ShareLink> share(int projectId) async {
    final j = await _c.post('/api/projects/$projectId/share');
    return ShareLink.fromJson(j);
  }

  /// 撤销分享。
  Future<void> unshare(int projectId) => _c.delete('/api/projects/$projectId/share');
}
