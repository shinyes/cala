/// 判分语义的客户端实现。
///
/// **本文件是 `backend/internal/scoring` 的镜像实现。**
///
/// 设计约束（规格 §5.5）：
///  - 判分路径**不出现浮点**：有理数一律用 [BigInt] 交叉相乘比较。
///  - 清洗规则**不由本文件定义**，而是应用服务端随 `/rounds/start` 下发的清洗表。
///    本文件只提供「应用那张表」的机制，不内置任何映射。
///  - 不使用任何 Unicode 归一化（NFKC 等）：Go 的 x/text 与 Dart 生态对应库
///    语义未必逐字一致，那是本项目最容易忽略的跨端漂移源。
///
/// 一致性由生成式语料钉死（≥10000 条边界向量），CI 门禁使分歧即构建失败。
/// 语料生成器命令见 `tool/gen_scoring_corpus.dart`。
library;

/// 信封种类。取值必须与 Go 侧 `scoring.KindRational` / `scoring.KindText` 一致。
class EnvelopeKind {
  static const rational = 'rational';
  static const text = 'text';
}

/// 答案长度上限（按 rune 计）。
///
/// 与 Go 侧 `scoring.MaxAnswerLen` 一致；由语料中的超长用例守护。
const int maxAnswerLen = 500;

/// 数值字母表：只由这些字符组成的答案被视为「作者想写数值」。
const String numericAlphabet = '0123456789+-./';

/// 作者答案的规范化形式，随 a_snapshot 一并落库，成为该次答题的判分依据。
class Envelope {
  final String kind;

  /// 仅在 [kind] == [EnvelopeKind.rational] 时有效，且 den > 0。
  final String num;
  final String den;

  /// 仅在 [kind] == [EnvelopeKind.text] 时有效，为规范化后的文本。
  final String value;

  const Envelope._({
    required this.kind,
    this.num = '',
    this.den = '',
    this.value = '',
  });

  const Envelope.rational(String num, String den)
      : this._(kind: EnvelopeKind.rational, num: num, den: den);

  const Envelope.text(String value)
      : this._(kind: EnvelopeKind.text, value: value);

  /// 从服务端下发的 JSON 构造。
  static Envelope fromJson(Map<String, dynamic> json) {
    final kind = json['kind'] as String? ?? '';
    switch (kind) {
      case EnvelopeKind.rational:
        return Envelope.rational(
          json['num'] as String? ?? '',
          json['den'] as String? ?? '',
        );
      case EnvelopeKind.text:
        return Envelope.text(json['value'] as String? ?? '');
      default:
        throw FormatException('未知的答案信封种类: $kind');
    }
  }

  /// 序列化为 JSON，便于与 Go 侧比对。
  Map<String, dynamic> toJson() {
    switch (kind) {
      case EnvelopeKind.rational:
        return {'kind': kind, 'num': num, 'den': den};
      case EnvelopeKind.text:
        return {'kind': kind, 'value': value};
      default:
        throw StateError('未知的答案信封种类: $kind');
    }
  }

  @override
  bool operator ==(Object other) =>
      other is Envelope &&
      other.kind == kind &&
      other.num == num &&
      other.den == den &&
      other.value == value;

  @override
  int get hashCode => Object.hash(kind, num, den, value);

  @override
  String toString() => 'Envelope(${toJson()})';
}

/// 分类失败的原因。与 Go 侧的报错条件一一对应。
class ClassifyException implements Exception {
  final String message;
  const ClassifyException(this.message);
  @override
  String toString() => message;
}

/// 精确有理数。`den` 恒为正。
class Rational {
  final BigInt num;
  final BigInt den;

  const Rational(this.num, this.den);

  /// 交叉相乘比较，全程整数（规格 §5.5.3）。
  int cmp(Rational o) => (num * o.den).compareTo(o.num * den);

  @override
  String toString() => den == BigInt.one ? '$num' : '$num/$den';
}

