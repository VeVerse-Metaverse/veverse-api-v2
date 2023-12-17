package model

import (
	"context"
	"dev.hackerman.me/artheon/veverse-shared/model"
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"veverse-api/database"
	"veverse-api/reflect"
)

type Tag struct {
	Identifier
	Name string
}

var (
	tagSingular = "tag"
	tagPlural   = "tags"
)

func GetTagsForAdmin(ctx context.Context, entityId uuid.UUID, offset int64, limit int64) (tags []Tag, total int32, err error) {

	var (
		q    string
		row  pgx.Row
		rows pgx.Rows
		db   *pgxpool.Pool
	)

	q = `SELECT COUNT(t.id) FROM tags t LEFT JOIN entity_tags et ON et.tag_id = t.id WHERE et.entity_id = $1`
	db = database.DB
	row = db.QueryRow(ctx, q, entityId)

	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", tagPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get entity tags")
	}

	q = `SELECT t.id, t.name
FROM tags t
    LEFT JOIN entity_tags et ON et.tag_id = t.id
WHERE et.entity_id = $1 OFFSET $2 LIMIT $3`

	rows, err = db.Query(ctx, q, entityId, offset, limit)
	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", tagPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get entity tags")
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetTagsForAdmin")
	}()
	for rows.Next() {
		var tag Tag
		err = rows.Scan(&tag.Id, &tag.Name)
		if err != nil {
			logrus.Errorf("failed to scan %s @ %s: %v", tagPlural, reflect.FunctionName(), err)
			return nil, -1, fmt.Errorf("failed to get entity tags")
		}

		tags = append(tags, tag)
	}

	return tags, total, nil
}

func GetTagsForRequester(ctx context.Context, requester *model.User, entityId uuid.UUID, offset int64, limit int64) (tags []Tag, total int32, err error) {

	var (
		q    string
		row  pgx.Row
		rows pgx.Rows
		db   *pgxpool.Pool
	)

	q = `SELECT COUNT(t.id)
FROM tags t
    LEFT JOIN entity_tags et ON et.tag_id = t.id
    LEFT JOIN accessibles a on et.entity_id = a.entity_id
WHERE et.entity_id = $1 AND a.user_id = $2`

	db = database.DB
	row = db.QueryRow(ctx, q, entityId, requester.Id)

	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", tagPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get entity tags")
	}

	q = `SELECT t.id, t.name
FROM tags t
    LEFT JOIN entity_tags et ON et.tag_id = t.id
    LEFT JOIN accessibles a on et.entity_id = a.entity_id
WHERE et.entity_id = $1 AND a.user_id = $2 OFFSET $3 LIMIT $4`

	rows, err = db.Query(ctx, q, entityId, requester.Id, offset, limit)
	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", tagPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get entity tags")
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetTagsForRequester")
	}()
	for rows.Next() {
		var tag Tag
		err = rows.Scan(&tag.Id, &tag.Name)
		if err != nil {
			logrus.Errorf("failed to scan %s @ %s: %v", tagPlural, reflect.FunctionName(), err)
			return nil, -1, fmt.Errorf("failed to get entity tags")
		}

		tags = append(tags, tag)
	}

	return tags, total, nil
}
