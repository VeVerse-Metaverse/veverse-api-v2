package handler

import (
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"
	"veverse-api/database"
	"veverse-api/helper"
	"veverse-api/model"
)

func IndexWorlds(c *fiber.Ctx) error {
	var (
		status      = fiber.StatusOK
		requesterId = uuid.Nil
	)
	defer func() {
		err := database.ReportRequestEvent(c, requesterId, status)
		if err != nil {
			logrus.Errorf("failed to report request: %v", err)
		}
	}()

	//region Requester

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		status = fiber.StatusUnauthorized
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	requesterId = requester.Id

	// Check if requester is banned
	if requester.IsBanned {
		status = fiber.StatusForbidden
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	//region Request metadata

	// Parse batch request metadata from the request
	m := model.WorldBatchRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var (
		offset     int64 = 0
		limit      int64 = 100
		query            = ""
		platform         = ""
		deployment       = ""
		packageId  uuid.UUID
	)

	if m.Offset > 0 {
		offset = m.Offset
	}

	if m.Limit > 0 && m.Limit < 100 {
		limit = m.Limit
	}

	if m.PackageId != nil {
		packageId = uuid.FromStringOrNil(*m.PackageId)
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

	//endregion

	var (
		entities []model.World
		total    int64
	)

	if requester.IsAdmin || requester.IsInternal {
		if query == "" {
			if packageId.IsNil() {
				entities, total, err = model.IndexWorldsForAdminWithPak(c.UserContext(), requester, offset, limit, platform, deployment)
			} else {
				entities, total, err = model.IndexWorldsForAdminForPackageWithPak(c.UserContext(), requester, packageId, offset, limit, platform, deployment)
			}
		} else {
			if packageId.IsNil() {
				entities, total, err = model.IndexWorldsForAdminWithQueryWithPak(c.UserContext(), requester, offset, limit, query, platform, deployment)
			} else {
				entities, total, err = model.IndexWorldsForAdminForPackageWithQueryWithPak(c.UserContext(), requester, packageId, offset, limit, query, platform, deployment)
			}
		}
	} else {
		if query == "" {
			if packageId.IsNil() {
				entities, total, err = model.IndexWorldsForRequesterWithPak(c.UserContext(), requester, offset, limit, platform, deployment)
			} else {
				entities, total, err = model.IndexWorldsForRequesterForPackageWithPak(c.UserContext(), requester, packageId, offset, limit, platform, deployment)
			}
		} else {
			if packageId.IsNil() {
				entities, total, err = model.IndexWorldsForRequesterWithQueryWithPak(c.UserContext(), requester, offset, limit, query, platform, deployment)
			} else {
				entities, total, err = model.IndexWorldsForRequesterForPackageWithQueryWithPak(c.UserContext(), requester, packageId, offset, limit, query, platform, deployment)
			}
		}
	}

	if err != nil {
		status = fiber.StatusInternalServerError
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(status).JSON(fiber.Map{"data": fiber.Map{"entities": entities, "offset": offset, "limit": limit, "total": total}})
}

func IndexWorldsV2(c *fiber.Ctx) error {
	requester, err := helper.GetRequester(c)
	if err != nil || requester == nil {
		logrus.Errorf("IndexLaunchers: %s", err.Error())
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to get requester",
			"data":    nil,
		})
	}

	if requester.IsBanned {
		logrus.Errorf("IndexLaunchers: %s", "user is banned")
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  "error",
			"message": "banned",
			"data":    nil,
		})
	}

	m := sm.IndexWorldRequest{}
	if err = c.BodyParser(&m); err != nil {
		logrus.Errorf("IndexLaunchers: %s", err.Error())
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to parse query",
			"data":    nil,
		})
	}

	worlds, err := sm.IndexWorld(c.UserContext(), requester, m)
	if err != nil {
		logrus.Errorf("IndexLaunchers: %s", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to index worlds",
			"data":    nil,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  "success",
		"message": "worlds indexed",
		"data":    worlds,
	})
}

