package log

import (
	"sync"

	"pusher/env"
	"pusher/internal/logger"
)

var (
	appLogger  logger.LoggingInterface
	loggerOnce sync.Once
)

func init() {
	Logger()
}

func initLogger() {
	appLogger = logger.NewAppLogger(
		logger.WithLogLevel(env.GetString("LOG_LEVEL", "info")),
	)
}

// Logger returns the process-wide logger.
func Logger() logger.LoggingInterface {
	if appLogger == nil {
		loggerOnce.Do(initLogger)
	}
	return appLogger
}
