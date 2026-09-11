package service

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shinyes/cala/backend/internal/rules"
	"github.com/shinyes/cala/backend/internal/scoring"
	"github.com/shinyes/cala/backend/internal/store"
)

const validRule = `
function generate(cfg) {
  const lo = cfg.min || 1, hi = cfg.max || 9;
  const a = lo + Math.floor(Math.random() * (hi - lo + 1));
  const b = lo + Math.floor(Math.random() * (hi - lo + 1));
  return { q: a + " + " + b + " = ?", a: String(a + b) };
}`

func newProjectSvc(t *testing.T) (*ProjectService, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return NewProjectService(st), st
}

func makeUser(t *testing.T, st *store.Store, name string) int64 {
	t.Helper()
	u, err := st.CreateUser(name, "hash", false)
	if err != nil {
		t.Fatalf("建用户失败: %v", err)
	}
	return u.ID
}

func validInput() ProjectInput {
	return ProjectInput{
		Title:         "口算",
		Description:   "两位数加法",
		QuestionCount: 5,
		CfgJSON:       `{"min":10,"max":99}`,
		RuleSource:    validRule,
	}
}

func TestCreateProjectAcceptsValid(t *testing.T) {
	svc, st := newProjectSvc(t)
	owner := makeUser(t, st, "owner")

	p, err := svc.Create(owner, validInput())
	if err != nil {
		t.Fatalf("创建项目失败: %v", err)
	}
	if p.ID == 0 || p.Access != store.AccessOwner {
		t.Errorf("返回值异常: %+v", p)
	}
	if p.CfgJSON != `{"min":10,"max":99}` {
		t.Errorf("cfg 未原样保存: %q", p.CfgJSON)
	}
}

// TestCreateProjectRejectsBadRule 覆盖基线 §5.2(2)：保存前必须校验。
// 作者改规则会同步影响所有订阅者，坏规则必须被挡在保存之前。
func TestCreateProjectRejectsBadRule(t *testing.T) {
	svc, st := newProjectSvc(t)
	owner := makeUser(t, st, "owner")

	badRules := map[string]string{
		"缺少 generate": `function gen(cfg){ return {q:"1",a:"1"} }`,
		"语法错误":        `function generate( { return }`,
		"死循环":         `function generate(cfg){ while(true){} }`,
		"返回非对象":       `function generate(cfg){ return 42 }`,
		"缺少 a 字段":     `function generate(cfg){ return {q:"1"} }`,
	}
	for name, src := range badRules {
		t.Run(name, func(t *testing.T) {
			in := validInput()
			in.RuleSource = src
			_, err := svc.Create(owner, in)
			if err == nil {
				t.Fatal("坏规则应被拒绝")
			}
			var ve *rules.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("应返回 *rules.ValidationError 以便映射为 rule_invalid, 得到 %T: %v", err, err)
			}
		})
	}
}

// TestCreateProjectRejectsTextAnswer 覆盖「文本答案无法作答」这一实测缺陷。
//
// 取证（真实后端，见 evidence/p7 §6.7）：`a:"质数"` 的规则**保存成功**、
// 服务端也正常下发 `kind=text` 的题目，但练习页只有数字键盘
// （`0-9` `.` `-` 与 `⌫`，规格 §9.3）且不唤起系统键盘 —— 该题永远答不对。
//
// 因此保存期必须拒绝，并给出可行动的说明；否则作者会做出一个坏项目
// 而毫无察觉（这正是该缺陷此前长期存在的原因）。
func TestCreateProjectRejectsTextAnswer(t *testing.T) {
	svc, st := newProjectSvc(t)
	owner := makeUser(t, st, "owner")

	cases := map[string]string{
		"中文文本":  `function generate(cfg){ return {q:"7 是质数吗", a:"质数"} }`,
		"英文文本":  `function generate(cfg){ return {q:"?", a:"prime"} }`,
		"数值加单位": `function generate(cfg){ return {q:"?", a:"3 个"} }`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			in := validInput()
			in.RuleSource = src
			_, err := svc.Create(owner, in)
			if err == nil {
				t.Fatal("文本答案应被拒绝：客户端没有输入途径，用户永远答不对")
			}
			if !errors.Is(err, ErrInvalidProject) {
				t.Errorf("应返回 ErrInvalidProject, 得到 %v", err)
			}
			msg := err.Error()
			// 必须指出题号，否则作者无从定位
			if !strings.Contains(msg, "题") {
				t.Errorf("应指出是第几题, 得到: %v", msg)
			}
			// 必须说明原因（键盘打不出来），否则作者只会困惑于「为什么不能文本」
			if !strings.Contains(msg, "无法作答") {
				t.Errorf("应说明该答案无法作答, 得到: %v", msg)
			}
			// 必须给出行动指引
			if !strings.Contains(msg, "数值") {
				t.Errorf("应提示改为数值答案, 得到: %v", msg)
			}
		})
	}
}

