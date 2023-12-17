package handler

import (
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"github.com/jackc/pgtype"
	pgtypeuuid "github.com/jackc/pgtype/ext/gofrs-uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"time"
	"veverse-api/database"
	"veverse-api/helper"
	"veverse-api/model"
	"veverse-api/reflect"
	"veverse-api/validation"
)

type SwaggerUserBatch struct {
	Entities []model.User `json:"entities,omitempty"`
	Offset   int64        `json:"offset,omitempty"`
	Limit    int64        `json:"limit,omitempty"`
	Total    int64        `json:"total,omitempty"`
}

// IndexUsers Index users godoc
// @Summary      User pagination
// @Description  Indexing users with optional search query
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        query query string false "Query"
// @Param        offset query int false "Offset"
// @Param        limit query int false "Limit"
// @Success      200  {object}  SwaggerUserBatch
// @Failure      400  {object}  error
// @Failure      404  {object}  error
// @Failure      500  {object}  error
// @Router       /users [get]
func IndexUsers(c *fiber.Ctx) error {
	db := database.DB

	var users []model.User

	// Parse batch request metadata from the request
	m := model.BatchRequestMetadata{}
	err := c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	var offset int64 = 0
	if m.Offset > 0 {
		offset = m.Offset
	}

	var limit int64 = 20
	if m.Limit > 0 && m.Limit < 20 {
		limit = m.Limit
	}

	var query = ""
	if m.Query != "" {
		query = fmt.Sprintf("%%%s%%", m.Query)
	}

	// Query to request users with avatar image files and presence data
	qSelect := `SELECT u.id, u.name, u.description, u.is_active, u.is_admin, u.is_muted, u.is_banned, u.is_internal, u.allow_emails, u.experience, u.default_persona_id,
		uf.id, uf.url, uf.type, uf.mime,
		p.space_id, p.server_id, p.status, p.updated_at`
	qFrom := `FROM users u
		LEFT JOIN entities e ON u.id = e.id
		LEFT JOIN files uf ON u.id = uf.entity_id AND uf.type = 'image_avatar'
        LEFT JOIN presence p ON u.id = p.user_id AND p.updated_at > now() - interval '1h'`
	qWhere := `WHERE NOT u.is_internal AND NOT u.is_banned`

	var rows pgx.Rows

	if query == "" {
		q := fmt.Sprintf(`%s %s %s OFFSET $1 LIMIT $2`, qSelect, qFrom, qWhere)
		rows, err = db.Query(c.UserContext(), q, offset, limit)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}
	} else {
		qWhereQuery := " AND u.name ILIKE $3 OR description ILIKE $3"
		q := fmt.Sprintf(`%s %s %s %s OFFSET $1 LIMIT $2`, qSelect, qFrom, qWhere, qWhereQuery)
		rows, err = db.Query(c.UserContext(), q, offset, limit, query)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}
	}

	defer rows.Close()

	for rows.Next() {
		var user model.User

		var personaId pgtypeuuid.UUID
		var avatarId pgtypeuuid.UUID
		var avatarUrl *string
		var avatarType *string
		var avatarMime *string
		var presenceWorldId pgtypeuuid.UUID
		var presenceServerId pgtypeuuid.UUID
		var presenceStatus *string
		var presenceUpdatedAt *time.Time

		err = rows.Scan(&user.Id,
			&user.Name,
			&user.Description,
			&user.IsActive,
			&user.IsAdmin,
			&user.IsMuted,
			&user.IsBanned,
			&user.IsInternal,
			&user.AllowEmails,
			&user.Experience,
			&personaId,
			&avatarId,
			&avatarUrl,
			&avatarType,
			&avatarMime,
			&presenceWorldId,
			&presenceServerId,
			&presenceStatus,
			&presenceUpdatedAt)
		if err != nil {
			return err
		}

		user.UpdateComputedProperties()

		if avatarId.Status == pgtype.Present && avatarType != nil && avatarUrl != nil {
			file := model.File{
				Url:  *avatarUrl,
				Type: *avatarType,
				Mime: avatarMime,
			}
			file.Id = &avatarId.UUID
			user.Files = append(user.Files, file)
		}

		if personaId.Status == pgtype.Present {
			user.DefaultPersona = new(model.Persona)
			user.DefaultPersona.Id = &personaId.UUID
		}

		if presenceWorldId.Status == pgtype.Present && presenceServerId.Status == pgtype.Present && presenceStatus != nil {
			user.Presence = new(model.Presence)
			user.Presence.Status = presenceStatus
			user.Presence.WorldId = &presenceWorldId
			user.Presence.ServerId = &presenceServerId
			user.Presence.UpdatedAt = presenceUpdatedAt
		}

		users = append(users, user)
	}

	var row pgx.Row
	var total int64

	if query == "" {
		row = db.QueryRow(c.UserContext(), fmt.Sprintf("SELECT COUNT(*) %s %s", qFrom, qWhere))
	} else {
		qWhereQuery := " AND u.name ILIKE $1 OR description ILIKE $1"
		row = db.QueryRow(c.UserContext(), fmt.Sprintf("SELECT COUNT(*) %s %s %s", qFrom, qWhere, qWhereQuery), query)
	}

	err = row.Scan(&total)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": users, "offset": offset, "limit": limit, "total": total})
}