func GetWorldV2(c *fiber.Ctx) error {
	requester, err := helper.GetRequester(c)
	if err != nil || requester == nil {
		logrus.Errorf("GetWorld: %s", err.Error())
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to get requester",
			"data":    nil,
		})
	}

	if requester.IsBanned {
		logrus.Errorf("GetWorld: %s", "user is banned")
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  "error",
			"message": "banned",
			"data":    nil,
		})
	}

	worldId := c.Params("id")

	m := sm.GetWorldRequest{}
	if err = c.BodyParser(&m); err != nil {
		logrus.Errorf("IndexLaunchers: %s", err.Error())
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to parse query",
			"data":    nil,
		})
	}

	// if world id is missing, use the one from the url
	if m.Id.IsNil() {
		m.Id = uuid.FromStringOrNil(worldId)
	}

	if m.Id.IsNil() {
		logrus.Errorf("GetWorld: %s", "world id is missing")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "world id is missing",
			"data":    nil,
		})
	}

	world, err := sm.GetWorld(c.UserContext(), requester, m)
	if err != nil {
		logrus.Errorf("GetWorld: %s", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to get world",
			"data":    nil,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": world,
	})
}

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
func GetWorld(c *fiber.Ctx) error {
	var (
		status      = fiber.StatusOK
		requesterId = uuid.Nil
	)
	defer func() {
		err := database.ReportRequestEvent(c, requesterId, status)
		if err != nil {
			logrus.Errorf("failed to report request: %v", err)
		}
	}()

	//region Requester

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	requesterId = requester.Id

	// Check if requester is banned
	if requester.IsBanned {
		status = fiber.StatusForbidden
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	//region Request metadata

	// Parse batch request metadata from the request
	m := model.WorldRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
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
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no id", "data": nil})
	}

	if model.SupportedPlatform[m.Platform] {
		platform = m.Platform
	}

	if model.SupportedDeployment[m.Deployment] {
		deployment = m.Deployment
	}

	var (
		entity *model.World
	)

	if requester.IsAdmin || requester.IsInternal {
		entity, err = model.GetWorldForAdminWithPak(c.UserContext(), requester, id, platform, deployment)
	} else {
		entity, err = model.GetWorldForRequesterWithPak(c.UserContext(), requester, id, platform, deployment)
	}

	if err != nil {
		status = fiber.StatusInternalServerError
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if entity == nil {
		status = fiber.StatusNotFound
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "not found", "data": nil})
	}

	return c.Status(status).JSON(fiber.Map{"data": entity})
}

func IndexWorldPlaceables(c *fiber.Ctx) error {
	var (
		status      = fiber.StatusOK
		requesterId = uuid.Nil
	)
	defer func() {
		err := database.ReportRequestEvent(c, requesterId, status)
		if err != nil {
			logrus.Errorf("failed to report request: %v", err)
		}
	}()

	//region Requester

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	requesterId = requester.Id

	// Check if requester is banned
	if requester.IsBanned {
		status = fiber.StatusForbidden
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	//region Request metadata

	// Parse batch request metadata from the request
	m := model.BatchRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var (
		worldId       = uuid.UUID{}
		offset  int64 = 0
		limit   int64 = 100
	)

	if m.Offset > 0 {
		offset = m.Offset
	}

	if m.Limit > 0 && m.Limit < 100 {
		limit = m.Limit
	}

	if id := c.Params("id"); id != "" {
		worldId = uuid.FromStringOrNil(id)
	}

	//endregion

	var (
		placeables []model.Object
		total      int64
	)

	if requester.IsAdmin || requester.IsInternal {
		placeables, total, err = model.IndexObjectsForAdminForWorld(c.UserContext(), worldId, offset, limit)
	} else {
		placeables, total, err = model.IndexObjectsForRequesterForWorld(c.UserContext(), requester, worldId, offset, limit)
	}

	if err != nil {
		status = fiber.StatusInternalServerError
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(status).JSON(fiber.Map{"data": fiber.Map{"entities": placeables, "offset": offset, "limit": limit, "total": total}})
}

func CreateWorldPlaceable(c *fiber.Ctx) error {
	var (
		status      = fiber.StatusOK
		requesterId = uuid.Nil
	)
	defer func() {
		err := database.ReportRequestEvent(c, requesterId, status)
		if err != nil {
			logrus.Errorf("failed to report request: %v", err)
		}
	}()

	//region Requester

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		status = fiber.StatusForbidden
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	requesterId = requester.Id
	//endregion

	// Get world
	worldId := uuid.FromStringOrNil(c.Params("id"))
	world, err := model.GetWorldForRequester(c.UserContext(), requester, worldId)
	if err != nil {
		status = fiber.StatusInternalServerError
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if world == nil {
		status = fiber.StatusNotFound
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "world not found", "data": nil})
	}

	accessibles, total, err := model.GetAccessEntityForRequester(c.UserContext(), requester, *world.Id, 0, 100)
	if err != nil {
		return err
	}

	if total == 0 {
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no edit access", "data": nil})
	}

	var bCanEdit = false
	for _, accessible := range accessibles {
		if *accessible.Id == requester.Id && (accessible.CanEdit || accessible.IsOwner) {
			bCanEdit = true
			break
		}
	}

	// Check if requester can edit world
	if !requester.IsAdmin || !bCanEdit {
		status = fiber.StatusForbidden
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "forbidden", "data": nil})
	}

	//region Request metadata

	// Parse batch request metadata from the request
	m := struct {
		Id               *uuid.UUID `json:"id,omitempty"` // If empty, it is a new object
		SlotId           *uuid.UUID `json:"slotId,omitempty"`
		EntityId         *uuid.UUID `json:"entityId,omitempty"`
		Type             string     `json:"type,omitempty"`
		PlaceableClassId uuid.UUID  `json:"placeableClassId"`
		OffsetX          float64    `json:"offsetX,omitempty"`
		OffsetY          float64    `json:"offsetY,omitempty"`
		OffsetZ          float64    `json:"offsetZ,omitempty"`
		RotationX        float64    `json:"rotationX,omitempty"`
		RotationY        float64    `json:"rotationY,omitempty"`
		RotationZ        float64    `json:"rotationZ,omitempty"`
		ScaleX           float64    `json:"scaleX,omitempty"`
		ScaleY           float64    `json:"scaleY,omitempty"`
		ScaleZ           float64    `json:"scaleZ,omitempty"`
	}{}

	err = c.BodyParser(&m)
	if err != nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	// Check if ID is set, if so, it is an update
	if m.Id == nil || m.Id.IsNil() {
		var id uuid.UUID
		id, err = uuid.NewV4()
		if err != nil {
			status = fiber.StatusInternalServerError
			return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}
		m.Id = &id
	}

	// Check if placeable class id is set
	if m.PlaceableClassId == uuid.Nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no class", "data": nil})
	}

	// model.CreateWorldPlaceable(c.UserContext(), m.Id, m.SlotId, m.EntityId, m.Type, m.PlaceableClassId, m.OffsetX, m.OffsetY, m.OffsetZ, m.RotationX, m.RotationY, m.RotationZ, m.ScaleX, m.ScaleY, m.ScaleZ)
	return c.Status(status).JSON(fiber.Map{"status": "error", "message": "not implemented", "data": m.Id})
}

