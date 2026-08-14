package attendance

import (
	"context"
	"fmt"

	"github.com/coderGtm/delta/go/db"
	"github.com/coderGtm/delta/go/httpapi"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// attendanceByOutletSQL lists the attendance entries of an outlet. The %s
// placeholder is replaced with a whitelisted ORDER BY clause.
const attendanceByOutletSQL = `SELECT id, outlet_id, user_id, type, entry_time, latitude, longitude, created_by, updated_by, created_at, updated_at
FROM attendance_entries
WHERE outlet_id = $1 %s
LIMIT $2 OFFSET $3`

// attendanceByOutletAndUserSQL lists the attendance entries of one user in an
// outlet. The %s placeholder is replaced with a whitelisted ORDER BY clause.
const attendanceByOutletAndUserSQL = `SELECT id, outlet_id, user_id, type, entry_time, latitude, longitude, created_by, updated_by, created_at, updated_at
FROM attendance_entries
WHERE outlet_id = $1 AND user_id = $2 %s
LIMIT $3 OFFSET $4`

// attendanceRow is an attendance entry row for the paginated listing queries.
// The db tags match the selected columns exactly, as required by
// pgx.RowToStructByName.
type attendanceRow struct {
	ID        pgtype.UUID        `db:"id"`
	OutletID  pgtype.UUID        `db:"outlet_id"`
	UserID    pgtype.UUID        `db:"user_id"`
	Type      string             `db:"type"`
	EntryTime pgtype.Timestamptz `db:"entry_time"`
	Latitude  pgtype.Numeric     `db:"latitude"`
	Longitude pgtype.Numeric     `db:"longitude"`
	CreatedBy pgtype.UUID        `db:"created_by"`
	UpdatedBy pgtype.UUID        `db:"updated_by"`
	CreatedAt pgtype.Timestamptz `db:"created_at"`
	UpdatedAt pgtype.Timestamptz `db:"updated_at"`
}

// attendanceSortable maps client sort field names to their SQL column names
// for the attendance listings.
var attendanceSortable = map[string]string{
	"id":        "id",
	"type":      "type",
	"entryTime": "entry_time",
	"createdAt": "created_at",
	"updatedAt": "updated_at",
}

// List returns the pages of attendance entries for an outlet. Owners may view
// every entry or filter by user; employees may only view their own entries.
func (s *Service) List(ctx context.Context, callerID, outletID uuid.UUID, targetUserID *uuid.UUID, p httpapi.PageParams) (*httpapi.PageResponse[EntryResponse], error) {
	m, err := s.assertAcceptedMembership(ctx, outletID, callerID, "Outlet membership was not found for the current user")
	if err != nil {
		return nil, err
	}
	if m.Role != "OWNER" {
		if targetUserID != nil && *targetUserID != callerID {
			return nil, httpapi.Forbidden("Employees can only view their own attendance entries")
		}
		targetUserID = &callerID
	}
	order, _ := p.OrderClause(attendanceSortable)
	if order == "" {
		order = " ORDER BY entry_time DESC, created_at DESC"
	}
	var rows pgx.Rows
	if targetUserID == nil {
		rows, err = s.Store.Pool().Query(ctx, fmt.Sprintf(attendanceByOutletSQL, order), pgUUID(outletID), int32(p.Size), int32(p.Page*p.Size))
	} else {
		rows, err = s.Store.Pool().Query(ctx, fmt.Sprintf(attendanceByOutletAndUserSQL, order), pgUUID(outletID), pgUUID(*targetUserID), int32(p.Size), int32(p.Page*p.Size))
	}
	if err != nil {
		return nil, err
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[attendanceRow])
	if err != nil {
		return nil, err
	}
	var total int64
	if targetUserID == nil {
		total, err = s.Store.Querier().CountAttendanceByOutlet(ctx, pgUUID(outletID))
	} else {
		total, err = s.Store.Querier().CountAttendanceByOutletAndUser(ctx, db.CountAttendanceByOutletAndUserParams{
			OutletID: pgUUID(outletID),
			UserID:   pgUUID(*targetUserID),
		})
	}
	if err != nil {
		return nil, err
	}
	out := make([]EntryResponse, 0, len(collected))
	for _, row := range collected {
		e := db.AttendanceEntry{
			ID:        row.ID,
			OutletID:  row.OutletID,
			UserID:    row.UserID,
			Type:      row.Type,
			EntryTime: row.EntryTime,
			Latitude:  row.Latitude,
			Longitude: row.Longitude,
			CreatedBy: row.CreatedBy,
			UpdatedBy: row.UpdatedBy,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		}
		u, err := s.Store.Querier().GetUserByIDIncludingDeleted(ctx, pgUUID(toUUID(e.UserID)))
		if err != nil {
			return nil, err
		}
		dn, err := s.memberDisplayName(ctx, outletID, toUUID(e.UserID), u.Name)
		if err != nil {
			return nil, err
		}
		item, err := s.toEntryResponse(&e, &u, dn)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return httpapi.NewPageResponse(out, total, p), nil
}
