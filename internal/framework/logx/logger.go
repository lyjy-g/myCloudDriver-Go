package logx

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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
	_, _ = l.body.Write(b)
	return l.ResponseWriter.Write(b)
}

func (l *loggingResponseWriter) Flush() {
	if f, ok := l.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
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

// GinLoggingMiddleware 记录 Gin 请求日志，输出字段保持与旧实现一致。
//
// 这里没有直接复用 Gin 默认日志中间件，而是继续沿用项目自己的日志格式，
// 这样迁移框架后，排障和日志检索口径不需要跟着变。
//
// 另外它不是只打一行 access log：
// - 会记录请求体摘要；
// - 会记录响应体摘要；
// - 对 4xx/5xx 排障比默认日志更直接。
func GinLoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		reqBody := readRequestBody(c.Request)

		// 包一层 writer，是为了同时拿到：
		// 1. 最终状态码；
		// 2. 响应体摘要；
		// 3. 仍然满足 gin.ResponseWriter 接口。
		lrw := &loggingResponseWriter{ResponseWriter: c.Writer, status: http.StatusOK}
		c.Writer = &ginResponseWriter{loggingResponseWriter: lrw, ResponseWriterExt: c.Writer}
		c.Next()

		duration := time.Since(start)
		log.Printf(
			"http_request method=%s path=%s query=%q status=%d duration=%s remote=%s ua=%q referer=%q content_type=%q\n  req=%q\n  resp=%q",
			c.Request.Method,
			c.Request.URL.Path,
			c.Request.URL.RawQuery,
			lrw.status,
			duration,
			c.Request.RemoteAddr,
			c.Request.UserAgent(),
			c.Request.Referer(),
			c.Request.Header.Get("Content-Type"),
			reqBody,
			trimBody(lrw.body.Bytes()),
		)
	}
}

// ginResponseWriter 复用原有日志包装，同时满足 gin.ResponseWriter 接口。
// 这层本质上是“把我们自己的记录能力适配到 Gin 的 writer 约定”。
type ginResponseWriter struct {
	*loggingResponseWriter
	ResponseWriterExt gin.ResponseWriter
}

func (g *ginResponseWriter) Header() http.Header { return g.loggingResponseWriter.Header() }
func (g *ginResponseWriter) WriteHeaderNow()     { g.ResponseWriterExt.WriteHeaderNow() }
func (g *ginResponseWriter) Pusher() http.Pusher { return g.ResponseWriterExt.Pusher() }
func (g *ginResponseWriter) CloseNotify() <-chan bool {
	return g.ResponseWriterExt.CloseNotify()
}
func (g *ginResponseWriter) Status() int                       { return g.loggingResponseWriter.status }
func (g *ginResponseWriter) Size() int                         { return g.ResponseWriterExt.Size() }
func (g *ginResponseWriter) Written() bool                     { return g.ResponseWriterExt.Written() }
func (g *ginResponseWriter) WriteString(s string) (int, error) { return g.Write([]byte(s)) }
func (g *ginResponseWriter) WriteHeader(code int)              { g.loggingResponseWriter.WriteHeader(code) }
func (g *ginResponseWriter) Flush()                            { g.loggingResponseWriter.Flush() }
func (g *ginResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return g.ResponseWriterExt.Hijack()
}
func (g *ginResponseWriter) Unwrap() http.ResponseWriter {
	return g.loggingResponseWriter.ResponseWriter
}
