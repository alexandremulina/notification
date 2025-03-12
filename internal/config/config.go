package config

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	Port        string
	Environment string

	// Email configuration
	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string

	// IAM configuration
	IAMUrl       string
	Realm        string
	ClientID     string
	ClientSecret string

	// DD IAM configuration
	DDIAMURL       string
	DDRealm        string
	DDClientID     string
	DDClientSecret string

	// RP IAM configuration
	RPIAMURL string
	RPRealm  string

	// AWS SQS configuration
	AWSSQSURL       string
	AWSSQSQueueName string
	AWSSQSARN       string
	AWSRegion       string

	// AWS SNS configuration
	AWSSNSTopicARN string

	// MongoDB configuration
	MongoURL string
}

var GlobalConfig *Config

func Load(logger *slog.Logger) *Config {
	if err := godotenv.Load(); err != nil {
		logger.Error("Error loading .env file", "error", err)
	}

	GlobalConfig = &Config{
		DatabaseURL: getEnvOrExit("DATABASE_URL", logger),
		Port:        getEnvOrDefault("PORT", "8080", logger),
		Environment: getEnvOrDefault("APP_ENV", "dev", logger),
		// SMTPHost:       getEnvOrExit("SMTP_HOST", logger),
		// SMTPPort:       getEnvOrExit("SMTP_PORT", logger),
		// SMTPUsername:   getEnvOrExit("SMTP_USERNAME", logger),
		// SMTPPassword:   getEnvOrExit("SMTP_PASSWORD", logger),
		// IAMUrl:         getEnvOrExit("IAM_URL", logger),
		// Realm:          getEnvOrExit("REALM", logger),
		// ClientID:       getEnvOrExit("CLIENT_ID", logger),
		// ClientSecret:   getEnvOrExit("CLIENT_SECRET", logger),
		// DDIAMURL:       getEnvOrExit("DDIAM_URL", logger),
		// DDRealm:        getEnvOrExit("DD_REALM", logger),
		// DDClientID:     getEnvOrExit("DD_CLIENT_ID", logger),
		// DDClientSecret: getEnvOrExit("DD_CLIENT_SECRET", logger),
		// RPIAMURL:       getEnvOrExit("RPIAM_URL", logger),
		// RPRealm:        getEnvOrExit("RP_REALM", logger),

		// AWS SQS configuration
		AWSSQSURL:       getEnvOrDefault("AWS_SQS_URL", "https://sqs.us-east-1.amazonaws.com/471112700878/data-link-queue", logger),
		AWSSQSQueueName: getEnvOrDefault("AWS_SQS_QUEUE_NAME", "data-link-queue", logger),
		AWSSQSARN:       getEnvOrDefault("AWS_SQS_ARN", "arn:aws:sqs:us-east-1:471112700878:data-link-queue", logger),
		AWSRegion:       getEnvOrDefault("AWS_REGION", "us-east-1", logger),

		// AWS SNS configuration
		AWSSNSTopicARN: getEnvOrDefault("AWS_SNS_TOPIC_ARN", "arn:aws:sns:us-east-1:471112700878:data-link-topic", logger),

		// MongoDB configuration
		MongoURL: getEnvOrDefault("MONGO_URL", "", logger),
	}

	return GlobalConfig
}

func getEnvOrExit(key string, logger *slog.Logger) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		logger.Error(fmt.Sprintf("Environment variable %s not found", key))
		os.Exit(1)
	}
	return value
}

func getEnvOrDefault(key, defaultValue string, logger *slog.Logger) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	logger.Info(fmt.Sprintf("Environment variable %s not found, using default: %s", key, defaultValue))
	return defaultValue
}
