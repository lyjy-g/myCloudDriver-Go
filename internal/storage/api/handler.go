package api

import (
	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/framework/security"
	"myclouddrive-go/internal/framework/web"
	gen "myclouddrive-go/internal/storage/api/gen"
	"myclouddrive-go/internal/storage/model"
	"myclouddrive-go/internal/storage/service"
	"net/http"
)

// StorageHandler 基于 OpenAPI 生成的 ServerInterface 实现 storage 接口。
type StorageHandler struct {
	svc *service.StorageService
}

func NewHandler(svc *service.StorageService) *StorageHandler {
	return &StorageHandler{svc: svc}
}

func (h *StorageHandler) ListActivePlatforms(w http.ResponseWriter, r *http.Request) {
	if !requireStoragePermission(w, r, security.PermissionStoragePlatformRead) {
		return
	}
	items, err := h.svc.ListActivePlatforms(r.Context())
	if err != nil {
		writeStorageError(w, err)
		return
	}
	data := toAPIPlatforms(items)
	web.WriteJSON(w, http.StatusOK, gen.PlatformListResponse{Code: strPtr("OK"), Message: strPtr("success"), Data: &data})
}

func (h *StorageHandler) ListStoragePlatforms(w http.ResponseWriter, r *http.Request) {
	if !requireStoragePermission(w, r, security.PermissionStoragePlatformRead) {
		return
	}
	items, err := h.svc.ListStoragePlatforms(r.Context())
	if err != nil {
		writeStorageError(w, err)
		return
	}
	data := toAPIPlatforms(items)
	web.WriteJSON(w, http.StatusOK, gen.PlatformListResponse{Code: strPtr("OK"), Message: strPtr("success"), Data: &data})
}

func (h *StorageHandler) GetStoragePlatformByIdentifier(w http.ResponseWriter, r *http.Request, identifier string) {
	if !requireStoragePermission(w, r, security.PermissionStoragePlatformRead) {
		return
	}
	item, err := h.svc.GetStoragePlatformByIdentifier(r.Context(), identifier)
	if err != nil {
		writeStorageError(w, err)
		return
	}
	data := toAPIPlatform(*item)
	web.WriteJSON(w, http.StatusOK, gen.PlatformResponse{Code: strPtr("OK"), Message: strPtr("success"), Data: &data})
}

func (h *StorageHandler) ListStorageSettings(w http.ResponseWriter, r *http.Request) {
	if !requireStoragePermission(w, r, security.PermissionStorageSettingRead) {
		return
	}
	items, err := h.svc.ListStorageSettings(r.Context())
	if err != nil {
		writeStorageError(w, err)
		return
	}
	data := toAPISettings(items)
	web.WriteJSON(w, http.StatusOK, gen.SettingListResponse{Code: strPtr("OK"), Message: strPtr("success"), Data: &data})
}

func (h *StorageHandler) CreateStorageSetting(w http.ResponseWriter, r *http.Request) {
	if !requireStoragePermission(w, r, security.PermissionStorageSettingWrite) {
		return
	}
	var req gen.CreateSettingRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		web.WriteError(w, http.StatusBadRequest, string(code.BadRequest), "invalid request body")
		return
	}
	item, err := h.svc.CreateStorageSetting(r.Context(), model.CreateSettingInput{
		StorageSettingName: req.StorageSettingName,
		Identifier:         req.Identifier,
		ConfigJSON:         req.ConfigJson,
	})
	if err != nil {
		writeStorageError(w, err)
		return
	}
	data := toAPISetting(*item)
	web.WriteJSON(w, http.StatusCreated, gen.SettingResponse{Code: strPtr("OK"), Message: strPtr("created"), Data: &data})
}

func (h *StorageHandler) DeleteStorageSetting(w http.ResponseWriter, r *http.Request, settingID string) {
	if !requireStoragePermission(w, r, security.PermissionStorageSettingWrite) {
		return
	}
	err := h.svc.DeleteStorageSetting(r.Context(), settingID)
	if err != nil {
		writeStorageError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *StorageHandler) UpdateStorageSetting(w http.ResponseWriter, r *http.Request, settingID string) {
	if !requireStoragePermission(w, r, security.PermissionStorageSettingWrite) {
		return
	}
	var req gen.UpdateSettingRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		web.WriteError(w, http.StatusBadRequest, string(code.BadRequest), "invalid request body")
		return
	}
	item, err := h.svc.UpdateStorageSetting(r.Context(), settingID, model.UpdateSettingInput{
		StorageSettingName: req.StorageSettingName,
		ConfigJSON:         req.ConfigJson,
	})
	if err != nil {
		writeStorageError(w, err)
		return
	}
	data := toAPISetting(*item)
	web.WriteJSON(w, http.StatusOK, gen.SettingResponse{Code: strPtr("OK"), Message: strPtr("success"), Data: &data})
}

