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
