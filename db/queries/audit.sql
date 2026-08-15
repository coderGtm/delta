-- name: InsertAuditEvent :one
INSERT INTO audit_events (actor_user_id, action, entity_type, entity_id, metadata_json, ip_address, user_agent)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;