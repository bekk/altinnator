package log

import (
	"context"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

type _ interface {
	Error(err error, msg string, keysAndValues ...any)
	Warning(msg string, keysAndValues ...any)
	Info(msg string, keysAndValues ...any)
	Debug(msg string, keysAndValues ...any)
}

type Logger struct {
	logr.Logger
}

func (l *Logger) Error(err error, msg string, keysAndValues ...any) {
	l.Logger.Error(err, msg, keysAndValues...)
}

func (l *Logger) Warning(msg string, keysAndValues ...any) {
	l.Logger.V(2).Info(msg, keysAndValues...)
}

func (l *Logger) Info(msg string, keysAndValues ...any) {
	l.Logger.Info(msg, keysAndValues...)
}

func (l *Logger) Debug(msg string, keysAndValues ...any) {
	l.Logger.V(1).Info(msg, keysAndValues...)
}

func GetLogger(ctx context.Context) Logger {
	return Logger{
		ctrl.LoggerFrom(ctx),
	}
}
