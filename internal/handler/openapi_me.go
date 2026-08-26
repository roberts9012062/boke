// internal/handler/openapi_me.go
// 开放网关「我的资料」：GET /api/v1/open/me（me.profile，X-Api-Key 鉴权后进入）。
//
// 说明：API Key 在生成时绑定站点用户（迁移 021），浏览器插件等外部应用
//       用本端点把「URL + Key」换成交互所需的用户信息（昵称、头像、计数等）。
package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/middleware"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// Me 处理我的资料（GET /api/v1/open/me）。
// 返回：Key 绑定用户的公开资料（self=false 公开视角，不暴露 email 等私密字段）；
//       未绑定用户的旧 Key 返回 403 并给出可操作的提示文案。
func (h *OpenAPIHandler) Me(c *gin.Context) {
	userID := middleware.GetAPIKeyUserID(c)
	if userID == 0 {
		resp.Fail(c, 403, errs.New(errs.CodeForbidden,
			"该 API Key 未绑定用户：请在后台「接口开放」重新生成 Key（生成后自动绑定管理员），并勾选「我的资料」接口"))
		return
	}

	profile, err := h.auth.GetProfile(c.Request.Context(), userID, false)
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	resp.OK(c, profile)
}
