import 'package:flutter/foundation.dart';

import '../api/models.dart';
import '../scoring/scoring.dart' as scoring;

/// 输入长度上限。
///
/// 与 keyboard 字母表配合，防止用户长按造成无意义超长输入。
const int maxInputLen = 20;

/// 一道题的作答记录。
@immutable
class AnswerRecord {
  final int index;

  /// 用户原始输入（落库用，**不做规范化**——服务端按原始输入重算）。
  final String input;

  /// 客户端判定。仅用于即时反馈；权威判定由服务端给出（规格 §6.1(5)）。
  final bool clientCorrect;

  final int elapsedMs;

  const AnswerRecord({
    required this.index,
    required this.input,
    required this.clientCorrect,
    required this.elapsedMs,
  });
}

/// 练习运行时状态（不可变快照）。
///
/// 设计意图：把「题目 + 作答 + 计时 + 暂停 + 推进」收敛为一个不可变快照，
/// UI 只渲染快照。这样「暂停是否真的停表」「答错是否停住」等行为
/// 可以在不启动 Widget 树的情况下断言。
///
/// 本类**不持有 Timer**：时间由调用方通过 [tick] 推进。
/// 这不只是为了可测试——持有 Timer 会让暂停/继续与计时器生命周期
/// 出现难以测试的耦合。
@immutable
class PracticeState {
  final List<Question> questions;

  /// 服务端下发的清洗表（规格 §5.5.4）。判分必须使用它。
  final Map<String, String> cleanupTable;

  final scoring.Tolerance tolerance;

  final int seed;

  /// 当前题号。
  final int current;

  final List<AnswerRecord> answers;

  /// 当前题的输入缓冲。
  final String input;

  /// 最后一次提交的判定结果；为 null 表示尚未提交。
  final bool? lastCorrect;

  /// 答错后等待用户按「下一题」。为 true 时拒绝输入。
  final bool awaitingNext;

  final bool paused;

  /// 当前题已耗时（毫秒）。
  final int questionElapsedMs;

  /// 本轮总耗时（毫秒），为各题耗时之和。
  final int totalElapsedMs;

  final bool finished;

  const PracticeState({
    required this.questions,
    required this.cleanupTable,
    required this.tolerance,
    required this.seed,
    this.current = 0,
    this.answers = const [],
    this.input = '',
    this.lastCorrect,
    this.awaitingNext = false,
    this.paused = false,
    this.questionElapsedMs = 0,
    this.totalElapsedMs = 0,
    this.finished = false,
  });

  Question? get currentQuestion =>
      current >= 0 && current < questions.length ? questions[current] : null;

  int get questionCount => questions.length;

  /// 已完成的题数（含当前已提交的）。
  int get answeredCount => answers.length;

  /// 进度显示用：第几题（1-based）。
  int get displayIndex => (current + 1).clamp(1, questionCount == 0 ? 1 : questionCount);

  double get progress => questionCount == 0 ? 0 : answers.length / questionCount;

  /// 是否接受键盘输入。
  bool get acceptsInput => !paused && !finished && !awaitingNext;

  /// 本轮错题（客户端判定视角，仅用于即时反馈）。
  List<AnswerRecord> get wrongAnswers =>
      answers.where((a) => !a.clientCorrect).toList(growable: false);

  PracticeState copyWith({
    int? current,
    List<AnswerRecord>? answers,
    String? input,
    bool? lastCorrect,
    bool clearLastCorrect = false,
    bool? awaitingNext,
    bool? paused,
    int? questionElapsedMs,
    int? totalElapsedMs,
    bool? finished,
  }) {
    return PracticeState(
      questions: questions,
      cleanupTable: cleanupTable,
      tolerance: tolerance,
      seed: seed,
      current: current ?? this.current,
      answers: answers ?? this.answers,
      input: input ?? this.input,
      lastCorrect: clearLastCorrect ? null : (lastCorrect ?? this.lastCorrect),
      awaitingNext: awaitingNext ?? this.awaitingNext,
      paused: paused ?? this.paused,
      questionElapsedMs: questionElapsedMs ?? this.questionElapsedMs,
      totalElapsedMs: totalElapsedMs ?? this.totalElapsedMs,
      finished: finished ?? this.finished,
    );
  }
}

/// 练习运行时的状态转移。
///
/// 所有行为都是纯函数：给定状态与动作，返回新状态。
/// 判分一律经 [scoring.compare]（基线 §9(5)），UI 不得另写比较逻辑。
class PracticeMachine {
  const PracticeMachine._();

