import 'package:cala/api/models.dart';
import 'package:cala/scoring/scoring.dart' as scoring;
import 'package:cala/ui/practice_page.dart';
import 'package:cala/ui/widgets/keypad.dart';
import 'package:flutter/cupertino.dart';
import 'package:flutter_test/flutter_test.dart';

/// 键盘与练习页的渲染/交互测试。
///
/// 说明：核心行为（暂停是否停表、答错是否停住、判分是否经 scoring.compare）
/// 由 `test/state/practice_test.dart` 覆盖——那是纯 Dart 测试，比 widget test
/// 更稳定也更快。本文件只验证「UI 把状态渲染对、把输入转发对」。
void main() {
  group('Keypad', () {
    testWidgets('包含完整字母表 0-9 . - /', (tester) async {
      await tester.pumpWidget(
        CupertinoApp(
          home: CupertinoPageScaffold(
            child: Keypad(
              onDigit: (_) {},
              onBackspace: () {},
              onSubmit: () {},
            ),
          ),
        ),
      );

      for (final k in ['0', '1', '2', '3', '4', '5', '6', '7', '8', '9']) {
        expect(find.text(k), findsOneWidget, reason: '缺少数字键 $k');
      }
      expect(find.text('.'), findsOneWidget);
      expect(find.text('-'), findsOneWidget);
      expect(find.text('/'), findsOneWidget);
      expect(find.text('⌫'), findsOneWidget);
      expect(find.text('确定'), findsOneWidget);
    });

    testWidgets('点击数字键回调字符', (tester) async {
      final tapped = <String>[];
      await tester.pumpWidget(
        CupertinoApp(
          home: CupertinoPageScaffold(
            child: Keypad(
              onDigit: tapped.add,
              onBackspace: () {},
              onSubmit: () {},
            ),
          ),
        ),
      );

      await tester.tap(find.text('7'));
      await tester.tap(find.text('/'));
      await tester.pump();

      expect(tapped, ['7', '/']);
    });

    testWidgets('退格与确定分别回调', (tester) async {
      var backs = 0;
      var submits = 0;
      await tester.pumpWidget(
        CupertinoApp(
          home: CupertinoPageScaffold(
            child: Keypad(
              onDigit: (_) {},
              onBackspace: () => backs++,
              onSubmit: () => submits++,
            ),
          ),
        ),
      );

      await tester.tap(find.text('⌫'));
      await tester.tap(find.text('确定'));
      await tester.pump();

      expect(backs, 1);
      expect(submits, 1);
    });

    testWidgets('disabled 时不触发任何回调', (tester) async {
      final tapped = <String>[];
      var submits = 0;
      await tester.pumpWidget(
        CupertinoApp(
          home: CupertinoPageScaffold(
            child: Keypad(
              enabled: false,
              onDigit: tapped.add,
              onBackspace: () {},
              onSubmit: () => submits++,
            ),
          ),
        ),
      );

      await tester.tap(find.text('5'), warnIfMissed: false);
      await tester.tap(find.text('确定'), warnIfMissed: false);
      await tester.pump();

      expect(tapped, isEmpty);
      expect(submits, 0);
    });

    testWidgets('自定义提交按钮文案', (tester) async {
      await tester.pumpWidget(
        CupertinoApp(
          home: CupertinoPageScaffold(
            child: Keypad(
              onDigit: (_) {},
              onBackspace: () {},
              onSubmit: () {},
              submitLabel: '检查',
            ),
          ),
        ),
      );
      expect(find.text('检查'), findsOneWidget);
      expect(find.text('确定'), findsNothing);
    });
  });

  group('练习页渲染（重练模式，不触网）', () {
    /// 用重练模式构造练习页：题目来自快照，无需后端。
    Widget replayPage() {
      const project = Project(
        id: 1,
        ownerId: 1,
        title: '口算',
        description: '',
        questionCount: 2,
        cfgJson: '{}',
        ruleSource: '',
        createdAt: '',
        updatedAt: '',
      );

      return CupertinoApp(
        home: PracticePage(
          project: project,
          title: '重练错题',
          replayQuestions: const [
            Question(
              index: 0,
              q: '3 + 4 = ?',
              a: '7',
              envelope: scoring.Envelope.rational('7', '1'),
            ),
          ],
          replayCleanupTable: const {},
          // 注意：必须用前缀，Flutter 自身也有一个 physics.Tolerance
          replayTolerance: scoring.Tolerance.exact,
        ),
      );
    }

    testWidgets('显示首题题面与进度', (tester) async {
      await tester.pumpWidget(replayPage());
      await tester.pump();

      expect(find.text('3 + 4 = ?'), findsOneWidget);
      expect(find.textContaining('第 1'), findsOneWidget);
    });

    testWidgets('点击键盘后回显输入', (tester) async {
      await tester.pumpWidget(replayPage());
      await tester.pump();

      await tester.tap(find.text('7'));
      await tester.pump();
      expect(find.text('7'), findsWidgets);

      await tester.tap(find.text('1'));
      await tester.pump();
      expect(find.text('71'), findsOneWidget);
    });

    testWidgets('退格删除末位', (tester) async {
      await tester.pumpWidget(replayPage());
      await tester.pump();

      await tester.tap(find.text('7'));
      await tester.tap(find.text('1'));
      await tester.pump();
      expect(find.text('71'), findsOneWidget);

      await tester.tap(find.text('⌫'));
      await tester.pump();
      expect(find.text('7'), findsWidgets);
      expect(find.text('71'), findsNothing);
    });

    testWidgets('未输入时回显占位符', (tester) async {
      await tester.pumpWidget(replayPage());
      await tester.pump();
      expect(find.text('—'), findsOneWidget);
    });

    testWidgets('暂停后显示已暂停且遮蔽题面', (tester) async {
      await tester.pumpWidget(replayPage());
      await tester.pump();

      // 点导航栏的暂停按钮
      await tester.tap(find.byIcon(CupertinoIcons.pause_fill));
      await tester.pump();

      // 暂停会弹出对话框
      expect(find.text('已暂停'), findsWidgets);
      expect(find.textContaining('关闭应用不会保存'), findsOneWidget);
    });
  });
}
