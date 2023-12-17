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

// Likable entity trait
type Likable struct {
	EntityTrait

	UserId uuid.UUID `json:"userId"` // User who liked or disliked the Entity
	Value  int8      `json:"value"`  // 1 for likes, -1 for dislikes

	Timestamps
}

type Rating struct {
	TotalLikes    int32 `json:"totalLikes"`
	TotalDislikes int32 `json:"totalDislikes"`
}

var (
	RatingSingular = "rating"
	RatingPlural   = "ratings"
)

func GetRatingsFor(ctx context.Context, entityId uuid.UUID) (rating *Rating, err error) {

	var (
		q   string
		row pgx.Row
		db  *pgxpool.Pool
	)

	q = `SELECT 
    SUM(CASE WHEN value >= 0 THEN value END) as total_likes,
  	SUM(CASE WHEN value < 0 THEN value END) as total_dislikes
FROM likables WHERE entity_id = $1`

	row = db.QueryRow(ctx, q, entityId)
	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", RatingSingular, reflect.FunctionName(), err)
		return nil, fmt.Errorf("failed to get entity rating")
	}

	err = row.Scan(&rating.TotalLikes, &rating.TotalDislikes)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", RatingPlural, reflect.FunctionName(), err)
		return nil, fmt.Errorf("failed to get entity ratings")
	}

	return rating, nil
}

func GetRatingsForAdmin(ctx context.Context, entityId uuid.UUID, offset int64, limit int64) (ratings []Likable, total int32, err error) {

	var (
		q    string
		row  pgx.Row
		rows pgx.Rows
		db   *pgxpool.Pool
	)

	q = `SELECT COUNT(l.id) FROM likables l LEFT JOIN entities e on e.id = l.entity_id WHERE e.id = $1`
	db = database.DB
	row = db.QueryRow(ctx, q, entityId)

	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", RatingPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get entity ratings")
	}

	q = `SELECT l.id, l.value, l.created_at, l.updated_at
FROM likables l
    LEFT JOIN entities e on e.id = l.entity_id
WHERE e.id = $1 OFFSET $2 LIMIT $3`

	rows, err = db.Query(ctx, q, entityId, offset, limit)
	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", RatingPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get entity ratings")
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetRatingsForAdmin")
	}()
	for rows.Next() {
		var rating Likable
		err = rows.Scan(&rating.Id, &rating.Value, &rating.CreatedAt, &rating.UpdatedAt)
		if err != nil {
			logrus.Errorf("failed to scan %s @ %s: %v", RatingPlural, reflect.FunctionName(), err)
			return nil, -1, fmt.Errorf("failed to get entity ratings")
		}

		ratings = append(ratings, rating)
	}

	return ratings, total, nil
}

func GetRatingsForRequester(ctx context.Context, requester *sm.User, entityId uuid.UUID, offset int64, limit int64) (ratings []Likable, total int32, err error) {

	var (
		q    string
		row  pgx.Row
		rows pgx.Rows
		db   *pgxpool.Pool
	)

	q = `SELECT COUNT(c.id)
FROM likables  c
    LEFT JOIN entities e on e.id = c.entity_id
	LEFT JOIN accessibles a on e.id = a.entity_id
WHERE e.id = $1 AND a.user_id = $2 AND (e.public OR a.can_view OR a.is_owner)`

	db = database.DB
	row = db.QueryRow(ctx, q, entityId, requester.Id)
	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", RatingPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get entity ratings")
	}

	q = `SELECT l.id, l.value, l.created_at, l.updated_at
FROM likables l
	LEFT JOIN  entities e on e.id = l.entity_id
	LEFT JOIN accessibles a on e.id = a.entity_id
WHERE e.id = $1 AND a.user_id = $2 AND (e.public OR a.can_view OR a.is_owner) OFFSET $3 LIMIT $4`

	rows, err = db.Query(ctx, q, entityId, requester.Id, offset, limit)
	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", RatingPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get entity ratings")
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetRatingsForRequester")
	}()
	for rows.Next() {
		var rating Likable
		err = rows.Scan(&rating.Id, &rating.Value, &rating.CreatedAt, &rating.UpdatedAt)
		if err != nil {
			logrus.Errorf("failed to scan %s @ %s: %v", RatingPlural, reflect.FunctionName(), err)
			return nil, -1, fmt.Errorf("failed to get entity ratings")
		}

		ratings = append(ratings, rating)
	}

	return ratings, total, nil
}

func SetRating(ctx context.Context, requester *sm.User, entityId uuid.UUID, rating int8) (err error) {
	db := database.DB

	if rating < 0 {
		rating = -1
	} else if rating > 0 {
		rating = 1
	} else {
		rating = 0
	}

	q := `UPDATE likables SET value = $1 WHERE user_id = $2 AND entity_id = $3`
	res, err := db.Exec(ctx, q, rating, requester.Id, entityId)

	if err == nil {
		if count := res.RowsAffected(); count == 0 {
			likableId, err1 := uuid.NewV4()
			if err1 != nil {
				logrus.Errorf("failed to generate uuid %s @ %s: %v", RatingSingular, reflect.FunctionName(), err)
				return fmt.Errorf("failed to set %s", RatingSingular)
			}

			q := `INSERT INTO likables (id, user_id, entity_id, value) VALUES ($1, $2, $3, $4)`
			if _, err = db.Query(ctx, q, likableId, requester.Id, entityId, rating); err != nil {
				logrus.Errorf("failed to insert query %s @ %s: %v", RatingSingular, reflect.FunctionName(), err)
				return fmt.Errorf("failed to set %s", RatingSingular)
			}
		}
	} else {
		logrus.Errorf("failed to query %s @ %s: %v", RatingSingular, reflect.FunctionName(), err)
		return fmt.Errorf("failed to set %s", RatingSingular)
	}

	return nil
}
