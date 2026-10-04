package internal

import (
	"context"
	"fmt"
	"math"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"pusher/internal/apps"
	"pusher/internal/util"
	"pusher/log"
)

type appContextKey struct{}

func withApp(r *http.Request, app *apps.App) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), appContextKey{}, app))
}

func getAppFromContext(r *http.Request) (*apps.App, bool) {
	app, ok := r.Context().Value(appContextKey{}).(*apps.App)
	return app, ok && app != nil
}

func AppMiddleware(server *Server) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			app, err := server.AppManager.FindByID(r.PathValue("app_id"))
			if err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			next.ServeHTTP(w, withApp(r, app))
		})
	}
}

// SignatureMiddleware validates the signature on an incoming http request
func SignatureMiddleware(_ *Server) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			app, exists := getAppFromContext(r)
			if !exists {
				writeError(w, http.StatusInternalServerError, "App not found in context")
				return
			}

			if ok, err := util.Verify(r, app.ID, app.Secret); !ok {
				writeError(w, http.StatusUnauthorized, err.Error())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func CorsMiddleware(_ *Server) middleware {
	return func(next http.Handler) http.Handler {
		return next
	}
}

func RateLimiterMiddleware(_ *Server) middleware {
	return func(next http.Handler) http.Handler {
		return next
	}
}

// RecoveryMiddleware turns a panic in a handler into a 500 response instead of
// dropping the connection.
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec)
			}
			log.Logger().Errorf("panic recovered: %v\n%s", rec, debug.Stack())
			w.WriteHeader(http.StatusInternalServerError)
		}()
		next.ServeHTTP(w, r)
	})
}

var timeFormat = "2006-01-02 15:04:05 -0700"

// LoggerMiddleware is the logrus access log handler
func LoggerMiddleware(logger *logrus.Logger) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			start := time.Now()
			rec := newResponseRecorder(w)

			next.ServeHTTP(rec, r)

			stop := time.Since(start)
			latency := int(math.Ceil(float64(stop.Nanoseconds()) / 1000000.0))
			statusCode := rec.Status()
			clientIP := clientIP(r)
			clientUserAgent := r.UserAgent()
			referer := r.Referer()
			dataLength := rec.size

			entry := logger.WithFields(logrus.Fields{
				"statusCode": statusCode,
				"latency":    latency, // time to process
				"clientIP":   clientIP,
				"method":     r.Method,
				"path":       path,
				"referer":    referer,
				"dataLength": dataLength,
				"userAgent":  clientUserAgent,
				"req_at":     start.Format(timeFormat),
			})

			msg := fmt.Sprintf("%s [%s] \"%s %s\" %d %d \"%s\" \"%s\" (%dms)", clientIP, start.Format(timeFormat), r.Method, path, statusCode, dataLength, referer, clientUserAgent, latency)
			if statusCode > 499 {
				entry.Error(msg)
			} else if statusCode > 399 {
				entry.Warn(msg)
			} else {
				entry.Info(msg)
			}
		})
	}
}

// clientIP returns the originating client address for logging, preferring
// proxy headers over the socket peer address.
func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		first, _, _ := strings.Cut(forwarded, ",")
		if ip := strings.TrimSpace(first); ip != "" {
			return ip
		}
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
