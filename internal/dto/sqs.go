package dto

// SQSMessage represents a message from SQS
type SQSMessage struct {
	MessageID string      `json:"messageId"`
	Body      interface{} `json:"body"`
}

// SQSResponse represents the response for SQS polling
type SQSResponse struct {
	Message string      `json:"message,omitempty"`
	Data    *SQSMessage `json:"data,omitempty"`
}
