package log

import (
	"os"
	"sync"

	"github.com/sirupsen/logrus"
	"pusher/env"
	"pusher/internal/constants"
)

var (
	logger     *logrus.Logger
	loggerOnce sync.Once
)

func init() {
	Logger()
}

func initLogger() {
	logger = logrus.New()
	if env.GetString("APP_ENV", constants.PRODUCTION) == constants.PRODUCTION {
		logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		// The TextFormatter is default, you don't actually have to do this.
		logger.SetFormatter(&logrus.TextFormatter{})
	}

	logger.SetOutput(os.Stdout)
	lvl, err := logrus.ParseLevel(env.GetString("LOG_LEVEL", "info"))
	if err != nil {
		logger.Warnln("Invalid log level:", env.GetString("LOG_LEVEL", "info"), lvl)
		logger.SetLevel(logrus.InfoLevel)
	} else {
		logger.Debugln("Log level:", lvl)
		logger.SetLevel(lvl)
	}
}

// Logger ...
func Logger() *logrus.Logger {
	if logger == nil {
		loggerOnce.Do(func() {
			initLogger()
		})
	}
	return logger
}
