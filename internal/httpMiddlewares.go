package internal

import (
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"pusher/internal/apps"
	"pusher/internal/util"
)

func getAppFromContext(c *gin.Context) (*apps.App, bool) {
	app, exists := c.Get("app")
	if !exists {
		return nil, false
	}
	appTyped, ok := app.(*apps.App)
	return appTyped, ok
}

func AppMiddleware(server *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		appID := c.Param("app_id")
		app, err := server.AppManager.FindByID(appID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			c.Abort()
			return
		}
		c.Set("app", app)
		c.Next()
	}
}

// Signature middleware validates the signature on an incoming http request
func SignatureMiddleware(_ *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		app, exists := getAppFromContext(c)
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "App not found in context"})
			c.Abort()
			return
		}

		ok, err := util.Verify(c.Request, app.ID, app.Secret)
		if ok {
			c.Next()
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			c.Abort()
			return
		}
	}
}

func CorsMiddleware(_ *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

func RateLimiterMiddleware(_ *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

var timeFormat = "2006-01-02 15:04:05 -0700"

// Logger is the logrus logger handler
func LoggerMiddleware(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// other handler can change c.Path so:
		path := c.Request.URL.Path
		start := time.Now()

		c.Next()

		stop := time.Since(start)
		latency := int(math.Ceil(float64(stop.Nanoseconds()) / 1000000.0))
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		clientUserAgent := c.Request.UserAgent()
		referer := c.Request.Referer()
		dataLength := c.Writer.Size()

		if dataLength < 0 {
			dataLength = 0
		}

		entry := logger.WithFields(logrus.Fields{
			"statusCode": statusCode,
			"latency":    latency, // time to process
			"clientIP":   clientIP,
			"method":     c.Request.Method,
			"path":       path,
			"referer":    referer,
			"dataLength": dataLength,
			"userAgent":  clientUserAgent,
			"req_at":     start.Format(timeFormat),
		})

		if len(c.Errors) > 0 {
			entry.Error(c.Errors.ByType(gin.ErrorTypePrivate).String())
		} else {
			msg := fmt.Sprintf("%s [%s] \"%s %s\" %d %d \"%s\" \"%s\" (%dms)", clientIP, start.Format(timeFormat), c.Request.Method, path, statusCode, dataLength, referer, clientUserAgent, latency)
			if statusCode > 499 {
				entry.Error(msg)
			} else if statusCode > 399 {
				entry.Warn(msg)
			} else {
				entry.Info(msg)
			}
		}
	}
}
