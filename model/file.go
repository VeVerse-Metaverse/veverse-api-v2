package model

import (
	"bufio"
	"bytes"
	"context"
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	st "dev.hackerman.me/artheon/veverse-shared/telegram"
	"errors"
	"fmt"
	"github.com/gabriel-vasile/mimetype"
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"github.com/jackc/pgtype"
	pgtypeuuid "github.com/jackc/pgtype/ext/gofrs-uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nfnt/resize"
	"github.com/sirupsen/logrus"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"mime/multipart"
	"strconv"
	"strings"
	"time"
	"veverse-api/aws/s3"
	"veverse-api/database"
	"veverse-api/reflect"
)

var (
	avatarPlural = "avatars"
)

type UploadImageType int64

//goland:noinspection GoUnusedConst
const (
	ImageFull UploadImageType = iota
	ImagePreview
	ImageTexture
)

var PlatformDependentFileTypes = map[string]bool{
	"pak":                 true,
	"release":             true,
	"release-archive":     true,
	"release-archive-sdk": true,
}

// File trait for the Entity
type File struct {
	EntityTrait

	Type         string     `json:"type"`
	Url          string     `json:"url"`
	Mime         *string    `json:"mime,omitempty"`
	Size         *int64     `json:"size,omitempty"`
	Version      int        `json:"version,omitempty"`        // version of the file if versioned
	Deployment   string     `json:"deploymentType,omitempty"` // server or client if applicable
	Platform     string     `json:"platform,omitempty"`       // platform if applicable
	UploadedBy   *uuid.UUID `json:"uploadedBy,omitempty"`     // user that uploaded the file
	Width        *int       `json:"width,omitempty"`
	Height       *int       `json:"height,omitempty"`
	CreatedAt    time.Time  `json:"createdAt,omitempty"`
	UpdatedAt    *time.Time `json:"updatedAt,omitempty"`
	Index        int        `json:"variation,omitempty"`    // variant of the file if applicable (e.g. PDF pages)
	OriginalPath *string    `json:"originalPath,omitempty"` // original relative path to maintain directory structure (e.g. for releases)
	Hash         *string    `json:"hash,omitempty"`

	Timestamps
}

func containsFile(h []File, id uuid.UUID) bool {
	for _, v := range h {
		if *v.Id == id {
			return true
		}
	}
	return false
}

func containsLink(h []Link, id uuid.UUID) bool {
	for _, v := range h {
		if *v.Id == id {
			return true
		}
	}
	return false
}

// FileBatchRequestMetadata Batch request metadata for requesting File entities
type FileBatchRequestMetadata struct {
	BatchRequestMetadata
	Type       string `json:"type,omitempty" query:"type"`             // SupportedPlatform (OS) of the pak file (Win64, Mac, Linux, IOS, Android)
	Platform   string `json:"platform,omitempty" query:"platform"`     // SupportedPlatform (OS) of the pak file (Win64, Mac, Linux, IOS, Android)
	Deployment string `json:"deployment,omitempty" query:"deployment"` // SupportedDeployment for the pak file (Server or Client)
}

type FileRequestMetadata struct {
	PackageRequestMetadata
}

type FileLinkRequestMetadata struct {
	Type         string  `json:"type,omitempty" query:"type"`                            // Type of the file
	Url          string  `json:"url,omitempty" query:"url"`                              // Url of the file
	Mime         *string `json:"mime,omitempty" query:"mime"`                            // Mime type (optional), by default set to binary/octet-stream)
	Size         *int    `json:"size,omitempty" query:"size"`                            // Size of the file (optional)
	Version      int64   `json:"version,omitempty" query:"version"`                      // Version of the file, automatically incremented if the file is re-uploaded or re-linked, used to check if the file has been updated and should be re-downloaded even if it has been cached
	Deployment   string  `json:"deployment,omitempty" query:"deployment"`                // Deployment for the destination pak file (Server or Client), usually set for package and release files
	Platform     string  `json:"platform,omitempty" query:"platform"`                    // Platform (OS) of the destination pak file (Win64, Mac, Linux, IOS, Android), usually set for package and release files
	Width        int     `json:"width,omitempty" query:"width"`                          // Width of the media surface (optional), usually set for multimedia files (images and videos)
	Height       int     `json:"height,omitempty" query:"height"`                        // Height of the media surface (optional), usually set for multimedia files (images and videos)
	Index        int64   `json:"index,omitempty" query:"index"`                          // Index of the file (for file arrays such as PDF pages rendered to images)
	OriginalPath string  `json:"originalPath,omitempty" query:"original-path,omitempty"` // Original path of the file (to be re-downloaded to the correct location, used by app release files)
	Hash         string  `json:"hash,omitempty" query:"hash,omitempty"`                  // Hash of the file (to be re-downloaded to the correct location, used by app release files)
}

type FileUploadRequestMetadata struct {
	Type         string  `json:"type,omitempty" query:"type"`                            // Type of the file
	Mime         *string `json:"mime,omitempty" query:"mime"`                            // Url of the file
	Version      int64   `json:"version,omitempty" query:"version"`                      // Version of the file, automatically incremented if the file is re-uploaded or re-linked, used to check if the file has been updated and should be re-downloaded even if it has been cached
	Deployment   string  `json:"deployment,omitempty" query:"deployment"`                // Deployment for the destination pak file (Server or Client), usually set for package and release files
	Platform     string  `json:"platform,omitempty" query:"platform"`                    // Platform (OS) of the destination pak file (Win64, Mac, Linux, IOS, Android), usually set for package and release files
	Width        int     `json:"width,omitempty" query:"width"`                          // Width of the media surface (optional), usually set for multimedia files (images and videos)
	Height       int     `json:"height,omitempty" query:"height"`                        // Height of the media surface (optional), usually set for multimedia files (images and videos)
	Index        int64   `json:"index,omitempty" query:"index"`                          // Index of the file (for file arrays such as PDF pages rendered to images)
	OriginalPath string  `json:"originalPath,omitempty" query:"original-path,omitempty"` // Original path of the file (to be re-downloaded to the correct location, used by app release files)
	Hash         string  `json:"hash,omitempty" query:"hash"`                            // Hash of the file (optional)
}

type FileUploadLinkRequestMetadata struct {
	FileId       *string `json:"fileId,omitempty"`                                       // SupportedPlatform (OS) of the pak file (Win64, Mac, Linux, IOS, Android)
	EntityId     string  `json:"entityId,omitempty"`                                     // SupportedPlatform (OS) of the pak file (Win64, Mac, Linux, IOS, Android)
	Type         string  `json:"type,omitempty" query:"type"`                            // Type of the file
	Url          *string `json:"url,omitempty" query:"url"`                              // Url of the file
	Mime         *string `json:"mime,omitempty" query:"mime"`                            // Mime type (optional), by default set to binary/octet-stream)
	Size         *int    `json:"size,omitempty" query:"size"`                            // Size of the file (optional)
	Version      int64   `json:"version,omitempty" query:"version"`                      // Version of the file, automatically incremented if the file is re-uploaded or re-linked, used to check if the file has been updated and should be re-downloaded even if it has been cached
	Deployment   string  `json:"deployment,omitempty" query:"deployment"`                // Deployment for the destination pak file (Server or Client), usually set for package and release files
	Platform     string  `json:"platform,omitempty" query:"platform"`                    // Platform (OS) of the destination pak file (Win64, Mac, Linux, IOS, Android), usually set for package and release files
	Width        int     `json:"width,omitempty" query:"width"`                          // Width of the media surface (optional), usually set for multimedia files (images and videos)
	Height       int     `json:"height,omitempty" query:"height"`                        // Height of the media surface (optional), usually set for multimedia files (images and videos)
	Index        int64   `json:"index,omitempty" query:"index"`                          // Index of the file (for file arrays such as PDF pages rendered to images)
	OriginalPath string  `json:"originalPath,omitempty" query:"original-path,omitempty"` // Original path of the file (to be re-downloaded to the correct location, used by app release files)
}

type FileDownloadByURLRequestMetadata struct {
	Url string `json:"url,omitempty"`
}

type FileDownloadRequestMetadata struct {
	EntityId string `json:"entityId,omitempty"` // SupportedPlatform (OS) of the pak file (Win64, Mac, Linux, IOS, Android)
	FileId   string `json:"fileId,omitempty"`   // SupportedPlatform (OS) of the pak file (Win64, Mac, Linux, IOS, Android)
}

func uintPow(x, n uint) uint {
	if n == 0 {
		return 1
	}
	if n == 1 {
		return x
	}
	y := uintPow(x, n/2)
	if n%2 == 0 {
		return y * y
	}
	return x * y * y
}

func nextPowerOfTwo(x uint) uint {
	if x == 0 {
		return 1
	}

	return uintPow(2, uint(math.Ceil(math.Log2(float64(x)))))
}

// IndexFilesForAdmin Index packages for admin
func IndexFilesForAdmin(ctx context.Context, entityId uuid.UUID, offset int64, limit int64) (entities []File, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) FROM files f WHERE entity_id = $1`

	row := db.QueryRow(ctx, q, entityId /*$1*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	if total == 0 {
		return []File{}, total, nil
	}

	q = `SELECT
