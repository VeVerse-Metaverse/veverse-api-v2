package model

import (
	"context"
	"dev.hackerman.me/artheon/veverse-shared/model"
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"
	"veverse-api/database"
	"veverse-api/reflect"
)

type Entity struct {
	Identifier

	EntityType *string `json:"entityType,omitempty"`
	Public     *bool   `json:"public,omitempty"`
	Views      *int32  `json:"views,omitempty"`

	Timestamps

	Owner       *User        `json:"owner,omitempty"`
	Accessibles []Accessible `json:"accessibles,omitempty"`
	Files       []File       `json:"files,omitempty"`
	Links       []Link       `json:"links,omitempty"`
	Properties  []Property   `json:"properties,omitempty"`
	Likables    []Likable    `json:"likables,omitempty"`
	Comments    []Comment    `json:"comments,omitempty"`
}

type EntityAccessRequest struct {
	UserId    uuid.UUID `json:"userId" validate:"required"`
	CanView   *bool     `json:"canView,omitempty"`
	CanEdit   *bool     `json:"canEdit,omitempty"`
	CanDelete *bool     `json:"canDelete,omitempty"`
	Public    *bool     `json:"public"`
}

type EntityPublicAccessRequest struct {
	Public *bool `json:"public"`
}

var (
	entitySingular = "entity"
	entityPlural   = "entities"
)

func GetEntityForAdmin(ctx context.Context, userId uuid.UUID) (entity *Entity, err error) {
	db := database.DB

	q := `SELECT id, entity_type, views, public, created_at, updated_at from entities WHERE id = $1`

	row := db.QueryRow(ctx, q, userId)
	var e = new(Entity)
	err = row.Scan(&e.Id, &e.EntityType, &e.Views, &e.Public, &e.CreatedAt, &e.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to query scan %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	entity = e
	return entity, nil
}

func GetEntityForRequester(ctx context.Context, userId uuid.UUID) (entity *Entity, err error) {
	db := database.DB

	q := `SELECT id, entity_type, views, public, e.created_at, e.updated_at from entities e
	LEFT JOIN accessibles a on e.id = a.entity_id AND a.user_id = $1 AND (e.public OR a.can_view OR a.is_owner)
	WHERE id = $1`

	row := db.QueryRow(ctx, q, userId)
	var e = new(Entity)
	err = row.Scan(&e.Id, &e.EntityType, &e.Views, &e.Public, &e.CreatedAt, &e.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to query scan %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	entity = e
	return entity, nil
}

func DeleteEntityForAdmin(ctx context.Context, entityId uuid.UUID) (err error) {
	db := database.DB

	q := `DELETE FROM entities WHERE id = $1 AND entity_type <> 'user'`

	_, err = db.Exec(ctx, q, entityId)

	if err != nil {
		return fmt.Errorf("failed to delete query %s @ %s: %v", entitySingular, reflect.FunctionName(), err)
	}

	return nil
}

func DeleteEntityForRequester(ctx context.Context, entityId uuid.UUID) (err error) {
	db := database.DB

	q := `DELETE FROM entities e
	USING accessibles a
	WHERE e.id = $1 AND e.entity_type <> 'user' AND e.id = a.entity_id AND (e.public OR a.can_view OR a.is_owner)`

	_, err = db.Exec(ctx, q, entityId)

	if err != nil {
		return fmt.Errorf("failed to delete query %s @ %s: %v", entitySingular, reflect.FunctionName(), err)
	}

	return nil
}

func IncrementViewEntityForAdmin(ctx context.Context, entityId uuid.UUID) (err error) {
	db := database.DB

	q := `UPDATE entities SET views = views + 1 WHERE id = $1`

	_, err = db.Exec(ctx, q, entityId)

	if err != nil {
		return fmt.Errorf("failed to update query %s @ %s: %v", entitySingular, reflect.FunctionName(), err)
	}

	return nil
}

func IncrementViewEntityForRequester(ctx context.Context, user *model.User, entityId uuid.UUID) (err error) {
	db := database.DB

	var (
		row   pgx.Row
		total int32
	)

	q := `SELECT COUNT(id) FROM entities e INNER JOIN accessibles a on e.id = a.entity_id WHERE a.entity_id = $1 AND a.user_id = $2 AND (e.public OR a.can_view OR a.is_owner)`
	row = db.QueryRow(ctx, q, entityId, user.Id)

	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", entitySingular, reflect.FunctionName(), err)
		return fmt.Errorf("failed to increment entity views")
	}

	if total > 0 {
		q = `UPDATE entities AS e SET views = e.views + 1 FROM accessibles AS a WHERE e.id = $1 AND a.user_id = $2 AND e.id = a.entity_id AND (e.public OR a.can_view OR a.is_owner)`

		_, err = db.Exec(ctx, q, entityId, user.Id)

		if err != nil {
			logrus.Errorf("failed to increment entity view query %s @ %s: %v", entitySingular, reflect.FunctionName(), err)
			return fmt.Errorf("failed to increment entity views")
		}

		return nil
	}

	return fmt.Errorf("no entity to increment views")
}
