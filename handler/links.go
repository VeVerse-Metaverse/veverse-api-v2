package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"veverse-api/model"
)

func GetSdkLink(c *fiber.Ctx) (err error) {

	m := model.IdRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}
	//m.Id = uuid.FromStringOrNil(c.Params("id"))

	var (
		id uuid.UUID
	)

	if !m.Id.IsNil() {
		id = m.Id
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no id", "data": nil})
	}

	link, err := model.GetAppSdkLinkForAdmin(c.UserContext(), id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if link == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "no link", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": link, "status": "ok", "message": ""})
}
