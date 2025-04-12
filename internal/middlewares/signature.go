package middlewares

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"pusher/internal/config"
	"pusher/internal/util"
)

// Signature middleware
func Signature(serverConfig *config.ServerConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		appID := c.Param("app_id")
		// if appID != env.GetString("APP_ID", "") {
		appFromConfig, err := serverConfig.LoadAppByID(appID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		ok, err := util.Verify(c.Request, appFromConfig.AppID, appFromConfig.AppSecret)
		if ok {
			c.Next()
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			c.Abort()
			return
		}
	}
}
