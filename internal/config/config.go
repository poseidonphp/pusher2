package config

import (
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"os"
	"pusher/env"
	"pusher/internal/constants"
	"pusher/log"
	"time"
)

// AppEnv is app env
var AppEnv string

var (
	ActivityTimeout          time.Duration
	ReadTimeout              time.Duration
	MaxPresenceUsers         int64
	MaxPresenceUserDataBytes int
)

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

// Setup loads environment variables and sets some global variables
func Setup() error {
	if fileExists("./.env") {
		err := godotenv.Load()
		if err != nil {
			log.Logger().Errorln("Error loading .env file")
			return err
		}
	}

	AppEnv = env.GetString("APP_ENV", constants.PRODUCTION)

	lvl, _ := logrus.ParseLevel(env.GetString("LOG_LEVEL", "info"))

	log.Logger().SetLevel(lvl)
	if AppEnv != constants.PRODUCTION {
		log.Logger().SetFormatter(&logrus.TextFormatter{})
	}

	log.Logger().Debugln("AppEnv:", AppEnv)
	log.Logger().Debugln("LogLevel", env.Get("LOG_LEVEL", "info"))

	//PongTimeout = time.Duration(env.GetInt64("PONG_TIMEOUT", 30)) * time.Second
	ActivityTimeout = time.Duration(env.GetInt64("ACTIVITY_TIMEOUT", 60)) * time.Second
	ReadTimeout = ActivityTimeout / 9 * 10
	MaxPresenceUsers = env.GetInt64("MAX_PRESENCE_USERS", 100)
	MaxPresenceUserDataBytes = env.GetInt("MAX_PRESENCE_USER_DATA_KB", 10) * 1024

	return nil
}
