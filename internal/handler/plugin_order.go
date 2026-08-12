// internal/handler/plugin_order.go
// 插件购买订单控制器（M3.9 支付渠道）：
//   创建订单 / 支付签发 / 配置签发私钥（独立文件避免 plugin.go 超 400 行）。
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// PluginOrderHandler 插件订单控制器（连接器类）。
type PluginOrderHandler struct {
	plugins *service.PluginService // 插件服务
}

// NewPluginOrderHandler 创建订单控制器。
func NewPluginOrderHandler(plugins *service.PluginService) *PluginOrderHandler {
	return &PluginOrderHandler{plugins: plugins}
}

// SetIssuerKey 配置服务端许可证签发私钥（PUT /api/v1/admin/plugins/issuer-key，body: {private_key_pem}）。
// 说明：支付签发前置配置；私钥 AES 加密存储（不落明文）。
func (h *PluginOrderHandler) SetIssuerKey(c *gin.Context) {
	var req struct {
		PrivateKeyPEM string `json:"private_key_pem"` // 作者私钥 PEM（cmd/license-issue keygen 生成）
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.PrivateKeyPEM == "" {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.plugins.SetIssuerKey(c.Request.Context(), req.PrivateKeyPEM); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"configured": true})
}

// CreateOrder 创建购买订单（POST /api/v1/admin/plugins/:id/orders，body: {price}）。
func (h *PluginOrderHandler) CreateOrder(c *gin.Context) {
	instanceID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || instanceID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	var req struct {
		Price int `json:"price"` // 金额（¥；dev 模拟场景不核价）
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Price <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	orderID, err := h.plugins.CreateOrder(c.Request.Context(), instanceID, req.Price)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"order_id": orderID, "state": "pending"})
}

// PayOrder 支付订单并签发许可证（POST /api/v1/admin/plugins/orders/:orderId/pay）。
// dev 模拟支付直接成功（真实渠道接入点：回调验签后调用签发）。
func (h *PluginOrderHandler) PayOrder(c *gin.Context) {
	orderID, err := strconv.ParseInt(c.Param("orderId"), 10, 64)
	if err != nil || orderID <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	licenseJWT, err := h.plugins.PayOrder(c.Request.Context(), orderID)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"state": "paid", "license_jwt": licenseJWT})
}