func GetUser(c *fiber.Ctx) error {
	db := database.DB

	// Parse batch request metadata from the request
	m := model.IdRequestMetadata{}
	err := c.ParamsParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	var id = m.Id
	if id.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid id", "data": nil})
	}

	// Query to request users with avatar image files and presence data
	q := `
SELECT u.id, u.name, u.description, u.is_active, u.is_admin, u.is_muted, u.is_banned, u.is_internal, u.allow_emails, u.experience, u.eth_address, u.default_persona_id,
		uf.id, uf.url, uf.type, uf.mime,
		p.space_id, p.server_id, p.status, p.updated_at 
FROM users u
		LEFT JOIN entities e ON u.id = e.id
		LEFT JOIN files uf ON u.id = uf.entity_id AND uf.type = 'image_avatar'
        LEFT JOIN presence p ON u.id = p.user_id AND p.updated_at > now() - interval '1h' 
WHERE u.id = $1`

	var row pgx.Row
	row = db.QueryRow(c.UserContext(), q, id)

	var user model.User
	var personaId pgtypeuuid.UUID
	var avatarId pgtypeuuid.UUID
	var avatarUrl *string
	var avatarType *string
	var avatarMime *string
	var presenceWorldId pgtypeuuid.UUID
	var presenceServerId pgtypeuuid.UUID
	var presenceStatus *string
	var presenceUpdatedAt *time.Time

	err = row.Scan(&user.Id,
		&user.Name,
		&user.Description,
		&user.IsActive,
		&user.IsAdmin,
		&user.IsMuted,
		&user.IsBanned,
		&user.IsInternal,
		&user.AllowEmails,
		&user.Experience,
		&user.EthAddress,
		&personaId,
		&avatarId,
		&avatarUrl,
		&avatarType,
		&avatarMime,
		&presenceWorldId,
		&presenceServerId,
		&presenceStatus,
		&presenceUpdatedAt)

	if err != nil {
		return err
	}

	user.UpdateComputedProperties()

	if avatarId.Status == pgtype.Present && avatarType != nil && avatarUrl != nil {
		file := model.File{
			Url:  *avatarUrl,
			Type: *avatarType,
			Mime: avatarMime,
		}
		file.Id = &avatarId.UUID
		user.Files = append(user.Files, file)
	}

	if personaId.Status == pgtype.Present {
		user.DefaultPersona = new(model.Persona)
		user.DefaultPersona.Id = &personaId.UUID
	}

	if presenceWorldId.Status == pgtype.Present && presenceServerId.Status == pgtype.Present && presenceStatus != nil {
		user.Presence = new(model.Presence)
		user.Presence.Status = presenceStatus
		user.Presence.WorldId = &presenceWorldId
		user.Presence.ServerId = &presenceServerId
		user.Presence.UpdatedAt = presenceUpdatedAt
	}

	if user.Id.IsNil() {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "not found", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": user})
}

