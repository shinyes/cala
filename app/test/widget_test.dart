import 'package:cala/main.dart';
import 'package:flutter/cupertino.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('应用可挂载并渲染 Cupertino 骨架', (WidgetTester tester) async {
    await tester.pumpWidget(const CalaApp());

    // 确认使用 Cupertino 而非 Material（D10）
    expect(find.byType(CupertinoApp), findsOneWidget);
    expect(find.byType(CupertinoPageScaffold), findsOneWidget);

    // 导航栏与应用名
    expect(find.text('Cala'), findsOneWidget);
    expect(find.text('速算练习'), findsOneWidget);
  });
}
