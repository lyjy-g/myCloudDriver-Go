package code

// Code 表示统一错误码。
type Code string

const (
	Unknown       Code = "UNKNOWN"
	BadRequest    Code = "BAD_REQUEST"
	NotFound      Code = "NOT_FOUND"
	NoPermission  Code = "NO_PERMISSION"
	InternalError Code = "INTERNAL_ERROR"
)
