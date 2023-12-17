/* Instance Statuses:
- Deleted - deleted (on demand or spot) (Terminated instance status)
Stopped - stopped (on demand only) (Stopped instance status)
- Pending - pending to be Free (pending to make instance)
- Free - ready to be used (its launcher is waiting for the session) (Starting instance status)
Occupied - launcher is running the game, a user is connected (Starting instance status)

Session Statuses:
Pending - waiting for any launcher to catch up the session and start game
Starting - waiting for the assigned launcher to prepare and start the game
Running - launcher is running the game, a user is connected
Closed - user has disconnected, session has been closed, all user data has been purged
*/

package handler

import (
	"fmt"
	"veverse-api/database"
	"veverse-api/helper"
	"veverse-api/model"
	"veverse-api/reflect"
	"veverse-api/validation"

	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"
)

func GetLatestPSLauncher(c *fiber.Ctx) (err error) {
	return c.Status(fiber.StatusOK).Send([]byte("https://s3.xxxx.amazonaws.com/storage.xxxx.com/launcher.exe"))
}

// GetPendingSession For pixelstreaming app launcher
func GetPendingSession(c *fiber.Ctx) (err error) {
	db := database.DB

	var (
		requester *sm.User
	)

	// Get requester
	requester, err = helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	// Check if requester is admin
	if !requester.IsAdmin {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "requester is not admin", "data": nil})
	}

	q := `SELECT pss.id, psi.instance_type, pss.app_id, pss.world_id, pss.Status
FROM
	pixel_streaming_instance psi
	INNER JOIN pixel_streaming_sessions pss ON psi.id = pss.instance_id
WHERE psi.status = 'free' AND pss.status = 'pending'
ORDER BY
	(
		CASE psi.instance_type WHEN 'spot' THEN 1 WHEN 'on-demand' THEN 2 END
	)
LIMIT 1`

	var (
		ps sm.PixelStreamingSessionData
	)

	row := db.QueryRow(c.UserContext(), q)

	err = row.Scan(&ps.Id, &ps.InstanceType, &ps.AppId, &ps.WorldId, &ps.Status)
	if err != nil {
		if err.Error() != "no rows in result set" {
			logrus.Errorf("failed to scan %s @ %s: %v", model.PSInstanceSingular, reflect.FunctionName(), err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "something went wrong", "data": nil})
		} else {
			return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": "no pending session", "data": nil})
		}
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": "ok", "data": ps})
}

func RequestPSSession(c *fiber.Ctx) (err error) {
	db := database.DB

	m := model.PixelStreamingSessionRequestMetadata{}
	if err = c.BodyParser(&m); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	err = validation.Validator.Struct(m)
	if err != nil {
		errors := model.GetErrors(err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "validation error", "data": errors})
	}

	q := `SELECT psi.id, count(pss.id) as sessions_count
FROM
	pixel_streaming_instance psi
	LEFT JOIN pixel_streaming_sessions pss ON psi.id = pss.instance_id AND pss.status <> 'closed' 
WHERE
	psi.status = 'free'
GROUP BY psi.id, psi.instance_type
ORDER BY
	(
		CASE psi.instance_type WHEN 'spot' THEN 1 WHEN 'on-demand' THEN 2 END
	), sessions_count
LIMIT 1`

	var (
		instanceUUID          *uuid.UUID
		instanceSessionsCount int32
	)

	row := db.QueryRow(c.UserContext(), q)

	err = row.Scan(&instanceUUID, &instanceSessionsCount)
	if err != nil {
		if err.Error() != "no rows in result set" {
			logrus.Errorf("failed to query %s @ %s: %v", model.PSInstanceSingular, reflect.FunctionName(), err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "something went wrong", "data": nil})
		} else {
			return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": "retry request", "data": fiber.Map{"freeInstance": false}})
		}
	}

	if instanceSessionsCount > 0 {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": "retry request", "data": fiber.Map{"freeInstance": false}})
	}

	var id uuid.UUID
	id, err = uuid.NewV4()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "failed to generate uuid", "data": nil})
	}

	q = `INSERT INTO pixel_streaming_sessions (id, instance_id, app_id, world_id, status) VALUES ($1, $2, $3, $4, $5)`
	if _, err = db.Exec(c.UserContext(), q, id, instanceUUID, m.AppId, m.WorldId, "pending"); err != nil {
		logrus.Errorf("failed to insert query %s @ %s: %v", model.PSInstanceSingular, reflect.FunctionName(), err)
		return fmt.Errorf("failed to set %s", model.PSInstanceSingular)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": "ok", "data": fiber.Map{"freeInstance": true, "sessionId": id.String()}})
}

