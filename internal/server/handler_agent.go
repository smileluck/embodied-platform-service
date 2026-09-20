package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	bizagent "github.com/smilex/smilex-admin-gin/internal/biz/agent"
	"github.com/smilex/smilex-admin-gin/internal/server/middleware"
	agentsvc "github.com/smilex/smilex-admin-gin/internal/service/agent"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

// ---- 智能体（LLM 配置底座） ----

// agentErr 智能体操作错误映射：上游错误带具体原因渲染，其余走 i18n 注册表
func (s *HTTPServer) agentErr(c *gin.Context, err error) {
	if errors.Is(err, bizagent.ErrLLMUpstream) {
		response.Fail(c, http.StatusBadRequest, response.CodeErr,
			i18n.T(c.Request.Context(), "agent.upstream_error", bizagent.UpstreamDetail(err)))
		return
	}
	response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
}

func statusQuery(c *gin.Context) *int {
	if v := c.Query("status"); v != "" {
		if st, err := strconv.Atoi(v); err == nil {
			return &st
		}
	}
	return nil
}

func (s *HTTPServer) listAgentProviders(c *gin.Context) {
	page, size := pageParams(c)
	q := bizagent.ProviderQuery{Name: c.Query("name"), Code: c.Query("code"), Status: statusQuery(c)}
	list, pg, err := s.agent.ListProviders(c.Request.Context(), q, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}

func (s *HTTPServer) createAgentProvider(c *gin.Context) {
	var req agentsvc.ProviderCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	p, err := s.agent.CreateProvider(c.Request.Context(), req)
	if err != nil {
		s.agentErr(c, err)
		return
	}
	response.OK(c, p)
}

func (s *HTTPServer) getAgentProvider(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	p, err := s.agent.GetProvider(c.Request.Context(), id)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, p)
}

