package handler

import (
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"
	"veverse-api/database"
	"veverse-api/helper"
	"veverse-api/model"
	"veverse-api/reflect"
	"veverse-api/validation"
)

// GetEntity godoc
// @Summary Get entity
// @Description Get entity
// @Tags Entity
// @Accept json
// @Produce json
// @Param id path string true "Entity ID"
// @Security	 Bearer
// @Success 200 {object} model.Entity
// @Failure 400 {object} model.ErrorResponse
// @Failure 403 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /entity/{id} [get]
func GetEntity(c *fiber.Ctx) error {
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

	//region Request metadata
	m := model.IdRequestMetadata{}
	if err = c.ParamsParser(&m); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var entity *model.Entity

	if requester.IsAdmin {
		entity, err = model.GetEntityForAdmin(c.UserContext(), m.Id)
	} else {
		entity, err = model.GetEntityForRequester(c.UserContext(), requester.Id)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil, "data": fiber.Map{"entity": entity}})
}

// DeleteEntity godoc
// @Summary Delete entity
// @Description Delete entity
// @Tags Entity
// @Accept json
// @Produce json
// @Param id path string true "Entity ID"
// @Security	 Bearer
// @Success 200 {object} model.Entity
// @Failure 400 {object} model.ErrorResponse
// @Failure 403 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /entity/{id} [delete]
func DeleteEntity(c *fiber.Ctx) error {
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

	if !requester.IsActive {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "inactive", "data": nil})
	}

	//region Request metadata
	m := model.IdRequestMetadata{}
	if err = c.ParamsParser(&m); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if requester.IsAdmin || requester.IsInternal {
		err = model.DeleteEntityForAdmin(c.UserContext(), m.Id)
	} else {
		err = model.DeleteEntityForRequester(c.UserContext(), m.Id)
	}

	if err != nil {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(200).JSON(fiber.Map{"status": "ok", "message": "ok", "data": nil})
}

// GetTags godoc
// @Summary Get tags
// @Description Get tags
// @Tags Entity
// @Accept json
// @Produce json
// @Param id path string true "Entity ID"
// @Param limit query integer false "Limit"
// @Param offset query integer false "Offset"
// @Security	 Bearer
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} model.ErrorResponse
// @Failure 403 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /entity/{id}/tags [get]
func GetTags(c *fiber.Ctx) (err error) {
	//region Requester

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

	//region Request metadata
	id := uuid.FromStringOrNil(c.Params("id"))
	if id.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no id", "data": nil})
	}

	m := model.BatchRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var (
		offset int64 = 0
		limit  int64 = 100
		total  int32
		tags   []model.Tag
	)

	if m.Offset > 0 {
		offset = m.Offset
	}

	if m.Limit > 0 && m.Limit < 100 {
		limit = m.Limit
	}

	if requester.IsAdmin || requester.IsInternal {
		tags, total, err = model.GetTagsForAdmin(c.UserContext(), id, offset, limit)
	} else {
		tags, total, err = model.GetTagsForRequester(c.UserContext(), requester, id, offset, limit)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "failed fetch entity tags", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fiber.Map{"offset": offset, "limit": limit, "total": total, "tags": tags}})
}

// GetProperties godoc
// @Summary Get properties
// @Description Get properties
// @Tags Entity
// @Accept json
// @Produce json
// @Param id path string true "Entity ID"
// @Param limit query integer false "Limit"
// @Param offset query integer false "Offset"
// @Security	 Bearer
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} model.ErrorResponse
// @Failure 403 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /entity/{id}/properties [get]
func GetProperties(c *fiber.Ctx) (err error) {
	//region Requester

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

	//region Request metadata
	id := uuid.FromStringOrNil(c.Params("id"))
	if id.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no id", "data": nil})
	}

	m := model.BatchRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var (
		offset     int64 = 0
		limit      int64 = 100
		total      int32
		properties []model.Property
	)

	if m.Offset > 0 {
		offset = m.Offset
	}

	if m.Limit > 0 && m.Limit < 100 {
		limit = m.Limit
	}

	if requester.IsAdmin || requester.IsInternal {
		properties, total, err = model.GetPropertiesForAdmin(c.UserContext(), id, offset, limit)
	} else {
		properties, total, err = model.GetPropertiesForRequester(c.UserContext(), requester, id, offset, limit)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "failed fetch entity properties", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fiber.Map{"offset": offset, "limit": limit, "total": total, "properties": properties}})
}

