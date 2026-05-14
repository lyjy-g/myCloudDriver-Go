package api

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	filemodel "myclouddrive-go/internal/file/model"
	"myclouddrive-go/internal/file/service"
	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/framework/security"
	"myclouddrive-go/internal/framework/web"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Handler 基于 OpenAPI 生成与手工补充路由实现 file 接口。
type Handler struct {
	svc *service.FileService
}

func NewHandler(svc *service.FileService) *Handler {
	return &Handler{svc: svc}
}

// GetHomes 查询首页信息。
func (h *Handler) GetHomes(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileRead) {
		return
	}
	home := h.svc.Home(r.Context())
	web.WriteJSON(w, http.StatusOK, ok(map[string]any{
		"usedBytes": home.UsedBytes,
		"recent":    home.Recent,
	}))
}

// CreateDirectory 创建目录。
func (h *Handler) CreateDirectory(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileWrite) {
		return
	}
	h.handleIdempotentWrite(w, r, "file.directory.create", func() (int, any, error) {
		body, err := readJSONBody(r)
		if err != nil {
			return http.StatusBadRequest, errorPayload(code.BadRequest, err.Error()), nil
		}
		parentID := stringField(body, "parentId", "parent_id", "pid")
		name := stringField(body, "name", "dirName")
		item, err := h.svc.CreateDirectory(r.Context(), parentID, name, currentStorageSettingID(r))
		if err != nil {
			return http.StatusBadRequest, errorPayload(code.BadRequest, err.Error()), nil
		}
		return http.StatusOK, ok(item), nil
	})
}

// RenameFile 文件重命名。
func (h *Handler) RenameFile(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileWrite) {
		return
	}
	h.handleIdempotentWrite(w, r, "file.rename", func() (int, any, error) {
		fileID := r.PathValue("fileId")
		body, err := readJSONBody(r)
		if err != nil {
			return http.StatusBadRequest, errorPayload(code.BadRequest, err.Error()), nil
		}
		name := stringField(body, "newName", "name")
		item, err := h.svc.Rename(fileID, name)
		if err != nil {
			return http.StatusBadRequest, errorPayload(code.BadRequest, err.Error()), nil
		}
		return http.StatusOK, ok(item), nil
	})
}

// MoveFile 文件移动。
func (h *Handler) MoveFile(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileWrite) {
		return
	}
	h.handleIdempotentWrite(w, r, "file.move", func() (int, any, error) {
		body, err := readJSONBody(r)
		if err != nil {
			return http.StatusBadRequest, errorPayload(code.BadRequest, err.Error()), nil
		}
		ids := stringArrayField(body, "fileIds", "ids", "file_ids")
		targetParentID := stringField(body, "targetParentId", "target_parent_id", "parentId")
		if len(ids) == 0 || strings.TrimSpace(targetParentID) == "" {
			return http.StatusBadRequest, errorPayload(code.BadRequest, "fileIds and targetParentId are required"), nil
		}
		if err = h.svc.Move(ids, targetParentID); err != nil {
			return http.StatusBadRequest, errorPayload(code.BadRequest, err.Error()), nil
		}
		return http.StatusOK, ok(map[string]any{"moved": len(ids)}), nil
	})
}

// GetList 查询所有文件列表。
func (h *Handler) GetList(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileRead) {
		return
	}
	items := h.svc.List(r.Context(), r.URL.Query().Get("parentId"), r.URL.Query().Get("keyword"), currentStorageSettingID(r))
	web.WriteJSON(w, http.StatusOK, ok(map[string]any{
		"total": len(items),
		"items": items,
	}))
}

// GetDirs 查询目录列表。
func (h *Handler) GetDirs(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileRead) {
		return
	}
	items := h.svc.ListDirs(r.Context(), r.URL.Query().Get("parentId"), currentStorageSettingID(r))
	web.WriteJSON(w, http.StatusOK, ok(items))
}

// GetFileDetails 查询文件详情。
func (h *Handler) GetFileDetails(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileRead) {
		return
	}
	item, err := h.svc.Get(r.Context(), r.PathValue("fileId"))
	if err != nil {
		web.WriteError(w, http.StatusNotFound, string(code.NotFound), err.Error())
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(item))
}

// GetDirectoryPath 获取目录路径。
func (h *Handler) GetDirectoryPath(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileRead) {
		return
	}
	items, err := h.svc.DirPath(r.PathValue("dirId"))
	if err != nil {
		web.WriteError(w, http.StatusNotFound, string(code.NotFound), err.Error())
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(items))
}

