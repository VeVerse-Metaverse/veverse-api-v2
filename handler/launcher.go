package handler

import (
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"
	"veverse-api/database"
	"veverse-api/helper"
	"veverse-api/model"
)

func IndexLaunchers(c *fiber.Ctx) error {
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

	m := sm.IndexLauncherV2Request{}
	err = c.QueryParser(&m)
	if err != nil {
		logrus.Errorf("IndexLaunchers: %s", err.Error())
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to parse query",
			"data":    nil,
		})
	}

	launchers, err := sm.IndexLauncherV2(c.UserContext(), requester, m)
	if err != nil {
		logrus.Errorf("IndexLaunchers: %s", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to get launchers",
			"data":    nil,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  "success",
		"message": "got launchers",
		"data":    launchers,
	})
}

func GetLauncher(c *fiber.Ctx) error {
	requester, err := helper.GetRequester(c)
	if err != nil {
		logrus.Errorf("GetLauncher: %s", err.Error())
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to get requester",
			"data":    nil,
		})
	}

	if requester != nil && requester.IsBanned {
		logrus.Errorf("GetLauncher: %s", "user is banned")
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  "error",
			"message": "banned",
			"data":    nil,
		})
	}

	m := struct {
		model.IdRequestMetadata
		Platform string `json:"platform"`
	}{}
	err = c.QueryParser(&m)
	if err != nil {
		logrus.Errorf("GetLauncher: %s", err.Error())
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to parse query",
			"data":    nil,
		})
	}
	m.Id = uuid.FromStringOrNil(c.Params("id"))

	var id uuid.UUID
	if !m.Id.IsNil() {
		id = m.Id
	} else {
		logrus.Errorf("GetLauncher: %s", "id is nil")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "invalid id",
			"data":    nil,
		})
	}

	// todo: temporary: convert user to sm.User, replace with proper user model after migration to veverse-shared
	var user *sm.User
	if requester != nil {
		user = &sm.User{
			Entity: sm.Entity{
				Identifier: sm.Identifier{Id: requester.Id},
			},
			IsAdmin: requester.IsAdmin,
		}
	} else {
		logrus.Errorf("GetLauncher: %s", "requester is nil")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "unauthorized",
			"data":    nil,
		})
	}

	launcher, err := sm.GetLauncherV2(c.UserContext(), user, id, m.Platform)
	if err != nil {
		logrus.Errorf("GetLauncher: %s", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to get launcher",
			"data":    nil,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": launcher})
}

func IndexLauncherApps(c *fiber.Ctx) error {
	requester, err := helper.GetRequester(c)
	if err != nil {
		logrus.Errorf("IndexLauncherApps: %s", err.Error())
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to get requester",
			"data":    nil,
		})
	}

	if requester != nil && requester.IsBanned {
		logrus.Errorf("IndexLauncherApps: %s", "user is banned")
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  "error",
			"message": "banned",
			"data":    nil,
		})
	}

	m := struct {
		model.IdRequestMetadata
		Platform string `json:"platform"`
		Offset   int64  `json:"offset"`
		Limit    int64  `json:"limit"`
	}{}
	err = c.QueryParser(&m)
	if err != nil {
		logrus.Errorf("IndexLauncherApps: %s", err.Error())
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to parse query",
			"data":    nil,
		})
	}
	m.Id = uuid.FromStringOrNil(c.Params("id"))

	var id uuid.UUID
	if !m.Id.IsNil() {
		id = m.Id
	} else {
		logrus.Errorf("IndexLauncherApps: %s", "id is nil")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "invalid id",
			"data":    nil,
		})
	}

	// todo: temporary: convert user to sm.User, replace with proper user model after migration to veverse-shared
	var user *sm.User
	if requester != nil {
		user = &sm.User{
			Entity: sm.Entity{
				Identifier: sm.Identifier{Id: requester.Id},
			},
			IsAdmin: requester.IsAdmin,
		}
	} else {
		logrus.Errorf("GetLauncher: %s", "requester is nil")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "unauthorized",
			"data":    nil,
		})
	}

	launcher, err := sm.IndexLauncherV2Apps(c.UserContext(), user, id, m.Platform, m.Offset, m.Limit)
	if err != nil {
		logrus.Errorf("IndexLauncherApps: %s", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to get launcher apps",
			"data":    nil,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": launcher})
}

func IndexLauncherReleases(c *fiber.Ctx) error {
	requester, err := helper.GetRequester(c)
	if err != nil {
		logrus.Errorf("IndexLauncherReleases: %s", err.Error())
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to get requester",
			"data":    nil,
		})
	}

	if requester != nil && requester.IsBanned {
		logrus.Errorf("IndexLauncherReleases: %s", "user is banned")
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  "error",
			"message": "banned",
			"data":    nil,
		})
	}

	m := struct {
		model.IdRequestMetadata
		Platform string `json:"platform"`
		Offset   int64  `json:"offset"`
		Limit    int64  `json:"limit"`
	}{}
	err = c.QueryParser(&m)
	if err != nil {
		logrus.Errorf("IndexLauncherReleases: %s", err.Error())
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to parse query",
			"data":    nil,
		})
	}
	m.Id = uuid.FromStringOrNil(c.Params("id"))

	var id uuid.UUID
	if !m.Id.IsNil() {
		id = m.Id
	} else {
		logrus.Errorf("IndexLauncherReleases: %s", "id is nil")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "invalid id",
			"data":    nil,
		})
	}

	// todo: temporary: convert user to sm.User, replace with proper user model after migration to veverse-shared
	var user *sm.User
	if requester != nil {
		user = &sm.User{
			Entity: sm.Entity{
				Identifier: sm.Identifier{Id: requester.Id},
			},
			IsAdmin: requester.IsAdmin,
		}
	} else {
		logrus.Errorf("GetLauncher: %s", "requester is nil")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "unauthorized",
			"data":    nil,
		})
	}

	launcher, err := sm.IndexLauncherV2Releases(c.UserContext(), user, id, m.Platform, m.Offset, m.Limit)
	if err != nil {
		logrus.Errorf("IndexLauncherReleases: %s", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to get launcher releases",
			"data":    nil,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": launcher})
}

func GetLauncherPublic(c *fiber.Ctx) error {
	m := struct {
		model.IdRequestMetadata
		Platform string `query:"platform"`
	}{}
	err := c.QueryParser(&m)
	if err != nil {
		logrus.Errorf("GetLauncher: %s", err.Error())
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to parse query",
			"data":    nil,
		})
	}
	m.Id = uuid.FromStringOrNil(c.Params("id"))

	var id uuid.UUID
	if !m.Id.IsNil() {
		id = m.Id
	} else {
		logrus.Errorf("GetLauncher: %s", "id is nil")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "invalid id",
			"data":    nil,
		})
	}

	var user = &sm.User{}

	launcher, err := sm.GetLauncherV2(c.UserContext(), user, id, m.Platform)
	if err != nil {
		logrus.Errorf("GetLauncher: %s", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to get launcher",
			"data":    nil,
		})
	}

	if launcher == nil {
		logrus.Errorf("GetLauncher: %s", "launcher is nil")
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status":  "error",
			"message": "launcher not found",
			"data":    nil,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": launcher})
}

