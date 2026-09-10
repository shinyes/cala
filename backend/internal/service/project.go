package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/shinyes/cala/backend/internal/rules"
	"github.com/shinyes/cala/backend/internal/scoring"
	"github.com/shinyes/cala/backend/internal/store"
)

var (
	// ErrNotOwner 表示调用者不是项目作者（订阅者只能只读跟随，D4）。
	ErrNotOwner = errors.New("只有项目作者可以执行此操作")
	// ErrInvalidProject 表示项目字段不合法。
	ErrInvalidProject = errors.New("项目配置不合法")
)

// 项目字段约束。
const (
	MaxTitleLen       = 100
	MaxDescriptionLen = 1000
	MinQuestionCount  = 1
	MaxQuestionCount  = 200
	// MaxRuleSourceLen 限制规则源码长度，避免有人粘贴超大文件到数据库。
	MaxRuleSourceLen = 20000
)

// ProjectInput 是创建/修改项目的输入（PUT 语义：整体替换可变更字段）。
type ProjectInput struct {
	Title         string
	Description   string
	QuestionCount int
	CfgJSON       string
	RuleSource    string
	ToleranceNum  *int64
	ToleranceDen  *int64
}

// ProjectService 承载项目业务规则。
type ProjectService struct {
	store *store.Store
}

// NewProjectService 构造项目服务。
func NewProjectService(st *store.Store) *ProjectService {
	return &ProjectService{store: st}
}

// validate 校验输入，并在通过后返回解析好的 cfg（供规则校验使用）。
//
// 规则校验（规格 §5.3）是**强制项**：作者改规则会同步影响所有订阅者，
// 一条坏规则必须在保存时被挡住，否则连坐全员（规格 §5.5.1）。
func (s *ProjectService) validate(in ProjectInput) (map[string]any, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: 标题不能为空", ErrInvalidProject)
	}
	if n := len([]rune(title)); n > MaxTitleLen {
		return nil, fmt.Errorf("%w: 标题过长（%d 字符，上限 %d）", ErrInvalidProject, n, MaxTitleLen)
	}
	if n := len([]rune(in.Description)); n > MaxDescriptionLen {
		return nil, fmt.Errorf("%w: 描述过长（%d 字符，上限 %d）", ErrInvalidProject, n, MaxDescriptionLen)
	}
	if in.QuestionCount < MinQuestionCount || in.QuestionCount > MaxQuestionCount {
		return nil, fmt.Errorf("%w: 每轮题数需在 %d-%d 之间，得到 %d",
			ErrInvalidProject, MinQuestionCount, MaxQuestionCount, in.QuestionCount)
	}
	if len(in.RuleSource) > MaxRuleSourceLen {
		return nil, fmt.Errorf("%w: 规则源码过长（%d 字符，上限 %d）",
			ErrInvalidProject, len(in.RuleSource), MaxRuleSourceLen)
	}
	if err := validateTolerance(in.ToleranceNum, in.ToleranceDen); err != nil {
		return nil, err
	}

	cfg, err := parseCfg(in.CfgJSON)
	if err != nil {
		return nil, err
	}

	// 规则必须能编译、能连续出题 20 次且返回形状正确。
	rule, err := rules.Compile(in.RuleSource)
	if err != nil {
		return nil, err
	}
	if err := rule.Validate(cfg); err != nil {
		return nil, err
	}

	// 答案信封必须在**保存期**可分类（规格 §5.5.2）：
	// 否则会在用户练习时才失败，且原因难以定位到具体是哪道题。
	if err := validateAnswerEnvelopes(rule, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validateAnswerEnvelopes 试出一轮题并逐题分类答案。
//
// 用固定的种子，使本步骤可复现；题目数量取一个较小的样本即可——
// 目的是发现「答案形态无法分类」这类系统性错误，而非穷举所有题目。
func validateAnswerEnvelopes(rule *rules.Rule, cfg map[string]any) error {
	const sampleCount = 10
	qs, err := rule.Generate(cfg, 0, sampleCount)
	if err != nil {
		return err
	}
	for _, q := range qs {
		if _, err := scoring.Classify(q.A); err != nil {
			return fmt.Errorf("%w: 第 %d 题的答案 %q 无法分类：%v",
				ErrInvalidProject, q.Index+1, q.A, err)
		}
	}
	return nil
}

// validateTolerance 强制「两列同时为空或同时非空且 den > 0」。
//
// 这是规格 §6 记录的不变式。放在 service 层而非 schema 的 CHECK 约束里，
// 是因为 SQLite 的 ALTER TABLE ADD COLUMN 不便追加引用其他列的约束。
func validateTolerance(num, den *int64) error {
	if num == nil && den == nil {
		return nil // 精确比较
	}
	if num == nil || den == nil {
		return fmt.Errorf("%w: 容差的分子与分母必须同时提供或同时省略", ErrInvalidProject)
	}
	if *den <= 0 {
		return fmt.Errorf("%w: 容差分母必须为正，得到 %d", ErrInvalidProject, *den)
	}
	if *num < 0 {
		return fmt.Errorf("%w: 容差不能为负，得到 %d", ErrInvalidProject, *num)
	}
	return nil
}

// parseCfg 校验 cfg_json 是 JSON 对象。
//
// 必须是对象而非数组或标量：规则以 cfg.min / cfg.max 这类形式读它，
// 若允许任意 JSON，作者会在保存成功后才在出题时踩到类型错误。
func parseCfg(raw string) (map[string]any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]any{}, nil
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(trimmed), &cfg); err != nil {
		return nil, fmt.Errorf("%w: 配置不是合法的 JSON 对象：%v", ErrInvalidProject, err)
	}
	if cfg == nil {
		// "null" 会解析成 nil map
		return map[string]any{}, nil
	}
	return cfg, nil
}

