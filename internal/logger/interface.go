package logger

type LoggingInterface interface {
	Trace(args ...any)
	Tracef(format string, args ...any)

	Info(args ...any)
	Infof(format string, args ...any)

	Warn(args ...any)
	Warnf(format string, args ...any)

	Error(args ...any)
	Errorf(format string, args ...any)

	Debug(args ...any)
	Debugf(format string, args ...any)

	Fatal(args ...any)
	Fatalf(format string, args ...any)

	WithField(key string, value any) LoggingInterface
	WithFields(fields map[string]any) LoggingInterface
}
