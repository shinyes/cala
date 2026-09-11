import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/client.dart';
import '../state/session.dart';

/// 登录与注册页。
///
/// 注册入口的显隐由服务端给出的**有效值**决定（规格 §6.2）：
/// 关闭注册且系统已有用户时隐藏；系统尚无用户时服务端会把有效值算成 true，
/// 因此全新部署不会因开关默认关闭而无法创建管理员。
class AuthPage extends ConsumerStatefulWidget {
  const AuthPage({super.key});

  @override
  ConsumerState<AuthPage> createState() => _AuthPageState();
}

class _AuthPageState extends ConsumerState<AuthPage> {
  final _username = TextEditingController();
  final _password = TextEditingController();

  bool _registerMode = false;
  bool _busy = false;
  String? _error;

  @override
  void dispose() {
    _username.dispose();
    _password.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (_busy) return;
    final username = _username.text.trim();
    final password = _password.text;

    if (username.isEmpty || password.isEmpty) {
      setState(() => _error = '请填写用户名与口令');
      return;
    }

    setState(() {
      _busy = true;
      _error = null;
    });

    final session = ref.read(sessionProvider.notifier);
    try {
      if (_registerMode) {
        final becameAdmin = await session.register(username, password);
        if (!mounted) return;
        if (becameAdmin) {
          await _showDialog('已创建管理员',
              '你是本系统的第一个用户，已被设为管理员。可在「我的」中开关注册。');
        }
      } else {
        await session.login(username, password);
      }
      // 成功后无需跳转：CalaApp 依据 isLoggedIn 切换根页面
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() => _error = e.message);
    } on Object catch (e) {
      if (!mounted) return;
      setState(() => _error = '$e');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _showDialog(String title, String message) {
    return showCupertinoDialog<void>(
      context: context,
      builder: (ctx) => CupertinoAlertDialog(
        title: Text(title),
        content: Text(message),
        actions: [
          CupertinoDialogAction(
            isDefaultAction: true,
            onPressed: () => Navigator.of(ctx).pop(),
            child: const Text('好'),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final session = ref.watch(sessionProvider);

    return CupertinoPageScaffold(
      navigationBar: const CupertinoNavigationBar(middle: Text('Cala')),
      child: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(24),
          children: [
            const SizedBox(height: 24),
            const Text(
              '速算练习',
              textAlign: TextAlign.center,
              style: TextStyle(fontSize: 28, fontWeight: FontWeight.w600),
            ),
            const SizedBox(height: 40),

            CupertinoTextField(
              controller: _username,
              placeholder: '用户名',
              autocorrect: false,
              enableSuggestions: false,
              textInputAction: TextInputAction.next,
              prefix: const Padding(
                padding: EdgeInsets.only(left: 12),
                child: Icon(CupertinoIcons.person, size: 18),
              ),
            ),
            const SizedBox(height: 12),
            CupertinoTextField(
              controller: _password,
              placeholder: '口令',
              obscureText: true,
              autocorrect: false,
              enableSuggestions: false,
              onSubmitted: (_) => _submit(),
              prefix: const Padding(
                padding: EdgeInsets.only(left: 12),
                child: Icon(CupertinoIcons.lock, size: 18),
              ),
            ),

            if (_error != null) ...[
              const SizedBox(height: 16),
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
                          color: CupertinoColors.systemRed,
                          fontSize: 14,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ],

            const SizedBox(height: 24),
            CupertinoButton.filled(
              onPressed: _busy ? null : _submit,
              child: _busy
                  ? const CupertinoActivityIndicator(radius: 8)
                  : Text(_registerMode ? '注册' : '登录'),
            ),

            // 注册入口：仅当服务端说「现在可以注册」时显示。
            // bootstrap 为 true 时额外提示将成为管理员。
            if (session.showRegistration) ...[
              const SizedBox(height: 12),
              CupertinoButton(
                onPressed: _busy
                    ? null
                    : () => setState(() {
                          _registerMode = !_registerMode;
                          _error = null;
                        }),
                child: Text(_registerMode ? '已有账号？去登录' : '没有账号？去注册'),
              ),
              if (_registerMode && session.bootstrap)
                const Padding(
                  padding: EdgeInsets.symmetric(horizontal: 16),
                  child: Text(
                    '系统尚无用户：注册后你将成为管理员。',
                    textAlign: TextAlign.center,
                    style: TextStyle(
                      fontSize: 13,
                      color: CupertinoColors.secondaryLabel,
                    ),
                  ),
                ),
            ],
          ],
        ),
      ),
    );
  }
}
