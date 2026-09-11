import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/client.dart';
import '../api/models.dart';
import '../state/projects.dart';

/// 新建 / 编辑项目。
///
/// 保存时把后端的规则错误 **原样展示**：后端已在 message 中指出是第几题、
/// 错在哪里（例如「第 1 题的答案 "1/0" 无法分类：分母不能为零」）。
/// 若替换成笼统的「保存失败」，作者将无从修正。
class ProjectEditorPage extends ConsumerStatefulWidget {
  const ProjectEditorPage({super.key, this.existing});

  final Project? existing;

  bool get isEdit => existing != null;

  @override
  ConsumerState<ProjectEditorPage> createState() => _ProjectEditorPageState();
}

const _sampleRule = '''function generate(cfg) {
  const lo = cfg.min;
  const hi = cfg.max;
  const a = lo + Math.floor(Math.random() * (hi - lo + 1));
  const b = lo + Math.floor(Math.random() * (hi - lo + 1));
  return { q: a + " + " + b + " = ?", a: String(a + b) };
}''';

class _ProjectEditorPageState extends ConsumerState<ProjectEditorPage> {
  late final TextEditingController _title;
  late final TextEditingController _desc;
  late final TextEditingController _rule;
  late final TextEditingController _cfg;
  late final TextEditingController _tolNum;
  late final TextEditingController _tolDen;