// Create 创建项目（仅需登录，任何人可创建自己的项目）。
func (s *ProjectService) Create(ownerID int64, in ProjectInput) (store.Project, error) {
	if _, err := s.validate(in); err != nil {
		return store.Project{}, err
	}
	p, err := s.store.CreateProject(store.NewProject{
		OwnerID:       ownerID,
		Title:         strings.TrimSpace(in.Title),
		Description:   in.Description,
		QuestionCount: in.QuestionCount,
		CfgJSON:       normalizedCfgJSON(in.CfgJSON),
		RuleSource:    in.RuleSource,
		ToleranceNum:  in.ToleranceNum,
		ToleranceDen:  in.ToleranceDen,
	})
	if err != nil {
		return store.Project{}, err
	}
	return s.withAccess(p, store.AccessOwner), nil
}

// Update 修改项目，仅作者可调用。
func (s *ProjectService) Update(userID, projectID int64, in ProjectInput) (store.Project, error) {
	access, err := s.store.ProjectAccess(userID, projectID)
	if err != nil {
		return store.Project{}, err
	}
	if access != store.AccessOwner {
		return store.Project{}, ErrNotOwner
	}
	if _, err := s.validate(in); err != nil {
		return store.Project{}, err
	}

	p, err := s.store.UpdateProject(projectID, store.ProjectUpdate{
		Title:         strings.TrimSpace(in.Title),
		Description:   in.Description,
		QuestionCount: in.QuestionCount,
		CfgJSON:       normalizedCfgJSON(in.CfgJSON),
		RuleSource:    in.RuleSource,
		ToleranceNum:  in.ToleranceNum,
		ToleranceDen:  in.ToleranceDen,
	})
	if err != nil {
		return store.Project{}, err
	}
	return s.withAccess(p, store.AccessOwner), nil
}

// normalizedCfgJSON 把空配置规范化为 "{}"，避免同一语义在库里有多种表示。
func normalizedCfgJSON(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "{}"
	}
	return raw
}

// Get 取项目，调用者必须有访问权（作者或订阅者）。
func (s *ProjectService) Get(userID, projectID int64) (store.Project, error) {
	access, err := s.store.ProjectAccess(userID, projectID)
	if err != nil {
		return store.Project{}, err
	}
	p, err := s.store.GetProject(projectID)
	if err != nil {
		return store.Project{}, err
	}
	return s.withAccess(p, access), nil
}

// List 返回用户拥有的与已订阅的项目。
func (s *ProjectService) List(userID int64) (owned, subscribed []store.Project, err error) {
	return s.store.ListProjectsForUser(userID)
}

// Delete 删除项目（仅作者）。级联删除全员记录由外键完成（功能8）。
func (s *ProjectService) Delete(userID, projectID int64) error {
	access, err := s.store.ProjectAccess(userID, projectID)
	if err != nil {
		return err
	}
	if access != store.AccessOwner {
		return ErrNotOwner
	}
	return s.store.DeleteProject(projectID)
}

// withAccess 填充 Access 字段。
func (s *ProjectService) withAccess(p store.Project, access string) store.Project {
	p.Access = access
	return p
}

// RuleFor 编译并返回项目的规则，供出题使用。
func (s *ProjectService) RuleFor(p store.Project) (*rules.Rule, map[string]any, error) {
	cfg, err := parseCfg(p.CfgJSON)
	if err != nil {
		return nil, nil, err
	}
	rule, err := rules.Compile(p.RuleSource)
	if err != nil {
		return nil, nil, err
	}
	return rule, cfg, nil
}

// ToleranceFor 把项目上的容差列转换为判分用容差。
func ToleranceFor(p store.Project) (scoring.Tolerance, error) {
	if p.ToleranceNum == nil && p.ToleranceDen == nil {
		return scoring.Exact(), nil
	}
	if p.ToleranceNum == nil || p.ToleranceDen == nil {
		// 数据层不变式被破坏：不静默退回精确比较，而是明确报错，
		// 否则用户会看到一个「配置坏了但仍在跑」的项目。
		return scoring.Tolerance{}, fmt.Errorf("项目 %d 的容差字段不完整", p.ID)
	}
	return scoring.NewTolerance(*p.ToleranceNum, *p.ToleranceDen)
}
