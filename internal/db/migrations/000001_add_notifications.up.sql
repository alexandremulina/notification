CREATE TABLE notifications (
    id VARCHAR PRIMARY KEY DEFAULT gen_random_uuid()::VARCHAR,
    user_id VARCHAR NOT NULL,
    tenant_id VARCHAR NOT NULL,
    template_id VARCHAR NOT NULL,
    type VARCHAR NOT NULL,
    status VARCHAR NOT NULL,
    data JSONB NOT NULL,
    sent_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    CONSTRAINT check_notification_status CHECK (status IN ('PENDING', 'SENT', 'FAILED'))
);

CREATE INDEX idx_notifications_user_id ON notifications(user_id);
CREATE INDEX idx_notifications_tenant_id ON notifications(tenant_id);
CREATE INDEX idx_notifications_template_id ON notifications(template_id);
CREATE INDEX idx_notifications_status ON notifications(status);
CREATE INDEX idx_notifications_type ON notifications(type);
CREATE INDEX idx_notifications_created_at ON notifications(created_at);
CREATE INDEX idx_notifications_sent_at ON notifications(sent_at);