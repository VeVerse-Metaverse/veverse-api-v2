package model

import (
	"context"
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"time"
	"veverse-api/database"
	"veverse-api/reflect"
)

var (
	ServerSingular = "server"
)

type ServerMatchRequestMetadata struct {
	BuildId uuid.UUID `json:"buildId"`
	Host    string    `json:"hostname"`
}

type ServerStatusRequestMetadata struct {
	IdRequestMetadata
	Status  string `json:"status,omitempty"`
	Details string `json:"details,omitempty"`
}

var SupportedServerStatuses = map[string]bool{
	"online":   true, // Server is online and is sending heartbeats
	"offline":  true, // Server has quit
	"starting": true, // Server is starting
	"error":    true, // Server closed with an error, check the details field
}

// Server struct
type Server struct {
	Identifier
	ServerPlayer

	Name          *string    `json:"name,omitempty"`
	Public        *bool      `json:"public,omitempty"`
	Build         *string    `json:"build,omitempty"`
	Map           *string    `json:"map,omitempty"`
	Host          *string    `json:"host,omitempty"`
	Port          *int32     `json:"port,omitempty"`
	MaxPlayers    *int32     `json:"maxPlayers,omitempty"`
	GameMode      *string    `json:"gameMode,omitempty"`
	Status        *string    `json:"status,omitempty"`
	Details       *string    `json:"details,omitempty"`
	Platform      string     `json:"platform,omitempty"`
	Configuration string     `json:"configuration,omitempty"`
	Type          string     `json:"type,omitempty"`
	Release       *Release   `json:"release,omitempty"`
	Package       *Package   `json:"package,omitempty"`
	CreatedAt     *time.Time `json:"createdAt,omitempty"`
	UpdatedAt     *time.Time `json:"updatedAt,omitempty"`
}

type ServerPlayer struct {
	Identifier

	ServerId       *uuid.UUID `json:"serverId,omitempty"`
	UserId         *uuid.UUID `json:"userId,omitempty"`
	ConnectedAt    *time.Time `json:"connectedAt,omitempty"`
	DisconnectedAt *time.Time `json:"disconnectedAt,omitempty"`
	OnlinePlayers  *int32     `json:"onlinePlayers,omitempty"`
}

func GetServer(ctx context.Context, id uuid.UUID) (entity *Server, err error) {
	var (
		q   string
		db  *pgxpool.Pool
		row pgx.Row
	)

	db = database.DB
	q = `SELECT id, name, public, build, map, host, port, max_players, game_mode, status, details,
       created_at, updated_at
FROM servers
WHERE id = $1`

	row = db.QueryRow(ctx, q, id)
	entity = &Server{}
	err = row.Scan(
		&entity.Id, &entity.Name, &entity.Public, &entity.Build, &entity.Map, &entity.Host,
		&entity.Port, &entity.MaxPlayers, &entity.GameMode, &entity.Status, &entity.Details,
		&entity.CreatedAt, &entity.UpdatedAt,
	)

	if err != nil {
		if err.Error() != "no rows in result set" {
			logrus.Errorf("failed to scan %s @ %s: %v", ServerSingular, reflect.FunctionName(), err)
			return nil, fmt.Errorf("failed to get server")
		}
	}

	return entity, nil
}

// UpdateServerStatus Updates a Server status
func UpdateServerStatus(ctx context.Context, id uuid.UUID, status string, details string) (entity *Server, err error) {
	db := database.DB

	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %s", err.Error())
	}

	defer tx.Rollback(ctx)

	q := `UPDATE servers j SET status = $1::text, details = $2::uuid WHERE j.id = $3::uuid`

	_, err = tx.Exec(ctx, q, status /*$1*/, details /*$2*/, id /*$3*/)
	if err != nil {
		return nil, fmt.Errorf("failed to update %s @ %s: %v", jobSingular, reflect.FunctionName(), err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to commit query %s @ %s: %v", jobSingular, reflect.FunctionName(), err)
	}

	return entity, err
}

func MatchServer(ctx context.Context, spaceId uuid.UUID, buildId uuid.UUID, host string) (entity *Server, err error) {
	db := database.DB

	var (
		row pgx.Row
		q   string
	)

	q = `SELECT
    s.id, s.name, s.public, s.build, s.map, s.host, s.port, s.max_players, s.game_mode, s.status, details,
    s.created_at, s.updated_at, COUNT(sp.id) AS online_players
    FROM servers s
         LEFT JOIN server_players sp on s.id = sp.server_id
         WHERE
             space_id = $1::uuid AND
             (status = 'online' OR status='starting' OR status = 'created') AND
             updated_at >= now() - interval '60 days'
		 GROUP BY s.id, s.name, s.public, s.build, s.map, s.host, s.port, s.max_players, s.game_mode, s.status, details, s.created_at, s.updated_at
		 HAVING COUNT(sp.id) < s.max_players`

	row = db.QueryRow(ctx, q, spaceId)
	entity = &Server{}
	err = row.Scan(
		&entity.Id, &entity.Name, &entity.Public, &entity.Build, &entity.Map, &entity.Host,
		&entity.Port, &entity.MaxPlayers, &entity.GameMode, &entity.Status, &entity.Details,
		&entity.CreatedAt, &entity.UpdatedAt, &entity.ServerPlayer.OnlinePlayers,
	)

	if err != nil {
		if err.Error() != "no rows in result set" {
			logrus.Errorf("failed to scan %s @ %s: %v", ServerSingular, reflect.FunctionName(), err)
			return nil, fmt.Errorf("failed to get matching server")
		}
	}

	return entity, nil
}

func MatchServerWithRelease(ctx context.Context, spaceId uuid.UUID, releaseId uuid.UUID) (entity *Server) {
	db := database.DB

	q := `SELECT * FROM servers WHERE space_id = $1::uuid AND release_id = $2::uuid`

	err := db.QueryRow(ctx, q, spaceId, releaseId).Scan(&entity)
	if err != nil {
		return nil
	}

	return entity
}

func MatchServerWithReleaseWithHost(ctx context.Context, spaceId uuid.UUID, releaseId uuid.UUID, host string) (entity *Server) {
	db := database.DB

	q := `SELECT * FROM servers WHERE space_id = $1::uuid AND release_id = $2::uuid AND host = $3::text`

	err := db.QueryRow(ctx, q, spaceId, releaseId, host).Scan(&entity)
	if err != nil {
		return nil
	}

	return entity
}
