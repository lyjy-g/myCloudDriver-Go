package api

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	filemodel "myclouddrive-go/internal/file/model"
	"myclouddrive-go/internal/file/service"
	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/framework/security"
)

type Handler struct {
	svc *service.FileService
}

// NewHandler 创建 file 模块的 HTTP 处理器。
func NewHandler(svc *service.FileService) *Handler {
	return &Handler{svc: svc}
}

// GetHomes 返回首页概览信息，如空间使用量和最近文件。
func (h *Handler) GetHomes(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileRead) {
		return
	}
	home := h.svc.Home(c.Request.Context())
	c.JSON(http.StatusOK, ok(map[string]any{"usedBytes": home.UsedBytes, "recent": home.Recent}))
}

// CreateDirectory 在指定父目录下创建新目录。
func (h *Handler) CreateDirectory(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileWrite) {
		return
	}
	h.handleWrite(c, "file.directory.create", func(body []byte) (int, any, error) {
		payload, err := decodeBodyMap(body)
		if err != nil {
			return http.StatusBadRequest, errorPayload(code.BadRequest, err.Error()), nil
		}
		parentID := stringField(payload, "parentId", "parent_id", "pid")
		name := stringField(payload, "name", "dirName")
		item, err := h.svc.CreateDirectory(c.Request.Context(), parentID, name, currentStorageSettingID(c))
		if err != nil {
			return http.StatusBadRequest, errorPayload(code.BadRequest, err.Error()), nil
		}
		return http.StatusOK, ok(item), nil
	})
}

// RenameFile 修改指定文件或目录的展示名称。
func (h *Handler) RenameFile(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileWrite) {
		return
	}
	h.handleWrite(c, "file.rename", func(body []byte) (int, any, error) {
		payload, err := decodeBodyMap(body)
		if err != nil {
			return http.StatusBadRequest, errorPayload(code.BadRequest, err.Error()), nil
		}
		name := stringField(payload, "newName", "name")
		item, err := h.svc.Rename(c.Param("fileId"), name)
		if err != nil {
			return http.StatusBadRequest, errorPayload(code.BadRequest, err.Error()), nil
		}
		return http.StatusOK, ok(item), nil
	})
}

// MoveFile 把一组文件或目录移动到目标目录下。
func (h *Handler) MoveFile(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileWrite) {
		return
	}
	h.handleWrite(c, "file.move", func(body []byte) (int, any, error) {
		payload, err := decodeBodyMap(body)
		if err != nil {
			return http.StatusBadRequest, errorPayload(code.BadRequest, err.Error()), nil
		}
		ids := stringArrayField(payload, "fileIds", "ids", "file_ids")
		targetParentID := stringField(payload, "targetParentId", "target_parent_id", "parentId")
		if len(ids) == 0 || strings.TrimSpace(targetParentID) == "" {
			return http.StatusBadRequest, errorPayload(code.BadRequest, "fileIds and targetParentId are required"), nil
		}
		if err = h.svc.Move(ids, targetParentID); err != nil {
			return http.StatusBadRequest, errorPayload(code.BadRequest, err.Error()), nil
		}
		return http.StatusOK, ok(map[string]any{"moved": len(ids)}), nil
	})
}

// GetList 返回指定目录下的文件列表，并支持关键字筛选。
func (h *Handler) GetList(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileRead) {
		return
	}
	items := h.svc.List(c.Request.Context(), c.Query("parentId"), c.Query("keyword"), currentStorageSettingID(c))
	c.JSON(http.StatusOK, ok(map[string]any{"total": len(items), "items": items}))
}

// GetDirs 返回指定目录下的子目录列表。
func (h *Handler) GetDirs(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileRead) {
		return
	}
	items := h.svc.ListDirs(c.Request.Context(), c.Query("parentId"), currentStorageSettingID(c))
	c.JSON(http.StatusOK, ok(items))
}

// GetFileDetails 返回单个文件或目录的详情。
func (h *Handler) GetFileDetails(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileRead) {
		return
	}
	item, err := h.svc.Get(c.Request.Context(), c.Param("fileId"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ok(item))
}

// GetDirectoryPath 返回目录从根到当前节点的路径链。
func (h *Handler) GetDirectoryPath(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileRead) {
		return
	}
	items, err := h.svc.DirPath(c.Param("dirId"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ok(items))
}

// GetFileUrl 返回文件下载地址及对象键等辅助信息。
func (h *Handler) GetFileUrl(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileRead) {
		return
	}
	expireSeconds := intQuery(c, "expireSeconds", 600)
	url, item, err := h.svc.ResolveDownloadURL(c.Request.Context(), c.Param("fileId"), time.Duration(expireSeconds)*time.Second)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ok(map[string]any{"url": url, "objectKey": item.ObjectKey}))
}

