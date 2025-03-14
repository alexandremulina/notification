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
	handler := &NotificationHandler{
		notificationService: notificationService,
		sqsService:          sqsService,
		stopMutex:           sync.Mutex{},
		workerPool:          make(chan types.Message, 100),
		numWorkers:          10,
	}

	// Start polling automatically
	handler.StartPolling()

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

// StartPolling begins continuous long polling of the SQS queue.
// This implementation uses AWS SQS's long polling capability, where:
// 1. Each ReceiveMessage call waits up to 20 seconds for messages to arrive
// 2. If messages arrive during that time, SQS returns them immediately
// 3. If no messages arrive during that time, the call returns empty after 20 seconds
// 4. A new call is made immediately after each response, with no artificial delay
// Long polling reduces the number of empty responses and is more efficient
// than short polling, as it maintains fewer connections and reduces API calls.
func (h *NotificationHandler) StartPolling() {
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
		log.Println("Starting SQS long polling")

		for {
			select {
			case <-h.pollingCtx.Done():
				h.isPolling = false
				log.Println("Polling stopped")
				return
			default:
				// Call pollAndProcessDirectly immediately and continuously
				// Each call will wait for up to 20 seconds at the SQS level
				messageCount, err := h.pollAndProcessDirectly()
				if err != nil {
					log.Printf("Error in polling cycle: %v", err)
					h.metrics.mu.Lock()
					h.metrics.errors++
					h.metrics.mu.Unlock()
				}

				// Log the message count for monitoring purposes
				if messageCount > 0 {
					log.Printf("Processed %d messages from SQS", messageCount)
				}

				// No delay between polls - true long polling
				// The next poll starts immediately
			}
		}
	}()
}

// pollAndProcessDirectly implements long polling by making a single SQS request that
// waits for up to 20 seconds, then processes any received messages.
// The SQS WaitTimeSeconds parameter (set to 20) enables true long polling,
// which reduces empty responses and is more efficient than short polling.
// When messages arrive, SQS returns immediately rather than waiting for the full duration.
func (h *NotificationHandler) pollAndProcessDirectly() (int, error) {
	if h.pollingCtx == nil {
		return 0, fmt.Errorf("polling context is nil")
	}

	// Receive messages from SQS with 20-second long polling
	result, err := h.sqsService.ReceiveMessageWithCount(h.pollingCtx, int32(10), int32(20)) // 20-second wait time for long polling
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to receive message: %w", err)
	}

	h.metrics.mu.Lock()
	h.metrics.lastPollTime = time.Now()
	h.metrics.mu.Unlock()

	messageCount := len(result.Messages)

	if !h.isPolling || h.pollingCtx.Err() != nil {
		return messageCount, nil
	}

	for _, message := range result.Messages {

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

func (h *NotificationHandler) GetMetrics(c *gin.Context) {
	h.metrics.mu.RLock()
	lastPollTime := h.metrics.lastPollTime
	messagesProcessed := h.metrics.messagesProcessed
	errors := h.metrics.errors
	retries := h.metrics.retries
	activeWorkers := h.metrics.activeWorkers
	h.metrics.mu.RUnlock()

	// With long polling, consider polling active if last poll was within the last 25 seconds
	// (20 seconds max wait time + 5 seconds buffer for processing)
	actuallyActive := time.Since(lastPollTime) < 25*time.Second

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
		"longPollingEnabled": true,
		"maxWaitTime":        "20 seconds",
	}

	c.JSON(http.StatusOK, metrics)
}

func (h *NotificationHandler) processMessage(_ context.Context, message types.Message) error {
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

	// Consider polling active if last poll was within the last 25 seconds
	// (accounts for 20-second long polling wait time)
	actuallyActive := time.Since(lastPollTime) < 25*time.Second

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
	ctx := c.Request.Context()
	// Use the same long polling approach with 20-second wait time
	result, err := h.sqsService.ReceiveMessageWithCount(ctx, int32(10), int32(20))
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

	h.stopMutex.Lock()
	defer h.stopMutex.Unlock()

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
			return nil
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
			return nil
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

// // ConfigureWorkerPool allows dynamic configuration of the worker pool
// func (h *NotificationHandler) ConfigureWorkerPool(c *gin.Context) {
// 	var req struct {
// 		WorkerCount int `json:"workerCount"`
// 		QueueSize   int `json:"queueSize"`
// 	}

// 	if err := c.ShouldBindJSON(&req); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
// 		return
// 	}

// 	// Validate input
// 	if req.WorkerCount < 1 {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Worker count must be at least 1"})
// 		return
// 	}

// 	if req.QueueSize < 1 {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Queue size must be at least 1"})
// 		return
// 	}

// 	// If polling is active, stop it first
// 	wasPolling := h.isPolling
// 	if wasPolling {
// 		h.StopPolling()
// 	}

// 	// Create new worker pool with updated size
// 	h.numWorkers = req.WorkerCount
// 	h.workerPool = make(chan types.Message, req.QueueSize)

// 	// Restart polling if it was active
// 	if wasPolling {
// 		h.StartPolling()
// 	}

// 	c.JSON(http.StatusOK, gin.H{
// 		"message":     "Worker pool reconfigured",
// 		"workerCount": h.numWorkers,
// 		"queueSize":   cap(h.workerPool),
// 	})
// }

func (h *NotificationHandler) Initialize() {
	// Initialize any resources needed

	// Start long polling automatically
	h.StartPolling()

	log.Println("Notification handler initialized and long polling started")
}
