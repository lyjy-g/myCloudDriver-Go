package api

import (
	"net/http"

	gen "myclouddrive-go/internal/share/api/gen"
	"myclouddrive-go/internal/share/service"
)

// RegisterRoutes 将 OpenAPI 生成路由与业务路由注册到标准库 ServeMux。
func RegisterRoutes(mux *http.ServeMux, svc *service.ShareService) {
	h := NewHandler(svc)
	gen.HandlerFromMux(h, mux)

	// Java 风格分享接口
	mux.HandleFunc("POST /apis/share/create", h.CreateShare)
	mux.HandleFunc("GET /apis/share/pages", h.ListMyShares)
	mux.HandleFunc("GET /apis/share/{shareId}", h.GetShareDetail)
	mux.HandleFunc("PUT /apis/share/{shareId}", h.UpdateShare)
	mux.HandleFunc("POST /apis/share/verify/code", h.VerifyShareCode)
	mux.HandleFunc("GET /apis/share/{shareId}/info", h.GetShareInfo)
	mux.HandleFunc("GET /apis/share/{shareId}/items", h.GetShareItems)
	mux.HandleFunc("GET /apis/share/{shareId}/download/{fileId}", h.DownloadShareFile)
	mux.HandleFunc("GET /apis/share/{shareId}/access/records", h.GetAccessRecords)
	mux.HandleFunc("DELETE /apis/share/cancels", h.CancelShares)
	mux.HandleFunc("DELETE /apis/share/clears", h.CancelAllShares)

	// 前端兼容接口
	mux.HandleFunc("POST /apis/shares", h.CreateShare)
	mux.HandleFunc("GET /apis/shares/mine", h.ListMyShares)
	mux.HandleFunc("GET /apis/shares/{shareId}", h.GetShareDetail)
	mux.HandleFunc("PUT /apis/shares/{shareId}", h.UpdateShare)
	mux.HandleFunc("POST /apis/shares/public/{shareId}/access", h.AccessPublicShare)
	mux.HandleFunc("GET /apis/shares/public/{shareId}/download/{fileId}", h.DownloadShareFile)
}
