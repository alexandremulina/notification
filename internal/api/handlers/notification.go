package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"go-data-distributor-notification/internal/dto"
	"go-data-distributor-notification/internal/services"

	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/gin-gonic/gin"
)

type NotificationHandler struct {
	notificationService *services.NotificationService
	sqsService          *services.SQSService
	isPolling           bool
	stopPolling         chan bool
	metrics             struct {
		messagesProcessed int64
		errors            int64
		lastPollTime      time.Time
	}
}

func NewNotificationHandler(notificationService *services.NotificationService, sqsService *services.SQSService) *NotificationHandler {
	handler := &NotificationHandler{
		notificationService: notificationService,
		sqsService:          sqsService,
		stopPolling:         make(chan bool),
	}
	return handler
}

func (h *NotificationHandler) Create(c *gin.Context) {
	var req dto.CreateNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	notification, err := h.notificationService.Create(c, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, notification)
}

func (h *NotificationHandler) Get(c *gin.Context) {
	id := c.Param("id")
	notification, err := h.notificationService.Get(c, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, notification)
}

// StartPolling starts the automatic polling process
func (h *NotificationHandler) StartPolling() {
	if h.isPolling {
		return
	}
	h.isPolling = true

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-h.stopPolling:
				h.isPolling = false
				log.Println("Polling stopped gracefully")
				return
			case <-ticker.C:
				if err := h.pollAndProcess(); err != nil {
					log.Printf("Error in polling cycle: %v", err)
					h.metrics.errors++
				}
			}
		}
	}()
}

// pollAndProcess handles one polling cycle
func (h *NotificationHandler) pollAndProcess() error {
	ctx := context.Background()
	result, err := h.sqsService.ReceiveMessage(ctx)
	if err != nil {
		return fmt.Errorf("failed to receive message: %w", err)
	}

	h.metrics.lastPollTime = time.Now()

	for _, message := range result.Messages {
		if err := h.processMessage(ctx, message); err != nil {
			log.Printf("Error processing message: %v", err)
			h.metrics.errors++
			continue
		}

		if err := h.sqsService.DeleteMessage(ctx, *message.ReceiptHandle); err != nil {
			log.Printf("Error deleting message: %v", err)
			h.metrics.errors++
			continue
		}

		h.metrics.messagesProcessed++
	}

	return nil
}

// GetMetrics returns current polling metrics
func (h *NotificationHandler) GetMetrics(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"isPolling": h.isPolling,
		"metrics":   h.metrics,
	})
}

// processMessage handles a single SQS message
func (h *NotificationHandler) processMessage(ctx context.Context, message types.Message) error {
	var parsedBody interface{}
	if err := json.Unmarshal([]byte(*message.Body), &parsedBody); err != nil {
		// Handle raw message if not JSON
		// Add your processing logic here
		return nil
	}

	// Add your JSON message processing logic here
	return nil
}

func (h *NotificationHandler) TogglePolling(c *gin.Context) {
	var req struct {
		Enable bool `json:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Enable {
		h.StartPolling()
		c.JSON(http.StatusOK, gin.H{"message": "Polling started"})
	} else {
		h.StopPolling()
		c.JSON(http.StatusOK, gin.H{"message": "Polling stopped"})
	}
}

func (h *NotificationHandler) PollSQS(c *gin.Context) {
	ctx := context.Background()
	result, err := h.sqsService.ReceiveMessage(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if len(result.Messages) == 0 {
		response := dto.SQSResponse{
			Message: "No messages available in the queue",
		}
		c.JSON(http.StatusOK, response)
		return
	}

	message := result.Messages[0]
	if err := h.processMessage(ctx, message); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.sqsService.DeleteMessage(ctx, *message.ReceiptHandle); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message processed successfully"})
}

// StopPolling stops the automatic polling process
func (h *NotificationHandler) StopPolling() {
	if h.isPolling {
		h.stopPolling <- true
		// Wait for confirmation that polling has stopped
		for h.isPolling {
			time.Sleep(100 * time.Millisecond)
		}
	}
}
