package services

import (
	"context"
	"fmt"

	"go-data-distributor-notification/internal/config"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type SQSService struct {
	client     *sqs.Client
	queueURL   string
	logger     *slog.Logger
	snsService *SNSService
}

func NewSQSService(logger *slog.Logger) (*SQSService, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
		awsconfig.WithRegion(config.GlobalConfig.AWSRegion),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := sqs.NewFromConfig(cfg)

	if config.GlobalConfig.AWSSQSURL == "" {
		return nil, fmt.Errorf("AWS SQS URL is required")
	}

	// Initialize SNS service
	snsService, err := NewSNSService(logger)
	if err != nil {
		logger.Warn("Failed to initialize SNS service, SNS notifications will be disabled", "error", err)
	}

	logger.Info("SQS service initialized",
		"queueURL", config.GlobalConfig.AWSSQSURL,
		"region", config.GlobalConfig.AWSRegion)

	return &SQSService{
		client:     client,
		queueURL:   config.GlobalConfig.AWSSQSURL,
		logger:     logger,
		snsService: snsService,
	}, nil
}

func (s *SQSService) ReceiveMessage(ctx context.Context) (*sqs.ReceiveMessageOutput, error) {
	s.logger.Info("Polling SQS queue for messages", "queueURL", s.queueURL)

	input := &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(s.queueURL),
		MaxNumberOfMessages: 10,
		WaitTimeSeconds:     5,
	}

	result, err := s.client.ReceiveMessage(ctx, input)
	if err != nil {
		s.logger.Error("Failed to receive message from SQS", "error", err)
		return nil, fmt.Errorf("failed to receive message from SQS: %w", err)
	}

	s.logger.Info("SQS poll completed", "messagesReceived", len(result.Messages))

	// Process messages and send to SNS if available
	if len(result.Messages) > 0 && s.snsService != nil {
		// Process all messages in the batch
		for i := range result.Messages {
			go s.processAndSendToSNS(context.Background(), &result.Messages[i])
		}
	}

	return result, nil
}

func (s *SQSService) processAndSendToSNS(ctx context.Context, message *types.Message) {
	s.logger.Info("Forwarding SQS message to SNS", "messageId", *message.MessageId)

	// Forward the original message body to SNS
	messageId, err := s.snsService.PublishMessage(ctx, *message.Body)
	if err != nil {
		s.logger.Error("Failed to publish message to SNS", "error", err)
		return
	}

	s.logger.Info("Successfully forwarded message to SNS", "snsMessageId", messageId)
}

// Helper functions to safely extract values from the message data
func getStringValue(data map[string]interface{}, key, defaultValue string) string {
	if val, ok := data[key]; ok {
		if strVal, ok := val.(string); ok {
			return strVal
		}
	}
	return defaultValue
}

func getStringArray(data map[string]interface{}, key string) []string {
	if val, ok := data[key]; ok {
		if arrVal, ok := val.([]interface{}); ok {
			result := make([]string, 0, len(arrVal))
			for _, v := range arrVal {
				if strVal, ok := v.(string); ok {
					result = append(result, strVal)
				}
			}
			return result
		}
	}
	return []string{}
}

func (s *SQSService) DeleteMessage(ctx context.Context, receiptHandle string) error {
	s.logger.Info("Deleting message from SQS queue", "receiptHandle", receiptHandle)

	input := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(s.queueURL),
		ReceiptHandle: aws.String(receiptHandle),
	}

	_, err := s.client.DeleteMessage(ctx, input)
	if err != nil {
		s.logger.Error("Failed to delete message from SQS", "error", err)
		return fmt.Errorf("failed to delete message from SQS: %w", err)
	}

	s.logger.Info("Message deleted from SQS queue")
	return nil
}

// ReceiveMessageWithCount receives messages with a specific batch size
func (s *SQSService) ReceiveMessageWithCount(ctx context.Context, maxMessages int32) (*sqs.ReceiveMessageOutput, error) {
	s.logger.Info("Polling SQS queue for messages", "queueURL", s.queueURL, "maxMessages", maxMessages)

	// Ensure maxMessages is within allowed range (1-10)
	if maxMessages < 1 {
		maxMessages = 1
	} else if maxMessages > 10 {
		maxMessages = 10
	}

	input := &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(s.queueURL),
		MaxNumberOfMessages: maxMessages,
		WaitTimeSeconds:     5,
	}

	result, err := s.client.ReceiveMessage(ctx, input)
	if err != nil {
		s.logger.Error("Failed to receive message from SQS", "error", err)
		return nil, fmt.Errorf("failed to receive message from SQS: %w", err)
	}

	s.logger.Info("SQS poll completed", "messagesReceived", len(result.Messages))

	// Process messages and send to SNS if available
	if len(result.Messages) > 0 && s.snsService != nil {
		// Process all messages in the batch
		for i := range result.Messages {
			go s.processAndSendToSNS(context.Background(), &result.Messages[i])
		}
	}

	return result, nil
}
