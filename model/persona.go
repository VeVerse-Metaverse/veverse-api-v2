package model

import (
	"context"
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"veverse-api/database"
	"veverse-api/reflect"
)

// Persona struct
type Persona struct {
	Entity

	Name          string      `json:"name,omitempty"`
	Type          string      `json:"type,omitempty"`
	Configuration interface{} `json:"configuration,omitempty"`
	UserId        *uuid.UUID  `json:"userId,omitempty"`
}

var (
	PersonaPlural = "Personas"
)

func IndexUserPersonas(ctx context.Context, id uuid.UUID, offset int64, limit int64) (personas []Persona, total int32, err error) {
	var (
		q    string
		db   *pgxpool.Pool
		row  pgx.Row
		rows pgx.Rows
	)

	db = database.DB

	q = `SELECT COUNT(*) FROM personas p WHERE p.user_id = $1`
	row = db.QueryRow(ctx, q, id)

	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", PersonaPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get %s", PersonaPlural)
	}

	if total == 0 {
		return nil, total, nil
	}

	q = `SELECT p.id, p.name, p.type, p.configuration FROM personas p WHERE p.user_id = $1 OFFSET $2 LIMIT $3`
	rows, err = db.Query(ctx, q, id, offset, limit)
	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", PersonaPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get %s", PersonaPlural)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexUserPersonasForAdmin")
	}()

	var p Persona
	for rows.Next() {
		err = rows.Scan(&p.Id, &p.Name, &p.Type, &p.Configuration)

		if err != nil {
			logrus.Errorf("failed to scan %s @ %s: %v", PersonaPlural, reflect.FunctionName(), err)
			return nil, -1, fmt.Errorf("failed to get %s", PersonaPlural)
		}

		personas = append(personas, p)
	}

	return personas, total, nil
}

func GetPersona(ctx context.Context, id uuid.UUID) (persona *Persona, err error) {
	var (
		q   string
		db  *pgxpool.Pool
		row pgx.Row
	)

	db = database.DB
	q = `SELECT p.id, p.user_id, p.name, p.type, p.configuration FROM personas p WHERE p.id = $1`
	row = db.QueryRow(ctx, q, id)

	persona = &Persona{}
	err = row.Scan(&persona.Id, &persona.UserId, &persona.Name, &persona.Type, &persona.Configuration)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", PersonaPlural, reflect.FunctionName(), err)
		return nil, fmt.Errorf("failed to get %s", PersonaPlural)
	}

	return persona, nil
}