f.id fileId,
f.entity_id fileEntityId,
f.type fileType,
f.url fileUrl,
f.mime fileMime,
f.size fileSize,
f.version fileVersion,
f.deployment_type fileDeployment,
f.platform filePlatform,
f.uploaded_by fileUploadedBy,
f.width fileWidth,
f.height fileHeight,
f.created_at fileCreatedAt,
f.updated_at fileUpdatedAt,
f.variation fileIndex,
f.hash fileHash,
f.original_path fileOriginalPath
FROM files f
WHERE entity_id = $1
ORDER BY type, platform, deployment_type, version DESC, variation`

	var (
		rows pgx.Rows
		ei   int64 = 0
	)

	rows, err = db.Query(ctx, q, entityId)

	if err != nil {
		return []File{}, total, err
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexFilesForAdmin")
	}()

	for rows.Next() {
		var (
			id           pgtypeuuid.UUID
			eId          pgtypeuuid.UUID
			fileType     string
			url          string
			mime         *string
			size         *int64
			version      int
			deployment   string
			platform     string
			uploadedBy   pgtypeuuid.UUID
			width        *int
			height       *int
			createdAt    pgtype.Timestamp
			updatedAt    *pgtype.Timestamp
			index        int
			hash         *string
			originalPath *string
		)

		err = rows.Scan(
			&id,
			&eId,
			&fileType,
			&url,
			&mime,
			&size,
			&version,
			&deployment,
			&platform,
			&uploadedBy,
			&width,
			&height,
			&createdAt,
			&updatedAt,
			&index,
			&hash,
			&originalPath,
		)
		if err != nil {
			return nil, -1, err
		}

		if ei < offset {
			ei++
			continue
		}

		if ei-offset >= limit {
			break
		}

		var e File
		e.Id = &id.UUID
		e.EntityId = &eId.UUID
		e.Type = fileType
		e.Url = url
		e.Mime = mime
		e.Size = size
		e.Version = version
		e.Deployment = deployment
		e.Platform = platform
		e.Index = index
		e.Hash = hash
		e.OriginalPath = originalPath
		if uploadedBy.Status == pgtype.Present {
			e.UploadedBy = &uploadedBy.UUID
		}
		e.Width = width
		e.Height = height
		e.CreatedAt = createdAt.Time
		if updatedAt != nil {
			e.UpdatedAt = &updatedAt.Time
		}

		entities = append(entities, e)
		ei++
	}

	return entities, total, err
}

// IndexFilesForAdminForDeployment Index packages for admin
func IndexFilesForAdminForDeployment(ctx context.Context, entityId uuid.UUID, offset int64, limit int64, deployment string) (entities []File, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) FROM files f WHERE f.entity_id = $1 AND f.deployment_type = $2`

	row := db.QueryRow(ctx, q, entityId, deployment)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	if total == 0 {
		return []File{}, total, nil
	}

	q = `SELECT
f.id fileId,
f.entity_id fileEntityId,
f.type fileType,
f.url fileUrl,
f.mime fileMime,
f.size fileSize,
f.version fileVersion,
f.deployment_type fileDeployment,
f.platform filePlatform,
f.uploaded_by fileUploadedBy,
f.width fileWidth,
f.height fileHeight,
f.created_at fileCreatedAt,
f.updated_at fileUpdatedAt,
f.variation fileIndex,
f.hash fileHash,
f.original_path fileOriginalPath
FROM files f
	LEFT JOIN entities e on e.id = f.entity_id
WHERE f.entity_id = $1 AND f.deployment_type = $2
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows pgx.Rows
		ei   int64 = 0
	)

	rows, err = db.Query(ctx, q, entityId, deployment)

	if err != nil {
		return []File{}, total, err
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexFilesForAdminForDeployment")
	}()

	for rows.Next() {
		var (
			id           pgtypeuuid.UUID
			eId          pgtypeuuid.UUID
			fileType     string
			url          string
			mime         *string
			size         *int64
			version      int
			fDeployment  string
			platform     string
			uploadedBy   pgtypeuuid.UUID
			width        *int
			height       *int
			createdAt    pgtype.Timestamp
			updatedAt    *pgtype.Timestamp
			index        int
			hash         *string
			originalPath *string
		)

		err = rows.Scan(
			&id,
			&eId,
			&fileType,
			&url,
			&mime,
			&size,
			&version,
			&fDeployment,
			&platform,
			&uploadedBy,
			&width,
			&height,
			&createdAt,
			&updatedAt,
			&index,
			&hash,
			&originalPath,
		)
		if err != nil {
			return nil, -1, err
		}

		if ei < offset {
			ei++
			continue
		}

		if ei-offset >= limit {
			break
		}

		var e File
		e.Id = &id.UUID
		e.EntityId = &eId.UUID
		e.Type = fileType
		e.Url = url
		e.Mime = mime
		e.Size = size
		e.Version = version
		e.Deployment = fDeployment
		e.Platform = platform
		e.Index = index
		e.Hash = hash
		e.OriginalPath = originalPath
		if uploadedBy.Status == pgtype.Present {
			e.UploadedBy = &uploadedBy.UUID
		}
		e.Width = width
		e.Height = height
		e.CreatedAt = createdAt.Time
		if updatedAt != nil {
			e.UpdatedAt = &updatedAt.Time
		}

		entities = append(entities, e)
		ei++
	}

	return entities, total, err
}

// IndexFilesForAdminForPlatform Index packages for admin
func IndexFilesForAdminForPlatform(ctx context.Context, entityId uuid.UUID, offset int64, limit int64, platform string) (entities []File, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) FROM files f WHERE f.entity_id = $1 AND f.platform = $2`

	row := db.QueryRow(ctx, q, entityId, platform)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	if total == 0 {
		return []File{}, total, nil
	}

	q = `SELECT
f.id fileId,
f.entity_id fileEntityId,
f.type fileType,
f.url fileUrl,
f.mime fileMime,
f.size fileSize,
f.version fileVersion,
f.deployment_type fileDeployment,
f.platform filePlatform,
f.uploaded_by fileUploadedBy,
f.width fileWidth,
f.height fileHeight,
f.created_at fileCreatedAt,
f.updated_at fileUpdatedAt,
f.variation fileIndex,
f.hash fileHash,
f.original_path fileOriginalPath
FROM files f
	LEFT JOIN entities e on e.id = f.entity_id
WHERE f.entity_id = $1 AND f.platform = $2
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows pgx.Rows
		ei   int64 = 0
	)

	rows, err = db.Query(ctx, q, entityId, platform)

	if err != nil {
		return []File{}, total, err
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexFilesForAdminForPlatform")
	}()

	for rows.Next() {
		var (
			id           pgtypeuuid.UUID
			eId          pgtypeuuid.UUID
			fileType     string
			url          string
			mime         *string
			size         *int64
			version      int
			deployment   string
			fPlatform    string
			uploadedBy   pgtypeuuid.UUID
			width        *int
			height       *int
			createdAt    pgtype.Timestamp
			updatedAt    *pgtype.Timestamp
			index        int
			hash         *string
			originalPath *string
		)

		err = rows.Scan(
			&id,
			&eId,
			&fileType,
			&url,
			&mime,
			&size,
			&version,
			&deployment,
			&fPlatform,
			&uploadedBy,
			&width,
			&height,
			&createdAt,
			&updatedAt,
			&index,
			&hash,
			&originalPath,
		)
		if err != nil {
			return nil, -1, err
		}

		if ei < offset {
			ei++
			continue
		}

		if ei-offset >= limit {
			break
		}

		var e File
		e.Id = &id.UUID
		e.EntityId = &eId.UUID
		e.Type = fileType
		e.Url = url
		e.Mime = mime
		e.Size = size
		e.Version = version
		e.Deployment = deployment
		e.Platform = fPlatform
		e.Index = index
		e.Hash = hash
		e.OriginalPath = originalPath
		if uploadedBy.Status == pgtype.Present {
			e.UploadedBy = &uploadedBy.UUID
		}
		e.Width = width
		e.Height = height
		e.CreatedAt = createdAt.Time
		if updatedAt != nil {
			e.UpdatedAt = &updatedAt.Time
		}

		entities = append(entities, e)
		ei++
	}

	return entities, total, err
}

// IndexFilesForAdminForPlatformForDeployment Index packages for admin
func IndexFilesForAdminForPlatformForDeployment(ctx context.Context, entityId uuid.UUID, offset int64, limit int64, platform string, deployment string) (entities []File, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) FROM files f WHERE f.entity_id = $1 AND f.platform = $2 AND f.deployment_type = $3`

	row := db.QueryRow(ctx, q, entityId, platform, deployment)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	if total == 0 {
		return []File{}, total, nil
	}

	q = `SELECT
f.id fileId,
f.entity_id fileEntityId,
f.type fileType,
f.url fileUrl,
f.mime fileMime,
f.size fileSize,
f.version fileVersion,
f.deployment_type fileDeployment,
f.platform filePlatform,
f.uploaded_by fileUploadedBy,
f.width fileWidth,
f.height fileHeight,
f.created_at fileCreatedAt,
f.updated_at fileUpdatedAt,
f.variation fileIndex,
f.hash fileHash,
f.original_path fileOriginalPath
FROM files f
	LEFT JOIN entities e on e.id = f.entity_id
WHERE f.entity_id = $1 AND f.platform = $2 AND f.deployment_type = $3
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows pgx.Rows
		ei   int64 = 0
	)

	rows, err = db.Query(ctx, q, entityId, platform, deployment)

	if err != nil {
		return []File{}, total, err
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexFilesForAdminForPlatformForDeployment")
	}()

	for rows.Next() {
		var (
			id           pgtypeuuid.UUID
			eId          pgtypeuuid.UUID
			fileType     string
			url          string
			mime         *string
			size         *int64
			version      int
			fDeployment  string
			fPlatform    string
			uploadedBy   pgtypeuuid.UUID
			width        *int
			height       *int
			createdAt    pgtype.Timestamp
			updatedAt    *pgtype.Timestamp
			index        int
			hash         *string
			originalPath *string
		)

		err = rows.Scan(
			&id,
			&eId,
			&fileType,
			&url,
			&mime,
			&size,
			&version,
			&fDeployment,
			&fPlatform,
			&uploadedBy,
			&width,
			&height,
			&createdAt,
			&updatedAt,
			&index,
			&hash,
			&originalPath,
		)
		if err != nil {
			return nil, -1, err
		}

		if ei < offset {
			ei++
			continue
		}

		if ei-offset >= limit {
			break
		}

		var e File
		e.Id = &id.UUID
		e.EntityId = &eId.UUID
		e.Type = fileType
		e.Url = url
		e.Mime = mime
		e.Size = size
		e.Version = version
		e.Deployment = fDeployment
		e.Platform = fPlatform
		e.Index = index
		e.Hash = hash
		e.OriginalPath = originalPath
		if uploadedBy.Status == pgtype.Present {
			e.UploadedBy = &uploadedBy.UUID
		}
		e.Width = width
		e.Height = height
		e.CreatedAt = createdAt.Time
		if updatedAt != nil {
			e.UpdatedAt = &updatedAt.Time
		}

		entities = append(entities, e)
		ei++
	}

	return entities, total, err
}

// IndexFilesForAdminForType Index packages for admin
func IndexFilesForAdminForType(ctx context.Context, entityId uuid.UUID, offset int64, limit int64, fileType string) (entities []File, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) FROM files f WHERE f.entity_id = $1 AND f.type = $2`

	row := db.QueryRow(ctx, q, entityId, fileType)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	if total == 0 {
		return []File{}, total, nil
	}

	q = `SELECT
f.id fileId,
f.entity_id fileEntityId,
f.type fileType,
f.url fileUrl,
f.mime fileMime,
f.size fileSize,
f.version fileVersion,
f.deployment_type fileDeployment,
f.platform filePlatform,
f.uploaded_by fileUploadedBy,
f.width fileWidth,
f.height fileHeight,
f.created_at fileCreatedAt,
f.updated_at fileUpdatedAt,
f.variation fileIndex,
f.hash fileHash,
f.original_path fileOriginalPath
FROM files f
	LEFT JOIN entities e on e.id = f.entity_id
WHERE f.entity_id = $1 AND f.type = $2
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows pgx.Rows
		ei   int64 = 0
	)

	rows, err = db.Query(ctx, q, entityId, fileType)

	if err != nil {
		return []File{}, total, err
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexFilesForAdminForType")
	}()

	for rows.Next() {
		var (
			id           pgtypeuuid.UUID
			eId          pgtypeuuid.UUID
			fFileType    string
			url          string
			mime         *string
			size         *int64
			version      int
			deployment   string
			platform     string
			uploadedBy   pgtypeuuid.UUID
			width        *int
			height       *int
			createdAt    pgtype.Timestamp
			updatedAt    *pgtype.Timestamp
			index        int
			hash         *string
			originalPath *string
		)

		err = rows.Scan(
			&id,
			&eId,
			&fFileType,
			&url,
			&mime,
			&size,
			&version,
			&deployment,
			&platform,
			&uploadedBy,
			&width,
			&height,
			&createdAt,
			&updatedAt,
			&index,
			&hash,
			&originalPath,
		)
		if err != nil {
			return nil, -1, err
		}

		if ei < offset {
			ei++
			continue
		}

		if ei-offset >= limit {
			break
		}

		var e File
		e.Id = &id.UUID
		e.EntityId = &eId.UUID
		e.Type = fileType
		e.Url = url
		e.Mime = mime
		e.Size = size
		e.Version = version
		e.Deployment = deployment
		e.Platform = platform
		e.Index = index
		e.Hash = hash
		e.OriginalPath = originalPath
		if uploadedBy.Status == pgtype.Present {
			e.UploadedBy = &uploadedBy.UUID
		}
		e.Width = width
		e.Height = height
		e.CreatedAt = createdAt.Time
		if updatedAt != nil {
			e.UpdatedAt = &updatedAt.Time
		}

		entities = append(entities, e)
		ei++
	}

	return entities, total, err
}

// IndexFilesForAdminForTypeForDeployment Index packages for admin
func IndexFilesForAdminForTypeForDeployment(ctx context.Context, entityId uuid.UUID, offset int64, limit int64, fileType string, deployment string) (entities []File, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) FROM files f WHERE f.entity_id = $1 AND f.deployment_type = $2 AND f.type = $3`

	row := db.QueryRow(ctx, q, entityId, deployment, fileType)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	if total == 0 {
		return []File{}, total, nil
	}

	q = `SELECT
f.id fileId,
f.entity_id fileEntityId,
f.type fileType,
f.url fileUrl,
f.mime fileMime,
f.size fileSize,
f.version fileVersion,
f.deployment_type fileDeployment,
f.platform filePlatform,
f.uploaded_by fileUploadedBy,
f.width fileWidth,
f.height fileHeight,
f.created_at fileCreatedAt,
f.updated_at fileUpdatedAt,
f.variation fileIndex,
f.hash fileHash,
f.original_path fileOriginalPath
FROM files f
	LEFT JOIN entities e on e.id = f.entity_id
WHERE f.entity_id = $1 AND f.platform = $2 AND f.deployment_type = $3
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows pgx.Rows
		ei   int64 = 0
	)

	rows, err = db.Query(ctx, q)

	if err != nil {
		return []File{}, total, err
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexFilesForAdminForTypeForDeployment")
	}()

	for rows.Next() {
		var (
			id           pgtypeuuid.UUID
			eId          pgtypeuuid.UUID
			fFileType    string
			url          string
			mime         *string
			size         *int64
			version      int
			fDeployment  string
			fPlatform    string
			uploadedBy   pgtypeuuid.UUID
			width        *int
			height       *int
			createdAt    pgtype.Timestamp
			updatedAt    *pgtype.Timestamp
			index        int
			hash         *string
			originalPath *string
		)

		err = rows.Scan(
			&id,
			&eId,
			&fFileType,
			&url,
			&mime,
			&size,
			&version,
			&fDeployment,
			&fPlatform,
			&uploadedBy,
			&width,
			&height,
			&createdAt,
			&updatedAt,
			&index,
			&hash,
			&originalPath,
		)
		if err != nil {
			return nil, -1, err
		}

		if ei < offset {
			ei++
			continue
		}

		if ei-offset >= limit {
			break
		}

		var e File
		e.Id = &id.UUID
		e.EntityId = &eId.UUID
		e.Type = fFileType
		e.Url = url
		e.Mime = mime
		e.Size = size
		e.Version = version
		e.Deployment = fDeployment
		e.Platform = fPlatform
		e.Index = index
		e.Hash = hash
		e.OriginalPath = originalPath
		if uploadedBy.Status == pgtype.Present {
			e.UploadedBy = &uploadedBy.UUID
		}
		e.Width = width
		e.Height = height
		e.CreatedAt = createdAt.Time
		if updatedAt != nil {
			e.UpdatedAt = &updatedAt.Time
		}

		entities = append(entities, e)
		ei++
	}

	return entities, total, err
}

// IndexFilesForAdminForTypeForPlatform Index packages for admin
func IndexFilesForAdminForTypeForPlatform(ctx context.Context, entityId uuid.UUID, offset int64, limit int64, fileType string, platform string) (entities []File, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) FROM files f WHERE f.entity_id = $1 AND f.type = $2 AND f.platform = $3`

	row := db.QueryRow(ctx, q, entityId, fileType, platform)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	if total == 0 {
		return []File{}, total, nil
	}

	q = `SELECT
f.id fileId,
f.entity_id fileEntityId,
f.type fileType,
f.url fileUrl,
f.mime fileMime,
f.size fileSize,
f.version fileVersion,
f.deployment_type fileDeployment,
f.platform filePlatform,
f.uploaded_by fileUploadedBy,
f.width fileWidth,
f.height fileHeight,
f.created_at fileCreatedAt,
f.updated_at fileUpdatedAt,
f.variation fileIndex,
f.hash fileHash,
f.original_path fileOriginalPath
FROM files f
	LEFT JOIN entities e on e.id = f.entity_id
WHERE f.entity_id = $1 AND f.type = $2 AND f.platform = $3
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows pgx.Rows
		ei   int64 = 0
	)

	rows, err = db.Query(ctx, q, entityId, fileType, platform)

	if err != nil {
		return []File{}, total, err
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexFilesForAdminForTypeForPlatform")
	}()

	for rows.Next() {
		var (
			id           pgtypeuuid.UUID
			eId          pgtypeuuid.UUID
			fFileType    string
			url          string
			mime         *string
			size         *int64
			version      int
			fDeployment  string
			fPlatform    string
			uploadedBy   pgtypeuuid.UUID
			width        *int
			height       *int
			createdAt    pgtype.Timestamp
			updatedAt    *pgtype.Timestamp
			index        int
			hash         *string
			originalPath *string
		)

		err = rows.Scan(
			&id,
			&eId,
			&fFileType,
			&url,
			&mime,
			&size,
			&version,
			&fDeployment,
			&fPlatform,
			&uploadedBy,
			&width,
			&height,
			&createdAt,
			&updatedAt,
			&index,
			&hash,
			&originalPath,
		)
		if err != nil {
			return nil, -1, err
		}

		if ei < offset {
			ei++
			continue
		}

		if ei-offset >= limit {
			break
		}

		var e File
		e.Id = &id.UUID
		e.EntityId = &eId.UUID
		e.Type = fFileType
		e.Url = url
		e.Mime = mime
		e.Size = size
		e.Version = version
		e.Deployment = fDeployment
		e.Platform = fPlatform
		e.Index = index
		e.Hash = hash
		e.OriginalPath = originalPath
		if uploadedBy.Status == pgtype.Present {
			e.UploadedBy = &uploadedBy.UUID
		}
		e.Width = width
		e.Height = height
		e.CreatedAt = createdAt.Time
		if updatedAt != nil {
			e.UpdatedAt = &updatedAt.Time
		}
		entities = append(entities, e)
		ei++
	}

	return entities, total, err
}

// IndexFilesForAdminForTypeForPlatformForDeployment Index packages for admin
func IndexFilesForAdminForTypeForPlatformForDeployment(ctx context.Context, entityId uuid.UUID, offset int64, limit int64, fileType string, platform string, deployment string) (entities []File, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) FROM files f WHERE f.entity_id = $1 AND f.platform = $2 AND f.type = $3 AND f.deployment_type = $4`

	row := db.QueryRow(ctx, q, entityId, platform, fileType, deployment)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	if total == 0 {
		return []File{}, total, nil
	}

	q = `SELECT
f.id fileId,
f.entity_id fileEntityId,
f.type fileType,
f.url fileUrl,
f.mime fileMime,
f.size fileSize,
f.version fileVersion,
f.deployment_type fileDeployment,
f.platform filePlatform,
f.uploaded_by fileUploadedBy,
f.width fileWidth,
f.height fileHeight,
f.created_at fileCreatedAt,
f.updated_at fileUpdatedAt,
f.variation fileIndex,
f.hash fileHash,
f.original_path fileOriginalPath
FROM files f
	LEFT JOIN entities e on e.id = f.entity_id
	WHERE f.entity_id = $1 AND f.platform = $2 AND f.type = $3 AND f.deployment_type = $4
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows pgx.Rows
		ei   int64 = 0
	)

	rows, err = db.Query(ctx, q, entityId, platform, fileType, deployment)

	if err != nil {
		return []File{}, total, err
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexFilesForAdminForTypeForPlatformForDeployment")
	}()

	for rows.Next() {
		var (
			id           pgtypeuuid.UUID
			eId          pgtypeuuid.UUID
			fFileType    string
			url          string
			mime         *string
			size         *int64
			version      int
			fDeployment  string
			fPlatform    string
			uploadedBy   pgtypeuuid.UUID
			width        *int
			height       *int
			createdAt    pgtype.Timestamp
			updatedAt    *pgtype.Timestamp
			index        int
			hash         *string
			originalPath *string
		)

		err = rows.Scan(
			&id,
			&eId,
			&fFileType,
			&url,
			&mime,
			&size,
			&version,
			&fDeployment,
			&fPlatform,
			&uploadedBy,
			&width,
			&height,
			&createdAt,
			&updatedAt,
			&index,
			&hash,
			&originalPath,
		)
		if err != nil {
			return nil, -1, err
		}

		if ei < offset {
			ei++
			continue
		}

		if ei-offset >= limit {
			break
		}

		var e File
		e.Id = &id.UUID
		e.EntityId = &eId.UUID
		e.Type = fFileType
		e.Url = url
		e.Mime = mime
		e.Size = size
		e.Version = version
		e.Deployment = fDeployment
		e.Platform = fPlatform
		e.Index = index
		e.Hash = hash
		e.OriginalPath = originalPath
		if uploadedBy.Status == pgtype.Present {
			e.UploadedBy = &uploadedBy.UUID
		}
		e.Width = width
		e.Height = height
		e.CreatedAt = createdAt.Time
		if updatedAt != nil {
			e.UpdatedAt = &updatedAt.Time
		}

		entities = append(entities, e)
		ei++
	}

	return entities, total, err
}

// IndexFilesForRequester Index files for requester
func IndexFilesForRequester(ctx context.Context, requester *sm.User, entityId uuid.UUID, offset int64, limit int64) (entities []File, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) 
FROM files f 
    LEFT JOIN entities e ON e.id = f.entity_id
    LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid 
WHERE e.id = $2 AND (e.public OR a.can_view OR a.is_owner)`

	row := db.QueryRow(ctx, q, requester.Id /*$1*/, entityId /*$2*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	if total == 0 {
		return []File{}, total, nil
	}

	q = `SELECT
f.id fileId,
f.entity_id fileEntityId,
f.type fileType,
f.url fileUrl,
f.mime fileMime,
f.size fileSize,
f.version fileVersion,
f.deployment_type fileDeployment,
f.platform filePlatform,
f.uploaded_by fileUploadedBy,
f.width fileWidth,
f.height fileHeight,
f.created_at fileCreatedAt,
f.updated_at fileUpdatedAt,
f.variation fileIndex,
f.hash fileHash,
f.original_path fileOriginalPath
FROM files f
	LEFT JOIN entities e ON f.entity_id = e.id
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
WHERE f.entity_id = $2 AND (e.public OR a.can_view OR a.is_owner)
ORDER BY type, platform, deployment_type, version DESC, variation`

	var (
		rows pgx.Rows
		ei   int64 = 0
	)

	rows, err = db.Query(ctx, q, requester.Id, entityId)

	if err != nil {
		return []File{}, total, err
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexFilesForRequester")
	}()

	for rows.Next() {
		var (
			id         pgtypeuuid.UUID
			eId        pgtypeuuid.UUID
			fileType   string
			url        string
			mime       *string
			size       *int64
			version    int
			deployment string
			platform   string
			uploadedBy pgtypeuuid.UUID
			width      *int
			height     *int
			createdAt  pgtype.Timestamp
			updatedAt  *pgtype.Timestamp
			index      int
			hash       *string
			original   *string
		)

		err = rows.Scan(
			&id,
			&eId,
			&fileType,
			&url,
			&mime,
			&size,
			&version,
			&deployment,
			&platform,
			&uploadedBy,
			&width,
			&height,
			&createdAt,
			&updatedAt,
			&index,
			&hash,
			&original,
		)
		if err != nil {
			return nil, -1, err
		}

		var e File
		e.Id = &id.UUID
		e.EntityId = &eId.UUID
		e.Type = fileType
		e.Url = url
		e.Mime = mime
		e.Size = size
		e.Version = version
		e.Deployment = deployment
		e.Platform = platform
		e.Index = index
		e.Hash = hash
		e.OriginalPath = original
		if uploadedBy.Status == pgtype.Present {
			e.UploadedBy = &uploadedBy.UUID
		}
		e.Width = width
		e.Height = height
		e.CreatedAt = createdAt.Time
		if updatedAt != nil {
			e.UpdatedAt = &updatedAt.Time
		}

		if ei < offset {
			ei++
			continue
		}

		if ei-offset >= limit {
			break
		}

		entities = append(entities, e)
	}

	return entities, total, err
}

// IndexFilesForRequesterForDeployment Index packages for admin
func IndexFilesForRequesterForDeployment(ctx context.Context, requester *sm.User, entityId uuid.UUID, offset int64, limit int64, deployment string) (entities []File, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) FROM files f 
    LEFT JOIN entities e ON e.id = f.entity_id 
    LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid 
WHERE e.id = $2 AND (e.public OR a.can_view OR a.is_owner) AND f.deployment_type = $3`

	row := db.QueryRow(ctx, q, requester.Id /*$1*/, entityId /*$2*/, deployment)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	if total == 0 {
		return []File{}, total, nil
	}

	q = `SELECT
f.id fileId,
f.entity_id fileEntityId,
f.type fileType,
f.url fileUrl,
f.mime fileMime,
f.size fileSize,
f.version fileVersion,
f.deployment_type fileDeployment,
f.platform filePlatform,
f.uploaded_by fileUploadedBy,
f.width fileWidth,
f.height fileHeight,
f.created_at fileCreatedAt,
f.updated_at fileUpdatedAt,
f.variation fileIndex,
f.hash fileHash,
f.original_path fileOriginalPath
FROM files f
	LEFT JOIN entities e ON e.id = f.entity_id
    LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
WHERE e.id = $2 AND (e.public OR a.can_view OR a.is_owner) AND f.deployment_type = $3
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows pgx.Rows
		ei   int64 = 0
	)

	rows, err = db.Query(ctx, q, requester.Id, entityId, deployment)

	if err != nil {
		return []File{}, total, err
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexFilesForRequesterForDeployment")
	}()

	for rows.Next() {
		var (
			id          pgtypeuuid.UUID
			eId         pgtypeuuid.UUID
			fileType    string
			url         string
			mime        *string
			size        *int64
			version     int
			fDeployment string
			platform    string
			uploadedBy  pgtypeuuid.UUID
			width       *int
			height      *int
			createdAt   pgtype.Timestamp
			updatedAt   *pgtype.Timestamp
			index       int
			hash        *string
			original    *string
		)

		err = rows.Scan(
			&id,
			&eId,
			&fileType,
			&url,
			&mime,
			&size,
			&version,
			&fDeployment,
			&platform,
			&uploadedBy,
			&width,
			&height,
			&createdAt,
			&updatedAt,
			&index,
			&hash,
			&original,
		)
		if err != nil {
			return nil, -1, err
		}

		if ei < offset {
			ei++
			continue
		}

		if ei-offset >= limit {
			break
		}

		var e File
		e.Id = &id.UUID
		e.EntityId = &eId.UUID
		e.Type = fileType
		e.Url = url
		e.Mime = mime
		e.Size = size
		e.Version = version
		e.Deployment = fDeployment
		e.Platform = platform
		e.Index = index
		e.Hash = hash
		e.OriginalPath = original
		if uploadedBy.Status == pgtype.Present {
			e.UploadedBy = &uploadedBy.UUID
		}
		e.Width = width
		e.Height = height
		e.CreatedAt = createdAt.Time
		if updatedAt != nil {
			e.UpdatedAt = &updatedAt.Time
		}

		entities = append(entities, e)
		ei++
	}

	return entities, total, err
}

// IndexFilesForRequesterForPlatform Index packages for Requester
func IndexFilesForRequesterForPlatform(ctx context.Context, requester *sm.User, entityId uuid.UUID, offset int64, limit int64, platform string) (entities []File, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) FROM files f
LEFT JOIN entities e ON e.id = f.entity_id 
LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid 
WHERE e.id = $2 AND (e.public OR a.can_view OR a.is_owner) AND f.platform = $3`

	row := db.QueryRow(ctx, q, requester.Id /*$1*/, entityId /*$2*/, platform)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	if total == 0 {
		return []File{}, total, nil
	}

	q = `SELECT
f.id fileId,
f.entity_id fileEntityId,
f.type fileType,
f.url fileUrl,
f.mime fileMime,
f.size fileSize,
f.version fileVersion,
f.deployment_type fileDeployment,
f.platform filePlatform,
f.uploaded_by fileUploadedBy,
f.width fileWidth,
f.height fileHeight,
f.created_at fileCreatedAt,
f.updated_at fileUpdatedAt,
f.variation fileIndex,
f.hash fileHash,
f.original_path fileOriginalPath
FROM files f
	LEFT JOIN entities e on e.id = f.entity_id
    LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
WHERE f.entity_id = $2 AND (e.public OR a.can_view OR a.is_owner) AND f.platform = $3
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows pgx.Rows
		ei   int64 = 0
	)

	rows, err = db.Query(ctx, q, requester.Id, entityId, platform)

	if err != nil {
		return []File{}, total, err
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexFilesForRequesterForPlatform")
	}()

	for rows.Next() {
		var (
			id           pgtypeuuid.UUID
			eId          pgtypeuuid.UUID
			fileType     string
			url          string
			mime         *string
			size         *int64
			version      int
			fDeployment  string
			fPlatform    string
			uploadedBy   pgtypeuuid.UUID
			width        *int
			height       *int
			createdAt    pgtype.Timestamp
			updatedAt    *pgtype.Timestamp
			index        int
			hash         *string
			originalPath *string
		)

		err = rows.Scan(
			&id,
			&eId,
			&fileType,
			&url,
			&mime,
			&size,
			&version,
			&fDeployment,
			&fPlatform,
			&uploadedBy,
			&width,
			&height,
			&createdAt,
			&updatedAt,
			&index,
			&hash,
			&originalPath,
		)
		if err != nil {
			return nil, -1, err
		}

		if ei < offset {
			ei++
			continue
		}

		if ei-offset >= limit {
			break
		}

		var e File
		e.Id = &id.UUID
		e.EntityId = &eId.UUID
		e.Type = fileType
		e.Url = url
		e.Mime = mime
		e.Size = size
		e.Version = version
		e.Deployment = fDeployment
		e.Platform = fPlatform
		e.Index = index
		e.Hash = hash
		e.OriginalPath = originalPath
		if uploadedBy.Status == pgtype.Present {
			e.UploadedBy = &uploadedBy.UUID
		}
		e.Width = width
		e.Height = height
		e.CreatedAt = createdAt.Time
		if updatedAt != nil {
			e.UpdatedAt = &updatedAt.Time
		}

		entities = append(entities, e)
		ei++
	}

	return entities, total, err
}

// IndexFilesForRequesterForPlatformForDeployment Index packages for Requester
func IndexFilesForRequesterForPlatformForDeployment(ctx context.Context, requester *sm.User, entityId uuid.UUID, offset int64, limit int64, platform string, deployment string) (entities []File, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) FROM files f 
    LEFT JOIN entities e ON e.id = f.entity_id 
    LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid 
WHERE e.id = $2 AND (e.public OR a.can_view OR a.is_owner) AND f.platform = $3 AND f.deployment_type = $4`

	row := db.QueryRow(ctx, q, requester.Id /*$1*/, entityId /*$2*/, platform, deployment)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	if total == 0 {
		return []File{}, total, nil
	}

	q = `SELECT
f.id fileId,
f.entity_id fileEntityId,
f.type fileType,
f.url fileUrl,
f.mime fileMime,
f.size fileSize,
f.version fileVersion,
f.deployment_type fileDeployment,
f.platform filePlatform,
f.uploaded_by fileUploadedBy,
f.width fileWidth,
f.height fileHeight,
f.created_at fileCreatedAt,
f.updated_at fileUpdatedAt,
f.variation fileIndex,
f.hash fileHash,
f.original_path fileOriginalPath
FROM files f
	LEFT JOIN entities e on e.id = f.entity_id
    LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
WHERE f.entity_id = $2 AND (e.public OR a.can_view OR a.is_owner) AND f.platform = $3  AND f.deployment_type = $4
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows pgx.Rows
		ei   int64 = 0
	)

	rows, err = db.Query(ctx, q, requester.Id, entityId, platform, deployment)

	if err != nil {
		return []File{}, total, err
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexFilesForRequesterForPlatformForDeployment")
	}()

	for rows.Next() {
		var (
			id           pgtypeuuid.UUID
			eId          pgtypeuuid.UUID
			fileType     string
			url          string
			mime         *string
			size         *int64
			version      int
			fDeployment  string
			fPlatform    string
			uploadedBy   pgtypeuuid.UUID
			width        *int
			height       *int
			createdAt    pgtype.Timestamp
			updatedAt    *pgtype.Timestamp
			index        int
			hash         *string
			originalPath *string
		)

		err = rows.Scan(
			&id,
			&eId,
			&fileType,
			&url,
			&mime,
			&size,
			&version,
			&fDeployment,
			&fPlatform,
			&uploadedBy,
			&width,
			&height,
			&createdAt,
			&updatedAt,
			&index,
			&hash,
			&originalPath,
		)
		if err != nil {
			return nil, -1, err
		}

		if ei < offset {
			ei++
			continue
		}

		if ei-offset >= limit {
			break
		}

		var e File
		e.Id = &id.UUID
		e.EntityId = &eId.UUID
		e.Type = fileType
		e.Url = url
		e.Mime = mime
		e.Size = size
		e.Version = version
		e.Deployment = fDeployment
		e.Platform = fPlatform
		e.Index = index
		e.Hash = hash
		e.OriginalPath = originalPath
		if uploadedBy.Status == pgtype.Present {
			e.UploadedBy = &uploadedBy.UUID
		}
		e.Width = width
		e.Height = height
		e.CreatedAt = createdAt.Time
		if updatedAt != nil {
			e.UpdatedAt = &updatedAt.Time
		}

		entities = append(entities, e)
		ei++
	}

	return entities, total, err
}

// IndexFilesForRequesterForType Index packages for Requester
func IndexFilesForRequesterForType(ctx context.Context, requester *sm.User, entityId uuid.UUID, offset int64, limit int64, fileType string) (entities []File, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) FROM files f 
    LEFT JOIN entities e ON e.id = f.entity_id 
    LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid 
WHERE e.id = $2 AND (e.public OR a.can_view OR a.is_owner) AND f.type = $3`

	row := db.QueryRow(ctx, q, requester.Id /*$1*/, entityId /*$2*/, fileType)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	if total == 0 {
		return []File{}, total, nil
	}

	q = `SELECT
f.id fileId,
f.entity_id fileEntityId,
f.type fileType,
f.url fileUrl,
f.mime fileMime,
f.size fileSize,
f.version fileVersion,
f.deployment_type fileDeployment,
f.platform filePlatform,
f.uploaded_by fileUploadedBy,
f.width fileWidth,
f.height fileHeight,
f.created_at fileCreatedAt,
f.updated_at fileUpdatedAt,
f.variation fileIndex,
f.hash fileHash,
f.original_path fileOriginalPath
FROM files f
	LEFT JOIN entities e on e.id = f.entity_id
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
WHERE e.id = $2 AND (e.public OR a.can_view OR a.is_owner) AND f.type = $3
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows pgx.Rows
		ei   int64 = 0
	)

	rows, err = db.Query(ctx, q, requester.Id, entityId, fileType)

	if err != nil {
		return []File{}, total, err
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexFilesForRequesterForType")
	}()

	for rows.Next() {
		var (
			id           pgtypeuuid.UUID
			eId          pgtypeuuid.UUID
			fFileType    string
			url          string
			mime         *string
			size         *int64
			version      int
			fDeployment  string
			fPlatform    string
			uploadedBy   pgtypeuuid.UUID
			width        *int
			height       *int
			createdAt    pgtype.Timestamp
			updatedAt    *pgtype.Timestamp
			index        int
			hash         *string
			originalPath *string
		)

		err = rows.Scan(
			&id,
			&eId,
			&fFileType,
			&url,
			&mime,
			&size,
			&version,
			&fDeployment,
			&fPlatform,
			&uploadedBy,
			&width,
			&height,
			&createdAt,
			&updatedAt,
			&index,
			&hash,
			&originalPath,
		)
		if err != nil {
			return nil, -1, err
		}

		if ei < offset {
			ei++
			continue
		}

		if ei-offset >= limit {
			break
		}

		var e File
		e.Id = &id.UUID
		e.EntityId = &eId.UUID
		e.Type = fFileType
		e.Url = url
		e.Mime = mime
		e.Size = size
		e.Version = version
		e.Deployment = fDeployment
		e.Platform = fPlatform
		e.Index = index
		e.Hash = hash
		e.OriginalPath = originalPath
		if uploadedBy.Status == pgtype.Present {
			e.UploadedBy = &uploadedBy.UUID
		}
		e.Width = width
		e.Height = height
		e.CreatedAt = createdAt.Time
		if updatedAt != nil {
			e.UpdatedAt = &updatedAt.Time
		}

		entities = append(entities, e)
		ei++
	}

	return entities, total, err
}

// IndexFilesForRequesterForTypeForDeployment Index packages for Requester
func IndexFilesForRequesterForTypeForDeployment(ctx context.Context, requester *sm.User, entityId uuid.UUID, offset int64, limit int64, fileType string, deployment string) (entities []File, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) FROM files f 
    LEFT JOIN entities e ON e.id = f.entity_id
    LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
WHERE e.id = $2 AND (e.public OR a.can_view OR a.is_owner) AND f.type = $3 AND f.deployment_type = $4`

	row := db.QueryRow(ctx, q, entityId /*$1*/, requester.Id /*$2*/, fileType, deployment)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	if total == 0 {
		return []File{}, total, nil
	}

	q = `SELECT
f.id fileId,
f.entity_id fileEntityId,
f.type fileType,
f.url fileUrl,
f.mime fileMime,
f.size fileSize,
f.version fileVersion,
f.deployment_type fileDeployment,
f.platform filePlatform,
f.uploaded_by fileUploadedBy,
f.width fileWidth,
f.height fileHeight,
f.created_at fileCreatedAt,
f.updated_at fileUpdatedAt,
f.variation fileIndex,
f.hash fileHash,
f.original_path fileOriginalPath
FROM files f
	LEFT JOIN entities e on e.id = f.entity_id
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
WHERE f.entity_id = $2 AND (e.public OR a.can_view OR a.is_owner) AND f.type = $3 AND f.deployment_type = $4
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows pgx.Rows
		ei   int64 = 0
	)

	rows, err = db.Query(ctx, q, requester.Id, entityId, fileType, deployment)

	if err != nil {
		return []File{}, total, err
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexFilesForRequesterForTypeForDeployment")
	}()

	for rows.Next() {
		var (
			id           pgtypeuuid.UUID
			eId          pgtypeuuid.UUID
			fFileType    string
			url          string
			mime         *string
			size         *int64
			version      int
			fDeployment  string
			fPlatform    string
			uploadedBy   pgtypeuuid.UUID
			width        *int
			height       *int
			createdAt    pgtype.Timestamp
			updatedAt    *pgtype.Timestamp
			index        int
			hash         *string
			originalPath *string
		)

		err = rows.Scan(
			&id,
			&eId,
			&fFileType,
			&url,
			&mime,
			&size,
			&version,
			&fDeployment,
			&fPlatform,
			&uploadedBy,
			&width,
			&height,
			&createdAt,
			&updatedAt,
			&index,
			&hash,
			&originalPath,
		)
		if err != nil {
			return nil, -1, err
		}

		if ei < offset {
			ei++
			continue
		}

		if ei-offset >= limit {
			break
		}

		var e File
		e.Id = &id.UUID
		e.EntityId = &eId.UUID
		e.Type = fFileType
		e.Url = url
		e.Mime = mime
		e.Size = size
		e.Version = version
		e.Deployment = fDeployment
		e.Platform = fPlatform
		e.Index = index
		e.Hash = hash
		e.OriginalPath = originalPath
		if uploadedBy.Status == pgtype.Present {
			e.UploadedBy = &uploadedBy.UUID
		}
		e.Width = width
		e.Height = height
		e.CreatedAt = createdAt.Time
		if updatedAt != nil {
			e.UpdatedAt = &updatedAt.Time
		}

		entities = append(entities, e)
		ei++
	}

	return entities, total, err
}

// IndexFilesForRequesterForTypeForPlatform Index packages for Requester
func IndexFilesForRequesterForTypeForPlatform(ctx context.Context, requester *sm.User, entityId uuid.UUID, offset int64, limit int64, fileType string, platform string) (entities []File, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) FROM files f 
    LEFT JOIN entities e ON e.id = f.entity_id 
    LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid 
WHERE e.id = $2 AND (e.public OR a.can_view OR a.is_owner) AND f.type = $3 AND f.platform = $4`

	row := db.QueryRow(ctx, q, requester.Id /*$1*/, entityId /*$2*/, fileType, platform)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	if total == 0 {
		return []File{}, total, nil
	}

	q = `SELECT
f.id fileId,
f.entity_id fileEntityId,
f.type fileType,
f.url fileUrl,
f.mime fileMime,
f.size fileSize,
f.version fileVersion,
f.deployment_type fileDeployment,
f.platform filePlatform,
f.uploaded_by fileUploadedBy,
f.width fileWidth,
f.height fileHeight,
f.created_at fileCreatedAt,
f.updated_at fileUpdatedAt,
f.variation fileIndex,
f.hash fileHash,
f.original_path fileOriginalPath
FROM files f
	LEFT JOIN entities e on e.id = f.entity_id
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
	WHERE f.entity_id = $2 AND (e.public OR a.can_view OR a.is_owner) AND f.type = $3 AND f.platform = $4
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows pgx.Rows
		ei   int64 = 0
	)

	rows, err = db.Query(ctx, q, requester.Id, entityId, fileType, platform)

	if err != nil {
		return []File{}, total, err
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexFilesForRequesterForTypeForPlatform")
	}()

	for rows.Next() {
		var (
			id           pgtypeuuid.UUID
			eId          pgtypeuuid.UUID
			fFileType    string
			url          string
			mime         *string
			size         *int64
			version      int
			fDeployment  string
			fPlatform    string
			uploadedBy   pgtypeuuid.UUID
			width        *int
			height       *int
			createdAt    pgtype.Timestamp
			updatedAt    *pgtype.Timestamp
			index        int
			hash         *string
			originalPath *string
		)

		err = rows.Scan(
			&id,
			&eId,
			&fFileType,
			&url,
			&mime,
			&size,
			&version,
			&fDeployment,
			&fPlatform,
			&uploadedBy,
			&width,
			&height,
			&createdAt,
			&updatedAt,
			&index,
			&hash,
			&originalPath,
		)
		if err != nil {
			return nil, -1, err
		}

		if ei < offset {
			ei++
			continue
		}

		if ei-offset >= limit {
			break
		}

		var e File
		e.Id = &id.UUID
		e.EntityId = &eId.UUID
		e.Type = fFileType
		e.Url = url
		e.Mime = mime
		e.Size = size
		e.Version = version
		e.Deployment = fDeployment
		e.Platform = fPlatform
		e.Index = index
		e.Hash = hash
		e.OriginalPath = originalPath
		if uploadedBy.Status == pgtype.Present {
			e.UploadedBy = &uploadedBy.UUID
		}
		e.Width = width
		e.Height = height
		e.CreatedAt = createdAt.Time
		if updatedAt != nil {
			e.UpdatedAt = &updatedAt.Time
		}

		entities = append(entities, e)
		ei++
	}

	return entities, total, err
}

// IndexFilesForRequesterForTypeForPlatformForDeployment Index packages for Requester
func IndexFilesForRequesterForTypeForPlatformForDeployment(ctx context.Context, requester *sm.User, entityId uuid.UUID, offset int64, limit int64, fileType string, platform string, deployment string) (entities []File, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) FROM files f 
    LEFT JOIN entities e ON e.id = f.entity_id 
    LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid 
WHERE e.id = $2 AND (e.public OR a.can_view OR a.is_owner) AND f.type = $3 AND f.platform = $4 AND f.deployment_type = $5`

	row := db.QueryRow(ctx, q, requester.Id, entityId, fileType, platform, deployment)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	if total == 0 {
		return []File{}, total, nil
	}

	q = `SELECT
f.id fileId,
f.entity_id fileEntityId,
f.type fileType,
f.url fileUrl,
f.mime fileMime,
f.size fileSize,
f.version fileVersion,
f.deployment_type fileDeployment,
f.platform filePlatform,
f.uploaded_by fileUploadedBy,
f.width fileWidth,
f.height fileHeight,
f.created_at fileCreatedAt,
f.updated_at fileUpdatedAt,
f.variation fileIndex,
f.hash fileHash,
f.original_path fileOriginalPath
FROM files f
	LEFT JOIN entities e on e.id = f.entity_id
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
WHERE f.entity_id = $2 AND (e.public OR a.can_view OR a.is_owner) AND f.type = $3 AND f.platform = $4 AND f.deployment_type = $5
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows pgx.Rows
		ei   int64 = 0
	)

	rows, err = db.Query(ctx, q, requester.Id, entityId, fileType, platform, deployment)

	if err != nil {
		return []File{}, total, err
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexFilesForRequesterForTypeForPlatformForDeployment")
	}()

	for rows.Next() {
		var (
			id           pgtypeuuid.UUID
			eId          pgtypeuuid.UUID
			fFileType    string
			url          string
			mime         *string
			size         *int64
			version      int
			fDeployment  string
			fPlatform    string
			uploadedBy   pgtypeuuid.UUID
			width        *int
			height       *int
			createdAt    pgtype.Timestamp
			updatedAt    *pgtype.Timestamp
			index        int
			hash         *string
			originalPath *string
		)

		err = rows.Scan(
			&id,
			&eId,
			&fFileType,
			&url,
			&mime,
			&size,
			&version,
			&fDeployment,
			&fPlatform,
			&uploadedBy,
			&width,
			&height,
			&createdAt,
			&updatedAt,
			&index,
			&hash,
			&originalPath,
		)
		if err != nil {
			return nil, -1, err
		}

		if ei < offset {
			ei++
			continue
		}

		if ei-offset >= limit {
			break
		}

		var e File
		e.Id = &id.UUID
		e.EntityId = &eId.UUID
		e.Type = fFileType
		e.Url = url
		e.Mime = mime
		e.Size = size
		e.Version = version
		e.Deployment = fDeployment
		e.Platform = fPlatform
		e.Index = index
		e.Hash = hash
		e.OriginalPath = originalPath
		if uploadedBy.Status == pgtype.Present {
			e.UploadedBy = &uploadedBy.UUID
		}
		e.Width = width
		e.Height = height
		e.CreatedAt = createdAt.Time
		if updatedAt != nil {
			e.UpdatedAt = &updatedAt.Time
		}

		entities = append(entities, e)
		ei++
	}

	return entities, total, err
}

func UploadFileForAdmin(c *fiber.Ctx, requester *sm.User, entityId uuid.UUID, m FileUploadRequestMetadata) (fileId uuid.UUID, err error) {
	db := database.DB
	ctx := c.UserContext()

	var (
		q        string                // query string
		tx       pgx.Tx                // pgx transaction
		row      pgx.Row               // pgx row
		fId      pgtypeuuid.UUID       // id of the file from the db
		fVersion int64                 // file version from the db
		id       uuid.UUID             // file id to use
		formFile *multipart.FileHeader // uploaded file
		buffer   multipart.File        // uploaded file contents
		mime     string                // MIME type to use
		pMIME    *mimetype.MIME        // auto-detected MIME type
	)

	if PlatformDependentFileTypes[m.Type] {
		if m.Deployment == "" {
			err = fmt.Errorf("file requires a deployment configuration")
			return uuid.UUID{}, err
		}

		if m.Platform == "" {
			err = fmt.Errorf("file requires a platform")
			return uuid.UUID{}, err
		}
	}

	// Try to find existing file with same properties
	q = `SELECT f.id, f.version
