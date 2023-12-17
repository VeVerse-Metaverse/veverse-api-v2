package model

import (
	"context"
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"
	"veverse-api/database"
	"veverse-api/reflect"
)

var (
	PSInstanceSingular = "PixelStreamingInstance"
	PSInstancePlural   = "PixelStreamingInstances"

	RegionSingular = "Region"
	RegionPlural   = "Regions"
)

type Region struct {
	Entity
	Name string `json:"name,omitempty"`
}

type PixelStreamingSessionRequestMetadata struct {
	AppId   string  `json:"appId" validate:"required"`
	WorldId *string `json:"worldId,omitempty"`
}

type PixelStreamingSessionUpdateMetadata struct {
	AppId   string `json:"appId" validate:"required,uuid4"`
	WorldId string `json:"worldId,omitempty"`
	Status  string `json:"status" validate:"required,lowercase,oneof='pending' 'starting' 'running' 'closed'"`
}

type PixelStreamingInstanceUpdateMetadata struct {
	InstanceId string `json:"instanceId" validate:"required"`
	Status     string `json:"status" validate:"required,lowercase,oneof='pending' 'free' 'occupied' 'stopped' 'closed'"`
}

func GetRegions(ctx context.Context) (regions map[string]*uuid.UUID, err error) {
	db := database.DB

	q := `SELECT id, name FROM region`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q)

	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", RegionPlural, reflect.FunctionName(), err)
		return nil, fmt.Errorf("failed to get %s", RegionPlural)
	}

	regions = make(map[string]*uuid.UUID)
	for rows.Next() {
		var region Region
		err = rows.Scan(&region.Id, &region.Name)

		regions[region.Name] = region.Id
	}

	return regions, nil
}

func GetGameSessions(ctx context.Context) (regions map[string]*uuid.UUID, err error) {
	db := database.DB

	q := `SELECT id, release_id, region_id, host, port, status, instance_id FROM pixel_streaming_instance`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q)

	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", RegionPlural, reflect.FunctionName(), err)
		return nil, fmt.Errorf("failed to get %s", RegionPlural)
	}

	regions = make(map[string]*uuid.UUID)
	for rows.Next() {
		var region Region
		err = rows.Scan(&region.Id, &region.Name)

		regions[region.Name] = region.Id
	}

	return regions, nil
}
