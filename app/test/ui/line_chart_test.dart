import 'package:cala/ui/widgets/line_chart.dart';
import 'package:flutter/cupertino.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('yRange 边界（最易除零的地方）', () {
    test('空输入不产生 NaN', () {
      final (lo, hi) = LineChartPainter.yRange(const []);
      expect(lo.isNaN, isFalse);
      expect(hi.isNaN, isFalse);
      expect(hi, greaterThan(lo), reason: '跨度必须为正，否则绘制会除以 0');
    });

    test('单元素给出非零跨度', () {
      final (lo, hi) = LineChartPainter.yRange(const [1000]);
      expect(hi, greaterThan(lo));
      expect(lo, lessThan(1000));
      expect(hi, greaterThan(1000));
    });

    test('全等值给出非零跨度', () {
      final (lo, hi) = LineChartPainter.yRange(const [500, 500, 500]);
      expect(hi, greaterThan(lo), reason: '全等值时跨度为 0 会导致除零');
      // 该值应落在范围中间
      expect((lo + hi) / 2, closeTo(500, 1e-9));
    });

    test('全为 0 时跨度非零', () {
      final (lo, hi) = LineChartPainter.yRange(const [0, 0]);
      expect(hi, greaterThan(lo));
    });

    test('一般数据上下各留余量', () {
      final (lo, hi) = LineChartPainter.yRange(const [1000, 2000, 3000]);
      expect(lo, lessThan(1000));
      expect(hi, greaterThan(3000));
    });

    test('正确率量级（0..1）也有合理跨度', () {
      final (lo, hi) = LineChartPainter.yRange(const [0.0, 1.0, 0.5]);
      expect(hi, greaterThan(lo));
      expect(hi - lo, greaterThan(1.0));
    });
  });

  group('LineChart 渲染', () {
    Widget wrap(Widget child) => CupertinoApp(
          home: CupertinoPageScaffold(
            child: Center(child: SizedBox(width: 320, child: child)),
          ),
        );

    testWidgets('无数据时显示占位文案而非空白', (tester) async {
      await tester.pumpWidget(wrap(
        const LineChart(points: [], color: CupertinoColors.systemBlue),
      ));
      expect(find.text('还没有可绘制的数据'), findsOneWidget);
    });

    testWidgets('单点不抛异常', (tester) async {
      await tester.pumpWidget(wrap(
        const LineChart(
          points: [ChartPoint('周一', 1200)],
          color: CupertinoColors.systemBlue,
        ),
      ));
      await tester.pump();
      expect(tester.takeException(), isNull);
    });

    testWidgets('多点不抛异常', (tester) async {
      await tester.pumpWidget(wrap(
        const LineChart(
          points: [
            ChartPoint('9/1', 1000),
            ChartPoint('9/2', 2000),
            ChartPoint('9/3', 1500),
          ],
          color: CupertinoColors.systemGreen,
        ),
      ));
      await tester.pump();
      expect(tester.takeException(), isNull);
    });

    testWidgets('全等值不抛异常（除零防护）', (tester) async {
      await tester.pumpWidget(wrap(
        const LineChart(
          points: [
            ChartPoint('a', 500),
            ChartPoint('b', 500),
            ChartPoint('c', 500),
          ],
          color: CupertinoColors.systemBlue,
        ),
      ));
      await tester.pump();
      expect(tester.takeException(), isNull);
    });

    testWidgets('大量点不抛异常（标签抽稀路径）', (tester) async {
      await tester.pumpWidget(wrap(LineChart(
        points: List.generate(
          60,
          (i) => ChartPoint('d$i', (i * 137 % 900 + 100).toDouble()),
        ),
        color: CupertinoColors.systemBlue,
      )));
      await tester.pump();
      expect(tester.takeException(), isNull);
    });

    testWidgets('极小尺寸不抛异常', (tester) async {
      await tester.pumpWidget(CupertinoApp(
        home: CupertinoPageScaffold(
          child: SizedBox(
            width: 20,
            height: 20,
            child: LineChart(
              height: 20,
              points: const [ChartPoint('a', 1), ChartPoint('b', 2)],
              color: CupertinoColors.systemBlue,
            ),
          ),
        ),
      ));
      await tester.pump();
      expect(tester.takeException(), isNull);
    });

    testWidgets('valueFormatter 被用于 y 轴标签', (tester) async {
      final calls = <double>[];
      await tester.pumpWidget(wrap(LineChart(
        points: const [ChartPoint('a', 1000), ChartPoint('b', 2000)],
        color: CupertinoColors.systemBlue,
        valueFormatter: (v) {
          calls.add(v);
          return '${(v / 1000).toStringAsFixed(1)}s';
        },
      )));
      await tester.pump();

      // 三条网格线各调用一次
      expect(calls.length, 3);
      expect(tester.takeException(), isNull);
    });
  });
}