FROM files AS f
WHERE f.entity_id = $1
  AND f.type = $2
  AND f.deployment_type = $3
  AND f.platform = $4
  AND f.variation = $5
  AND f.original_path = $6`

	row = db.QueryRow(ctx, q, entityId, m.Type, m.Deployment, m.Platform, m.Index, m.OriginalPath)

	logrus.Infof("uploading file, entityId: %v, type: %v, deployment: %v, platform: %v, index: %v, originalPath: %v", entityId, m.Type, m.Deployment, m.Platform, m.Index, m.OriginalPath)

	err = row.Scan(&fId, &fVersion)
	if err != nil && err.Error() == "no rows in result set" {
		logrus.Infof("inserting a new file")

		//region Insert a new file
		tx, err = db.Begin(ctx)
		if err != nil {
			return uuid.UUID{}, err // failed to initiate tx
		}

		//region Process multipart form uploaded file

		// Get upload file
		formFile, err = c.FormFile("file")
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		// Get upload file buffer
		buffer, err = formFile.Open()
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}
		defer func(buffer multipart.File) {
			err := buffer.Close()
			if err != nil {
				logrus.Errorf("failed to close multipart file buffer: %v", err)
			}
		}(buffer)
		//endregion

		//region Determine MIME type
		if m.Mime != nil && *m.Mime != "" {
			// Use explicitly specified MIME type
			mime = *m.Mime
		} else {
			// Seek back to the start of the file
			_, err = buffer.Seek(0, io.SeekStart)
			if err != nil {
				return uuid.UUID{}, err
			}

			// Try to detect MIME
			pMIME, _ = mimetype.DetectReader(buffer)
			mime = pMIME.String()

			// Seek back to the start of the file
			_, err = buffer.Seek(0, io.SeekStart)
			if err != nil {
				return uuid.UUID{}, err
			}
		}
		//endregion

		//region S3 Object data

		// Generate a new uuid for the file
		id, err = uuid.NewV4()
		if err != nil {
			return uuid.UUID{}, err
		}

		// Get url and the key for the entity file
		url := s3.GetS3UrlForEntityFile(entityId, id)
		key := s3.GetS3KeyForEntityFile(entityId, id)

		//endregion

		//region Add a new file metadata record and upload the Object to S3

		q = `INSERT INTO files AS f (id, entity_id, url, type, mime, size, version, deployment_type, platform, uploaded_by, width, height, created_at, updated_at, variation, original_path, hash)
			VALUES ($1::uuid, $2::uuid, $3::text, $4::text, $5::text, $6::bigint, $7::bigint, $8::text, $9::text, $10::uuid, $11::integer, $12::integer, now(), null, $13::bigint, $14::text, $15::text)`
		_, err = tx.Exec(ctx, q, id, entityId, url, m.Type, mime, formFile.Size, m.Version, m.Deployment, m.Platform, requester.Id, m.Width, m.Height, m.Index, m.OriginalPath, m.Hash)
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		var public = true
		if m.Type == "uplugin_content" || m.Type == "uplugin" {
			// Protect source files
			public = false
		}

		if m.Type == "uplugin_content" {
			// Send the job status email to the requester
			if err = SendPackageJobStatusEmail(c, requester, entityId, "unclaimed"); err != nil {
				logrus.Errorf("failed to send job status email: %v", err)
			}

			if err = st.SendTelegramMessage(fmt.Sprintf("Creating a new job for %s", entityId)); err != nil {
				logrus.Errorf("failed to send a job status telegram message: %v", err)
			}

			// Schedule package jobs
			if err = CreatePackageJobs(c, requester, entityId); err != nil {
				logrus.Errorf("failed to create package jobs: %v", err)
			}

			// deprecated
			if err = ScheduleBuildJobsForPackage(c, requester, entityId); err != nil {
				fmt.Printf("failed to schedule a build jobs: %s", err.Error())
			}
		}

		metadata := map[string]string{
			"version":      strconv.FormatInt(m.Version, 10),
			"index":        strconv.FormatInt(m.Index, 10),
			"type":         m.Type,
			"originalPath": m.OriginalPath,
		}

		if m.Deployment != "" {
			metadata["deployment"] = m.Deployment
		}

		if m.Platform != "" {
			metadata["platform"] = m.Platform
		}

		err = s3.UploadObject(key, buffer, mime, public, &metadata, nil)
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		//endregion

		err = tx.Commit(ctx)
		if err == nil {
			if m.Type == "pak" {
				// Deprecated
				if err = NotifyBuildJobCompleted(c, entityId); err != nil {
					fmt.Printf("failed to notify job complete: %s", err.Error())
				}
			}
		}

		return id, err

		//endregion

	} else if err == nil {
		logrus.Infof("replacing an existing file")

		//region Replace an existing file

		tx, err = db.Begin(ctx)
		if err != nil {
			return uuid.UUID{}, err
		}

		//region Process multipart form uploaded file

		// Get upload file
		formFile, err = c.FormFile("file")
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		// Get upload file buffer
		buffer, err = formFile.Open()
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}
		defer func(buffer multipart.File) {
			err := buffer.Close()
			if err != nil {
				logrus.Errorf("error closing multipart file: %v", err)
			}
		}(buffer)
		//endregion

		//region Determine MIME type
		if m.Mime != nil && *m.Mime != "" {
			// Use explicitly specified MIME type
			mime = *m.Mime
		} else {
			// Seek back to the start of the file
			_, err = buffer.Seek(0, io.SeekStart)
			if err != nil {
				return uuid.UUID{}, err
			}

			// Try to detect MIME
			pMIME, _ = mimetype.DetectReader(buffer)
			mime = pMIME.String()

			// Seek back to the start of the file
			_, err = buffer.Seek(0, io.SeekStart)
			if err != nil {
				return uuid.UUID{}, err
			}
		}
		//endregion

		//region S3 Object data

		// Generate a new uuid for the file
		id, err = uuid.NewV4()

		// Get url and the key for the entity file
		url := s3.GetS3UrlForEntityFile(entityId, id)
		previousKey := s3.GetS3KeyForEntityFile(entityId, fId.UUID)
		newKey := s3.GetS3KeyForEntityFile(entityId, id)

		//endregion

		//region Remove old file metadata record and S3 object

		// Delete the old file record
		q = `DELETE FROM files f WHERE f.id = $1`
		_, err = tx.Exec(ctx, q, fId.UUID)
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		// Delete the old object
		exists := s3.ObjectExists(previousKey)
		if exists {
			err = s3.DeleteObject(previousKey)
			if err != nil {
				_ = tx.Rollback(ctx)
				return uuid.UUID{}, err
			}
		}

		//endregion

		//region Add a new file metadata record and upload the object to S3

		fVersion++

		// Add a new file record
		q = `INSERT INTO files AS f (id, entity_id, url, type, mime, size, version, deployment_type, platform, uploaded_by, width, height, created_at, updated_at, variation, original_path, hash)
			VALUES ($1::uuid, $2::uuid, $3::text, $4::text, $5::text, $6::bigint, $7::bigint, $8::text, $9::text, $10::uuid, $11::integer, $12::integer, now(), null, $13::bigint, $14::text, $15::text)`
		_, err = tx.Exec(ctx, q, id, entityId, url, m.Type, mime, formFile.Size, fVersion, m.Deployment, m.Platform, requester.Id, m.Width, m.Height, m.Index, m.OriginalPath, m.Hash)
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		var public = true
		if m.Type == "uplugin_content" || m.Type == "uplugin" {
			// Protect source files
			public = false
		}

		if m.Type == "uplugin_content" {
			// Send the job status email to the requester
			if err = SendPackageJobStatusEmail(c, requester, entityId, "unclaimed"); err != nil {
				logrus.Errorf("failed to send a job status email: %v", err)
			}

			if err = st.SendTelegramMessage(fmt.Sprintf("Creating a new job for %s", entityId)); err != nil {
				logrus.Errorf("failed to send a telegram message: %v", err)
			}

			// Schedule package jobs
			if err = CreatePackageJobs(c, requester, entityId); err != nil {
				logrus.Errorf("failed to create package jobs: %v", err)
			}

			// deprecated
			if err = ScheduleBuildJobsForPackage(c, requester, entityId); err != nil {
				fmt.Printf("failed to schedule a build jobs: %s", err.Error())
			}
		}

		metadata := map[string]string{
			"version":      strconv.FormatInt(fVersion, 10),
			"index":        strconv.FormatInt(m.Index, 10),
			"type":         m.Type,
			"originalPath": m.OriginalPath,
		}

		if m.Deployment != "" {
			metadata["deployment"] = m.Deployment
		}

		if m.Platform != "" {
			metadata["platform"] = m.Platform
		}

		// Upload the new Object
		err = s3.UploadObject(newKey, buffer, mime, public, &metadata, nil)
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		//endregion

		err = tx.Commit(ctx)
		if err == nil {
			if m.Type == "pak" {
				if err = NotifyBuildJobCompleted(c, entityId); err != nil {
					fmt.Printf("failed to notify job complete: %s", err.Error())
				}
			}
		}

		return id, err

		//endregion
	}

	return uuid.UUID{}, err
}

func UploadFileForRequester(c *fiber.Ctx, requester *sm.User, entityId uuid.UUID, m FileUploadRequestMetadata) (fileId uuid.UUID, err error) {
	db := database.DB
	ctx := c.UserContext()

	var (
		q        string                // query string
		tx       pgx.Tx                // pgx transaction
		row      pgx.Row               // pgx row
		fId      pgtypeuuid.UUID       // id of the file from the db
		fVersion int64                 // file version from the db
		id       uuid.UUID             // file id to use
		formFile *multipart.FileHeader // uploaded file
		buffer   multipart.File        // uploaded file contents
		mime     string                // MIME type to use
		pMIME    *mimetype.MIME        // auto-detected MIME type
		isOwner  bool
		canEdit  bool
	)

	if m.Type == "pak" || m.Type == "release-archive" {
		if m.Deployment == "" {
			err = fmt.Errorf("pak file has no deployment configuration set")
			return uuid.UUID{}, err
		}

		if m.Platform == "" {
			err = fmt.Errorf("pak file has no platform set")
			return uuid.UUID{}, err
		}
	} else {
		m.Deployment = ""
		m.Platform = ""
	}

	// Try to find existing file with same properties
	q = `SELECT f.id,
       f.version,
       a.is_owner,
	   a.can_edit