func CreateWorld(c *fiber.Ctx) error {
	var (
		status      = fiber.StatusOK
		requesterId = uuid.Nil
	)
	defer func() {
		err := database.ReportRequestEvent(c, requesterId, status)
		if err != nil {
			logrus.Errorf("failed to report request: %v", err)
		}
	}()

	//region Requester

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		status = fiber.StatusForbidden
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	requesterId = requester.Id
	//endregion

	//region Request metadata

	// Parse batch request metadata from the request
	m := model.WorldCreateMetadata{}
	err = c.BodyParser(&m)
	if err != nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if m.Public == nil {
		var public = true
		m.Public = &public
	}

	if m.Name == "" {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no name", "data": nil})
	}

	if m.Title == nil || *m.Title == "" {
		m.Title = &m.Name
	}

	if m.Map == "" {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no map", "data": nil})
	}

	if m.PackageId.IsNil() {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no package id", "data": nil})
	}

	//endregion

	var entity *model.World
	entity, err = model.CreateWorldForRequester(c.UserContext(), requester, m)
	if err != nil {
		status = fiber.StatusInternalServerError
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(status).JSON(fiber.Map{"status": "ok", "message": nil, "data": entity})
}

func UpdateWorld(c *fiber.Ctx) error {
	var (
		status      = fiber.StatusOK
		requesterId = uuid.Nil
	)
	defer func() {
		err := database.ReportRequestEvent(c, requesterId, status)
		if err != nil {
			logrus.Errorf("failed to report request: %v", err)
		}
	}()

	//region Requester

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	requesterId = requester.Id

	// Check if requester is banned
	if requester.IsBanned {
		status = fiber.StatusForbidden
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	//region Request metadata
	id := uuid.FromStringOrNil(c.Params("id"))

	if id.IsNil() {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no id", "data": nil})
	}

	// Parse batch request metadata from the request
	m := model.WorldUpdateMetadata{}
	err = c.BodyParser(&m)
	if err != nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	//endregion

	var entity *model.World
	if requester.IsAdmin {
		entity, err = model.UpdateWorldForAdmin(c.UserContext(), requester, id, m)
	} else {
		entity, err = model.UpdateWorldForRequester(c.UserContext(), requester, id, m)
	}

	if err != nil {
		status = fiber.StatusInternalServerError
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(status).JSON(fiber.Map{"status": "ok", "message": nil, "data": entity})
}

func DeleteWorld(c *fiber.Ctx) error {
	var (
		status      = fiber.StatusOK
		requesterId = uuid.Nil
	)
	defer func() {
		err := database.ReportRequestEvent(c, requesterId, status)
		if err != nil {
			logrus.Errorf("failed to report request: %v", err)
		}
	}()

	//region Requester

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	requesterId = requester.Id

	// Check if requester is banned
	if requester.IsBanned {
		status = fiber.StatusForbidden
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}
	//endregion

	//region Request metadata
	id := uuid.FromStringOrNil(c.Params("id"))

	if id.IsNil() {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no id", "data": nil})
	}

	//endregion

	if requester.IsAdmin || requester.IsInternal {
		err = model.DeleteWorldForAdmin(c.UserContext(), id)
	} else {
		err = model.DeleteWorldForRequester(c.UserContext(), requester, id)
	}

	if err != nil {
		status = fiber.StatusInternalServerError
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(status).JSON(fiber.Map{"status": "ok", "message": "ok", "data": nil})
}
