-- name: CreateAttendanceEntry :one
INSERT INTO attendance_entries (user_id, outlet_id, type, entry_time, latitude, longitude, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetAttendanceEntryByIDAndOutlet :one
SELECT * FROM attendance_entries WHERE id = $1 AND outlet_id = $2 LIMIT 1;

-- name: UpdateAttendanceEntry :one
UPDATE attendance_entries
SET type = $2, entry_time = $3, latitude = $4, longitude = $5, updated_by = $6, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteAttendanceEntry :exec
DELETE FROM attendance_entries WHERE id = $1;

-- name: ListAttendanceByOutlet :many
SELECT * FROM attendance_entries
WHERE outlet_id = $1
ORDER BY entry_time DESC, created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountAttendanceByOutlet :one
SELECT count(*) FROM attendance_entries WHERE outlet_id = $1;

-- name: ListAttendanceByOutletAndUser :many
SELECT * FROM attendance_entries
WHERE outlet_id = $1 AND user_id = $2
ORDER BY entry_time DESC, created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountAttendanceByOutletAndUser :one
SELECT count(*) FROM attendance_entries WHERE outlet_id = $1 AND user_id = $2;

-- name: ListAttendanceByOutletUserRange :many
SELECT * FROM attendance_entries
WHERE outlet_id = $1 AND user_id = $2 AND entry_time >= $3 AND entry_time < $4
ORDER BY entry_time ASC;