func GetUserByEthAddress(c *fiber.Ctx) error {
	db := database.DB

	// Parse batch request metadata from the request
	m := struct {
		EthAddress string `params:"ethAddr" validate:"required"`
	}{}

	err := c.ParamsParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	err = validation.Validator.Struct(m)
	if err != nil {
		errors := model.GetErrors(err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "validation error", "data": errors})
	}

	//region Requester

	var (
		requester *sm.User
	)

	// Get requester
	requester, err = helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	// Query to request users with avatar image files and presence data
	q := `
SELECT u.id, u.name, u.description, u.is_active, u.is_admin, u.is_muted, u.is_banned, u.is_internal, u.allow_emails, u.experience, u.default_persona_id,
		uf.id, uf.url, uf.type, uf.mime,
		p.space_id, p.server_id, p.status, p.updated_at 
FROM users u
		LEFT JOIN entities e ON u.id = e.id
		LEFT JOIN files uf ON u.id = uf.entity_id AND uf.type = 'image_avatar'
        LEFT JOIN presence p ON u.id = p.user_id AND p.updated_at > now() - interval '1h' 
WHERE LOWER(u.eth_address) = LOWER($1)`

	var row pgx.Row
	row = db.QueryRow(c.UserContext(), q, m.EthAddress)

	var user model.User = model.User{}
	var personaId pgtypeuuid.UUID
	var avatarId pgtypeuuid.UUID
	var avatarUrl *string
	var avatarType *string
	var avatarMime *string
	var presenceWorldId pgtypeuuid.UUID
	var presenceServerId pgtypeuuid.UUID
	var presenceStatus *string
	var presenceUpdatedAt *time.Time

	err = row.Scan(&user.Id,
		&user.Name,
		&user.Description,
		&user.IsActive,
		&user.IsAdmin,
		&user.IsMuted,
		&user.IsBanned,
		&user.IsInternal,
		&user.AllowEmails,
		&user.Experience,
		&personaId,
		&avatarId,
		&avatarUrl,
		&avatarType,
		&avatarMime,
		&presenceWorldId,
		&presenceServerId,
		&presenceStatus,
		&presenceUpdatedAt)

	if err != nil {
		if err.Error() != "no rows in result set" {
			logrus.Errorf("failed to scan %s @ %s: %v", model.UserSingular, reflect.FunctionName(), err)
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": fmt.Errorf("failed to get user entity"), "data": nil})
		}
	}

	if user.Id == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "not found", "data": nil})
	}

	user.UpdateComputedProperties()

	if avatarId.Status == pgtype.Present && avatarType != nil && avatarUrl != nil {
		file := model.File{
			Url:  *avatarUrl,
			Type: *avatarType,
			Mime: avatarMime,
		}
		file.Id = &avatarId.UUID
		user.Files = append(user.Files, file)
	}

	if personaId.Status == pgtype.Present {
		user.DefaultPersona = new(model.Persona)
		user.DefaultPersona.Id = &personaId.UUID
	}

	if presenceWorldId.Status == pgtype.Present && presenceServerId.Status == pgtype.Present && presenceStatus != nil {
		user.Presence = new(model.Presence)
		user.Presence.Status = presenceStatus
		user.Presence.WorldId = &presenceWorldId
		user.Presence.ServerId = &presenceServerId
		user.Presence.UpdatedAt = presenceUpdatedAt
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": user})
}