// TestScoringStillClassifiesTextAnswer 锁定兼容边界：
// **scoring.Classify 必须继续接受文本答案**。
//
// 产品层拒绝了新的文本答案，但已落库的历史信封（kind=text）仍要靠
// Classify 判分；若哪天有人「顺手」把拒绝逻辑下沉到 scoring，
// 历史数据与错题重练会立刻无法判定。本测试防止这种下沉。
func TestScoringStillClassifiesTextAnswer(t *testing.T) {
	env, err := scoring.Classify("质数")
	if err != nil {
		t.Fatalf("scoring 必须继续能分类文本答案（历史数据依赖它判分）: %v", err)
	}
	if env.Kind != scoring.KindText || env.Value != "质数" {
		t.Errorf("信封 = %+v, 期望 kind=text value=质数", env)
	}

	// 数值答案不受影响
	num, err := scoring.Classify("3/4")
	if err != nil || num.Kind != scoring.KindRational {
		t.Errorf("数值答案应仍可分类, 得到 %+v err=%v", num, err)
	}
}

// TestClassifyAnswerableAllowsNumeric 确认新增约束没有误伤数值答案。
func TestClassifyAnswerableAllowsNumeric(t *testing.T) {
	for _, a := range []string{"42", "-7", "0.75", "3/4", "-0.5", "  12  "} {
		if _, err := classifyAnswerable(a); err != nil {
			t.Errorf("数值答案 %q 不应被拒绝: %v", a, err)
		}
	}
	// 文本必须被拒
	for _, a := range []string{"质数", "prime", "3 个"} {
		if _, err := classifyAnswerable(a); err == nil {
			t.Errorf("文本答案 %q 应被拒绝", a)
		}
	}
}

// TestCreateProjectRejectsMalformedAnswer 覆盖验收标准 A11：
// 答案无法分类时必须在保存期报错，并指出是第几题，否则问题会在用户练习时才暴露。
func TestCreateProjectRejectsMalformedAnswer(t *testing.T) {
	svc, st := newProjectSvc(t)
	owner := makeUser(t, st, "owner")

	cases := map[string]string{
		"分母为零":  `function generate(cfg){ return {q:"1/0 = ?", a:"1/0"} }`,
		"畸形数值":  `function generate(cfg){ return {q:"1.2.3 = ?", a:"1.2.3"} }`,
		"答案为空":  `function generate(cfg){ return {q:"?", a:" "} }`,
		"含控制字符": `function generate(cfg){ return {q:"?", a:"a\u0000b"} }`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			in := validInput()
			in.RuleSource = src
			_, err := svc.Create(owner, in)
			if err == nil {
				t.Fatal("无法分类的答案应被拒绝")
			}
			if !errors.Is(err, ErrInvalidProject) {
				t.Errorf("应返回 ErrInvalidProject, 得到 %v", err)
			}
			if !strings.Contains(err.Error(), "无法分类") {
				t.Errorf("错误信息应说明是答案无法分类, 得到: %v", err)
			}
			// 必须指出题号，否则作者无从定位
			if !strings.Contains(err.Error(), "题") {
				t.Errorf("错误信息应指出是第几题, 得到: %v", err)
			}
		})
	}
}

