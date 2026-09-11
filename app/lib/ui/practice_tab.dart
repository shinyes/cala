import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/models.dart';
import '../state/projects.dart';
import 'import_subscriptions_page.dart';
import 'practice_page.dart';
import 'project_editor_page.dart';

/// 练习 Tab：项目列表 + 管理入口（规格 §4.3）。
///
/// 订阅**导入**入口在此（它是「获得一个可练习的项目」），
/// 而退订在「我的」Tab —— 两处职责不重叠。
class PracticeTab extends ConsumerWidget {
  const PracticeTab({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(projectsProvider);

    return CupertinoPageScaffold(
      navigationBar: CupertinoNavigationBar(
        middle: const Text('练习'),
        leading: CupertinoButton(
          padding: EdgeInsets.zero,
          onPressed: () => ref.read(projectsProvider.notifier).refresh(),
          child: const Icon(CupertinoIcons.refresh),
        ),
        trailing: CupertinoButton(
          padding: EdgeInsets.zero,
          child: const Icon(CupertinoIcons.add),
          onPressed: () => _showAddSheet(context, ref),
        ),
      ),
      child: SafeArea(
        child: state.loading && state.list == null
            ? const Center(child: CupertinoActivityIndicator())
            : _body(context, ref, state),
      ),
    );
  }

  Widget _body(BuildContext context, WidgetRef ref, ProjectsState state) {
    if (state.error != null && state.list == null) {
      return ListView(
        children: [
          const SizedBox(height: 80),
          Center(
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: Column(
                children: [
                  const Icon(CupertinoIcons.wifi_slash,
                      size: 40, color: CupertinoColors.systemGrey),
                  const SizedBox(height: 12),
                  Text(state.error!, textAlign: TextAlign.center),
                  const SizedBox(height: 16),
                  CupertinoButton.filled(
                    onPressed: () => ref.read(projectsProvider.notifier).refresh(),
                    child: const Text('重试'),
                  ),
                ],
              ),
            ),
          ),
        ],
      );
    }

    if (state.isEmpty) {
      return ListView(
        children: const [
          SizedBox(height: 100),
          Center(
            child: Padding(
              padding: EdgeInsets.all(24),
              child: Column(
                children: [
                  Icon(CupertinoIcons.square_list,
                      size: 44, color: CupertinoColors.systemGrey3),
                  SizedBox(height: 12),
                  Text('还没有练习项目', style: TextStyle(fontSize: 16)),
                  SizedBox(height: 6),
                  Text(
                    '点击右上角 + 新建一个，或导入他人的订阅链接',
                    textAlign: TextAlign.center,
                    style: TextStyle(
                        fontSize: 13, color: CupertinoColors.secondaryLabel),
                  ),
                ],
              ),
            ),
          ),
        ],
      );
    }

    return ListView(
      children: [
        if (state.owned.isNotEmpty) ...[
          const _SectionHeader('我的项目'),
          ...state.owned.map((p) => _ProjectTile(project: p)),
        ],
        if (state.subscribed.isNotEmpty) ...[
          const _SectionHeader('已订阅'),
          ...state.subscribed.map((p) => _ProjectTile(project: p)),
        ],
        const SizedBox(height: 24),
      ],
    );
  }

  void _showAddSheet(BuildContext context, WidgetRef ref) {
    showCupertinoModalPopup<void>(
      context: context,
      builder: (ctx) => CupertinoActionSheet(
        title: const Text('添加练习项目'),
        actions: [
          CupertinoActionSheetAction(
            onPressed: () {
              Navigator.of(ctx).pop();
              Navigator.of(context).push(
                CupertinoPageRoute<void>(
                  builder: (_) => const ProjectEditorPage(),
                ),
              );
            },
            child: const Text('新建项目'),
          ),
          CupertinoActionSheetAction(
            onPressed: () {
              Navigator.of(ctx).pop();
              Navigator.of(context).push(
                CupertinoPageRoute<void>(
                  builder: (_) => const ImportSubscriptionsPage(),
                ),
              );
            },
            child: const Text('导入订阅链接'),
          ),
        ],
        cancelButton: CupertinoActionSheetAction(
          onPressed: () => Navigator.of(ctx).pop(),
          child: const Text('取消'),
        ),
      ),
    );
  }
}

class _SectionHeader extends StatelessWidget {
  const _SectionHeader(this.text);
  final String text;

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.fromLTRB(16, 20, 16, 6),
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

class _ProjectTile extends StatelessWidget {
  const _ProjectTile({required this.project});
  final Project project;

  @override
  Widget build(BuildContext context) {
    return CupertinoListTile(
      title: Text(project.title),
      subtitle: Text(
        '每轮 ${project.questionCount} 题'
        '${project.isOwner ? '' : ' · 订阅'}'
        '${project.hasTolerance ? ' · 容差 ${project.toleranceNum}/${project.toleranceDen}' : ''}',
      ),
      trailing: const CupertinoListTileChevron(),
      onTap: () => Navigator.of(context).push(
        CupertinoPageRoute<void>(
          builder: (_) => PracticePage(project: project),
        ),
      ),
    );
  }
}
