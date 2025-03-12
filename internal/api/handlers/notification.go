package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
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
	cancelPolling       context.CancelFunc // New field to cancel polling
	pollingCtx          context.Context    // New field to hold the context
	stopMutex           sync.Mutex         // Mutex for stopping operations
	workerPool          chan types.Message
	numWorkers          int
	metrics             struct {
		messagesProcessed int64
		errors            int64
		retries           int64
		lastPollTime      time.Time
		activeWorkers     int
		mu                sync.RWMutex
	}
}

func NewNotificationHandler(notificationService *services.NotificationService, sqsService *services.SQSService) *NotificationHandler {
	return &NotificationHandler{
		notificationService: notificationService,
		sqsService:          sqsService,
		stopMutex:           sync.Mutex{},
		workerPool:          make(chan types.Message, 100),
		numWorkers:          10, // Default worker count - can be made configurable
	}
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

func (h *NotificationHandler) StartPolling() {
	// Use mutex to ensure thread safety
	h.stopMutex.Lock()
	defer h.stopMutex.Unlock()

	if h.isPolling {
		return
	}

	// Create a new context with cancel function
	ctx, cancel := context.WithCancel(context.Background())
	h.pollingCtx = ctx
	h.cancelPolling = cancel

	h.isPolling = true

	go func() {
		baseInterval := 2 * time.Second
		minInterval := 500 * time.Millisecond
		maxInterval := 10 * time.Second
		currentInterval := baseInterval

		for {
			select {
			case <-h.pollingCtx.Done():
				h.isPolling = false
				log.Println("Polling stopped")
				return
			default:
				startTime := time.Now()
				messageCount, err := h.pollAndProcessDirectly()
				if err != nil {
					log.Printf("Error in polling cycle: %v", err)
					h.metrics.mu.Lock()
					h.metrics.errors++
					h.metrics.mu.Unlock()
				}

				if messageCount > 10 {
					currentInterval = minFloat(minInterval, currentInterval/2)
				} else if messageCount == 0 {
					newInterval := time.Duration(float64(currentInterval) * 1.5)
					if newInterval > maxInterval {
						newInterval = maxInterval
					}
					currentInterval = newInterval
				}

				elapsed := time.Since(startTime)
				sleepTime := currentInterval - elapsed
				if sleepTime > 0 {
					select {
					case <-h.pollingCtx.Done():
						return
					case <-time.After(sleepTime):
						// Continue polling
					}
				}
			}
		}
	}()
}

func (h *NotificationHandler) pollAndProcessDirectly() (int, error) {
	// Use the polling context to ensure it respects cancellation
	if h.pollingCtx == nil {
		return 0, fmt.Errorf("polling context is nil")
	}

	// Receive messages from SQS
	result, err := h.sqsService.ReceiveMessageWithCount(h.pollingCtx, int32(10)) // Fixed batch size
	if err != nil {
		// Check if the context was canceled
		if errors.Is(err, context.Canceled) {
			return 0, nil // Gracefully handle cancellation
		}
		return 0, fmt.Errorf("failed to receive message: %w", err)
	}

	h.metrics.mu.Lock()
	h.metrics.lastPollTime = time.Now()
	h.metrics.mu.Unlock()

	messageCount := len(result.Messages)

	// Only process messages if we're still polling
	if !h.isPolling || h.pollingCtx.Err() != nil {
		return messageCount, nil
	}

	// Process messages directly in this goroutine
	for _, message := range result.Messages {
		// Recheck polling status before each message
		if !h.isPolling || h.pollingCtx.Err() != nil {
			break
		}

		// Create a derived context with timeout for this message processing
		msgCtx, cancel := context.WithTimeout(h.pollingCtx, 30*time.Second)

		// Process message with retries
		if err := h.processMessageWithRetries(msgCtx, message); err != nil {
			log.Printf("Failed to process message after retries: %v", err)
			h.metrics.mu.Lock()
			h.metrics.errors++
			h.metrics.mu.Unlock()
			cancel()
			continue
		}

		// Delete message with retries
		if err := h.deleteMessageWithRetries(msgCtx, *message.ReceiptHandle); err != nil {
			log.Printf("Failed to delete message after retries: %v", err)
			h.metrics.mu.Lock()
			h.metrics.errors++
			h.metrics.mu.Unlock()
			cancel()
			continue
		}

		// Update success metric
		h.metrics.mu.Lock()
		h.metrics.messagesProcessed++
		h.metrics.mu.Unlock()

		// Release resources
		cancel()
	}

	return messageCount, nil
}

func minFloat(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func (h *NotificationHandler) GetMetrics(c *gin.Context) {
	h.metrics.mu.RLock()
	lastPollTime := h.metrics.lastPollTime
	messagesProcessed := h.metrics.messagesProcessed
	errors := h.metrics.errors
	retries := h.metrics.retries
	activeWorkers := h.metrics.activeWorkers
	h.metrics.mu.RUnlock()

	// Calculate actual polling state
	actuallyActive := time.Since(lastPollTime) < 5*time.Second

	metrics := gin.H{
		"isPolling":          h.isPolling,
		"actuallyActive":     actuallyActive,
		"messagesProcessed":  messagesProcessed,
		"errors":             errors,
		"retries":            retries,
		"lastPollTime":       lastPollTime,
		"timeSinceLastPoll":  time.Since(lastPollTime).String(),
		"activeWorkers":      activeWorkers,
		"totalWorkers":       h.numWorkers,
		"workerPoolCapacity": cap(h.workerPool),
		"workerPoolUsage":    len(h.workerPool),
	}

	c.JSON(http.StatusOK, metrics)
}

func (h *NotificationHandler) processMessage(ctx context.Context, message types.Message) error {
	var parsedBody interface{}
	if err := json.Unmarshal([]byte(*message.Body), &parsedBody); err != nil {
		return nil
	}

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

	// Get polling metrics to determine actual state
	h.metrics.mu.RLock()
	lastPollTime := h.metrics.lastPollTime
	h.metrics.mu.RUnlock()

	// Consider polling active if last poll was within the last 5 seconds
	actuallyActive := time.Since(lastPollTime) < 5*time.Second

	if req.Enable {
		// Only start if not already polling
		if !h.isPolling || !actuallyActive {
			h.StartPolling()
			c.JSON(http.StatusOK, gin.H{"message": "Polling started"})
		} else {
			c.JSON(http.StatusOK, gin.H{"message": "Polling already active"})
		}
	} else {
		// Check if polling is actually running
		if h.isPolling || actuallyActive {
			// Use a mutex to safely handle this operation
			go func() {
				// Do the stop in a goroutine to avoid blocking the API response
				h.StopPolling()
			}()
			c.JSON(http.StatusOK, gin.H{"message": "Polling stopping"})
		} else {
			c.JSON(http.StatusOK, gin.H{"message": "Polling already inactive"})
		}
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

func (h *NotificationHandler) StopPolling() {
	// Use a mutex to ensure only one goroutine can stop the polling
	h.stopMutex.Lock()
	defer h.stopMutex.Unlock()

	// Log state before stopping
	log.Printf("StopPolling called, current isPolling: %v", h.isPolling)

	// Force stop even if flag says not polling
	h.isPolling = false

	// Cancel the context if it exists
	if h.cancelPolling != nil {
		h.cancelPolling()
		log.Println("Cancel signal sent to polling context")
	}

	log.Println("Polling stopped")
}

func (h *NotificationHandler) processMessageWithRetries(ctx context.Context, message types.Message) error {
	maxRetries := 3
	baseDelay := 200 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		err := h.processMessage(ctx, message)
		if err == nil {
			return nil // Success!
		}

		log.Printf("Error processing message (attempt %d/%d): %v",
			attempt+1, maxRetries, err)

		// If this was the last attempt, return the error
		if attempt == maxRetries-1 {
			return err
		}

		// Update retry metrics
		h.metrics.mu.Lock()
		h.metrics.retries++
		h.metrics.mu.Unlock()

		// Exponential backoff with jitter
		delay := baseDelay * time.Duration(1<<attempt)
		jitter := time.Duration(rand.Int63n(int64(delay) / 2))
		time.Sleep(delay + jitter)
	}

	return fmt.Errorf("exhausted retries for message %s", *message.MessageId)
}

func (h *NotificationHandler) deleteMessageWithRetries(ctx context.Context, receiptHandle string) error {
	maxRetries := 3
	baseDelay := 200 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		err := h.sqsService.DeleteMessage(ctx, receiptHandle)
		if err == nil {
			return nil // Success!
		}

		log.Printf("Error deleting message (attempt %d/%d): %v",
			attempt+1, maxRetries, err)

		// If this was the last attempt, return the error
		if attempt == maxRetries-1 {
			return err
		}

		// Update retry metrics
		h.metrics.mu.Lock()
		h.metrics.retries++
		h.metrics.mu.Unlock()

		// Exponential backoff with jitter
		delay := baseDelay * time.Duration(1<<attempt)
		jitter := time.Duration(rand.Int63n(int64(delay) / 2))
		time.Sleep(delay + jitter)
	}

	return fmt.Errorf("exhausted retries for deleting message with receipt handle %s", receiptHandle)
}

// Helper functions for min and max operations
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ConfigureWorkerPool allows dynamic configuration of the worker pool
func (h *NotificationHandler) ConfigureWorkerPool(c *gin.Context) {
	var req struct {
		WorkerCount int `json:"workerCount"`
		QueueSize   int `json:"queueSize"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate input
	if req.WorkerCount < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Worker count must be at least 1"})
		return
	}

	if req.QueueSize < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Queue size must be at least 1"})
		return
	}

	// If polling is active, stop it first
	wasPolling := h.isPolling
	if wasPolling {
		h.StopPolling()
	}

	// Create new worker pool with updated size
	h.numWorkers = req.WorkerCount
	h.workerPool = make(chan types.Message, req.QueueSize)

	// Restart polling if it was active
	if wasPolling {
		h.StartPolling()
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Worker pool reconfigured",
		"workerCount": h.numWorkers,
		"queueSize":   cap(h.workerPool),
	})
}
