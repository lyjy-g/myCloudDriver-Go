package logx

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Logger 是日志能力抽象，后续可接入 zap/slog。
type Logger interface {
	Info(msg string)
	Error(msg string)
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (l *loggingResponseWriter) WriteHeader(statusCode int) {
	// 捕获响应状态码，避免默认 200 丢失。
	l.status = statusCode
	l.ResponseWriter.WriteHeader(statusCode)
}

func (l *loggingResponseWriter) Write(b []byte) (int, error) {
	// 缓存响应体摘要用于日志，不影响真实响应写出。
	_, _ = l.body.Write(b)
	return l.ResponseWriter.Write(b)
}

const maxLogBodyBytes = 2048

func readRequestBody(req *http.Request) string {
	if req == nil || req.Body == nil {
		return ""
	}
	raw, err := io.ReadAll(req.Body)
	if err != nil {
		return "[read body failed]"
	}
	// 还原 body，避免影响后续 handler 读取。
	req.Body = io.NopCloser(bytes.NewBuffer(raw))
	return trimBody(raw)
}

func trimBody(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	if len(raw) > maxLogBodyBytes {
		return strings.TrimSpace(string(raw[:maxLogBodyBytes])) + "...[truncated]"
	}
	return strings.TrimSpace(string(raw))
}

// LoggingMiddleware 记录通用 HTTP 请求日志。
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// 记录请求开始时间
		start := time.Now()
		reqBody := readRequestBody(req)

		// 使用包装 writer 捕获 HTTP 状态码。
		lrw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}

		// 调用实际的 handler
		next.ServeHTTP(lrw, req)

		duration := time.Since(start)
		// 4xx/5xx 额外输出错误上下文，便于快速排查。
		log.Printf(
			"http_request method=%s path=%s query=%q status=%d duration=%s remote=%s ua=%q referer=%q content_type=%q\n  req=%q\n  resp=%q",
			req.Method,
			req.URL.Path,
			req.URL.RawQuery,
			lrw.status,
			duration,
			req.RemoteAddr,
			req.UserAgent(),
			req.Referer(),
			req.Header.Get("Content-Type"),
			reqBody,
			trimBody(lrw.body.Bytes()),
		)
	})
}
