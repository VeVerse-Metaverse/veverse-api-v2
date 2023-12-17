package handler

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"
	"path/filepath"
	"strings"
	"time"
	"veverse-api/aws/s3"
	"veverse-api/database"
	"veverse-api/helper"
	"veverse-api/model"
)

func IndexFiles(c *fiber.Ctx) error {
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
	m := model.FileBatchRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var (
		offset     int64 = 0
		limit      int64 = 100
		fileType         = ""
		platform         = ""
		deployment       = ""
		entityId   uuid.UUID
	)

	entityId = uuid.FromStringOrNil(c.Params("id"))

	if m.Offset > 0 {
		offset = m.Offset
	}

	if m.Limit > 0 && m.Limit < 100 {
		limit = m.Limit
	}

	if m.Type != "" {
		fileType = m.Type
	}

	if model.SupportedPlatform[m.Platform] {
		platform = m.Platform
	}

	if model.SupportedDeployment[m.Deployment] {
		deployment = m.Deployment
	}

	//endregion

	var (
		entities []model.File
		total    int64
	)

	if requester.IsAdmin || requester.IsInternal {
		if fileType == "" {
			if platform == "" {
				if deployment == "" {
					entities, total, err = model.IndexFilesForAdmin(c.UserContext(), entityId, offset, limit)
				} else {
					entities, total, err = model.IndexFilesForAdminForDeployment(c.UserContext(), entityId, offset, limit, deployment)
				}
			} else {
				if deployment == "" {
					entities, total, err = model.IndexFilesForAdminForPlatform(c.UserContext(), entityId, offset, limit, platform)
				} else {
					entities, total, err = model.IndexFilesForAdminForPlatformForDeployment(c.UserContext(), entityId, offset, limit, platform, deployment)
				}
			}
		} else {
			if platform == "" {
				if deployment == "" {
					entities, total, err = model.IndexFilesForAdminForType(c.UserContext(), entityId, offset, limit, fileType)
				} else {
					entities, total, err = model.IndexFilesForAdminForTypeForDeployment(c.UserContext(), entityId, offset, limit, fileType, deployment)
				}
			} else {
				if deployment == "" {
					entities, total, err = model.IndexFilesForAdminForTypeForPlatform(c.UserContext(), entityId, offset, limit, fileType, platform)
				} else {
					entities, total, err = model.IndexFilesForAdminForTypeForPlatformForDeployment(c.UserContext(), entityId, offset, limit, fileType, platform, deployment)
				}
			}
		}
	} else {
		if fileType == "" {
			if platform == "" {
				if deployment == "" {
					entities, total, err = model.IndexFilesForRequester(c.UserContext(), requester, entityId, offset, limit)
				} else {
					entities, total, err = model.IndexFilesForRequesterForDeployment(c.UserContext(), requester, entityId, offset, limit, deployment)
				}
			} else {
				if deployment == "" {
					entities, total, err = model.IndexFilesForRequesterForPlatform(c.UserContext(), requester, entityId, offset, limit, platform)
				} else {
					entities, total, err = model.IndexFilesForRequesterForPlatformForDeployment(c.UserContext(), requester, entityId, offset, limit, platform, deployment)
				}
			}
		} else {
			if platform == "" {
				if deployment == "" {
					entities, total, err = model.IndexFilesForRequesterForType(c.UserContext(), requester, entityId, offset, limit, fileType)
				} else {
					entities, total, err = model.IndexFilesForRequesterForTypeForDeployment(c.UserContext(), requester, entityId, offset, limit, fileType, deployment)
				}
			} else {
				if deployment == "" {
					entities, total, err = model.IndexFilesForRequesterForTypeForPlatform(c.UserContext(), requester, entityId, offset, limit, fileType, platform)
				} else {
					entities, total, err = model.IndexFilesForRequesterForTypeForPlatformForDeployment(c.UserContext(), requester, entityId, offset, limit, fileType, platform, deployment)
				}
			}
		}
	}

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fiber.Map{"entities": entities, "offset": offset, "limit": limit, "total": total}})
}

