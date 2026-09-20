// 会话与消息实体：调试对话的持久化形态。
// 会话归属创建者本人（user_id 过滤，无管理面），消息为追加流水；AgentName 冗余存储，
// Agent 改名/删除后历史会话仍可读。
package agent

import "time"

// Conversation 对话会话（本人数据）
type Conversation struct {
	ID        uint      `json:"id"`
	UserID    uint      `json:"user_id"`
	AgentID   uint      `json:"agent_id"`
	AgentName string    `json:"agent_name"` // 冗余展示名
	Title     string    `json:"title"`
	LastMsgAt time.Time `json:"last_msg_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ConversationMessage 会话消息（追加流水，不更新）
type ConversationMessage struct {
	ID             uint      `json:"id"`
	ConversationID uint      `json:"conversation_id"`
	Role           string    `json:"role"` // user | assistant
	Content        string    `json:"content"`
	TotalTokens    int       `json:"total_tokens"` // assistant 消息的用量（usage.total_tokens）
	CreatedAt      time.Time `json:"created_at"`
}

// 消息角色（system 由 Agent 配置注入，不允许作为消息落库）
const (
	MsgRoleUser      = "user"
	MsgRoleAssistant = "assistant"
)

// ConversationQuery 会话列表查询条件
type ConversationQuery struct {
	AgentID *uint // 按 Agent 过滤（聊天页左侧切换）
}

// 会话默认标题（首条用户消息到达后替换为其前 20 字）
const DefaultConversationTitle = "新对话"