/// 严格解析有理数字面量。
///
/// 文法与 Go 侧 `scoring.ParseRational` 必须逐条一致：
///
///     number := sign? ( digits | digits '.' digits | '.' digits | digits '/' digits )
///     sign   := '+' | '-'
///
/// 分母必须非零。允许前导零。不接受科学计数法。
/// 解析失败时抛出 [FormatException]。
Rational parseRational(String s) {
  var t = s.trim();
  if (t.isEmpty) {
    throw const FormatException('空字符串');
  }

  var neg = false;
  if (t.startsWith('+')) {
    t = t.substring(1);
  } else if (t.startsWith('-')) {
    neg = true;
    t = t.substring(1);
  }
  if (t.isEmpty) {
    throw const FormatException('缺少数字');
  }

  Rational build(BigInt n, BigInt d) {
    if (d == BigInt.zero) {
      throw const FormatException('分母不能为零');
    }
    return Rational(neg ? -n : n, d);
  }

  // 整数
  if (_isAllDigits(t)) {
    return build(BigInt.parse(t), BigInt.one);
  }

  // 分数 a/b
  final slash = t.indexOf('/');
  if (slash >= 0) {
    final numPart = t.substring(0, slash);
    final denPart = t.substring(slash + 1);
    if (!_isAllDigits(numPart) || !_isAllDigits(denPart)) {
      throw const FormatException('分数格式应为 整数/整数');
    }
    return build(BigInt.parse(numPart), BigInt.parse(denPart));
  }

  // 小数
  final dot = t.indexOf('.');
  if (dot >= 0) {
    final intPart = t.substring(0, dot);
    final fracPart = t.substring(dot + 1);
    if (fracPart.isEmpty) {
      throw const FormatException('小数点后缺少数字');
    }
    if (!_isAllDigits(fracPart)) {
      throw const FormatException('小数点后应为数字');
    }
    if (intPart.isNotEmpty && !_isAllDigits(intPart)) {
      throw const FormatException('小数点前应为数字');
    }
    // 与 Go 侧一致：0.25 -> 25/100（不约分，保持表示形式可比对）
    final digits = intPart + fracPart;
    var den = BigInt.one;
    for (var i = 0; i < fracPart.length; i++) {
      den *= BigInt.from(10);
    }
    return build(BigInt.parse(digits), den);
  }

  throw const FormatException('既不是整数、小数，也不是分数');
}

bool _isAllDigits(String s) {
  if (s.isEmpty) return false;
  for (var i = 0; i < s.length; i++) {
    final c = s.codeUnitAt(i);
    if (c < 0x30 || c > 0x39) return false;
  }
  return true;
}

/// 应用服务端下发的清洗表，并去除首尾空白。
///
/// [table] 必须来自 `/rounds/start` 的 `scoring.cleanupTable`。
/// 本函数**只做表驱动的字符替换**，随后仅裁剪 ASCII 空白——
/// 不内置任何映射，也不做任何 Unicode 归一化。
///
/// 客户端不得自行硬编码清洗规则：那会让清洗规则出现第二个 owner，
/// 从而重新引入跨端漂移的可能（规格 §5.5.4）。
String clean(String s, Map<String, String> table) {
  final buf = StringBuffer();
  for (final rune in s.runes) {
    final ch = String.fromCharCode(rune);
    final repl = table[ch];
    if (repl != null) {
      buf.write(repl);
      continue;
    }
    buf.write(ch);
  }
  return _trimAsciiWhitespace(buf.toString());
}

/// 只裁剪 ASCII 空白字符（与 Go 侧 `strings.TrimSpace` 在清洗后的输入上等价，
/// 因为清洗表已把所有非 ASCII 空白转换为 ASCII 空格）。
String _trimAsciiWhitespace(String s) {
  var start = 0;
  var end = s.length;
  while (start < end && _isAsciiSpace(s.codeUnitAt(start))) {
    start++;
  }
  while (end > start && _isAsciiSpace(s.codeUnitAt(end - 1))) {
    end--;
  }
  return s.substring(start, end);
}

bool _isAsciiSpace(int c) =>
    c == 0x20 || c == 0x09 || c == 0x0A || c == 0x0D || c == 0x0B || c == 0x0C;

/// 规范化文本答案（规格 §5.5.2）：
///  1. 去除首尾空白
///  2. 内部连续空白折叠为单个 ASCII 空格
///  3. ASCII 大写 -> 小写（仅 ASCII）
///  4. 不做任何 Unicode 形式变换
String normalizeText(String s) {
  final buf = StringBuffer();
  var prevSpace = false;
  for (final rune in s.runes) {
    if (rune == 0x20) {
      if (prevSpace) continue;
      prevSpace = true;
      buf.write(' ');
      continue;
    }
    prevSpace = false;
    // 仅 ASCII 折叠：É 等字符保持原样
    if (rune >= 0x41 && rune <= 0x5A) {
      buf.writeCharCode(rune + 0x20);
      continue;
    }
    buf.writeCharCode(rune);
  }
  return _trimAsciiWhitespace(buf.toString());
}

/// 判断清洗后的字符串是否「只由数值字母表组成」。
bool _isNumericCandidate(String s) {
  if (s.isEmpty) return false;
  for (final rune in s.runes) {
    if (rune == 0x20) continue;
    if (!numericAlphabet.contains(String.fromCharCode(rune))) return false;
  }
  return true;
}

int _indexControl(String s) {
  final runes = s.runes.toList();
  for (var i = 0; i < runes.length; i++) {
    final r = runes[i];
    if (r < 0x20 || r == 0x7F) return i;
  }
  return -1;
}

