package handler

import (
	vModel "dev.hackerman.me/artheon/veverse-shared/model"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"veverse-api/helper"
	"veverse-api/model"
)

func IndexPackages(c *fiber.Ctx) error {
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
	m := model.PackageBatchRequestMetadata{}
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
	)

	if m.Offset > 0 {
		offset = m.Offset
	}

	if m.Limit > 0 && m.Limit < 100 {
		limit = m.Limit
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
		entities []model.Package
		total    int64
	)

	if requester.IsAdmin || requester.IsInternal {
		if query == "" {
			if withPak {
				entities, total, err = model.IndexPackagesForAdminWithPak(c.UserContext(), requester, offset, limit, platform, deployment)
			} else {
				entities, total, err = model.IndexPackagesForAdmin(c.UserContext(), requester, offset, limit)
			}
		} else {
			if withPak {
				entities, total, err = model.IndexPackagesForAdminWithQueryWithPak(c.UserContext(), requester, offset, limit, query, platform, deployment)
			} else {
				entities, total, err = model.IndexPackagesForAdminWithQuery(c.UserContext(), requester, offset, limit, query)
			}
		}
	} else {
		if query == "" {
			if withPak {
				entities, total, err = model.IndexPackagesForRequesterWithPak(c.UserContext(), requester, offset, limit, platform, deployment)
			} else {
				entities, total, err = model.IndexPackagesForRequester(c.UserContext(), requester, offset, limit)
			}
		} else {
			if withPak {
				entities, total, err = model.IndexPackagesForRequesterWithQueryWithPak(c.UserContext(), requester, offset, limit, query, platform, deployment)
			} else {
				entities, total, err = model.IndexPackagesForRequesterWithQuery(c.UserContext(), requester, offset, limit, query)
			}
		}
	}

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fiber.Map{"entities": entities, "offset": offset, "limit": limit, "total": total}})
}

func GetPackage(c *fiber.Ctx) error {
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
	m := model.PackageRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	m.Id = uuid.FromStringOrNil(c.Params("id"))

	var (
		platform   = ""
		deployment = ""
	)

	if model.SupportedPlatform[m.Platform] {
		platform = m.Platform
	}

	if model.SupportedDeployment[m.Deployment] {
		deployment = m.Deployment
	}

	withPak := deployment != "" && platform != ""
	//endregion

	var (
		entity *model.Package
	)

	if requester.IsAdmin || requester.IsInternal {
		if withPak {
			entity, err = model.GetPackageForAdminWithPak(c.UserContext(), requester, m.Id, platform, deployment)
		} else {
			entity, err = model.GetPackageForAdmin(c.UserContext(), requester, m.Id)
		}
	} else {
		if withPak {
			entity, err = model.GetPackageForRequesterWithPak(c.UserContext(), requester, m.Id, platform, deployment)
		} else {
			entity, err = model.GetPackageForRequester(c.UserContext(), requester, m.Id)
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

func CreatePackage(c *fiber.Ctx) error {
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
	m := model.PackageCreateMetadata{}
	err = c.BodyParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if m.Public == nil {
		var public = true
		m.Public = &public
	}

	if m.Title == nil || *m.Title == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no title", "data": nil})
	}

	m.Name = *m.Title

	if m.Release == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no base release name", "data": nil})
	}

	//endregion

	var entity *model.Package
	entity, err = model.CreatePackageForRequester(c.UserContext(), requester, m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil, "data": entity})
}

func UpdatePackage(c *fiber.Ctx) error {
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
	m := model.PackageUpdateMetadata{}
	err = c.BodyParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	//endregion

	var entity *model.Package
	entity, err = model.UpdatePackageForRequester(c.UserContext(), requester, id, m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil, "data": entity})
}

func IndexPackageMaps(c *fiber.Ctx) error {
	//region Requester

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester or invalid token", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	//region Request metadata

	// Parse batch request metadata from the request

	m := vModel.BatchRequestMetadata{}
	err = c.QueryParser(&m)

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	//endregion
	var (
		id     uuid.UUID
		offset int64 = 0
		limit  int64 = 100
	)

	id = uuid.FromStringOrNil(c.Params("id"))

	if m.Offset > 0 {
		offset = m.Offset
	}

	if m.Limit > 0 && m.Limit < 100 {
		limit = m.Limit
	}

	var entities []string

	entities, err = model.IndexPackageMapsForRequester(c.UserContext(), requester, id, offset, limit)

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil, "data": entities})
}
