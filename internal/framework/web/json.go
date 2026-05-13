package web

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

// ErrorResponse 定义统一错误响应结构。
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Result 定义统一响应结构。
type Result[T any] struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

// DecodeJSON 解码请求体 JSON。
func DecodeJSON(req *http.Request, dst any) error {
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

// WriteJSON 输出 JSON 响应。
func WriteJSON(resp http.ResponseWriter, status int, data any) {
	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(status)
	_ = json.NewEncoder(resp).Encode(data)
}

func normalizeErrorCode(code any) int {
	switch v := code.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case uint:
		return int(v)
	case uint32:
		return int(v)
	case uint64:
		return int(v)
	case string:
		switch strings.ToUpper(strings.TrimSpace(v)) {
		case "BAD_REQUEST":
			return 400
		case "NO_PERMISSION":
			return 403
		case "NOT_FOUND":
			return 404
		case "INTERNAL_ERROR":
			return 500
		default:
			return 500
		}
	default:
		return 500
	}
}

// WriteError 输出统一错误响应。
// code 参数兼容 int 以及字符串错误码（如 BAD_REQUEST/NOT_FOUND）。
func WriteError(w http.ResponseWriter, status int, code any, message string) {
	normalizedCode := normalizeErrorCode(code)
	// 统一在服务端输出错误日志，便于联调与排障。
	log.Printf("http_error status=%d code=%d message=%q", status, normalizedCode, message)
	WriteJSON(w, status, ErrorResponse{
		Code:    normalizedCode,
		Message: message,
	})
}
