import 'package:flutter/cupertino.dart';
import 'package:flutter/services.dart';

/// 自带数字键盘（功能 6）。
///
/// 字母表与规格 §5.5.4 一致：`0-9 . - /`。该表与判分清洗逻辑是两件事：
/// 键盘决定**能输入什么**，清洗表决定**如何解释输入**。前者是界面约束，
/// 后者由服务端下发。
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

  @override
  Widget build(BuildContext context) {
    // 4 列布局：1 2 3 4 / 5 6 7 8 / 9 0 . - / / ⌫ 确定(宽)
    const rows = <List<String>>[
      ['1', '2', '3', '4'],
      ['5', '6', '7', '8'],
      ['9', '0', '.', '-'],
      ['/', '⌫'],
    ];

    return Container(
      color: CupertinoColors.secondarySystemBackground.resolveFrom(context),
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 8),
      child: SafeArea(
        top: false,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            for (final row in rows)
              Padding(
                padding: const EdgeInsets.symmetric(vertical: 3),
                child: Row(
                  children: [
                    for (final key in row)
                      Expanded(
                        child: Padding(
                          padding: const EdgeInsets.symmetric(horizontal: 3),
                          child: _KeyButton(
                            label: key,
                            enabled: enabled,
                            destructive: key == '⌫',
                            onTap: () {
                              if (key == '⌫') {
                                onBackspace();
                              } else {
                                onDigit(key);
                              }
                            },
                          ),
                        ),
                      ),
                    // 最后一行的「确定」占两列宽
                    if (row.length == 2)
                      Expanded(
                        flex: 2,
                        child: Padding(
                          padding: const EdgeInsets.symmetric(horizontal: 3),
                          child: _KeyButton(
                            label: submitLabel,
                            enabled: enabled,
                            primary: true,
                            onTap: onSubmit,
                          ),
                        ),
                      ),
                  ],
                ),
              ),
          ],
        ),
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

  static const _allowed = '0123456789.-/';

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
