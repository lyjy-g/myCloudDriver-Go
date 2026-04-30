package api

import (
	"net/http"

	gen "myclouddrive-go/internal/file/api/gen"
	"myclouddrive-go/internal/file/service"
)

// RegisterRoutes 将 OpenAPI 生成路由注册到标准库 ServeMux。
func RegisterRoutes(mux *http.ServeMux, svc *service.FileService) {
	h := NewHandler(svc)
	// 保留 OpenAPI 生成的 ping 路由。
	gen.HandlerFromMux(h, mux)

	// 业务路由（按 Java 模块接口实现）。
	mux.HandleFunc("GET /apis/home/info", h.GetHomes)
	mux.HandleFunc("POST /apis/transfer/check", h.CheckUpload)
	mux.HandleFunc("POST /apis/transfer/init", h.InitUpload)
	mux.HandleFunc("POST /apis/transfer/chunk", h.UploadChunk)
	mux.HandleFunc("POST /apis/transfer/merge/{taskId}", h.MergeChunks)
	mux.HandleFunc("POST /apis/transfer/pause/{taskId}", h.PauseTransfer)
	mux.HandleFunc("POST /apis/transfer/resume/{taskId}", h.ResumeTransfer)
	mux.HandleFunc("DELETE /apis/transfer/cancel/{taskId}", h.CancelUpload)
	mux.HandleFunc("GET /apis/transfer/files", h.GetTransferFiles)
	mux.HandleFunc("GET /apis/transfer/chunks/{taskId}", h.GetUploadedChunks)
	mux.HandleFunc("GET /apis/transfer/download/chunks/{taskId}", h.GetDownloadedChunks)
	mux.HandleFunc("GET /apis/transfer/download/{fileId}", h.DownloadFile)
	mux.HandleFunc("GET /apis/transfer/download/chunk", h.DownloadChunk)
	mux.HandleFunc("DELETE /apis/transfer/clears", h.ClearTransfers)

	// 兼容旧版上传接口（前端降级候选地址）。
	mux.HandleFunc("POST /apis/upload/precheck", h.CheckUpload)
	mux.HandleFunc("POST /apis/upload/part", h.UploadChunk)
	mux.HandleFunc("POST /apis/upload/merge", h.MergeChunks)

	mux.HandleFunc("PUT /apis/file/{fileId}/rename", h.RenameFile)
	mux.HandleFunc("PUT /apis/file/recycles", h.RestoreFile)
	mux.HandleFunc("DELETE /apis/file/recycles", h.PermanentlyDeleteFiles)
	mux.HandleFunc("PUT /apis/file/moves", h.MoveFile)
	mux.HandleFunc("POST /preview/token/{fileId}", h.PreviewToken)
	mux.HandleFunc("POST /archive/preview/token/{archiveFileId}", h.ArchivePreviewToken)
	mux.HandleFunc("POST /apis/file/favorites", h.FavoritesFile)
	mux.HandleFunc("DELETE /apis/file/favorites", h.UnFavoritesFile)
	mux.HandleFunc("POST /apis/file/directory", h.CreateDirectory)
	mux.HandleFunc("GET /apis/file/{fileId}", h.GetFileDetails)
	mux.HandleFunc("GET /apis/file/url/{fileId}", h.GetFileUrl)
	mux.HandleFunc("GET /apis/file/recycle/pages", h.GetRecyclePages)
	mux.HandleFunc("GET /apis/file/list", h.GetList)
	mux.HandleFunc("GET /apis/file/dirs", h.GetDirs)
	mux.HandleFunc("GET /apis/file/directory/{dirId}/path", h.GetDirectoryPath)
	mux.HandleFunc("GET /api/file/stream/preview/{fileId}", h.Preview)
	mux.HandleFunc("GET /api/file/stream/preview/archive/inner/{tempId}", h.PreviewArchiveInner)
	mux.HandleFunc("DELETE /apis/file", h.DeleteFiles)
	mux.HandleFunc("DELETE /apis/file/recycles/clear", h.ClearRecycles)
}
