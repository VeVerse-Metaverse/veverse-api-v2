package handler

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"veverse-api/helper"
	"veverse-api/model"
)

// IndexObjectClasses godoc
// @Summary      Index Object Classes
// @Description  Fetch object classes in batches
// @Tags         objects
// @Accept       json
// @Produce      json
// @Security	 Bearer
// @Param        category query string false "Specify to filter by category"
// @Param        query query string false "Specify to filter by the portal, destination portal, destination space or destination metaverse name"
// @Param        offset query int false "Pagination offset, default 0"
// @Param        offset query int false "Pagination limit, default 100"
// @Success      200  {object}  []model.Portal
// @Failure      400  {object}  error
// @Failure      404  {object}  error
// @Failure      500  {object}  error
// @Router       /placeable_classes [get]
func IndexObjectClasses(c *fiber.Ctx) error {
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
	m := model.ObjectClassBatchRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var (
		offset   int64 = 0
		limit    int64 = 100
		query          = ""
		category       = ""
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

	if m.Category != "" {
		category = m.Category
	}

	//endregion

	var (
		entities []model.ObjectClass
		total    int64
	)

	if requester.IsAdmin || requester.IsInternal {
		if query == "" {
			if category == "" {
				entities, total, err = model.IndexObjectClassesForAdmin(c.UserContext(), offset, limit)
			} else {
				entities, total, err = model.IndexObjectClassesForAdminForCategory(c.UserContext(), category, offset, limit)
			}
		} else {
			if category == "" {
				entities, total, err = model.IndexObjectClassesForAdminWithQuery(c.UserContext(), offset, limit, query)
			} else {
				entities, total, err = model.IndexObjectClassesForAdminForCategoryWithQuery(c.UserContext(), category, offset, limit, query)
			}
		}
	} else {
		if query == "" {
			if category == "" {
				entities, total, err = model.IndexObjectClassesForRequester(c.UserContext(), requester, offset, limit)
			} else {
				entities, total, err = model.IndexObjectClassesForRequesterForCategory(c.UserContext(), requester, category, offset, limit)
			}
		} else {
			if category == "" {
				entities, total, err = model.IndexObjectClassesForRequesterWithQuery(c.UserContext(), requester, offset, limit, query)
			} else {
				entities, total, err = model.IndexObjectClassesForRequesterForCategoryWithQuery(c.UserContext(), requester, category, offset, limit, query)
			}
		}
	}

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fiber.Map{"entities": entities, "offset": offset, "limit": limit, "total": total}})
}

func IndexObjectClassCategories(c *fiber.Ctx) error {
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
	m := model.BatchRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var (
		offset int64 = 0
		limit  int64 = 100
		query  string
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

	//endregion

	var (
		placeables []string
		total      int64
	)

	if requester.IsAdmin || requester.IsInternal {
		if query == "" {
			placeables, total, err = model.IndexObjectClassCategoriesForAdmin(c.UserContext(), offset, limit)
		} else {
			placeables, total, err = model.IndexObjectClassCategoriesForAdminWithQuery(c.UserContext(), offset, limit, query)
		}
	} else {
		if query == "" {
			placeables, total, err = model.IndexObjectClassCategoriesForRequester(c.UserContext(), requester, offset, limit)
		} else {
			placeables, total, err = model.IndexObjectClassCategoriesForRequesterWithQuery(c.UserContext(), requester, offset, limit, query)
		}
	}

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fiber.Map{"entities": placeables, "offset": offset, "limit": limit, "total": total}})
}
