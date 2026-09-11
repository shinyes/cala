import 'package:cala/api/models.dart';
import 'package:cala/scoring/scoring.dart' as scoring;
import 'package:cala/scoring/scoring.dart' show Envelope;
import 'package:cala/state/practice.dart';
import 'package:flutter_test/flutter_test.dart';

/// 服务端下发的清洗表（真实形状，取自 P3.5 的语料）。
const _cleanupTable = <String, String>{
  '０': '0',
  '１': '1',
  '２': '2',
  '３': '3',
  '９': '9',
  '．': '.',
  '－': '-',
  '−': '-',
  ',': '',
  '\u3000': ' ',
  '\u00A0': ' ',
};

Question _q(int idx, String q, String a, {int num = 0, int den = 1}) =>
    Question(
      index: idx,
      q: q,
      a: a,
      envelope: num == 0 && den == 1
          ? Envelope.rational(a, '1')
          : Envelope.rational('$num', '$den'),
    );

/// 构造一轮状态机：[n] 道题，答案依次为 1..n。
PracticeState _round(int n, {List<Question>? questions}) => PracticeState(
      questions: questions ??
          List.generate(n, (i) => _q(i, '${i + 1} + ${i + 1} = ?', '${(i + 1) * 2}')),
      cleanupTable: _cleanupTable,
      tolerance: scoring.Tolerance.exact,
      seed: 42,
    );

/// 输入一串字符。
PracticeState _type(PracticeState s, String text) {
  var out = s;
  for (final ch in text.split('')) {
    out = PracticeMachine.appendChar(out, ch);
  }
  return out;
}

