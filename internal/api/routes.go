package api

import (
	"go-data-distributor-notification/internal/api/handlers"
	"go-data-distributor-notification/internal/services"
)

func setupRoutes(s *Server) {
	emailService := services.NewEmailService(s.logger)

	// Initialize SQS service
	sqsService, err := services.NewSQSService(s.logger)
	if err != nil {
		s.logger.Error("Failed to initialize SQS service", "error", err)
	}

	notificationService := services.NewNotificationService(s.store, emailService)
	notificationHandler := handlers.NewNotificationHandler(notificationService, sqsService)

	v1 := s.router.Group("/api/v1")
	{
		notifications := v1.Group("/notifications")
		{
			// POST /api/v1/notifications - Create a new notification
			notifications.POST("", notificationHandler.Create)

			// GET /api/v1/notifications/:id - Get a notification by ID
			notifications.GET("/:id", notificationHandler.Get)

			// GET /api/v1/notifications - Poll SQS queue for messages
			notifications.GET("", notificationHandler.PollSQS)

			// POST /api/v1/notifications/polling - Toggle polling
			notifications.POST("/polling", notificationHandler.TogglePolling)

			// GET /api/v1/notifications/polling/metrics - Get polling metrics
			notifications.GET("/polling/metrics", notificationHandler.GetMetrics)

			// GET /api/v1/notifications/metrics - Get polling metrics
			notifications.GET("/metrics", notificationHandler.GetMetrics)

			// POST /api/v1/notifications/worker-config - Configure worker pool
			notifications.POST("/worker-config", notificationHandler.ConfigureWorkerPool)
		}
	}
}