func IndexUserAvatars(c *fiber.Ctx) error {
	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	// Parse batch request metadata from the request
	m := model.IdRequestMetadata{}
	err = c.ParamsParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var id = m.Id
	if id.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid id", "data": nil})
	}

	// Parse batch request metadata from the request
	filterMetadata := model.BatchRequestMetadata{}
	err = c.QueryParser(&filterMetadata)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var offset int64 = 0
	if filterMetadata.Offset > 0 {
		offset = filterMetadata.Offset
	}

	var limit int64 = 10
	if filterMetadata.Limit > 0 {
		limit = filterMetadata.Limit
	}

	var (
		avatars []model.File
		total   int64
	)

	avatars, total, err = model.IndexAvatars(c.UserContext(), id, offset, limit)
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fiber.Map{"entity": avatars}, "offset": offset, "limit": limit, "total": total})
}

func IndexUserFiles(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": nil, "offset": 0, "limit": 0, "total": 0})
}

func GetUserAvatarMesh(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": nil, "offset": 0, "limit": 0, "total": 0})
}

func GetMe(c *fiber.Ctx) error {
	db := database.DB

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Query to request users with avatar image files and presence data
	q := `
SELECT u.id, u.name, u.email,  u.description, u.is_active, u.is_admin, u.is_muted, u.is_banned, u.is_internal, u.allow_emails, u.experience, u.default_persona_id, u.eth_address, u.api_key,
		uf.id, uf.url, uf.type, uf.mime,
		p.space_id, p.server_id, p.status, p.updated_at 
FROM users u
		LEFT JOIN entities e ON u.id = e.id
		LEFT JOIN files uf ON u.id = uf.entity_id AND uf.type = 'image_avatar'
        LEFT JOIN presence p ON u.id = p.user_id AND p.updated_at > now() - interval '1h' 
WHERE u.id = $1`

	var row pgx.Row
	row = db.QueryRow(c.UserContext(), q, requester.Id)

	var (
		user              model.User
		personaId         pgtypeuuid.UUID
		avatarId          pgtypeuuid.UUID
		avatarUrl         *string
		avatarType        *string
		avatarMime        *string
		presenceWorldId   pgtypeuuid.UUID
		presenceServerId  pgtypeuuid.UUID
		presenceStatus    *string
		presenceUpdatedAt *time.Time
	)

	err = row.Scan(&user.Id,
		&user.Name,
		&user.Email,
		&user.Description,
		&user.IsActive,
		&user.IsAdmin,
		&user.IsMuted,
		&user.IsBanned,
		&user.IsInternal,
		&user.AllowEmails,
		&user.Experience,
		&personaId,
		&user.EthAddress,
		&user.ApiKey,
		&avatarId,
		&avatarUrl,
		&avatarType,
		&avatarMime,
		&presenceWorldId,
		&presenceServerId,
		&presenceStatus,
		&presenceUpdatedAt)

	if err != nil {
		return err
	}

	user.UpdateComputedProperties()

	if avatarId.Status == pgtype.Present && avatarType != nil && avatarUrl != nil {
		file := model.File{
			Url:  *avatarUrl,
			Type: *avatarType,
			Mime: avatarMime,
		}
		file.Id = &avatarId.UUID
		user.Files = append(user.Files, file)
	}

	if personaId.Status == pgtype.Present {
		user.DefaultPersona = new(model.Persona)
		user.DefaultPersona.Id = &personaId.UUID
	}

	if presenceWorldId.Status == pgtype.Present && presenceServerId.Status == pgtype.Present && presenceStatus != nil {
		user.Presence = new(model.Presence)
		user.Presence.Status = presenceStatus
		user.Presence.WorldId = &presenceWorldId
		user.Presence.ServerId = &presenceServerId
		user.Presence.UpdatedAt = presenceUpdatedAt
	}

	// Check if requester is banned
	if user.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": user})
}