/// 把作者返回的答案原文分类为规范信封。
///
/// 与 Go 侧 `scoring.Classify` 的条件顺序必须一致：
///  1. 空白 -> 报错
///  2. 超长 -> 报错
///  3. 含控制字符 -> 报错
///  4. 清洗后是数值候选：解析成功 -> rational；失败 -> **报错**（笔误）
///  5. 否则 -> text（规范化后）
///
/// 第 4 条的「失败即报错」是关键：静默降级为文本会让用户在答题时永远答不对。
Envelope classify(String raw, Map<String, String> table) {
  if (raw.trim().isEmpty) {
    throw const ClassifyException('答案不能为空');
  }
  if (raw.runes.length > maxAnswerLen) {
    throw ClassifyException(
        '答案过长（${raw.runes.length} 个字符，上限 $maxAnswerLen）');
  }
  final ctrl = _indexControl(raw);
  if (ctrl >= 0) {
    throw ClassifyException('答案含有控制字符（位置 $ctrl）');
  }

  final cleaned = clean(raw, table);

  if (_isNumericCandidate(cleaned)) {
    try {
      final r = parseRational(cleaned);
      return Envelope.rational(r.num.toString(), r.den.toString());
    } on FormatException catch (e) {
      throw ClassifyException(
          '答案看起来是数值但无法解析（${e.message}）；若要作为文本答案，请包含数值以外的字符');
    }
  }

  final v = normalizeText(cleaned);
  if (v.isEmpty) {
    throw const ClassifyException('答案不能为空');
  }
  return Envelope.text(v);
}

/// 项目级可选容差，表示为整数比 [num]/[den]。[num] 为 0 表示精确比较。
class Tolerance {
  final BigInt num;
  final BigInt den;

  const Tolerance(this.num, this.den);

  static final Tolerance exact = Tolerance(BigInt.zero, BigInt.one);

  bool get isZero => num == BigInt.zero;

  static Tolerance of(int num, int den) {
    if (den <= 0) {
      throw ArgumentError('容差分母必须为正，得到 $den');
    }
    if (num < 0) {
      throw ArgumentError('容差不能为负，得到 $num');
    }
    return Tolerance(BigInt.from(num), BigInt.from(den));
  }
}

/// 一次判分的结果。
class ComparisonResult {
  final bool correct;
  final String reason;
  const ComparisonResult(this.correct, this.reason);
}

/// 判定用户作答是否正确。
///
/// 判定规则（规格 §5.5.3），与 Go 侧 `scoring.Compare` 必须逐例一致：
///  - 文本答案：规范化后精确相等
///  - 有理数答案：|n1·d2 − n2·d1| · t_d  ≤  t_n · d₁ · d₂
///  - 作答无法解析为数值时判为错误（不容差兜底）
///
/// [table] 必须来自服务端下发的清洗表。
ComparisonResult compare(
  String userInput,
  Envelope env,
  Tolerance tol,
  Map<String, String> table,
) {
  final cleaned = clean(userInput, table);
  if (cleaned.trim().isEmpty) {
    return const ComparisonResult(false, '未作答');
  }

  switch (env.kind) {
    case EnvelopeKind.text:
      final got = normalizeText(cleaned);
      if (got == env.value) {
        return const ComparisonResult(true, '文本相等');
      }
      return ComparisonResult(false, '文本不等: 期望 "${env.value}" 得到 "$got"');

    case EnvelopeKind.rational:
      Rational got;
      try {
        got = parseRational(cleaned);
      } on FormatException catch (e) {
        return ComparisonResult(false, '作答无法解析为数值: ${e.message}');
      }

      Rational want;
      try {
        want = Rational(BigInt.parse(env.num), BigInt.parse(env.den));
        if (want.den == BigInt.zero) {
          throw const FormatException('den 为零');
        }
      } catch (e) {
        return ComparisonResult(false, '正确答案信封损坏: $e');
      }

      if (tol.isZero) {
        if (got.cmp(want) == 0) {
          return const ComparisonResult(true, '数值精确相等');
        }
        return ComparisonResult(false, '数值不等: 期望 $want 得到 $got');
      }

      // |n1*d2 - n2*d1| * t_d <= t_n * d1 * d2
      final lhs =
          ((got.num * want.den) - (want.num * got.den)).abs() * tol.den;
      final rhs = tol.num * got.den * want.den;

      if (lhs <= rhs) {
        return ComparisonResult(true, '在容差内: 期望 $want 得到 $got');
      }
      return ComparisonResult(false, '超出容差: 期望 $want 得到 $got');

    default:
      return ComparisonResult(false, '未知的信封种类 ${env.kind}');
  }
}
