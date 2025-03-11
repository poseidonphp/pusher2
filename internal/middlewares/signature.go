package middlewares

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"pusher/env"
	"pusher/internal/util"
)

// Signature middleware
func Signature() gin.HandlerFunc {
	return func(c *gin.Context) {
		appID := c.Param("app_id")
		if appID != env.GetString("APP_ID", "") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid app id"})
			c.Abort()
			return
		}

		ok, err := util.Verify(c.Request)
		if ok {
			c.Next()
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			c.Abort()
			return
		}
	}
}
