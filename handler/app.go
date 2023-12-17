package handler

import (
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"
	"strings"
	"veverse-api/database"
	"veverse-api/helper"
	"veverse-api/model"
	"veverse-api/reflect"
)

func CreateApp(c *fiber.Ctx) error {
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
	m := sm.CreateAppV2Request{}
	err = c.BodyParser(&m)
	if err != nil {
		status = fiber.StatusBadRequest
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "failed to parse body", "data": nil})
	}

	//endregion

	//region Create

	// Create app
	app, err := sm.CreateAppV2(c.UserContext(), requester, m)
	if err != nil {
		status = fiber.StatusInternalServerError
		return c.Status(status).JSON(fiber.Map{"status": "error", "message": "failed to create app", "data": nil})
	}

	//endregion

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": app})
}

func IndexApps(c *fiber.Ctx) error {
	db := database.DB

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	// Parse batch request metadata from the request
	m := model.IndexJobsRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var offset int64 = 0
	if m.Offset > 0 {
		offset = m.Offset
	}

	var limit int64 = 10
	if m.Limit > 0 && m.Limit < 20 {
		limit = m.Limit
	}

	var (
		row   pgx.Row
		total int32
	)

	q := `SELECT COUNT(*) FROM apps`
	row = db.QueryRow(c.UserContext(), q)

	err = row.Scan(&total)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}
	//endregion

	var apps []model.App
	if requester.IsAdmin || requester.IsInternal {
		apps, total, err = model.IndexAppsForAdmin(c.UserContext(), offset, limit)
	} else {
		apps, total, err = model.IndexAppsForRequester(c.UserContext(), requester, offset, limit)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fiber.Map{"offset": offset, "limit": limit, "total": total, "entities": apps}})
}

func IndexOwnedApps(c *fiber.Ctx) error {
	db := database.DB

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	// Parse batch request metadata from the request
	m := model.IndexJobsRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var offset int64 = 0
	if m.Offset > 0 {
		offset = m.Offset
	}

	var limit int64 = 10
	if m.Limit > 0 && m.Limit < 20 {
		limit = m.Limit
	}

	var (
		row   pgx.Row
		total int32
	)

	q := `SELECT COUNT(*) FROM apps LEFT JOIN accessibles a ON a.entity_id = apps.id WHERE user_id = $1 AND a.is_owner = true`
	row = db.QueryRow(c.UserContext(), q, requester.Id)

	err = row.Scan(&total)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}
	//endregion

	var apps []model.App
	apps, total, err = model.IndexOwnedAppsForRequester(c.UserContext(), requester, offset, limit)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fiber.Map{"offset": offset, "limit": limit, "total": total, "apps": apps}})
}

func UpdateAppMetadata(c *fiber.Ctx) (err error) {
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

	// Parse batch request metadata from the request
	m := model.App{}
	err = c.BodyParser(&m)
	if err != nil {
		logrus.Errorf("failed to parse %s @ %s: %v", model.AppSingular, reflect.FunctionName(), err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "error parse body", "data": nil})
	}

	//endregion

	var entity *model.World
	if requester.IsAdmin {
		//err = model.UpdateAppForAdmin(c.UserContext(), id, m)
	} else {
		//err = model.UpdateAppForRequester(c.UserContext(), requester, id, m)
	}

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil, "data": entity})
}

func NewAppRelease(c *fiber.Ctx) (err error) {
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
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no app id", "data": nil})
	}

	// Parse batch request metadata from the request
	m := model.ReleaseUpdateMetadata{}
	err = c.BodyParser(&m)
	if err != nil {
		logrus.Errorf("failed to parse %s @ %s: %v", model.ReleaseSingular, reflect.FunctionName(), err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "error parse body", "data": nil})
	}

	//endregion
	if requester.IsAdmin {
		err = model.AddReleaseForAdmin(c.UserContext(), requester, id, m)
	} else {
		err = model.AddReleaseForRequester(c.UserContext(), requester, id, m)
	}

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil})
}

func UpdateAppRelease(c *fiber.Ctx) error {
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

	id := uuid.FromStringOrNil(c.Params("id"))
	if id.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no release id", "data": nil})
	}

	// Parse batch request metadata from the request
	m := model.ReleaseUpdateMetadata{}
	err = c.BodyParser(&m)
	if err != nil {
		logrus.Errorf("failed to parse %s @ %s: %v", model.ReleaseSingular, reflect.FunctionName(), err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "error parse body", "data": nil})
	}

	//endregion
	if requester.IsAdmin || requester.IsInternal {
		err = model.UpdateReleaseForAdmin(c.UserContext(), id, m)
	} else {
		err = model.UpdateReleaseForRequester(c.UserContext(), requester, id, m)
	}

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil})
}

