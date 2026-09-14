// Package session 会话限界上下文 —— 领域层。
// 会话状态存 Redis：每个「用户 × 设备端」至多一个活跃会话（同端互斥），
// JWT 携带 sid，token 仅在对应会话存活期间有效。
package session

import (
	"errors"
	"time"
)

// 设备端类型：同一用户同端互斥（新登录顶掉旧会话），不同端可并存
const (
	DeviceWeb = "web" // 网页端
	DeviceApp = "app" // 移动端
)

// NormalizeDevice 未知/空设备类型归一为 web（保持旧客户端行为不变）
func NormalizeDevice(d string) string {
	if d == DeviceApp {
		return DeviceApp
	}
	return DeviceWeb
}

// ErrSessionNotFound 会话不存在或已过期/被吊销
var ErrSessionNotFound = errors.New("session not found")

// Session 在线会话
type Session struct {
	ID           string    // 会话 ID（JWT sid）
	UserID       uint      // 归属用户
	Username     string    // 用户名快照（展示用）
	Nickname     string    // 昵称快照（展示用）
	Device       string    // 设备端：web / app
	IP           string    // 登录 IP
	UserAgent    string    // 登录 User-Agent 摘要
	LoginAt      time.Time // 登录时间
	LastActiveAt time.Time // 最近活跃时间
	ExpiresAt    time.Time // 过期时间（= TTL 到期）
}

// Query 在线列表查询条件
type Query struct {
	Username string // 用户名模糊匹配
	Device   string // 设备端精确匹配（空为全部）
}