func UploadFile(c *fiber.Ctx) error {
	database.LogPgxStat("uploadFile before")
	//region Requester

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		logrus.Warningf("%d: no requester", fiber.StatusForbidden)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		logrus.Warningf("%d: banned", fiber.StatusForbidden)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	//region Request metadata
	m := model.IdRequestMetadata{}
	if err = c.ParamsParser(&m); err != nil {
		logrus.Warningf("%d: failed to parse params: %v", fiber.StatusBadRequest, err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var (
		entityId uuid.UUID
	)

	if m.Id.IsNil() {
		logrus.Warningf("%d: invalid entity id", fiber.StatusBadRequest)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid entity id", "data": nil})
	} else {
		entityId = m.Id
	}

	metadata := model.FileUploadRequestMetadata{OriginalPath: ""}
	if err = c.QueryParser(&metadata); err != nil {
		logrus.Warningf("%d: failed to parse query: %v", fiber.StatusBadRequest, err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}
	//endregion

	var (
		fileId uuid.UUID
		file   *model.File
	)

	if metadata.Type == "" {
		logrus.Warningf("%d: no file type", fiber.StatusBadRequest)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no file type", "data": nil})
	}

	// Process special image case that requires texture and preview generation
	if metadata.Type == "image_full_initial" {
		// Handle special case for image_full_initial type that requires image_full, image_preview and texture_diffuse to be stored

		// Try to upload a preview image
		if requester.IsAdmin || requester.IsInternal {
			fileId, err = model.UploadResizedImageForAdmin(c, requester, entityId, metadata, model.ImagePreview)
		} else {
			fileId, err = model.UploadResizedImageForRequester(c, requester, entityId, metadata, model.ImagePreview)
		}

		if err != nil {
			if err.Error() == "no rows in result set" {
				logrus.Warningf("%d: failed to upload preview image: entity %s not found", fiber.StatusNotFound, entityId.String())
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "not found", "data": nil})
			} else if err.Error() == "no access" {
				logrus.Warningf("%d: failed to upload preview image: forbidden, requester %s can't upload files for entity %s", fiber.StatusForbidden, requester.Id.String(), entityId.String())
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "forbidden", "data": nil})
			}
			logrus.Warningf("%d: failed to upload preview image: %v", fiber.StatusBadRequest, err.Error())
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}

		if requester.IsAdmin || requester.IsInternal {
			fileId, err = model.UploadResizedImageForAdmin(c, requester, entityId, metadata, model.ImageTexture)
		} else {
			fileId, err = model.UploadResizedImageForRequester(c, requester, entityId, metadata, model.ImageTexture)
		}

		if err != nil {
			if err.Error() == "no rows in result set" {
				logrus.Warningf("%d: failed to upload texture image: entity %s not found", fiber.StatusNotFound, entityId.String())
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "not found", "data": nil})
			} else if err.Error() == "no access" {
				logrus.Warningf("%d: failed to upload texture image: forbidden, requester %s can't upload files for entity %s", fiber.StatusForbidden, requester.Id.String(), entityId.String())
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "forbidden", "data": nil})
			}
			logrus.Warningf("%d: failed to upload texture image: %v", fiber.StatusBadRequest, err.Error())
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}

		metadata.Type = "image_full"
		if requester.IsAdmin || requester.IsInternal {
			fileId, err = model.UploadFileForAdmin(c, requester, entityId, metadata)
		} else {
			fileId, err = model.UploadFileForRequester(c, requester, entityId, metadata)
		}

		if err != nil {
			if err.Error() == "no rows in result set" {
				logrus.Warningf("%d: failed to upload full image: entity %s not found", fiber.StatusNotFound, entityId.String())
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "not found", "data": nil})
			} else if err.Error() == "no access" {
				logrus.Warningf("%d: failed to upload full image: forbidden, requester %s can't upload files for entity %s", fiber.StatusForbidden, requester.Id.String(), entityId.String())
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "forbidden", "data": nil})
			}
			logrus.Warningf("%d: failed to upload full image: %v", fiber.StatusBadRequest, err.Error())
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}

	} else {
		if requester.IsAdmin || requester.IsInternal {
			fileId, err = model.UploadFileForAdmin(c, requester, entityId, metadata)
		} else {
			fileId, err = model.UploadFileForRequester(c, requester, entityId, metadata)
		}

		if err != nil {
			if err.Error() == "no rows in result set" {
				logrus.Warningf("%d: failed to upload image: entity %s not found", fiber.StatusNotFound, entityId.String())
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "not found", "data": nil})
			} else if err.Error() == "no access" {
				logrus.Warningf("%d: failed to upload image: forbidden, requester %s can't upload files for entity %s", fiber.StatusForbidden, requester.Id.String(), entityId.String())
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "forbidden", "data": nil})
			}
			logrus.Warningf("%d: failed to upload image: %v", fiber.StatusBadRequest, err.Error())
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}
	}

	if requester.IsAdmin || requester.IsInternal {
		file, err = model.GetFileForAdmin(c.UserContext(), fileId)
	} else {
		file, err = model.GetFileForRequester(c.UserContext(), requester, fileId)
	}

	if err != nil {
		logrus.Warningf("%d: failed to get uploaded image: %v", fiber.StatusBadRequest, err.Error())
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	database.LogPgxStat("uploadFile after")
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": "ok", "data": file})
}