// DeleteFiles 把指定文件移入回收站。
func (h *Handler) DeleteFiles(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileWrite) {
		return
	}
	h.handleWrite(c, "file.recycle.soft_delete", func(body []byte) (int, any, error) {
		ids, err := decodeIDArray(body)
		if err != nil {
			return http.StatusBadRequest, errorPayload(code.BadRequest, err.Error()), nil
		}
		h.svc.Recycle(c.Request.Context(), ids)
		return http.StatusOK, ok(map[string]any{"deleted": len(ids)}), nil
	})
}

// RestoreFile 从回收站恢复指定文件。
func (h *Handler) RestoreFile(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileWrite) {
		return
	}
	h.handleWrite(c, "file.recycle.restore", func(body []byte) (int, any, error) {
		ids, err := decodeIDArray(body)
		if err != nil {
			return http.StatusBadRequest, errorPayload(code.BadRequest, err.Error()), nil
		}
		h.svc.Restore(c.Request.Context(), ids)
		return http.StatusOK, ok(map[string]any{"restored": len(ids)}), nil
	})
}

// PermanentlyDeleteFiles 永久删除指定文件。
func (h *Handler) PermanentlyDeleteFiles(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileWrite) {
		return
	}
	h.handleWrite(c, "file.recycle.permanent_delete", func(body []byte) (int, any, error) {
		ids, err := decodeIDArray(body)
		if err != nil {
			return http.StatusBadRequest, errorPayload(code.BadRequest, err.Error()), nil
		}
		report := h.svc.PermanentlyDelete(c.Request.Context(), ids)
		return http.StatusOK, ok(report), nil
	})
}

// ClearRecycles 清空当前工作空间的回收站。
func (h *Handler) ClearRecycles(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileWrite) {
		return
	}
	h.handleWrite(c, "file.recycle.clear", func(_ []byte) (int, any, error) {
		report := h.svc.ClearRecycle(c.Request.Context())
		return http.StatusOK, ok(report), nil
	})
}

// GetRecyclePages 分页返回回收站文件列表。
func (h *Handler) GetRecyclePages(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileRead) {
		return
	}
	page := intQuery(c, "page", 1)
	size := intQuery(c, "size", 20)
	items, total := h.svc.ListRecycle(c.Request.Context(), page, size)
	c.JSON(http.StatusOK, ok(map[string]any{"page": page, "size": size, "total": total, "items": items}))
}

// FavoritesFile 把指定文件标记为收藏。
func (h *Handler) FavoritesFile(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileWrite) {
		return
	}
	h.handleWrite(c, "file.favorite.add", func(body []byte) (int, any, error) {
		ids, err := decodeIDArray(body)
		if err != nil {
			return http.StatusBadRequest, errorPayload(code.BadRequest, err.Error()), nil
		}
		h.svc.SetFavorite(ids, true)
		return http.StatusOK, ok(map[string]any{"favorite": len(ids)}), nil
	})
}

// UnFavoritesFile 取消指定文件的收藏标记。
func (h *Handler) UnFavoritesFile(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileWrite) {
		return
	}
	h.handleWrite(c, "file.favorite.remove", func(body []byte) (int, any, error) {
		ids, err := decodeIDArray(body)
		if err != nil {
			return http.StatusBadRequest, errorPayload(code.BadRequest, err.Error()), nil
		}
		h.svc.SetFavorite(ids, false)
		return http.StatusOK, ok(map[string]any{"unfavorite": len(ids)}), nil
	})
}

// PreviewToken 生成普通文件预览令牌。
func (h *Handler) PreviewToken(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileRead) {
		return
	}
	h.handleWrite(c, "file.preview.token", func(_ []byte) (int, any, error) {
		return http.StatusOK, ok(tokenPayload("preview", c.Param("fileId"), "")), nil
	})
}

// ArchivePreviewToken 生成压缩包内文件预览令牌。
func (h *Handler) ArchivePreviewToken(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileRead) {
		return
	}
	h.handleWrite(c, "file.archive.preview.token", func(_ []byte) (int, any, error) {
		return http.StatusOK, ok(tokenPayload("archive-preview", c.Param("archiveFileId"), c.Query("innerPath"))), nil
	})
}

