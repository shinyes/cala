import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/subscription_api.dart';
import '../state/subscriptions.dart';

/// 导入订阅链接页（功能 5）。
///
/// 支持**一次粘贴多行**：从聊天软件里复制多条链接后整段粘贴即可。
///
/// 结果**逐条展示**：某条失效时其余仍会成功，界面必须让用户看清哪条成了、
/// 哪条没成、为什么。整批失败或只给一个总数都会让用户反复试错。
class ImportSubscriptionsPage extends ConsumerStatefulWidget {
  const ImportSubscriptionsPage({super.key});

  @override
  ConsumerState<ImportSubscriptionsPage> createState() =>
      _ImportSubscriptionsPageState();
}

class _ImportSubscriptionsPageState
    extends ConsumerState<ImportSubscriptionsPage> {
  final _input = TextEditingController();

  @override
  void dispose() {
    _input.dispose();
    super.dispose();
  }

  Future<void> _import() async {
    FocusScope.of(context).unfocus();
    await ref.read(subscriptionProvider.notifier).importFromText(_input.text);
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(subscriptionProvider);

    return CupertinoPageScaffold(
      navigationBar: const CupertinoNavigationBar(middle: Text('导入订阅')),
      child: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            const Text(
              '粘贴分享链接，每行一条。',
              style: TextStyle(fontSize: 14, color: CupertinoColors.secondaryLabel),
            ),
            const SizedBox(height: 6),
            const Text(
              '链接形如：cala://subscribe?h=服务器&t=令牌',
              style: TextStyle(fontSize: 12, color: CupertinoColors.tertiaryLabel),
            ),
            const SizedBox(height: 12),

            CupertinoTextField(
              controller: _input,
              placeholder: 'cala://subscribe?...\ncala://subscribe?...',
              maxLines: 6,
              minLines: 4,
              style: const TextStyle(fontFamily: 'monospace', fontSize: 12),
              padding: const EdgeInsets.all(12),
            ),
            const SizedBox(height: 16),

            CupertinoButton.filled(
              onPressed: state.busy ? null : _import,
              child: state.busy
                  ? const CupertinoActivityIndicator(radius: 8)
                  : const Text('导入'),
            ),

            if (state.error != null) ...[
              const SizedBox(height: 16),
              _Banner(
                color: CupertinoColors.systemRed,
                icon: CupertinoIcons.exclamationmark_circle,
                text: state.error!,
              ),
            ],

            if (state.outcome != null) ...[
              const SizedBox(height: 20),
              _Summary(outcome: state.outcome!),
              const SizedBox(height: 12),
              for (final item in state.outcome!.results)
                _ResultTile(item: item),
            ],

            const SizedBox(height: 40),
          ],
        ),
      ),
    );
  }
}

class _Summary extends StatelessWidget {
  const _Summary({required this.outcome});
  final ImportOutcome outcome;

  @override
  Widget build(BuildContext context) {
    final allOk = outcome.succeeded == outcome.total;
    return _Banner(
      color: allOk ? CupertinoColors.systemGreen : CupertinoColors.systemOrange,
      icon: allOk
          ? CupertinoIcons.checkmark_circle
          : CupertinoIcons.info_circle,
      text: '成功 ${outcome.succeeded} / ${outcome.total} 条',
    );
  }
}

class _ResultTile extends StatelessWidget {
  const _ResultTile({required this.item});
  final ImportResultItem item;

  @override
  Widget build(BuildContext context) {
    final (icon, color, label) = switch (item.status) {
      ImportStatus.added => (
          CupertinoIcons.checkmark_circle_fill,
          CupertinoColors.systemGreen,
          '已订阅',
        ),
      ImportStatus.alreadySubscribed => (
          CupertinoIcons.checkmark_circle,
          CupertinoColors.systemGrey,
          '已在订阅列表中',
        ),
      ImportStatus.failed => (
          CupertinoIcons.xmark_circle_fill,
          CupertinoColors.systemRed,
          '失败',
        ),
    };

    return Container(
      margin: const EdgeInsets.only(bottom: 10),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: CupertinoColors.tertiarySystemFill.resolveFrom(context),
        borderRadius: BorderRadius.circular(10),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, color: color, size: 18),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  item.title ?? '（未识别）',
                  style: const TextStyle(
                      fontSize: 15, fontWeight: FontWeight.w500),
                ),
                const SizedBox(height: 2),
                Text(
                  label,
                  style: TextStyle(fontSize: 12, color: color),
                ),
                if (item.error != null) ...[
                  const SizedBox(height: 4),
                  Text(
                    item.error!,
                    style: const TextStyle(
                        fontSize: 12, color: CupertinoColors.secondaryLabel),
                  ),
                ],
                const SizedBox(height: 4),
                Text(
                  item.link,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                      fontSize: 11, color: CupertinoColors.tertiaryLabel),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _Banner extends StatelessWidget {
  const _Banner({
    required this.color,
    required this.icon,
    required this.text,
  });

  final Color color;
  final IconData icon;
  final String text;

  @override
  Widget build(BuildContext context) => Container(
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.12),
          borderRadius: BorderRadius.circular(10),
        ),
        child: Row(
          children: [
            Icon(icon, color: color, size: 18),
            const SizedBox(width: 8),
            Expanded(child: Text(text, style: TextStyle(color: color, fontSize: 13))),
          ],
        ),
      );
}
