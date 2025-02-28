package api

import (
	"go-data-distributor-notification/internal/api/handlers"
	"go-data-distributor-notification/internal/services"
)

func setupRoutes(s *Server) {
	emailService := services.NewEmailService(s.logger)
	notificationService := services.NewNotificationService(s.store, emailService)
	notificationHandler := handlers.NewNotificationHandler(notificationService)

	v1 := s.router.Group("/api/v1")
	{
		notifications := v1.Group("/notifications")
		{
			notifications.POST("", notificationHandler.Create)
			notifications.GET("/:id", notificationHandler.Get)
		}
	}
}