// handleWrite 是 Gin 写接口的统一入口。
// 它做三件事：
// 1. 先把 body 读出来，作为幂等键校验输入的一部分；
// 2. 再把 body 放回 request，避免后续逻辑读不到；
// 3. 最后交给 service 做真正的幂等执行与重放判断。
func (h *Handler) handleWrite(c *gin.Context, endpoint string, execute func(body []byte) (int, any, error)) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request body"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	idemKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if idemKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Idempotency-Key header is required for write operations"})
		return
	}

	statusCode, payload, replayed, execErr := h.svc.ExecuteIdempotent(endpoint, idemKey, body, func() (int, any, error) {
		return execute(body)
	})
	if execErr != nil {
		switch {
		case errors.Is(execErr, service.ErrIdempotencyConflict), errors.Is(execErr, service.ErrIdempotencyInProgress):
			c.JSON(http.StatusConflict, gin.H{"code": 400, "message": execErr.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": execErr.Error()})
		}
		return
	}
	if replayed {
		c.Header("X-Idempotent-Replayed", "true")
	}
	c.JSON(statusCode, payload)
}

// Preview 直接输出文件预览流。
func (h *Handler) Preview(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileRead) {
		return
	}
	rc, info, item, err := h.svc.OpenPreviewContent(c.Request.Context(), c.Param("fileId"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	if rc == nil {
		c.Header("Content-Type", "text/plain; charset=utf-8")
		_, _ = c.Writer.Write([]byte(fmt.Sprintf("preview stream: fileId=%s name=%s objectKey=%s", item.ID, item.Name, item.ObjectKey)))
		return
	}
	defer rc.Close()

	if strings.TrimSpace(info.ContentType) != "" {
		c.Header("Content-Type", info.ContentType)
	} else {
		c.Header("Content-Type", "application/octet-stream")
	}
	if info.Size > 0 {
		c.Header("Content-Length", strconv.FormatInt(info.Size, 10))
	}
	if _, err = io.Copy(c.Writer, rc); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
}

// PreviewArchiveInner 输出压缩包内文件预览流。
func (h *Handler) PreviewArchiveInner(c *gin.Context) {
	c.Header("Content-Type", "text/plain; charset=utf-8")
	_, _ = c.Writer.Write([]byte("archive inner preview stream: tempId=" + c.Param("tempId")))
}

// CheckUpload 执行上传预检，判断是否可秒传或需要继续分片上传。
func (h *Handler) CheckUpload(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileTransferExec) {
		return
	}
	var req map[string]any
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request body"})
		return
	}
	result, err := h.svc.PrecheckUpload(c.Request.Context(), genCheckToInitInput(req), currentStorageSettingID(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ok(result))
}

// InitUpload 初始化一个新的上传任务。
func (h *Handler) InitUpload(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileTransferExec) {
		return
	}
	var req map[string]any
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request body"})
		return
	}
	taskID, err := h.svc.InitUpload(genCheckToInitInput(req), currentStorageSettingID(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ok(map[string]any{"taskId": taskID, "uploadId": taskID}))
}

// UploadChunk 接收上传任务的单个分片。
func (h *Handler) UploadChunk(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileTransferExec) {
		return
	}
	taskID := strings.TrimSpace(c.Query("taskId"))
	if taskID == "" {
		taskID = strings.TrimSpace(c.PostForm("taskId"))
	}
	chunkIndex := intQuery(c, "chunkIndex", 0)
	if chunkIndex <= 0 {
		if v, err := strconv.Atoi(strings.TrimSpace(c.PostForm("chunkIndex"))); err == nil {
			chunkIndex = v
		}
	}
	chunkSha256 := strings.TrimSpace(c.Query("chunkSha256"))
	if chunkSha256 == "" {
		chunkSha256 = strings.TrimSpace(c.PostForm("chunkSha256"))
	}
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "file part is required"})
		return
	}
	defer file.Close()
	chunk, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "read chunk failed"})
		return
	}
	if err = h.svc.UploadChunk(taskID, chunkIndex, chunk, chunkSha256); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ok(map[string]any{"taskId": taskID, "chunkIndex": chunkIndex, "uploaded": true}))
}

// MergeChunks 触发上传任务的分片合并。
func (h *Handler) MergeChunks(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileTransferExec) {
		return
	}
	taskID := strings.TrimSpace(c.Param("taskId"))
	if taskID == "" {
		var req map[string]any
		if err := c.ShouldBindJSON(&req); err == nil {
			taskID = stringField(req, "taskId", "uploadId")
		}
	}
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "taskId is required"})
		return
	}
	item, err := h.svc.MergeUpload(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ok(item))
}

// PauseTransfer 暂停指定上传任务。
func (h *Handler) PauseTransfer(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileTransferExec) {
		return
	}
	taskID := c.Param("taskId")
	if err := h.svc.PauseTransfer(taskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ok(map[string]any{"taskId": taskID, "status": "PAUSED"}))
}

// ResumeTransfer 恢复指定上传任务。
func (h *Handler) ResumeTransfer(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileTransferExec) {
		return
	}
	taskID := c.Param("taskId")
	if err := h.svc.ResumeTransfer(taskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ok(map[string]any{"taskId": taskID, "status": "UPLOADING"}))
}

