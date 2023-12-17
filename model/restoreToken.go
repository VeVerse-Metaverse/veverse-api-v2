package model

import (
	"context"
	"github.com/gofrs/uuid"
	"time"
	"veverse-api/database"
)

type RestoreToken struct {
	Identifier
	UserId    *uuid.UUID `json:"userId,omitempty"`
	Token     string     `json:"token,omitempty"`
	IsValid   bool       `json:"isValid,omitempty"`
	CreatedAt time.Time  `json:"createdAt,omitempty"`
}

func AddRestoreToken(ctx context.Context, userId *uuid.UUID, token string) (err error) {
	db := database.DB
	q := `INSERT INTO restore_tokens (user_id, token, is_valid) VALUES ($1, $2, $3)`

	if _, err = db.Query(ctx, q, userId, token, true); err != nil {
		return err
	}

	return nil
}

func CheckRestoreToken(ctx context.Context, userId *uuid.UUID, exp int8) (hasActive bool, err error) {
	db := database.DB
	q := `SELECT count(id) FROM restore_tokens WHERE user_id = $1 AND created_at > (now() - INTERVAL '1 hour' * $2) AND is_valid = true`
	row := db.QueryRow(ctx, q, userId, exp)

	var (
		total int32
	)

	err = row.Scan(&total)

	return total > 0, nil
}

func SetRestoreTokensToInvalid(ctx context.Context, requester User) error {
	db := database.DB

	q := `UPDATE restore_tokens SET is_valid = false WHERE user_id = $1`
	row := db.QueryRow(ctx, q, requester.Id)

	if err := row.Scan(); err != nil {
		return err
	}

	return nil
}
