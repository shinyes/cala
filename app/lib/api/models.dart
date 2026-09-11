import '../scoring/scoring.dart' show Envelope;

/// 与后端契约对应的数据模型。
///
/// 字段名与后端 JSON 严格对应（后端 Go tag 用 camelCase）。
/// 解析一律「缺字段给默认值」而非抛异常：客户端不应因为后端新增/省略一个
/// 可选字段就整页崩掉。

/// 用户表示。后端永不返回口令哈希。
class User {
  final int id;
  final String username;
  final bool isAdmin;
  final bool disabled;
  final String createdAt;

  const User({
    required this.id,
    required this.username,
    required this.isAdmin,
    required this.disabled,
    required this.createdAt,
  });

  factory User.fromJson(Map<String, dynamic> j) => User(
        id: (j['id'] as num?)?.toInt() ?? 0,
        username: j['username'] as String? ?? '',
        isAdmin: j['isAdmin'] as bool? ?? false,
        disabled: j['disabled'] as bool? ?? false,
        createdAt: j['createdAt'] as String? ?? '',
      );

  @override
  bool operator ==(Object other) => other is User && other.id == id;

  @override
  int get hashCode => id.hashCode;
}

/// 项目访问权。取值与后端 store.AccessOwner / AccessSubscriber 一致。
class ProjectAccess {
  static const owner = 'owner';
  static const subscriber = 'subscriber';
}

/// 项目。
class Project {
  final int id;
  final int ownerId;
  final String title;
  final String description;
  final int questionCount;
  final String cfgJson;
  final String ruleSource;

  /// 容差两列同时为空表示精确比较（规格 §6）。
  final int? toleranceNum;
  final int? toleranceDen;
  final String createdAt;
  final String updatedAt;

  /// 分享 token。未分享时为 null。
  ///
  /// 后端只在项目详情里返回它；完整链接（含服务器地址）需要调用
  /// `POST /api/projects/:id/share` 获取，因为链接里的 host 取自该请求的 Host 头。
  final String? shareToken;

  /// owner 或 subscriber，由后端查询填充。
  final String access;

  const Project({
    required this.id,
    required this.ownerId,
    required this.title,
    required this.description,
    required this.questionCount,
    required this.cfgJson,
    required this.ruleSource,
    this.toleranceNum,
    this.toleranceDen,
    required this.createdAt,
    required this.updatedAt,
    this.shareToken,
    this.access = '',
  });

  factory Project.fromJson(Map<String, dynamic> j) => Project(
        id: (j['id'] as num?)?.toInt() ?? 0,
        ownerId: (j['ownerId'] as num?)?.toInt() ?? 0,
        title: j['title'] as String? ?? '',
        description: j['description'] as String? ?? '',
        questionCount: (j['questionCount'] as num?)?.toInt() ?? 0,
        cfgJson: j['cfgJson'] as String? ?? '{}',
        ruleSource: j['ruleSource'] as String? ?? '',
        toleranceNum: (j['toleranceNum'] as num?)?.toInt(),
        toleranceDen: (j['toleranceDen'] as num?)?.toInt(),
        createdAt: j['createdAt'] as String? ?? '',
        updatedAt: j['updatedAt'] as String? ?? '',
        shareToken: j['shareToken'] as String?,
        access: j['access'] as String? ?? '',
      );

  bool get isOwner => access == ProjectAccess.owner;

  /// 是否配置了容差（两列同时非空）。
  bool get hasTolerance => toleranceNum != null && toleranceDen != null;

  @override
  bool operator ==(Object other) => other is Project && other.id == id;

  @override
  int get hashCode => id.hashCode;
}

/// 项目列表的分组结果。
///
/// 后端刻意分开返回而非混在一起：界面需要分组显示（规格 §4.3），
/// 合并会丢掉「这个项目是我的还是订阅的」这一信息。
class ProjectList {
  final List<Project> owned;
  final List<Project> subscribed;

  const ProjectList({required this.owned, required this.subscribed});

  factory ProjectList.fromJson(Map<String, dynamic> j) => ProjectList(
        owned: _list(j['owned'], Project.fromJson),
        subscribed: _list(j['subscribed'], Project.fromJson),
      );

  static List<T> _list<T>(Object? raw, T Function(Map<String, dynamic>) f) {
    if (raw is! List) return const [];
    return raw
        .whereType<Map<String, dynamic>>()
        .map(f)
        .toList(growable: false);
  }

  bool get isEmpty => owned.isEmpty && subscribed.isEmpty;
}

/// 一轮中的一道题。
///
/// [a] 与 [envelope] 都由服务端下发：前者用于展示正确答案，
/// 后者是判分依据。
///
/// 客户端**不**从 [a] 自行推断信封——那等于把 Go 侧的分类规则再实现一遍，
/// 正是 §5.5 要消除的重复 owner。服务端已给出信封，直接用。
class Question {
  final int index;
  final String q;
  final String a;
  final Envelope? envelope;

  const Question({
    required this.index,
    required this.q,
    required this.a,
    required this.envelope,
  });

