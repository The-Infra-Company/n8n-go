package n8n

type Logger interface {
	Debug(v ...any)
	Debugf(format string, v ...any)
}
