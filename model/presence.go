package model

import (
	pgtypeuuid "github.com/jackc/pgtype/ext/gofrs-uuid"
	"time"
)

// Presence struct
type Presence struct {
	UpdatedAt *time.Time       `json:"updatedAt,omitempty"`
	Status    *string          `json:"status,omitempty"`
	WorldId   *pgtypeuuid.UUID `json:"spaceId,omitempty"`
	ServerId  *pgtypeuuid.UUID `json:"serverId,omitempty"`
}
