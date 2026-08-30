// internal/handler/openapi_media.go
// 开放网关「图片转存」：POST /api/v1/open/media/transfer（media.transfer，X-Api-Key 鉴权后进入）。
//
// 说明：浏览器插件「AI 生成文章」链路的配套端点——源站图片多为防盗链外链，
// 直接引用发布后会裂图；插件发布前调用本端点把外链图落到站点媒体库，
// 以本站持久地址替换后再发布。仅放行公网 http/https 图片地址（SSRF 防护）。
package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/middleware"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// MediaTransfer 处理图片转存（POST /api/v1/open/media/transfer，body: {url}）。
// 返回：{url（本站地址）, media_id, mime_type, size_bytes}。
func (h *OpenAPIHandler) MediaTransfer(c *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.New(errs.CodeBadRequest, "请求体需包含 url（http/https 图片地址）"))
		return
	}
	result, err := h.ai.TransferImage(c.Request.Context(), req.URL)
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	resp.OK(c, result)
}

// MediaUpload 处理媒体上传（POST /api/v1/open/media，multipart，字段名 file）。
// 说明：浏览器插件「写说说」通道——本地图/粘贴图以 Key 绑定用户身份落站点媒体库，
// 返回本站地址与媒体 ID（正文内嵌 <img> + media_ids 关联）；类型/大小校验沿用存储层。
func (h *OpenAPIHandler) MediaUpload(c *gin.Context) {
	userID := middleware.GetAPIKeyUserID(c)
	if userID == 0 {
		resp.Fail(c, 403, errs.New(errs.CodeForbidden,
			"该 API Key 未绑定用户：请在后台重新生成 Key 后再上传"))
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		resp.Fail(c, 400, errs.New(errs.CodeBadRequest, "请选择要上传的文件（multipart 字段名 file）"))
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	defer file.Close()
	result, err := h.posts.UploadMedia(c.Request.Context(), userID, fileHeader, file)
	if err != nil {
		h.failWithLog(c, err)
		return
	}
	resp.OK(c, result)
}
