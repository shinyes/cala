import 'package:flutter/cupertino.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/models.dart';
import '../api/subscription_api.dart';
import '../state/subscriptions.dart';
import 'shell.dart' show describeError;

/// 分享链接管理页（功能 5）。
///
/// 仅项目作者可进入（调用方负责限制入口）。
///
/// 「重置」会使旧链接立即失效——这是撤销分享给别人用的能力，
/// 因此需要二次确认并说清后果。
class SharePage extends ConsumerStatefulWidget {
  const SharePage({super.key, required this.project});

  final Project project;

  @override
  ConsumerState<SharePage> createState() => _SharePageState();
}

class _SharePageState extends ConsumerState<SharePage> {
  ShareLink? _link;
  bool _busy = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    // 项目上已有 token 时先展示（避免用户以为没分享过）。
    // 完整链接里含服务器地址，只有在生成/重置时后端才知道当前 Host，
    // 因此此时先显示 token，用户点「生成」即可拿到完整链接。
    final existing = widget.project.shareToken;
    if (existing != null && existing.isNotEmpty) {
      _link = ShareLink(token: existing, link: '');
    }
  }

  Future<void> _generate() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final link = await ref.read(subscriptionProvider.notifier).share(widget.project.id);
      if (!mounted) return;
      setState(() => _link = link);
    } on Object catch (e) {
      if (!mounted) return;
      setState(() => _error = describeError(e));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _confirmRegenerate() async {
    final ok = await showCupertinoDialog<bool>(
      context: context,
      builder: (ctx) => CupertinoAlertDialog(
        title: const Text('重置分享链接'),
        content: const Text(
          '生成新链接后，**旧链接立即失效**，已用它订阅的人不受影响（他们已订阅）。\n\n'
          '把新链接发给需要订阅的人即可。',
        ),
        actions: [
          CupertinoDialogAction(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: const Text('取消'),
          ),
          CupertinoDialogAction(
            isDestructiveAction: true,
            onPressed: () => Navigator.of(ctx).pop(true),
            child: const Text('重置'),
          ),
        ],
      ),
    );
    if (ok == true) await _generate();
  }

  Future<void> _confirmUnshare() async {
    final ok = await showCupertinoDialog<bool>(
      context: context,
      builder: (ctx) => CupertinoAlertDialog(
        title: const Text('撤销分享'),
        content: const Text(
          '撤销后链接立即失效，其他人无法再通过它订阅。\n'
          '已订阅的人不受影响。',
        ),
        actions: [
          CupertinoDialogAction(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: const Text('取消'),
          ),
          CupertinoDialogAction(
            isDestructiveAction: true,
            onPressed: () => Navigator.of(ctx).pop(true),
            child: const Text('撤销'),
          ),
        ],
      ),
    );
    if (ok != true) return;

    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await ref.read(subscriptionProvider.notifier).unshare(widget.project.id);
      if (!mounted) return;
      setState(() => _link = null);
    } on Object catch (e) {
      if (!mounted) return;
      setState(() => _error = describeError(e));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _copy() async {
    final text = _link?.link.isNotEmpty == true ? _link!.link : _link?.token ?? '';
    if (text.isEmpty) return;
    await Clipboard.setData(ClipboardData(text: text));
    if (!mounted) return;
    await showCupertinoDialog<void>(
      context: context,
      builder: (ctx) => CupertinoAlertDialog(
        title: const Text('已复制'),
        content: const Text('把链接发给需要订阅的人即可。'),
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

  bool get _hasLink => _link != null && _link!.token.isNotEmpty;

  @override
  Widget build(BuildContext context) {
    final project = widget.project;

    return CupertinoPageScaffold(
      navigationBar: CupertinoNavigationBar(middle: Text('分享「${project.title}」')),
      child: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            const Text(
              '订阅者只能查看与练习，不能修改这个项目。\n'
              '你之后修改项目配置，所有订阅者都会同步看到新版本。',
              style: TextStyle(fontSize: 13, color: CupertinoColors.secondaryLabel, height: 1.5),
            ),
            const SizedBox(height: 20),

            if (_error != null) ...[
              Container(
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: CupertinoColors.systemRed.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(10),
                ),
                child: Text(_error!,
                    style: const TextStyle(
                        fontSize: 13, color: CupertinoColors.systemRed)),
              ),
              const SizedBox(height: 16),
            ],

            if (!_hasLink)
              const Padding(
                padding: EdgeInsets.symmetric(vertical: 20),
                child: Text(
                  '尚未分享。点击下方按钮生成链接。',
                  textAlign: TextAlign.center,
                  style: TextStyle(color: CupertinoColors.secondaryLabel),
                ),
              )
            else ...[
              Container(
                padding: const EdgeInsets.all(14),
                decoration: BoxDecoration(
                  color: CupertinoColors.tertiarySystemFill.resolveFrom(context),
                  borderRadius: BorderRadius.circular(12),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Text('分享链接',
                        style: TextStyle(
                            fontSize: 12, color: CupertinoColors.secondaryLabel)),
                    const SizedBox(height: 6),
                    // 不使用 Material 的 SelectableText / SelectionArea（D10）；
                    // 文本已可通过「复制链接」按钮取走，无需长按选择。
                    Text(
                      _link!.link.isNotEmpty ? _link!.link : _link!.token,
                      style: const TextStyle(fontFamily: 'monospace', fontSize: 12),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 12),
              CupertinoButton.filled(
                onPressed: _busy ? null : _copy,
                child: const Text('复制链接'),
              ),
            ],

            const SizedBox(height: 12),
            if (!_hasLink)
              CupertinoButton.filled(
                onPressed: _busy ? null : _generate,
                child: _busy
                    ? const CupertinoActivityIndicator(radius: 8)
                    : const Text('生成分享链接'),
              )
            else ...[
              CupertinoButton(
                color: CupertinoColors.tertiarySystemFill.resolveFrom(context),
                onPressed: _busy ? null : _confirmRegenerate,
                child: const Text('重置链接（旧链接失效）'),
              ),
              CupertinoButton(
                onPressed: _busy ? null : _confirmUnshare,
                child: const Text('撤销分享',
                    style: TextStyle(color: CupertinoColors.systemRed)),
              ),
            ],

            const SizedBox(height: 30),
          ],
        ),
      ),
    );
  }
}