func SetName(c *fiber.Ctx) error {
	db := database.DB

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Query to request users with avatar image files and presence data
	q := `update users set name = $1 where id = $2`

	_, err = db.Exec(c.UserContext(), q, c.Query("name"), requester.Id)

	if err != nil {
		if err.Error() == "duplicate key value violates unique constraint \"users_name_uindex\"" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "name already taken", "data": nil})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "internal error", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": true})
}

func IndexFriends(c *fiber.Ctx) error {
	//region Requester

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	//region Request metadata

	// Parse batch request metadata from the request
	requestMetadata := model.BatchRequestMetadata{}
	err = c.QueryParser(&requestMetadata)

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	// Parse batch params metadata from the request
	paramsMetadata := model.IdRequestMetadata{}
	err = c.ParamsParser(&paramsMetadata)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var (
		offset int64 = 0
		limit  int64 = 100
	)

	if requestMetadata.Offset > 0 {
		offset = requestMetadata.Offset
	}

	if requestMetadata.Limit > 0 && requestMetadata.Limit < 100 {
		limit = requestMetadata.Limit
	}

	var friends []model.User
	var total int64
	friends, total, err = model.IndexFriends(c.UserContext(), paramsMetadata.Id, offset, limit)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fiber.Map{"friends": friends, "offset": offset, "limit": limit, "total": total}})
}

func GetNonce(c *fiber.Ctx) error {
	metadata := model.NonceRequestMetadata{}
	if err := c.QueryParser(&metadata); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	err, nonce, _ := model.GetUserNonce(c.UserContext(), metadata.Address)
	if err != nil && err.Error() != "no rows in result set" {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "internal server error", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "data": fiber.Map{"nonce": nonce}})
}

func IsFollowing(c *fiber.Ctx) error {
	//region Requester

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}
	//endregion

	// Parse batch request metadata from the request
	m := model.IdRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	// Parse request metadata from the request
	requestParams := struct {
		FollowerId string `params:"followerId" validate:"required,uuid"`
		LeaderId   string `params:"leaderId" validate:"required,uuid"`
	}{}

	err = c.ParamsParser(&requestParams)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	err = validation.Validator.Struct(requestParams)
	if err != nil {
		errors := model.GetErrors(err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "validation error", "data": errors})
	}

	var (
		followerId uuid.UUID
		leaderId   uuid.UUID
		q          string
		row        pgx.Row
		db         *pgxpool.Pool
		total      int64
	)

	db = database.DB
	followerId = uuid.FromStringOrNil(requestParams.FollowerId)
	leaderId = uuid.FromStringOrNil(requestParams.LeaderId)

	q = `SELECT COUNT(f.id) FROM followers f WHERE f.leader_id = $1 AND f.follower_id = $2`
	row = db.QueryRow(c.UserContext(), q, leaderId, followerId)

	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", model.FollowerSingular, reflect.FunctionName(), err)
		return fmt.Errorf("failed to check user is following to another")
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fiber.Map{"isFollowing": total > 0}})
}

func IndexFollowers(c *fiber.Ctx) error {
	//region Requester

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	//region Request metadata

	// Parse batch request metadata from the request
	requestMetadata := model.BatchRequestMetadata{}
	err = c.QueryParser(&requestMetadata)

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	// Parse batch params metadata from the request
	paramsMetadata := model.IdRequestMetadata{}
	err = c.ParamsParser(&paramsMetadata)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var (
		offset int64 = 0
		limit  int64 = 100
	)

	if requestMetadata.Offset > 0 {
		offset = requestMetadata.Offset
	}

	if requestMetadata.Limit > 0 && requestMetadata.Limit < 100 {
		limit = requestMetadata.Limit
	}

	var followers []model.User
	var total int64
	followers, total, err = model.IndexFollowers(c.UserContext(), paramsMetadata.Id, offset, limit)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fiber.Map{"followers": followers, "offset": offset, "limit": limit, "total": total}})
}

func IndexLeaders(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": nil, "offset": 0, "limit": 0, "total": 0})
}

