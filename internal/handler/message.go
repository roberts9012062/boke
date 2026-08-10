// internal/handler/message.go
// 私信控制器（M2，需求 3.10 私信/消息）：
// 会话列表/发起、消息分页、发送、未读总数。
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/roberts9012062/boke/internal/middleware"
	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// MessageHandler 私信控制器（连接器类）。
type MessageHandler struct {
	messages *service.MessageService // 私信服务
	logger   *zap.Logger             // 错误日志（5xx 留痕）
}

// NewMessageHandler 创建私信控制器。
func NewMessageHandler(messages *service.MessageService, logger *zap.Logger) *MessageHandler {
	return &MessageHandler{messages: messages, logger: logger}
}

// ListConversations 会话列表（GET /api/v1/conversations?filter=all|unread&page=）。
func (h *MessageHandler) ListConversations(c *gin.Context) {
	page, pageSize := parsePage(c)
	onlyUnread := c.Query("filter") == "unread"
	items, total, err := h.messages.ListConversations(c.Request.Context(), middleware.GetUserID(c), onlyUnread, page, pageSize)
	if err != nil {
		h.logger.Error("查询会话列表失败", zap.Int64("uid", middleware.GetUserID(c)), zap.Error(err))
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"page": page, "page_size": pageSize, "total": total, "items": items})
}

// OpenConversation 发起/打开会话（POST /api/v1/conversations，body: {user_id}）。
func (h *MessageHandler) OpenConversation(c *gin.Context) {
	var req struct {
		UserID int64 `json:"user_id" binding:"required"` // 对方用户 ID
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	dto, err := h.messages.Open(c.Request.Context(), middleware.GetUserID(c), req.UserID)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, dto)
}

// ListMessages 会话消息列表（GET /api/v1/conversations/:id/messages?page=；打开即已读）。
func (h *MessageHandler) ListMessages(c *gin.Context) {
	conversationID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || conversationID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	page, pageSize := parsePage(c)
	items, total, err := h.messages.ListMessages(c.Request.Context(), middleware.GetUserID(c), conversationID, page, pageSize)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"page": page, "page_size": pageSize, "total": total, "items": items})
}

// SendMessage 发送消息（POST /api/v1/conversations/:id/messages，body: {content}）。
func (h *MessageHandler) SendMessage(c *gin.Context) {
	conversationID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || conversationID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	var req struct {
		Content string `json:"content"` // 消息内容
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	dto, err := h.messages.Send(c.Request.Context(), middleware.GetUserID(c), conversationID, req.Content)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, dto)
}

// UnreadTotal 全部会话未读总数（GET /api/v1/conversations/unread-count）。
func (h *MessageHandler) UnreadTotal(c *gin.Context) {
	total, err := h.messages.UnreadTotal(c.Request.Context(), middleware.GetUserID(c))
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"unread": total})
}