func TestCreateProjectValidatesFields(t *testing.T) {
	svc, st := newProjectSvc(t)
	owner := makeUser(t, st, "owner")

	cases := []struct {
		name   string
		mutate func(*ProjectInput)
	}{
		{"空标题", func(in *ProjectInput) { in.Title = "   " }},
		{"标题过长", func(in *ProjectInput) { in.Title = strings.Repeat("x", MaxTitleLen+1) }},
		{"描述过长", func(in *ProjectInput) { in.Description = strings.Repeat("x", MaxDescriptionLen+1) }},
		{"题数为零", func(in *ProjectInput) { in.QuestionCount = 0 }},
		{"题数为负", func(in *ProjectInput) { in.QuestionCount = -1 }},
		{"题数过多", func(in *ProjectInput) { in.QuestionCount = MaxQuestionCount + 1 }},
		{"cfg 不是 JSON", func(in *ProjectInput) { in.CfgJSON = `{not json` }},
		{"cfg 是数组", func(in *ProjectInput) { in.CfgJSON = `[1,2,3]` }},
		{"cfg 是标量", func(in *ProjectInput) { in.CfgJSON = `42` }},
		{"规则源码过长", func(in *ProjectInput) { in.RuleSource = strings.Repeat("//x\n", MaxRuleSourceLen) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := validInput()
			tc.mutate(&in)
			_, err := svc.Create(owner, in)
			if err == nil {
				t.Fatal("应被拒绝")
			}
			if !errors.Is(err, ErrInvalidProject) {
				t.Errorf("应返回 ErrInvalidProject, 得到 %v", err)
			}
		})
	}
}

// TestToleranceMustComeAsPair 覆盖规格 §6 的容差不变式。
func TestToleranceMustComeAsPair(t *testing.T) {
	svc, st := newProjectSvc(t)
	owner := makeUser(t, st, "owner")

	num, den := int64(1), int64(100)

	t.Run("只给分子", func(t *testing.T) {
		in := validInput()
		in.ToleranceNum = &num
		if _, err := svc.Create(owner, in); !errors.Is(err, ErrInvalidProject) {
			t.Errorf("只给分子应被拒, 得到 %v", err)
		}
	})
	t.Run("只给分母", func(t *testing.T) {
		in := validInput()
		in.ToleranceDen = &den
		if _, err := svc.Create(owner, in); !errors.Is(err, ErrInvalidProject) {
			t.Errorf("只给分母应被拒, 得到 %v", err)
		}
	})
	t.Run("分母为零", func(t *testing.T) {
		in := validInput()
		zero := int64(0)
		in.ToleranceNum, in.ToleranceDen = &num, &zero
		if _, err := svc.Create(owner, in); !errors.Is(err, ErrInvalidProject) {
			t.Errorf("分母为零应被拒, 得到 %v", err)
		}
	})
	t.Run("分子为负", func(t *testing.T) {
		in := validInput()
		neg := int64(-1)
		in.ToleranceNum, in.ToleranceDen = &neg, &den
		if _, err := svc.Create(owner, in); !errors.Is(err, ErrInvalidProject) {
			t.Errorf("负容差应被拒, 得到 %v", err)
		}
	})
	t.Run("成对提供", func(t *testing.T) {
		in := validInput()
		in.ToleranceNum, in.ToleranceDen = &num, &den
		p, err := svc.Create(owner, in)
		if err != nil {
			t.Fatalf("合法容差应通过: %v", err)
		}
		if p.ToleranceNum == nil || *p.ToleranceNum != 1 || p.ToleranceDen == nil || *p.ToleranceDen != 100 {
			t.Errorf("容差未正确保存: %+v", p)
		}
	})
	t.Run("都不给即精确比较", func(t *testing.T) {
		p, err := svc.Create(owner, validInput())
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		if p.ToleranceNum != nil || p.ToleranceDen != nil {
			t.Errorf("应为精确比较, 得到 %v/%v", p.ToleranceNum, p.ToleranceDen)
		}
	})
}

