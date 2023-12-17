package handler

import (
	"dev.hackerman.me/artheon/veverse-shared/model"
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"
	"veverse-api/database"
	"veverse-api/helper"
	"veverse-api/k8s"
)

func IndexGameServersV2(c *fiber.Ctx) error {
	var (
		status      = fiber.StatusOK
		requesterId = uuid.Nil // used for deferred event reporting
	)

	defer func() {
		err := database.ReportRequestEvent(c, requesterId, status)
		if err != nil {
			logrus.Errorf("failed to report request: %v", err)
		}
	}()

	//region Requester

	requester, err := helper.GetRequester(c)
	if err != nil {
		status = fiber.StatusUnauthorized
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	requesterId = requester.Id

	if requester.IsBanned {
		status = fiber.StatusForbidden
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	//region Query

	m := struct {
		Offset    int64     `json:"offset"`    // Start index
		Limit     int64     `json:"limit"`     // Number of elements to fetch
		ReleaseId uuid.UUID `json:"releaseId"` // Release
	}{}
	err = c.QueryParser(&m)
	if err != nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var (
		offset    int64 = 0
		limit     int64 = 100
		releaseId uuid.UUID
	)

	if m.Offset > 0 {
		offset = m.Offset
	}

	if m.Limit > 0 && m.Limit < 100 {
		limit = m.Limit
	}

	if m.ReleaseId != uuid.Nil {
		releaseId = m.ReleaseId
	}

	//endregion

	var (
		gameServers model.GameServerV2Batch
	)

	gameServers, err = model.IndexGameServersV2(c.UserContext(), requester, releaseId, offset, limit)
	if err != nil {
		status = fiber.StatusInternalServerError
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil, "data": gameServers})
}

func GetGameServerV2(c *fiber.Ctx) error {
	var (
		status      = fiber.StatusOK
		requesterId = uuid.Nil // used for deferred event reporting
	)

	defer func() {
		err := database.ReportRequestEvent(c, requesterId, status)
		if err != nil {
			logrus.Errorf("failed to report request: %v", err)
		}
	}()

	//region Requester

	requester, err := helper.GetRequester(c)
	if err != nil {
		status = fiber.StatusUnauthorized
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	requesterId = requester.Id

	if requester.IsBanned {
		status = fiber.StatusForbidden
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	//region Query

	// parse id from params and convert to uuid
	id, err := uuid.FromString(c.Params("id"))
	if err != nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "invalid id", "data": nil})
	}

	// endregion

	if id == uuid.Nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "invalid id", "data": nil})
	}

	var gameServer *model.GameServerV2
	gameServer, err = model.GetGameServerV2(c.UserContext(), requester, id)
	if err != nil {
		status = fiber.StatusInternalServerError
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if gameServer == nil {
		status = fiber.StatusNotFound
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "not found", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil, "data": gameServer})
}

