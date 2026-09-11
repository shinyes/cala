import 'package:flutter/cupertino.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/client.dart';
import '../api/server_address.dart';
import '../state/server_address.dart';
import '../state/session.dart' show sessionProvider;

/// 服务端地址设置页。
///
/// 为什么需要它：手机端 127.0.0.1 指向手机自身，默认地址在真机上毫无意义，
/// 因此用户**必须**能填写自己部署的后端地址；而且必须在**登录之前**就能填 ——
/// 连不上服务器时，登录页本身就是死的。
///
/// 页内提供「测试连接」：单纯保存一个错地址会让人以为是后端问题，
/// 先探测一次能把「地址错」与「服务端故障」区分开。
class ServerAddressPage extends ConsumerStatefulWidget {
  const ServerAddressPage({super.key});

  @override
  ConsumerState<ServerAddressPage> createState() => _ServerAddressPageState();
}

class _ServerAddressPageState extends ConsumerState<ServerAddressPage> {
  late final TextEditingController _controller;

  bool _busy = false;
  String? _error;
  String? _ok;

  @override
  void initState() {
    super.initState();
    _controller = TextEditingController(text: ref.read(serverAddressProvider));
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  /// 解析当前输入。失败时把消息写进 _error 并返回 null。
  String? _parse() {
    try {
      return normalizeServerAddress(_controller.text);
    } on ServerAddressError catch (e) {
      setState(() {
        _error = e.message;
        _ok = null;
      });
      return null;
    }
  }

  /// 探测候选地址：请求 `/api/healthz`。
  ///
  /// 用一个临时客户端，**不**改动全局客户端的地址 ——
  /// 测试失败不应该影响当前正在使用的连接。
  Future<void> _test() async {
    final candidate = _parse();
    if (candidate == null) return;

    setState(() {
      _busy = true;
      _error = null;
      _ok = null;
    });

    // 用临时客户端探测，**不**改动全局客户端的地址 ——
    // 测试失败不应该影响当前正在使用的连接。
    final probe = ApiClient(baseUrl: candidate);

    try {
      final res = await probe.get('/api/healthz');
      if (!mounted) return;
      setState(() => _ok = describeHealthResponse(res));
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() => _error = '${e.message}\n（地址：$candidate）');
    } on Object catch (e) {
      if (!mounted) return;
      setState(() => _error = '$e');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _save() async {
    final candidate = _parse();
    if (candidate == null) return;

    final changing = candidate != ref.read(serverAddressProvider);

    setState(() {
      _busy = true;
      _error = null;
      _ok = null;
    });

    try {
      final wasLoggedIn = ref.read(sessionProvider).isLoggedIn;
      await ref.read(serverAddressProvider.notifier).set(candidate);
      if (!mounted) return;

      // 切换地址会清除登录态（token 属于旧服务器），这是必须让用户知道的后果。
      if (changing && wasLoggedIn) {
        setState(() => _ok = '地址已保存。由于更换了服务器，需要重新登录。');
      } else {
        Navigator.of(context).pop(true);
      }
    } on ServerAddressError catch (e) {
      if (mounted) setState(() => _error = e.message);
    } on Object catch (e) {
      if (mounted) setState(() => _error = '$e');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final current = ref.watch(serverAddressProvider);
    final isDefault = current == defaultServerAddress;

    return CupertinoPageScaffold(
      navigationBar: CupertinoNavigationBar(
        middle: const Text('服务器地址'),
        trailing: CupertinoButton(
          padding: EdgeInsets.zero,
          onPressed: _busy ? null : _save,
          child: const Text('保存'),
        ),
      ),
      child: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(20),
          children: [
            const Text(
              '填写你部署的 Cala 后端地址。',
              style: TextStyle(fontSize: 14, color: CupertinoColors.secondaryLabel),
            ),
            const SizedBox(height: 16),

            CupertinoTextField(
              controller: _controller,
              placeholder: 'http://192.168.1.5:8080',
              autocorrect: false,
              enableSuggestions: false,
              keyboardType: TextInputType.url,
              clearButtonMode: OverlayVisibilityMode.editing,
              prefix: const Padding(
                padding: EdgeInsets.only(left: 12),
                child: Icon(CupertinoIcons.cloud, size: 18),
              ),
              onSubmitted: (_) => _test(),
            ),
            const SizedBox(height: 8),
            const Text(
              '可以省略 http://；只需填到端口，不要带路径。',
              style: TextStyle(fontSize: 12, color: CupertinoColors.tertiaryLabel),
            ),

            if (isDefault) ...[
              const SizedBox(height: 12),
              _Notice(
                icon: CupertinoIcons.info_circle,
                color: CupertinoColors.systemOrange,
                text: '当前是默认地址 127.0.0.1，在手机上它指向手机自身。\n'
                    '要连接电脑上运行的后端，请改成电脑在局域网中的 IP。',
              ),
            ],

            if (_error != null) ...[
              const SizedBox(height: 12),
              _Notice(
                icon: CupertinoIcons.exclamationmark_circle,
                color: CupertinoColors.systemRed,
                text: _error!,
              ),
            ],
            if (_ok != null) ...[
              const SizedBox(height: 12),
              _Notice(
                icon: CupertinoIcons.checkmark_circle,
                color: CupertinoColors.systemGreen,
                text: _ok!,
              ),
            ],

            const SizedBox(height: 20),
            CupertinoButton(
              color: CupertinoColors.systemGrey5,
              onPressed: _busy ? null : _test,
              child: _busy
                  ? const CupertinoActivityIndicator(radius: 8)
                  : const Text('测试连接',
                      style: TextStyle(color: CupertinoColors.label)),
            ),
            const SizedBox(height: 10),
            CupertinoButton.filled(
              onPressed: _busy ? null : _save,
              child: const Text('保存'),
            ),

            const SizedBox(height: 28),
            const Text(
              '当前地址',
              style: TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w600,
                color: CupertinoColors.secondaryLabel,
              ),
            ),
            const SizedBox(height: 4),
            // 不用 SelectableText：它是 Material 组件，本项目为纯 Cupertino（D10）。
            // 地址可用上方的输入框选中复制。
            Text(
              current,
              style: const TextStyle(fontSize: 13, fontFamily: 'monospace'),
            ),
          ],
        ),
      ),
    );
  }
}

/// 把 `/api/healthz` 的响应转成给用户看的成功文案。
///
/// 独立成纯函数（而不是内联在 setState 里）是为了能直接单测：
/// 它有一条**兼容分支** —— 旧版本服务端不返回 `version` 字段，
/// 那不是错误，只是没有这项信息，不能因此报「响应不符合预期」。
///
/// 显示版本号的用途：这页要回答「我连的是哪个服务器」，
/// 而版本是分辨服务器最直接的线索（尤其是自建了多台的情况）。
String describeHealthResponse(Map<String, dynamic> res) {
  if (res['status'] != 'ok') {
    return '已连上，但响应不符合预期：$res';
  }
  final v = res['version'];
  if (v is String && v.isNotEmpty) {
    return '连接成功，服务端正常（版本 $v）';
  }
  return '连接成功，服务端正常（该服务端未提供版本号）';
}

class _Notice extends StatelessWidget {
  const _Notice({
    required this.icon,
    required this.color,
    required this.text,
  });

  final IconData icon;
  final Color color;
  final String text;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, color: color, size: 18),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              text,
              style: TextStyle(color: color, fontSize: 13),
            ),
          ),
        ],
      ),
    );
  }
}
