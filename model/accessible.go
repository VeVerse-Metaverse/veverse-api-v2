package model

import (
	"context"
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"errors"
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"
	"veverse-api/database"
	"veverse-api/reflect"
)

// Entity accessible trait
type Accessible struct {
	EntityTrait

	User      *User `json:"user,omitempty"`
	IsOwner   bool  `json:"isOwner"`
	CanView   bool  `json:"canView"`
	CanEdit   bool  `json:"canEdit"`
	CanDelete bool  `json:"canDelete"`

	Timestamps
}

var (
	AccessibleSingular = "accessible"
	AccessiblePlural   = "accessibles"
)

func EntityAccessible(ctx context.Context, userId uuid.UUID, entityId uuid.UUID) (isOwner bool, canView bool, canEdit bool, canDelete bool, err error) {
	q := `SELECT
    a.is_owner, a.can_view, a.can_edit, a.can_delete
FROM 
    accessibles a
	INNER JOIN entities e ON e.id = a.entity_id AND a.user_id = $1
WHERE e.id = $2`

	db := database.DB
	accessibles := db.QueryRow(ctx, q, userId, entityId)

	err = accessibles.Scan(&isOwner, &canView, &canEdit, &canDelete)
	if err != nil {
		return false, false, false, false, err
	}

	return isOwner, canView, canEdit, canDelete, err
}

func GetAccessEntityForAdmin(ctx context.Context, user *sm.User, entityId uuid.UUID, offset int64, limit int64) (accessibles []Accessible, total int32, err error) {
	db := database.DB

	q := `SELECT COUNT(entity_id) FROM accessibles WHERE entity_id = $1 AND user_id <> $2::uuid`
	row := db.QueryRow(ctx, q, entityId, user.Id)

	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", AccessiblePlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get entity accessibles")
	}

	if total > 0 {
		q = `SELECT a.user_id, u.name, a.is_owner, a.can_view, a.can_edit, a.can_delete, a.created_at, a.updated_at
FROM accessibles a
LEFT JOIN users u ON a.user_id = u.id
WHERE entity_id = $1 AND user_id <> $2::uuid
LIMIT $3 OFFSET $4`

		var rows pgx.Rows
		rows, err = db.Query(ctx, q, entityId, user.Id, limit, offset)
		if err != nil {
			logrus.Errorf("failed to query %s @ %s: %v", entitySingular, reflect.FunctionName(), err)
			return nil, -1, fmt.Errorf("failed to get entity accessibles")
		}

		defer func() {
			database.LogPgxStat("defer GetAccessEntityForAdmin")
			rows.Close()
		}()

		for rows.Next() {
			var a Accessible
			a.User = &User{}

			err = rows.Scan(&a.User.Id, &a.User.Name, &a.IsOwner, &a.CanView, &a.CanEdit, &a.CanDelete, &a.CreatedAt, &a.UpdatedAt)

			if err != nil {
				logrus.Errorf("failed to scan %s @ %s: %v", entitySingular, reflect.FunctionName(), err)
				return nil, -1, fmt.Errorf("failed to get entity accessibles")
			}

			accessibles = append(accessibles, a)
		}

		return accessibles, total, nil
	}

	return nil, 0, nil
}

