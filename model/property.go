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
	queryHelper "veverse-api/helper/query"
	"veverse-api/reflect"
)

// Custom entity property trait
type Property struct {
	EntityTrait

	Type  string `json:"type"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

type InsertProperty struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

var (
	propertySingular = "property"
	propertyPlural   = "properties"
)

func UpsertProperties(ctx context.Context, values []interface{}) (err error) {
	query := queryHelper.GetBulkInsertSQL("properties", []string{"entity_id", "type", "name", "value"}, len(values))
	query += "ON CONFLICT (entity_id, name) DO UPDATE SET type=excluded.type, value=excluded.value"

	db := database.DB
	_, err = db.Exec(ctx, query, values...)

	return err
}

func GetPropertiesForAdmin(ctx context.Context, entityId uuid.UUID, offset int64, limit int64) (properties []Property, total int32, err error) {

	var (
		q    string
		row  pgx.Row
		rows pgx.Rows
		db   *pgxpool.Pool
	)

	q = `SELECT COUNT(p.entity_id) FROM properties p LEFT JOIN entities e on e.id = p.entity_id WHERE e.id = $1`
	db = database.DB
	row = db.QueryRow(ctx, q, entityId)

	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", propertyPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get entity properties")
	}

	q = `SELECT p.name, p.type, p.value
FROM properties p
    LEFT JOIN entities e on e.id = p.entity_id
WHERE e.id = $1 OFFSET $2 LIMIT $3`

	rows, err = db.Query(ctx, q, entityId, offset, limit)
	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", propertyPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get entity properties")
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetPropertiesForAdmin")
	}()
	for rows.Next() {
		var property Property
		err = rows.Scan(&property.Name, &property.Type, &property.Value)
		if err != nil {
			logrus.Errorf("failed to scan %s @ %s: %v", propertyPlural, reflect.FunctionName(), err)
			return nil, -1, fmt.Errorf("failed to get entity properties")
		}

		properties = append(properties, property)
	}

	return properties, total, nil
}

func GetPropertiesForRequester(ctx context.Context, requester *model.User, entityId uuid.UUID, offset int64, limit int64) (properties []Property, total int32, err error) {

	var (
		q    string
		row  pgx.Row
		rows pgx.Rows
		db   *pgxpool.Pool
	)

	q = `SELECT COUNT(p.entity_id)
FROM properties p 
    LEFT JOIN entities e on e.id = p.entity_id
	LEFT JOIN accessibles a on e.id = a.entity_id
WHERE e.id = $1 AND a.user_id = $2`

	db = database.DB

	row = db.QueryRow(ctx, q, entityId, requester.Id)
	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", propertyPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get entity properties")
	}

	q = `SELECT p.name, p.type, p.value
FROM properties p
	LEFT JOIN  entities e on e.id = p.entity_id
	LEFT JOIN accessibles a on e.id = a.entity_id
WHERE e.id = $1 AND a.user_id = $2 OFFSET $3 LIMIT $4`

	rows, err = db.Query(ctx, q, entityId, requester.Id, offset, limit)
	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", propertyPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get entity properties")
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetPropertiesForRequester")
	}()
	for rows.Next() {
		var property Property
		err = rows.Scan(&property.Name, &property.Type, &property.Value)
		if err != nil {
			logrus.Errorf("failed to scan %s @ %s: %v", propertyPlural, reflect.FunctionName(), err)
			return nil, -1, fmt.Errorf("failed to get entity properties")
		}

		properties = append(properties, property)
	}

	return properties, total, nil
}
