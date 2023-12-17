package handler

import (
	"github.com/gofiber/fiber/v2"
	"veverse-api/helper"
	"veverse-api/model"
)

func GetStats(c *fiber.Ctx) error {
	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	m := model.StatsRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

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

	stats, err := model.GetStats(c.UserContext(), requester, platform, deployment)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": stats})
}