FROM files AS f
	LEFT JOIN entities e on f.entity_id = e.id
	LEFT JOIN accessibles a on e.id = a.entity_id AND a.user_id = $1::uuid
WHERE f.entity_id = $2
  AND f.type = $3
  AND f.deployment_type = $4
  AND f.platform = $5
  AND f.variation = $6
  AND f.original_path = $7`

	row = db.QueryRow(ctx, q, requester.Id, entityId, m.Type, m.Deployment, m.Platform, m.Index, m.OriginalPath)

	err = row.Scan(&fId, &fVersion, &isOwner, &canEdit)
	if err != nil && err.Error() == "no rows in result set" {
		// Check entity access
		q = `SELECT a.is_owner, a.can_edit FROM entities e LEFT JOIN accessibles a on e.id = a.entity_id AND a.user_id = $1::uuid WHERE e.id = $2`
		row = db.QueryRow(ctx, q, requester.Id, entityId)
		err = row.Scan(&isOwner, &canEdit)

		if !(isOwner || canEdit) {
			return uuid.UUID{}, errors.New("no access")
		}

		//region Insert a new file
		tx, err = db.Begin(ctx)
		if err != nil {
			return uuid.UUID{}, err // failed to initiate tx
		}

		//region Process multipart form uploaded file

		// Get upload file
		formFile, err = c.FormFile("file")
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		// Get upload file buffer
		buffer, err = formFile.Open()
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}
		defer func(buffer multipart.File) {
			err := buffer.Close()
			if err != nil {
				logrus.Errorf("failed to close multipart file buffer: %v", err)
			}
		}(buffer)
		//endregion

		//region Determine MIME type
		if m.Mime != nil && *m.Mime != "" {
			// Use explicitly specified MIME type
			mime = *m.Mime
		} else {
			// Seek back to the start of the file
			_, err = buffer.Seek(0, io.SeekStart)
			if err != nil {
				return uuid.UUID{}, err
			}

			// Try to detect MIME
			pMIME, _ = mimetype.DetectReader(buffer)
			mime = pMIME.String()

			// Seek back to the start of the file
			_, err = buffer.Seek(0, io.SeekStart)
			if err != nil {
				return uuid.UUID{}, err
			}
		}
		//endregion

		//region S3 Object data

		// Generate a new uuid for the file
		id, err = uuid.NewV4()
		if err != nil {
			return uuid.UUID{}, err
		}

		// Get url and the key for the entity file
		url := s3.GetS3UrlForEntityFile(entityId, id)
		key := s3.GetS3KeyForEntityFile(entityId, id)

		//endregion

		//region Upload the Object to S3

		var public = true
		if m.Type == "uplugin_content" || m.Type == "uplugin" {
			// Protect source files
			public = false
		}

		metadata := map[string]string{
			"version": strconv.FormatInt(m.Version, 10),
			"index":   strconv.FormatInt(m.Index, 10),
			"type":    m.Type,
		}

		if m.Deployment != "" {
			metadata["deployment"] = m.Deployment
		}

		if m.Platform != "" {
			metadata["platform"] = m.Platform
		}

		err = s3.UploadObject(key, buffer, mime, public, &metadata, nil)
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		//endregion

		//region Add a file record
		q = `INSERT INTO files AS f (id, entity_id, url, type, mime, size, version, deployment_type, platform, uploaded_by, width, height, created_at, updated_at, variation, original_path, hash)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now(), null, $13, $14, $15)`
		_, err = tx.Exec(ctx, q, id, entityId, url, m.Type, mime, formFile.Size, m.Version, m.Deployment, m.Platform, requester.Id, m.Width, m.Height, m.Index, m.OriginalPath, m.Hash)
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		err = tx.Commit(ctx)
		if err == nil {
			if m.Type == "pak" {
				if err = NotifyBuildJobCompleted(c, entityId); err != nil {
					fmt.Printf("failed to notify job complete: %s", err.Error())
				}
			}
		}

		return id, err
		//endregion

		//endregion
	} else if err == nil {
		if !isOwner || !canEdit {
			return uuid.UUID{}, errors.New("no access")
		}

		//region Replace an existing file
		fVersion++

		tx, err = db.Begin(ctx)
		if err != nil {
			return uuid.UUID{}, err
		}

		//region Process multipart form uploaded file

		// Get upload file
		formFile, err = c.FormFile("file")
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		// Get upload file buffer
		buffer, err = formFile.Open()
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}
		defer func(buffer multipart.File) {
			err := buffer.Close()
			if err != nil {
				logrus.Errorf("failed to close multipart file buffer: %v", err)
			}
		}(buffer)
		//endregion

		//region Determine MIME type
		if m.Mime != nil && *m.Mime != "" {
			// Use explicitly specified MIME type
			mime = *m.Mime
		} else {
			// Seek back to the start of the file
			_, err = buffer.Seek(0, io.SeekStart)
			if err != nil {
				return uuid.UUID{}, err
			}

			// Try to detect MIME
			pMIME, _ = mimetype.DetectReader(buffer)
			mime = pMIME.String()

			// Seek back to the start of the file
			_, err = buffer.Seek(0, io.SeekStart)
			if err != nil {
				return uuid.UUID{}, err
			}
		}
		//endregion

		//region S3 Object data

		// Generate a new uuid for the file
		id, err = uuid.NewV4()

		// Get url and the key for the entity file
		url := s3.GetS3UrlForEntityFile(entityId, id)
		previousKey := s3.GetS3KeyForEntityFile(entityId, fId.UUID)
		newKey := s3.GetS3KeyForEntityFile(entityId, id)

		//endregion

		//region Upload the Object to S3

		// Delete the old Object

		exists := s3.ObjectExists(previousKey)
		if exists {
			err = s3.DeleteObject(previousKey)
			if err != nil {
				_ = tx.Rollback(ctx)
				return uuid.UUID{}, err
			}
		}

		var public = true
		if strings.HasPrefix(m.Type, "uplugin") {
			// Protect package source files
			public = false
		}

		metadata := map[string]string{
			"version": strconv.FormatInt(fVersion, 10),
			"index":   strconv.FormatInt(m.Index, 10),
			"type":    m.Type,
		}

		if m.Deployment != "" {
			metadata["deployment"] = m.Deployment
		}

		if m.Platform != "" {
			metadata["platform"] = m.Platform
		}

		// Upload the new Object
		err = s3.UploadObject(newKey, buffer, mime, public, &metadata, nil)
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		//endregion

		//region Update a file record
		_, err = tx.Exec(ctx, "SET CONSTRAINTS files_pkey DEFERRED")
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		q = `UPDATE files f
SET id=$1,
    url=$2,
    mime=$3,
    size=$4,
    version=$5,
    uploaded_by=$6,
    width=$7,
    height=$8,
    updated_at=now(),
    original_path=$14
