import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/client.dart';
import '../state/session.dart';

/// 修改口令页。
///
/// 校验规则的 owner 是**服务端**（口令长度下限、新旧是否相同），
/// 因此本页只做服务端**无法**检查的两件事：
///   1. 三个输入框都不为空；
///   2. 两次输入的新口令一致（确认框只存在于客户端，服务端看不到它）。
/// 其余一律交给服务端判断，并把它的错误消息原样展示 ——
/// 若在这里再实现一遍长度规则，规则就有了两个 owner，日后必然漂移。
class ChangePasswordPage extends ConsumerStatefulWidget {
  const ChangePasswordPage({super.key});

  @override
  ConsumerState<ChangePasswordPage> createState() => _ChangePasswordPageState();
}

class _ChangePasswordPageState extends ConsumerState<ChangePasswordPage> {
  final _current = TextEditingController();
  final _new = TextEditingController();
  final _confirm = TextEditingController();

  bool _busy = false;
  String? _error;

  @override
  void dispose() {
    _current.dispose();
    _new.dispose();
    _confirm.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (_busy) return;

    final current = _current.text;
    final next = _new.text;

    // 客户端能判断的两件事（见类注释）
    if (current.isEmpty || next.isEmpty) {
      setState(() => _error = '请填写当前口令与新口令');
      return;
    }
    if (next != _confirm.text) {
      setState(() => _error = '两次输入的新口令不一致');
      return;
    }

    setState(() {
      _busy = true;
      _error = null;
    });

    try {
      await ref.read(sessionProvider.notifier).changePassword(current, next);
      if (!mounted) return;

      // 明确告知「其他设备已被登出」：这是改密的安全效果，
      // 用户应当知道，否则在多设备场景下会以为是故障。
      await showCupertinoDialog<void>(
        context: context,
        builder: (ctx) => CupertinoAlertDialog(
          title: const Text('口令已修改'),
          content: const Text(
            '本机保持登录。\n\n'
            '其他设备上的登录已全部失效，需要用新口令重新登录。',
          ),
          actions: [
            CupertinoDialogAction(
              isDefaultAction: true,
              onPressed: () => Navigator.of(ctx).pop(),
              child: const Text('好'),
            ),
          ],
        ),
      );
      if (mounted) Navigator.of(context).pop();
    } on ApiException catch (e) {
      if (mounted) setState(() => _error = e.message);
    } on Object catch (e) {
      if (mounted) setState(() => _error = '$e');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return CupertinoPageScaffold(
      navigationBar: CupertinoNavigationBar(
        middle: const Text('修改口令'),
        trailing: CupertinoButton(
          padding: EdgeInsets.zero,
          onPressed: _busy ? null : _submit,
          child: const Text('保存'),
        ),
      ),
      child: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(20),
          children: [
            _Field(
              controller: _current,
              placeholder: '当前口令',
              textInputAction: TextInputAction.next,
            ),
            const SizedBox(height: 12),
            _Field(
              controller: _new,
              placeholder: '新口令',
              textInputAction: TextInputAction.next,
            ),
            const Padding(
              padding: EdgeInsets.only(top: 6, left: 4),
              child: Text(
                '至少 8 个字符。',
                style: TextStyle(fontSize: 12, color: CupertinoColors.tertiaryLabel),
              ),
            ),
            const SizedBox(height: 12),
            _Field(
              controller: _confirm,
              placeholder: '确认新口令',
              textInputAction: TextInputAction.done,
              onSubmitted: (_) => _submit(),
            ),

            if (_error != null) ...[
              const SizedBox(height: 14),
              Container(
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: CupertinoColors.systemRed.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Icon(CupertinoIcons.exclamationmark_circle,
                        color: CupertinoColors.systemRed, size: 18),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        _error!,
                        style: const TextStyle(
                            color: CupertinoColors.systemRed, fontSize: 14),
                      ),
                    ),
                  ],
                ),
              ),
            ],

            const SizedBox(height: 20),
            CupertinoButton.filled(
              onPressed: _busy ? null : _submit,
              child: _busy
                  ? const CupertinoActivityIndicator(radius: 8)
                  : const Text('修改口令'),
            ),

            const SizedBox(height: 24),
            const Text(
              '修改后，其他设备上的登录会全部失效 —— '
              '这正是怀疑账号被人登录时的处理办法。',
              style: TextStyle(fontSize: 12, color: CupertinoColors.secondaryLabel),
            ),
          ],
        ),
      ),
    );
  }
}

class _Field extends StatelessWidget {
  const _Field({
    required this.controller,
    required this.placeholder,
    this.textInputAction,
    this.onSubmitted,
  });

  final TextEditingController controller;
  final String placeholder;
  final TextInputAction? textInputAction;
  final ValueChanged<String>? onSubmitted;

  @override
  Widget build(BuildContext context) {
    return CupertinoTextField(
      controller: controller,
      placeholder: placeholder,
      obscureText: true,
      autocorrect: false,
      enableSuggestions: false,
      textInputAction: textInputAction,
      onSubmitted: onSubmitted,
      prefix: const Padding(
        padding: EdgeInsets.only(left: 12),
        child: Icon(CupertinoIcons.lock, size: 18),
      ),
    );
  }
}