func LinkFile(c *fiber.Ctx) error {
	database.LogPgxStat("uploadFile after")
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
	m := model.IdRequestMetadata{}
	if err = c.ParamsParser(&m); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var (
		entityId uuid.UUID
	)

	if m.Id.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid entity id", "data": nil})
	} else {
		entityId = m.Id
	}

	metadata := model.FileLinkRequestMetadata{}
	if err = c.BodyParser(&metadata); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if metadata.Type == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no type", "data": nil})
	}

	if metadata.Url == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no url", "data": nil})
	}

	if !strings.HasPrefix(metadata.Url, "http") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": fmt.Sprintf("unsupported URL format: %s", metadata.Url), "data": nil})
	}

	//endregion

	if metadata.Type == "image_full_initial" {
		// Handle special case for image_full_initial type that requires image_full, image_preview and texture_diffuse to be stored

		metadata.Type = "image_full"
		if requester.IsAdmin || requester.IsInternal {
			err = model.LinkFileForAdmin(c.UserContext(), requester, entityId, metadata)
		} else {
			err = model.LinkFileForRequester(c.UserContext(), requester, entityId, metadata)
		}

		if err != nil {
			if err.Error() == "no rows in result set" {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "not found", "data": nil})
			} else if err.Error() == "no access" {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "forbidden", "data": nil})
			}
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}

		metadata.Type = "image_preview"
		if requester.IsAdmin || requester.IsInternal {
			err = model.LinkFileForAdmin(c.UserContext(), requester, entityId, metadata)
		} else {
			err = model.LinkFileForRequester(c.UserContext(), requester, entityId, metadata)
		}

		if err != nil {
			if err.Error() == "no rows in result set" {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "not found", "data": nil})
			} else if err.Error() == "no access" {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "forbidden", "data": nil})
			}
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}

		metadata.Type = "texture_diffuse"
		if requester.IsAdmin || requester.IsInternal {
			err = model.LinkFileForAdmin(c.UserContext(), requester, entityId, metadata)
		} else {
			err = model.LinkFileForRequester(c.UserContext(), requester, entityId, metadata)
		}

		if err != nil {
			if err.Error() == "no rows in result set" {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "not found", "data": nil})
			} else if err.Error() == "no access" {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "forbidden", "data": nil})
			}
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}
	} else {
		if requester.IsAdmin || requester.IsInternal {
			err = model.LinkFileForAdmin(c.UserContext(), requester, entityId, metadata)
		} else {
			err = model.LinkFileForRequester(c.UserContext(), requester, entityId, metadata)
		}

		if err != nil {
			if err.Error() == "no rows in result set" {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "not found", "data": nil})
			} else if err.Error() == "no access" {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "forbidden", "data": nil})
			}
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}
	}
	database.LogPgxStat("uploadFile after")
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": "ok", "data": nil})
}

