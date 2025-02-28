package services

import (
	"context"
	"encoding/json"

	db "go-data-distributor-notification/internal/db/sqlc"
	"go-data-distributor-notification/internal/dto"
)

type NotificationService struct {
	store        db.Querier
	emailService *EmailService
}

func NewNotificationService(store db.Querier, emailService *EmailService) *NotificationService {
	return &NotificationService{
		store:        store,
		emailService: emailService,
	}
}

func (s *NotificationService) Create(ctx context.Context, req *dto.CreateNotificationRequest) (*dto.Notification, error) {
	data, err := json.Marshal(req.Data)
	if err != nil {
		return nil, err
	}

	params := db.CreateNotificationParams{
		UserID:     req.UserID,
		TenantID:   req.TenantID,
		TemplateID: req.TemplateID,
		Type:       req.Type,
		Status:     "PENDING",
		Data:       data,
	}

	notification, err := s.store.CreateNotification(ctx, params)
	if err != nil {
		return nil, err
	}

	go s.processNotification(notification)

	return &dto.Notification{
		ID:         notification.ID,
		UserID:     notification.UserID,
		TenantID:   notification.TenantID,
		TemplateID: notification.TemplateID,
		Type:       notification.Type,
		Status:     notification.Status,
	}, nil
}

func (s *NotificationService) processNotification(notification db.Notifications) {
	// Implementation for processing different types of notifications
}

func (s *NotificationService) Get(ctx context.Context, id string) (*dto.Notification, error) {
	notification, err := s.store.GetNotification(ctx, id)
	if err != nil {
		return nil, err
	}

	return &dto.Notification{
		ID:         notification.ID,
		UserID:     notification.UserID,
		TenantID:   notification.TenantID,
		TemplateID: notification.TemplateID,
		Type:       notification.Type,
		Status:     notification.Status,
	}, nil
}