// TestToleranceComesFromColumn 防止有人从 cfg_json 里读容差（两个来源）。
func TestToleranceComesFromColumn(t *testing.T) {
	svc, st := newProjectSvc(t)
	owner := makeUser(t, st, "owner")

	// cfg 里放一个 tolerance 字段，但它不应影响判分用的容差
	in := validInput()
	in.CfgJSON = `{"min":1,"max":9,"tolerance":"0.5"}`
	p, err := svc.Create(owner, in)
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}

	tol, err := ToleranceFor(p)
	if err != nil {
		t.Fatalf("ToleranceFor 失败: %v", err)
	}
	if !tol.IsZero() {
		t.Error("容差必须来自 tolerance_num/den 列，而不是 cfg_json —— cfg 中的 tolerance 不应生效")
	}
}

func TestToleranceForRejectsHalfSetColumns(t *testing.T) {
	svc, st := newProjectSvc(t)
	owner := makeUser(t, st, "owner")
	p, err := svc.Create(owner, validInput())
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}

	// 直接破坏数据层不变式，模拟「列被写坏」
	only := int64(1)
	p.ToleranceNum = &only
	if _, err := ToleranceFor(p); err == nil {
		t.Error("容差列只设一半时应报错，而不是静默退回精确比较")
	}
}

func TestUpdateRequiresOwnership(t *testing.T) {
	svc, st := newProjectSvc(t)
	owner := makeUser(t, st, "owner")
	other := makeUser(t, st, "other")
	sub := makeUser(t, st, "sub")

	p, err := svc.Create(owner, validInput())
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if err := st.Subscribe(sub, p.ID); err != nil {
		t.Fatalf("Subscribe 失败: %v", err)
	}

	t.Run("订阅者不能改", func(t *testing.T) {
		in := validInput()
		in.Title = "被篡改"
		if _, err := svc.Update(sub, p.ID, in); !errors.Is(err, ErrNotOwner) {
			t.Errorf("订阅者修改应返回 ErrNotOwner, 得到 %v", err)
		}
	})
	t.Run("无关用户不能改", func(t *testing.T) {
		if _, err := svc.Update(other, p.ID, validInput()); err == nil {
			t.Error("无关用户修改应报错")
		}
	})
	t.Run("作者可以改", func(t *testing.T) {
		in := validInput()
		in.Title = "新标题"
		got, err := svc.Update(owner, p.ID, in)
		if err != nil {
			t.Fatalf("作者修改失败: %v", err)
		}
		if got.Title != "新标题" {
			t.Errorf("标题未更新: %q", got.Title)
		}
	})
	t.Run("作者改规则时同样要过校验", func(t *testing.T) {
		in := validInput()
		in.RuleSource = `function generate(cfg){ while(true){} }`
		_, err := svc.Update(owner, p.ID, in)
		var ve *rules.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("改规则时坏规则应被拒, 得到 %v", err)
		}
	})
}

func TestGetAndListRespectAccess(t *testing.T) {
	svc, st := newProjectSvc(t)
	owner := makeUser(t, st, "owner")
	sub := makeUser(t, st, "sub")
	stranger := makeUser(t, st, "stranger")

	p, err := svc.Create(owner, validInput())
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if err := st.Subscribe(sub, p.ID); err != nil {
		t.Fatalf("Subscribe 失败: %v", err)
	}

	got, err := svc.Get(sub, p.ID)
	if err != nil {
		t.Fatalf("订阅者应能查看: %v", err)
	}
	if got.Access != store.AccessSubscriber {
		t.Errorf("Access = %q, 期望 subscriber", got.Access)
	}

	if _, err := svc.Get(stranger, p.ID); !errors.Is(err, store.ErrNoAccess) {
		t.Errorf("无关用户应被拒, 得到 %v", err)
	}

	owned, subscribed, err := svc.List(sub)
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(owned) != 0 {
		t.Errorf("sub 没有自己的项目, owned 应为空, 得到 %d", len(owned))
	}
	if len(subscribed) != 1 || subscribed[0].ID != p.ID {
		t.Errorf("subscribed 应含订阅的项目, 得到 %+v", subscribed)
	}
}

