package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"golang.org/x/exp/slices"
	"veverse-api/helper"
	"veverse-api/model"
)

type PropertyInput struct {
	*model.Property
}

func AddProperties(c *fiber.Ctx) error {
	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	props := []model.Property{}
	if err = c.BodyParser(&props); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	entityId := uuid.FromStringOrNil(c.Params("id"))
	if entityId.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no id", "data": nil})
	}

	vals := []interface{}{}
	names := []string{}
	for _, row := range props {

		if slices.Contains(names, row.Name) {
			continue
		}

		names = append(names, row.Name)
		vals = append(vals, entityId, row.Type, row.Name, row.Value)
	}

	if requester.IsAdmin || requester.IsInternal {
		err = model.UpsertProperties(c.Context(), vals)

		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}
	} else {
		isOwner, _, canEdit, _, err1 := model.EntityAccessible(c.Context(), requester.Id, entityId)

		if isOwner || canEdit {
			err1 = model.UpsertProperties(c.Context(), vals)
		} else {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "requester doesn't have access to edit entity properties", "data": nil})
		}

		if err1 != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil, "data": props})
}