func GetAccessEntityForRequester(ctx context.Context, user *sm.User, entityId uuid.UUID, offset int64, limit int64) (accessibles []Accessible, total int32, err error) {
	db := database.DB

	q := `SELECT COUNT(a.user_id) FROM accessibles a
LEFT JOIN accessibles a2 ON a.entity_id = a2.entity_id AND a.user_id <> $2::uuid
WHERE a.entity_id = $1 AND a2.user_id = $2::uuid`
	row := db.QueryRow(ctx, q, entityId, user.Id)

	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", AccessiblePlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get entity accessibles")
	}

	q = `SELECT a.user_id, u.name, a.is_owner, a.can_view, a.can_edit, a.can_delete, a.created_at, a.updated_at
FROM accessibles a
LEFT JOIN users u ON a.user_id = u.id
LEFT JOIN accessibles a2 ON a.entity_id = a2.entity_id AND a.user_id <> $2::uuid
WHERE a.entity_id = $1 AND a2.user_id = $2::uuid
LIMIT $3 OFFSET $4`

	if total > 0 {
		var rows pgx.Rows
		rows, err = db.Query(ctx, q, entityId, user.Id, limit, offset)
		if err != nil {
			logrus.Errorf("failed to query %s @ %s: %v", entitySingular, reflect.FunctionName(), err)
			return nil, -1, fmt.Errorf("failed to get entity accessibles")
		}

		defer func() {
			rows.Close()
			database.LogPgxStat("defer GetAccessEntityForRequester")
		}()

		for rows.Next() {
			var a Accessible
			a.User = &User{}

			err = rows.Scan(&a.User.Id, &a.User.Name, &a.IsOwner, &a.CanView, &a.CanEdit, &a.CanDelete, &a.CreatedAt, &a.UpdatedAt)

			if err != nil {
				logrus.Errorf("failed to scan %s @ %s: %v", entitySingular, reflect.FunctionName(), err)
				return nil, -1, fmt.Errorf("failed to get entity accessibles")
			}

			accessibles = append(accessibles, a)
		}

		return accessibles, total, nil
	}

	return nil, 0, nil
}

func AccessEntityForAdmin(ctx context.Context, user *sm.User, entityId uuid.UUID, data EntityAccessRequest) (err error) {
	var (
		db               = database.DB
		entityTotal      int64
		accessiblesTotal int64
	)

	q := `SELECT count(e.id) AS e_count, count(a.entity_id) AS a_count
FROM entities e
    LEFT JOIN accessibles a ON a.entity_id = e.id AND a.user_id = $1
WHERE e.id = $2`

	row := db.QueryRow(ctx, q, data.UserId, entityId)
	err = row.Scan(&entityTotal, &accessiblesTotal)

	if err != nil {
		return err
	}

	if entityTotal > 0 {
		if data.Public != nil {
			q = `UPDATE entities SET public = $1 WHERE id = $2`

			row = db.QueryRow(ctx, q, data.Public, user.Id)

			if err = row.Scan(); err != nil {
				if err.Error() != "no rows in result set" {
					logrus.Errorf("failed to query %s @ %s: %v", entitySingular, reflect.FunctionName(), err)
				}
			}
		}

		if accessiblesTotal > 0 {
			q = `UPDATE accessibles SET`

			isUpdate := false
			if data.CanView != nil {
				isUpdate = true
				q += fmt.Sprintf(" can_view = %t", *data.CanView)
			}

			if data.CanEdit != nil {
				if isUpdate {
					q += ","
				} else {
					isUpdate = true
				}

				q += fmt.Sprintf(" can_edit = %t", *data.CanEdit)
			}

			if data.CanDelete != nil {
				if isUpdate {
					q += ","
				} else {
					isUpdate = true
				}

				q += fmt.Sprintf(" can_delete = %t", *data.CanDelete)
			}

			if isUpdate {
				q += ` WHERE entity_id = $1 AND user_id = $2`
				row = db.QueryRow(ctx, q, entityId, data.UserId)

				if err = row.Scan(); err != nil {
					if err.Error() != "no rows in result set" {
						logrus.Errorf("failed to query %s @ %s: %v", AccessibleSingular, reflect.FunctionName(), err)
						return err
					}
				}
			}
		} else {
			//region Accessible
			b := false
			if data.CanView == nil {
				data.CanView = &b
			}

			if data.CanEdit == nil {
				data.CanEdit = &b
			}

			if data.CanDelete == nil {
				data.CanDelete = &b
			}

			q = `INSERT INTO accessibles (user_id, entity_id, is_owner, can_view, can_edit, can_delete) VALUES ($1, $2, false, $3, $4, $5)`
			if _, err = db.Query(ctx, q, data.UserId, entityId, data.CanView, data.CanEdit, data.CanDelete); err != nil {
				logrus.Errorf("failed to query %s @ %s: %v", AccessibleSingular, reflect.FunctionName(), err)
				return err
			}
			//endregion
		}
	}

	return nil
}

