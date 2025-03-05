package services

import (
	"context"
	"fmt"
	"go-data-distributor-notification/internal/config"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

type SNSService struct {
	client   *sns.Client
	topicARN string
	logger   *slog.Logger
}

func NewSNSService(logger *slog.Logger) (*SNSService, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
		awsconfig.WithRegion(config.GlobalConfig.AWSRegion),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := sns.NewFromConfig(cfg)

	if config.GlobalConfig.AWSSNSTopicARN == "" {
		return nil, fmt.Errorf("AWS SNS Topic ARN is required")
	}

	logger.Info("SNS service initialized",
		"topicARN", config.GlobalConfig.AWSSNSTopicARN,
		"region", config.GlobalConfig.AWSRegion)

	return &SNSService{
		client:   client,
		topicARN: config.GlobalConfig.AWSSNSTopicARN,
		logger:   logger,
	}, nil
}

func (s *SNSService) PublishMessage(ctx context.Context, message string) (string, error) {
	s.logger.Info("Publishing message to SNS topic", "topicARN", s.topicARN)

	input := &sns.PublishInput{
		TopicArn: aws.String(s.topicARN),
		Message:  aws.String(message),
	}

	result, err := s.client.Publish(ctx, input)
	if err != nil {
		s.logger.Error("Failed to publish message to SNS", "error", err)
		return "", fmt.Errorf("failed to publish message to SNS: %w", err)
	}

	messageID := ""
	if result.MessageId != nil {
		messageID = *result.MessageId
	}

	s.logger.Info("Message published to SNS", "messageId", messageID)
	return messageID, nil
}
