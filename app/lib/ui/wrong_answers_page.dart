import 'dart:convert';

import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/models.dart';
import '../scoring/scoring.dart' as scoring;
import '../state/practice.dart';
import 'practice_page.dart';

/// 错题页（功能 7）。
///
/// 错题一律以**服务端权威判定**（`serverIsCorrect == false`）筛选，
/// 不用客户端判定——客户端判定只用于答题时的即时反馈（规格 §6.1(5)）。
///
/// 数据来源优先请求服务端已落库的 attempt 记录（含快照），
/// 拿不到时退回本轮内存状态（离线或重练场景）。
class WrongAnswersPage extends ConsumerStatefulWidget {
  const WrongAnswersPage({
    super.key,
    required this.roundResult,
    required this.state,
    required this.cleanupTable,
    required this.tolerance,
    required this.project,
  });

  final CompleteRoundResult roundResult;
  final PracticeState state;
  final Map<String, String> cleanupTable;
  final scoring.Tolerance tolerance;
  final Project project;

  @override
  ConsumerState<WrongAnswersPage> createState() => _WrongAnswersPageState();
}

/// 一条错题的显示数据。
class _WrongItem {
  final String question;
  final String yourAnswer;
  final String correctAnswer;
  final Question snapshot;

  const _WrongItem({
    required this.question,
    required this.yourAnswer,
    required this.correctAnswer,
    required this.snapshot,
  });
}

class _WrongAnswersPageState extends ConsumerState<WrongAnswersPage> {
  List<_WrongItem>? _items;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    // 重练产生的结果没有 roundId（未落库），直接用内存状态
    if (widget.roundResult.roundId == 0) {
      setState(() => _items = _fromMemory());
      return;
    }
    try {
      final attempts =
          await ref.read(roundApiProvider).attempts(widget.roundResult.roundId);
      setState(() => _items = _fromServer(attempts));
    } on Object catch (_) {
      // 取不到落库记录时退回内存状态：用户仍能看到错题，
      // 只是失去「服务端权威判定」这一更强保证。
      setState(() => _items = _fromMemory());
    }
  }

  /// 由服务端记录构造。判错与否**只依据 serverIsCorrect**。
  List<_WrongItem> _fromServer(List<StoredAttempt> attempts) {
    final byIndex = {for (final q in widget.state.questions) q.index: q};
    final out = <_WrongItem>[];
    for (final a in attempts) {
      if (a.serverIsCorrect) continue;
      out.add(_WrongItem(
        question: a.qSnapshot,
        yourAnswer: a.userInput,
        correctAnswer: a.aSnapshot,
        snapshot: byIndex[a.index] ??
            Question(
              index: a.index,
              q: a.qSnapshot,
              a: a.aSnapshot,
              envelope: _envelopeFromJson(a.envelopeJson),
            ),
      ));
    }
    return out;
  }

  List<_WrongItem> _fromMemory() {
    final byIndex = {for (final q in widget.state.questions) q.index: q};
    return widget.state.wrongAnswers
        .map((r) => _WrongItem(
              question: byIndex[r.index]?.q ?? '',
              yourAnswer: r.input,
              correctAnswer: byIndex[r.index]?.a ?? '',
              snapshot: byIndex[r.index]!,
            ))
        .where((w) => w.snapshot.q.isNotEmpty)
        .toList(growable: false);
  }

  static scoring.Envelope? _envelopeFromJson(String raw) {
    if (raw.isEmpty) return null;
    try {
      final decoded = jsonDecode(raw);
      if (decoded is! Map) return null;
      return scoring.Envelope.fromJson(Map<String, dynamic>.from(decoded));
    } on Object {
      // 信封损坏时返回 null：该题判分时会被判错（状态机已定义该行为），
      // 好过让整个错题页打不开。
      return null;
    }
  }

  void _redoWrong() {
    final items = _items;
    if (items == null || items.isEmpty) return;

    // 用**快照**重练：不调用规则重新出题（规格 §6.1(3)）
    Navigator.of(context).push(
      CupertinoPageRoute<void>(
        builder: (_) => PracticePage(
          project: widget.project,
          title: '重练错题',
          replayQuestions: items.map((e) => e.snapshot).toList(growable: false),
          replayCleanupTable: widget.cleanupTable,
          replayTolerance: widget.tolerance,
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final items = _items;

    return CupertinoPageScaffold(
      navigationBar: CupertinoNavigationBar(
        middle: Text(items == null ? '错题' : '错题 ${items.length} 道'),
      ),
      child: SafeArea(
        child: _error != null
            ? Center(child: Text(_error!))
            : items == null
                ? const Center(child: CupertinoActivityIndicator())
                : Column(
                    children: [
                      Expanded(
                        child: items.isEmpty
                            ? const Center(child: Text('没有错题'))
                            : ListView.separated(
                                padding: const EdgeInsets.all(16),
                                itemCount: items.length,
                                separatorBuilder: (_, _) =>
                                    const SizedBox(height: 12),
                                itemBuilder: (_, i) =>
                                    _WrongCard(item: items[i], ordinal: i + 1),
                              ),
                      ),
                      Padding(
                        padding: const EdgeInsets.all(16),
                        child: Row(
                          children: [
                            Expanded(
                              child: CupertinoButton.filled(
                                onPressed: items.isEmpty ? null : _redoWrong,
                                child: const Text('重练错题'),
                              ),
                            ),
                            const SizedBox(width: 12),
                            Expanded(
                              child: CupertinoButton(
                                color: CupertinoColors.tertiarySystemFill
                                    .resolveFrom(context),
                                onPressed: () =>
                                    Navigator.of(context).pop(),
                                child: const Text('返回'),
                              ),
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
      ),
    );
  }
}

class _WrongCard extends StatelessWidget {
  const _WrongCard({required this.item, required this.ordinal});
  final _WrongItem item;
  final int ordinal;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: CupertinoColors.tertiarySystemFill.resolveFrom(context),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('第 $ordinal 题',
              style: const TextStyle(
                  fontSize: 12, color: CupertinoColors.secondaryLabel)),
          const SizedBox(height: 6),
          Text(item.question,
              style: const TextStyle(fontSize: 20, fontWeight: FontWeight.w600)),
          const SizedBox(height: 12),
          Row(
            children: [
              const Icon(CupertinoIcons.xmark,
                  size: 15, color: CupertinoColors.systemRed),
              const SizedBox(width: 6),
              Text('你的作答：${item.yourAnswer.isEmpty ? '（空）' : item.yourAnswer}',
                  style: const TextStyle(
                      fontSize: 15, color: CupertinoColors.systemRed)),
            ],
          ),
          const SizedBox(height: 4),
          Row(
            children: [
              const Icon(CupertinoIcons.checkmark,
                  size: 15, color: CupertinoColors.systemGreen),
              const SizedBox(width: 6),
              Text('正确答案：${item.correctAnswer}',
                  style: const TextStyle(
                      fontSize: 15, color: CupertinoColors.systemGreen)),
            ],
          ),
        ],
      ),
    );
  }
}