  int _questionCount = 20;
  bool _useTolerance = false;
  bool _busy = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    final p = widget.existing;
    _title = TextEditingController(text: p?.title ?? '');
    _desc = TextEditingController(text: p?.description ?? '');
    _rule = TextEditingController(text: p?.ruleSource ?? _sampleRule);
    _cfg = TextEditingController(
        text: (p == null || p.cfgJson.isEmpty) ? '{"min": 10, "max": 99}' : p.cfgJson);
    _questionCount = p?.questionCount ?? 20;
    _useTolerance = p?.hasTolerance ?? false;
    _tolNum = TextEditingController(text: p?.toleranceNum?.toString() ?? '0');
    _tolDen = TextEditingController(text: p?.toleranceDen?.toString() ?? '100');
  }

  @override
  void dispose() {
    _title.dispose();
    _desc.dispose();
    _rule.dispose();
    _cfg.dispose();
    _tolNum.dispose();
    _tolDen.dispose();
    super.dispose();
  }

  /// 容差：后端要求两列同时提供或同时省略（规格 §6）。
  /// 关闭开关时两者都不传，即精确比较。
  ({int? num, int? den}) _tolerance() {
    if (!_useTolerance) return (num: null, den: null);
    return (
      num: int.tryParse(_tolNum.text.trim()) ?? 0,
      den: int.tryParse(_tolDen.text.trim()) ?? 0,
    );
  }

  Future<void> _save() async {
    if (_busy) return;

    final title = _title.text.trim();
    if (title.isEmpty) {
      setState(() => _error = '请填写项目标题');
      return;
    }

    final tol = _tolerance();
    if (_useTolerance && (tol.den == null || tol.den! <= 0)) {
      setState(() => _error = '容差分母必须为正整数');
      return;
    }

    setState(() {
      _busy = true;
      _error = null;
    });

    try {
      final notifier = ref.read(projectsProvider.notifier);
      if (widget.isEdit) {
        await notifier.update(
          widget.existing!.id,
          title: title,
          description: _desc.text.trim(),
          questionCount: _questionCount,
          cfgJson: _cfg.text.trim(),
          ruleSource: _rule.text,
          toleranceNum: tol.num,
          toleranceDen: tol.den,
        );
      } else {
        await notifier.create(
          title: title,
          description: _desc.text.trim(),
          questionCount: _questionCount,
          cfgJson: _cfg.text.trim(),
          ruleSource: _rule.text,
          toleranceNum: tol.num,
          toleranceDen: tol.den,
        );
      }
      if (!mounted) return;
      Navigator.of(context).pop();
    } on ApiException catch (e) {
      if (!mounted) return;
      // 规则错误单独呈现，并给出更醒目的提示
      setState(() => _error = e.message);
    } on Object catch (e) {
      if (!mounted) return;
      setState(() => _error = '$e');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return CupertinoPageScaffold(
      navigationBar: CupertinoNavigationBar(
        middle: Text(widget.isEdit ? '编辑项目' : '新建项目'),
        trailing: _busy
            ? const CupertinoActivityIndicator(radius: 8)
            : CupertinoButton(
                padding: EdgeInsets.zero,
                onPressed: _save,
                child: const Text('保存'),
              ),
      ),
      child: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            const _Label('标题'),
            CupertinoTextField(
              controller: _title,
              placeholder: '例如：两位数加法',
              padding: const EdgeInsets.all(12),
            ),
            const SizedBox(height: 16),

            const _Label('说明（可选）'),
            CupertinoTextField(
              controller: _desc,
              placeholder: '这个项目练什么',
              padding: const EdgeInsets.all(12),
            ),
            const SizedBox(height: 16),

            const _Label('每轮题数'),
            Row(
              children: [
                Expanded(
                  child: CupertinoSlider(
                    value: _questionCount.toDouble(),
                    min: 1,
                    max: 100,
                    divisions: 99,
                    onChanged: (v) =>
                        setState(() => _questionCount = v.round()),
                  ),
                ),
                SizedBox(
                  width: 44,
                  child: Text('$_questionCount',
                      textAlign: TextAlign.end,
                      style: const TextStyle(
                          fontSize: 17, fontWeight: FontWeight.w600)),
                ),
              ],
            ),
            const SizedBox(height: 16),

            const _Label('规则配置（JSON）'),
            CupertinoTextField(
              controller: _cfg,
              maxLines: 4,
              minLines: 2,
              style: const TextStyle(fontFamily: 'monospace', fontSize: 13),
              padding: const EdgeInsets.all(12),
            ),
            const SizedBox(height: 16),

            const _Label('出题规则（JavaScript）'),
            const Padding(
              padding: EdgeInsets.only(bottom: 6),
              child: Text(
                '定义 function generate(cfg)，返回 { q: 题面, a: 答案 }。\n'
                '答案须为数字、分数、小数，或纯文本。',
                style: TextStyle(fontSize: 12, color: CupertinoColors.secondaryLabel),
              ),
            ),
            CupertinoTextField(
              controller: _rule,
              maxLines: 14,
              minLines: 8,
              style: const TextStyle(fontFamily: 'monospace', fontSize: 13),
              padding: const EdgeInsets.all(12),
            ),
            const SizedBox(height: 16),

            Row(
              children: [
                const Expanded(child: Text('允许答案容差')),
                CupertinoSwitch(
                  value: _useTolerance,
                  onChanged: (v) => setState(() => _useTolerance = v),
                ),
              ],
            ),
            if (_useTolerance) ...[
              const Padding(
                padding: EdgeInsets.only(top: 4, bottom: 8),
                child: Text(
                  '例如 1 / 100 表示允许 0.01 的误差。关闭则为精确比较。',
                  style: TextStyle(fontSize: 12, color: CupertinoColors.secondaryLabel),
                ),
              ),
              Row(
                children: [
                  Expanded(
                    child: CupertinoTextField(
                      controller: _tolNum,
                      placeholder: '分子',
                      keyboardType: TextInputType.number,
                      padding: const EdgeInsets.all(12),
                    ),
                  ),
                  const Padding(
                    padding: EdgeInsets.symmetric(horizontal: 10),
                    child: Text('/'),
                  ),
                  Expanded(
                    child: CupertinoTextField(
                      controller: _tolDen,
                      placeholder: '分母',
                      keyboardType: TextInputType.number,
                      padding: const EdgeInsets.all(12),
                    ),
                  ),
                ],
              ),
            ],

            if (_error != null) ...[
              const SizedBox(height: 16),
              Container(
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: CupertinoColors.systemRed.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Row(
                      children: [
                        Icon(CupertinoIcons.exclamationmark_triangle,
                            size: 16, color: CupertinoColors.systemRed),
                        SizedBox(width: 6),
                        Text('规则未通过校验',
                            style: TextStyle(
                                color: CupertinoColors.systemRed,
                                fontWeight: FontWeight.w600)),
                      ],
                    ),
                    const SizedBox(height: 6),
                    Text(
                      _error!,
                      style: const TextStyle(
                          fontSize: 13, color: CupertinoColors.systemRed),
                    ),
                  ],
                ),
              ),
            ],

            const SizedBox(height: 28),
            CupertinoButton.filled(
              onPressed: _busy ? null : _save,
              child: Text(widget.isEdit ? '保存修改' : '创建项目'),
            ),
            const SizedBox(height: 40),
          ],
        ),
      ),
    );
  }
}

class _Label extends StatelessWidget {
  const _Label(this.text);
  final String text;

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.only(bottom: 6),
        child: Text(text,
            style: const TextStyle(
                fontSize: 13, fontWeight: FontWeight.w600)),
      );
}
