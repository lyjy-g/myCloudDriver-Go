package api

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/share/service"
)

type Handler struct {
	svc *service.ShareService
}

// NewHandler 创建 share 模块的 HTTP 处理器。
func NewHandler(svc *service.ShareService) *Handler {
	return &Handler{svc: svc}
}

// PingShare 返回分享模块基础存活状态，便于联调或健康检查。
func (h *Handler) PingShare(c *gin.Context) {
	msg, err := h.svc.Ping(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error(), "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "message": msg})
}

// CreateShare 创建新的分享记录。
func (h *Handler) CreateShare(c *gin.Context) {
	var req service.CreateShareReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeShareError(c, code.New(code.BadRequest, "invalid request body"))
		return
	}
	item, err := h.svc.CreateShare(c.Request.Context(), req)
	if err != nil {
		writeShareError(c, err)
		return
	}
	c.JSON(http.StatusOK, ok(item))
}

// ListMyShares 返回当前用户在当前空间下创建的分享列表。
func (h *Handler) ListMyShares(c *gin.Context) {
	items, err := h.svc.ListMyShares(c.Request.Context())
	if err != nil {
		writeShareError(c, err)
		return
	}
	c.JSON(http.StatusOK, ok(items))
}

// GetShareDetail 返回指定分享的完整详情。
func (h *Handler) GetShareDetail(c *gin.Context) {
	item, err := h.svc.GetShareDetail(c.Request.Context(), c.Param("shareId"), true)
	if err != nil {
		writeShareError(c, err)
		return
	}
	c.JSON(http.StatusOK, ok(item))
}

// UpdateShare 更新指定分享的配置。
func (h *Handler) UpdateShare(c *gin.Context) {
	var req service.UpdateShareReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeShareError(c, code.New(code.BadRequest, "invalid request body"))
		return
	}
	item, err := h.svc.UpdateShare(c.Request.Context(), c.Param("shareId"), req)
	if err != nil {
		writeShareError(c, err)
		return
	}
	c.JSON(http.StatusOK, ok(item))
}

// AccessPublicShare 校验提取码并进入公开分享访问链路。
func (h *Handler) AccessPublicShare(c *gin.Context) {
	var req struct {
		ShareCode string `json:"shareCode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeShareError(c, code.New(code.BadRequest, "invalid request body"))
		return
	}
	item, err := h.svc.PublicAccess(c.Request.Context(), c.Param("shareId"), req.ShareCode, c.Request)
	if err != nil {
		writeShareError(c, err)
		return
	}
	c.JSON(http.StatusOK, ok(item))
}

// VerifyShareCode 校验分享提取码是否正确。
func (h *Handler) VerifyShareCode(c *gin.Context) {
	var req service.VerifyShareCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeShareError(c, code.New(code.BadRequest, "invalid request body"))
		return
	}
	okValue, err := h.svc.VerifyShareCode(c.Request.Context(), req.ShareID, req.ShareCode)
	if err != nil {
		writeShareError(c, err)
		return
	}
	c.JSON(http.StatusOK, ok(okValue))
}

// GetShareInfo 返回分享页头部需要的基础信息。
func (h *Handler) GetShareInfo(c *gin.Context) {
	item, err := h.svc.GetShareInfo(c.Request.Context(), c.Param("shareId"))
	if err != nil {
		writeShareError(c, err)
		return
	}
	c.JSON(http.StatusOK, ok(item))
}

// GetShareItems 返回分享内某个目录下的文件列表。
func (h *Handler) GetShareItems(c *gin.Context) {
	items, err := h.svc.GetShareItems(c.Request.Context(), c.Param("shareId"), c.Query("parentId"))
	if err != nil {
		writeShareError(c, err)
		return
	}
	c.JSON(http.StatusOK, ok(items))
}

// DownloadShareFile 下载分享中的指定文件。
func (h *Handler) DownloadShareFile(c *gin.Context) {
	shareCode := strings.TrimSpace(c.GetHeader("X-Share-Code"))
	if shareCode == "" {
		shareCode = strings.TrimSpace(c.Query("shareCode"))
	}
	content, info, fileName, err := h.svc.DownloadShareFile(c.Request.Context(), c.Param("shareId"), c.Param("fileId"), shareCode, c.Request)
	if err != nil {
		writeShareError(c, err)
		return
	}
	defer content.Close()
	contentType := strings.TrimSpace(info.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+url.QueryEscape(fileName))
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, content)
}

// GetAccessRecords 查询分享访问记录。
func (h *Handler) GetAccessRecords(c *gin.Context) {
	items, err := h.svc.ListAccessRecords(c.Request.Context(), c.Param("shareId"))
	if err != nil {
		writeShareError(c, err)
		return
	}
	c.JSON(http.StatusOK, ok(items))
}

// CancelShares 批量取消指定分享。
func (h *Handler) CancelShares(c *gin.Context) {
	var shareIDs []string
	if err := c.ShouldBindJSON(&shareIDs); err != nil {
		writeShareError(c, code.New(code.BadRequest, "invalid request body"))
		return
	}
	if err := h.svc.CancelShares(c.Request.Context(), shareIDs); err != nil {
		writeShareError(c, err)
		return
	}
	c.JSON(http.StatusOK, ok(nil))
}

// CancelAllShares 取消当前用户在当前空间下的全部分享。
func (h *Handler) CancelAllShares(c *gin.Context) {
	if err := h.svc.CancelAllShares(c.Request.Context()); err != nil {
		writeShareError(c, err)
		return
	}
	c.JSON(http.StatusOK, ok(nil))
}

func ok(data any) map[string]any {
	return map[string]any{"code": 200, "msg": "success", "data": data}
}

func writeShareError(c *gin.Context, err error) {
	switch {
	case code.Is(err, code.BadRequest):
		c.JSON(http.StatusBadRequest, map[string]any{"code": 400, "msg": err.Error(), "data": nil})
	case code.Is(err, code.NotFound):
		c.JSON(http.StatusNotFound, map[string]any{"code": 404, "msg": err.Error(), "data": nil})
	case code.Is(err, code.NoPermission):
		c.JSON(http.StatusForbidden, map[string]any{"code": 403, "msg": err.Error(), "data": nil})
	default:
		c.JSON(http.StatusInternalServerError, map[string]any{"code": 500, "msg": err.Error(), "data": nil})
	}
}