func (s *HTTPServer) updateAgentProvider(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req agentsvc.ProviderUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.agent.UpdateProvider(c.Request.Context(), id, req); err != nil {
		s.agentErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deleteAgentProvider(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.agent.DeleteProvider(c.Request.Context(), id); err != nil {
		s.agentErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) testAgentProvider(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req agentsvc.TestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	// 测试为同步短调用（MaxTokens=64），适当放宽 ctx 超时由 LLM 客户端整体超时兜底
	result, err := s.agent.TestProvider(c.Request.Context(), id, req)
	if err != nil {
		s.agentErr(c, err)
		return
	}
	response.OK(c, result)
}

func (s *HTTPServer) listAgentRemoteModels(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	models, err := s.agent.RemoteModels(c.Request.Context(), id)
	if err != nil {
		s.agentErr(c, err)
		return
	}
	response.OK(c, models)
}

func (s *HTTPServer) listAgentModels(c *gin.Context) {
	page, size := pageParams(c)
	q := bizagent.ModelQuery{Name: c.Query("name"), Status: statusQuery(c)}
	if v := c.Query("provider_id"); v != "" {
		if pid, err := strconv.ParseUint(v, 10, 64); err == nil && pid > 0 {
			id := uint(pid)
			q.ProviderID = &id
		}
	}
	list, pg, err := s.agent.ListModels(c.Request.Context(), q, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}

func (s *HTTPServer) createAgentModel(c *gin.Context) {
	var req agentsvc.ModelCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	m, err := s.agent.CreateModel(c.Request.Context(), req)
	if err != nil {
		s.agentErr(c, err)
		return
	}
	response.OK(c, m)
}

func (s *HTTPServer) updateAgentModel(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req agentsvc.ModelUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.agent.UpdateModel(c.Request.Context(), id, req); err != nil {
		s.agentErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deleteAgentModel(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.agent.DeleteModel(c.Request.Context(), id); err != nil {
		s.agentErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) testAgentModel(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	result, err := s.agent.TestModel(c.Request.Context(), id)
	if err != nil {
		s.agentErr(c, err)
		return
	}
	response.OK(c, result)
}

func (s *HTTPServer) listAgents(c *gin.Context) {
	page, size := pageParams(c)
	q := bizagent.AgentQuery{Name: c.Query("name"), Code: c.Query("code"), Status: statusQuery(c)}
	list, pg, err := s.agent.ListAgents(c.Request.Context(), q, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}

func (s *HTTPServer) createAgent(c *gin.Context) {
	var req agentsvc.AgentCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	a, err := s.agent.CreateAgent(c.Request.Context(), req)
	if err != nil {
		s.agentErr(c, err)
		return
	}
	response.OK(c, a)
}

func (s *HTTPServer) getAgent(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	a, err := s.agent.GetAgent(c.Request.Context(), id)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, a)
}

func (s *HTTPServer) updateAgent(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req agentsvc.AgentUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.agent.UpdateAgent(c.Request.Context(), id, req); err != nil {
		s.agentErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deleteAgent(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.agent.DeleteAgent(c.Request.Context(), id); err != nil {
		s.agentErr(c, err)
		return
	}
	response.OK(c, nil)
}

// writeSSE 写一帧 SSE 事件（event + data JSON）
func writeSSE(w io.Writer, event string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
}

// chatAgent Agent 调试对话（Playground）：SSE 流式输出。
// 帧协议：meta(元信息) -> delta*(增量) -> error(中断,可选) ；流结束即关闭
func (s *HTTPServer) chatAgent(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req agentsvc.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	sub := middleware.Subject(c)
	events, meta, err := s.agent.ChatStream(c.Request.Context(), id, sub.UserID, req)
	if err != nil {
		s.agentErr(c, err)
		return
	}

	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no") // 反代（nginx）禁用缓冲，保证流式透传
	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	// 首帧元信息（Agent/模型归属，前端展示用）
	writeSSE(c.Writer, "meta", meta)
	c.Writer.Flush()

	c.Stream(func(w io.Writer) bool {
		select {
		case ev, ok := <-events:
			if !ok {
				return false
			}
			if ev.Err != nil {
				if errors.Is(ev.Err, bizagent.ErrLLMUpstream) {
					writeSSE(w, "error", gin.H{"message": i18n.T(c.Request.Context(), "agent.upstream_error", bizagent.UpstreamDetail(ev.Err))})
				} else {
					writeSSE(w, "error", gin.H{"message": ev.Err.Error()})
				}
				return false
			}
			writeSSE(w, "delta", ev)
			return true
		case <-keepalive.C:
			// 保活注释帧，防中间层空闲断连
			fmt.Fprint(w, ": ping\n\n")
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}

// ---- 会话（本人数据：user_id 在 biz 层强制过滤） ----

func (s *HTTPServer) listAgentConversations(c *gin.Context) {
	page, size := pageParams(c)
	var agentID *uint
	if v := c.Query("agent_id"); v != "" {
		if id, err := strconv.ParseUint(v, 10, 64); err == nil && id > 0 {
			u := uint(id)
			agentID = &u
		}
	}
	sub := middleware.Subject(c)
	list, pg, err := s.agent.ListConversations(c.Request.Context(), sub.UserID, agentID, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}

func (s *HTTPServer) createAgentConversation(c *gin.Context) {
	var req agentsvc.ConversationCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	sub := middleware.Subject(c)
	cv, err := s.agent.CreateConversation(c.Request.Context(), sub.UserID, req)
	if err != nil {
		s.agentErr(c, err)
		return
	}
	response.OK(c, cv)
}

func (s *HTTPServer) renameAgentConversation(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req agentsvc.ConversationRenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	sub := middleware.Subject(c)
	cv, err := s.agent.RenameConversation(c.Request.Context(), sub.UserID, id, req)
	if err != nil {
		s.agentErr(c, err)
		return
	}
	response.OK(c, cv)
}

func (s *HTTPServer) deleteAgentConversation(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	sub := middleware.Subject(c)
	if err := s.agent.DeleteConversation(c.Request.Context(), sub.UserID, id); err != nil {
		s.agentErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) listAgentConversationMessages(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	page, size := pageParams(c)
	sub := middleware.Subject(c)
	list, pg, err := s.agent.ListConversationMessages(c.Request.Context(), sub.UserID, id, page, size)
	if err != nil {
		s.agentErr(c, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}
