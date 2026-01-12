package mproxy

type Logger interface {
	Printf(format string, v ...any)
}
