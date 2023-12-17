package model

import "github.com/gofrs/uuid"

// Identifier
type Identifier struct {
	Id *uuid.UUID `json:"id,omitempty"`
}

type IdentifierWithValidation struct {
	Id *uuid.UUID `json:"id,omitempty" validate:"required"`
}