func (h *StorageHandler) ActivateStorageSetting(w http.ResponseWriter, r *http.Request, settingID string) {
	if !requireStoragePermission(w, r, security.PermissionStorageSettingWrite) {
		return
	}
	item, err := h.svc.ActivateStorageSetting(r.Context(), settingID)
	if err != nil {
		writeStorageError(w, err)
		return
	}
	data := toAPISetting(*item)
	web.WriteJSON(w, http.StatusOK, gen.SettingResponse{Code: strPtr("OK"), Message: strPtr("success"), Data: &data})
}

func (h *StorageHandler) SelectCurrentStorageSetting(w http.ResponseWriter, r *http.Request, settingID string) {
	if !requireStoragePermission(w, r, security.PermissionStorageSettingRead) {
		return
	}
	item, err := h.svc.SetDefaultStorageSetting(r.Context(), settingID)
	if err != nil {
		writeStorageError(w, err)
		return
	}
	data := toAPISetting(*item)
	web.WriteJSON(w, http.StatusOK, gen.SettingResponse{Code: strPtr("OK"), Message: strPtr("success"), Data: &data})
}

// ActivateOrDisableStorageSettingByAction 兼容 action 风格开关接口：1 启用，0 禁用。
func (h *StorageHandler) ActivateOrDisableStorageSettingByAction(w http.ResponseWriter, r *http.Request, settingID string, action string) {
	if !requireStoragePermission(w, r, security.PermissionStorageSettingWrite) {
		return
	}
	switch action {
	case "1":
		h.ActivateStorageSetting(w, r, settingID)
	case "0":
		item, err := h.svc.DisableStorageSetting(r.Context(), settingID)
		if err != nil {
			writeStorageError(w, err)
			return
		}
		data := toAPISetting(*item)
		web.WriteJSON(w, http.StatusOK, gen.SettingResponse{
			Code:    strPtr("OK"),
			Message: strPtr("success"),
			Data:    &data,
		})
	default:
		web.WriteError(w, http.StatusBadRequest, string(code.BadRequest), "unsupported action, use 1(enable) or 0(disable)")
	}
}

func toAPIPlatform(p model.Platform) gen.StoragePlatform {
	return gen.StoragePlatform{
		Identifier:  p.Identifier,
		Name:        p.Name,
		Enabled:     p.Enabled,
		Description: p.Description,
	}
}

func toAPIPlatforms(items []model.Platform) []gen.StoragePlatform {
	result := make([]gen.StoragePlatform, 0, len(items))
	for _, item := range items {
		result = append(result, toAPIPlatform(item))
	}
	return result
}

func toAPISetting(s model.Setting) gen.StorageSetting {
	return gen.StorageSetting{
		Id:                 s.ID,
		StorageSettingName: s.StorageSettingName,
		Identifier:         s.Identifier,
		Active:             s.Active,
		ConfigJson:         s.ConfigJSON,
		UpdatedAt:          s.UpdatedAt,
	}
}

func toAPISettings(items []model.Setting) []gen.StorageSetting {
	result := make([]gen.StorageSetting, 0, len(items))
	for _, item := range items {
		result = append(result, toAPISetting(item))
	}
	return result
}

func writeStorageError(w http.ResponseWriter, err error) {
	switch {
	case code.Is(err, code.BadRequest):
		web.WriteError(w, http.StatusBadRequest, string(code.BadRequest), err.Error())
	case code.Is(err, code.NotFound):
		web.WriteError(w, http.StatusNotFound, string(code.NotFound), err.Error())
	default:
		web.WriteError(w, http.StatusInternalServerError, string(code.InternalError), err.Error())
	}
}

func strPtr(s string) *string { return &s }

func requireStoragePermission(w http.ResponseWriter, r *http.Request, permission string) bool {
	if _, err := security.RequirePermission(r.Context(), permission); err != nil {
		web.WriteError(w, http.StatusForbidden, string(code.NoPermission), err.Error())
		return false
	}
	return true
}