WHERE f.entity_id = $9
  AND f.type = $10
  AND f.deployment_type = $11
  AND f.platform = $12
  AND f.variation = $13`
		_, err = tx.Exec(ctx, q, id, url, mime, formFile.Size, fVersion, requester.Id, m.Width, m.Height, entityId, m.Type, m.Deployment, m.Platform, m.Index, m.OriginalPath)
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		err = tx.Commit(ctx)
		if err == nil {
			if m.Type == "pak" {
				if err = NotifyBuildJobCompleted(c, entityId); err != nil {
					fmt.Printf("failed to notify job complete: %s", err.Error())
				}
			}
		}

		return id, err
		//endregion

		//endregion
	}

	return uuid.UUID{}, err
}

func UploadResizedImageForAdmin(c *fiber.Ctx, requester *sm.User, entityId uuid.UUID, m FileUploadRequestMetadata, uploadImageType UploadImageType) (fileId uuid.UUID, err error) {
	db := database.DB
	ctx := c.UserContext()

	var (
		q         string                // query string
		tx        pgx.Tx                // pgx transaction
		row       pgx.Row               // pgx row
		fId       pgtypeuuid.UUID       // id of the file from the db
		fVersion  int64                 // version of the file from the db
		id        uuid.UUID             // file id to use
		formFile  *multipart.FileHeader // uploaded file
		buffer    multipart.File        // uploaded file contents
		mime      string                // MIME type to use
		pMIME     *mimetype.MIME        // auto-detected MIME type
		imageType string                // image type to use
	)

	switch uploadImageType {
	case ImagePreview:
		imageType = "image_preview"
	case ImageTexture:
		imageType = "texture_diffuse"
	default:
		imageType = "image_full"
	}

	// Try to find existing file with same properties
	q = `SELECT id, version
