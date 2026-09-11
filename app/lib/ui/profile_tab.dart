import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/client.dart';
import '../api/models.dart';
import '../state/projects.dart';
import '../state/server_address.dart';
import '../state/session.dart';
import '../state/subscriptions.dart';
import 'change_password_page.dart';
import 'project_editor_page.dart';
import 'server_address_page.dart';
import 'share_page.dart';
import 'shell.dart' show describeError;

/// 我的 Tab：账号与我创建的项目（规格 §4.3）。
///
/// 与练习 Tab 的职责划分：
///   - 练习 Tab 管「进入练习」与**导入**订阅
///   - 我的 Tab 管「账号」与**退订**、以及我创建的项目的管理/删除
///
/// 退订已在 P6 范围，此处给出明确提示而非静默失效。
class ProfileTab extends ConsumerWidget {
  const ProfileTab({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final session = ref.watch(sessionProvider);
    final projects = ref.watch(projectsProvider);
    final serverUrl = ref.watch(serverAddressProvider);
    final user = session.user;

    return CupertinoPageScaffold(
      navigationBar: const CupertinoNavigationBar(middle: Text('我的')),
      child: SafeArea(
        child: ListView(
          children: [
            const SizedBox(height: 8),
            _AccountTile(user: user),

            CupertinoListTile(
              leading: const Icon(CupertinoIcons.lock),
              title: const Text('修改口令'),
              trailing: const CupertinoListTileChevron(),
              onTap: () => Navigator.of(context).push(
                CupertinoPageRoute<void>(
                  builder: (_) => const ChangePasswordPage(),
                ),
              ),
            ),

            if (user?.isAdmin ?? false) ...[
              const _SectionHeader('管理员'),
              _RegistrationSwitch(session: session, ref: ref),
            ],

            const _SectionHeader('我创建的项目'),
            if (projects.owned.isEmpty)
              const _EmptyHint('还没有创建过项目')
            else
              ...projects.owned.map(
                (p) => CupertinoListTile(
                  title: Text(p.title),
                  subtitle: Text('每轮 ${p.questionCount} 题'),
                  trailing: CupertinoButton(
                    padding: EdgeInsets.zero,
                    child: const Icon(CupertinoIcons.ellipsis_circle),
                    onPressed: () => _showManageSheet(context, ref, p),
                  ),
                  onTap: () => Navigator.of(context).push(
                    CupertinoPageRoute<void>(
                      builder: (_) => ProjectEditorPage(existing: p),
                    ),
                  ),
                ),
              ),

            const _SectionHeader('我订阅的项目'),
            if (projects.subscribed.isEmpty)
              const _EmptyHint('还没有订阅任何项目')
            else
              ...projects.subscribed.map(
                (p) => CupertinoListTile(
                  title: Text(p.title),
                  subtitle: Text('每轮 ${p.questionCount} 题 · 只读跟随作者配置'),
                  trailing: CupertinoButton(
                    padding: EdgeInsets.zero,
                    child: const Icon(CupertinoIcons.minus_circle),
                    onPressed: () => _confirmUnsubscribe(context, ref, p),
                  ),
                ),
              ),

            const _SectionHeader('服务器'),
            CupertinoListTile(
              leading: const Icon(CupertinoIcons.cloud),
              title: const Text('服务端地址'),
              subtitle: Text(
                serverUrl,
                style: const TextStyle(fontSize: 13),
              ),
              trailing: const CupertinoListTileChevron(),
              onTap: () => Navigator.of(context).push(
                CupertinoPageRoute<void>(
                  builder: (_) => const ServerAddressPage(),
                ),
              ),
            ),

            const SizedBox(height: 28),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16),
              child: CupertinoButton(
                color: CupertinoColors.systemRed.withValues(alpha: 0.1),
                onPressed: () => _confirmLogout(context, ref),
                child: const Text('退出登录',
                    style: TextStyle(color: CupertinoColors.systemRed)),
              ),
            ),
            const SizedBox(height: 40),
          ],
        ),
      ),
    );
  }

  void _showManageSheet(BuildContext context, WidgetRef ref, Project p) {
    showCupertinoModalPopup<void>(
      context: context,
      builder: (ctx) => CupertinoActionSheet(
        title: Text(p.title),
        message: const Text('修改项目后，所有订阅者的项目会同步改变'),
        actions: [
          CupertinoActionSheetAction(
            onPressed: () {
              Navigator.of(ctx).pop();
              Navigator.of(context).push(
                CupertinoPageRoute<void>(
                  builder: (_) => ProjectEditorPage(existing: p),
                ),
              );
            },
            child: const Text('编辑'),
          ),
          CupertinoActionSheetAction(
            onPressed: () {
              Navigator.of(ctx).pop();
              Navigator.of(context).push(
                CupertinoPageRoute<void>(
                  builder: (_) => SharePage(project: p),
                ),
              );
            },
            child: const Text('分享订阅链接'),
          ),
          CupertinoActionSheetAction(
            isDestructiveAction: true,
            onPressed: () {
              Navigator.of(ctx).pop();
              _confirmDelete(context, ref, p);
            },
            child: const Text('删除项目'),
          ),
        ],
        cancelButton: CupertinoActionSheetAction(
          onPressed: () => Navigator.of(ctx).pop(),
          child: const Text('取消'),
        ),
      ),
    );
  }

  /// 删除项目会级联删除**所有人**的答题记录（功能8）。
  /// 破坏性且不可逆，因此必须二次确认并把后果说清楚。
  void _confirmDelete(BuildContext context, WidgetRef ref, Project p) {
    showCupertinoDialog<void>(
      context: context,
      builder: (ctx) => CupertinoAlertDialog(
        title: const Text('删除项目'),
        content: Text(
          '将删除「${p.title}」。\n\n'
          '这会同时删除**所有用户**在该项目下的答题记录与统计，'
          '且无法恢复。',
        ),
        actions: [
          CupertinoDialogAction(
            onPressed: () => Navigator.of(ctx).pop(),
            child: const Text('取消'),
          ),
          CupertinoDialogAction(
            isDestructiveAction: true,
            onPressed: () async {
              Navigator.of(ctx).pop();
              try {
                await ref.read(projectsProvider.notifier).delete(p.id);
              } on Object catch (e) {
                if (!context.mounted) return;
                _alert(context, '删除失败', describeError(e));
              }
            },
            child: const Text('删除'),
          ),
        ],
      ),
    );
  }

  /// 退订是**不可逆的破坏性操作**：会清空用户自己在该项目下的全部答题记录与统计。
  ///
  /// 因此确认对话框必须把后果说清楚，而不是只问「确定吗」。
  void _confirmUnsubscribe(BuildContext context, WidgetRef ref, Project p) {
    showCupertinoDialog<void>(
      context: context,
      builder: (ctx) => CupertinoAlertDialog(
        title: Text('退订「${p.title}」'),
        content: const Text(
          '这会删除你在该项目下的**全部答题记录与统计**，且无法恢复。\n\n'
          '项目本身与其他人的记录不受影响。之后你可以重新订阅，'
          '但已删除的历史不会回来。',
        ),
        actions: [
          CupertinoDialogAction(
            onPressed: () => Navigator.of(ctx).pop(),
            child: const Text('取消'),
          ),
          CupertinoDialogAction(
            isDestructiveAction: true,
            onPressed: () async {
              Navigator.of(ctx).pop();
              try {
                await ref
                    .read(subscriptionProvider.notifier)
                    .unsubscribe(p.id);
              } on Object catch (e) {
                if (!context.mounted) return;
                _alert(context, '退订失败', describeError(e));
              }
            },
            child: const Text('退订并删除记录'),
          ),
        ],
      ),
    );
  }

  void _confirmLogout(BuildContext context, WidgetRef ref) {
    showCupertinoDialog<void>(
      context: context,
      builder: (ctx) => CupertinoAlertDialog(
        title: const Text('退出登录'),
        content: const Text('本轮的练习进度不会被保存。'),
        actions: [
          CupertinoDialogAction(
            onPressed: () => Navigator.of(ctx).pop(),
            child: const Text('取消'),
          ),
          CupertinoDialogAction(
            isDestructiveAction: true,
            onPressed: () {
              Navigator.of(ctx).pop();
              ref.read(sessionProvider.notifier).logout();
            },
            child: const Text('退出'),
          ),
        ],
      ),
    );
  }
}

