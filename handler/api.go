package handler

import "github.com/gofiber/fiber/v2"

// Home handle api status
func Home(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok", "message": "ok", "data": nil})
}

func HealthCheck(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success"})
}
