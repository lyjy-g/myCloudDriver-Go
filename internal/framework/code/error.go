package code

import "errors"

var (
	ErrObjectNotFound     = errors.New("storage: object not found")
	ErrInvalidKey         = errors.New("storage: invalid key")
	ErrDriverNotFound     = errors.New("storage: driver not found")
	ErrSettingNotFound    = errors.New("storage: setting not found")
	ErrSettingDisabled    = errors.New("storage: setting disabled")
	ErrCapabilityNotMatch = errors.New("storage: capability not supported")
)

// Error 表示统一错误对象。
type Error struct {
	Code    Code
	Message string
}

func (e *Error) Error() string {
	return string(e.Code) + ": " + e.Message
}

// New 创建统一错误对象。
func New(c Code, message string) *Error {
	return &Error{Code: c, Message: message}
}

// Is 判断错误是否为指定错误码。
func Is(err error, c Code) bool {
	if err == nil {
		return false
	}
	var ce *Error
	if errors.As(err, &ce) {
		return ce.Code == c
	}
	return false
}