// GetRatings godoc
// @Summary Get ratings
// @Description Get ratings
// @Tags Entity
// @Accept json
// @Produce json
// @Param id path string true "Entity ID"
// @Param limit query integer false "Limit"
// @Param offset query integer false "Offset"
// @Security	 Bearer
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} model.ErrorResponse
// @Failure 403 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /entity/{id}/ratings [get]
func GetRatings(c *fiber.Ctx) (err error) {
	//region Requester

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

	//region Request metadata
	id := uuid.FromStringOrNil(c.Params("id"))
	if id.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no id", "data": nil})
	}

	m := model.BatchRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var (
		offset  int64 = 0
		limit   int64 = 100
		total   int32
		ratings []model.Likable
	)

	if m.Offset > 0 {
		offset = m.Offset
	}

	if m.Limit > 0 && m.Limit < 100 {
		limit = m.Limit
	}

	if requester.IsAdmin || requester.IsInternal {
		ratings, total, err = model.GetRatingsForAdmin(c.UserContext(), id, offset, limit)
	} else {
		ratings, total, err = model.GetRatingsForRequester(c.UserContext(), requester, id, offset, limit)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "failed fetch entity properties", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fiber.Map{"offset": offset, "limit": limit, "total": total, "ratings": ratings}})
}

// GetComments godoc
// @Summary Get comments
// @Description Get comments
// @Tags Entity
// @Accept json
// @Produce json
// @Param id path string true "Entity ID"
// @Param limit query integer false "Limit"
// @Param offset query integer false "Offset"
// @Security	 Bearer
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} model.ErrorResponse
// @Failure 403 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /entity/{id}/comments [get]
func GetComments(c *fiber.Ctx) (err error) {
	//region Requester

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

	//region Request metadata
	id := uuid.FromStringOrNil(c.Params("id"))
	if id.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no id", "data": nil})
	}

	m := model.BatchRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var (
		offset   int64 = 0
		limit    int64 = 100
		total    int32
		comments []model.Comment
	)

	if m.Offset > 0 {
		offset = m.Offset
	}

	if m.Limit > 0 && m.Limit < 100 {
		limit = m.Limit
	}

	if requester.IsAdmin || requester.IsInternal {
		comments, total, err = model.GetCommentsForAdmin(c.UserContext(), id, offset, limit)
	} else {
		comments, total, err = model.GetCommentsForRequester(c.UserContext(), requester, id, offset, limit)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "failed fetch entity comments", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fiber.Map{"offset": offset, "limit": limit, "total": total, "comments": comments}})
}

