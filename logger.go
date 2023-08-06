package libmangal

import (
	"io"
	"log"
)

type Logger struct {
	onLog  func(string)
	logger *log.Logger
}

func NewLogger() *Logger {
	logger := log.New(io.Discard, "", log.Default().Flags())

	return &Logger{
		onLog:  func(string) {},
		logger: logger,
	}
}

func (l *Logger) SetPrefix(prefix string) {
	l.logger.SetPrefix(prefix)
}

func (l *Logger) Writer() io.Writer {
	return l.logger.Writer()
}

func (l *Logger) SetOutput(writer io.Writer) {
	l.logger.SetOutput(writer)
}

func (l *Logger) SetOnLog(hook func(string)) {
	l.onLog = hook
}

func (l *Logger) Log(message string) {
	if l.onLog != nil {
		l.onLog(message)
	}

	l.logger.Println(message)
}
