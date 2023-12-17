package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"veverse-api/helper"
	"veverse-api/model"
)

func MatchServer(c *fiber.Ctx) error {
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

	// Parse batch request metadata from the request
	m := model.IdRequestMetadata{}
	err = c.ParamsParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var id = m.Id
	if id.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no space id", "data": nil})
	}

	matchMetadata := model.ServerMatchRequestMetadata{}
	err = c.QueryParser(&matchMetadata)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var (
		entity *model.Server
	)

	entity, err = model.MatchServer(c.UserContext(), id, matchMetadata.BuildId, matchMetadata.Host)

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil, "data": entity})
}

func GetServer(c *fiber.Ctx) error {
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
	m := model.IdRequestMetadata{}
	err = c.ParamsParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var id = m.Id
	if id.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid id", "data": nil})
	}

	var server *model.Server

	server, err = model.GetServer(c.UserContext(), id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil, "data": server})
}

func UpdateServerStatus(c *fiber.Ctx) error {
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

	// Parse request metadata from the request
	m := model.ServerStatusRequestMetadata{}
	err = c.BodyParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	// Server ID
	m.Id = uuid.FromStringOrNil(c.Params("id"))

	var (
		id uuid.UUID // server id
	)

	if !m.Id.IsNil() {
		id = m.Id
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no id", "data": nil})
	}

	if m.Status == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no status", "data": nil})
	}

	if !model.SupportedServerStatuses[m.Status] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid server status", "data": nil})
	}

	//endregion

	var (
		entity *model.Server
	)

	if requester.IsAdmin || requester.IsInternal {
		entity, err = model.UpdateServerStatus(c.UserContext(), id, m.Status, m.Details)
	} else {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "forbidden", "data": nil})
	}

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if entity == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "not found", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": entity})
}