// IncrementEntityView godoc
// @Summary Increment entity view
// @Description Increment entity view
// @Tags Entity
// @Accept json
// @Produce json
// @Param id path string true "Entity ID"
// @Security	 Bearer
// @Success 200 {object} model.Entity
// @Failure 400 {object} model.ErrorResponse
// @Failure 403 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /entity/{id}/view [post]
func IncrementEntityView(c *fiber.Ctx) error {
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
	//region Request metadata
	m := model.IdRequestMetadata{}
	if err = c.ParamsParser(&m); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if requester.IsAdmin || requester.IsInternal {
		err = model.IncrementViewEntityForAdmin(c.UserContext(), m.Id)
	} else {
		err = model.IncrementViewEntityForRequester(c.UserContext(), requester, m.Id)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": "ok", "data": nil})
}

// LikeEntity godoc
// @Summary Like entity
// @Description Like entity
// @Tags Entity
// @Accept json
// @Produce json
// @Param id path string true "Entity ID"
// @Security	 Bearer
// @Success 200 {object} model.Entity
// @Failure 400 {object} model.ErrorResponse
// @Failure 403 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /entity/{id}/like [post]
func LikeEntity(c *fiber.Ctx) (err error) {
	//region Requester

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

	// Check if requester is inactive
	if !requester.IsActive {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "inactive", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//region Request metadata
	id := uuid.FromStringOrNil(c.Params("id"))
	if id.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no id", "data": nil})
	}

	db := database.DB

	q := `SELECT e.id, e.public FROM entities e LEFT JOIN accessibles a ON e.id = a.entity_id WHERE id = $1`
	row := db.QueryRow(c.UserContext(), q, id)

	var entity model.Entity
	err = row.Scan(&entity.Id, &entity.Public)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", model.RatingSingular, reflect.FunctionName(), err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": fmt.Errorf("failed to set like"), "data": nil})
	}

	if isLikable := helper.CanLikable(c.UserContext(), requester, entity); !isLikable {
		logrus.Errorf("failed to scan %s @ %s: %v", model.RatingSingular, reflect.FunctionName(), err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": fmt.Errorf("failed to like entity"), "data": nil})
	}

	err = model.SetRating(c.UserContext(), requester, id, 1)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": fmt.Errorf("failed to like entity"), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": "ok", "data": nil})
}

// DislikeEntity godoc
// @Summary Dislike entity
// @Description Dislike entity
// @Tags Entity
// @Accept json
// @Produce json
// @Param id path string true "Entity ID"
// @Security	 Bearer
// @Success 200 {object} model.Entity
// @Failure 400 {object} model.ErrorResponse
// @Failure 403 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /entity/{id}/dislike [post]
func DislikeEntity(c *fiber.Ctx) (err error) {
	//region Requester

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

	// Check if requester is inactive
	if !requester.IsActive {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "inactive", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//region Request metadata
	id := uuid.FromStringOrNil(c.Params("id"))
	if id.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no id", "data": nil})
	}

	db := database.DB

	q := `SELECT e.id, e.public FROM entities e LEFT JOIN accessibles a ON e.id = a.entity_id WHERE id = $1`
	row := db.QueryRow(c.UserContext(), q, id)

	var entity model.Entity
	err = row.Scan(&entity.Id, &entity.Public)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", model.RatingSingular, reflect.FunctionName(), err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": fmt.Errorf("failed to set like"), "data": nil})
	}

	if isLikable := helper.CanLikable(c.UserContext(), requester, entity); !isLikable {
		logrus.Errorf("failed to scan %s @ %s: %v", model.RatingSingular, reflect.FunctionName(), err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": fmt.Errorf("failed to like entity"), "data": nil})
	}

	err = model.SetRating(c.UserContext(), requester, id, -1)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": fmt.Errorf("failed to like entity"), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": "ok", "data": nil})
}

// UnlikeEntity godoc
// @Summary Unlike entity
// @Description Unlike entity
// @Tags Entity
// @Accept json
// @Produce json
// @Param id path string true "Entity ID"
// @Security	 Bearer
// @Success 200 {object} model.Entity
// @Failure 400 {object} model.ErrorResponse
// @Failure 403 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /entity/{id}/unlike [post]
func UnlikeEntity(c *fiber.Ctx) (err error) {
	//region Requester

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

	// Check if requester is inactive
	if !requester.IsActive {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "inactive", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//region Request metadata
	id := uuid.FromStringOrNil(c.Params("id"))
	if id.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no id", "data": nil})
	}

	db := database.DB

	q := `SELECT e.id, e.public FROM entities e LEFT JOIN accessibles a ON e.id = a.entity_id WHERE id = $1`
	row := db.QueryRow(c.UserContext(), q, id)

	var entity model.Entity
	err = row.Scan(&entity.Id, &entity.Public)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", model.RatingSingular, reflect.FunctionName(), err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": fmt.Errorf("failed to set like"), "data": nil})
	}

	if isLikable := helper.CanLikable(c.UserContext(), requester, entity); !isLikable {
		logrus.Errorf("failed to scan %s @ %s: %v", model.RatingSingular, reflect.FunctionName(), err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": fmt.Errorf("failed to like entity"), "data": nil})
	}

	err = model.SetRating(c.UserContext(), requester, id, 0)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": fmt.Errorf("failed to like entity"), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": "ok", "data": nil})
}