func CreateGameServerV2(c *fiber.Ctx) error {
	var (
		status      = fiber.StatusOK
		requesterId = uuid.Nil // used for deferred event reporting
	)

	defer func() {
		err := database.ReportRequestEvent(c, requesterId, status)
		if err != nil {
			logrus.Errorf("failed to report request: %v", err)
		}
	}()

	//region Requester

	requester, err := helper.GetRequester(c)
	if err != nil {
		status = fiber.StatusUnauthorized
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	requesterId = requester.Id

	if requester.IsBanned {
		status = fiber.StatusForbidden
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	//region Body

	m := model.CreateGameServerV2Args{}
	err = c.BodyParser(&m)
	if err != nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	//endregion

	var gameServer *model.GameServerV2
	gameServer, err = model.CreateGameServerV2(c.UserContext(), requester, m)
	if err != nil || gameServer == nil {
		status = fiber.StatusInternalServerError
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	//region Cluster
	_, err = k8s.AddGameServerResource(c.UserContext(), *gameServer)
	if err != nil {
		logrus.Errorf("failed to add game server to cluster: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}
	//endregion

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil, "data": gameServer})
}

func UpdateGameServerV2Status(c *fiber.Ctx) error {
	var (
		status      = fiber.StatusOK
		requesterId = uuid.Nil // used for deferred event reporting
	)

	defer func() {
		err := database.ReportRequestEvent(c, requesterId, status)
		if err != nil {
			logrus.Errorf("failed to report request: %v", err)
		}
	}()

	//region Requester

	requester, err := helper.GetRequester(c)
	if err != nil {
		status = fiber.StatusUnauthorized
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	requesterId = requester.Id

	if requester.IsBanned {
		status = fiber.StatusForbidden
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	//region Body

	m := model.UpdateGameServerV2StatusArgs{}
	err = c.BodyParser(&m)
	if err != nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	// endregion

	m.Id = uuid.FromStringOrNil(c.Params("id"))
	if m.Id == uuid.Nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "invalid id", "data": nil})
	}

	err = model.UpdateGameServerV2Status(c.UserContext(), requester, m)
	if err != nil {
		status = fiber.StatusInternalServerError
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil, "data": nil})
}

func DeleteGameServerV2(c *fiber.Ctx) error {
	var (
		status      = fiber.StatusOK
		requesterId = uuid.Nil // used for deferred event reporting
	)

	defer func() {
		err := database.ReportRequestEvent(c, requesterId, status)
		if err != nil {
			logrus.Errorf("failed to report request: %v", err)
		}
	}()

	//region Requester

	requester, err := helper.GetRequester(c)
	if err != nil {
		status = fiber.StatusUnauthorized
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	requesterId = requester.Id

	if requester.IsBanned {
		status = fiber.StatusForbidden
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	//region Body

	// update game server status to offline as we never delete game servers from the database completely
	m := model.UpdateGameServerV2StatusArgs{}
	m.Status = model.GameServerV2StatusOffline

	// endregion

	m.Id = uuid.FromStringOrNil(c.Params("id"))
	if m.Id == uuid.Nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "invalid id", "data": nil})
	}

	err = model.UpdateGameServerV2Status(c.UserContext(), requester, m)
	if err != nil {
		status = fiber.StatusInternalServerError
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	//region Cluster
	err = k8s.DeleteGameServerResource(c.UserContext(), m.Id)
	if err != nil {
		logrus.Errorf("failed to delete game server from cluster: %v", err)
	}
	//endregion

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil, "data": nil})
}

func MatchGameServerV2(c *fiber.Ctx) error {
	var (
		status      = fiber.StatusOK
		requesterId = uuid.Nil // used for deferred event reporting
	)

	defer func() {
		err := database.ReportRequestEvent(c, requesterId, status)
		if err != nil {
			logrus.Errorf("failed to report request: %v", err)
		}
	}()

	//region Requester

	requester, err := helper.GetRequester(c)
	if err != nil {
		status = fiber.StatusUnauthorized
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	requesterId = requester.Id

	if requester.IsBanned {
		status = fiber.StatusForbidden
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	//region Body

	m := model.MatchGameServerV2Args{}
	err = c.BodyParser(&m)
	if err != nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	//endregion

	var (
		gameServer *model.GameServerV2
		created    bool
	)
	gameServer, created, err = model.MatchGameServerV2(c.UserContext(), requester, m)
	if err != nil {
		status = fiber.StatusInternalServerError
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if gameServer == nil {
		status = fiber.StatusNotFound
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no game server found", "data": nil})
	}

	//region Cluster
	if created {
		_, err = k8s.AddGameServerResource(c.UserContext(), *gameServer)
		if err != nil {
			logrus.Errorf("failed to add game server to cluster: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}
	}
	//endregion

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil, "data": gameServer})
}

func AddPlayerToGameServerV2(c *fiber.Ctx) error {
	var (
		status      = fiber.StatusOK
		requesterId = uuid.Nil // used for deferred event reporting
	)

	defer func() {
		err := database.ReportRequestEvent(c, requesterId, status)
		if err != nil {
			logrus.Errorf("failed to report request: %v", err)
		}
	}()

	//region Requester

	requester, err := helper.GetRequester(c)
	if err != nil {
		status = fiber.StatusUnauthorized
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	requesterId = requester.Id

	if requester.IsBanned {
		status = fiber.StatusForbidden
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	// only admins and internal users can register players with game servers
	if !(requester.IsAdmin || requester.IsInternal) {
		status = fiber.StatusForbidden
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "forbidden", "data": nil})
	}

	//endregion

	//region Body

	m := model.AddPlayerToGameServerV2Args{}
	err = c.BodyParser(&m)
	if err != nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	//endregion

	m.GameServerId = uuid.FromStringOrNil(c.Params("id"))
	if m.GameServerId == uuid.Nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "invalid game server id", "data": nil})
	}

	err = model.AddPlayerToGameServerV2(c.UserContext(), requester, m)
	if err != nil {
		status = fiber.StatusInternalServerError
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil, "data": nil})
}

