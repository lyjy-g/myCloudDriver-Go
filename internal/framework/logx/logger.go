package logx

// Logger 是日志能力抽象，后续可接入 zap/slog。
type Logger interface {
	Info(msg string)
	Error(msg string)
}