func IndexLauncherAppsPublic(c *fiber.Ctx) error {
	m := struct {
		model.IdRequestMetadata
		Platform string `json:"platform"`
		Offset   int64  `json:"offset"`
		Limit    int64  `json:"limit"`
	}{}
	err := c.QueryParser(&m)
	if err != nil {
		logrus.Errorf("IndexLauncherApps: %s", err.Error())
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to parse query",
			"data":    nil,
		})
	}
	m.Id = uuid.FromStringOrNil(c.Params("id"))

	var id uuid.UUID
	if !m.Id.IsNil() {
		id = m.Id
	} else {
		logrus.Errorf("IndexLauncherApps: %s", "id is nil")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "invalid id",
			"data":    nil,
		})
	}

	var user = &sm.User{}

	launcher, err := sm.IndexLauncherV2Apps(c.UserContext(), user, id, m.Platform, m.Offset, m.Limit)
	if err != nil {
		logrus.Errorf("IndexLauncherApps: %s", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to get launcher apps",
			"data":    nil,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": launcher})
}

func IndexLauncherReleasesPublic(c *fiber.Ctx) error {
	m := struct {
		model.IdRequestMetadata
		Platform string `json:"platform"`
		Offset   int64  `json:"offset"`
		Limit    int64  `json:"limit"`
	}{}
	err := c.QueryParser(&m)
	if err != nil {
		logrus.Errorf("IndexLauncherReleases: %s", err.Error())
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to parse query",
			"data":    nil,
		})
	}
	m.Id = uuid.FromStringOrNil(c.Params("id"))

	var id uuid.UUID
	if !m.Id.IsNil() {
		id = m.Id
	} else {
		logrus.Errorf("IndexLauncherReleases: %s", "id is nil")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "invalid id",
			"data":    nil,
		})
	}

	var user = &sm.User{}

	launcher, err := sm.IndexLauncherV2Releases(c.UserContext(), user, id, m.Platform, m.Offset, m.Limit)
	if err != nil {
		logrus.Errorf("IndexLauncherReleases: %s", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to get launcher releases",
			"data":    nil,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": launcher})
}

func CreateLauncher(c *fiber.Ctx) error {
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

	//region Request

	// Get request
	m := sm.CreateLauncherV2Request{}
	err = c.BodyParser(&m)
	if err != nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "failed to parse body", "data": nil})
	}

	//endregion

	//region Create

	// Create launcher
	launcher, err := sm.CreateLauncherV2(c.UserContext(), requester, m)
	if err != nil {
		status = fiber.StatusInternalServerError
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "failed to create launcher", "data": nil})
	}

	//endregion

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": launcher})
}

func UpdateLauncher(c *fiber.Ctx) error {
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
	m := struct {
		Name string `json:"name"`
	}{}
	err = c.BodyParser(&m)
	if err != nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	//endregion

	var entity *sm.LauncherV2
	entity, err = sm.UpdateLauncherV2(c.UserContext(), requester, id, m.Name)

	if err != nil {
		status = fiber.StatusInternalServerError
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(status).JSON(fiber.Map{"status": "ok", "message": nil, "data": entity})
}
