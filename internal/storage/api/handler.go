package api

import (
	"myclouddrive-go/internal/framework/code"
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
	items, err := h.svc.ListActivePlatforms(r.Context())
	if err != nil {
		web.WriteError(w, http.StatusInternalServerError, string(code.InternalError), err.Error())
		return
	}
	web.WriteJSON(w, http.StatusOK, gen.PlatformListResponse{Code: "OK", Message: "success", Data: toAPIPlatforms(items)})
}

func (h *StorageHandler) ListStoragePlatforms(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListStoragePlatforms(r.Context())
	if err != nil {
		web.WriteError(w, http.StatusInternalServerError, string(code.InternalError), err.Error())
		return
	}
	web.WriteJSON(w, http.StatusOK, gen.PlatformListResponse{Code: "OK", Message: "success", Data: toAPIPlatforms(items)})
}

func (h *StorageHandler) GetStoragePlatformByIdentifier(w http.ResponseWriter, r *http.Request, identifier gen.Identifier) {
	item, err := h.svc.GetStoragePlatformByIdentifier(r.Context(), string(identifier))
	if err != nil {
		if code.Is(err, code.NotFound) {
			web.WriteError(w, http.StatusNotFound, string(code.NotFound), "platform not found")
			return
		}
		web.WriteError(w, http.StatusInternalServerError, string(code.InternalError), err.Error())
		return
	}
	web.WriteJSON(w, http.StatusOK, gen.PlatformResponse{Code: "OK", Message: "success", Data: toAPIPlatform(*item)})
}

func (h *StorageHandler) ListStorageSettings(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListStorageSettings(r.Context())
	if err != nil {
		web.WriteError(w, http.StatusInternalServerError, string(code.InternalError), err.Error())
		return
	}
	web.WriteJSON(w, http.StatusOK, gen.SettingListResponse{Code: "OK", Message: "success", Data: toAPISettings(items)})
}

func (h *StorageHandler) CreateStorageSetting(w http.ResponseWriter, r *http.Request) {
	var req gen.CreateSettingRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		web.WriteError(w, http.StatusBadRequest, string(code.BadRequest), "invalid request body")
		return
	}
	item, err := h.svc.CreateStorageSetting(r.Context(), model.CreateSettingInput{
		Identifier: req.Identifier,
		ConfigJSON: req.ConfigJson,
	})
	if err != nil {
		web.WriteError(w, http.StatusInternalServerError, string(code.InternalError), err.Error())
		return
	}
	web.WriteJSON(w, http.StatusCreated, gen.SettingResponse{Code: "OK", Message: "created", Data: toAPISetting(*item)})
}

func (h *StorageHandler) DeleteStorageSetting(w http.ResponseWriter, r *http.Request, settingID gen.SettingId) {
	err := h.svc.DeleteStorageSetting(r.Context(), string(settingID))
	if err != nil {
		if code.Is(err, code.NotFound) {
			web.WriteError(w, http.StatusNotFound, string(code.NotFound), "setting not found")
			return
		}
		web.WriteError(w, http.StatusInternalServerError, string(code.InternalError), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *StorageHandler) UpdateStorageSetting(w http.ResponseWriter, r *http.Request, settingID gen.SettingId) {
	var req gen.UpdateSettingRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		web.WriteError(w, http.StatusBadRequest, string(code.BadRequest), "invalid request body")
		return
	}
	item, err := h.svc.UpdateStorageSetting(r.Context(), string(settingID), model.UpdateSettingInput{
		ConfigJSON: req.ConfigJson,
	})
	if err != nil {
		if code.Is(err, code.NotFound) {
			web.WriteError(w, http.StatusNotFound, string(code.NotFound), "setting not found")
			return
		}
		web.WriteError(w, http.StatusInternalServerError, string(code.InternalError), err.Error())
		return
	}
	web.WriteJSON(w, http.StatusOK, gen.SettingResponse{Code: "OK", Message: "success", Data: toAPISetting(*item)})
}

func (h *StorageHandler) ActivateStorageSetting(w http.ResponseWriter, r *http.Request, settingID gen.SettingId) {
	item, err := h.svc.ActivateStorageSetting(r.Context(), string(settingID))
	if err != nil {
		if code.Is(err, code.NotFound) {
			web.WriteError(w, http.StatusNotFound, string(code.NotFound), "setting not found")
			return
		}
		web.WriteError(w, http.StatusInternalServerError, string(code.InternalError), err.Error())
		return
	}
	web.WriteJSON(w, http.StatusOK, gen.SettingResponse{Code: "OK", Message: "success", Data: toAPISetting(*item)})
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
		Id:         s.ID,
		Identifier: s.Identifier,
		Active:     s.Active,
		ConfigJson: s.ConfigJSON,
		UpdatedAt:  s.UpdatedAt,
	}
}

func toAPISettings(items []model.Setting) []gen.StorageSetting {
	result := make([]gen.StorageSetting, 0, len(items))
	for _, item := range items {
		result = append(result, toAPISetting(item))
	}
	return result
}
