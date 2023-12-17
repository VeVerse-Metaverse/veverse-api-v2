package model

import "github.com/gofrs/uuid"

type EntityIdentifier struct {
	EntityId *uuid.UUID `json:"entityId,omitempty"`
}

// EntityTrait Base for traits related to the entity
type EntityTrait struct {
	Identifier
	EntityId *uuid.UUID `json:"entityId,omitempty"`
}
