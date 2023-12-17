package handler

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"golang.org/x/mod/semver"
	"veverse-api/database"
	"veverse-api/helper"
	"veverse-api/model"
)

func IndexJobs(c *fiber.Ctx) error {
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

	// Check if requester is admin
	if !requester.IsAdmin {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "requester is not admin", "data": nil})
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

	var queryArgs []interface{}
	q := `SELECT COUNT(*) FROM jobs WHERE owner_id = $1`
	queryArgs = append(queryArgs, requester.Id)

	if m.Status != nil {
		q += fmt.Sprintf(" AND status = '%s'", *m.Status)
	}

	if m.Type != nil {
		q += fmt.Sprintf(` AND type = '%s'`, *m.Type)
	}

	var row = db.QueryRow(c.UserContext(), q, queryArgs...)

	var total int32
	err = row.Scan(&total)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}
	//endregion

	if total == 0 {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fiber.Map{"jobs": []model.IndexJobs{}, "offset": offset, "limit": limit, "total": nil}})
	}

	q = `SELECT
    	b.id,
    	m.name pakName,
    	a.name appName,
    	a.description appDescription,
    	r.version releaseVersion,
    	b.status,
    	b.configuration,
    	b.platform,
    	m.map,
    	b.type,
    	b.deployment,
    	b.message,
    	b.created_at,
		b.updated_at
FROM jobs b
    LEFT JOIN mods m ON m.id = b.entity_id
    LEFT JOIN releases r ON r.id = b.entity_id
    LEFT JOIN apps a on a.id = r.app_id
	WHERE owner_id = $1`

	var rows pgx.Rows

	queryArgs = queryArgs[:0]
	queryArgs = append(queryArgs, requester.Id)

	if m.Status != nil {
		q += fmt.Sprintf(" AND status = '%s'", *m.Status)
	}

	if m.Type != nil {
		q += fmt.Sprintf(` AND type = '%s'`, *m.Type)
	}

	q += fmt.Sprintf("ORDER BY created_at DESC, updated_at DESC LIMIT %d OFFSET %d", limit, offset)
	rows, err = db.Query(c.UserContext(), q, queryArgs...)

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	defer rows.Close()

	var jobs []model.IndexJobs
	for rows.Next() {
		var job model.IndexJobs
		err = rows.Scan(
			&job.Id,
			&job.PackageName,
			&job.AppName,
			&job.AppDescription,
			&job.ReleaseVersion,
			&job.Status,
			&job.Configuration,
			&job.Platform,
			&job.Map,
			&job.Type,
			&job.Deployment,
			&job.Message,
			&job.CreatedAt,
			&job.UpdatedAt,
		)

		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}

		jobs = append(jobs, job)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fiber.Map{"jobs": jobs, "offset": offset, "limit": limit, "total": total}})
}

func CreateJob(c *fiber.Ctx) error {
	//db := database.DB

	// Get requester
	requester, err := helper.GetRequester(c)
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

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
}

func CreatePackageJobs(c *fiber.Ctx) error {
	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	// Check if requester is admin
	//if !requester.IsAdmin {
	//	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "requester is not admin", "data": nil})
	//}

	m := model.EntityIdentifier{}
	err = c.BodyParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if m.EntityId == nil || m.EntityId.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no id", "data": nil})
	}

	err = model.CreatePackageJobs(c, requester, *m.EntityId)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
}

func PublishReleaseForAllApps(c *fiber.Ctx) error {
	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	// Check if requester is admin
	if !requester.IsAdmin || !requester.IsInternal {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "requester has no access", "data": nil})
	}

	m := model.ReleaseVersion{}
	err = c.BodyParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if semver.IsValid(m.CodeVersion) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no id", "data": nil})
	}

	err = model.PublishCodeReleaseForAllApps(c, requester, m.CodeVersion, m.ContentVersion)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
}

func RescheduleJob(c *fiber.Ctx) error {
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

	// Check if requester is banned
	if !requester.IsAdmin {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "requester is not admin", "data": nil})
	}

	// Parse batch request metadata from the request
	m := model.ScheduleRequestMetadata{}
	err = c.BodyParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	q := `UPDATE jobs SET status = 'pending' WHERE owner_id=$1 AND id=$2 AND status !='pending' AND status != 'authenticated' AND status != 'processing' AND status != 'uploading'`

	row := db.QueryRow(c.UserContext(), q, requester.Id, m.Id)

	if err = row.Scan(); err != nil {
		if err.Error() != "no rows in result set" {
			fmt.Printf(err.Error())
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "something went wrong", "data": nil})
		}
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "data": nil})
}

func CancelJob(c *fiber.Ctx) error {
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

	// Check if requester is banned
	if !requester.IsAdmin {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "requester is not admin", "data": nil})
	}

	// Parse batch request metadata from the request
	m := model.ScheduleRequestMetadata{}
	err = c.BodyParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var id = m.Id
	if id.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid id", "data": nil})
	}

	q := `UPDATE jobs SET status = 'cancel' WHERE owner_id = $1 AND id=$2 AND status ='pending'`

	row := db.QueryRow(c.UserContext(), q, requester.Id, id)

	if err = row.Scan(); err != nil {
		if err.Error() != "no rows in result set" {
			fmt.Printf(err.Error())
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "something went wrong", "data": nil})
		}
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "data": nil})
}