void _alert(BuildContext context, String title, String message) {
  showCupertinoDialog<void>(
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

class _AccountTile extends StatelessWidget {
  const _AccountTile({required this.user});
  final User? user;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      child: Row(
        children: [
          const Icon(CupertinoIcons.person_crop_circle_fill,
              size: 52, color: CupertinoColors.systemGrey3),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(user?.username ?? '—',
                    style: const TextStyle(
                        fontSize: 18, fontWeight: FontWeight.w600)),
                const SizedBox(height: 2),
                Text(
                  (user?.isAdmin ?? false) ? '管理员' : '普通用户',
                  style: const TextStyle(
                      fontSize: 13, color: CupertinoColors.secondaryLabel),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// 管理员：注册开关。
///
/// 关闭后新用户无法注册；但若系统尚无用户，服务端的有效值仍为 true
/// （规格 §6.2），界面据此显示状态说明。
class _RegistrationSwitch extends ConsumerStatefulWidget {
  const _RegistrationSwitch({required this.session, required this.ref});
  final SessionState session;
  final WidgetRef ref;

  @override
  ConsumerState<_RegistrationSwitch> createState() => _RegistrationSwitchState();
}

class _RegistrationSwitchState extends ConsumerState<_RegistrationSwitch> {
  bool _busy = false;
  String? _error;

  @override
  Widget build(BuildContext context) {
    final open = widget.session.registrationOpen;

    return Column(
      children: [
        Row(
          children: [
            const Expanded(
              child: Padding(
                padding: EdgeInsets.only(left: 16),
                child: Text('开放注册'),
              ),
            ),
            if (_busy)
              const Padding(
                padding: EdgeInsets.only(right: 16),
                child: CupertinoActivityIndicator(radius: 8),
              )
            else
              Padding(
                padding: const EdgeInsets.only(right: 16),
                child: CupertinoSwitch(
                  value: open,
                  onChanged: (v) async {
                    setState(() {
                      _busy = true;
                      _error = null;
                    });
                    try {
                      await ref
                          .read(sessionProvider.notifier)
                          .setRegistrationOpen(v);
                    } on ApiException catch (e) {
                      // 错误就地渲染，而不是在 await 之后弹对话框：
                      // 既避免跨 async 间隙使用 BuildContext，
                      // 也让错误信息持续可见而非一闪而过。
                      if (mounted) setState(() => _error = e.message);
                    } finally {
                      if (mounted) setState(() => _busy = false);
                    }
                  },
                ),
              ),
          ],
        ),
        if (_error != null)
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 4, 16, 8),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Icon(CupertinoIcons.exclamationmark_circle,
                    size: 14, color: CupertinoColors.systemRed),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(_error!,
                      style: const TextStyle(
                          fontSize: 12, color: CupertinoColors.systemRed)),
                ),
              ],
            ),
          ),
        if (widget.session.bootstrap)
          const Padding(
            padding: EdgeInsets.fromLTRB(16, 4, 16, 8),
            child: Text(
              '系统尚无用户：此时任何人注册都会成为管理员。',
              style: TextStyle(
                  fontSize: 12, color: CupertinoColors.systemOrange),
            ),
          ),
      ],
    );
  }
}

class _SectionHeader extends StatelessWidget {
  const _SectionHeader(this.text);
  final String text;

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.fromLTRB(16, 22, 16, 6),
        child: Text(
          text,
          style: const TextStyle(
            fontSize: 13,
            fontWeight: FontWeight.w600,
            color: CupertinoColors.secondaryLabel,
          ),
        ),
      );
}

class _EmptyHint extends StatelessWidget {
  const _EmptyHint(this.text);
  final String text;

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.fromLTRB(16, 6, 16, 6),
        child: Text(
          text,
          style: const TextStyle(
              fontSize: 14, color: CupertinoColors.tertiaryLabel),
        ),
      );
}