// GetFileUrl 获取文件 URL。
func (h *Handler) GetFileUrl(w http.ResponseWriter, request *http.Request) {
	if !requireFilePermission(w, request, security.PermissionFileRead) {
		return
	}
	fileID := request.PathValue("fileId")
	expireSeconds := intQuery(request, "expireSeconds", 600)
	url, item, err := h.svc.ResolveDownloadURL(request.Context(), fileID, time.Duration(expireSeconds)*time.Second)
	if err != nil {
		web.WriteError(w, http.StatusNotFound, string(code.NotFound), err.Error())
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(map[string]any{
		"url":       url,
		"objectKey": item.ObjectKey,
	}))
}

// DeleteFiles 移到回收站。
func (h *Handler) DeleteFiles(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileWrite) {
		return
	}
	h.handleIdempotentWrite(w, r, "file.recycle.soft_delete", func() (int, any, error) {
		ids, err := readIDArray(r)
		if err != nil {
			return http.StatusBadRequest, errorPayload(code.BadRequest, err.Error()), nil
		}
		h.svc.Recycle(r.Context(), ids)
		return http.StatusOK, ok(map[string]any{"deleted": len(ids)}), nil
	})
}

// RestoreFile 恢复文件。
func (h *Handler) RestoreFile(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileWrite) {
		return
	}
	h.handleIdempotentWrite(w, r, "file.recycle.restore", func() (int, any, error) {
		ids, err := readIDArray(r)
		if err != nil {
			return http.StatusBadRequest, errorPayload(code.BadRequest, err.Error()), nil
		}
		h.svc.Restore(r.Context(), ids)
		return http.StatusOK, ok(map[string]any{"restored": len(ids)}), nil
	})
}

// PermanentlyDeleteFiles 永久删除文件。
func (h *Handler) PermanentlyDeleteFiles(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileWrite) {
		return
	}
	h.handleIdempotentWrite(w, r, "file.recycle.permanent_delete", func() (int, any, error) {
		ids, err := readIDArray(r)
		if err != nil {
			return http.StatusBadRequest, errorPayload(code.BadRequest, err.Error()), nil
		}
		report := h.svc.PermanentlyDelete(r.Context(), ids)
		return http.StatusOK, ok(report), nil
	})
}

// ClearRecycles 清空回收站。
func (h *Handler) ClearRecycles(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileWrite) {
		return
	}
	h.handleIdempotentWrite(w, r, "file.recycle.clear", func() (int, any, error) {
		report := h.svc.ClearRecycle(r.Context())
		return http.StatusOK, ok(report), nil
	})
}

// GetRecyclePages 分页获取回收站。
func (h *Handler) GetRecyclePages(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileRead) {
		return
	}
	page := intQuery(r, "page", 1)
	size := intQuery(r, "size", 20)
	items, total := h.svc.ListRecycle(r.Context(), page, size)
	web.WriteJSON(w, http.StatusOK, ok(map[string]any{
		"page":  page,
		"size":  size,
		"total": total,
		"items": items,
	}))
}

// FavoritesFile 收藏文件。
func (h *Handler) FavoritesFile(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileWrite) {
		return
	}
	h.handleIdempotentWrite(w, r, "file.favorite.add", func() (int, any, error) {
		ids, err := readIDArray(r)
		if err != nil {
			return http.StatusBadRequest, errorPayload(code.BadRequest, err.Error()), nil
		}
		h.svc.SetFavorite(ids, true)
		return http.StatusOK, ok(map[string]any{"favorite": len(ids)}), nil
	})
}

// UnFavoritesFile 取消收藏文件。
func (h *Handler) UnFavoritesFile(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileWrite) {
		return
	}
	h.handleIdempotentWrite(w, r, "file.favorite.remove", func() (int, any, error) {
		ids, err := readIDArray(r)
		if err != nil {
			return http.StatusBadRequest, errorPayload(code.BadRequest, err.Error()), nil
		}
		h.svc.SetFavorite(ids, false)
		return http.StatusOK, ok(map[string]any{"unfavorite": len(ids)}), nil
	})
}

// PreviewToken 生成文件预览 token。
func (h *Handler) PreviewToken(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileRead) {
		return
	}
	h.handleIdempotentWrite(w, r, "file.preview.token", func() (int, any, error) {
		fileID := r.PathValue("fileId")
		return http.StatusOK, ok(tokenPayload("preview", fileID, "")), nil
	})
}