// TestDeleteProjectCascadesAllUsers 覆盖功能8：作者删项目 -> 所有人的记录一起删。
func TestDeleteProjectCascadesAllUsers(t *testing.T) {
	svc, st := newProjectSvc(t)
	owner := makeUser(t, st, "owner")
	sub := makeUser(t, st, "sub")

	p, err := svc.Create(owner, validInput())
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if err := st.Subscribe(sub, p.ID); err != nil {
		t.Fatalf("Subscribe 失败: %v", err)
	}

	// 作者与订阅者各落一轮
	for _, uid := range []int64{owner, sub} {
		if _, err := st.CreateRound(store.NewRound{
			ProjectID: p.ID, UserID: uid, Seed: 1,
			StartedAt: store.Now(), FinishedAt: store.Now(),
			TotalMs: 1000, QuestionCount: 1, CorrectCount: 1,
			Attempts: []store.NewAttempt{{
				Index: 0, QSnapshot: "1+1", ASnapshot: "2",
				EnvelopeJSON: `{"kind":"rational","num":"2","den":"1"}`,
				UserInput:    "2", ClientIsCorrect: true, ServerIsCorrect: true, ElapsedMs: 1000,
			}},
		}); err != nil {
			t.Fatalf("CreateRound 失败: %v", err)
		}
	}

	counts := func() (rounds, attempts, subs int) {
		st.DB().QueryRow(`SELECT COUNT(*) FROM practice_round`).Scan(&rounds)
		st.DB().QueryRow(`SELECT COUNT(*) FROM attempt`).Scan(&attempts)
		st.DB().QueryRow(`SELECT COUNT(*) FROM subscription`).Scan(&subs)
		return
	}
	if r, a, s := counts(); r != 2 || a != 2 || s != 1 {
		t.Fatalf("种子异常: rounds=%d attempts=%d subs=%d", r, a, s)
	}

	if err := svc.Delete(owner, p.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}

	r, a, s := counts()
	if r != 0 || a != 0 || s != 0 {
		t.Errorf("级联删除不完整: rounds=%d attempts=%d subs=%d, 期望全为 0", r, a, s)
	}
}

func TestDeleteRequiresOwnership(t *testing.T) {
	svc, st := newProjectSvc(t)
	owner := makeUser(t, st, "owner")
	sub := makeUser(t, st, "sub")

	p, err := svc.Create(owner, validInput())
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if err := st.Subscribe(sub, p.ID); err != nil {
		t.Fatalf("Subscribe 失败: %v", err)
	}

	if err := svc.Delete(sub, p.ID); !errors.Is(err, ErrNotOwner) {
		t.Errorf("订阅者删除应返回 ErrNotOwner, 得到 %v", err)
	}
	if _, err := svc.Get(owner, p.ID); err != nil {
		t.Errorf("项目不应被删除: %v", err)
	}
}

// TestEmptyCfgNormalized 保证空配置统一存为 {}，避免同一语义多种表示。
func TestEmptyCfgNormalized(t *testing.T) {
	svc, st := newProjectSvc(t)
	owner := makeUser(t, st, "owner")

	in := validInput()
	in.CfgJSON = "   "
	p, err := svc.Create(owner, in)
	if err != nil {
		t.Fatalf("空 cfg 应被接受: %v", err)
	}
	if p.CfgJSON != "{}" {
		t.Errorf("空 cfg 应规范化为 {}, 得到 %q", p.CfgJSON)
	}
}
