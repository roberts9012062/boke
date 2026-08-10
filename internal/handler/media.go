// internal/handler/media.go
// 媒体控制器：上传（multipart）与静态访问。
package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/middleware"
	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// MediaHandler 媒体控制器（连接器类）。
type MediaHandler struct {
	posts *service.PostService // 媒体上传逻辑在帖子服务内（复用存储与记录）
}

// NewMediaHandler 创建媒体控制器。
func NewMediaHandler(posts *service.PostService) *MediaHandler {
	return &MediaHandler{posts: posts}
}

// Upload 处理媒体上传（POST /api/v1/media，multipart，需登录）。
func (h *MediaHandler) Upload(c *gin.Context) {
	// 解析上传文件（字段名 file）
	fileHeader, err := c.FormFile("file")
	if err != nil {
		resp.Fail(c, 400, errs.New(errs.CodeBadRequest, "请选择要上传的文件"))
		return
	}
	// 打开文件内容
	file, err := fileHeader.Open()
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	defer file.Close()

	// 委托服务层（类型/大小校验在存储层完成）
	result, err := h.posts.UploadMedia(c.Request.Context(), middleware.GetUserID(c), fileHeader, file)
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, result)
}