// ArchivePreviewToken 生成压缩包预览 token。
func (h *Handler) ArchivePreviewToken(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileRead) {
		return
	}
	h.handleIdempotentWrite(w, r, "file.archive.preview.token", func() (int, any, error) {
		archiveID := r.PathValue("archiveFileId")
		innerPath := r.URL.Query().Get("innerPath")
		return http.StatusOK, ok(tokenPayload("archive-preview", archiveID, innerPath)), nil
	})
}

func (h *Handler) handleIdempotentWrite(w http.ResponseWriter, r *http.Request, endpoint string, execute func() (int, any, error)) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		web.WriteError(w, http.StatusBadRequest, string(code.BadRequest), "invalid request body")
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idemKey == "" {
		web.WriteError(w, http.StatusBadRequest, string(code.BadRequest), "Idempotency-Key header is required for write operations")
		return
	}

	statusCode, payload, replayed, execErr := h.svc.ExecuteIdempotent(endpoint, idemKey, body, execute)
	if execErr != nil {
		switch {
		case errors.Is(execErr, service.ErrIdempotencyConflict):
			web.WriteError(w, http.StatusConflict, string(code.BadRequest), execErr.Error())
		case errors.Is(execErr, service.ErrIdempotencyInProgress):
			web.WriteError(w, http.StatusConflict, string(code.BadRequest), execErr.Error())
		default:
			web.WriteError(w, http.StatusInternalServerError, string(code.InternalError), execErr.Error())
		}
		return
	}
	if replayed {
		w.Header().Set("X-Idempotent-Replayed", "true")
	}
	web.WriteJSON(w, statusCode, payload)
}

// Preview 文件流预览（演示实现）。
func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileRead) {
		return
	}
	fileID := r.PathValue("fileId")
	rc, info, item, err := h.svc.OpenPreviewContent(r.Context(), fileID)
	if err != nil {
		web.WriteError(w, http.StatusNotFound, string(code.NotFound), err.Error())
		return
	}

	// 无底层对象时返回演示内容，避免空响应。
	if rc == nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(fmt.Sprintf("preview stream: fileId=%s name=%s objectKey=%s", item.ID, item.Name, item.ObjectKey)))
		return
	}
	defer func() { _ = rc.Close() }()

	if strings.TrimSpace(info.ContentType) != "" {
		w.Header().Set("Content-Type", info.ContentType)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	if info.Size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	}

	if _, err = io.Copy(w, rc); err != nil && !errors.Is(err, io.EOF) {
		web.WriteError(w, http.StatusInternalServerError, string(code.InternalError), err.Error())
		return
	}
}

// PreviewArchiveInner 压缩包内文件预览（演示实现）。
func (h *Handler) PreviewArchiveInner(w http.ResponseWriter, r *http.Request) {
	tempID := r.PathValue("tempId")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("archive inner preview stream: tempId=" + tempID))
}

// CheckUpload 上传预检。
func (h *Handler) CheckUpload(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileTransferExec) {
		return
	}
	var req map[string]any
	if err := web.DecodeJSON(r, &req); err != nil {
		web.WriteError(w, http.StatusBadRequest, string(code.BadRequest), "invalid request body")
		return
	}
	in := genCheckToInitInput(req)
	skip, taskID, err := h.svc.PrecheckUpload(in, currentStorageSettingID(r))
	if err != nil {
		web.WriteError(w, http.StatusBadRequest, string(code.BadRequest), err.Error())
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(map[string]any{
		"skipUpload": skip,
		"taskId":     taskID,
		"uploadId":   taskID,
	}))
}

// InitUpload 初始化上传任务。
func (h *Handler) InitUpload(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileTransferExec) {
		return
	}
	var req map[string]any
	if err := web.DecodeJSON(r, &req); err != nil {
		web.WriteError(w, http.StatusBadRequest, string(code.BadRequest), "invalid request body")
		return
	}
	taskID, err := h.svc.InitUpload(genCheckToInitInput(req), currentStorageSettingID(r))
	if err != nil {
		web.WriteError(w, http.StatusBadRequest, string(code.BadRequest), err.Error())
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(map[string]any{"taskId": taskID, "uploadId": taskID}))
}

