package model

import "github.com/gofrs/uuid"

type IdRequestMetadata struct {
	Id uuid.UUID `json:"id"` // Entity ID
}

type IdRequestMetadataWithValidation struct {
	Id string `json:"id" validate:"uuid"` // Entity ID
}

type BatchRequestMetadata struct {
	Offset int64  `json:"offset"` // Start index
	Limit  int64  `json:"limit"`  // Number of elements to fetch
	Query  string `json:"query"`  // Search query string
}

type KeyRequestMetadata struct {
	Key string `json:"key"`
}
