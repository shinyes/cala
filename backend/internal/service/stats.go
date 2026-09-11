package service

import (
	"time"

	"github.com/shinyes/cala/backend/internal/stats"
)

// Stats 返回某用户在某项目上的统计数据。
//
// 三件事，且只做这三件：判定访问权、取回该用户的轮次、交给 stats 聚合。
// 聚合逻辑不在本层——它是 internal/stats 的唯一职责。
//
// 隐私边界：取数按 (userID, projectID) 过滤，因此
//   - 作者看不到订阅者的统计
//   - 订阅者也看不到作者的统计
//
// 这不是权限细节，而是数据所有权：答题记录属于做题的人。
func (s *ProjectService) Stats(
	userID, projectID int64,
	grain stats.Grain,
	tzOffset time.Duration,
) (stats.Result, error) {
	if _, err := s.store.ProjectAccess(userID, projectID); err != nil {
		return stats.Result{}, err
	}

	rows, err := s.store.ListRoundsForUserProject(userID, projectID)
	if err != nil {
		return stats.Result{}, err
	}

	rounds := make([]stats.Round, 0, len(rows))
	for _, r := range rows {
		finished, err := time.Parse(time.RFC3339, r.FinishedAt)
		if err != nil {
			// 单条时间损坏不应让整页统计打不开：跳过它并继续。
			// 这类数据只可能来自外部手工改动，不值得为此让功能不可用。
			continue
		}
		rounds = append(rounds, stats.Round{
			FinishedAt:    finished,
			TotalMs:       r.TotalMs,
			CorrectCount:  r.CorrectCount,
			QuestionCount: r.QuestionCount,
		})
	}

	return stats.Compute(grain, rounds, tzOffset), nil
}