func UpdateGameServerV2PlayerStatus(c *fiber.Ctx) error {
	var (
		status      = fiber.StatusOK
		requesterId = uuid.Nil // used for deferred event reporting
	)

	defer func() {
		err := database.ReportRequestEvent(c, requesterId, status)
		if err != nil {
			logrus.Errorf("failed to report request: %v", err)
		}
	}()

	//region Requester

	requester, err := helper.GetRequester(c)
	if err != nil {
		status = fiber.StatusUnauthorized
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	requesterId = requester.Id

	if requester.IsBanned {
		status = fiber.StatusForbidden
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	// only admins and internal users can register players with game servers
	if !(requester.IsAdmin || requester.IsInternal) {
		status = fiber.StatusForbidden
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "forbidden", "data": nil})
	}

	//endregion

	//region Body

	m := model.UpdateGameServerV2PlayerStatusArgs{}
	err = c.BodyParser(&m)
	if err != nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	//endregion

	m.GameServerId = uuid.FromStringOrNil(c.Params("id"))
	if m.GameServerId == uuid.Nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "invalid game server id", "data": nil})
	}

	m.UserId = uuid.FromStringOrNil(c.Params("playerId"))
	if m.UserId == uuid.Nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "invalid player id", "data": nil})
	}

	err = model.UpdateGameServerV2PlayerStatus(c.UserContext(), requester, m)
	if err != nil {
		status = fiber.StatusInternalServerError
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil, "data": nil})
}

func RemovePlayerFromGameServerV2(c *fiber.Ctx) error {
	var (
		status      = fiber.StatusOK
		requesterId = uuid.Nil // used for deferred event reporting
	)

	defer func() {
		err := database.ReportRequestEvent(c, requesterId, status)
		if err != nil {
			logrus.Errorf("failed to report request: %v", err)
		}
	}()

	//region Requester

	requester, err := helper.GetRequester(c)
	if err != nil {
		status = fiber.StatusUnauthorized
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	requesterId = requester.Id

	if requester.IsBanned {
		status = fiber.StatusForbidden
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	// only admins and internal users can register players with game servers
	if !(requester.IsAdmin || requester.IsInternal) {
		status = fiber.StatusForbidden
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "forbidden", "data": nil})
	}

	//endregion
	//
	//m := model.RemovePlayerFromGameServerV2Args{}
	//m.GameServerId = uuid.FromStringOrNil(c.Params("id"))
	//if m.GameServerId == uuid.Nil {
	//	status = fiber.StatusBadRequest
	//	return c.Status(status).JSON(fiber.Map{"status": "error", "message": "invalid game server id", "data": nil})
	//}
	//
	//m.PlayerId = uuid.FromStringOrNil(c.Params("playerId"))
	//if m.PlayerId == uuid.Nil {
	//	status = fiber.StatusBadRequest
	//	return c.Status(status).JSON(fiber.Map{"status": "error", "message": "invalid player id", "data": nil})
	//}
	//
	//err = model.RemovePlayerFromGameServerV2(c.UserContext(), requester, m)
	//if err != nil {
	//	status = fiber.StatusInternalServerError
	//	return c.Status(status).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	//}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil, "data": nil})
}
