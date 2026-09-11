import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/models.dart';
import '../api/project_api.dart';
import 'session.dart';

/// 项目接口。
final projectApiProvider = Provider<ProjectApi>((ref) {
  return ProjectApi(ref.watch(apiClientProvider));
});

/// 项目列表状态。
@immutable
class ProjectsState {
  final bool loading;
  final ProjectList? list;
  final String? error;

  const ProjectsState({this.loading = false, this.list, this.error});

  List<Project> get owned => list?.owned ?? const [];
  List<Project> get subscribed => list?.subscribed ?? const [];
  bool get isEmpty => owned.isEmpty && subscribed.isEmpty;
}

/// 项目列表与增删改。
class ProjectsNotifier extends Notifier<ProjectsState> {
  @override
  ProjectsState build() {
    Future.microtask(refresh);
    return const ProjectsState(loading: true);
  }

  ProjectApi get _api => ref.read(projectApiProvider);

  Future<void> refresh() async {
    state = ProjectsState(loading: true, list: state.list);
    try {
      final list = await _api.list();
      state = ProjectsState(list: list);
    } on Object catch (e) {
      state = ProjectsState(list: state.list, error: '$e');
    }
  }

  /// 创建项目。规则不合法时后端返回的 message 已指出是第几题，
  /// 此处**原样向上抛出**，由界面展示给作者。
  Future<Project> create({
    required String title,
    required String description,
    required int questionCount,
    required String cfgJson,
    required String ruleSource,
    int? toleranceNum,
    int? toleranceDen,
  }) async {
    final p = await _api.create(
      title: title,
      description: description,
      questionCount: questionCount,
      cfgJson: cfgJson,
      ruleSource: ruleSource,
      toleranceNum: toleranceNum,
      toleranceDen: toleranceDen,
    );
    await refresh();
    return p;
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
    final p = await _api.update(
      id,
      title: title,
      description: description,
      questionCount: questionCount,
      cfgJson: cfgJson,
      ruleSource: ruleSource,
      toleranceNum: toleranceNum,
      toleranceDen: toleranceDen,
    );
    await refresh();
    return p;
  }

  /// 删除项目。会级联删除**所有人**在该项目下的做题记录（功能8）。
  Future<void> delete(int id) async {
    await _api.delete(id);
    await refresh();
  }
}

final projectsProvider =
    NotifierProvider<ProjectsNotifier, ProjectsState>(ProjectsNotifier.new);
