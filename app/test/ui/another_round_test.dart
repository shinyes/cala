import 'package:cala/api/client.dart';
import 'package:cala/api/models.dart';
import 'package:cala/api/round_api.dart';
import 'package:cala/scoring/scoring.dart' as scoring;
import 'package:cala/ui/practice_page.dart';
import 'package:cala/ui/wrong_answers_page.dart';
import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

/// 可编程的 RoundApi 替身：记录取题次数，返回固定的单题轮次。
class _FakeRoundApi extends RoundApi {
  _FakeRoundApi() : super(ApiClient(baseUrl: 'http://unused.invalid'));

  int startCalls = 0;
  int completeCalls = 0;

  @override
  Future<StartRoundResult> start(int projectId) async {
    startCalls++;
    return StartRoundResult(
      seed: startCalls,
      scoring: const ScoringConfig(version: 1, cleanupTable: {}),
      questions: const [
        Question(index: 0, q: '3 + 4 = ?', a: '7', envelope: scoring.Envelope.rational('7', '1')),
      ],
    );
  }

  @override
  Future<CompleteRoundResult> complete({
    required int projectId,
    required int seed,
    required DateTime startedAt,
    required DateTime finishedAt,
    required List<AttemptInput> attempts,
  }) async {
    completeCalls++;
    final correct =
        attempts.where((a) => a.clientIsCorrect).length;
    return CompleteRoundResult(
      roundId: completeCalls,
      totalMs: 3000,
      questionCount: attempts.length,
      correctCount: correct,
      discrepancies: 0,
      staleProject: false,
    );
  }

  @override
  Future<List<StoredAttempt>> attempts(int roundId) async => const [];
}

const _project = Project(
  id: 1,
  ownerId: 1,
  title: '口算',
  description: '',
  questionCount: 1,
  cfgJson: '{}',
  ruleSource: '',
  createdAt: '',
  updatedAt: '',
);

