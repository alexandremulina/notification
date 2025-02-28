package middlewares

import (
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Logger middleware logs request and response details
func Logger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestID, _ := c.Get("RequestID")
		log.Debug("Request START",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"time", start,
			"request_id", requestID,
		)

		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		end := time.Now()
		status := c.Writer.Status()
		latency := end.Sub(start)

		if status >= 400 {
			if len(c.Errors) > 0 {
				LogError(log, c, c.Errors[0])
			}
		} else {
			log.Info("Request OK",
				"method", c.Request.Method,
				"path", path,
				"query", query,
				"ip", c.ClientIP(),
				"status", c.Writer.Status(),
				"latency", latency,
				"request_id", requestID,
			)
		}

		log.Debug("Request END",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"time", start,
			"request_id", requestID,
		)
	}
}

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("RequestID", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func LogError(log *slog.Logger, c *gin.Context, err error) {
	log.Error("❌ Request error",
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"query", c.Request.URL.RawQuery,
		"ip", c.ClientIP(),
		"status", c.Writer.Status(),
		"error", err,
		"request_id", c.GetString("RequestID"),
	)

	log.Debug("Stack",
		"stack", string(debug.Stack()),
	)
}
