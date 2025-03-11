package log

import (
	"github.com/sirupsen/logrus"
	"os"
	"pusher/env"
	"pusher/internal/constants"
	"sync"
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
	if env.Get("APP_ENV", constants.DEVELOPMENT) == constants.PRODUCTION {
		logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		// The TextFormatter is default, you don't actually have to do this.
		logger.SetFormatter(&logrus.TextFormatter{})
	}
	logger.SetOutput(os.Stdout)
	lvl, err := logrus.ParseLevel(env.GetString("LOG_LEVEL", "debug"))
	if err != nil {
		logger.Infoln("Invalid log level:", env.GetString("LOG_LEVEL", "debug"), lvl)
		logger.SetLevel(logrus.InfoLevel)
	} else {
		logger.Infoln("Log level:", lvl)
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
