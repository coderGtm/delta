package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/coderGtm/delta/go/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Recorder struct {
	store *db.Store
}

func NewRecorder(store *db.Store) *Recorder { return &Recorder{store: store} }

// Record persists an audit event in its own transaction so a failure here
// never rolls back the business write. Failures are logged and dropped.
func (r *Recorder) Record(ctx context.Context, actorUserID, action, entityType string, entityID uuid.UUID, metadata map[string]any, ip, userAgent string) {
	var metaJSON []byte
	var err error
	if len(metadata) > 0 {
		metaJSON, err = json.Marshal(metadata)
		if err != nil {
			slog.Error("audit: marshal metadata", "action", action, "err", err)
			return
		}
	}
	ua := userAgent
	if len(ua) > 500 {
		ua = ua[:500]
	}
	ipStr := ip
	if len(ipStr) > 100 {
		ipStr = ipStr[:100]
	}

	actorID := pgtype.UUID{}
	if actorUserID != "" {
		if id, perr := uuid.Parse(actorUserID); perr == nil {
			actorID = pgtype.UUID{Bytes: id, Valid: true}
		}
	}

	txCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := r.store.Tx(txCtx, func(q db.Querier) error {
		_, err := q.InsertAuditEvent(txCtx, db.InsertAuditEventParams{
			ActorUserID:  actorID,
			Action:       action,
			EntityType:   entityType,
			EntityID:     pgtype.UUID{Bytes: entityID, Valid: true},
			MetadataJson: metaJSON,
			IpAddress:    pgtype.Text{String: ipStr, Valid: true},
			UserAgent:    pgtype.Text{String: ua, Valid: true},
		})
		return err
	}); err != nil {
		slog.Error("audit: insert failed", "action", action, "err", err)
	}
}
