package api

import (
	"github.com/gin-gonic/gin"

	"myclouddrive-go/internal/storage/service"
)

// RegisterRoutes 注册 storage 模块路由。
// 路由按“平台 -> 配置 -> 当前用户选择配置”三层能力排列。
func RegisterRoutes(router gin.IRouter, svc *service.StorageService) {
	h := NewHandler(svc)

	// 查询当前空间启用的平台。
	router.GET("/apis/storage/active-platforms", h.ListActivePlatforms)
	// 查询系统支持的存储平台。
	router.GET("/apis/storage/platforms", h.ListStoragePlatforms)
	// 查询单个平台详情。
	router.GET("/apis/storage/platform/:identifier", h.GetStoragePlatformByIdentifier)

	// 查询当前空间存储配置列表。
	router.GET("/apis/storage/platform/settings", h.ListStorageSettings)
	// 创建存储配置。
	router.POST("/apis/storage/settings", h.CreateStorageSetting)
	// 更新存储配置。
	router.PUT("/apis/storage/settings/:settingId", h.UpdateStorageSetting)
	// 删除存储配置。
	router.DELETE("/apis/storage/settings/:settingId", h.DeleteStorageSetting)
	// 激活存储配置。
	router.POST("/apis/storage/settings/:settingId/activate", h.ActivateStorageSetting)

	// 选择当前用户使用的存储配置。
	router.POST("/apis/storage/settings/:settingId/select", h.SelectCurrentStorageSetting)
}