FROM files AS f
WHERE f.entity_id = $1
  AND f.type = $2
  AND f.deployment_type = $3
  AND f.platform = $4
  AND f.variation = $5`

	row = db.QueryRow(ctx, q, entityId, imageType, m.Deployment, m.Platform, m.Index)

	err = row.Scan(&fId, &fVersion)
	if err != nil && err.Error() == "no rows in result set" {
		//region Insert a new file
		tx, err = db.Begin(ctx)
		if err != nil {
			return uuid.UUID{}, err // failed to initiate tx
		}

		//region Process multipart form uploaded file

		// Get upload file
		formFile, err = c.FormFile("file")
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		// Get upload file buffer
		buffer, err = formFile.Open()
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}
		defer func(buffer multipart.File) {
			err := buffer.Close()
			if err != nil {
				logrus.Errorf("failed to close multipart file buffer: %v", err)
			}
		}(buffer)
		//endregion

		//region Determine MIME type
		if m.Mime != nil && *m.Mime != "" {
			// Use explicitly specified MIME type
			mime = *m.Mime
		} else {
			// Seek back to the start of the file
			_, err = buffer.Seek(0, io.SeekStart)
			if err != nil {
				_ = tx.Rollback(ctx)
				return uuid.UUID{}, err
			}

			// Try to detect MIME
			pMIME, _ = mimetype.DetectReader(buffer)
			mime = pMIME.String()

			// Seek back to the start of the file
			_, err = buffer.Seek(0, io.SeekStart)
			if err != nil {
				_ = tx.Rollback(ctx)
				return uuid.UUID{}, err
			}
		}

		var (
			resizedImageBuffer bytes.Buffer
			originalImage      image.Image
			newImage           image.Image
		)

		if mime == "image/jpeg" {
			// Decode to bitmap
			originalImage, err = jpeg.Decode(buffer)
			if err != nil {
				_ = tx.Rollback(ctx)
				return uuid.UUID{}, err
			}

			if uploadImageType == ImagePreview {
				// Resize to generate preview
				newImage = resize.Resize(256, 0, originalImage, resize.Lanczos3)
			} else if uploadImageType == ImageTexture {
				w := uint(originalImage.Bounds().Size().X)
				h := uint(originalImage.Bounds().Size().Y)
				newImage = resize.Resize(nextPowerOfTwo(w), nextPowerOfTwo(h), originalImage, resize.Lanczos3)
			}

			// Encode back to JPEG
			buf := bufio.NewWriter(&resizedImageBuffer)
			err = jpeg.Encode(buf, newImage, nil)
			if err != nil {
				_ = tx.Rollback(ctx)
				return uuid.UUID{}, err
			}
		} else if mime == "image/png" {
			// Decode to bitmap
			originalImage, err = png.Decode(buffer)
			if err != nil {
				_ = tx.Rollback(ctx)
				return uuid.UUID{}, err
			}

			if uploadImageType == ImagePreview {
				// Resize to generate preview
				newImage = resize.Resize(256, 0, originalImage, resize.Lanczos3)
			} else if uploadImageType == ImageTexture {
				w := uint(originalImage.Bounds().Size().X)
				h := uint(originalImage.Bounds().Size().Y)
				newImage = resize.Resize(nextPowerOfTwo(w), nextPowerOfTwo(h), originalImage, resize.Lanczos3)
			}

			// Encode back to JPEG
			buf := bufio.NewWriter(&resizedImageBuffer)
			err = png.Encode(buf, newImage)
			if err != nil {
				_ = tx.Rollback(ctx)
				return uuid.UUID{}, err
			}
		} else {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, errors.New(fmt.Sprintf("unsupported image MIME type: %s", mime))
		}

		//endregion

		//region S3 Object data

		// Generate a new uuid for the file
		id, err = uuid.NewV4()

		// Get url and the key for the entity file
		url := s3.GetS3UrlForEntityFile(entityId, id)
		key := s3.GetS3KeyForEntityFile(entityId, id)

		//endregion

		//region Upload the Object to S3

		metadata := map[string]string{
			"version": strconv.FormatInt(m.Version, 10),
			"index":   strconv.FormatInt(m.Index, 10),
			"type":    imageType,
		}

		if m.Deployment != "" {
			metadata["deployment"] = m.Deployment
		}

		if m.Platform != "" {
			metadata["platform"] = m.Platform
		}

		err = s3.UploadObject(key, bufio.NewReaderSize(&resizedImageBuffer, resizedImageBuffer.Len()), mime, true, &metadata, nil)
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		//endregion

		//region Add a file record
		q = `INSERT INTO files AS f (id, entity_id, url, type, mime, size, version, deployment_type, platform, uploaded_by, width, height, created_at, updated_at, variation)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now(), null, $13)`
		_, err = tx.Exec(ctx, q, id, entityId, url, imageType, mime, formFile.Size, m.Version, m.Deployment, m.Platform, requester.Id, m.Width, m.Height, m.Index)
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		return id, tx.Commit(ctx)
		//endregion

		//endregion
	} else if err == nil {
		//region Replace an existing file

		fVersion++

		tx, err = db.Begin(ctx)
		if err != nil {
			return uuid.UUID{}, err
		}

		//region Process multipart form uploaded file

		// Get upload file
		formFile, err = c.FormFile("file")
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		// Get upload file buffer
		buffer, err = formFile.Open()
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}
		defer func(buffer multipart.File) {
			err := buffer.Close()
			if err != nil {
				logrus.Errorf("failed to close multipart file buffer: %v", err)
			}
		}(buffer)
		//endregion

		//region Determine MIME type
		if m.Mime != nil && *m.Mime != "" {
			// Use explicitly specified MIME type
			mime = *m.Mime
		} else {
			// Seek back to the start of the file
			_, err = buffer.Seek(0, io.SeekStart)
			if err != nil {
				return uuid.UUID{}, err
			}

			// Try to detect MIME
			pMIME, _ = mimetype.DetectReader(buffer)
			mime = pMIME.String()

			// Seek back to the start of the file
			_, err = buffer.Seek(0, io.SeekStart)
			if err != nil {
				return uuid.UUID{}, err
			}
		}
		//endregion

		//region S3 Object data

		// Generate a new uuid for the file
		id, err = uuid.NewV4()

		// Get url and the key for the entity file
		url := s3.GetS3UrlForEntityFile(entityId, id)
		previousKey := s3.GetS3KeyForEntityFile(entityId, fId.UUID)
		newKey := s3.GetS3KeyForEntityFile(entityId, id)

		//endregion

		//region Upload the Object to S3

		// Delete the old Object

		exists := s3.ObjectExists(previousKey)
		if exists {
			err = s3.DeleteObject(previousKey)
			if err != nil {
				_ = tx.Rollback(ctx)
				return uuid.UUID{}, err
			}
		}

		metadata := map[string]string{
			"version": strconv.FormatInt(fVersion, 10),
			"index":   strconv.FormatInt(m.Index, 10),
			"type":    imageType,
		}

		if m.Deployment != "" {
			metadata["deployment"] = m.Deployment
		}

		if m.Platform != "" {
			metadata["platform"] = m.Platform
		}

		// Upload the new Object
		err = s3.UploadObject(newKey, buffer, mime, true, &metadata, nil)
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		//endregion

		//region Update a file record
		q = `UPDATE files f
SET id=$1,
    url=$2,
    mime=$3,
    size=$4,
    version=(f.version + 1),
    uploaded_by=$5,
    width=$6,
    height=$7,
    updated_at=now()
WHERE f.entity_id = $8
  AND f.type = $9
  AND f.deployment_type = $10
  AND f.platform = $11
  AND f.variation = $12`
		_, err = tx.Exec(ctx, q, id, url, mime, formFile.Size, requester.Id, m.Width, m.Height, entityId, imageType, m.Deployment, m.Platform, m.Index)
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		return id, tx.Commit(ctx)
		//endregion

		//endregion
	}

	return uuid.UUID{}, err
}

func UploadResizedImageForRequester(c *fiber.Ctx, requester *sm.User, entityId uuid.UUID, m FileUploadRequestMetadata, uploadImageType UploadImageType) (fileId uuid.UUID, err error) {
	db := database.DB
	ctx := c.UserContext()

	var (
		q         string                // query string
		tx        pgx.Tx                // pgx transaction
		row       pgx.Row               // pgx row
		fId       pgtypeuuid.UUID       // id of the file from the db
		fVersion  int64                 // version of the file from the db
		id        uuid.UUID             // file id to use
		formFile  *multipart.FileHeader // uploaded file
		buffer    multipart.File        // uploaded file contents
		mime      string                // MIME type to use
		pMIME     *mimetype.MIME        // auto-detected MIME type
		imageType string                // image type to use
		isOwner   bool
		canEdit   bool
	)

	switch uploadImageType {
	case ImagePreview:
		imageType = "image_preview"
	case ImageTexture:
		imageType = "texture_diffuse"
	default:
		imageType = "image_full"
	}

	// Try to find existing file with same properties
	q = `SELECT f.id,
       version,
       a.is_owner,
	   a.can_edit
FROM files AS f
	LEFT JOIN entities e on f.entity_id = e.id
	LEFT JOIN accessibles a on e.id = a.entity_id AND a.user_id = $1::uuid
WHERE f.entity_id = $2
  AND f.type = $3
  AND f.deployment_type = $4
  AND f.platform = $5
  AND f.variation = $6`

	row = db.QueryRow(ctx, q, requester.Id, entityId, imageType, m.Deployment, m.Platform, m.Index)

	err = row.Scan(&fId, &fVersion, &isOwner, &canEdit)
	if err != nil && err.Error() == "no rows in result set" {

		// Check entity access
		q = `SELECT a.is_owner, a.can_edit FROM entities e LEFT JOIN accessibles a on e.id = a.entity_id AND a.user_id = $1::uuid WHERE e.id = $2`
		row = db.QueryRow(ctx, q, requester.Id, entityId)
		err = row.Scan(&isOwner, &canEdit)

		if !(isOwner || canEdit) {
			return uuid.UUID{}, errors.New("no access")
		}

		//region Insert a new file
		tx, err = db.Begin(ctx)
		if err != nil {
			return uuid.UUID{}, err // failed to initiate tx
		}

		//region Process multipart form uploaded file

		// Get upload file
		formFile, err = c.FormFile("file")
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		// Get upload file buffer
		buffer, err = formFile.Open()
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}
		defer func(buffer multipart.File) {
			err := buffer.Close()
			if err != nil {
				logrus.Errorf("failed to close multipart file buffer: %v", err)
			}
		}(buffer)
		//endregion

		//region Determine MIME type
		if m.Mime != nil && *m.Mime != "" {
			// Use explicitly specified MIME type
			mime = *m.Mime
		} else {
			// Seek back to the start of the file
			_, err = buffer.Seek(0, io.SeekStart)
			if err != nil {
				_ = tx.Rollback(ctx)
				return uuid.UUID{}, err
			}

			// Try to detect MIME
			pMIME, _ = mimetype.DetectReader(buffer)
			mime = pMIME.String()

			// Seek back to the start of the file
			_, err = buffer.Seek(0, io.SeekStart)
			if err != nil {
				_ = tx.Rollback(ctx)
				return uuid.UUID{}, err
			}
		}

		var (
			resizedImageBuffer bytes.Buffer
			originalImage      image.Image
			newImage           image.Image
		)

		if mime == "image/jpeg" {
			// Decode to bitmap
			originalImage, err = jpeg.Decode(buffer)
			if err != nil {
				_ = tx.Rollback(ctx)
				return uuid.UUID{}, err
			}

			if uploadImageType == ImagePreview {
				// Resize to generate preview
				newImage = resize.Resize(256, 0, originalImage, resize.Lanczos3)
			} else if uploadImageType == ImageTexture {
				w := uint(originalImage.Bounds().Size().X)
				h := uint(originalImage.Bounds().Size().Y)
				newImage = resize.Resize(nextPowerOfTwo(w), nextPowerOfTwo(h), originalImage, resize.Lanczos3)
			}

			// Encode back to JPEG
			buf := bufio.NewWriter(&resizedImageBuffer)
			err = jpeg.Encode(buf, newImage, nil)
			if err != nil {
				_ = tx.Rollback(ctx)
				return uuid.UUID{}, err
			}
		} else if mime == "image/png" {
			// Decode to bitmap
			originalImage, err = png.Decode(buffer)
			if err != nil {
				_ = tx.Rollback(ctx)
				return uuid.UUID{}, err
			}

			if uploadImageType == ImagePreview {
				// Resize to generate preview
				newImage = resize.Resize(256, 0, originalImage, resize.Lanczos3)
			} else if uploadImageType == ImageTexture {
				w := uint(originalImage.Bounds().Size().X)
				h := uint(originalImage.Bounds().Size().Y)
				newImage = resize.Resize(nextPowerOfTwo(w), nextPowerOfTwo(h), originalImage, resize.Lanczos3)
			}

			// Encode back to JPEG
			buf := bufio.NewWriter(&resizedImageBuffer)
			err = png.Encode(buf, newImage)
			if err != nil {
				_ = tx.Rollback(ctx)
				return uuid.UUID{}, err
			}
		} else {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, errors.New(fmt.Sprintf("unsupported image MIME type: %s", mime))
		}

		//endregion

		//region S3 Object data

		// Generate a new uuid for the file
		id, err = uuid.NewV4()

		// Get url and the key for the entity file
		url := s3.GetS3UrlForEntityFile(entityId, id)
		key := s3.GetS3KeyForEntityFile(entityId, id)

		//endregion

		//region Upload the Object to S3

		metadata := map[string]string{
			"version": strconv.FormatInt(m.Version, 10),
			"index":   strconv.FormatInt(m.Index, 10),
			"type":    imageType,
		}

		if m.Deployment != "" {
			metadata["deployment"] = m.Deployment
		}

		if m.Platform != "" {
			metadata["platform"] = m.Platform
		}

		err = s3.UploadObject(key, bufio.NewReaderSize(&resizedImageBuffer, resizedImageBuffer.Len()), mime, true, &metadata, nil)
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		//endregion

		//region Add a file record
		q = `INSERT INTO files AS f (id, entity_id, url, type, mime, size, version, deployment_type, platform, uploaded_by, width, height, created_at, updated_at, variation)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now(), null, $13)`
		_, err = tx.Exec(ctx, q, id, entityId, url, imageType, mime, formFile.Size, m.Version, m.Deployment, m.Platform, requester.Id, m.Width, m.Height, m.Index)
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		return id, tx.Commit(ctx)
		//endregion

		//endregion
	} else if err == nil {
		if !(isOwner || canEdit) {
			return uuid.UUID{}, errors.New("no access")
		}

		//region Replace an existing file

		fVersion++

		tx, err = db.Begin(ctx)
		if err != nil {
			return uuid.UUID{}, err
		}

		//region Process multipart form uploaded file

		// Get upload file
		formFile, err = c.FormFile("file")
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		// Get upload file buffer
		buffer, err = formFile.Open()
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}
		defer func(buffer multipart.File) {
			err := buffer.Close()
			if err != nil {
				logrus.Errorf("failed to close multipart file buffer: %v", err)
			}
		}(buffer)
		//endregion

		//region Determine MIME type
		if m.Mime != nil && *m.Mime != "" {
			// Use explicitly specified MIME type
			mime = *m.Mime
		} else {
			// Seek back to the start of the file
			_, err = buffer.Seek(0, io.SeekStart)
			if err != nil {
				return uuid.UUID{}, err
			}

			// Try to detect MIME
			pMIME, _ = mimetype.DetectReader(buffer)
			mime = pMIME.String()

			// Seek back to the start of the file
			_, err = buffer.Seek(0, io.SeekStart)
			if err != nil {
				return uuid.UUID{}, err
			}
		}
		//endregion

		//region S3 Object data

		// Generate a new uuid for the file
		id, err = uuid.NewV4()

		// Get url and the key for the entity file
		url := s3.GetS3UrlForEntityFile(entityId, id)
		previousKey := s3.GetS3KeyForEntityFile(entityId, fId.UUID)
		newKey := s3.GetS3KeyForEntityFile(entityId, id)

		//endregion

		//region Upload the Object to S3

		// Delete the old Object

		exists := s3.ObjectExists(previousKey)
		if exists {
			err = s3.DeleteObject(previousKey)
			if err != nil {
				_ = tx.Rollback(ctx)
				return uuid.UUID{}, err
			}
		}

		metadata := map[string]string{
			"version": strconv.FormatInt(fVersion, 10),
			"index":   strconv.FormatInt(m.Index, 10),
			"type":    imageType,
		}

		if m.Deployment != "" {
			metadata["deployment"] = m.Deployment
		}

		if m.Platform != "" {
			metadata["platform"] = m.Platform
		}

		// Upload the new Object
		err = s3.UploadObject(newKey, buffer, mime, true, &metadata, nil)
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		//endregion

		//region Update a file record
		q = `UPDATE files f
