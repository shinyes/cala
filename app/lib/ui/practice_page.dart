import 'dart:async';

import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/client.dart';
import '../api/models.dart';
import '../api/round_api.dart';
import '../scoring/scoring.dart' as scoring;
import '../state/practice.dart';
import '../state/session.dart';
import 'summary_page.dart';
import 'widgets/keypad.dart';

final roundApiProvider = Provider<RoundApi>((ref) {
  return RoundApi(ref.watch(apiClientProvider));
});

/// 练习页：自带键盘、可暂停、打错即时反馈（功能 6/9）。
///
/// 全部行为由 [PracticeMachine] 决定；本组件只做渲染与手势转发。
/// 判分**不在此处实现**——见 state/practice.dart 的说明。
class PracticePage extends ConsumerStatefulWidget {
  const PracticePage({
    super.key,
    required this.project,
    this.replayQuestions,
    this.replayCleanupTable,
    this.replayTolerance,
    this.title,
  });

  final Project project;

  /// 错题重练：直接使用已落库的快照题目，不调用规则重新出题
  /// （规格 §6.1(3)：作者事后改规则不得改变历史错题）。
  final List<Question>? replayQuestions;
  final Map<String, String>? replayCleanupTable;
  final scoring.Tolerance? replayTolerance;
  final String? title;

  bool get isReplay => replayQuestions != null;

  @override
  ConsumerState<PracticePage> createState() => _PracticePageState();
}

class _PracticePageState extends ConsumerState<PracticePage> {
  PracticeState? _state;
  String? _error;
  bool _submitting = false;

  /// 本轮开始时刻（用于交卷时上报）。
  DateTime _startedAt = DateTime.now();

  Timer? _ticker;

  @override
  void initState() {
    super.initState();
    if (widget.isReplay) {
      _startReplay();
    } else {
      _startRound();
    }
  }

  @override
  void dispose() {
    _ticker?.cancel();
    super.dispose();
  }

  void _startReplay() {
    _startedAt = DateTime.now();
    setState(() {
      _state = PracticeMachine.replayWrong(
        snapshotQuestions: widget.replayQuestions!,
        cleanupTable: widget.replayCleanupTable ?? const {},
        tolerance: widget.replayTolerance ?? scoring.Tolerance.exact,
      );
      _error = null;
    });
    _startTicker();
  }

  Future<void> _startRound() async {
    try {
      final res = await ref.read(roundApiProvider).start(widget.project.id);
      if (!mounted) return;
      _startedAt = DateTime.now();
      setState(() {
        _state = PracticeState(
          questions: res.questions,
          // 判分必须使用服务端下发的清洗表（规格 §5.5.4）
          cleanupTable: res.scoring.cleanupTable,
          tolerance: _toleranceFromProject(),
          seed: res.seed,
        );
        _error = null;
      });
      _startTicker();
    } on Object catch (e) {
      if (!mounted) return;
      setState(() => _error = e is ApiException ? e.message : '$e');
    }
  }

  scoring.Tolerance _toleranceFromProject() {
    final n = widget.project.toleranceNum;
    final d = widget.project.toleranceDen;
    if (n == null || d == null) return scoring.Tolerance.exact;
    return scoring.Tolerance(BigInt.from(n), BigInt.from(d));
  }

  /// 计时由 UI 驱动状态机（状态机自身不持有 Timer，便于测试）。
  void _startTicker() {
    _ticker?.cancel();
    _ticker = Timer.periodic(const Duration(milliseconds: 100), (_) {
      if (!mounted) return;
      final s = _state;
      if (s == null || s.finished || s.paused) return;
      setState(() => _state = PracticeMachine.tick(s, 100));
    });
  }

  void _apply(PracticeState next) {
    setState(() => _state = next);
    if (next.finished) {
      _ticker?.cancel();
      _finish();
    }
  }

  void _onDigit(String ch) {
    final s = _state;
    if (s == null) return;
    _apply(PracticeMachine.appendChar(s, ch));
  }

  void _onBackspace() {
    final s = _state;
    if (s == null) return;
    _apply(PracticeMachine.backspace(s));
  }

