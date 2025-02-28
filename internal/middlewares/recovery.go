package middlewares

import (
	"fmt"
	"log/slog"

	apiErrors "go-data-distributor-notification/internal/pkg/errors"

	"github.com/gin-gonic/gin"
)

func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				LogError(logger, c, fmt.Errorf("%v", err))

				apiErrors.HandleError(c, apiErrors.InternalServerError("An unexpected error occurred"))
			}
		}()
		c.Next()
	}
}