// CancelUpload 取消指定上传任务。
func (h *Handler) CancelUpload(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileTransferExec) {
		return
	}
	taskID := c.Param("taskId")
	if err := h.svc.CancelTransfer(taskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ok(map[string]any{"taskId": taskID, "status": "CANCELED"}))
}

// GetTransferFiles 返回当前传输任务列表。
func (h *Handler) GetTransferFiles(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileTransferRead) {
		return
	}
	c.JSON(http.StatusOK, ok(h.svc.ListTransferTasks()))
}

// GetUploadedChunks 返回已上传分片列表。
func (h *Handler) GetUploadedChunks(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileTransferRead) {
		return
	}
	c.JSON(http.StatusOK, ok([]int{}))
}

// GetDownloadedChunks 返回已下载分片列表。
func (h *Handler) GetDownloadedChunks(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileTransferRead) {
		return
	}
	c.JSON(http.StatusOK, ok([]int{}))
}

// DownloadFile 下载指定文件。
func (h *Handler) DownloadFile(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileRead) {
		return
	}
	fileID := strings.TrimSpace(c.Param("fileId"))
	rc, info, item, err := h.svc.OpenPreviewContent(c.Request.Context(), fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	if rc == nil {
		url, _, urlErr := h.svc.ResolveDownloadURL(c.Request.Context(), fileID, 10*time.Minute)
		if urlErr != nil || strings.TrimSpace(url) == "" {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "download source not found"})
			return
		}
		c.Redirect(http.StatusFound, url)
		return
	}
	defer rc.Close()

	fileName := strings.TrimSpace(item.Name)
	if fileName == "" {
		fileName = "download.bin"
	}
	contentType := strings.TrimSpace(info.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", sanitizeDownloadName(fileName)))
	if info.Size > 0 {
		c.Header("Content-Length", strconv.FormatInt(info.Size, 10))
	}
	if _, err = io.Copy(c.Writer, rc); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
}

// DownloadChunk 下载指定分片。
func (h *Handler) DownloadChunk(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileTransferRead) {
		return
	}
	c.JSON(http.StatusNotImplemented, gin.H{"code": 400, "message": "download chunk not implemented"})
}

// ClearTransfers 清空或结束当前传输任务集合。
func (h *Handler) ClearTransfers(c *gin.Context) {
	if !requireFilePermission(c, security.PermissionFileTransferExec) {
		return
	}
	c.JSON(http.StatusOK, ok(map[string]any{"cleared": true}))
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

func errorPayload(c code.Code, msg string) map[string]any {
	return map[string]any{"code": string(c), "message": msg}
}

func decodeIDArray(body []byte) ([]string, error) {
	var ids []string
	if len(strings.TrimSpace(string(body))) == 0 {
		return ids, nil
	}
	err := json.Unmarshal(body, &ids)
	return ids, err
}

func decodeBodyMap(body []byte) (map[string]any, error) {
	if len(strings.TrimSpace(string(body))) == 0 {
		return map[string]any{}, nil
	}
	var payload map[string]any
	err := json.Unmarshal(body, &payload)
	return payload, err
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

func intQuery(c *gin.Context, key string, def int) int {
	raw := strings.TrimSpace(c.Query(key))
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

// currentStorageSettingID 统一封装“当前请求到底走哪个存储配置”的读取逻辑。
// 优先级是：
// 1. 显式请求头；
// 2. 中间件已经写入的 ctx；
// 这样 handler 不需要自己关心存储切换细节。
func currentStorageSettingID(c *gin.Context) string {
	if settingID := strings.TrimSpace(c.GetHeader("X-Storage-Setting-Id")); settingID != "" {
		return settingID
	}
	if principal, ok := security.GetCtxInfo(c.Request.Context()); ok {
		return strings.TrimSpace(principal.CurrentStorageSettingID)
	}
	return ""
}

func tokenPayload(scene, fileID, inner string) map[string]any {
	src := fmt.Sprintf("%s|%s|%s|%d", scene, fileID, inner, time.Now().UnixNano())
	h := sha1.Sum([]byte(src))
	return map[string]any{"token": hex.EncodeToString(h[:]), "scene": scene, "fileId": fileID, "inner": inner}
}

func sanitizeDownloadName(name string) string {
	replacer := strings.NewReplacer("\\", "_", "/", "_", "\"", "_", "\n", "_", "\r", "_")
	return replacer.Replace(name)
}

func requireFilePermission(c *gin.Context, permission string) bool {
	if _, err := security.RequirePermission(c.Request.Context(), permission); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": err.Error()})
		return false
	}
	return true
}