func DeleteFile(c *fiber.Ctx) error {
	database.LogPgxStat("uploadFile after")
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
	m := model.IdRequestMetadata{}
	err = c.ParamsParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var (
		id uuid.UUID
	)

	if m.Id.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid id", "data": nil})
	} else {
		id = m.Id
	}
	//endregion

	if requester.IsAdmin || requester.IsInternal {
		err = model.DeleteFileForAdmin(c.UserContext(), id)
	} else {
		err = model.DeleteFileForRequester(c.UserContext(), requester, id)
	}

	if err != nil {
		if err.Error() == "no rows in result set" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "not found", "data": nil})
		} else if err.Error() == "no access" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "forbidden", "data": nil})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}
	database.LogPgxStat("uploadFile after")
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": "ok", "data": nil})
}

func GetFileDownloadLink(c *fiber.Ctx) error {
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

	m := model.FileDownloadRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	entityId := uuid.FromStringOrNil(m.EntityId)
	if entityId.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no entity id", "data": nil})
	}

	fileId := uuid.FromStringOrNil(m.FileId)
	if fileId.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no file id", "data": nil})
	}

	if requester.IsAdmin || requester.IsInternal {
		key := s3.GetS3KeyForEntityFile(entityId, fileId)
		url, err := s3.GetS3PresignedDownloadUrlForEntityFile(key, 30*time.Minute)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": url})
	}

	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"data": fiber.Map{"status": "error", "message": "forbidden", "data": nil}})
}

func GetFilePreSignedDownloadLink(c *fiber.Ctx) error {
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

	m := model.FileDownloadRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	entityId := uuid.FromStringOrNil(m.EntityId)
	if entityId.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no entity id", "data": nil})
	}

	fileId := uuid.FromStringOrNil(m.FileId)
	if fileId.IsNil() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no file id", "data": nil})
	}

	if requester.IsAdmin || requester.IsInternal {
		key := s3.GetS3KeyForEntityFile(entityId, fileId)
		url, err := s3.GetS3PresignedDownloadUrlForEntityFile(key, 72*time.Hour)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}
		c.Set("Location", url)
		return c.Status(fiber.StatusCreated).Send([]byte{})
	} else {
		file, err := model.GetFileForRequester(c.Context(), requester, fileId)
		if err == nil && file != nil {
			key := s3.GetS3KeyForEntityFile(entityId, fileId)
			url, err := s3.GetS3PresignedDownloadUrlForEntityFile(key, 72*time.Hour)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
			}
			c.Set("Location", url)
			return c.Status(fiber.StatusCreated).Send([]byte{})
		}
	}

	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"data": fiber.Map{"status": "error", "message": "forbidden", "data": nil}})
}

func GetFilePreSignedDownloadLinkByURL(c *fiber.Ctx) error {
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

	m := model.FileDownloadByURLRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if m.Url == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no url", "data": nil})
	}

	if requester.IsAdmin || requester.IsInternal {
		key := s3.GetS3KeyForEntityUrl(m.Url)
		url, err := s3.GetS3PresignedDownloadUrlForEntityFile(key, 72*time.Hour)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}
		c.Set("Location", url)
		return c.Status(fiber.StatusCreated).Send([]byte{})
	} else {
		fileId := uuid.FromStringOrNil(filepath.Base(m.Url))
		if fileId.IsNil() {
			file, err := model.GetFileForRequesterByUrl(c.Context(), requester, m.Url)
			if err == nil && file != nil {
				key := s3.GetS3KeyForEntityUrl(m.Url)
				url, err := s3.GetS3PresignedDownloadUrlForEntityFile(key, 72*time.Hour)
				if err != nil {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
				}
				c.Set("Location", url)
				return c.Status(fiber.StatusCreated).Send([]byte{})
			}
		} else {
			file, err := model.GetFileForRequester(c.Context(), requester, fileId)
			if err == nil && file != nil {
				key := s3.GetS3KeyForEntityUrl(m.Url)
				url, err := s3.GetS3PresignedDownloadUrlForEntityFile(key, 72*time.Hour)
				if err != nil {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
				}
				c.Set("Location", url)
				return c.Status(fiber.StatusCreated).Send([]byte{})
			}
		}
	}

	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"data": fiber.Map{"status": "error", "message": "forbidden", "data": nil}})
}

