package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
)

// SettingRegistrationOpen 是注册开关的键名。
const SettingRegistrationOpen = "registration_open"

// RegistrationOpen 读取注册开关。键不存在时写入并返回 fallback，
// 使首次启动即有一个明确、可被管理员修改的值。
func (s *Store) RegistrationOpen(fallback bool) (bool, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM setting WHERE key = ?`, SettingRegistrationOpen).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		if err := s.SetRegistrationOpen(fallback); err != nil {
			return false, err
		}
		return fallback, nil
	}
	if err != nil {
		return false, fmt.Errorf("读取注册开关失败: %w", err)
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		// 存量值损坏时退回 fallback，而不是让整个服务起不来
		return fallback, nil
	}
	return b, nil
}

// SetRegistrationOpen 写入注册开关。
func (s *Store) SetRegistrationOpen(open bool) error {
	_, err := s.db.Exec(
		`INSERT INTO setting(key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		SettingRegistrationOpen, strconv.FormatBool(open))
	if err != nil {
		return fmt.Errorf("写入注册开关失败: %w", err)
	}
	return nil
}
