import 'package:dio/dio.dart';

import 'server_address.dart';

/// 后端统一错误契约对应的异常。
///
/// 后端返回 `{"error":{"code","message"}}`（P0.4 确立），
/// 各页面一律通过本异常的 [code] 分支，**不解析 message 文本**。
class ApiException implements Exception {
  /// 后端错误码之一：
  /// bad_request / unauthorized / forbidden / not_found / conflict /
  /// registration_closed / rule_invalid / internal；
  /// 客户端自造：network（无法连接或超时）。
  final String code;
  final String message;
  final int? status;

  const ApiException({
    required this.code,
    required this.message,
    this.status,
  });

  /// 令牌无效或过期——调用方应清除登录态。
  bool get isUnauthorized => status == 401 || code == 'unauthorized';

  /// 规则不合法。调用方应把 [message] **原样**展示给作者：
  /// 后端已在其中指出是第几题、错在哪里。
  bool get isRuleInvalid => code == 'rule_invalid';

  bool get isRegistrationClosed => code == 'registration_closed';

  bool get isNetwork => code == 'network';

  @override
  String toString() => message.isEmpty ? code : message;
}

/// 统一 API 客户端。
///
/// 职责：baseUrl 解析、token 注入、错误契约到异常的映射。
/// 各页面不得自行拼 URL、自行解析错误体或自行设置 Authorization。
class ApiClient {
  final Dio _dio;

  String _baseUrl;

  /// 当前服务端地址（不含末尾斜杠）。
  String get baseUrl => _baseUrl;

  /// 切换服务端地址。
  ///
  /// 设计说明：**就地修改**而不是重建 ApiClient。原因有两条：
  ///   1. token 存在客户端实例上，重建会丢掉登录态；
  ///   2. 所有 API 包装器（AuthApi / ProjectApi / StatsApi / ...）
  ///      都持有同一个实例，重建后它们仍指向旧实例。
  /// 就地修改对 Dio 是安全的：它按请求读取 `options.baseUrl`。
  set baseUrl(String value) {
    _baseUrl = value;
    _dio.options.baseUrl = value;
  }

  /// 当前令牌。由 state 层设置；为空则不注入 Authorization 头。
  String? token;

  ApiClient({required String baseUrl, Dio? dio})
      : _dio = dio ?? Dio(),
        _baseUrl = baseUrl {
    _dio.options
      ..baseUrl = baseUrl
      ..connectTimeout = const Duration(seconds: 10)
      ..receiveTimeout = const Duration(seconds: 30)
      // 自行判断状态码：非 2xx 也要读取错误体（后端在其中给出 code/message）
      ..validateStatus = (_) => true;
  }

  /// 默认 baseUrl。
  ///
  /// 这是桌面/浏览器调试用的地址（后端在本机）。
  /// 真机上 127.0.0.1 指向手机自身，因此必须在设置里改为实际地址 ——
  /// 见 `server_address.dart` 与设置页。
  ///
  /// 构建期可用 `--dart-define=CALA_API=http://host:8080` 覆盖默认值
  /// （便于出定制包），运行时用户设置优先于它。
  static String defaultBaseUrl() {
    const fromEnv = String.fromEnvironment('CALA_API');
    return fromEnv.isEmpty ? defaultServerAddress : fromEnv;
  }

  Future<Map<String, dynamic>> get(String path) => _send('GET', path);
  Future<Map<String, dynamic>> post(String path, [Object? body]) =>
      _send('POST', path, body);
  Future<Map<String, dynamic>> put(String path, [Object? body]) =>
      _send('PUT', path, body);
  Future<Map<String, dynamic>> delete(String path) => _send('DELETE', path);

  Future<Map<String, dynamic>> _send(
    String method,
    String path, [
    Object? body,
  ]) async {
    final headers = <String, String>{'Content-Type': 'application/json'};
    if (token != null && token!.isNotEmpty) {
      headers['Authorization'] = 'Bearer ${token!}';
    }

    Response<dynamic> res;
    try {
      res = await _dio.request<dynamic>(
        path,
        data: body,
        options: Options(method: method, headers: headers),
      );
    } on DioException catch (e) {
      throw ApiException(
        code: 'network',
        message: _networkMessage(e),
      );
    }

    final status = res.statusCode ?? 0;

    if (status >= 200 && status < 300) {
      final data = res.data;
      if (data is Map<String, dynamic>) return data;
      if (data == null) return const {};
      // 204 或后端返回了非对象（不应发生）：给空对象而非崩溃
      return const {};
    }

    throw _fromErrorBody(res.data, status);
  }

  /// 解析后端的统一错误体。
  ///
  /// 若响应体不是预期形状（例如反代返回了 HTML 错误页），
  /// 仍要抛出可用的异常而不是让解析错误掩盖真实问题。
  static ApiException _fromErrorBody(Object? data, int status) {
    if (data is Map) {
      final err = data['error'];
      if (err is Map) {
        final code = err['code'];
        final message = err['message'];
        if (code is String) {
          return ApiException(
            code: code,
            message: message is String ? message : '',
            status: status,
          );
        }
      }
    }
    return ApiException(
      code: _codeFromStatus(status),
      message: '请求失败（HTTP $status）',
      status: status,
    );
  }

  static String _codeFromStatus(int status) {
    switch (status) {
      case 400:
        return 'bad_request';
      case 401:
        return 'unauthorized';
      case 403:
        return 'forbidden';
      case 404:
        return 'not_found';
      case 409:
        return 'conflict';
      default:
        return 'internal';
    }
  }

  static String _networkMessage(DioException e) {
    switch (e.type) {
      case DioExceptionType.connectionTimeout:
      case DioExceptionType.sendTimeout:
      case DioExceptionType.receiveTimeout:
        return '连接超时，请检查网络与服务器地址';
      case DioExceptionType.connectionError:
        return '无法连接服务器，请确认后端已启动且地址正确';
      case DioExceptionType.badCertificate:
        return '证书校验失败';
      case DioExceptionType.cancel:
        return '请求已取消';
      default:
        return '网络请求失败';
    }
  }
}