void main() {
  late _FakeRoundApi api;

  setUp(() => api = _FakeRoundApi());

  /// 最底层放一个占位页，模拟真实栈里的 AppShell。
  /// 这一层很关键：练习页是用 pushReplacement 换成总结页的，
  /// 因此总结页**不是**首路由，而「再来一轮」必须能回到一个可用的页面。
  Widget host({required Widget Function() pushed}) {
    return ProviderScope(
      overrides: [roundApiProvider.overrideWithValue(api)],
      child: CupertinoApp(
        home: _Host(pushed: pushed),
      ),
    );
  }

  /// 推进若干帧，等异步落定（含路由转场与交卷请求）。
  Future<void> settle(WidgetTester tester) async {
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));
    await tester.pump(const Duration(milliseconds: 400));
  }

  /// 作答并推进。**必须如实反映既有产品行为**：
  /// 答对会在 220ms 后自动推进（不打断节奏），答错则停住等用户点「下一题」
  /// （功能 9 原文）。测试不能绕过这一点，否则测的就不是真实流程。
  Future<void> answer(WidgetTester tester, {required bool correct}) async {
    await tester.tap(find.text('确定'));
    await tester.pump();
    if (correct) {
      // 等待自动推进
      await tester.pump(const Duration(milliseconds: 400));
    } else {
      await tester.tap(find.text('下一题'));
      await tester.pump();
    }
    await settle(tester);
  }

  Future<void> beginRound(WidgetTester tester, Widget page) async {
    await tester.pumpWidget(host(pushed: () => page));
    await tester.tap(find.text('进入'));
    await settle(tester);
  }

  /// 用真实手机尺寸的画布，而不是 flutter_test 默认的 800×600。
  ///
  /// 默认画布比手机**矮**得多（600 vs 844），而键盘固定占约 286px，
  /// 于是题目区被压到放不下答错反馈 —— 布局会进入需要滚动的分支，
  /// 按钮跑到可视区外，`tester.tap` 便只发警告而静默不生效。
  /// 测试画布必须贴近真实机型，否则测的不是真实场景。
  /// （矮屏的溢出不变量由另一个专门的测试守护。）
  void usePhoneSurface(WidgetTester tester) {
    tester.view.physicalSize = const Size(780, 1688); // 390×844 @2x
    tester.view.devicePixelRatio = 2.0;
    addTearDown(tester.view.reset);
  }

  testWidgets('总结页「再来一轮」必须真的开出新一轮', (tester) async {
    usePhoneSurface(tester);
    await beginRound(tester, const PracticePage(project: _project, title: '练习'));
    expect(api.startCalls, 1, reason: '进入练习页应取一轮题目');

    // 输入正确答案 7
    await tester.tap(find.text('7'));
    await tester.pump();
    await answer(tester, correct: true);

    // 应先到总结页
    expect(find.text('本轮总结'), findsOneWidget,
        reason: '答完唯一一题后应进入总结页');
    expect(find.text('再来一轮'), findsOneWidget);

    await tester.tap(find.text('再来一轮'));
    await settle(tester);

    // 核心断言：又取了一轮题目，且界面上真的在练习（而不是回到空白页）
    expect(api.startCalls, 2,
        reason: '「再来一轮」必须真的向服务端再取一轮题目');
    expect(find.text('3 + 4 = ?'), findsOneWidget,
        reason: '点完应停在练习页并能看到题目，而不是什么都没发生');
    expect(find.text('本轮总结'), findsNothing, reason: '总结页应已离开');
  });

  testWidgets('错题页「再来一轮」必须真的开出新一轮', (tester) async {
    usePhoneSurface(tester);
    await beginRound(tester, const PracticePage(project: _project, title: '练习'));

    // 输入错误答案 9，制造一道错题
    await tester.tap(find.text('9'));
    await tester.pump();
    await answer(tester, correct: false);

    expect(find.text('本轮总结'), findsOneWidget);
    await tester.tap(find.text('错题 1 道'));
    await settle(tester);
    expect(find.byType(WrongAnswersPage), findsOneWidget);

    // 规格功能 7 明确要求错题页有「再来一轮」
    expect(find.text('再来一轮'), findsOneWidget,
        reason: '错题页必须有「再来一轮」（规格功能 7）');

    final before = api.startCalls;
    await tester.tap(find.text('再来一轮'));
    await settle(tester);

    expect(api.startCalls, before + 1, reason: '「再来一轮」应再取一轮题目');
    expect(find.text('3 + 4 = ?'), findsOneWidget);
    expect(find.byType(WrongAnswersPage), findsNothing, reason: '错题页应已离开');
  });

  testWidgets('「再来一轮」不残留上一轮的总结页与错题页', (tester) async {
    usePhoneSurface(tester);
    await beginRound(tester, const PracticePage(project: _project, title: '练习'));
    await tester.tap(find.text('9'));
    await tester.pump();
    await answer(tester, correct: false);

    await tester.tap(find.text('错题 1 道'));
    await settle(tester);
    await tester.tap(find.text('再来一轮'));
    await settle(tester);

    // 从新一轮返回时应直接回到最底层，而不是回到上一轮的总结页
    await tester.pageBack();
    await settle(tester);

    expect(find.text('本轮总结'), findsNothing,
        reason: '返回后不应看到上一轮的总结页');
    expect(find.byType(WrongAnswersPage), findsNothing);
    expect(find.text('占位首页'), findsOneWidget, reason: '应回到最底层');
  });

  // 这一组守护一个实测发现的布局缺陷：
  // 答错时会多出「答错了 / 正确答案 / 下一题」，内容明显变高；
  // 键盘固定占约 286px，屏幕一矮，题目区就放不下 ——
  // 原先会产生 RenderFlex overflow，被裁掉的正是**「下一题」按钮**，
  // 也就是用户此刻唯一能推进的操作。
  group('矮屏不溢出', () {
    /// 逐档降低高度，确认任何尺寸下都不出现 RenderFlex overflow。
    for (final h in <double>[844, 700, 640, 600, 560, 480]) {
      testWidgets('画布高 ${h.toInt()}px：答错反馈不溢出', (tester) async {
        tester.view.physicalSize = Size(780, h * 2); // 390×h @2x
        tester.view.devicePixelRatio = 2.0;
        addTearDown(tester.view.reset);

        await beginRound(
            tester, const PracticePage(project: _project, title: '练习'));
        await tester.tap(find.text('9')); // 错误答案，触发最长的反馈布局
        await tester.pump();

        await tester.tap(find.text('确定'), warnIfMissed: false);
        await tester.pump();

        // 溢出会以 FlutterError 形式出现在异常通道里
        final err = tester.takeException();
        expect(err, isNull,
            reason: '画布高 ${h.toInt()}px 时答错反馈溢出：$err');

        // 无论多矮，「下一题」都必须能被滚到 —— 不允许被裁掉而取不到。
        expect(find.text('下一题'), findsOneWidget,
            reason: '画布高 ${h.toInt()}px 时「下一题」按钮不应被裁掉');
      });
    }
  });
}

/// 最底层占位页，模拟 AppShell。
class _Host extends StatelessWidget {
  const _Host({required this.pushed});
  final Widget Function() pushed;

  @override
  Widget build(BuildContext context) {
    return CupertinoPageScaffold(
      navigationBar: const CupertinoNavigationBar(middle: Text('占位首页')),
      child: Center(
        child: CupertinoButton.filled(
          onPressed: () => Navigator.of(context).push(
            CupertinoPageRoute<void>(builder: (_) => pushed()),
          ),
          child: const Text('进入'),
        ),
      ),
    );
  }
}
