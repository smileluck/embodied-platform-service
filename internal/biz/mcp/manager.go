package mcp

// 会话管理：serverCode -> 活跃客户端（惰性建立、空闲过期、配置变更失效）。
// 单进程内存态实现：进程重启后首个调用自动重建；并发下同 code 至多保留一个客户端
//（竞态产生的多余连接由 put 端关闭，胜者不变）。

import (
	"sync"
	"time"
)

const (
	// sessionIdleTTL 会话空闲过期：超过未使用即关闭（下次调用重建握手）
	sessionIdleTTL = 10 * time.Minute
	// toolsCacheTTL 工具清单缓存时长（发现接口与 chat 下发共用，减少重复 tools/list）
	toolsCacheTTL = 5 * time.Minute
)

type sessionEntry struct {
	client   *Client
	lastUsed time.Time
}

type toolsCacheEntry struct {
	tools     []ToolInfo
	server    ServerInfo
	expiresAt time.Time
}

// Manager MCP 会话池（Usecase 内嵌使用，非独立依赖）
type Manager struct {
	mu    sync.Mutex
	live  map[string]*sessionEntry
	tools map[string]*toolsCacheEntry
}

func NewManager() *Manager {
	return &Manager{live: make(map[string]*sessionEntry), tools: make(map[string]*toolsCacheEntry)}
}

// get 取活跃客户端（空闲过期的顺手关闭）
func (m *Manager) get(code string) *Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.live[code]
	if !ok {
		return nil
	}
	if time.Since(e.lastUsed) > sessionIdleTTL {
		delete(m.live, code)
		go e.client.Close()
		return nil
	}
	e.lastUsed = time.Now()
	return e.client
}

// put 存入客户端（已有存活会话时关闭新来者，保留先到者）
func (m *Manager) put(code string, c *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.live[code]; ok && !e.client.closedSnapshot() {
		go c.Close()
		return
	}
	m.live[code] = &sessionEntry{client: c, lastUsed: time.Now()}
}

// drop 主动失效会话与工具缓存（配置更新/删除时调用）
func (m *Manager) drop(code string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.live[code]; ok {
		delete(m.live, code)
		go e.client.Close()
	}
	delete(m.tools, code)
}

// cachedTools 命中的工具清单缓存（含服务器身份，供测试展示）
func (m *Manager) cachedTools(code string) ([]ToolInfo, ServerInfo, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.tools[code]
	if !ok || time.Now().After(e.expiresAt) {
		return nil, ServerInfo{}, false
	}
	return e.tools, e.server, true
}

// cacheTools 写入工具清单缓存
func (m *Manager) cacheTools(code string, tools []ToolInfo, server ServerInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tools[code] = &toolsCacheEntry{tools: tools, server: server, expiresAt: time.Now().Add(toolsCacheTTL)}
}