func UpdatePSSession(c *fiber.Ctx) (err error) {
	db := database.DB

	var (
		requester *sm.User
	)

	// Get requester
	requester, err = helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	// Check if requester is admin
	if !requester.IsAdmin {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "requester is not admin", "data": nil})
	}

	//region Request metadata
	id := uuid.FromStringOrNil(c.Params("id"))
	if id.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid id", "data": nil})
	}

	// Parse batch request metadata from the request
	m := model.PixelStreamingSessionUpdateMetadata{}
	err = c.BodyParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}
	fmt.Println(c.Params("id"), m.AppId)
	err = validation.Validator.Struct(m)
	if err != nil {
		errors := model.GetErrors(err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "validation error", "data": errors})
	}

	appId, _ := uuid.FromString(m.AppId)

	q := `UPDATE pixel_streaming_sessions SET status = $1 WHERE id=$2 AND app_id = $3`

	_, err = db.Exec(c.UserContext(), q, m.Status, id, appId)
	if err != nil {
		logrus.Errorf("failed to update query %s @ %s: %v", model.PSInstanceSingular, reflect.FunctionName(), err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "something went wrong", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "data": nil})
}

func UpdatePSInstanceStatus(ctx *fiber.Ctx) (err error) {
	db := database.DB

	m := model.PixelStreamingInstanceUpdateMetadata{}
	err = ctx.BodyParser(&m)
	if err != nil {
		ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		return
	}

	err = validation.Validator.Struct(m)
	if err != nil {
		errors := model.GetErrors(err)
		ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "validation error", "data": errors})
		return
	}

	q := `UPDATE pixel_streaming_instance SET status = $1 WHERE instance_id=$2`

	_, err = db.Exec(ctx.UserContext(), q, m.Status, m.InstanceId)
	if err != nil {
		logrus.Errorf("failed to update query %s @ %s: %v", model.PSInstanceSingular, reflect.FunctionName(), err)
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "something went wrong", "data": nil})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "data": nil})
}

func GetPSSessionData(c *fiber.Ctx) (err error) {
	db := database.DB

	id := uuid.FromStringOrNil(c.Params("id"))
	if id.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid id", "data": nil})
	}

	q := `SELECT
		pss.id, pss.app_id, pss.world_id, pss.status, psi.status, psi.host, psi.port
FROM pixel_streaming_sessions pss
INNER JOIN pixel_streaming_instance psi ON pss.instance_id = psi.id
WHERE pss.id = $1`

	row := db.QueryRow(c.UserContext(), q, id)

	var (
		pss sm.PixelStreamingSession
		psi sm.PixelStreamingInstance
	)

	err = row.Scan(&pss.Id, &pss.AppId, &pss.WorldId, &pss.Status, &psi.Status, &psi.Host, &psi.Port)
	if err != nil {
		if err.Error() != "no rows in result set" {
			logrus.Errorf("failed to query %s @ %s: %v", model.PSInstanceSingular, reflect.FunctionName(), err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "something went wrong", "data": nil})
		}
	}

	response := fiber.Map{
		"sessionId": pss.Id,
		"appId":     pss.AppId,
		"worldId":   pss.WorldId,
		"status":    pss.Status,
		"instance": fiber.Map{
			"status": psi.Status,
			"host":   psi.Host,
			"port":   psi.Port,
		},
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "data": response})
}