  void _onSubmit() {
    final s = _state;
    if (s == null) return;
    final next = PracticeMachine.submit(s);
    _apply(next);

    // 答对：短暂高亮后自动推进，不打断节奏。
    // 答错：停住等用户点「下一题」（功能 9 原文）。
    if (next.lastCorrect == true && next.awaitingNext) {
      Future<void>.delayed(const Duration(milliseconds: 220), () {
        if (!mounted) return;
        final cur = _state;
        if (cur != null && cur.awaitingNext && cur.lastCorrect == true) {
          _apply(PracticeMachine.nextQuestion(cur));
        }
      });
    }
  }

  void _onNext() {
    final s = _state;
    if (s == null) return;
    _apply(PracticeMachine.nextQuestion(s));
  }

  void _togglePause() {
    final s = _state;
    if (s == null) return;
    if (s.paused) {
      setState(() => _state = PracticeMachine.resume(s));
    } else {
      _confirmPause();
    }
  }

  void _confirmPause() {
    final s = _state;
    if (s == null) return;
    setState(() => _state = PracticeMachine.pause(s));

    showCupertinoDialog<void>(
      context: context,
      barrierDismissible: false,
      builder: (ctx) => CupertinoAlertDialog(
        title: const Text('已暂停'),
        content: const Text('继续后需要重新输入当前这道题。\n关闭应用不会保存本轮进度。'),
        actions: [
          CupertinoDialogAction(
            isDestructiveAction: true,
            onPressed: () {
              Navigator.of(ctx).pop();
              Navigator.of(context).pop(); // 放弃本轮：不落库（D6）
            },
            child: const Text('放弃本轮'),
          ),
          CupertinoDialogAction(
            isDefaultAction: true,
            onPressed: () {
              Navigator.of(ctx).pop();
              if (!mounted) return;
              setState(() => _state = PracticeMachine.resume(_state!));
            },
            child: const Text('继续'),
          ),
        ],
      ),
    );
  }

  Future<void> _finish() async {
    final s = _state;
    if (s == null || _submitting) return;

    // 重练错题不进统计：它是基于历史快照的复习，不应写入新的一轮记录
    if (widget.isReplay) {
      _goSummary(
        CompleteRoundResult(
          roundId: 0,
          totalMs: s.totalElapsedMs,
          questionCount: s.answers.length,
          correctCount: s.answers.where((a) => a.clientCorrect).length,
          discrepancies: 0,
          staleProject: false,
        ),
        s,
      );
      return;
    }

    setState(() => _submitting = true);
    try {
      final res = await ref.read(roundApiProvider).complete(
            projectId: widget.project.id,
            seed: s.seed,
            startedAt: _startedAt,
            finishedAt: DateTime.now(),
            attempts: PracticeMachine.toAttempts(s),
          );
      if (!mounted) return;
      _goSummary(res, s);
    } on Object catch (e) {
      if (!mounted) return;
      setState(() => _submitting = false);
      await showCupertinoDialog<void>(
        context: context,
        builder: (ctx) => CupertinoAlertDialog(
          title: const Text('交卷失败'),
          content: Text(e is ApiException ? e.message : '$e'),
          actions: [
            CupertinoDialogAction(
              isDefaultAction: true,
              onPressed: () => Navigator.of(ctx).pop(),
              child: const Text('好'),
            ),
          ],
        ),
      );
    }
  }

