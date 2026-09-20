// 用量计量：每次 LLM 调用（含无状态调试）记录 token 用量与耗时流水，
// 供用量统计页聚合展示；单价取模型当前配置（历史流水不回溯旧单价）。
package agent

import "time"

// UsageLog 单次调用用量流水（追加，不更新）
type UsageLog struct {
	ID               uint      `json:"id"`
	AgentID          uint      `json:"agent_id"`
	ModelID          uint      `json:"model_id"`
	UserID           uint      `json:"user_id"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	LatencyMs        int64     `json:"latency_ms"`
	CreatedAt        time.Time `json:"created_at"`
}

// UsageDailyPoint 按日聚合点
type UsageDailyPoint struct {
	Date             string  `json:"date"` // YYYY-MM-DD
	Calls            int     `json:"calls"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	Cost             float64 `json:"cost"` // 按模型当前单价的估算值
}

// UsageAgentPoint 按 Agent 聚合点
type UsageAgentPoint struct {
	AgentID     uint    `json:"agent_id"`
	AgentName   string  `json:"agent_name"`
	Calls       int     `json:"calls"`
	TotalTokens int64   `json:"total_tokens"`
	Cost        float64 `json:"cost"`
}

// UsageStats 统计聚合结果
type UsageStats struct {
	Days    []UsageDailyPoint `json:"days"`    // 最近 N 日（含无调用日补零），按日期升序
	Agents  []UsageAgentPoint `json:"agents"`  // Top Agent（按总 token 降序，最多 10 个）
	Calls   int               `json:"calls"`   // 区间内总调用次数
	Tokens  int64             `json:"tokens"`  // 区间内总 token
	Cost    float64           `json:"cost"`    // 区间费用估算合计
}
