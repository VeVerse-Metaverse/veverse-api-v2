package handler

import (
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"veverse-api/helper"
	"veverse-api/model"
)

// GetWorld godoc
// @Summary      Get World
// @Description  Fetch a single world
// @Tags         worlds
// @Accept       json
// @Produce      json
// @Security	 Bearer
// @Param        id query string false "ID"
// @Param        platform query string false "Specify to attach the pak file"
// @Param        deployment query string false "Specify to attach the pak file"
// @Success      200  {object}  model.World
// @Failure      400  {object}  error
// @Failure      404  {object}  error
// @Failure      500  {object}  error
// @Router       /worlds/:id [get]
func GetObject(c *fiber.Ctx) error {
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
	m := model.ObjectRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	m.Id = uuid.FromStringOrNil(c.Params("id"))

	var (
		id uuid.UUID
	)

	if !m.Id.IsNil() {
		id = m.Id
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no id", "data": nil})
	}

	//endregion

	var (
		entity *model.Object
	)

	if requester.IsAdmin || requester.IsInternal {
		entity, err = model.GetObjectForAdmin(c.UserContext(), id)
	} else {
		entity, err = model.GetObjectForRequester(c.UserContext(), requester, id)
	}

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if entity == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "not found", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": entity})
}

func IndexObjects(c *fiber.Ctx) (err error) {
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

	m := model.BatchRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var (
		offset  int64 = 0
		limit   int64 = 100
		total   int32
		objects []model.Object
	)

	if m.Offset > 0 {
		offset = m.Offset
	}

	if m.Limit > 0 && m.Limit < 100 {
		limit = m.Limit
	}

	if requester.IsAdmin || requester.IsInternal {
		objects, total, err = model.GetObjectsForAdmin(c.UserContext(), offset, limit)
	} else {
		objects, total, err = model.GetObjectsForRequester(c.UserContext(), requester, offset, limit)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "failed to fetch objects", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fiber.Map{"offset": offset, "limit": limit, "total": total, "entities": objects}})
}

func IndexArtObjects(c *fiber.Ctx) (err error) {
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

	m := model.BatchRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var (
		offset  int64 = 0
		limit   int64 = 100
		query         = ""
		total   int32
		objects []model.ArtObject
	)

	if m.Offset > 0 {
		offset = m.Offset
	}

	if m.Limit > 0 && m.Limit < 100 {
		limit = m.Limit
	}

	//if m.Query != "" {
	query = fmt.Sprintf("%%%s%%", m.Query)
	//}

	if requester.IsAdmin || requester.IsInternal {
		objects, total, err = model.GetArtObjectsForAdmin(c.UserContext(), requester, offset, limit, query)
	} else {
		objects, total, err = model.GetArtObjectsForRequester(c.UserContext(), requester, offset, limit, query)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "failed to fetch objects", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fiber.Map{"offset": offset, "limit": limit, "total": total, "entities": objects}})
}

func GetArtObject(c *fiber.Ctx) (err error) {
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

	// Parse batch request metadata from the request
	m := model.ObjectRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	m.Id = uuid.FromStringOrNil(c.Params("id"))

	var object *model.ArtObject

	if requester.IsAdmin || requester.IsInternal {
		object, err = model.GetArtObjectForAdmin(c.UserContext(), requester, m.Id)
	} else {
		object, err = model.GetArtObjectForRequester(c.UserContext(), requester, m.Id)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "failed to get object", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": object})
}

//func GetObjectTypes(с *fiber.Ctx) (err error) {
//
//}
