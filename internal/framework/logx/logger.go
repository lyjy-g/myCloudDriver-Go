package logx

import (
	"log"
	"net/http"
	"time"
)

// Logger 是日志能力抽象，后续可接入 zap/slog。
type Logger interface {
	Info(msg string)
	Error(msg string)
}

type loggingResponseWriter struct {
	ResponseWriter http.ResponseWriter
	status         int
}

func (l loggingResponseWriter) Header() http.Header {
	return l.ResponseWriter.Header()
}

func (l loggingResponseWriter) Write(bytes []byte) (int, error) {
	return l.ResponseWriter.Write(bytes)
}

func (l loggingResponseWriter) WriteHeader(statusCode int) {
	// 记录状态码
	l.status = statusCode
	l.ResponseWriter.WriteHeader(statusCode)
}

// LoggingMiddleware 记录通用 HTTP 请求日志。
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// 记录请求开始时间
		start := time.Now()

		// 使用 loggingResponseWriter 捕获 HTTP 状态码
		lrw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}

		// 调用实际的 handler
		next.ServeHTTP(lrw, req)

		// 日志输出请求信息
		log.Printf(
			"method=%s path=%s status=%d duration=%s remote=%s",
			req.Method,
			req.URL.Path,
			lrw.status,
			time.Since(start),
			req.RemoteAddr,
		)
	})
}
