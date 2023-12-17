package handler

import (
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"
	"veverse-api/helper"
	"veverse-api/model"
	"veverse-api/validation"
)

func IndexAnalyticEvent(c *fiber.Ctx) error {
	requester, err := helper.GetRequester(c)
	if err != nil {
		logrus.Errorf("IndexAnalyticEvent: %s", err.Error())
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to get requester",
			"data":    nil,
		})
	}

	if requester != nil && requester.IsBanned {
		logrus.Errorf("IndexAnalyticEvent: %s", "user is banned")
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  "error",
			"message": "banned",
			"data":    nil,
		})
	}

	m := sm.IndexAnalyticEventRequest{}
	err = c.QueryParser(&m)
	if err != nil {
		logrus.Errorf("IndexAnalyticEvent: %s", err.Error())
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to parse query",
			"data":    nil,
		})
	}

	analyticEvents, err := sm.IndexAnalyticEvent(c.UserContext(), requester, m)
	if err != nil {
		logrus.Errorf("IndexAnalyticEvent: %s", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to get analytic events",
			"data":    nil,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  "success",
		"message": "got entities",
		"data":    analyticEvents,
	})
}

func IndexEntityAnalytic(c *fiber.Ctx) error {
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

	//region Request metadata
	// Parse batch request metadata from the request
	requestMetadata := model.AnalyticRequestMetadata{}
	err = c.QueryParser(&requestMetadata)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	// Get app id from the request params
	requestMetadata.AppId = c.Params("id")

	err = validation.Validator.Struct(requestMetadata)
	if err != nil {
		errors := model.GetErrors(err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "validation error", "data": errors})
	}

	var (
		offset int64 = 0
		limit  int64 = 1000
	)

	appId := uuid.FromStringOrNil(requestMetadata.AppId)

	if requestMetadata.Offset > 0 {
		offset = requestMetadata.Offset
	}

	if requestMetadata.Limit > 0 && requestMetadata.Limit < 1000 {
		limit = requestMetadata.Limit
	}

	var (
		analytics []sm.AnalyticEvent
		total     uint64
	)

	analytics, total, err = model.IndexAnalyticsForApp(
		c.UserContext(),
		appId,
		requestMetadata.Platform,
		requestMetadata.Deployment,
		offset,
		limit,
	)

	//analytics, total, err = model.IndexAnalyticsForEntity(
	//	c.UserContext(),
	//	id,
	//	requestMetadata.Platform,
	//	requestMetadata.Deployment,
	//	offset,
	//	limit,
	//)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fiber.Map{"analytics": analytics, "offset": offset, "limit": limit, "total": total}})
}

func ReportEvent(c *fiber.Ctx) error {
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	// Parse analytic metadata from the request
	var metadata model.AnalyticEventRequest

	err = c.BodyParser(&metadata)

	err = model.ReportEvent(c.UserContext(), requester, metadata)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "message": "event reported", "data": nil})
}
