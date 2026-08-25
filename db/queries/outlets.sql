-- name: CreateOutlet :one
INSERT INTO outlets (name, latitude, longitude, radius_meters)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetOutletByID :one
SELECT * FROM outlets WHERE id = $1 AND removed_at IS NULL LIMIT 1;

-- name: GetOutletByIDIncludingDeleted :one
SELECT * FROM outlets WHERE id = $1 LIMIT 1;

-- name: UpdateOutlet :one
UPDATE outlets
SET name = $2, latitude = $3, longitude = $4, radius_meters = $5, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateOutletGeofence :one
UPDATE outlets SET geofence_enabled = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: UpdateOutletRecentEntriesVisibility :one
UPDATE outlets SET show_recent_entries_to_employees = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: UpdateOutletTotalTimeTodayVisibility :one
UPDATE outlets SET show_total_time_today_to_employees = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: DeleteOutlet :one
UPDATE outlets SET removed_at = now(), removed_by_user_id = $2, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: CreateMembership :one
INSERT INTO outlet_memberships
  (outlet_id, user_id, role, status, display_name, invited_by_user_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetMembershipByOutletAndUser :one
SELECT * FROM outlet_memberships
WHERE outlet_id = $1 AND user_id = $2 AND removed_at IS NULL LIMIT 1;

-- name: GetMembershipByOutletAndUserIncludingRemoved :one
SELECT * FROM outlet_memberships
WHERE outlet_id = $1 AND user_id = $2 LIMIT 1;

-- name: GetMembershipByIDIncludingRemoved :one
SELECT * FROM outlet_memberships WHERE id = $1 LIMIT 1;

-- name: UpdateMembershipInvite :one
UPDATE outlet_memberships
SET role = $2, status = $3, invited_by_user_id = $4, removed_at = NULL, removed_by_user_id = NULL, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateMembershipStatus :one
UPDATE outlet_memberships SET status = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: UpdateMembershipDisplayName :one
UPDATE outlet_memberships SET display_name = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: RemoveMembership :one
UPDATE outlet_memberships
SET removed_at = now(), removed_by_user_id = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListMembershipsForUserByStatus :many
SELECT m.*, o.id AS outlet_id, o.name AS outlet_name, o.latitude, o.longitude, o.radius_meters,
       o.geofence_enabled, o.show_recent_entries_to_employees, o.show_total_time_today_to_employees,
       o.removed_at AS outlet_removed_at, o.created_at AS outlet_created_at,
       o.updated_at AS outlet_updated_at
FROM outlet_memberships m
JOIN outlets o ON o.id = m.outlet_id
WHERE m.user_id = $1 AND m.status = $2 AND m.removed_at IS NULL AND o.removed_at IS NULL
ORDER BY m.updated_at DESC
LIMIT $3 OFFSET $4;

-- name: CountMembershipsForUserByStatus :one
SELECT count(*) FROM outlet_memberships m
JOIN outlets o ON o.id = m.outlet_id
WHERE m.user_id = $1 AND m.status = $2 AND m.removed_at IS NULL AND o.removed_at IS NULL;

-- name: ListMembershipsForOutlet :many
SELECT m.*, u.id AS user_id, u.name AS user_name, u.email AS user_email,
       iu.id AS invited_by_user_id, iu.name AS invited_by_user_name
FROM outlet_memberships m
JOIN users u ON u.id = m.user_id
LEFT JOIN users iu ON iu.id = m.invited_by_user_id
WHERE m.outlet_id = $1 AND m.removed_at IS NULL
ORDER BY m.created_at ASC
LIMIT $2 OFFSET $3;

-- name: CountMembershipsForOutlet :one
SELECT count(*) FROM outlet_memberships WHERE outlet_id = $1 AND removed_at IS NULL;

-- name: GetMembershipDetailsByID :one
SELECT m.id, m.outlet_id, m.user_id, m.role, m.status, m.display_name, m.invited_by_user_id, m.removed_at, m.removed_by_user_id, m.created_at, m.updated_at,
       u.id AS user_id, u.name AS user_name, u.email AS user_email,
       o.id AS outlet_id, o.name AS outlet_name, o.latitude, o.longitude, o.radius_meters,
       o.geofence_enabled, o.show_recent_entries_to_employees, o.show_total_time_today_to_employees,
       o.removed_at AS outlet_removed_at, o.created_at AS outlet_created_at,
       o.updated_at AS outlet_updated_at,
       iu.id AS invited_by_user_id, iu.name AS invited_by_user_name
FROM outlet_memberships m
JOIN users u ON u.id = m.user_id
JOIN outlets o ON o.id = m.outlet_id
LEFT JOIN users iu ON iu.id = m.invited_by_user_id
WHERE m.id = $1 AND m.removed_at IS NULL
LIMIT 1;

-- name: ListMembershipsForOutletByUser :many
SELECT * FROM outlet_memberships
WHERE outlet_id = $1 AND user_id = $2
ORDER BY created_at ASC;

-- name: ListActiveOwnedOutletsByUser :many
SELECT o.id, o.name
FROM outlets o
JOIN outlet_memberships m ON m.outlet_id = o.id
WHERE m.user_id = $1 AND m.role = 'OWNER' AND m.status = 'ACCEPTED'
  AND m.removed_at IS NULL AND o.removed_at IS NULL
ORDER BY o.name;