func GetApp(c *fiber.Ctx) error {
	// Get requester
	requester, err := helper.GetRequester(c)

	//if err != nil || requester == nil {
	//	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	//}

	// Check if requester is banned
	if requester != nil && requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	// Parse batch request metadata from the request
	m := model.IdRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}
	m.Id = uuid.FromStringOrNil(c.Params("id"))

	var (
		id uuid.UUID
	)

	if !m.Id.IsNil() {
		id = m.Id
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no id", "data": nil})
	}

	var entity *model.AppWithRelease
	var emptyUser = sm.User{}
	emptyUser.Id = uuid.Nil
	entity, err = model.GetAppForRequester(c.UserContext(), &emptyUser, id)

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if entity == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "not found", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": entity})
}

func GetAppIdentityImages(c *fiber.Ctx) error {
	// Parse batch request metadata from the request
	m := sm.GetAppLogoRequest{
		Id: c.Params("id"),
	}

	files, err := sm.GetAppIdentityFiles(c.UserContext(), m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "failed to get identity images", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": files})
}

func GetAppPublic(c *fiber.Ctx) error {
	// Get requester

	// Parse batch request metadata from the request
	m := struct {
		model.IdRequestMetadata
		Platform string `query:"platform"`
	}{}
	err := c.QueryParser(&m)
	if err != nil {
		logrus.Errorf("failed to parse app @ %s: %v", reflect.FunctionName(), err)
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
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "no id",
			"data":    nil,
		})
	}

	var user = &sm.User{}

	app, err := sm.GetAppV2(c.UserContext(), user, id, m.Platform)
	if err != nil {
		logrus.Errorf("failed to get app @ %s: %v", reflect.FunctionName(), err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "failed to get app",
			"data":    nil,
		})
	}

	if app == nil {
		logrus.Errorf("app not found @ %s: %v", reflect.FunctionName(), err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"status":  "error",
			"message": "not found",
			"data":    nil,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": app})
}

func GetAppForReleaseManager(c *fiber.Ctx) error {
	// Get requester
	requester, err := helper.GetRequester(c)

	// Check if requester is banned
	if requester != nil && requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	if requester == nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Parse batch request metadata from the request
	m := model.IdRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}
	m.Id = uuid.FromStringOrNil(c.Params("id"))

	var (
		id uuid.UUID
	)

	if !m.Id.IsNil() {
		id = m.Id
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no id", "data": nil})
	}

	var entity *model.AppWithRelease
	if requester.IsAdmin || requester.IsInternal {
		entity, err = model.GetAppForAdmin(c.UserContext(), id)
	} else {
		entity, err = model.GetAppForRequester(c.UserContext(), requester, id)
	}

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if entity == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "not found", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": entity})
}

func GetLatestRelease(c *fiber.Ctx) error {
	//region Requester

	// Get requester
	requester, err := helper.GetRequester(c)

	// Check if requester is banned
	if requester != nil && requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	//region Request metadata

	// Parse batch request metadata from the request
	m := model.ReleaseRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	m.Id = uuid.FromStringOrNil(c.Params("id"))

	var (
		id            uuid.UUID // app id
		platform      = ""
		deployment    = ""
		configuration = ""
	)

	if !m.Id.IsNil() {
		id = m.Id
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no id", "data": nil})
	}

	if model.SupportedPlatform[m.Platform] {
		platform = m.Platform
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no platform", "data": nil})
	}

	if model.SupportedDeployment[m.Deployment] {
		deployment = m.Deployment
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no deployment", "data": nil})
	}

	if model.SupportedConfiguration[m.Configuration] {
		configuration = m.Configuration
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no configuration", "data": nil})
	}
	//endregion

	var (
		entity *model.Release
	)

	if requester != nil {
		if requester.IsAdmin || requester.IsInternal {
			entity, err = model.GetLatestReleaseForAdmin(c.UserContext(), id, platform, deployment, configuration)
		} else {
			entity, err = model.GetLatestReleaseForRequester(c.UserContext(), requester, id, platform, deployment, configuration)
		}
	} else {
		var emptyUser = sm.User{}
		emptyUser.Id = uuid.Nil
		entity, err = model.GetLatestReleaseForRequester(c.UserContext(), &emptyUser, id, platform, deployment, configuration)
	}

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if entity == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "not found", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": entity})
}

func GetLatestLauncher(c *fiber.Ctx) error {
	//region Requester

	// Get requester
	requester, err := helper.GetRequester(c)

	// Check if requester is banned
	if requester != nil && requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	//region Request metadata

	// Parse batch request metadata from the request
	m := model.LauncherRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	m.Id = uuid.FromStringOrNil(c.Params("id"))

	var (
		id            uuid.UUID // app id
		platform      = ""
		deployment    = ""
		configuration = ""
	)

	if !m.Id.IsNil() {
		id = m.Id
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no id", "data": nil})
	}

	if model.SupportedPlatform[m.Platform] {
		platform = m.Platform
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no platform", "data": nil})
	}

	if model.SupportedDeployment[m.Deployment] {
		deployment = m.Deployment
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no deployment", "data": nil})
	}

	if model.SupportedConfiguration[m.Configuration] {
		configuration = m.Configuration
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no configuration", "data": nil})
	}
	//endregion

	var (
		entity *model.Launcher
	)

	if requester != nil {
		if requester.IsAdmin || requester.IsInternal {
			entity, err = model.GetLatestLauncherForAdmin(c.UserContext(), id, platform, deployment, configuration)
		} else {
			entity, err = model.GetLatestLauncherForRequester(c.UserContext(), requester, id, platform, deployment, configuration)
		}
	} else {
		var emptyUser = sm.User{}
		emptyUser.Id = uuid.Nil
		entity, err = model.GetLatestLauncherForRequester(c.UserContext(), &emptyUser, id, platform, deployment, configuration)
	}

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if entity == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "not found", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": entity})
}