  /// 追加一个字符。返回原状态表示无变化。
  static PracticeState appendChar(PracticeState s, String ch) {
    if (!s.acceptsInput) return s;
    if (s.input.length >= maxInputLen) return s;
    return s.copyWith(input: s.input + ch);
  }

  /// 退格。空输入时为 no-op。
  static PracticeState backspace(PracticeState s) {
    if (!s.acceptsInput) return s;
    if (s.input.isEmpty) return s;
    return s.copyWith(input: s.input.substring(0, s.input.length - 1));
  }

  /// 提交当前作答。
  ///
  /// 判定使用服务端下发的清洗表与信封（规格 §5.5.4），一律经 [scoring.compare]。
  ///
  /// 两条路径都置 [PracticeState.awaitingNext]，语义是「等推进」：
  /// - 答对：UI 短暂高亮后自动调用 [nextQuestion]（不打断节奏）
  /// - 答错：UI 显示正确答案与「下一题」按钮，由用户点击（功能 9 原文）
  ///
  /// 两者共用同一状态标志而非各设一个，避免出现「既在等确认又在等自动推进」的矛盾状态。
  static PracticeState submit(PracticeState s) {
    if (!s.acceptsInput) return s;

    final q = s.currentQuestion;
    if (q == null) return s;

    final env = q.envelope;
    final bool correct;
    if (env == null) {
      // 服务端下发了未知信封：无法判定。按错误处理并在 UI 提示，
      // 而不是假定正确——那会让用户以为自己答对了。
      correct = false;
    } else {
      correct =
          scoring.compare(s.input, env, s.tolerance, s.cleanupTable).correct;
    }

    final record = AnswerRecord(
      index: q.index,
      input: s.input,
      clientCorrect: correct,
      elapsedMs: s.questionElapsedMs,
    );

    return s.copyWith(
      answers: [...s.answers, record],
      totalElapsedMs: s.totalElapsedMs + s.questionElapsedMs,
      lastCorrect: correct,
      awaitingNext: true,
    );
  }

  /// 推进到下一题（答对后由 UI 自动调用；答错后由用户点击触发）。
  static PracticeState nextQuestion(PracticeState s) {
    if (!s.awaitingNext) return s;

    final nextIndex = s.current + 1;
    if (nextIndex >= s.questions.length) {
      return s.copyWith(
        finished: true,
        input: '',
        questionElapsedMs: 0,
        awaitingNext: false,
        clearLastCorrect: true,
      );
    }
    return s.copyWith(
      current: nextIndex,
      input: '',
      questionElapsedMs: 0,
      awaitingNext: false,
      clearLastCorrect: true,
    );
  }

  /// 暂停。
  ///
  /// 清空当前输入：既避免「暂停时慢慢想再继续」，也避免暂停期间
  /// 输入缓冲与计时状态不一致。
  static PracticeState pause(PracticeState s) {
    if (s.paused || s.finished) return s;
    return s.copyWith(paused: true, input: '');
  }

  /// 继续。计时从暂停处续上（暂停期间 [tick] 被忽略）。
  static PracticeState resume(PracticeState s) {
    if (!s.paused) return s;
    return s.copyWith(paused: false);
  }

  /// 推进计时。
  ///
  /// 暂停或已结束时**忽略**——这是「暂停是否真的停表」的唯一实现点。
  static PracticeState tick(PracticeState s, int deltaMs) {
    if (deltaMs <= 0) return s;
    if (s.paused || s.finished) return s;
    return s.copyWith(
      questionElapsedMs: s.questionElapsedMs + deltaMs,
    );
  }

  /// 构造交卷载荷。耗时由状态机累加值给出（服务端亦会独立派生汇总）。
  static List<AttemptInput> toAttempts(PracticeState s) => s.answers
      .map((a) => AttemptInput(
            index: a.index,
            input: a.input,
            clientIsCorrect: a.clientCorrect,
            elapsedMs: a.elapsedMs,
          ))
      .toList(growable: false);

  /// 用错题快照开一轮重练。
  ///
  /// **不调用规则重新出题**：直接使用已落库的题面与答案快照，
  /// 因此作者事后修改规则不会改变历史错题（规格 §6.1(3)）。
  static PracticeState replayWrong({
    required List<Question> snapshotQuestions,
    required Map<String, String> cleanupTable,
    required scoring.Tolerance tolerance,
  }) {
    return PracticeState(
      questions: snapshotQuestions,
      cleanupTable: cleanupTable,
      tolerance: tolerance,
      seed: 0,
    );
  }
}