  void _goSummary(CompleteRoundResult result, PracticeState s) {
    Navigator.of(context).pushReplacement(
      CupertinoPageRoute<void>(
        builder: (_) => SummaryPage(
          project: widget.project,
          result: result,
          state: s,
          cleanupTable: s.cleanupTable,
          tolerance: s.tolerance,
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final s = _state;

    if (_error != null) {
      return CupertinoPageScaffold(
        navigationBar: CupertinoNavigationBar(
          middle: Text(widget.title ?? widget.project.title),
        ),
        child: Center(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                const Icon(CupertinoIcons.exclamationmark_triangle,
                    size: 40, color: CupertinoColors.systemOrange),
                const SizedBox(height: 12),
                Text(_error!, textAlign: TextAlign.center),
                const SizedBox(height: 16),
                CupertinoButton.filled(
                  onPressed: () {
                    setState(() => _error = null);
                    widget.isReplay ? _startReplay() : _startRound();
                  },
                  child: const Text('重试'),
                ),
              ],
            ),
          ),
        ),
      );
    }

    if (s == null) {
      return const CupertinoPageScaffold(
        child: Center(child: CupertinoActivityIndicator()),
      );
    }

    return PracticeKeyHandler(
      enabled: s.acceptsInput,
      onChar: _onDigit,
      onBackspace: _onBackspace,
      onSubmit: _onSubmit,
      child: CupertinoPageScaffold(
        navigationBar: CupertinoNavigationBar(
          middle: Text(widget.title ?? widget.project.title),
          trailing: CupertinoButton(
            padding: EdgeInsets.zero,
            onPressed: s.finished ? null : _togglePause,
            child: Icon(s.paused
                ? CupertinoIcons.play_fill
                : CupertinoIcons.pause_fill),
          ),
        ),
        child: SafeArea(
          child: Column(
            children: [
              _Header(state: s),
              Expanded(
                child: _QuestionArea(state: s, onNext: _onNext),
              ),
              Keypad(
                enabled: s.acceptsInput,
                onDigit: _onDigit,
                onBackspace: _onBackspace,
                onSubmit: _onSubmit,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _Header extends StatelessWidget {
  const _Header({required this.state});
  final PracticeState state;

  @override
  Widget build(BuildContext context) {
    final seconds = (state.questionElapsedMs / 1000);
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
      child: Column(
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                '第 ${state.displayIndex} / ${state.questionCount} 题',
                style: const TextStyle(
                    fontSize: 14, color: CupertinoColors.secondaryLabel),
              ),
              Text(
                '${seconds.toStringAsFixed(1)}s',
                style: TextStyle(
                  fontSize: 14,
                  fontFeatures: const [FontFeature.tabularFigures()],
                  color: state.paused
                      ? CupertinoColors.secondaryLabel
                      : CupertinoColors.label,
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          // 自绘进度条：Cupertino 没有 Material 的 LinearProgressIndicator，
          // 且本项目不使用 Material 组件（D10）。
          ClipRRect(
            borderRadius: BorderRadius.circular(3),
            child: Container(
              height: 5,
              color: CupertinoColors.tertiarySystemFill.resolveFrom(context),
              child: FractionallySizedBox(
                alignment: Alignment.centerLeft,
                widthFactor: state.progress.clamp(0.0, 1.0),
                child: Container(color: CupertinoColors.systemBlue),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// 开新一轮：用一个**全新的** [PracticePage] 替换总结页 / 错题页。
///
/// 为什么不用回调：总结页是由练习页 `pushReplacement` 出来的，
/// 练习页此时**已经被销毁**。若「再来一轮」回调到原练习页 State 上，
/// 它内部的 `if (!mounted) return;` 会让整件事**静默失效** ——
/// 用户点了按钮却什么都没发生，还会白发一次网络请求。
/// （本改动之前正是如此，回归测试见 test/ui/another_round_test.dart。）
///
/// 改为导航到一个全新的练习页：它会在 initState 里自行取新一轮题目，
/// 因此不依赖任何可能已销毁的 State。
///
/// 用 pushAndRemoveUntil 而非 push：清掉夹在中间的总结页与错题页，
/// 否则从新一轮返回会回到上一轮的总结页 —— 那既无意义又容易误操作。
/// 保留最底层（route.isFirst，即 AppShell），使返回仍能回到 Tab 骨架。
void pushNewRound(BuildContext context, Project project) {
  Navigator.of(context).pushAndRemoveUntil(
    CupertinoPageRoute<void>(
      builder: (_) => PracticePage(project: project, title: project.title),
    ),
    (route) => route.isFirst,
  );
}

class _QuestionArea extends StatelessWidget {
  const _QuestionArea({required this.state, required this.onNext});
  final PracticeState state;
  final VoidCallback onNext;

  @override
  Widget build(BuildContext context) {
    final q = state.currentQuestion;
    if (q == null) return const SizedBox.shrink();

    final showFeedback = state.awaitingNext && state.lastCorrect != null;
    final correct = state.lastCorrect == true;

    // 可滚动的题目区。
    //
    // 为什么不能只是一个 Column：答错时会出现「答错了 / 正确答案 / 下一题」，
    // 内容明显变高。键盘固定占约 286px 后，屏幕较矮时（横屏、小屏、
    // 系统大字号）剩余高度不足以容纳 —— 实测在 600px 高的画布上溢出 54px，
    // 被裁掉的正是**「下一题」按钮**，也就是用户此刻唯一能推进的操作。
    //
    // 用 LayoutBuilder + ConstrainedBox(minHeight) 而不是单纯的
    // SingleChildScrollView：空间充足时仍要垂直居中（否则题面会贴顶），
    // 空间不足时才退化为滚动。
    return LayoutBuilder(
      builder: (context, constraints) {
        const pad = 20.0;
        final available = constraints.maxHeight - pad * 2;
        return SingleChildScrollView(
          padding: const EdgeInsets.all(pad),
          child: ConstrainedBox(
            constraints: BoxConstraints(minHeight: available > 0 ? available : 0),
            child: _questionColumn(context, q, showFeedback, correct),
          ),
        );
      },
    );
  }

  Widget _questionColumn(
    BuildContext context,
    Question q,
    bool showFeedback,
    bool correct,
  ) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
          // 暂停时遮蔽题面：避免暂停期间继续思考
          Opacity(
            opacity: state.paused ? 0.12 : 1,
            child: Text(
              q.q,
              textAlign: TextAlign.center,
              style: const TextStyle(fontSize: 32, fontWeight: FontWeight.w600),
            ),
          ),
          const SizedBox(height: 28),

          // 输入回显
          Container(
            width: double.infinity,
            padding: const EdgeInsets.symmetric(vertical: 14, horizontal: 16),
            decoration: BoxDecoration(
              color: CupertinoColors.tertiarySystemFill.resolveFrom(context),
              borderRadius: BorderRadius.circular(10),
            ),
            child: Text(
              state.paused ? '已暂停' : (state.input.isEmpty ? '—' : state.input),
              textAlign: TextAlign.center,
              style: TextStyle(
                fontSize: 26,
                fontFeatures: const [FontFeature.tabularFigures()],
                color: state.input.isEmpty
                    ? CupertinoColors.tertiaryLabel
                    : CupertinoColors.label,
              ),
            ),
          ),

          if (state.paused) ...[
            const SizedBox(height: 20),
            const Text('已暂停（不计时）',
                style: TextStyle(color: CupertinoColors.secondaryLabel)),
          ],

          // 答错：立即显示正确答案并出现「下一题」（功能 9）
          if (showFeedback && !correct) ...[
            const SizedBox(height: 20),
            Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                const Icon(CupertinoIcons.xmark_circle_fill,
                    color: CupertinoColors.systemRed, size: 22),
                const SizedBox(width: 8),
                const Text('答错了',
                    style: TextStyle(
                        fontSize: 17,
                        color: CupertinoColors.systemRed,
                        fontWeight: FontWeight.w600)),
              ],
            ),
            const SizedBox(height: 8),
            Text('正确答案：${q.a}',
                style: const TextStyle(fontSize: 20, fontWeight: FontWeight.w500)),
            const SizedBox(height: 16),
            CupertinoButton.filled(
              onPressed: onNext,
              child: const Text('下一题'),
            ),
          ],

          if (showFeedback && correct) ...[
            const SizedBox(height: 20),
            const Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Icon(CupertinoIcons.checkmark_circle_fill,
                    color: CupertinoColors.systemGreen, size: 22),
                SizedBox(width: 8),
                Text('正确',
                    style: TextStyle(
                        fontSize: 17,
                        color: CupertinoColors.systemGreen,
                        fontWeight: FontWeight.w600)),
              ],
            ),
          ],
      ],
    );
  }
}
