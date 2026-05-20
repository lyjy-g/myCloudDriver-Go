package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/framework/security"
	"myclouddrive-go/internal/storage/model"
	"myclouddrive-go/internal/storage/service"
)

type StorageHandler struct {
	svc *service.StorageService
}

type createSettingRequest struct {
	StorageSettingName string `json:"storageSettingName"`
	Identifier         string `json:"identifier"`
	ConfigJSON         string `json:"configJson"`
}

type updateSettingRequest struct {
	StorageSettingName *string `json:"storageSettingName"`
	ConfigJSON         string  `json:"configJson"`
}

// NewHandler 创建 storage 模块的 HTTP 处理器。
func NewHandler(svc *service.StorageService) *StorageHandler {
	return &StorageHandler{svc: svc}
}

// ListActivePlatforms 返回当前空间已启用的存储平台列表。
func (h *StorageHandler) ListActivePlatforms(c *gin.Context) {
	if !requireStoragePermission(c, security.PermissionStoragePlatformRead) {
		return
	}
	items, err := h.svc.ListActivePlatforms(c.Request.Context())
	if err != nil {
		writeStorageError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "message": "success", "data": items})
}

// ListStoragePlatforms 返回系统内可用的存储平台列表。
func (h *StorageHandler) ListStoragePlatforms(c *gin.Context) {
	if !requireStoragePermission(c, security.PermissionStoragePlatformRead) {
		return
	}
	items, err := h.svc.ListStoragePlatforms(c.Request.Context())
	if err != nil {
		writeStorageError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "message": "success", "data": items})
}

// GetStoragePlatformByIdentifier 按平台标识查询单个存储平台详情。
func (h *StorageHandler) GetStoragePlatformByIdentifier(c *gin.Context) {
	if !requireStoragePermission(c, security.PermissionStoragePlatformRead) {
		return
	}
	item, err := h.svc.GetStoragePlatformByIdentifier(c.Request.Context(), c.Param("identifier"))
	if err != nil {
		writeStorageError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "message": "success", "data": item})
}

// ListStorageSettings 返回当前工作空间下的存储配置列表。
func (h *StorageHandler) ListStorageSettings(c *gin.Context) {
	if !requireStoragePermission(c, security.PermissionStorageSettingRead) {
		return
	}
	items, err := h.svc.ListStorageSettings(c.Request.Context())
	if err != nil {
		writeStorageError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "message": "success", "data": items})
}

// CreateStorageSetting 创建新的存储配置。
func (h *StorageHandler) CreateStorageSetting(c *gin.Context) {
	if !requireStoragePermission(c, security.PermissionStorageSettingWrite) {
		return
	}
	var req createSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request body"})
		return
	}
	item, err := h.svc.CreateStorageSetting(c.Request.Context(), model.CreateSettingInput{
		StorageSettingName: req.StorageSettingName,
		Identifier:         req.Identifier,
		ConfigJSON:         req.ConfigJSON,
	})
	if err != nil {
		writeStorageError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": "OK", "message": "created", "data": item})
}

// DeleteStorageSetting 删除指定的存储配置。
func (h *StorageHandler) DeleteStorageSetting(c *gin.Context) {
	if !requireStoragePermission(c, security.PermissionStorageSettingWrite) {
		return
	}
	if err := h.svc.DeleteStorageSetting(c.Request.Context(), c.Param("settingId")); err != nil {
		writeStorageError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// UpdateStorageSetting 更新指定的存储配置。
func (h *StorageHandler) UpdateStorageSetting(c *gin.Context) {
	if !requireStoragePermission(c, security.PermissionStorageSettingWrite) {
		return
	}
	var req updateSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request body"})
		return
	}
	item, err := h.svc.UpdateStorageSetting(c.Request.Context(), c.Param("settingId"), model.UpdateSettingInput{
		StorageSettingName: req.StorageSettingName,
		ConfigJSON:         req.ConfigJSON,
	})
	if err != nil {
		writeStorageError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "message": "success", "data": item})
}

// ActivateStorageSetting 将指定配置切换为当前空间的激活配置。
func (h *StorageHandler) ActivateStorageSetting(c *gin.Context) {
	if !requireStoragePermission(c, security.PermissionStorageSettingWrite) {
		return
	}
	item, err := h.svc.ActivateStorageSetting(c.Request.Context(), c.Param("settingId"))
	if err != nil {
		writeStorageError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "message": "success", "data": item})
}

// SelectCurrentStorageSetting 设置当前用户在当前空间下使用的存储配置。
func (h *StorageHandler) SelectCurrentStorageSetting(c *gin.Context) {
	if !requireStoragePermission(c, security.PermissionStorageSettingRead) {
		return
	}
	item, err := h.svc.SelectCurrentStorageSetting(c.Request.Context(), c.Param("settingId"))
	if err != nil {
		writeStorageError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "message": "success", "data": item})
}

func writeStorageError(c *gin.Context, err error) {
	switch {
	case code.Is(err, code.BadRequest):
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
	case code.Is(err, code.NotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
	}
}

func requireStoragePermission(c *gin.Context, permission string) bool {
	if _, err := security.RequirePermission(c.Request.Context(), permission); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": err.Error()})
		return false
	}
	return true
}
