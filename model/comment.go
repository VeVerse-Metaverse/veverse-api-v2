package model

import (
	"context"
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"veverse-api/database"
	"veverse-api/reflect"
)

type Comment struct {
	//Entity
	EntityTrait

	UserId uuid.UUID `json:"userId"`
	Text   string    `json:"text"`
}

var (
	commentSingular = "comment"
	commentPlural   = "comments"
)

func GetCommentsForAdmin(ctx context.Context, entityId uuid.UUID, offset int64, limit int64) (comments []Comment, total int32, err error) {

	var (
		q    string
		row  pgx.Row
		rows pgx.Rows
		db   *pgxpool.Pool
	)

	q = `SELECT COUNT(c.id) FROM comments c LEFT JOIN entities e on e.id = c.entity_id WHERE e.id = $1`
	db = database.DB
	row = db.QueryRow(ctx, q, entityId)

	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", commentPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get entity comments")
	}

	q = `SELECT c.id, c.text
FROM comments c
    LEFT JOIN entities e on e.id = c.entity_id
WHERE e.id = $1 OFFSET $2 LIMIT $3`

	rows, err = db.Query(ctx, q, entityId, offset, limit)
	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", commentPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get entity comments")
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetCommentsForAdmin")
	}()
	for rows.Next() {
		var comment Comment
		err = rows.Scan(&comment.Id, &comment.Text)
		if err != nil {
			logrus.Errorf("failed to scan %s @ %s: %v", commentPlural, reflect.FunctionName(), err)
			return nil, -1, fmt.Errorf("failed to get entity comments")
		}

		comments = append(comments, comment)
	}

	return comments, total, nil
}
func GetCommentsForRequester(ctx context.Context, requester *sm.User, entityId uuid.UUID, offset int64, limit int64) (comments []Comment, total int32, err error) {

	var (
		q    string
		row  pgx.Row
		rows pgx.Rows
		db   *pgxpool.Pool
	)

	q = `SELECT COUNT(c.id)
FROM comments c 
    LEFT JOIN entities e on e.id = c.entity_id
	LEFT JOIN accessibles a on e.id = a.entity_id
WHERE e.id = $1 AND a.user_id = $2 AND (e.public OR a.can_view OR a.is_owner)`

	db = database.DB
	row = db.QueryRow(ctx, q, entityId, requester.Id)
	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", commentPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get entity comments")
	}

	q = `SELECT c.id, c.text
FROM comments c
	LEFT JOIN  entities e on e.id = c.entity_id
	LEFT JOIN accessibles a on e.id = a.entity_id
WHERE e.id = $1 AND a.user_id = $2 AND (e.public OR a.can_view OR a.is_owner) OFFSET $3 LIMIT $4`

	rows, err = db.Query(ctx, q, entityId, requester.Id, offset, limit)
	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", commentPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get entity comments")
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetCommentsForRequester")
	}()
	for rows.Next() {
		var comment Comment
		err = rows.Scan(&comment.Id, &comment.Text)
		if err != nil {
			logrus.Errorf("failed to scan %s @ %s: %v", commentPlural, reflect.FunctionName(), err)
			return nil, -1, fmt.Errorf("failed to get entity comments")
		}

		comments = append(comments, comment)
	}

	return comments, total, nil
}