func Follow(c *fiber.Ctx) error {
	db := database.DB

	//region Requester
	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}
	//endregion

	// Parse batch request metadata from the request
	m := model.IdRequestMetadata{}
	err = c.ParamsParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var (
		q     string
		total int64
		row   pgx.Row
	)

	q = `SELECT COUNT(f.id) FROM followers f WHERE f.leader_id = $1 AND f.follower_id = $2`
	row = db.QueryRow(c.UserContext(), q, m.Id, requester.Id)

	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", model.FollowerSingular, reflect.FunctionName(), err)
		return fmt.Errorf("failed to follow the user")
	}

	if total == 0 {
		var id uuid.UUID
		id, err = uuid.NewV4()
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "failed to generate uuid", "data": nil})
		}

		q = `INSERT INTO followers (id, follower_id, leader_id, created_at, updated_at) VALUES ($1, $2, $3, now(), null)`

		if _, err = db.Query(c.UserContext(), q, id, requester.Id, m.Id); err != nil {
			logrus.Errorf("failed to follow the user %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "error",
				"message": "failed to follow the user",
				"data":    nil,
			})
		}
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil, "data": nil})
}

func Unfollow(c *fiber.Ctx) error {
	db := database.DB

	//region Requester
	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}
	//endregion

	// Parse batch request metadata from the request
	m := model.IdRequestMetadata{}
	err = c.ParamsParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var (
		q     string
		total int64
		row   pgx.Row
	)

	q = `SELECT COUNT(f.id) FROM followers f WHERE f.leader_id = $1 AND f.follower_id = $2`
	row = db.QueryRow(c.UserContext(), q, m.Id, requester.Id)

	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", model.FollowerSingular, reflect.FunctionName(), err)
		return fmt.Errorf("failed to unfollow the user")
	}

	if total > 0 {
		q = `DELETE FROM followers f WHERE f.leader_id = $1 AND f.follower_id = $2`

		if _, err = db.Query(c.UserContext(), q, m.Id, requester.Id); err != nil {
			logrus.Errorf("failed to unfollow the user %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "error",
				"message": "failed to unfollow the user",
				"data":    nil,
			})
		}
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil, "data": nil})
}

func IndexUserPersonas(c *fiber.Ctx) error {
	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	// Parse batch request metadata from the request
	m := model.IdRequestMetadata{}
	err = c.ParamsParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var id = m.Id
	if id.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid id", "data": nil})
	}

	// Parse batch request metadata from the request
	filterMetadata := model.BatchRequestMetadata{}
	err = c.QueryParser(&filterMetadata)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var offset int64 = 0
	if filterMetadata.Offset > 0 {
		offset = filterMetadata.Offset
	}

	var limit int64 = 100
	if filterMetadata.Limit > 0 {
		limit = filterMetadata.Limit
	}

	var personas []model.Persona
	var total int32
	personas, total, err = model.IndexUserPersonas(c.UserContext(), id, offset, limit)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": personas, "offset": offset, "limit": limit, "total": total})
}

func GetUserPersona(c *fiber.Ctx) error {
	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	// Parse batch request metadata from the request
	m := model.IdRequestMetadata{}
	err = c.ParamsParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var id = m.Id
	if id.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid id", "data": nil})
	}

	var persona *model.Persona
	persona, err = model.GetPersona(c.UserContext(), id)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": persona})
}

func IndexUsersForManager(c *fiber.Ctx) error {
	requester, err := helper.GetRequester(c)
	if err != nil || requester == nil {
		logrus.Errorf("IndexUsers: failed to get requester %v", err)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		logrus.Errorf("IndexUsers: requester is banned %v", err)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	m := sm.IndexUserRequest{}
	err = c.QueryParser(&m)
	if err != nil {
		logrus.Errorf("IndexUsers: failed to parse query %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	users, err := sm.IndexUser(c.UserContext(), requester, m)
	if err != nil {
		logrus.Errorf("IndexUsers: failed to index users %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "failed to get users", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil, "data": users})
}
