package services

import (
	"context"
	"encoding/json"
	"fmt"

	"go-data-distributor-notification/internal/config"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type SQSService struct {
	client       *sqs.Client
	queueURL     string
	logger       *slog.Logger
	snsService   *SNSService
	mongoService *MongoDBService
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

	// Initialize MongoDB service
	mongoService, err := NewMongoDBService(logger)
	if err != nil {
		logger.Warn("Failed to initialize MongoDB service, tenant lookups will be disabled", "error", err)
	}

	logger.Info("SQS service initialized",
		"queueURL", config.GlobalConfig.AWSSQSURL,
		"region", config.GlobalConfig.AWSRegion)

	return &SQSService{
		client:       client,
		queueURL:     config.GlobalConfig.AWSSQSURL,
		logger:       logger,
		snsService:   snsService,
		mongoService: mongoService,
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

	// Default webhook URL as fallback
	var slackWebhook string
	var emailAddress string
	var webhookAddress string
	var smsAddress string

	var messageData map[string]interface{}
	if err := json.Unmarshal([]byte(*message.Body), &messageData); err != nil {
		s.logger.Error("Failed to parse message body", "error", err)
	} else {
		tenantID := getStringValue(messageData, "tenantId", "")

		// Store notification in MongoDB if tenant ID is present and MongoDB service is available
		if tenantID != "" && s.mongoService != nil {
			// Store the notification in MongoDB
			if err := s.mongoService.StoreNotification(ctx, tenantID, *message.Body); err != nil {
				s.logger.Error("Failed to store notification in MongoDB", "error", err, "tenantId", tenantID)
			}

			s.logger.Info("Fetching tenant channels", "tenantId", tenantID)
			tenantChannels, err := s.mongoService.FindTenantChannels(ctx, tenantID)
			if err != nil {
				s.logger.Error("Failed to fetch tenant channels", "error", err, "tenantId", tenantID)
			} else if tenantChannels != nil {
				for _, channel := range tenantChannels.Channels {
					if channel.Enable {
						switch channel.Type {
						case "slack":
							slackWebhook = channel.Value
							s.logger.Info("Using Slack webhook URL from MongoDB", "slackWebhookUrl", slackWebhook, "tenantId", tenantID)
						case "email":
							emailAddress = channel.Value
							s.logger.Info("Using email from MongoDB", "email", emailAddress, "tenantId", tenantID)
						case "webhook":
							webhookAddress = channel.Value
							s.logger.Info("Using webhook from MongoDB", "webhook", webhookAddress, "tenantId", tenantID)
						case "sms":
							smsAddress = channel.Value
							s.logger.Info("Using SMS from MongoDB", "sms", smsAddress, "tenantId", tenantID)
						}
					}
				}
			}
		} else {
			if url, ok := messageData["webhookUrl"].(string); ok && url != "" {
				slackWebhook = url
				s.logger.Info("Using webhook URL from message", "webhookUrl", slackWebhook)
			}
		}
	}

	messageWithChannels := map[string]interface{}{
		"originalMessage": *message.Body,
		"slackWebhookUrl": slackWebhook,
		"email":           emailAddress,
		"webhook":         webhookAddress,
		"sms":             smsAddress,
	}

	// Add email address if found
	if emailAddress != "" {
		messageWithChannels["email"] = emailAddress
	}

	// Add webhook address if found
	if webhookAddress != "" {
		messageWithChannels["webhook"] = webhookAddress
	}

	// Add SMS address if found
	if smsAddress != "" {
		messageWithChannels["sms"] = smsAddress
	}

	messageBytes, err := json.Marshal(messageWithChannels)
	if err != nil {
		s.logger.Error("Failed to marshal message with channels", "error", err)
		return
	}

	messageId, err := s.snsService.PublishMessage(ctx, string(messageBytes))
	if err != nil {
		s.logger.Error("Failed to publish message to SNS", "error", err)
		return
	}

	s.logger.Info("Successfully forwarded message to SNS",
		"snsMessageId", messageId,
		"hasSlack", slackWebhook != "",
		"hasEmail", emailAddress != "",
		"hasWebhook", webhookAddress != "",
		"hasSMS", smsAddress != "")
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
func (s *SQSService) ReceiveMessageWithCount(ctx context.Context, maxMessages int32, waitTimeSeconds int32) (*sqs.ReceiveMessageOutput, error) {
	// s.logger.Info("Polling SQS queue for messages", "queueURL", s.queueURL, "maxMessages", maxMessages)

	// Ensure maxMessages is within allowed range (1-10)
	if maxMessages < 1 {
		maxMessages = 1
	} else if maxMessages > 10 {
		maxMessages = 10
	}

	input := &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(s.queueURL),
		MaxNumberOfMessages: maxMessages,
		WaitTimeSeconds:     waitTimeSeconds,
		AttributeNames: []types.QueueAttributeName{
			"All",
		},
		MessageAttributeNames: []string{
			"All",
		},
	}

	result, err := s.client.ReceiveMessage(ctx, input)
	if err != nil {
		s.logger.Error("Failed to receive message from SQS", "error", err)
		return nil, fmt.Errorf("failed to receive message from SQS: %w", err)
	}

	// s.logger.Info("SQS poll completed", "messagesReceived", len(result.Messages))

	// Process messages and send to SNS if available
	if len(result.Messages) > 0 && s.snsService != nil {
		// Process all messages in the batch
		for i := range result.Messages {
			go s.processAndSendToSNS(context.Background(), &result.Messages[i])
		}
	}

	return result, nil
}