func IndexAppReleases(c *fiber.Ctx) error {
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
	m := model.ReleaseBatchRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	m.Id = uuid.FromStringOrNil(c.Params("id"))

	var (
		offset int64 = 0
		limit  int64 = 100
	)

	if m.Offset > 0 {
		offset = m.Offset
	}

	if m.Limit > 0 && m.Limit < 100 {
		limit = m.Limit
	}

	//endregion

	var (
		entities []model.Release
		total    int64
	)

	if requester.IsAdmin || requester.IsInternal {
		entities, total, err = model.IndexReleasesForAdmin(c.UserContext(), m.Id, offset, limit)
	} else {
		entities, total, err = model.IndexReleasesForAdmin(c.UserContext(), m.Id, offset, limit)
	}

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fiber.Map{"offset": offset, "limit": limit, "total": total, "entities": entities}})
}

func GetUnclaimedJob(c *fiber.Ctx) error {
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

	if !requester.IsInternal {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "forbidden", "data": nil})
	}

	//endregion

	//region Request metadata

	// Parse batch request metadata from the request
	m := model.JobRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if m.Platform == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no platform", "data": nil})
	}

	platforms := strings.Split(m.Platform, ",")

	var supportedPlatforms []string
	for _, platform := range platforms {
		if model.SupportedPlatform[platform] {
			supportedPlatforms = append(supportedPlatforms, platform)
		}
	}

	var supportedTypes []string
	if len(m.Type) == 0 {
		supportedTypes = []string{"Launcher", "Release", "Package"}
	} else {
		types := strings.Split(m.Type, ",")
		for _, t := range types {
			if model.SupportedJobTypes[t] {
				supportedTypes = append(supportedTypes, t)
			}
		}
	}

	var supportedDeployments []string
	if len(m.Deployment) == 0 {
		supportedDeployments = []string{"Client", "Server"}
	} else {
		deployments := strings.Split(m.Deployment, ",")
		for _, deployment := range deployments {
			if model.SupportedJobDeployments[deployment] {
				supportedDeployments = append(supportedDeployments, deployment)
			}
		}
	}

	//endregion

	var (
		entity *model.Job
	)

	if requester.IsAdmin || requester.IsInternal {
		entity, err = model.GetUnclaimedJob(c.UserContext(), requester.Id, supportedPlatforms, supportedTypes, supportedDeployments)
	} else {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "forbidden", "data": nil})
	}

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if entity == nil {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "no jobs", "message": "no unclaimed jobs", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": entity})
}

func UpdateJobStatus(c *fiber.Ctx) error {
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

	if !requester.IsInternal {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "forbidden", "data": nil})
	}

	//endregion

	//region Request metadata

	// Parse batch request metadata from the request
	m := model.JobStatusRequestMetadata{}
	err = c.BodyParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	// Job ID
	m.Id = uuid.FromStringOrNil(c.Params("id"))

	var (
		id uuid.UUID // app id
	)

	if !m.Id.IsNil() {
		id = m.Id
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no id", "data": nil})
	}

	if m.Status == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no status", "data": nil})
	}

	if !model.SupportedJobStatuses[m.Status] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid job status", "data": nil})
	}

	//endregion

	if requester.IsAdmin || requester.IsInternal {
		err = model.UpdateJobStatus(c, id, m.Status, m.Message)
	} else {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "forbidden", "data": nil})
	}

	if err != nil {
		if err.Error() == "no rows in result set" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "not found", "data": nil})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
}

func ReportJobLog(c *fiber.Ctx) error {
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

	if !requester.IsInternal {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "forbidden", "data": nil})
	}

	//endregion

	//region Request metadata

	// Parse batch request metadata from the request
	m := model.JobLogRequestMetadata{}
	err = c.BodyParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	// Job ID
	m.Id = uuid.FromStringOrNil(c.Params("id"))

	var (
		id uuid.UUID // app id
	)

	if !m.Id.IsNil() {
		id = m.Id
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no id", "data": nil})
	}

	if len(m.Warnings) == 0 && len(m.Errors) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no status", "data": nil})
	}

	//endregion

	if requester.IsAdmin || requester.IsInternal {
		err = model.ReportJobLogs(c, id, m.Warnings, m.Errors)
	} else {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "forbidden", "data": nil})
	}

	if err != nil {
		if err.Error() == "no rows in result set" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "not found", "data": nil})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
}