SET id=$1,
    url=$2,
    mime=$3,
    size=$4,
    version=(f.version + 1),
    uploaded_by=$5,
    width=$6,
    height=$7,
    updated_at=now()
WHERE f.entity_id = $8
  AND f.type = $9
  AND f.deployment_type = $10
  AND f.platform = $11
  AND f.variation = $12`
		_, err = tx.Exec(ctx, q, id, url, mime, formFile.Size, requester.Id, m.Width, m.Height, entityId, imageType, m.Deployment, m.Platform, m.Index)
		if err != nil {
			_ = tx.Rollback(ctx)
			return uuid.UUID{}, err
		}

		return id, tx.Commit(ctx)

		//endregion

		//endregion
	}

	return uuid.UUID{}, err
}

func LinkFileForAdmin(ctx context.Context, requester *sm.User, entityId uuid.UUID, m FileLinkRequestMetadata) (err error) {
	db := database.DB

	var (
		q   string
		row pgx.Row
		fId pgtypeuuid.UUID
		id  uuid.UUID
	)

	// Try to find existing file with same properties
	q = `SELECT id
FROM files AS f
WHERE f.entity_id = $1
  AND f.type = $2
  AND f.deployment_type = $3
  AND f.platform = $4
  AND f.variation = $5`

	row = db.QueryRow(ctx, q, entityId, m.Type, m.Deployment, m.Platform, m.Index)

	err = row.Scan(&fId)
	if err != nil && err.Error() == "no rows in result set" {
		//region Insert a new file

		//region Add a file record
		q = `INSERT INTO files AS f (
                        id, entity_id, url, type, mime, size, version, deployment_type, platform, uploaded_by, width, height, created_at, updated_at, variation, original_path, hash)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now(), null, $12, $13, $14)`

		_, err = db.Exec(ctx, q, entityId /*$1*/, m.Url /*$2*/, m.Type /*$3*/, m.Mime /*$4*/, m.Size /*$5*/, m.Version /*$6*/, m.Deployment /*$7*/, m.Platform /*$8*/, requester.Id /*$9*/, m.Width /*$10*/, m.Height /*$11*/, m.Index /*$12*/, m.OriginalPath /*$13*/, m.Hash /*$14*/)
		return err
		//endregion

		//endregion
	} else if err == nil {
		//region Replace existing file

		//region S3 Object data

		// Generate a new uuid for the file
		id, err = uuid.NewV4()

		// Get url and the key for the entity file
		previousKey := s3.GetS3KeyForEntityFile(entityId, fId.UUID)

		//endregion

		//region Delete the Object from S3 if required

		exists := s3.ObjectExists(previousKey)
		if exists {
			// Delete the old Object
			err = s3.DeleteObject(previousKey)
			if err != nil {
				return err
			}
		}

		//endregion

		//region Update a file record

		q = `UPDATE files f
SET id=$1,
    url=$2,
    mime=$3,
    size=$4,
    version=(f.version + 1),
    uploaded_by=$5,
    width=$6,
    height=$7,
    updated_at=now(),
    original_path=$13,
	hash=$14
WHERE f.entity_id = $8
  AND f.type = $9
  AND f.deployment_type = $10
  AND f.platform = $11
  AND f.variation = $12`
		_, err = db.Exec(ctx, q, id /*$1*/, m.Url /*$2*/, m.Mime /*$3*/, m.Size /*$4*/, requester.Id /*$5*/, m.Width /*$6*/, m.Height /*$7*/, entityId /*$8*/, m.Type /*$9*/, m.Deployment /*$10*/, m.Platform /*$11*/, m.Index /*$12*/, m.OriginalPath /*$13*/, m.Hash /*$14*/)
		if err != nil {
			return err
		}

		return nil
		//endregion

		//endregion
	}

	return err
}

func LinkFileForRequester(ctx context.Context, requester *sm.User, id uuid.UUID, m FileLinkRequestMetadata) (err error) {
	db := database.DB

	q := `SELECT f.id,
	   f.url,
	   f.version,
	   a.is_owner,
	   a.can_edit
FROM files AS f
	LEFT JOIN entities e on f.entity_id = e.id
	LEFT JOIN accessibles a on e.id = a.entity_id AND a.user_id = $1::uuid
WHERE f.entity_id = $2
  AND f.type = $3
  AND f.deployment_type = $4
  AND f.platform = $5
  AND f.variation = $6`

	row := db.QueryRow(ctx, q, requester.Id, id, m.Type, m.Deployment, m.Platform, m.Index)
	var (
		fId      pgtypeuuid.UUID
		fUrl     string
		fVersion int64
		isOwner  bool
		canEdit  bool
	)

	if err = row.Scan(&fId, &fUrl, &fVersion, &isOwner, &canEdit); err != nil {
		if !isOwner || !canEdit {
			return errors.New("no access")
		}

		if err.Error() == "no rows in result set" {
			q = `INSERT INTO files AS f (
                        id, entity_id, url, type, mime, size, version, deployment_type, platform, uploaded_by, width, height, created_at, updated_at, variation, original_path, hash)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now(), null, $12, $13, $14)`

			_, err = db.Exec(ctx, q, id /*$1*/, m.Url /*$2*/, m.Type /*$3*/, m.Mime /*$4*/, m.Size /*$5*/, m.Version /*$6*/, m.Deployment /*$7*/, m.Platform /*$8*/, requester.Id /*$9*/, m.Width /*$10*/, m.Height /*$11*/, m.Index /*$12*/, m.OriginalPath /*$13*/, m.Hash /*$14*/)
			if err != nil {
				return err
			}

			return nil
		} else {
			return err
		}
	}

	if !isOwner || !canEdit {
		return errors.New("no access")
	}

	q = `UPDATE files f
SET url=$1,
    mime=$2,
    size=$3,
    version=(f.version + 1),
    uploaded_by=$4,
    width=$5,
    height=$6,
    updated_at=now(),
    original_path=$12,
	hash=$13
WHERE f.entity_id = $7
  AND f.type = $8
  AND f.deployment_type = $9
  AND f.platform = $10
  AND f.variation = $11`
	_, err = db.Exec(ctx, q, m.Url /*$1*/, m.Mime /*$2*/, m.Size /*$3*/, requester.Id /*$4*/, m.Width /*$5*/, m.Height /*$6*/, id /*$7*/, m.Type /*$8*/, m.Deployment /*$9*/, m.Platform /*$10*/, m.Index /*$11*/, m.OriginalPath /*$12*/, m.Hash /*$13*/)
	if err != nil {
		return err
	}

	return nil
}

// PreCreateFileForAdmin Pre-create the file record allowing requester to upload file separately to the storage using presigned URL
func PreCreateFileForAdmin(c *fiber.Ctx, requester *sm.User, entityId uuid.UUID, fileId uuid.UUID, m FileUploadLinkRequestMetadata) (err error) {
	db := database.DB
	ctx := c.UserContext()

	var (
		q        string          // query string
		tx       pgx.Tx          // pgx tx
		row      pgx.Row         // pgx row
		fId      pgtypeuuid.UUID // existing db file id
		fVersion int64           // file version from the db
	)

	if PlatformDependentFileTypes[m.Type] {
		if m.Deployment == "" {
			err = fmt.Errorf("file requires a deployment configuration")
			return err
		}

		if m.Platform == "" {
			err = fmt.Errorf("file requires a platform")
			return err
		}
	}

	// Try to find existing file with same properties
	q = `SELECT f.id, f.version
FROM files AS f
WHERE f.entity_id = $1
  AND f.type = $2
  AND f.deployment_type = $3
  AND f.platform = $4
  AND f.variation = $5
  AND f.original_path = $6`

	row = db.QueryRow(ctx, q, entityId /*$1*/, m.Type /*$2*/, m.Deployment /*$3*/, m.Platform /*$4*/, m.Index /*$5*/, m.OriginalPath /*$6*/)

	logrus.Infof("pre-creating a file metadata record, entityId: %v, type: %v, deployment: %v, platform: %v, index: %v, originalPath: %v", entityId, m.Type, m.Deployment, m.Platform, m.Index, m.OriginalPath)

	err = row.Scan(&fId, &fVersion)
	if err != nil && err.Error() == "no rows in result set" {
		logrus.Infof("inserting a new file")

		//region Insert a new file
		tx, err = db.Begin(ctx)
		if err != nil {
			return err // failed to initiate tx
		}

		//region Add a new file metadata record
		q = `INSERT INTO files AS f (id, entity_id, url, type, mime, size, version, deployment_type, platform, uploaded_by, width, height, created_at, updated_at, variation, original_path)
			VALUES ($1::uuid, $2::uuid, $3::text, $4::text, $5::text, $6::bigint, $7::bigint, $8::text, $9::text, $10::uuid, $11::integer, $12::integer, now(), null, $13::bigint, $14::text)`
		_, err = db.Exec(ctx, q, fileId /*$1*/, entityId /*$2*/, m.Url /*$3*/, m.Type /*$4*/, m.Mime /*$5*/, m.Size /*$6*/, m.Version /*$7*/, m.Deployment /*$8*/, m.Platform /*$9*/, requester.Id /*$10*/, m.Width /*$11*/, m.Height /*$12*/, m.Index /*$13*/, m.OriginalPath /*$14*/)
		if err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		//endregion

		err = tx.Commit(ctx)
		if err == nil {
			if m.Type == "pak" {
				if err = NotifyBuildJobCompleted(c, entityId); err != nil {
					fmt.Printf("failed to notify job complete: %s", err.Error())
				}
			}
		}

		return err

		//endregion
	} else if err == nil {
		logrus.Infof("replacing an existing file")
		//region Replace an existing file

		tx, err = db.Begin(ctx)
		if err != nil {
			return err
		}

		//region Remove old file metadata record and S3 object
		// Get url and the key for the entity file
		url := s3.GetS3UrlForEntityFile(entityId, fileId)
		previousKey := s3.GetS3KeyForEntityFile(entityId, fId.UUID)

		// Delete the old file record
		q = `DELETE FROM files f WHERE f.id = $1`
		_, err = tx.Exec(ctx, q, fId.UUID)
		if err != nil {
			_ = tx.Rollback(ctx)
			return err
		}

		// Delete the old object
		exists := s3.ObjectExists(previousKey)
		if exists {
			err = s3.DeleteObject(previousKey)
			if err != nil {
				_ = tx.Rollback(ctx)
				return err
			}
		}
		//endregion

		fVersion++

		// Add a new file record
		q = `INSERT INTO files AS f (id, entity_id, url, type, mime, size, version, deployment_type, platform, uploaded_by, width, height, created_at, updated_at, variation, original_path)
			VALUES ($1::uuid, $2::uuid, $3::text, $4::text, $5::text, $6::bigint, $7::bigint, $8::text, $9::text, $10::uuid, $11::integer, $12::integer, now(), null, $13::bigint, $14::text)`
		_, err = tx.Exec(ctx, q, fileId /*$1*/, entityId /*$2*/, url /*$3*/, m.Type /*$4*/, m.Mime /*$5*/, m.Size /*$6*/, fVersion /*$7*/, m.Deployment /*$8*/, m.Platform /*$9*/, requester.Id /*$10*/, m.Width /*$11*/, m.Height /*$12*/, m.Index /*$13*/, m.OriginalPath /*$14*/)
		if err != nil {
			_ = tx.Rollback(ctx)
			return err
		}

		err = tx.Commit(ctx)
		return err

		//endregion
	}

	return err
}

// PreCreateFileForRequester Pre-create the file record allowing requester to upload file separately to the storage using presigned URL
func PreCreateFileForRequester(c *fiber.Ctx, requester *sm.User, entityId uuid.UUID, fileId uuid.UUID, m FileUploadLinkRequestMetadata) (err error) {
	db := database.DB
	ctx := c.UserContext()

	var (
		q        string          // query string
		tx       pgx.Tx          // pgx tx
		row      pgx.Row         // pgx row
		fId      pgtypeuuid.UUID // existing db file id
		fVersion int64           // file version from the db
		isOwner  bool
		canEdit  bool
	)

	if PlatformDependentFileTypes[m.Type] {
		if m.Deployment == "" {
			err = fmt.Errorf("file requires a deployment configuration")
			return err
		}

		if m.Platform == "" {
			err = fmt.Errorf("file requires a platform")
			return err
		}
	}

	qAccess := `SELECT a.is_owner, a.can_edit
