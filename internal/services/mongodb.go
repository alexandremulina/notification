package services

import (
	"context"
	"fmt"
	"go-data-distributor-notification/internal/config"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TenantChannel represents a notification channel for a tenant
type TenantChannel struct {
	Type   string `bson:"type" json:"type"`
	Value  string `bson:"value" json:"value"`
	Enable bool   `bson:"enable" json:"enable"`
	Secret string `bson:"secret" json:"secret"`
}

// TenantChannels represents a tenant channels configuration in MongoDB
type TenantChannels struct {
	TenantID  string          `bson:"tenantId" json:"tenantId"`
	Channels  []TenantChannel `bson:"channels" json:"channels"`
	CreatedAt time.Time       `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time       `bson:"updatedAt" json:"updatedAt"`
}

// Notification represents a notification document in MongoDB
type Notification struct {
	TenantID  string    `bson:"tenantId" json:"tenantId"`
	Message   string    `bson:"message" json:"message"`
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}

// MongoDBService handles MongoDB operations
type MongoDBService struct {
	client   *mongo.Client
	database *mongo.Database
	logger   *slog.Logger
}

// NewMongoDBService creates a new MongoDB service
func NewMongoDBService(logger *slog.Logger) (*MongoDBService, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	if config.GlobalConfig.MongoURL == "" {
		return nil, fmt.Errorf("MongoDB URL is required")
	}

	// Set up the MongoDB client
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(config.GlobalConfig.MongoURL)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		logger.Error("Failed to connect to MongoDB", "error", err)
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Ping the database to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		logger.Error("Failed to ping MongoDB", "error", err)
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	// Select the database (assuming default name, adjust as needed)
	database := client.Database("data-distributor")

	logger.Info("MongoDB service initialized", "url", config.GlobalConfig.MongoURL)

	return &MongoDBService{
		client:   client,
		database: database,
		logger:   logger,
	}, nil
}

// Close closes the MongoDB connection
func (s *MongoDBService) Close(ctx context.Context) error {
	return s.client.Disconnect(ctx)
}

// FindTenantChannels retrieves tenant channels by tenant ID
func (s *MongoDBService) FindTenantChannels(ctx context.Context, tenantID string) (*TenantChannels, error) {
	s.logger.Info("Fetching tenant channels", "tenantId", tenantID)

	collection := s.database.Collection("tenant-channels")
	filter := bson.M{"tenantId": tenantID}

	var tenantChannels TenantChannels
	err := collection.FindOne(ctx, filter).Decode(&tenantChannels)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			s.logger.Warn("Tenant channels not found", "tenantId", tenantID)
			return nil, nil
		}
		s.logger.Error("Failed to fetch tenant channels", "error", err, "tenantId", tenantID)
		return nil, fmt.Errorf("failed to fetch tenant channels: %w", err)
	}

	return &tenantChannels, nil
}

// StoreNotification saves a notification to MongoDB
func (s *MongoDBService) StoreNotification(ctx context.Context, tenantID string, message string) error {
	s.logger.Info("Storing notification", "tenantId", tenantID)

	collection := s.database.Collection("notifications")

	now := time.Now()
	notification := Notification{
		TenantID:  tenantID,
		Message:   message,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err := collection.InsertOne(ctx, notification)
	if err != nil {
		s.logger.Error("Failed to store notification", "error", err, "tenantId", tenantID)
		return fmt.Errorf("failed to store notification: %w", err)
	}

	s.logger.Info("Notification stored successfully", "tenantId", tenantID)
	return nil
}
