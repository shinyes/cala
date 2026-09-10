import 'package:cala/scoring/scoring.dart';
import 'package:flutter_test/flutter_test.dart';

import 'corpus_generated.dart';

/// 跨端判分一致性门禁（规格 §5.5.5）。
///
/// 语料由 Go 侧 `internal/scoring` 生成（见 `backend/cmd/scorpus`），
/// Dart 侧实现必须**逐例一致**。任一分歧即本测试失败，进而 CI 失败。
///
/// 这是「界面显示答对、落库记为错」这一风险面的最终验收（验收标准 A12）。
void main() {
  group('语料规模', () {
    test('语料数量满足门禁下限', () {
      final total = corpusClassify.length + corpusCompare.length;
      expect(total, greaterThanOrEqualTo(10000),
          reason: '语料不足 10000 条，门禁强度不够');
    });

    test('清洗表已下发且非空', () {
      expect(corpusCleanupTable, isNotEmpty);
      expect(corpusCleanupTable.length, greaterThanOrEqualTo(30));
    });
  });

  group('A12 分类一致性', () {
    test('每条分类语料与 Go 侧一致', () {
      final failures = <String>[];

      for (final v in corpusClassify) {
        try {
          final got = classify(v.input, corpusCleanupTable);
          if (v.expectError) {
            failures.add('输入 ${_q(v.input)}: Go 侧报错，Dart 侧却返回 '
                '${got.kind}/${got.num}/${got.den}/${_q(got.value)}');
            continue;
          }
          if (got.kind != v.kind ||
              got.num != v.num ||
              got.den != v.den ||
              got.value != v.value) {
            failures.add('输入 ${_q(v.input)}: '
                'Go=(${v.kind},${v.num},${v.den},${_q(v.value)}) '
                'Dart=(${got.kind},${got.num},${got.den},${_q(got.value)})');
          }
        } on ClassifyException {
          if (!v.expectError) {
            failures.add('输入 ${_q(v.input)}: Go 侧接受（${v.kind}），Dart 侧却报错');
          }
        } catch (e) {
          failures.add('输入 ${_q(v.input)}: Dart 侧抛出非预期异常 $e');
        }
      }

      _report(failures, corpusClassify.length);
    });
  });

  group('A12 判分一致性', () {
    test('每条比较语料与 Go 侧一致', () {
      final failures = <String>[];

      for (final v in corpusCompare) {
        // 答案信封按 Go 侧的分类结果重建：
        // 重新分类一次以复用同一实现，避免在语料里重复存信封。
        Envelope env;
        try {
          env = classify(v.answer, corpusCleanupTable);
        } on ClassifyException catch (e) {
          failures.add('答案 ${_q(v.answer)} 在 Dart 侧无法分类: $e');
          continue;
        }

        final tol = v.tolNum == 0
            ? Tolerance.exact
            : Tolerance(BigInt.from(v.tolNum), BigInt.from(v.tolDen));

        final got = compare(v.input, env, tol, corpusCleanupTable);
        if (got.correct != v.correct) {
          failures.add('作答 ${_q(v.input)} 对答案 ${_q(v.answer)} '
              '容差 ${v.tolNum}/${v.tolDen}: '
              'Go=${v.correct} Dart=${got.correct} (${got.reason})');
        }
      }

      _report(failures, corpusCompare.length);
    });
  });

  group('A13 判分路径无浮点', () {
    // 这两个用例在 float64 下会得出「相等」，只有精确有理数比较才能区分。
    // 若有人把 Dart 实现改回 double，这里会立即失败。
    test('2^53 与 2^53+1 必须不等', () {
      final env = classify('9007199254740993', corpusCleanupTable);
      final got = compare('9007199254740992', env, Tolerance.exact, corpusCleanupTable);
      expect(got.correct, isFalse,
          reason: '2^53 与 2^53+1 在 float64 下相等；判为正确说明用了浮点');
    });

    test('1/3 与 0.3333333333333333 精确比较必须不等', () {
      final env = classify('1/3', corpusCleanupTable);
      final got =
          compare('0.3333333333333333', env, Tolerance.exact, corpusCleanupTable);
      expect(got.correct, isFalse,
          reason: '两者在 float64 下相等；判为正确说明用了浮点');
    });

    test('同一对值在足够小的显式容差下算对', () {
      final env = classify('1/3', corpusCleanupTable);
      final tol = Tolerance(BigInt.one, BigInt.parse('10000000000000000'));
      final got = compare('0.3333333333333333', env, tol, corpusCleanupTable);
      expect(got.correct, isTrue, reason: '这正是容差存在的意义');
    });

    test('大整数映射为 BigInt 而非 double', () {
      // 若实现用了 double，这个值会被舍入
      final env = classify('9007199254740993', corpusCleanupTable);
      expect(env.num, '9007199254740993');
      expect(env.den, '1');
    });
  });

  group('清洗规则不由客户端定义', () {
    test('空清洗表时不做任何替换', () {
      // 证明客户端没有内置映射：表为空则清洗是恒等（除首尾空白）
      final got = classify('１２３', const <String, String>{});
      // 全角数字不含数值字母表以外…实际上全角数字不在 numericAlphabet 中，
      // 因此会被当作文本；关键是它**没有**被转成 123。
      expect(got.kind, EnvelopeKind.text,
          reason: '空清洗表下 １２３ 不应被转成数值，说明客户端未内置映射');
    });

    test('服务端下发的表生效后 １２３ 变为数值', () {
      final got = classify('１２３', corpusCleanupTable);
      expect(got.kind, EnvelopeKind.rational);
      expect(got.num, '123');
    });

    test('清洗是纯表驱动：自定义表也能生效', () {
      // 用一张自定义表把字母 X 映射为 5，验证客户端确实只应用数据
      final table = <String, String>{'X': '5'};
      final got = classify('X', table);
      expect(got.kind, EnvelopeKind.rational);
      expect(got.num, '5');
    });
  });
}

/// 汇总并报告失败，避免上万条语料各抛一个异常。
void _report(List<String> failures, int total) {
  if (failures.isEmpty) {
    // 显式断言通过条数，使「语料被清空」这类问题也能被发现
    expect(total, greaterThan(0));
    return;
  }
  final shown = failures.take(20).join('\n  ');
  fail('${failures.length}/$total 条语料与 Go 侧不一致：\n  $shown'
      '${failures.length > 20 ? '\n  …还有 ${failures.length - 20} 条' : ''}');
}

/// 把字符串渲染成可读形式，不可见字符转义显示。
String _q(String s) {
  final b = StringBuffer('"');
  for (final r in s.runes) {
    if (r < 0x20 || r == 0x7F || r > 0x7E) {
      b.write('\\u{${r.toRadixString(16).toUpperCase()}}');
    } else {
      b.writeCharCode(r);
    }
  }
  b.write('"');
  return b.toString();
}
