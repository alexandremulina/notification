-- name: CreateTemplate :one
INSERT INTO templates (
    tenant_id,
    name,
    type,
    subject,
    content,
    variables
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetTemplateByID :one
SELECT * FROM templates
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListTemplatesByTenantID :many
SELECT * FROM templates
WHERE tenant_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListTemplatesByType :many
SELECT * FROM templates
WHERE tenant_id = $1 AND type = $2 AND deleted_at IS NULL
ORDER BY name ASC
LIMIT $3 OFFSET $4;

-- name: UpdateTemplate :one
UPDATE templates
SET 
    name = $2,
    type = $3,
    subject = $4,
    content = $5,
    variables = $6,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteTemplate :exec
UPDATE templates
SET 
    deleted_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1;

-- name: GetTemplateByName :one
SELECT * FROM templates
WHERE tenant_id = $1 AND name = $2 AND deleted_at IS NULL;

-- name: SearchTemplates :many
SELECT * FROM templates
WHERE tenant_id = $1 
AND deleted_at IS NULL
AND (
    name ILIKE '%' || $2 || '%'
    OR subject ILIKE '%' || $2 || '%'
)
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;
