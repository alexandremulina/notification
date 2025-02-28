-- name: GetNotification :one
SELECT * FROM notifications
WHERE id = $1 AND deleted_at IS NULL
LIMIT 1;

-- name: CreateNotification :one
INSERT INTO notifications (
    user_id, tenant_id, template_id, type, status, data
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: GetNotificationByID :one
SELECT * FROM notifications
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListNotificationsByUserID :many
SELECT * FROM notifications
WHERE user_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListNotificationsByTenantID :many
SELECT * FROM notifications
WHERE tenant_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateNotificationStatus :one
UPDATE notifications
SET 
    status = $2,
    sent_at = CASE WHEN $2 = 'SENT' THEN CURRENT_TIMESTAMP ELSE sent_at END,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: SoftDeleteNotification :exec
UPDATE notifications
SET 
    deleted_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1;

-- name: GetPendingNotifications :many
SELECT * FROM notifications
WHERE status = 'PENDING' 
AND deleted_at IS NULL
ORDER BY created_at ASC
LIMIT $1;

-- name: GetNotificationsByTemplateID :many
SELECT * FROM notifications
WHERE template_id = $1 
AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
