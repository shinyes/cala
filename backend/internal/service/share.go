package service

import (
	"errors"
	"fmt"

	"github.com/shinyes/cala/backend/internal/auth"
	"github.com/shinyes/cala/backend/internal/store"
)

// ErrNotShared 表示项目尚未分享（没有 token）。
var ErrNotShared = errors.New("该项目尚未分享")

// ShareLink 是分享结果。
type ShareLink struct {
	Token string
	// Link 是服务端拼好的完整链接，客户端可直接复制转发。
	//
	// 由服务端拼而非客户端拼：链接格式是**服务端契约**（规格 §4.4），
	// 两个实现会漂移。
	Link string
}

// SubscribeLinkFormat 说明链接格式，用于错误信息与文档。
const SubscribeLinkFormat = "cala://subscribe?h=<服务器地址>&t=<分享令牌>"

// Share 生成（或重置）项目的分享 token，并返回完整链接。
//
// 仅作者可操作。重置会使旧链接立即失效。
func (s *ProjectService) Share(userID, projectID int64, host string) (ShareLink, error) {
	access, err := s.store.ProjectAccess(userID, projectID)
	if err != nil {
		return ShareLink{}, err
	}
	if access != store.AccessOwner {
		return ShareLink{}, ErrNotOwner
	}

	// 复用会话令牌的生成器：同样是 32 字节随机 + base64url。
	// 存储方式的差异见 store.SetShareToken 的注释。
	raw, _, err := auth.NewToken()
	if err != nil {
		return ShareLink{}, err
	}
	if err := s.store.SetShareToken(projectID, raw); err != nil {
		return ShareLink{}, err
	}
	return ShareLink{Token: raw, Link: BuildSubscribeLink(host, raw)}, nil
}

// Unshare 撤销分享。
func (s *ProjectService) Unshare(userID, projectID int64) error {
	access, err := s.store.ProjectAccess(userID, projectID)
	if err != nil {
		return err
	}
	if access != store.AccessOwner {
		return ErrNotOwner
	}
	return s.store.ClearShareToken(projectID)
}

// BuildSubscribeLink 按规格 §4.4 的格式拼接链接。
//
// 为什么不用 https:// 链接：后端不提供网页前端（D9），
// 形如 https://host/s/<token> 的链接在浏览器中会 404，
// 无法作为「点开即订阅」的落地页。因此采用应用内可粘贴的链接字符串。
func BuildSubscribeLink(host, token string) string {
	return fmt.Sprintf("cala://subscribe?h=%s&t=%s", host, token)
}