func GetFileUploadLink(c *fiber.Ctx) error {
	//region Requester

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		logrus.Warningf("%d: no requester", fiber.StatusForbidden)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		logrus.Warningf("%d: banned", fiber.StatusForbidden)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	m := model.FileUploadLinkRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		logrus.Warningf("%d: failed to parse query: %v", fiber.StatusForbidden, err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if m.Type != "uplugin_content" {
		logrus.Warningf("%d: file type not allowed: %s", fiber.StatusBadRequest, m.Type)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "file type not allowed", "data": nil})
	}

	entityId := uuid.FromStringOrNil(m.EntityId)
	if entityId.IsNil() {
		logrus.Warningf("%d: no entity id", fiber.StatusBadRequest)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no entity id", "data": nil})
	}

	var fileId uuid.UUID
	if m.FileId == nil {
		fileId, err = uuid.NewV4()
		if err != nil || fileId.IsNil() {
			logrus.Warningf("%d: no file id", fiber.StatusBadRequest)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no file id", "data": nil})
		}
	} else {
		fileId = uuid.FromStringOrNil(*m.FileId)
		if fileId.IsNil() {
			fileId, err = uuid.NewV4()
			if err != nil || fileId.IsNil() {
				logrus.Warningf("%d: failed to generate a new file id", fiber.StatusBadRequest)
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no file id", "data": nil})
			}
		}
	}

	//var public = true
	//if m.Type == "uplugin_content" || m.Type == "uplugin" {
	//	// Protect source files
	//	public = false
	//}

	//if requester.IsAdmin || requester.IsInternal {
	// Generate temporary pre-signed upload URL.
	key := s3.GetS3KeyForEntityFile(entityId, fileId)
	preSignedUploadUrl, err := s3.GetS3PresignedUploadUrlForEntityFile(key, 1*time.Minute)
	if err != nil {
		logrus.Warningf("%d: failed to get presigned upload url for %s: %v", fiber.StatusBadRequest, key, err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	// Generate static URL.
	url := s3.GetS3UrlForEntityFile(entityId, fileId)
	m.Url = &url

	var fileMetadata *model.File
	if requester.IsAdmin || requester.IsInternal {
		err = model.PreCreateFileForAdmin(c, requester, entityId, fileId, m)
		if err != nil {
			logrus.Warningf("%d: failed to pre-create file %s: %v", fiber.StatusInternalServerError, key, err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}
		fileMetadata, err = model.GetFileForAdmin(c.UserContext(), fileId)
		if err != nil {
			logrus.Warningf("%d: failed to get file %s: %v", fiber.StatusForbidden, key, err)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}
	} else {
		err = model.PreCreateFileForRequester(c, requester, entityId, fileId, m)
		if err != nil {
			logrus.Warningf("%d: failed to pre-create file %s: %v", fiber.StatusInternalServerError, key, err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}
		fileMetadata, err = model.GetFileForRequester(c.UserContext(), requester, fileId)
		if err != nil {
			logrus.Warningf("%d: failed to get file %s: %v", fiber.StatusInternalServerError, key, err)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}
	}

	fileMetadata.Url = preSignedUploadUrl

	if err != nil {
		if err.Error() == "no rows in result set" {
			logrus.Warningf("%d: failed to pre-create file %s: %v", fiber.StatusNotFound, key, err)
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "not found", "data": nil})
		} else if err.Error() == "no access" {
			logrus.Warningf("%d: failed to pre-create file %s: %v", fiber.StatusForbidden, key, err)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "forbidden", "data": nil})
		}
		logrus.Warningf("%d: failed to pre-create file %s: %v", fiber.StatusBadRequest, key, err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fileMetadata})
	//}

	//logrus.Warningf("%d: failed to pre-create file for non-admin", fiber.StatusForbidden)
	//return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"data": fiber.Map{"status": "error", "message": "forbidden", "data": nil}})
}