FROM entities e
	LEFT JOIN accessibles a ON a.entity_id = e.id AND a.user_id = $1::uuid
WHERE e.id = $2::uuid`
	rowAccess := db.QueryRow(ctx, qAccess, requester.Id /*$1*/, entityId /*$2*/)
	err = rowAccess.Scan(&isOwner, &canEdit)
	if err != nil {
		if err == pgx.ErrNoRows {
			err = fmt.Errorf("entity not found")
		}
		return err
	}

	// Try to find existing file with same properties
	q = `SELECT f.id, f.version
FROM files AS f
    LEFT JOIN entities e ON f.entity_id = e.id
WHERE f.entity_id = $1
  AND f.type = $2
  AND f.deployment_type = $3
  AND f.platform = $4
  AND f.variation = $5
  AND f.original_path = $6`

	row = db.QueryRow(ctx, q,
		entityId,       /*$1*/
		m.Type,         /*$2*/
		m.Deployment,   /*$3*/
		m.Platform,     /*$4*/
		m.Index,        /*$5*/
		m.OriginalPath, /*$6*/
	)

	logrus.Infof("pre-creating a file metadata record, entityId: %v, type: %v, deployment: %v, platform: %v, index: %v, originalPath: %v", entityId, m.Type, m.Deployment, m.Platform, m.Index, m.OriginalPath)

	err = row.Scan(&fId, &fVersion)
	if err != nil && err.Error() == "no rows in result set" {
		if !isOwner || !canEdit {
			return errors.New("no access")
		}

		logrus.Infof("inserting a new file")

		//region Insert a new file
		tx, err = db.Begin(ctx)
		if err != nil {
			return err // failed to initiate tx
		}

		//region Add a new file metadata record
		q = `INSERT INTO files AS f (id, entity_id, url, type, mime, size, version, deployment_type, platform, uploaded_by, width, height, created_at, updated_at, variation, original_path)
			VALUES ($1::uuid, $2::uuid, $3::text, $4::text, $5::text, $6::bigint, $7::bigint, $8::text, $9::text, $10::uuid, $11::integer, $12::integer, now(), null, $13::bigint, $14::text)`
		_, err = db.Exec(ctx, q, fileId /*$1*/, entityId /*$2*/, m.Url /*$3*/, m.Type /*$4*/, m.Mime /*$5*/, m.Size /*$6*/, m.Version /*$7*/, m.Deployment /*$8*/, m.Platform /*$9*/, requester.Id /*$10*/, m.Width /*$11*/, m.Height /*$12*/, m.Index /*$13*/, m.OriginalPath /*$14*/)
		if err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		//endregion

		err = tx.Commit(ctx)
		if err == nil {
			if m.Type == "pak" {
				if err = NotifyBuildJobCompleted(c, entityId); err != nil {
					fmt.Printf("failed to notify job complete: %s", err.Error())
				}
			}
		}

		return err

		//endregion
	} else if err == nil {
		logrus.Infof("replacing an existing file")
		//region Replace an existing file

		tx, err = db.Begin(ctx)
		if err != nil {
			return err
		}

		//region Remove old file metadata record and S3 object
		// Get url and the key for the entity file
		url := s3.GetS3UrlForEntityFile(entityId, fileId)
		previousKey := s3.GetS3KeyForEntityFile(entityId, fId.UUID)

		// Delete the old file record
		q = `DELETE FROM files f WHERE f.id = $1`
		_, err = tx.Exec(ctx, q, fId.UUID)
		if err != nil {
			_ = tx.Rollback(ctx)
			return err
		}

		// Delete the old object
		exists := s3.ObjectExists(previousKey)
		if exists {
			err = s3.DeleteObject(previousKey)
			if err != nil {
				_ = tx.Rollback(ctx)
				return err
			}
		}
		//endregion

		fVersion++

		// Add a new file record
		q = `INSERT INTO files AS f (id, entity_id, url, type, mime, size, version, deployment_type, platform, uploaded_by, width, height, created_at, updated_at, variation, original_path)
			VALUES ($1::uuid, $2::uuid, $3::text, $4::text, $5::text, $6::bigint, $7::bigint, $8::text, $9::text, $10::uuid, $11::integer, $12::integer, now(), null, $13::bigint, $14::text)`
		_, err = tx.Exec(ctx, q, fileId /*$1*/, entityId /*$2*/, url /*$3*/, m.Type /*$4*/, m.Mime /*$5*/, m.Size /*$6*/, fVersion /*$7*/, m.Deployment /*$8*/, m.Platform /*$9*/, requester.Id /*$10*/, m.Width /*$11*/, m.Height /*$12*/, m.Index /*$13*/, m.OriginalPath /*$14*/)
		if err != nil {
			_ = tx.Rollback(ctx)
			return err
		}

		err = tx.Commit(ctx)
		return err

		//endregion
	}

	return err
}

func DeleteFileForAdmin(ctx context.Context, id uuid.UUID) (err error) {
	db := database.DB

	q := `SELECT
f.id fileId,
f.entity_id fileEntityId,
f.url fileUrl
FROM files f
WHERE f.id = $1`

	var (
		fId pgtypeuuid.UUID
		eId pgtypeuuid.UUID
		url string
	)

	row := db.QueryRow(ctx, q, id)
	if err = row.Scan(&fId, &eId, &url); err != nil {
		return err
	}

	var key = s3.GetS3KeyForEntityFile(eId.UUID, fId.UUID)
	if err = s3.DeleteObject(key); err != nil {
		return err
	}

	q = `DELETE FROM files f WHERE f.id = $1`

	_, err = db.Exec(ctx, q, id /*$1*/)

	if err != nil {
		return err
	}

	return nil
}

func DeleteFileForRequester(ctx context.Context, requester *sm.User, id uuid.UUID) (err error) {
	db := database.DB

	q := `SELECT
f.id fileId,
f.entity_id fileEntityId,
f.url fileUrl,
a.is_owner,
a.can_delete
FROM files f
	LEFT JOIN entities e ON f.entity_id = e.id
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
WHERE f.id = $2`

	var (
		fId       pgtypeuuid.UUID
		eId       pgtypeuuid.UUID
		url       string
		isOwner   bool
		canDelete bool
	)

	row := db.QueryRow(ctx, q, id, requester.Id)
	if err = row.Scan(&fId, &eId, &url, &isOwner, &canDelete); err != nil {
		return err
	}

	if !isOwner || !canDelete {
		return errors.New("no access")
	}

	var key = s3.GetS3KeyForEntityFile(eId.UUID, fId.UUID)
	if err = s3.DeleteObject(key); err != nil {
		return err
	}

	q = `DELETE FROM files f WHERE f.id = $1`

	_, err = db.Exec(ctx, q, id /*$1*/)

	if err != nil {
		return err
	}

	return nil
}

func GetFileForAdmin(ctx context.Context, fileId uuid.UUID) (file *File, err error) {
	db := database.DB

	q := `SELECT
f.id fileId,
f.entity_id fileEntityId,
f.type fileType,
f.url fileUrl,
f.mime fileMime,
f.size fileSize,
f.version fileVersion,
f.deployment_type fileDeployment,
f.platform filePlatform,
f.uploaded_by fileUploadedBy,
f.width fileWidth,
f.height fileHeight,
f.created_at fileCreatedAt,
f.updated_at fileUpdatedAt,
f.variation fileVariation,
f.original_path fileOriginalPath
FROM files f
WHERE id = $1::uuid
ORDER BY type, platform, deployment_type, version DESC, variation`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, fileId /*$1*/)

	if err != nil {
		return nil, err
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetFileForAdmin")
	}()

	for rows.Next() {
		var (
			id           pgtypeuuid.UUID
			eId          pgtypeuuid.UUID
			fileType     string
			url          string
			mime         *string
			size         *int64
			version      int
			deployment   string
			platform     string
			uploadedBy   pgtypeuuid.UUID
			width        *int
			height       *int
			createdAt    pgtype.Timestamp
			updatedAt    *pgtype.Timestamp
			variation    int
			originalPath string
		)

		err = rows.Scan(
			&id,
			&eId,
			&fileType,
			&url,
			&mime,
			&size,
			&version,
			&deployment,
			&platform,
			&uploadedBy,
			&width,
			&height,
			&createdAt,
			&updatedAt,
			&variation,
			&originalPath,
		)
		if err != nil {
			return nil, err
		}

		var e File
		e.Id = &id.UUID
		e.EntityId = &eId.UUID
		e.Type = fileType
		e.Url = url
		e.Mime = mime
		e.Size = size
		e.Version = version
		e.Deployment = deployment
		e.Platform = platform
		e.OriginalPath = &originalPath
		if uploadedBy.Status == pgtype.Present {
			e.UploadedBy = &uploadedBy.UUID
		}
		e.Width = width
		e.Height = height
		e.CreatedAt = createdAt.Time
		if updatedAt != nil {
			e.UpdatedAt = &updatedAt.Time
		}

		file = &e
	}

	return file, err
}

func GetFileForRequester(ctx context.Context, requester *sm.User, fileId uuid.UUID) (file *File, err error) {
	db := database.DB

	q := `SELECT
f.id fileId,
f.entity_id fileEntityId,
f.type fileType,
f.url fileUrl,
f.mime fileMime,
f.size fileSize,
f.version fileVersion,
f.deployment_type fileDeployment,
f.platform filePlatform,
f.uploaded_by fileUploadedBy,
f.width fileWidth,
f.height fileHeight,
f.created_at fileCreatedAt,
f.updated_at fileUpdatedAt,
f.variation fileVariation,
f.original_path fileOriginalPath
FROM files f
    LEFT JOIN entities e ON f.entity_id = e.id
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid 
WHERE f.id = $2::uuid AND (e.public OR a.can_view)
ORDER BY type, platform, deployment_type, version DESC, variation`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, requester.Id /*$1*/, fileId /*$2*/)

	if err != nil {
		return nil, err
	}

	defer func() {
		rows.Close()
		database.LogPgxStat(fmt.Sprintf("GetFileForRequester: %s", requester.Id))
	}()

	for rows.Next() {
		var (
			id           pgtypeuuid.UUID
			eId          pgtypeuuid.UUID
			fileType     string
			url          string
			mime         *string
			size         *int64
			version      int
			deployment   string
			platform     string
			uploadedBy   pgtypeuuid.UUID
			width        *int
			height       *int
			createdAt    pgtype.Timestamp
			updatedAt    *pgtype.Timestamp
			variation    int
			originalPath string
		)

		err = rows.Scan(
			&id,
			&eId,
			&fileType,
			&url,
			&mime,
			&size,
			&version,
			&deployment,
			&platform,
			&uploadedBy,
			&width,
			&height,
			&createdAt,
			&updatedAt,
			&variation,
			&originalPath,
		)
		if err != nil {
			return nil, err
		}

		var e File
		e.Id = &id.UUID
		e.EntityId = &eId.UUID
		e.Type = fileType
		e.Url = url
		e.Mime = mime
		e.Size = size
		e.Version = version
		e.Deployment = deployment
		e.Platform = platform
		e.OriginalPath = &originalPath
		if uploadedBy.Status == pgtype.Present {
			e.UploadedBy = &uploadedBy.UUID
		}
		e.Width = width
		e.Height = height
		e.CreatedAt = createdAt.Time
		if updatedAt != nil {
			e.UpdatedAt = &updatedAt.Time
		}

		file = &e
	}

	return file, err
}

func GetFileForRequesterByUrl(ctx context.Context, requester *sm.User, url string) (file *File, err error) {
	db := database.DB

	q := `SELECT
f.id fileId,
f.entity_id fileEntityId,
f.type fileType,
f.url fileUrl,
f.mime fileMime,
f.size fileSize,
f.version fileVersion,
f.deployment_type fileDeployment,
f.platform filePlatform,
f.uploaded_by fileUploadedBy,
f.width fileWidth,
f.height fileHeight,
f.created_at fileCreatedAt,
f.updated_at fileUpdatedAt,
f.variation fileVariation,
f.original_path fileOriginalPath
FROM files f
    LEFT JOIN entities e ON f.entity_id = e.id
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid 
WHERE f.url = $2::text AND (e.public OR a.can_view)
ORDER BY type, platform, deployment_type, version DESC, variation`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, requester.Id /*$1*/, url /*$2*/)

	if err != nil {
		return nil, err
	}

	defer func() {
		rows.Close()
		database.LogPgxStat(fmt.Sprintf("GetFileForRequester: %s", requester.Id))
	}()

	for rows.Next() {
		var (
			id           pgtypeuuid.UUID
			eId          pgtypeuuid.UUID
			fileType     string
			url          string
			mime         *string
			size         *int64
			version      int
			deployment   string
			platform     string
			uploadedBy   pgtypeuuid.UUID
			width        *int
			height       *int
			createdAt    pgtype.Timestamp
			updatedAt    *pgtype.Timestamp
			variation    int
			originalPath string
		)

		err = rows.Scan(
			&id,
			&eId,
			&fileType,
			&url,
			&mime,
			&size,
			&version,
			&deployment,
			&platform,
			&uploadedBy,
			&width,
			&height,
			&createdAt,
			&updatedAt,
			&variation,
			&originalPath,
		)
		if err != nil {
			return nil, err
		}

		var e File
		e.Id = &id.UUID
		e.EntityId = &eId.UUID
		e.Type = fileType
		e.Url = url
		e.Mime = mime
		e.Size = size
		e.Version = version
		e.Deployment = deployment
		e.Platform = platform
		e.OriginalPath = &originalPath
		if uploadedBy.Status == pgtype.Present {
			e.UploadedBy = &uploadedBy.UUID
		}
		e.Width = width
		e.Height = height
		e.CreatedAt = createdAt.Time
		if updatedAt != nil {
			e.UpdatedAt = &updatedAt.Time
		}

		file = &e
	}

	return file, err
}

func IndexAvatars(ctx context.Context, userId uuid.UUID, offset int64, limit int64) (avatars []File, total int64, err error) {
	db := database.DB
	q := `SELECT COUNT(f.id) FROM files f WHERE entity_id = $1 AND type = $2`

	row := db.QueryRow(ctx, q, userId, "image_avatar")

	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to scan total %s @ %s: %v", AvatarPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get %s", AvatarPlural)
	}
	//endregion

	if total == 0 {
		return nil, total, nil
	}

	// Query to request users with avatar image files and presence data
	q = `SELECT id, url, mime FROM files WHERE entity_id = $1 AND type = $2 LIMIT $3 OFFSET $4`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, userId /*$1*/, "image_avatar" /*$2*/, limit, offset)

	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", AvatarPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get %s", AvatarPlural)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexAvatars")
	}()

	for rows.Next() {
		var avatar File

		err = rows.Scan(
			&avatar.Id,
			&avatar.Url,
			&avatar.Mime,
		)

		if err != nil {
			logrus.Errorf("failed to scan %s @ %s: %v", AvatarPlural, reflect.FunctionName(), err)
			return nil, -1, fmt.Errorf("failed to get %s", AvatarPlural)
		}

		avatars = append(avatars, avatar)
	}

	return avatars, total, nil
}
