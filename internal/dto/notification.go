package dto

type CreateNotificationRequest struct {
	UserID     string                 `json:"user_id" binding:"required"`
	TenantID   string                 `json:"tenant_id" binding:"required"`
	TemplateID string                 `json:"template_id" binding:"required"`
	Type       string                 `json:"type" binding:"required"`
	Data       map[string]interface{} `json:"data" binding:"required"`
}

type Notification struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	TenantID   string `json:"tenant_id"`
	TemplateID string `json:"template_id"`
	Type       string `json:"type"`
	Status     string `json:"status"`
}
