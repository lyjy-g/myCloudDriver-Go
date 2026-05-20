package api

import (
	"github.com/gin-gonic/gin"

	"myclouddrive-go/internal/file/service"
)

// RegisterRoutes 注册 file 模块路由。
// 这里按“首页/上传传输/文件管理/预览下载”几类能力顺序展开，方便后续继续维护。
func RegisterRoutes(router gin.IRouter, svc *service.FileService) {
	h := NewHandler(svc)

	// 查询首页概览。
	router.GET("/apis/home/info", h.GetHomes)
	// 上传预检。
	router.POST("/apis/transfer/check", h.CheckUpload)
	// 初始化上传任务。
	router.POST("/apis/transfer/init", h.InitUpload)
	// 上传单个分片。
	router.POST("/apis/transfer/chunk", h.UploadChunk)
	// 合并上传分片。
	router.POST("/apis/transfer/merge/:taskId", h.MergeChunks)
	// 暂停上传任务。
	router.POST("/apis/transfer/pause/:taskId", h.PauseTransfer)
	// 恢复上传任务。
	router.POST("/apis/transfer/resume/:taskId", h.ResumeTransfer)
	// 取消上传任务。
	router.DELETE("/apis/transfer/cancel/:taskId", h.CancelUpload)
	// 查询传输任务列表。
	router.GET("/apis/transfer/files", h.GetTransferFiles)
	// 查询已上传分片。
	router.GET("/apis/transfer/chunks/:taskId", h.GetUploadedChunks)
	// 查询已下载分片。
	router.GET("/apis/transfer/download/chunks/:taskId", h.GetDownloadedChunks)
	// 下载文件。
	router.GET("/apis/transfer/download/:fileId", h.DownloadFile)
	// 下载单个分片。
	router.GET("/apis/transfer/download/chunk", h.DownloadChunk)
	// 清理传输任务。
	router.DELETE("/apis/transfer/clears", h.ClearTransfers)

	// 重命名文件或目录。
	router.PUT("/apis/file/:fileId/rename", h.RenameFile)
	// 从回收站恢复文件。
	router.PUT("/apis/file/recycles", h.RestoreFile)
	// 永久删除回收站文件。
	router.DELETE("/apis/file/recycles", h.PermanentlyDeleteFiles)
	// 移动文件或目录。
	router.PUT("/apis/file/moves", h.MoveFile)
	// 生成文件预览 token。
	router.POST("/preview/token/:fileId", h.PreviewToken)
	// 生成压缩包内文件预览 token。
	router.POST("/archive/preview/token/:archiveFileId", h.ArchivePreviewToken)
	// 收藏文件。
	router.POST("/apis/file/favorites", h.FavoritesFile)
	// 取消收藏文件。
	router.DELETE("/apis/file/favorites", h.UnFavoritesFile)
	// 创建目录。
	router.POST("/apis/file/directory", h.CreateDirectory)
	// 查询文件详情。
	router.GET("/apis/file/:fileId", h.GetFileDetails)
	// 获取文件下载地址。
	router.GET("/apis/file/url/:fileId", h.GetFileUrl)
	// 分页查询回收站。
	router.GET("/apis/file/recycle/pages", h.GetRecyclePages)
	// 查询文件列表。
	router.GET("/apis/file/list", h.GetList)
	// 查询目录列表。
	router.GET("/apis/file/dirs", h.GetDirs)
	// 查询目录路径链。
	router.GET("/apis/file/directory/:dirId/path", h.GetDirectoryPath)
	// 预览文件流。
	router.GET("/api/file/stream/preview/:fileId", h.Preview)
	// 预览压缩包内文件流。
	router.GET("/api/file/stream/preview/archive/inner/:tempId", h.PreviewArchiveInner)
	// 移入回收站。
	router.DELETE("/apis/file", h.DeleteFiles)
	// 清空回收站。
	router.DELETE("/apis/file/recycles/clear", h.ClearRecycles)
}
