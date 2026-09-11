package service

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/shinyes/cala/backend/internal/store"
)

var (
	// ErrBadLink 表示分享链接格式不正确。
	ErrBadLink = errors.New("分享链接格式不正确")
	// ErrLinkOtherInstance 表示链接指向另一个服务器。
	ErrLinkOtherInstance = errors.New("链接属于其他服务器")
	// ErrLinkInvalid 表示令牌无效或已被撤销。
	ErrLinkInvalid = errors.New("链接无效或已被撤销")
	// ErrNotSubscribed 表示没有这条订阅关系。
	ErrNotSubscribed = errors.New("未订阅该项目")
	// ErrOwnProject 表示试图订阅自己的项目。
	ErrOwnProject = errors.New("这是你自己的项目，无需订阅")
)

// ParseSubscribeLink 解析分享链接，返回其中的服务器地址与令牌。
//
// 格式（规格 §4.4）：cala://subscribe?h=<host>&t=<token>
//
// 宽容处理：
//   - 首尾空白
//   - 从聊天软件复制时附带的说明文字（取第一个 cala:// 开头的片段）
//   - host 中带端口、带 http(s):// 前缀
//
// 严格处理：
//   - 缺少 t 或 t 为空 -> ErrBadLink（并指出是缺令牌）
//   - scheme 不是 cala://subscribe -> ErrBadLink
func ParseSubscribeLink(raw string) (host, token string, err error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", "", fmt.Errorf("%w: 链接为空", ErrBadLink)
	}

	// 从混杂文本中提取链接本体
	if i := strings.Index(s, "cala://"); i >= 0 {
		s = s[i:]
		// 截到第一个空白为止（后面可能是说明文字）
		if j := strings.IndexAny(s, " \t\r\n"); j > 0 {
			s = s[:j]
		}
	} else {
		return "", "", fmt.Errorf(
			"%w: 应以 cala://subscribe? 开头，实际为 %q", ErrBadLink, truncate(s, 40))
	}

	u, err := url.Parse(s)
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrBadLink, err)
	}
	if u.Scheme != "cala" {
		return "", "", fmt.Errorf("%w: 协议应为 cala，实际为 %q", ErrBadLink, u.Scheme)
	}
	if u.Host != "subscribe" {
		return "", "", fmt.Errorf("%w: 应为 cala://subscribe，实际为 cala://%s",
			ErrBadLink, u.Host)
	}

	q := u.Query()
	token = strings.TrimSpace(q.Get("t"))
	if token == "" {
		return "", "", fmt.Errorf("%w: 缺少分享令牌（t 参数）", ErrBadLink)
	}

	// host 可选：老链接或手工构造的链接可能省略。
	// 省略时视为当前实例（调用方用空串表示「不限制」）。
	host = normalizeHost(q.Get("h"))
	return host, token, nil
}

// normalizeHost 去掉协议前缀与末尾斜杠，便于比较。
func normalizeHost(h string) string {
	h = strings.TrimSpace(h)
	h = strings.TrimPrefix(h, "https://")
	h = strings.TrimPrefix(h, "http://")
	h = strings.TrimSuffix(h, "/")
	return h
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// ImportItem 是导入一条链接的结果。
//
// 逐条返回而非整批失败：用户粘贴 5 个链接时，其中一个失效不应让其余 4 个也失败。
type ImportItem struct {
	Link              string `json:"link"`
	OK                bool   `json:"ok"`
	ProjectID         int64  `json:"projectId,omitempty"`
	Title             string `json:"title,omitempty"`
	AlreadySubscribed bool   `json:"alreadySubscribed,omitempty"`
	// Error 是给用户看的可读原因（哪一条链接、为什么失败）。
	Error string `json:"error,omitempty"`
}

// ImportSubscriptions 逐条解析并订阅。
//
// currentHost 是当前实例的 host（取自请求头）。链接中的 h 与之不符时拒绝：
// 静默订阅失败或订阅到错误的项目，比明确报错糟得多。
func (s *ProjectService) ImportSubscriptions(
	userID int64, links []string, currentHost string,
) []ImportItem {
	cur := normalizeHost(currentHost)

	out := make([]ImportItem, 0, len(links))
	for _, raw := range links {
		out = append(out, s.importOne(userID, raw, cur))
	}
	return out
}

func (s *ProjectService) importOne(userID int64, raw, currentHost string) ImportItem {
	item := ImportItem{Link: strings.TrimSpace(raw)}

	host, token, err := ParseSubscribeLink(raw)
	if err != nil {
		item.Error = err.Error()
		return item
	}

	// 跨实例：链接里的 host 与当前实例不符
	if host != "" && currentHost != "" && !sameHost(host, currentHost) {
		item.Error = fmt.Sprintf("%s（链接指向 %s，当前为 %s）",
			ErrLinkOtherInstance.Error(), host, currentHost)
		return item
	}

	p, err := s.store.ProjectByShareToken(token)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			item.Error = ErrLinkInvalid.Error()
			return item
		}
		item.Error = err.Error()
		return item
	}

	// 订阅自己的项目：不报错，但也不建订阅行
	if p.OwnerID == userID {
		item.ProjectID = p.ID
		item.Title = p.Title
		item.Error = ErrOwnProject.Error()
		return item
	}

	already, err := s.store.IsSubscribed(userID, p.ID)
	if err != nil {
		item.Error = err.Error()
		return item
	}
	if already {
		item.OK = true
		item.ProjectID = p.ID
		item.Title = p.Title
		item.AlreadySubscribed = true
		return item
	}

	if err := s.store.Subscribe(userID, p.ID); err != nil {
		item.Error = err.Error()
		return item
	}

	item.OK = true
	item.ProjectID = p.ID
	item.Title = p.Title
	return item
}

// sameHost 比较两个 host 是否指向同一实例。
//
// 宽松匹配：忽略端口差异之外的写法差异（大小写、末尾斜杠已在 normalizeHost 处理）。
// 刻意**不**忽略端口 —— 同一台机器上 8080 与 9090 是两个不同实例。
func sameHost(a, b string) bool {
	return strings.EqualFold(a, b)
}

// Unsubscribe 退订：终止关系并清空该用户在该项目下的历史。
//
// 返回被删除的轮次数，供界面明示代价。
func (s *ProjectService) Unsubscribe(userID, projectID int64) (int, error) {
	// 作者「退订」自己的项目没有意义——他应当删除项目。
	// 显式检查以给出更好的提示，而不是让用户困惑于「我没有订阅啊」。
	p, err := s.store.GetProject(projectID)
	if err != nil {
		return 0, err
	}
	if p.OwnerID == userID {
		return 0, ErrNotOwner
	}

	subscribed, err := s.store.IsSubscribed(userID, projectID)
	if err != nil {
		return 0, err
	}
	if !subscribed {
		return 0, ErrNotSubscribed
	}

	return s.store.Unsubscribe(userID, projectID)
}
