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

const ()
const (
	writeWait      = 10 * time.Second // Time allowed to write a message to the peer.
	maxMessageSize = 1024             // Maximum message size allowed from peer.
)

var (
	PongTimeout              time.Duration
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
		log.Logger().Debugln("Loading .env file")
		err := godotenv.Load()
		if err != nil {
			log.Logger().Fatal("Error loading .env file")
			return err
		}
	}
	AppEnv = env.Get("APP_ENV", constants.DEVELOPMENT)
	log.Logger().Infoln("AppEnv:", AppEnv)
	lvl, _ := logrus.ParseLevel(env.GetString("LOG_LEVEL", "debug"))
	log.Logger().SetLevel(lvl)

	log.Logger().Infoln("LogLevel", env.Get("LOG_LEVEL", "debug"))

	PongTimeout = time.Duration(env.GetInt64("PONG_TIMEOUT", 30)) * time.Second
	ActivityTimeout = time.Duration(env.GetInt64("ACTIVITY_TIMEOUT", 60)) * time.Second
	ReadTimeout = ActivityTimeout / 9 * 10
	MaxPresenceUsers = env.GetInt64("MAX_PRESENCE_USERS", 100)
	MaxPresenceUserDataBytes = env.GetInt("MAX_PRESENCE_USER_DATA_KB", 10) * 1024

	return nil
}
