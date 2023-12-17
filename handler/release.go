package handler

import (
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"
	"veverse-api/helper"
	"veverse-api/model"
)

func IndexReleases(c *fiber.Ctx) error {
	requester, err := helper.GetRequester(c)
	if err != nil {
		logrus.Errorf("IndexReleases: %s", err.Error())
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to get requester",
			"data":    nil,
		})
	}

	if requester != nil && requester.IsBanned {
		logrus.Errorf("IndexReleases: %s", "user is banned")
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"status":  "error",
			"message": "banned",
			"data":    nil,
		})
	}

	m := sm.IndexReleaseV2Request{}
	err = c.QueryParser(&m)
	if err != nil {
		logrus.Errorf("IndexReleases: %s", err.Error())
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to parse query",
			"data":    nil,
		})
	}

	launchers, err := sm.IndexReleaseV2(c.UserContext(), requester, m)
	if err != nil {
		logrus.Errorf("IndexReleases: %s", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to get launchers",
			"data":    nil,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  "success",
		"message": "got entities",
		"data":    launchers,
	})
}

func GetRelease(c *fiber.Ctx) error {
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

	launcher, err := sm.GetReleaseV2(c.UserContext(), user, id)
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

func GetLatestReleaseV2Public(c *fiber.Ctx) error {
	m := struct {
		AppId         uuid.UUID `json:"appId" query:"app-id"`
		Target        string    `json:"target"`        // Server, Client, etc.
		Platform      string    `json:"platform"`      // Windows, Linux, etc.
		Configuration string    `json:"configuration"` // Development, Test, Shipping, etc.
	}{}
	err := c.QueryParser(&m)
	if err != nil {
		logrus.Errorf("GetLatestReleaseV2: %s", err.Error())
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to parse query",
			"data":    nil,
		})
	}

	if m.AppId.IsNil() {
		logrus.Errorf("GetLatestReleaseV2: %s", "appId is nil")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "invalid appId",
			"data":    nil,
		})
	}

	if m.Target == "" {
		logrus.Errorf("GetLatestReleaseV2: %s", "target is empty")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "invalid target",
			"data":    nil,
		})
	}

	if m.Platform == "" {
		logrus.Errorf("GetLatestReleaseV2: %s", "platform is empty")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "invalid platform",
			"data":    nil,
		})
	}

	request := sm.GetLatestReleaseRequest{
		AppId: m.AppId,
		Options: &sm.LatestReleaseRequestOptions{
			Files: true,
			FileOptions: &sm.LatestReleaseRequestFileOptions{
				Platform: m.Platform,
				Target:   m.Target,
			},
			Owner: true,
		},
	}

	releaseV2, err := sm.GetLatestReleaseV2Public(c.UserContext(), request)
	if err != nil {
		logrus.Errorf("GetLatestReleaseV2: %s", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to get release",
			"data":    nil,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": releaseV2})
}