void main() {
  group('输入缓冲', () {
    test('追加字符', () {
      final s = _type(_round(3), '12');
      expect(s.input, '12');
    });

    test('退格删除末位', () {
      var s = _type(_round(3), '12');
      s = PracticeMachine.backspace(s);
      expect(s.input, '1');
    });

    test('空输入退格为 no-op', () {
      final s = PracticeMachine.backspace(_round(3));
      expect(s.input, '');
    });

    test('长度上限生效', () {
      final s = _type(_round(3), '1' * 50);
      expect(s.input.length, maxInputLen);
    });

    test('接受键盘字母表字符：0-9 . - /', () {
      final s = _type(_round(3), '0.5-1/2');
      expect(s.input, '0.5-1/2');
    });
  });

  group('提交与推进', () {
    test('答对后 awaitingNext 为 true 且 lastCorrect 为 true', () {
      final s = PracticeMachine.submit(_type(_round(3), '2'));
      expect(s.lastCorrect, isTrue);
      expect(s.awaitingNext, isTrue);
      expect(s.answers.length, 1);
      expect(s.answers.single.clientCorrect, isTrue);
    });

    test('答对后推进到下一题并清空输入', () {
      var s = PracticeMachine.submit(_type(_round(3), '2'));
      s = PracticeMachine.nextQuestion(s);
      expect(s.current, 1);
      expect(s.input, '');
      expect(s.lastCorrect, isNull);
      expect(s.awaitingNext, isFalse);
    });

    test('答错后停住等待「下一题」', () {
      final s = PracticeMachine.submit(_type(_round(3), '999'));
      expect(s.lastCorrect, isFalse);
      expect(s.awaitingNext, isTrue);
      // 关键：未推进
      expect(s.current, 0);
      expect(s.answers.single.clientCorrect, isFalse);
    });

    test('答错后按「下一题」才推进', () {
      var s = PracticeMachine.submit(_type(_round(3), '999'));
      expect(s.current, 0);
      s = PracticeMachine.nextQuestion(s);
      expect(s.current, 1);
      expect(s.awaitingNext, isFalse);
    });

    test('等待推进期间拒绝输入', () {
      final s = PracticeMachine.submit(_type(_round(3), '999'));
      expect(s.acceptsInput, isFalse);
      final after = PracticeMachine.appendChar(s, '7');
      expect(after.input, s.input, reason: '等待「下一题」时不应接受输入');
    });

    test('未答完时 nextQuestion 为 no-op', () {
      final s = PracticeMachine.nextQuestion(_round(3));
      expect(s.current, 0);
    });

    test('最后一题答对并推进后 finished', () {
      var s = _round(1);
      s = PracticeMachine.submit(_type(s, '2'));
      s = PracticeMachine.nextQuestion(s);
      expect(s.finished, isTrue);
      expect(s.acceptsInput, isFalse);
    });

    test('最后一题答错时先停住，推进后才 finished', () {
      var s = _round(1);
      s = PracticeMachine.submit(_type(s, '999'));
      expect(s.awaitingNext, isTrue);
      expect(s.finished, isFalse, reason: '答错后应先让用户看到正确答案');

      s = PracticeMachine.nextQuestion(s);
      expect(s.finished, isTrue);
    });

    test('结束后拒绝输入与提交', () {
      var s = _round(1);
      s = PracticeMachine.submit(_type(s, '2'));
      s = PracticeMachine.nextQuestion(s);

      expect(PracticeMachine.appendChar(s, '5').input, '');
      expect(PracticeMachine.submit(s).answers.length, s.answers.length);
    });
  });

  group('计时', () {
    test('tick 累加到当前题', () {
      var s = PracticeMachine.tick(_round(3), 1500);
      expect(s.questionElapsedMs, 1500);
    });

    test('提交时把当前题耗时计入总耗时', () {
      var s = PracticeMachine.tick(_round(3), 1200);
      s = PracticeMachine.submit(_type(s, '2'));
      expect(s.answers.single.elapsedMs, 1200);
      expect(s.totalElapsedMs, 1200);
    });

    test('推进后当前题耗时归零，总耗时保留', () {
      var s = PracticeMachine.tick(_round(3), 1200);
      s = PracticeMachine.submit(_type(s, '2'));
      s = PracticeMachine.nextQuestion(s);
      expect(s.questionElapsedMs, 0);
      expect(s.totalElapsedMs, 1200);
    });

    test('暂停期间 tick 被忽略（暂停确实停表）', () {
      var s = PracticeMachine.tick(_round(3), 1000);
      s = PracticeMachine.pause(s);
      s = PracticeMachine.tick(s, 5000);
      expect(s.questionElapsedMs, 1000, reason: '暂停期间不应累加计时');
      expect(s.paused, isTrue);
    });

    test('继续后恢复累加', () {
      var s = PracticeMachine.tick(_round(3), 1000);
      s = PracticeMachine.pause(s);
      s = PracticeMachine.tick(s, 5000);
      s = PracticeMachine.resume(s);
      s = PracticeMachine.tick(s, 300);
      expect(s.questionElapsedMs, 1300);
    });

    test('已结束时 tick 被忽略', () {
      var s = _round(1);
      s = PracticeMachine.submit(_type(s, '2'));
      s = PracticeMachine.nextQuestion(s);
      final after = PracticeMachine.tick(s, 9999);
      expect(after.questionElapsedMs, 0);
      expect(after.totalElapsedMs, s.totalElapsedMs);
    });

    test('非正 delta 被忽略', () {
      final s = PracticeMachine.tick(_round(3), -5);
      expect(s.questionElapsedMs, 0);
    });
  });

  group('暂停', () {
    test('暂停清空当前输入', () {
      var s = _type(_round(3), '12');
      s = PracticeMachine.pause(s);
      expect(s.input, '', reason: '暂停时清空输入，避免暂停期间继续作答');
      expect(s.paused, isTrue);
    });

    test('暂停期间拒绝输入', () {
      var s = PracticeMachine.pause(_round(3));
      expect(s.acceptsInput, isFalse);
      expect(PracticeMachine.appendChar(s, '5').input, '');
    });

    test('暂停期间拒绝提交', () {
      final s = PracticeMachine.pause(_round(3));
      expect(PracticeMachine.submit(s).answers, isEmpty);
    });

    test('重复暂停与无暂停继续均为 no-op', () {
      final p = PracticeMachine.pause(_round(3));
      expect(PracticeMachine.pause(p).paused, isTrue);
      final r = PracticeMachine.resume(_round(3));
      expect(r.paused, isFalse);
    });

    test('继续后状态保留（进度不丢）', () {
      var s = _round(3);
      s = PracticeMachine.submit(_type(s, '2'));
      s = PracticeMachine.nextQuestion(s);
      s = PracticeMachine.pause(s);
      s = PracticeMachine.resume(s);
      expect(s.current, 1);
      expect(s.answers.length, 1);
    });
  });

  group('判分经 scoring.compare（不得自写比较）', () {
    test('全角数字经服务端清洗表后判对', () {
      // 这一条是「UI 未自写比较逻辑」的关键证据：
      // 若在页面里写 `input == answer`，全角 ２ 不会等于半角 2，会判错。
      final s = PracticeState(
        questions: [_q(0, '1+1 = ?', '2')],
        cleanupTable: _cleanupTable,
        tolerance: scoring.Tolerance.exact,
        seed: 1,
      );
      final submitted = PracticeMachine.submit(_type(s, '２'));
      expect(submitted.lastCorrect, isTrue,
          reason: '全角２应经清洗表转为 2 后判对；判错说明未走 scoring.compare');
    });

    test('等价分数与小数判对', () {
      final s = PracticeState(
        questions: [
          Question(
            index: 0,
            q: '1/2 = ?',
            a: '0.5',
            envelope: const Envelope.rational('5', '10'),
          )
        ],
        cleanupTable: _cleanupTable,
        tolerance: scoring.Tolerance.exact,
        seed: 1,
      );
      expect(PracticeMachine.submit(_type(s, '1/2')).lastCorrect, isTrue);
    });

    test('容差生效', () {
      final s = PracticeState(
        questions: [
          Question(
            index: 0,
            q: 'pi?',
            a: '3.14159',
            envelope: const Envelope.rational('314159', '100000'),
          )
        ],
        cleanupTable: _cleanupTable,
        tolerance: scoring.Tolerance(BigInt.one, BigInt.from(100)),
        seed: 1,
      );
      expect(PracticeMachine.submit(_type(s, '3.14')).lastCorrect, isTrue);
    });

    test('未作答时提交判错', () {
      final s = PracticeMachine.submit(_round(3));
      expect(s.lastCorrect, isFalse);
      expect(s.answers.single.input, '');
    });

    test('信封缺失时判错而非假定正确', () {
      final s = PracticeState(
        questions: [
          const Question(index: 0, q: 'x', a: 'y', envelope: null),
        ],
        cleanupTable: _cleanupTable,
        tolerance: scoring.Tolerance.exact,
        seed: 1,
      );
      expect(PracticeMachine.submit(_type(s, 'y')).lastCorrect, isFalse,
          reason: '信封缺失时不应假定正确——那会让用户以为自己答对了');
    });

    test('文本答案判分', () {
      final s = PracticeState(
        questions: [
          const Question(
            index: 0,
            q: '2 是？',
            a: '质数',
            envelope: Envelope.text('质数'),
          )
        ],
        cleanupTable: _cleanupTable,
        tolerance: scoring.Tolerance.exact,
        seed: 1,
      );
      // 注意：键盘只有 0-9 . - /，文本答案无法通过键盘输入。
      // 这里直接用状态机验证判分路径本身正确（用于重练错题时回显）。
      var st = s.copyWith(input: '质数');
      st = PracticeMachine.submit(st);
      expect(st.lastCorrect, isTrue);
    });
  });

  group('交卷载荷', () {
    test('toAttempts 保留原始输入（不规范化）', () {
      var s = PracticeState(
        questions: [_q(0, '1+1 = ?', '2')],
        cleanupTable: _cleanupTable,
        tolerance: scoring.Tolerance.exact,
        seed: 1,
      );
      s = PracticeMachine.submit(_type(s, '２'));
      final attempts = PracticeMachine.toAttempts(s);
      expect(attempts.single.input, '２',
          reason: '落库应为用户原始输入，服务端据它独立重算');
      expect(attempts.single.index, 0);
      expect(attempts.single.clientIsCorrect, isTrue);
    });

    test('题号来自服务端下发的 idx', () {
      final questions = [
        _q(10, 'a', '1'),
        _q(11, 'b', '2'),
      ];
      var s = _round(0, questions: questions);
      s = PracticeMachine.submit(_type(s, '1'));
      s = PracticeMachine.nextQuestion(s);
      s = PracticeMachine.submit(_type(s, '2'));
      final attempts = PracticeMachine.toAttempts(s);
      expect(attempts.map((a) => a.index), [10, 11]);
    });
  });

  group('错题与重练', () {
    test('wrongAnswers 只含客户端判错的题', () {
      var s = _round(3);
      s = PracticeMachine.submit(_type(s, '2')); // 对
      s = PracticeMachine.nextQuestion(s);
      s = PracticeMachine.submit(_type(s, '999')); // 错
      s = PracticeMachine.nextQuestion(s);
      s = PracticeMachine.submit(_type(s, '6')); // 对

      expect(s.wrongAnswers.length, 1);
      expect(s.wrongAnswers.single.index, 1);
    });

    test('replayWrong 使用快照题目，不重新出题', () {
      final snapshot = [_q(0, '3 + 4 = ?', '7'), _q(1, '5 + 6 = ?', '11')];
      final s = PracticeMachine.replayWrong(
        snapshotQuestions: snapshot,
        cleanupTable: _cleanupTable,
        tolerance: scoring.Tolerance.exact,
      );
      expect(s.questions.length, 2);
      // 逐字一致：题面与答案来自快照
      expect(s.questions[0].q, '3 + 4 = ?');
      expect(s.questions[0].a, '7');
      expect(s.questions[1].q, '5 + 6 = ?');
      expect(s.current, 0);
      expect(s.answers, isEmpty);
    });
  });

  group('进度与显示', () {
    test('displayIndex 从 1 开始', () {
      final s = _round(5);
      expect(s.displayIndex, 1);
      expect(s.questionCount, 5);
    });

    test('progress 随作答推进', () {
      var s = _round(4);
      expect(s.progress, 0);
      s = PracticeMachine.submit(_type(s, '2'));
      expect(s.progress, closeTo(0.25, 1e-9));
    });

    test('currentQuestion 越界时为 null', () {
      final s = _round(1);
      final past = s.copyWith(current: 99);
      expect(past.currentQuestion, isNull);
    });
  });
}
