CREATE TABLE templates (
    id VARCHAR PRIMARY KEY DEFAULT gen_random_uuid()::VARCHAR,
    tenant_id VARCHAR NOT NULL,
    name VARCHAR NOT NULL,
    type VARCHAR NOT NULL,
    subject VARCHAR,
    content TEXT NOT NULL,
    variables JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    CONSTRAINT check_template_type CHECK (type IN ('EMAIL', 'SMS', 'PUSH'))
);

CREATE INDEX idx_templates_tenant_id ON templates(tenant_id);
CREATE INDEX idx_templates_type ON templates(type);
CREATE INDEX idx_templates_name ON templates(name);