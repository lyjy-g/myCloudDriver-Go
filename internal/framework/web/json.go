package web

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse 定义统一错误响应结构。
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Result 定义统一响应结构。
type Result[T any] struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

// DecodeJSON 解码请求体 JSON。
func DecodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

// WriteJSON 输出 JSON 响应。
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// WriteError 输出统一错误响应。
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, ErrorResponse{
		Code:    code,
		Message: message,
	})
}
