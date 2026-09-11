import 'package:flutter/cupertino.dart';
import 'package:flutter/services.dart';

/// 自带数字键盘（功能 6）。
///
/// 字母表 = `0-9` `.` `-` 与 `⌫`、确定。
/// 该表与判分清洗逻辑是两件事：键盘决定**能输入什么**，
/// 清洗表决定**如何解释输入**（后者由服务端下发，见规格 §5.5.4）。
///
/// **为什么不含 `/`**：分数答案不需要单独按键 ——
/// 判分是有理数比较（交叉相乘），`3/4` 与 `0.75` 判为相等，
/// 因此**能写成有限小数的分数**都能直接输入。
///
/// 唯一的例外是**无限循环小数**（如答案 `1/3`）：它没有有限小数写法，
/// 去掉 `/` 后无法精确输入。这类题目必须配置容差
/// （例如容差 `1/100`，接受 `0.33`）。作者应在这一点上被明确提示，
/// 否则会做出「永远答不对」的题。
///
/// **为什么含 `-`**：速算练习常有负数结果（规格 §9.3）。
///
/// 开发期同时接受物理键盘输入（浏览器调试用），见 [PracticeKeyHandler]。
class Keypad extends StatelessWidget {
  const Keypad({
    super.key,
    required this.onDigit,
    required this.onBackspace,
    required this.onSubmit,
    this.enabled = true,
    this.submitLabel = '确定',
  });

  final ValueChanged<String> onDigit;
  final VoidCallback onBackspace;
  final VoidCallback onSubmit;
  final bool enabled;
  final String submitLabel;

  /// 三列布局。分两段声明而不是「按列数猜最后一行的结构」：
  /// 后者会让「确定」的占宽依赖行长度这一巧合，改动布局时容易静默错位。
  static const _digitRows = <List<String>>[
    ['1', '2', '3'],
    ['4', '5', '6'],
    ['7', '8', '9'],
    // 负号置于最左：与书写习惯一致（先写符号再写数字），
    // 且与 0 和 . 相邻，输入 -0.5 这类值时手指不用来回移动。
    ['-', '0', '.'],
  ];

  @override
  Widget build(BuildContext context) {
    return Container(
      color: CupertinoColors.secondarySystemBackground.resolveFrom(context),
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 8),
      child: SafeArea(
        top: false,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            for (final row in _digitRows) _row(row),
            // 最后一行：退格占 1 列，确定占 2 列
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 3),
              child: Row(
                children: [
                  Expanded(child: _cell('⌫', destructive: true)),
                  Expanded(
                    flex: 2,
                    child: _cell(submitLabel, primary: true, isSubmit: true),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _row(List<String> keys) => Padding(
        padding: const EdgeInsets.symmetric(vertical: 3),
        child: Row(
          children: [for (final k in keys) Expanded(child: _cell(k))],
        ),
      );

  Widget _cell(
    String label, {
    bool primary = false,
    bool destructive = false,
    // 参数名刻意不叫 onSubmit：那会遮蔽同名的 VoidCallback 字段，
    // 于是 `onSubmit()` 变成调用一个 bool。
    bool isSubmit = false,
  }) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 3),
      child: _KeyButton(
        label: label,
        enabled: enabled,
        primary: primary,
        destructive: destructive,
        onTap: () {
          if (isSubmit) {
            onSubmit();
          } else if (label == '⌫') {
            onBackspace();
          } else {
            onDigit(label);
          }
        },
      ),
    );
  }
}

class _KeyButton extends StatelessWidget {
  const _KeyButton({
    required this.label,
    required this.onTap,
    this.enabled = true,
    this.primary = false,
    this.destructive = false,
  });

  final String label;
  final VoidCallback onTap;
  final bool enabled;
  final bool primary;
  final bool destructive;

  @override
  Widget build(BuildContext context) {
    final bg = primary
        ? CupertinoColors.systemBlue
        : CupertinoColors.tertiarySystemFill.resolveFrom(context);
    final fg = primary
        ? CupertinoColors.white
        : destructive
            ? CupertinoColors.systemRed.resolveFrom(context)
            : CupertinoColors.label.resolveFrom(context);

    return CupertinoButton(
      padding: EdgeInsets.zero,
      borderRadius: BorderRadius.circular(10),
      color: bg,
      onPressed: enabled ? onTap : null,
      child: SizedBox(
        height: 48,
        child: Center(
          child: Text(
            label,
            style: TextStyle(
              fontSize: primary ? 17 : 22,
              fontWeight: FontWeight.w500,
              color: fg,
            ),
          ),
        ),
      ),
    );
  }
}

/// 物理键盘适配（开发期浏览器调试用）。
///
/// 只接收键盘字母表内的字符；回车等同「确定」，退格等同「⌫」。
/// 生产环境（手机）不会有物理键盘，因此这是纯增益、不影响设计。
class PracticeKeyHandler extends StatelessWidget {
  const PracticeKeyHandler({
    super.key,
    required this.child,
    required this.onChar,
    required this.onBackspace,
    required this.onSubmit,
    this.enabled = true,
  });

  final Widget child;
  final ValueChanged<String> onChar;
  final VoidCallback onBackspace;
  final VoidCallback onSubmit;
  final bool enabled;

  /// 与 [Keypad] 的字母表保持一致。
  ///
  /// 刻意**不**接受 `/`：界面上打不出的字符，物理键盘也不应能输入，
  /// 否则「开发期能答、手机上答不了」会掩盖问题。
  static const _allowed = '0123456789.-';

  @override
  Widget build(BuildContext context) {
    return Focus(
      autofocus: true,
      onKeyEvent: (node, event) {
        if (!enabled) return KeyEventResult.ignored;
        if (event is! KeyDownEvent) return KeyEventResult.ignored;

        final key = event.logicalKey;

        if (key == LogicalKeyboardKey.backspace) {
          onBackspace();
          return KeyEventResult.handled;
        }
        if (key == LogicalKeyboardKey.enter ||
            key == LogicalKeyboardKey.numpadEnter) {
          onSubmit();
          return KeyEventResult.handled;
        }

        final ch = event.character;
        if (ch != null && ch.length == 1 && _allowed.contains(ch)) {
          onChar(ch);
          return KeyEventResult.handled;
        }
        return KeyEventResult.ignored;
      },
      child: child,
    );
  }
}