// GetEntityAccess godoc
// @Summary Get entity access
// @Description Get entity access
// @Tags Entity
// @Accept json
// @Produce json
// @Param id path string true "Entity ID"
// @Security	 Bearer
// @Success 200 {object} model.Entity
// @Failure 400 {object} model.ErrorResponse
// @Failure 403 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /entity/{id}/access [get]
func GetEntityAccess(c *fiber.Ctx) error {
	//region

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

	m := model.BatchRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var (
		accessibles []model.Accessible
		offset      int64 = 0
		limit       int64 = 100
		total       int32
	)

	if m.Offset > 0 {
		offset = m.Offset
	}

	if m.Limit > 0 && m.Limit < 100 {
		limit = m.Limit
	}

	if requester.IsAdmin || requester.IsInternal {
		accessibles, total, err = model.GetAccessEntityForAdmin(c.UserContext(), requester, id, offset, limit)
	} else {
		accessibles, total, err = model.GetAccessEntityForRequester(c.UserContext(), requester, id, offset, limit)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "something went wrong", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fiber.Map{"entities": accessibles, "offset": offset, "limit": limit, "total": total}})
}

// UpdateEntityAccess godoc
// @Summary Update entity access
// @Description Update entity access
// @Tags Entity
// @Accept json
// @Produce json
// @Param id path string true "Entity ID"
// @Param body body model.EntityAccessRequest true "Entity access request"
// @Security	 Bearer
// @Success 200 {object} model.Entity
// @Failure 400 {object} model.ErrorResponse
// @Failure 403 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /entity/{id}/access [put]
func UpdateEntityAccess(c *fiber.Ctx) error {
	//region

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
	mReq := model.IdRequestMetadataWithValidation{}
	err = c.ParamsParser(&mReq)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	err = validation.Validator.Struct(mReq)
	if err != nil {
		errors := model.GetErrors(err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "validation error", "data": errors})
	}

	//region Patch Body metadata
	//Parse batch metadata from the patch body
	mBody := model.EntityAccessRequest{}
	err = c.BodyParser(&mBody)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	err = validation.Validator.Struct(mBody)
	if err != nil {
		errors := model.GetErrors(err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "validation error", "data": errors})
	}

	var id uuid.UUID
	id, err = uuid.FromString(mReq.Id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if requester.IsAdmin || requester.IsInternal {
		err = model.AccessEntityForAdmin(c.UserContext(), requester, id, mBody)
	} else {
		err = model.AccessEntityForRequester(c.UserContext(), requester, id, mBody)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "something went wrong", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "data": nil})
}

func UpdateEntityPublic(c *fiber.Ctx) error {
	//region

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
	mReq := model.IdRequestMetadataWithValidation{}
	err = c.ParamsParser(&mReq)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	err = validation.Validator.Struct(mReq)
	if err != nil {
		errors := model.GetErrors(err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "validation error", "data": errors})
	}

	//region Patch Body metadata
	//Parse batch metadata from the patch body
	mBody := model.EntityPublicAccessRequest{}
	err = c.BodyParser(&mBody)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	err = validation.Validator.Struct(mBody)
	if err != nil {
		errors := model.GetErrors(err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "validation error", "data": errors})
	}

	var id uuid.UUID
	id, err = uuid.FromString(mReq.Id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if requester.IsAdmin || requester.IsInternal {
		err = model.PublicAccessEntityForAdmin(c.UserContext(), requester, id, mBody)
	} else {
		err = model.PublicAccessEntityForRequester(c.UserContext(), requester, id, mBody)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "something went wrong", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "data": nil})
}
