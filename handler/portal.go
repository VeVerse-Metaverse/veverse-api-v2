package handler

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"veverse-api/helper"
	"veverse-api/model"
)

// IndexPortals godoc
// @Summary      Index Portals
// @Description  Fetch portal in batches
// @Tags         portals
// @Accept       json
// @Produce      json
// @Security	 Bearer
// @Param        spaceId query string false "Specify to filter by the owning space ID"
// @Param        platform query string false "Specify to attach the pak file"
// @Param        deployment query string false "Specify to attach the pak file"
// @Param        query query string false "Specify to filter by the portal, destination portal, destination space or destination metaverse name"
// @Param        offset query int false "Pagination offset, default 0"
// @Param        offset query int false "Pagination limit, default 100"
// @Success      200  {object}  []model.Portal
// @Failure      400  {object}  error
// @Failure      404  {object}  error
// @Failure      500  {object}  error
// @Router       /portals [get]
func IndexPortals(c *fiber.Ctx) error {
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
	m := model.PortalBatchRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var (
		offset     int64 = 0
		limit      int64 = 100
		query            = ""
		platform         = ""
		deployment       = ""
		worldId    uuid.UUID
	)

	if m.Offset > 0 {
		offset = m.Offset
	}

	if m.Limit > 0 && m.Limit < 100 {
		limit = m.Limit
	}

	if m.WorldId != nil {
		worldId = *m.WorldId
	}

	if m.Query != "" {
		query = fmt.Sprintf("%%%s%%", m.Query)
	}

	if model.SupportedPlatform[m.Platform] {
		platform = m.Platform
	}

	if model.SupportedDeployment[m.Deployment] {
		deployment = m.Deployment
	}

	withPak := deployment != "" && platform != ""

	//endregion

	var (
		entities []model.Portal
		total    int64
	)

	if requester.IsAdmin || requester.IsInternal {
		if query == "" {
			if withPak {
				if worldId.IsNil() {
					entities, total, err = model.IndexPortalsForAdminWithPak(c.UserContext(), platform, deployment, offset, limit)
				} else {
					entities, total, err = model.IndexPortalsForAdminForWorldWithPak(c.UserContext(), worldId, platform, deployment, offset, limit)
				}
			} else {
				if worldId.IsNil() {
					entities, total, err = model.IndexPortalsForAdmin(c.UserContext(), offset, limit)
				} else {
					entities, total, err = model.IndexPortalsForAdminForWorld(c.UserContext(), worldId, offset, limit)
				}
			}
		} else {
			if withPak {
				if worldId.IsNil() {
					entities, total, err = model.IndexPortalsForAdminWithQueryWithPak(c.UserContext(), platform, deployment, query, offset, limit)
				} else {
					entities, total, err = model.IndexPortalsForAdminForWorldWithQueryWithPak(c.UserContext(), worldId, platform, deployment, query, offset, limit)
				}
			} else {
				if worldId.IsNil() {
					entities, total, err = model.IndexPortalsForAdminWithQuery(c.UserContext(), query, offset, limit)
				} else {
					entities, total, err = model.IndexPortalsForAdminForWorldWithQuery(c.UserContext(), worldId, query, offset, limit)
				}
			}
		}
	} else {
		if query == "" {
			if withPak {
				if worldId.IsNil() {
					entities, total, err = model.IndexPortalsForRequesterWithPak(c.UserContext(), requester, platform, deployment, offset, limit)
				} else {
					entities, total, err = model.IndexPortalsForRequesterForWorldWithPak(c.UserContext(), requester, worldId, platform, deployment, offset, limit)
				}
			} else {
				if worldId.IsNil() {
					entities, total, err = model.IndexPortalsForRequester(c.UserContext(), requester, offset, limit)
				} else {
					entities, total, err = model.IndexPortalsForRequesterForWorld(c.UserContext(), requester, worldId, offset, limit)
				}
			}
		} else {
			if withPak {
				if worldId.IsNil() {
					entities, total, err = model.IndexPortalsForRequesterWithQueryWithPak(c.UserContext(), requester, platform, deployment, query, offset, limit)
				} else {
					entities, total, err = model.IndexPortalsForRequesterForWorldWithQueryWithPak(c.UserContext(), requester, worldId, platform, deployment, query, offset, limit)
				}
			} else {
				if worldId.IsNil() {
					entities, total, err = model.IndexPortalsForRequesterWithQuery(c.UserContext(), requester, query, offset, limit)
				} else {
					entities, total, err = model.IndexPortalsForRequesterForWorldWithQuery(c.UserContext(), requester, worldId, query, offset, limit)
				}
			}
		}
	}

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fiber.Map{"entities": entities, "offset": offset, "limit": limit, "total": total}})
}

// GetPortal godoc
// @Summary      Get Portal
// @Description  Fetch a single portal
// @Tags         portals
// @Accept       json
// @Produce      json
// @Security	 Bearer
// @Param        id query string false "ID"
// @Param        platform query string false "Specify to attach the pak file"
// @Param        deployment query string false "Specify to attach the pak file"
// @Success      200  {object}  model.Portal
// @Failure      400  {object}  error
// @Failure      404  {object}  error
// @Failure      500  {object}  error
// @Router       /portals/:id [get]
func GetPortal(c *fiber.Ctx) error {
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
	m := model.PortalRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	m.Id = uuid.FromStringOrNil(c.Params("id"))

	var (
		id         uuid.UUID
		platform   = ""
		deployment = ""
	)

	if !m.Id.IsNil() {
		id = m.Id
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no id", "data": nil})
	}

	if model.SupportedPlatform[m.Platform] {
		platform = m.Platform
	}

	if model.SupportedDeployment[m.Deployment] {
		deployment = m.Deployment
	}

	withPak := deployment != "" && platform != ""
	//endregion

	var (
		entity *model.Portal
	)

	if requester.IsAdmin || requester.IsInternal {
		if withPak {
			entity, err = model.GetPortalForAdminWithPak(c.UserContext(), id, platform, deployment)
		} else {
			entity, err = model.GetPortalForAdmin(c.UserContext(), id)
		}
	} else {
		if withPak {
			entity, err = model.GetPortalForRequesterWithPak(c.UserContext(), requester, id, platform, deployment)
		} else {
			entity, err = model.GetPortalForRequester(c.UserContext(), requester, id)
		}
	}

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if entity == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "not found", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": entity})
}

func CreatePortal(c *fiber.Ctx) error {
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
	m := model.PortalCreateMetadata{}
	err = c.BodyParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if m.Public == nil {
		var public = true
		m.Public = &public
	}

	if m.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no name", "data": nil})
	}

	if m.WorldId.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no world", "data": nil})
	}

	//endregion

	var entity *model.Portal
	entity, err = model.CreatePortalForRequester(c.UserContext(), requester, m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil, "data": entity})
}

func UpdatePortal(c *fiber.Ctx) error {
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
	id := uuid.FromStringOrNil(c.Params("id"))

	if id.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no id", "data": nil})
	}

	// Parse batch request metadata from the request
	m := model.PortalUpdateMetadata{}
	err = c.BodyParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	//endregion

	var entity *model.Portal
	entity, err = model.UpdatePortalForRequester(c.UserContext(), requester, id, m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil, "data": entity})
}