func AccessEntityForRequester(ctx context.Context, user *sm.User, entityId uuid.UUID, data EntityAccessRequest) (err error) {
	var (
		db               = database.DB
		entityTotal      int64
		accessiblesTotal int64
	)

	q := `SELECT count(e.id) AS e_count, count(a.entity_id) AS a_count
FROM entities e 
    LEFT JOIN accessibles a ON a.entity_id = e.id AND (a.user_id = $1 OR (e.id = $3 AND a.user_id = $2 AND is_owner = false))
WHERE e.id = $3`

	row := db.QueryRow(ctx, q, user.Id, data.UserId, entityId)
	err = row.Scan(&entityTotal, &accessiblesTotal)

	if err != nil {
		return err
	}

	if entityTotal > 0 {
		if data.Public != nil {
			q = `UPDATE entities SET public = $1 WHERE id = $2`

			row = db.QueryRow(ctx, q, data.Public, user.Id)

			if err = row.Scan(); err != nil {
				if err.Error() != "no rows in result set" {
					logrus.Errorf("failed to query %s @ %s: %v", entitySingular, reflect.FunctionName(), err)
				}
			}
		}

		if accessiblesTotal > 1 {
			q = `UPDATE accessibles SET`

			isUpdate := false
			if data.CanView != nil {
				isUpdate = true
				q += fmt.Sprintf(" can_view = %t", *data.CanView)
			}

			if data.CanEdit != nil {
				if isUpdate {
					q += ","
				} else {
					isUpdate = true
				}

				q += fmt.Sprintf(" can_edit = %t", *data.CanEdit)
			}

			if data.CanDelete != nil {
				if isUpdate {
					q += ","
				} else {
					isUpdate = true
				}

				q += fmt.Sprintf(" can_delete = %t", *data.CanDelete)
			}

			if isUpdate {
				q += ` WHERE entity_id = $1 AND user_id = $2`
				row = db.QueryRow(ctx, q, entityId, data.UserId)

				if err = row.Scan(); err != nil {
					if err.Error() != "no rows in result set" {
						logrus.Errorf("failed to query %s @ %s: %v", AccessibleSingular, reflect.FunctionName(), err)
						return err
					}
				}
			}
		} else if accessiblesTotal > 0 {
			//region Accessible
			b := false
			if data.CanView == nil {
				data.CanView = &b
			}

			if data.CanEdit == nil {
				data.CanEdit = &b
			}

			if data.CanDelete == nil {
				data.CanDelete = &b
			}

			q = `INSERT INTO accessibles (user_id, entity_id, is_owner, can_view, can_edit, can_delete) VALUES ($1, $2, false, $3, $4, $5)`
			if _, err = db.Query(ctx, q, data.UserId, entityId, data.CanView, data.CanEdit, data.CanDelete); err != nil {
				logrus.Errorf("failed to query %s @ %s: %v", AccessibleSingular, reflect.FunctionName(), err)
				return err
			}
			//endregion
		}
	}

	return nil
}

func PublicAccessEntityForAdmin(ctx context.Context, user *sm.User, entityId uuid.UUID, data EntityPublicAccessRequest) (err error) {
	var (
		db = database.DB
	)

	q := `UPDATE entities SET public = $1 WHERE id = $2`
	_, err = db.Exec(ctx, q, data.Public, entityId)
	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", entitySingular, reflect.FunctionName(), err)
		return err
	}

	return nil
}

func PublicAccessEntityForRequester(ctx context.Context, user *sm.User, entityId uuid.UUID, data EntityPublicAccessRequest) (err error) {
	var (
		db      = database.DB
		isOwner bool
	)

	// check if user has ownership of the entity
	q := `SELECT a.is_owner FROM accessibles a WHERE a.user_id = $1 AND a.entity_id = $2`
	row := db.QueryRow(ctx, q, user.Id, entityId)
	err = row.Scan(&isOwner)

	if err != nil {
		return err
	}

	if !isOwner {
		return errors.New("user is not owner of the entity")
	}

	q = `UPDATE entities SET public = $1 WHERE id = $2`
	_, err = db.Exec(ctx, q, data.Public, entityId)
	if err != nil {
		return err
	}

	return nil
}