  factory Question.fromJson(Map<String, dynamic> j) {
    final raw = j['envelope'];
    Envelope? env;
    if (raw is Map<String, dynamic>) {
      try {
        env = Envelope.fromJson(raw);
      } on FormatException {
        // 未知信封种类：保留为 null，判分时按「无法判定」处理，
        // 而不是让整个响应解析失败导致无法开始练习。
        env = null;
      }
    }
    return Question(
      index: (j['idx'] as num?)?.toInt() ?? 0,
      q: j['q'] as String? ?? '',
      a: j['a'] as String? ?? '',
      envelope: env,
    );
  }
}

/// 随轮次下发的判分配置。
///
/// 清洗表由服务端下发（规格 §5.5.4）：客户端**只应用**这张表，
/// 不内置任何映射。版本号用于日后分辨「客户端用了旧表」与「算法真的不一致」。
class ScoringConfig {
  final int version;
  final Map<String, String> cleanupTable;

  const ScoringConfig({required this.version, required this.cleanupTable});

  factory ScoringConfig.fromJson(Map<String, dynamic> j) {
    final raw = j['cleanupTable'];
    final table = <String, String>{};
    if (raw is Map) {
      raw.forEach((k, v) {
        if (k is String && v is String) table[k] = v;
      });
    }
    return ScoringConfig(
      version: (j['version'] as num?)?.toInt() ?? 0,
      cleanupTable: table,
    );
  }
}

/// `/rounds/start` 的响应。
class StartRoundResult {
  final int seed;
  final ScoringConfig scoring;
  final List<Question> questions;

  const StartRoundResult({
    required this.seed,
    required this.scoring,
    required this.questions,
  });

  factory StartRoundResult.fromJson(Map<String, dynamic> j) {
    final qs = j['questions'];
    return StartRoundResult(
      seed: (j['seed'] as num?)?.toInt() ?? 0,
      scoring: ScoringConfig.fromJson(
          (j['scoring'] as Map<String, dynamic>?) ?? const {}),
      questions: qs is List
          ? qs
              .whereType<Map<String, dynamic>>()
              .map(Question.fromJson)
              .toList(growable: false)
          : const [],
    );
  }
}

/// 交卷时上报的一道题。
///
/// [clientIsCorrect] 是客户端判定，**仅用于即时反馈与分歧告警**；
/// 后端会独立重算权威值（规格 §6.1(5)）。
class AttemptInput {
  final int index;
  final String input;
  final bool clientIsCorrect;
  final int elapsedMs;

  const AttemptInput({
    required this.index,
    required this.input,
    required this.clientIsCorrect,
    required this.elapsedMs,
  });

  Map<String, dynamic> toJson() => {
        'idx': index,
        'input': input,
        'clientIsCorrect': clientIsCorrect,
        'elapsedMs': elapsedMs,
      };
}

/// `/rounds/complete` 的响应。均为**服务端派生**的权威值。
class CompleteRoundResult {
  final int roundId;
  final int totalMs;
  final int questionCount;
  final int correctCount;

  /// 客户端判定与服务端判定不一致的题数。正常为 0。
  final int discrepancies;

  /// 项目在出题之后被作者修改过，服务端重放结果可能与客户端所见不同。
  final bool staleProject;

  const CompleteRoundResult({
    required this.roundId,
    required this.totalMs,
    required this.questionCount,
    required this.correctCount,
    required this.discrepancies,
    required this.staleProject,
  });

  factory CompleteRoundResult.fromJson(Map<String, dynamic> j) =>
      CompleteRoundResult(
        roundId: (j['roundId'] as num?)?.toInt() ?? 0,
        totalMs: (j['totalMs'] as num?)?.toInt() ?? 0,
        questionCount: (j['questionCount'] as num?)?.toInt() ?? 0,
        correctCount: (j['correctCount'] as num?)?.toInt() ?? 0,
        discrepancies: (j['discrepancies'] as num?)?.toInt() ?? 0,
        staleProject: j['staleProject'] as bool? ?? false,
      );

  double get accuracy =>
      questionCount == 0 ? 0 : correctCount / questionCount;

  int get avgMsPerQuestion =>
      questionCount == 0 ? 0 : (totalMs / questionCount).round();
}

/// 一条已落库的答题记录（用于错题页）。
class StoredAttempt {
  final int index;
  final String qSnapshot;
  final String aSnapshot;
  final String envelopeJson;
  final String userInput;
  final bool clientIsCorrect;

  /// 权威判定值。错题筛选一律用它，不用客户端判定。
  final bool serverIsCorrect;
  final int elapsedMs;

  const StoredAttempt({
    required this.index,
    required this.qSnapshot,
    required this.aSnapshot,
    required this.envelopeJson,
    required this.userInput,
    required this.clientIsCorrect,
    required this.serverIsCorrect,
    required this.elapsedMs,
  });

  factory StoredAttempt.fromJson(Map<String, dynamic> j) => StoredAttempt(
        index: (j['idx'] as num?)?.toInt() ?? 0,
        qSnapshot: j['qSnapshot'] as String? ?? '',
        aSnapshot: j['aSnapshot'] as String? ?? '',
        envelopeJson: j['envelopeJson'] as String? ?? '',
        userInput: j['userInput'] as String? ?? '',
        clientIsCorrect: j['clientIsCorrect'] as bool? ?? false,
        serverIsCorrect: j['serverIsCorrect'] as bool? ?? false,
        elapsedMs: (j['elapsedMs'] as num?)?.toInt() ?? 0,
      );
}