// UploadChunk 上传分片。
func (h *Handler) UploadChunk(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileTransferExec) {
		return
	}
	taskID := strings.TrimSpace(r.URL.Query().Get("taskId"))
	if taskID == "" {
		taskID = strings.TrimSpace(r.FormValue("taskId"))
	}
	chunkIndex := intQuery(r, "chunkIndex", 0)
	if chunkIndex <= 0 {
		if v, err := strconv.Atoi(strings.TrimSpace(r.FormValue("chunkIndex"))); err == nil {
			chunkIndex = v
		}
	}
	chunkMd5 := strings.TrimSpace(r.URL.Query().Get("chunkMd5"))
	if chunkMd5 == "" {
		chunkMd5 = strings.TrimSpace(r.FormValue("chunkMd5"))
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		web.WriteError(w, http.StatusBadRequest, string(code.BadRequest), "file part is required")
		return
	}
	defer func() { _ = file.Close() }()
	chunk, err := io.ReadAll(file)
	if err != nil {
		web.WriteError(w, http.StatusBadRequest, string(code.BadRequest), "read chunk failed")
		return
	}
	if err = h.svc.UploadChunk(taskID, chunkIndex, chunk, chunkMd5); err != nil {
		web.WriteError(w, http.StatusBadRequest, string(code.BadRequest), err.Error())
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(map[string]any{
		"taskId":     taskID,
		"chunkIndex": chunkIndex,
		"uploaded":   true,
	}))
}

// MergeChunks 合并上传分片。
func (h *Handler) MergeChunks(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileTransferExec) {
		return
	}
	taskID := strings.TrimSpace(r.PathValue("taskId"))
	if taskID == "" {
		var req map[string]any
		if err := web.DecodeJSON(r, &req); err == nil {
			taskID = stringField(req, "taskId", "uploadId")
		}
	}
	if taskID == "" {
		web.WriteError(w, http.StatusBadRequest, string(code.BadRequest), "taskId is required")
		return
	}
	item, err := h.svc.MergeUpload(r.Context(), taskID)
	if err != nil {
		web.WriteError(w, http.StatusBadRequest, string(code.BadRequest), err.Error())
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(item))
}

func (h *Handler) PauseTransfer(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileTransferExec) {
		return
	}
	taskID := strings.TrimSpace(r.PathValue("taskId"))
	if err := h.svc.PauseTransfer(taskID); err != nil {
		web.WriteError(w, http.StatusBadRequest, string(code.BadRequest), err.Error())
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(map[string]any{"taskId": taskID, "status": "PAUSED"}))
}

func (h *Handler) ResumeTransfer(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileTransferExec) {
		return
	}
	taskID := strings.TrimSpace(r.PathValue("taskId"))
	if err := h.svc.ResumeTransfer(taskID); err != nil {
		web.WriteError(w, http.StatusBadRequest, string(code.BadRequest), err.Error())
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(map[string]any{"taskId": taskID, "status": "UPLOADING"}))
}

func (h *Handler) CancelUpload(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileTransferExec) {
		return
	}
	taskID := strings.TrimSpace(r.PathValue("taskId"))
	if err := h.svc.CancelTransfer(taskID); err != nil {
		web.WriteError(w, http.StatusBadRequest, string(code.BadRequest), err.Error())
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(map[string]any{"taskId": taskID, "status": "CANCELED"}))
}

func (h *Handler) GetTransferFiles(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileTransferRead) {
		return
	}
	web.WriteJSON(w, http.StatusOK, ok(h.svc.ListTransferTasks()))
}

func (h *Handler) GetUploadedChunks(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileTransferRead) {
		return
	}
	// 当前实现未持久化分片索引，返回空列表用于前端兼容。
	web.WriteJSON(w, http.StatusOK, ok([]int{}))
}

func (h *Handler) GetDownloadedChunks(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileTransferRead) {
		return
	}
	web.WriteJSON(w, http.StatusOK, ok([]int{}))
}

func (h *Handler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileRead) {
		return
	}
	fileID := strings.TrimSpace(r.PathValue("fileId"))
	rc, info, item, err := h.svc.OpenPreviewContent(r.Context(), fileID)
	if err != nil {
		web.WriteError(w, http.StatusNotFound, string(code.NotFound), err.Error())
		return
	}
	// 优先走统一流输出，便于后端审计与权限控制；无对象时再回退到预签名URL跳转。
	if rc == nil {
		url, _, urlErr := h.svc.ResolveDownloadURL(r.Context(), fileID, 10*time.Minute)
		if urlErr != nil || strings.TrimSpace(url) == "" {
			web.WriteError(w, http.StatusNotFound, string(code.NotFound), "download source not found")
			return
		}
		http.Redirect(w, r, url, http.StatusFound)
		return
	}
	defer func() { _ = rc.Close() }()

	fileName := strings.TrimSpace(item.Name)
	if fileName == "" {
		fileName = "download.bin"
	}
	contentType := strings.TrimSpace(info.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", sanitizeDownloadName(fileName)))
	if info.Size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	}
	if _, err = io.Copy(w, rc); err != nil && !errors.Is(err, io.EOF) {
		web.WriteError(w, http.StatusInternalServerError, string(code.InternalError), err.Error())
		return
	}
}

