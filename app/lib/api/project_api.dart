import 'client.dart';
import 'models.dart';

/// 项目相关端点（规格 §7）。
class ProjectApi {
  final ApiClient _c;
  ProjectApi(this._c);

  /// 我拥有的与我订阅的项目（后端已分组）。
  Future<ProjectList> list() async {
    final j = await _c.get('/api/projects');
    return ProjectList.fromJson(j);
  }

  Future<Project> get(int id) async {
    final j = await _c.get('/api/projects/$id');
    return Project.fromJson((j['project'] as Map<String, dynamic>?) ?? const {});
  }

  /// 创建项目。规则不合法时抛出 code == 'rule_invalid' 的 [ApiException]，
  /// 其 message 由后端给出（含第几题、错在哪里），调用方应原样展示。
  Future<Project> create({
    required String title,
    required String description,
    required int questionCount,
    required String cfgJson,
    required String ruleSource,
    int? toleranceNum,
    int? toleranceDen,
  }) async {
    final j = await _c.post('/api/projects', {
      'title': title,
      'description': description,
      'questionCount': questionCount,
      'cfgJson': cfgJson,
      'ruleSource': ruleSource,
      // 后端要求容差两列同时提供或同时省略（规格 §6）
      if (toleranceNum != null && toleranceDen != null) ...{
        'toleranceNum': toleranceNum,
        'toleranceDen': toleranceDen,
      },
    });
    return Project.fromJson((j['project'] as Map<String, dynamic>?) ?? const {});
  }

  Future<Project> update(
    int id, {
    required String title,
    required String description,
    required int questionCount,
    required String cfgJson,
    required String ruleSource,
    int? toleranceNum,
    int? toleranceDen,
  }) async {
    final j = await _c.put('/api/projects/$id', {
      'title': title,
      'description': description,
      'questionCount': questionCount,
      'cfgJson': cfgJson,
      'ruleSource': ruleSource,
      if (toleranceNum != null && toleranceDen != null) ...{
        'toleranceNum': toleranceNum,
        'toleranceDen': toleranceDen,
      },
    });
    return Project.fromJson((j['project'] as Map<String, dynamic>?) ?? const {});
  }

  /// 删除项目。级联删除**所有人**在该项目下的做题记录（功能8）。
  Future<void> delete(int id) => _c.delete('/api/projects/$id');
}
