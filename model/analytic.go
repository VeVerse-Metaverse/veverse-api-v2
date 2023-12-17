package model

import (
	"context"
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"encoding/json"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	gofrsUUID "github.com/gofrs/uuid"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"veverse-api/database"
	"veverse-api/reflect"
)

type AnalyticRequestMetadata struct {
	ContextType string `json:"contextType,omitempty"`
	Event       string `json:"event,omitempty"`
	AppId       string `json:"appId"`
	Platform    string `json:"platform"`   // SupportedPlatform (OS) of the pak file (Win64, Mac, Linux, IOS, Android)
	Deployment  string `json:"deployment"` // SupportedDeployment for the pak file (Server or Client)

	BatchRequestMetadata
}

type AnalyticEventRequest struct {
	AppId             uuid.UUID `json:"appId" query:"app-id" validate:"required"`
	ContextEntityId   uuid.UUID `json:"contextEntityId" query:"context-entity-id" validate:"required"`
	ContextEntityType string    `json:"contextEntityType" query:"context-entity-type" validate:"required"`
	UserId            uuid.UUID `json:"userId" query:"user-id" validate:"required"`
	Platform          string    `json:"platform" query:"platform" validate:"required"`
	Deployment        string    `json:"deployment" query:"deployment" validate:"required"`
	Configuration     string    `json:"configuration" query:"configuration" validate:"required"`
	Event             string    `json:"event" query:"event" validate:"required"`
	Payload           any       `json:"data" query:"data" validate:"required"`
}

//type AnalyticEventBatch Batch[AnalyticEvent]

var (
	AnalyticSingular = "analytic"
	AnalyticPlural   = "analytics"
)

func IndexAnalyticsForApp(ctx context.Context, appId gofrsUUID.UUID, platform string, deployment string, offset int64, limit int64) (analytics []sm.AnalyticEvent, total uint64, err error) {
	clickhouse := database.Clickhouse

	q := `SELECT count(id) FROM events WHERE appId = $1`
	row := clickhouse.QueryRow(ctx, q, appId)

	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", AnalyticPlural, reflect.FunctionName(), err)
		return nil, 0, fmt.Errorf("failed to get entity analytics")
	}

	q = `SELECT
    id, appId, contextEntityId, contextEntityType, userId, platform, deployment, configuration, event, timestamp, payload
FROM events
WHERE contextEntityId = $1
LIMIT $2 OFFSET $3`

	var (
		rows driver.Rows
	)

	rows, err = clickhouse.Query(ctx, q, appId, limit, offset)

	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", AnalyticPlural, reflect.FunctionName(), err)
		return nil, 0, fmt.Errorf("failed to get entity analytics")
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexAnalytics")
	}()

	for rows.Next() {

		var (
			analytic = &sm.AnalyticEvent{}
		)

		err = rows.Scan(
			&analytic.Id,
			&analytic.AppId,
			&analytic.ContextEntityId,
			&analytic.ContextEntityType,
			&analytic.UserId,
			&analytic.Platform,
			&analytic.Deployment,
			&analytic.Configuration,
			&analytic.Event,
			&analytic.Timestamp,
			&analytic.Payload,
		)

		if err != nil {
			logrus.Errorf("failed to scan %s @ %s: %v", AnalyticPlural, reflect.FunctionName(), err)
			return nil, 0, fmt.Errorf("failed to get entity analytics")
		}

		analytics = append(analytics, *analytic)
	}

	return analytics, total, nil
}

func IndexAnalyticsForEntity(ctx context.Context, entityId gofrsUUID.UUID, platform string, deployment string, offset int64, limit int64) (analytics []sm.AnalyticEvent, total int64, err error) {
	clickhouse := database.Clickhouse

	q := `SELECT count(id) FROM events WHERE contextEntityId = $1 AND platform = $2 AND deployment = $3`
	row := clickhouse.QueryRow(ctx, q, entityId, platform, deployment)

	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", AnalyticPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get entity analytics")
	}

	q = `SELECT
    id, appId, contextEntityId, contextEntityType, userId, platform, deployment, configuration, event, timestamp, payload
FROM events
WHERE contextEntityId = $1 AND platform = $2 AND deployment = $3
LIMIT $4 OFFSET $5`

	var (
		rows driver.Rows
	)

	rows, err = clickhouse.Query(ctx, q, entityId, platform, deployment, limit, offset)

	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", AnalyticPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get entity analytics")
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexAnalyticsForEntity")
	}()

	for rows.Next() {

		var (
			analytic = &sm.AnalyticEvent{}
		)

		err = rows.Scan(
			&analytic.Id,
			&analytic.AppId,
			&analytic.ContextEntityId,
			&analytic.ContextEntityType,
			&analytic.UserId,
			&analytic.Platform,
			&analytic.Deployment,
			&analytic.Configuration,
			&analytic.Event,
			&analytic.Timestamp,
			&analytic.Payload,
		)

		if err != nil {
			logrus.Errorf("failed to scan %s @ %s: %v", AnalyticPlural, reflect.FunctionName(), err)
			return nil, -1, fmt.Errorf("failed to get entity analytics")
		}

		analytics = append(analytics, *analytic)
	}

	return analytics, total, nil
}

func convertUuid(u gofrsUUID.UUID) uuid.UUID {
	return uuid.UUID{
		u[0],
		u[1],
		u[2],
		u[3],
		u[4],
		u[5],
		u[6],
		u[7],
		u[8],
		u[9],
		u[10],
		u[11],
		u[12],
		u[13],
		u[14],
		u[15],
	}
}

func ReportEvent(ctx context.Context, requester *sm.User, event AnalyticEventRequest) error {
	if requester == nil {
		return fmt.Errorf("requester is nil")
	}

	clickhouse := database.Clickhouse

	var userId uuid.UUID
	if requester.IsInternal {
		userId = event.UserId
	} else {
		userId = convertUuid(requester.Id)
	}

	bytes, err := json.Marshal(event.Payload)
	if err != nil {
		logrus.Errorf("failed to marshal payload %s @ %s: %v", AnalyticSingular, reflect.FunctionName(), err)
		return fmt.Errorf("failed to report event")
	}

	q := `INSERT INTO events (appId, contextEntityId, contextEntityType, userId, platform, deployment, configuration, event, payload)
	VALUES	($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	err = clickhouse.Exec(ctx, q,
		event.AppId,
		event.ContextEntityId,
		event.ContextEntityType,
		userId,
		event.Platform,
		event.Deployment,
		event.Configuration,
		event.Event,
		string(bytes),
	)

	if err != nil {
		logrus.Errorf("failed to insert %s @ %s: %v", AnalyticSingular, reflect.FunctionName(), err)
		return fmt.Errorf("failed to report event")
	}

	return nil
}