func (h *Handler) DownloadChunk(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileTransferRead) {
		return
	}
	web.WriteError(w, http.StatusNotImplemented, string(code.BadRequest), "download chunk not implemented")
}

func (h *Handler) ClearTransfers(w http.ResponseWriter, r *http.Request) {
	if !requireFilePermission(w, r, security.PermissionFileTransferExec) {
		return
	}
	// 简化实现：沿用取消逻辑由客户端逐条调用；此接口返回成功用于前端兼容。
	web.WriteJSON(w, http.StatusOK, ok(map[string]any{"cleared": true}))
}

func genCheckToInitInput(body map[string]any) filemodel.UploadInitInput {
	return filemodel.UploadInitInput{
		FileName:    stringField(body, "fileName"),
		FileHash:    stringField(body, "fileHash"),
		FileSize:    int64Field(body, "fileSize"),
		ContentType: stringField(body, "contentType"),
		ParentID:    stringField(body, "parentId"),
		TotalParts:  int(int64Field(body, "totalParts")),
	}
}

func ok(data any) map[string]any {
	return map[string]any{"code": "OK", "message": "success", "data": data}
}

func badRequest(w http.ResponseWriter, err error) {
	web.WriteError(w, http.StatusBadRequest, string(code.BadRequest), err.Error())
}

func errorPayload(c code.Code, msg string) map[string]any {
	return map[string]any{
		"code":    string(c),
		"message": msg,
	}
}

func readIDArray(r *http.Request) ([]string, error) {
	defer func() { _ = r.Body.Close() }()
	var ids []string
	if err := json.NewDecoder(r.Body).Decode(&ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func readJSONBody(r *http.Request) (map[string]any, error) {
	defer func() { _ = r.Body.Close() }()
	buf, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(buf))) == 0 {
		return map[string]any{}, nil
	}
	var body map[string]any
	if err = json.Unmarshal(buf, &body); err != nil {
		return nil, err
	}
	return body, nil
}

func stringField(m map[string]any, keys ...string) string {
	for _, key := range keys {
		v, ok := m[key]
		if !ok || v == nil {
			continue
		}
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func stringArrayField(m map[string]any, keys ...string) []string {
	for _, key := range keys {
		v, ok := m[key]
		if !ok || v == nil {
			continue
		}
		arr, ok := v.([]any)
		if !ok {
			continue
		}
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok2 := item.(string); ok2 && strings.TrimSpace(s) != "" {
				result = append(result, strings.TrimSpace(s))
			}
		}
		return result
	}
	return nil
}

func intQuery(r *http.Request, key string, def int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return def
	}
	return v
}

func int64Field(m map[string]any, keys ...string) int64 {
	for _, key := range keys {
		v, ok := m[key]
		if !ok || v == nil {
			continue
		}
		switch value := v.(type) {
		case float64:
			return int64(value)
		case float32:
			return int64(value)
		case int:
			return int64(value)
		case int64:
			return value
		case json.Number:
			if n, err := value.Int64(); err == nil {
				return n
			}
		case string:
			if n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil {
				return n
			}
		}
	}
	return 0
}

func currentStorageSettingID(r *http.Request) string {
	//从header获取存储id
	if settingID := strings.TrimSpace(r.Header.Get("X-Storage-Setting-Id")); settingID != "" {
		return settingID
	}
	//从ctx获取存储id
	if principal, ok := security.GetCtxInfo(r.Context()); ok {
		return strings.TrimSpace(principal.CurrentStorageSettingID)
	}
	return ""
}

func tokenPayload(scene, fileID, inner string) map[string]any {
	src := fmt.Sprintf("%s|%s|%s|%d", scene, fileID, inner, time.Now().UnixNano())
	h := sha1.Sum([]byte(src))
	return map[string]any{
		"token":  hex.EncodeToString(h[:]),
		"scene":  scene,
		"fileId": fileID,
		"inner":  inner,
	}
}

func sanitizeDownloadName(name string) string {
	replacer := strings.NewReplacer("\\", "_", "/", "_", "\"", "_", "\n", "_", "\r", "_")
	return replacer.Replace(name)
}

func requireFilePermission(w http.ResponseWriter, r *http.Request, permission string) bool {
	if _, err := security.RequirePermission(r.Context(), permission); err != nil {
		web.WriteError(w, http.StatusForbidden, string(code.NoPermission), err.Error())
		return false
	}
	return true
}
