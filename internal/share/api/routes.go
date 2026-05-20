package api

import (
	"github.com/gin-gonic/gin"

	"myclouddrive-go/internal/share/service"
)

// RegisterRoutes 注册 share 模块路由。
// 后端只保留分享主路径，不再维护前端兼容别名。
func RegisterRoutes(router gin.IRouter, svc *service.ShareService) {
	h := NewHandler(svc)

	// 分享模块健康检查。
	router.GET("/apis/share/ping", h.PingShare)
	// 创建分享。
	router.POST("/apis/share/create", h.CreateShare)
	// 查询我的分享列表。
	router.GET("/apis/share/pages", h.ListMyShares)
	// 查询单个分享详情。
	router.GET("/apis/share/:shareId", h.GetShareDetail)
	// 更新分享配置。
	router.PUT("/apis/share/:shareId", h.UpdateShare)
	// 校验分享提取码。
	router.POST("/apis/share/verify/code", h.VerifyShareCode)
	// 查询分享页基础信息。
	router.GET("/apis/share/:shareId/info", h.GetShareInfo)
	// 查询分享页文件列表。
	router.GET("/apis/share/:shareId/items", h.GetShareItems)
	// 下载分享中的文件。
	router.GET("/apis/share/:shareId/download/:fileId", h.DownloadShareFile)
	// 查询分享访问记录。
	router.GET("/apis/share/:shareId/access/records", h.GetAccessRecords)
	// 批量取消指定分享。
	router.DELETE("/apis/share/cancels", h.CancelShares)
	// 取消当前用户的全部分享。
	router.DELETE("/apis/share/clears", h.CancelAllShares)
}
