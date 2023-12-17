package model

import (
	"context"
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	st "dev.hackerman.me/artheon/veverse-shared/telegram"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"github.com/jackc/pgtype"
	pgtypeuuid "github.com/jackc/pgtype/ext/gofrs-uuid"
	"github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
	"veverse-api/aws/s3"
	"veverse-api/database"
	"veverse-api/reflect"
)

const (
	jobSingular = "job"
)

var SupportedJobTypes = map[string]bool{
	"Release":  true, // App release building and deployment
	"Package":  true, // Package processing
	"Launcher": true,
}

var SupportedJobDeployments = map[string]bool{
	"Client": true,
	"Server": true,
	"SDK":    true,
}

var SupportedJobStatuses = map[string]bool{
	"unclaimed":  true, // Job is scheduled and not claimed by any worker
	"claimed":    true, // Job is claimed by a worker
	"processing": true, // Job is currently processed by a worker
	"uploading":  true, // Job is currently uploading its results to the cloud storage
	"completed":  true, // Job has been completed successfully
	"error":      true, // Job failed with an error message
	"cancelled":  true, // Job has been cancelled
}

// Job struct
type Job struct {
	Identifier
	Files         []File    `json:"files,omitempty"` // Job can have files (of its entity)
	Platform      string    `json:"platform,omitempty"`
	Configuration string    `json:"configuration,omitempty"`
	Type          string    `json:"type,omitempty"`
	Deployment    string    `json:"deployment,omitempty"`
	App           *App      `json:"app,omitempty"`
	Release       *Release  `json:"release,omitempty"`
	Package       *Package  `json:"package,omitempty"`
	Status        string    `json:"status,omitempty"`
	OwnerId       uuid.UUID `json:"ownerId,omitempty"`
	EntityId      uuid.UUID `json:"entityId,omitempty"`
}

type JobRequestMetadata struct {
	IdRequestMetadata
	Platform   string `json:"platform,omitempty"`
	Type       string `json:"type,omitempty"`
	Deployment string `json:"deployment,omitempty"`
}

type CreateJobRequestMetadata struct {
	Platform      string    `json:"platform,omitempty"`
	Type          string    `json:"type,omitempty"`
	Deployment    string    `json:"deployment,omitempty"`
	Configuration string    `json:"configuration,omitempty"`
	EntityId      uuid.UUID `json:"entityId,omitempty"`
}

type JobStatusRequestMetadata struct {
	IdRequestMetadata
	Status  string `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
}

type JobLogRequestMetadata struct {
	IdRequestMetadata
	Warnings []string `json:"warnings,omitempty"`
	Errors   []string `json:"errors,omitempty"`
}

type IndexJobsRequestMetadata struct {
	BatchRequestMetadata
	Status *string `json:"status,omitempty"`
	Type   *string `json:"type,omitempty"`
}

type IndexJobs struct {
	Identifier
	OwnerId        *uuid.UUID `json:"userId,omitempty"`
	WorkerId       *uuid.UUID `json:"workerId,omitempty"`
	PackageName    *string    `json:"packageName,omitempty"`
	AppName        *string    `json:"appName,omitempty"`
	AppDescription *string    `json:"appDescription,omitempty"`
	ReleaseVersion *string    `json:"releaseVersion,omitempty"`
	Configuration  *string    `json:"configuration,omitempty"`
	Status         *string    `json:"status,omitempty"`
	Map            *string    `json:"map,omitempty"`
	Type           *string    `json:"type,omitempty"`
	Deployment     *string    `json:"deployment,omitempty"`
	Message        *string    `json:"message,omitempty"`
	Platform       *string    `json:"platform,omitempty"`
	CreatedAt      *time.Time `json:"createdAt,omitempty"`
	UpdatedAt      *time.Time `json:"updatedAt,omitempty"`
}

// GetUnclaimedJob Get the next unclaimed pending job
func GetUnclaimedJob(ctx context.Context, workerId uuid.UUID, platforms []string, types []string, deployments []string) (entity *Job, err error) {
	db := database.DB

	q := `SELECT
    j.id				jobId,
    j.platform			jobPlatform,
    j.configuration		jobConfiguration,
    j.type 				jobType,
    j.deployment		jobDeployment,
	r.id				releaseId,
	r.version 			releaseVersion,
	r.code_version 		releaseCodeVersion,
	r.content_version 	releaseContentVersion,
	ra.id 				releaseAppId,
	ra.name 			releaseAppName,
	ra.description 		releaseAppDescription,
	ra.url 				releaseAppUrl,
	ra.external 		releaseAppExternal,
	a.id				appId,
	a.name				appName,
	a.description 		appDescription,
	a.url 				appUrl,
	a.external 			appExternal,
	p.id				packageId,
	p.name				packageName,
	p.map				packageMap,
	p.version			packageVersion,
	p.release_name		packageBase,
	f.id				fileId,
	f.entity_id			fileEntityId,
	f.type				fileType,
	f.url				fileUrl,
	f.mime				fileMime,
	f.size				fileSize,
	f.version			fileVersion,
	f.deployment_type	fileDeployment,
	f.platform			filePlatform,
	f.uploaded_by		fileUploadedBy,
	f.created_at		fileCreatedAt,
	f.updated_at		fileUpdatedAt,
	f.variation			fileVariation,
	f.original_path		fileOriginalPath
FROM jobs j
	LEFT JOIN entities e ON j.entity_id = e.id /* app, release or package */ 
    LEFT JOIN mods p ON e.id = p.id /* package */
	LEFT JOIN releases r ON e.id = r.id /* release */
    LEFT JOIN apps ra ON r.app_id = ra.id /* release apps */
    LEFT JOIN apps a ON e.id = a.id /* apps */
	LEFT JOIN files f ON f.entity_id = j.entity_id AND f.type IN ('uplugin', 'uplugin_content', 'image-app-icon') /* possibly required source files */
WHERE j.status = 'unclaimed' /* only unclaimed jobs */
  AND (j.platform = ANY ($1::text[])) /* only platforms supported by the builder */ 
  AND (j.type = ANY ($2::text[])) /* only types supported by the builder */
  AND (j.deployment = ANY ($3::text[])) /* only deployments supported by the builder */
ORDER BY r.version DESC, p.version DESC`

	var (
		rows pgx.Rows
	)

	p := fmt.Sprintf("{%s}", strings.Join(platforms, ","))
	t := fmt.Sprintf("{%s}", strings.Join(types, ","))
	d := fmt.Sprintf("{%s}", strings.Join(deployments, ","))

	rows, err = db.Query(ctx, q, p /*$1*/, t /*$2*/, d /*$3*/)
	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", jobSingular, reflect.FunctionName(), err)
	}

	var currentJobId uuid.UUID

	defer func() {
		rows.Close()
		database.LogPgxStat("GetUnclaimedJob")
	}()

	for rows.Next() {
		// Declare all vars to store values
		var (
			jobId                 pgtypeuuid.UUID
			jobPlatform           string
			jobConfiguration      string
			jobType               string
			jobDeployment         string
			releaseId             *pgtypeuuid.UUID
			releaseVersion        *string
			releaseCodeVersion    *string
			releaseContentVersion *string
			releaseAppId          *pgtypeuuid.UUID
			releaseAppName        *string
			releaseAppDescription *string
			releaseAppUrl         *string
			releaseAppExternal    *bool
			appId                 *pgtypeuuid.UUID
			appName               *string
			appDescription        *string
			appUrl                *string
			appExternal           *bool
			packageId             *pgtypeuuid.UUID
			packageName           *string
			packageMaps           *string
			packageVersion        *string
			packageBaseRelease    *string
			fileId                *pgtypeuuid.UUID
			fileEntityId          *pgtypeuuid.UUID
			fileType              *string
			fileUrl               *string
			fileMime              *string
			fileSize              *int64
			fileVersion           *int
			fileDeployment        *string
			filePlatform          *string
			fileUploadedBy        *pgtypeuuid.UUID
			fileCreatedAt         *time.Time
			fileUpdatedAt         *time.Time
			fileVariation         *int
			fileOriginalPath      *string
		)
		// Scan rows values into temp vars
		err = rows.Scan(
			&jobId,
			&jobPlatform,
			&jobConfiguration,
			&jobType,
			&jobDeployment,
			&releaseId,
			&releaseVersion,
			&releaseCodeVersion,
			&releaseContentVersion,
			&releaseAppId,
			&releaseAppName,
			&releaseAppDescription,
			&releaseAppUrl,
			&releaseAppExternal,
			&appId,
			&appName,
			&appDescription,
			&appUrl,
			&appExternal,
			&packageId,
			&packageName,
			&packageMaps,
			&packageVersion,
			&packageBaseRelease,
			&fileId,
			&fileEntityId,
			&fileType,
			&fileUrl,
			&fileMime,
			&fileSize,
			&fileVersion,
			&fileDeployment,
			&filePlatform,
			&fileUploadedBy,
			&fileCreatedAt,
			&fileUpdatedAt,
			&fileVariation,
			&fileOriginalPath,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan %s @ %s: %v", jobSingular, reflect.FunctionName(), err)
		}

		if jobId.Status == pgtype.Null {
			// Skip invalid job row
			continue
		}

		if currentJobId.IsNil() {
			currentJobId = jobId.UUID
		} else if currentJobId != jobId.UUID {
			// Skip another job row
			continue
		}

		var file *File
		if fileId != nil && fileId.Status != pgtype.Null {
			file = new(File)
			file.Id = &fileId.UUID
			if fileEntityId != nil && fileEntityId.Status != pgtype.Null {
				file.EntityId = &fileEntityId.UUID
			}
			if fileType != nil {
				file.Type = *fileType
			}
			if fileUrl != nil {
				file.Url = *fileUrl
			}
			file.Mime = fileMime
			file.Size = fileSize
			if fileVersion != nil {
				file.Version = *fileVersion
			}
			if fileDeployment != nil {
				file.Deployment = *fileDeployment
			}
			if filePlatform != nil {
				file.Platform = *filePlatform
			}
			if fileUploadedBy != nil && fileUploadedBy.Status != pgtype.Null {
				file.UploadedBy = &fileUploadedBy.UUID
			}
			if fileCreatedAt != nil {
				file.CreatedAt = *fileCreatedAt
			}
			file.UpdatedAt = fileUpdatedAt
			if fileVariation != nil {
				file.Index = *fileVariation
			}
			file.OriginalPath = fileOriginalPath

			if file.Type == "uplugin" || file.Type == "uplugin_content" {
				if file.EntityId != nil && file.Id != nil {
					key := s3.GetS3KeyForEntityFile(*file.EntityId, *file.Id)
					url, err := s3.GetS3PresignedDownloadUrlForEntityFile(key, 30*time.Minute)
					if err != nil {
						return nil, fmt.Errorf("failed to get a presigned url for a file: %v", err)
					}
					file.Url = url
				}
			}
		}

		if entity != nil {
			// Add a next file to the previously created entity
			if file != nil && !containsFile(entity.Files, *file.Id) {
				entity.Files = append(entity.Files, *file)
			}
		} else {
			var e Job
			e.Id = &jobId.UUID
			e.Type = jobType
			e.Deployment = jobDeployment
			e.Platform = jobPlatform
			e.Configuration = jobConfiguration
			// Add a file to the newly created entity
			if file != nil && !containsFile(e.Files, *file.Id) {
				e.Files = append(e.Files, *file)
			}
			entity = &e
		}

		if entity.Package == nil {
			if packageId != nil && packageId.Status == pgtype.Present {
				entity.Package = new(Package)
				entity.Package.Id = &packageId.UUID
				if packageName != nil {
					entity.Package.Name = *packageName
				}
				if packageVersion != nil {
					entity.Package.Version = *packageVersion
				}
				if packageMaps != nil {
					entity.Package.Map = *packageMaps
				}
				if packageVersion != nil {
					entity.Package.Version = *packageVersion
				}
				if packageBaseRelease != nil {
					entity.Package.Release = *packageBaseRelease
				}
			}
		}
		if entity.Release == nil {
			if releaseId != nil && releaseId.Status == pgtype.Present {
				entity.Release = new(Release)
				entity.Release.Id = &releaseId.UUID
				if releaseVersion != nil {
					entity.Release.Version = *releaseVersion
				}
				if releaseCodeVersion != nil {
					entity.Release.CodeVersion = *releaseCodeVersion
				}
				if releaseContentVersion != nil {
					entity.Release.ContentVersion = *releaseContentVersion
				}
				if releaseAppId != nil {
					entity.Release.AppId = &releaseAppId.UUID
				}
				if releaseAppName != nil {
					entity.Release.AppName = *releaseAppName
				}
				if releaseAppDescription != nil {
					entity.Release.AppDescription = releaseAppDescription
				}
				if releaseAppExternal != nil {
					entity.Release.AppExternal = releaseAppExternal
				}
			}
		}
		if entity.App == nil {
			if appId != nil && appId.Status == pgtype.Present {
				entity.App = new(App)
				entity.App.Id = &appId.UUID
				if appName != nil {
					entity.App.Name = *appName
				}
				if appDescription != nil {
					entity.App.Description = *appDescription
				}
				if appUrl != nil {
					entity.App.Url = *appUrl
				}
				if appExternal != nil {
					entity.App.External = *appExternal
				}
			}
		}
	}

	//tx, err := db.Begin(ctx)
	//if err != nil {
	//	return nil, fmt.Errorf("failed to begin transaction: %s", err.Error())
	//}

	//defer func(tx pgx.Tx, ctx context.Context) {
	//	database.LogPgxStat("unclaimed job rollback")
	//
	//	err := tx.Rollback(ctx)
	//	if err != nil && err != pgx.ErrTxClosed {
	//		logrus.Errorf("failed to rollback tx: %v", err)
	//	}
	//
	//	err = tx.Conn().Close(ctx)
	//	if err != nil {
	//		logrus.Errorf("failed to close tx connection: %v", err)
	//	}
	//}(tx, ctx)

	q = `UPDATE jobs j SET status = 'claimed'::text, worker_id = $1::uuid WHERE j.id = $2::uuid`

	_, err = db.Exec(ctx, q, workerId /*$1*/, currentJobId /*$2*/)
	if err != nil {
		return nil, fmt.Errorf("failed to update %s @ %s: %v", jobSingular, reflect.FunctionName(), err)
	}

	//err = tx.Commit(ctx)
	//if err != nil {
	//	return nil, fmt.Errorf("failed to commit query %s @ %s: %v", jobSingular, reflect.FunctionName(), err)
	//}

	return entity, err
}

// GetJobForAdmin Get the next unclaimed pending job
func GetJobForAdmin(c *fiber.Ctx, id uuid.UUID) (entity *Job, err error) {
	db := database.DB
	ctx := c.Context()

	q := `SELECT
    j.id				jobId,
    j.platform			jobPlatform,
    j.configuration		jobConfiguration,
    j.type 				jobType,
    j.deployment		jobDeployment,
    j.status			jobStatus,
    j.owner_id          jobOwnerId,
    j.entity_id         jobEntityId,
	r.id				releaseId,
	r.version 			releaseVersion,
	r.code_version 		releaseCodeVersion,
	r.content_version 	releaseContentVersion,
	ra.id				releaseAppId,
	ra.name				releaseAppName,
	ra.description		releaseAppDescription,
	ra.url				releaseAppUrl,
	ra.external			releaseAppExternal,
	a.id				appId,
	a.name				appName,
	a.description		appDescription,
	a.url				appUrl,
	a.external			appExternal,
	p.id				packageId,
	p.name				packageName,
	p.map				packageMap,
	p.version			packageVersion,
	p.release_name		packageBase,
	f.id				fileId,
	f.entity_id			fileEntityId,
	f.type				fileType,
	f.url				fileUrl,
	f.mime				fileMime,
	f.size				fileSize,
	f.version			fileVersion,
	f.deployment_type	fileDeployment,
	f.platform			filePlatform,
	f.uploaded_by		fileUploadedBy,
	f.created_at		fileCreatedAt,
	f.updated_at		fileUpdatedAt,
	f.variation			fileVariation,
	f.original_path		fileOriginalPath
FROM jobs j
	LEFT JOIN entities e ON j.entity_id = e.id /* package or release */
    LEFT JOIN mods p ON e.id = p.id
	LEFT JOIN releases r ON e.id = r.id
    LEFT JOIN apps ra ON r.app_id = ra.id
    LEFT JOIN apps a ON e.id = a.id
	LEFT JOIN files f ON f.entity_id = j.entity_id AND f.type IN ('uplugin', 'uplugin_content') /* possibly required source files */
WHERE j.id = $1
ORDER BY r.version DESC, p.version DESC`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, id /*$1*/)
	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", jobSingular, reflect.FunctionName(), err)
	}

	var currentJobId uuid.UUID

	defer func() {
		rows.Close()
		database.LogPgxStat("GetJobForAdmin")
	}()
	for rows.Next() {
		// Declare all vars to store values
		var (
			jobId                 pgtypeuuid.UUID
			jobPlatform           string
			jobConfiguration      string
			jobType               string
			jobDeployment         string
			jobStatus             string
			jobOwnerId            *pgtypeuuid.UUID
			entityId              *pgtypeuuid.UUID
			releaseId             *pgtypeuuid.UUID
			releaseVersion        *string
			releaseCodeVersion    *string
			releaseContentVersion *string
			releaseAppId          *pgtypeuuid.UUID
			releaseAppName        *string
			releaseAppDescription *string
			releaseAppUrl         *string
			releaseAppExternal    *bool
			appId                 *pgtypeuuid.UUID
			appName               *string
			appDescription        *string
			appUrl                *string
			appExternal           *bool
			packageId             *pgtypeuuid.UUID
			packageName           *string
			packageMaps           *string
			packageVersion        *string
			packageBaseRelease    *string
			fileId                *pgtypeuuid.UUID
			fileEntityId          *pgtypeuuid.UUID
			fileType              *string
			fileUrl               *string
			fileMime              *string
			fileSize              *int64
			fileVersion           *int
			fileDeployment        *string
			filePlatform          *string
			fileUploadedBy        *pgtypeuuid.UUID
			fileCreatedAt         *time.Time
			fileUpdatedAt         *time.Time
			fileVariation         *int
			fileOriginalPath      *string
		)
		// Scan rows values into temp vars
		err = rows.Scan(
			&jobId,
			&jobPlatform,
			&jobConfiguration,
			&jobType,
			&jobDeployment,
			&jobStatus,
			&jobOwnerId,
			&entityId,
			&releaseId,
			&releaseVersion,
			&releaseCodeVersion,
			&releaseContentVersion,
			&releaseAppId,
			&releaseAppName,
			&releaseAppDescription,
			&releaseAppUrl,
			&releaseAppExternal,
			&appId,
			&appName,
			&appDescription,
			&appUrl,
			&appExternal,
			&packageId,
			&packageName,
			&packageMaps,
			&packageVersion,
			&packageBaseRelease,
			&fileId,
			&fileEntityId,
			&fileType,
			&fileUrl,
			&fileMime,
			&fileSize,
			&fileVersion,
			&fileDeployment,
			&filePlatform,
			&fileUploadedBy,
			&fileCreatedAt,
			&fileUpdatedAt,
			&fileVariation,
			&fileOriginalPath,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan %s @ %s: %v", jobSingular, reflect.FunctionName(), err)
		}

		if jobId.Status == pgtype.Null {
			// Skip invalid job row
			continue
		}

		if currentJobId.IsNil() {
			currentJobId = jobId.UUID
		} else if currentJobId != jobId.UUID {
			// Skip another job row
			continue
		}

		var file *File
		if fileId != nil && fileId.Status != pgtype.Null {
			file = new(File)
			file.Id = &fileId.UUID
			if fileEntityId != nil && fileEntityId.Status != pgtype.Null {
				file.EntityId = &fileEntityId.UUID
			}
			if fileType != nil {
				file.Type = *fileType
			}
			if fileUrl != nil {
				file.Url = *fileUrl
			}
			file.Mime = fileMime
			file.Size = fileSize
			if fileVersion != nil {
				file.Version = *fileVersion
			}
			if fileDeployment != nil {
				file.Deployment = *fileDeployment
			}
			if filePlatform != nil {
				file.Platform = *filePlatform
			}
			if fileUploadedBy != nil && fileUploadedBy.Status != pgtype.Null {
				file.UploadedBy = &fileUploadedBy.UUID
			}
			if fileCreatedAt != nil {
				file.CreatedAt = *fileCreatedAt
			}
			file.UpdatedAt = fileUpdatedAt
			if fileVariation != nil {
				file.Index = *fileVariation
			}
			file.OriginalPath = fileOriginalPath

			if file.Type == "uplugin" || file.Type == "uplugin_content" {
				if file.EntityId != nil && file.Id != nil {
					key := s3.GetS3KeyForEntityFile(*file.EntityId, *file.Id)
					url, err := s3.GetS3PresignedDownloadUrlForEntityFile(key, 30*time.Minute)
					if err != nil {
						return nil, fmt.Errorf("failed to get a presigned url for a file: %v", err)
					}
					file.Url = url
				}
			}
		}

		if entity != nil {
			// Add a next file to the previously created entity
			if file != nil && !containsFile(entity.Files, *file.Id) {
				entity.Files = append(entity.Files, *file)
			}
		} else {
			var e Job
			e.Id = &jobId.UUID
			e.Type = jobType
			e.Deployment = jobDeployment
			e.Platform = jobPlatform
			e.Configuration = jobConfiguration
			e.Status = jobStatus
			if entityId != nil {
				e.EntityId = entityId.UUID
			}
			if jobOwnerId != nil {
				e.OwnerId = jobOwnerId.UUID
			}
			// Add a file to the newly created entity
			if file != nil && !containsFile(e.Files, *file.Id) {
				e.Files = append(e.Files, *file)
			}
			entity = &e
		}

		if entity.Package == nil {
			if packageId != nil && packageId.Status == pgtype.Present {
				entity.Package = new(Package)
				entity.Package.Id = &packageId.UUID
				if packageName != nil {
					entity.Package.Name = *packageName
				}
				if packageVersion != nil {
					entity.Package.Version = *packageVersion
				}
				if packageMaps != nil {
					entity.Package.Map = *packageMaps
				}
				if packageVersion != nil {
					entity.Package.Version = *packageVersion
				}
				if packageBaseRelease != nil {
					entity.Package.Release = *packageBaseRelease
				}
			}
		}
		if entity.Release == nil {
			if releaseId != nil && releaseId.Status == pgtype.Present {
				entity.Release = new(Release)
				entity.Release.Id = &releaseId.UUID
				if releaseVersion != nil {
					entity.Release.Version = *releaseVersion
				}
				if releaseCodeVersion != nil {
					entity.Release.CodeVersion = *releaseCodeVersion
				}
				if releaseContentVersion != nil {
					entity.Release.ContentVersion = *releaseContentVersion
				}
				if releaseAppId != nil {
					entity.Release.AppId = &releaseAppId.UUID
				}
				if releaseAppName != nil {
					entity.Release.AppName = *releaseAppName
				}
			}
		}
		if entity.App == nil {
			if appId != nil && appId.Status == pgtype.Present {
				entity.App = new(App)
				entity.App.Id = &appId.UUID
				if appName != nil {
					entity.App.Name = *appName
				}
				if appDescription != nil {
					entity.App.Description = *appDescription
				}
				if appUrl != nil {
					entity.App.Url = *appUrl
				}
				if appExternal != nil {
					entity.App.External = *appExternal
				}
			}
		}
	}

	return entity, err
}

// UpdateJobStatus Updates a job status
func UpdateJobStatus(c *fiber.Ctx, id uuid.UUID, status string, message string) (err error) {
	db := database.DB
	ctx := c.Context()

	//tx, err := db.Begin(ctx)
	//if err != nil {
	//	return fmt.Errorf("failed to begin transaction: %s", err.Error())
	//}
	//
	//defer func(tx pgx.Tx, ctx context.Context) {
	//	err := tx.Rollback(ctx)
	//	if err != nil && err != pgx.ErrTxClosed {
	//		logrus.Errorf("failed to rollback tx: %v", err)
	//	}
	//}(tx, ctx)

	q := `UPDATE jobs j SET status = $1::text, message = $2::text WHERE j.id = $3::uuid`

	_, err = db.Exec(ctx, q, status /*$1*/, message /*$2*/, id.String() /*$3*/)
	if err != nil {
		return fmt.Errorf("failed to update %s @ %s: %v", jobSingular, reflect.FunctionName(), err)
	}

	//err = tx.Commit(ctx)
	//if err != nil {
	//	return fmt.Errorf("failed to commit query %s @ %s: %v", jobSingular, reflect.FunctionName(), err)
	//}

	job, err := GetJobForAdmin(c, id)
	if err != nil || job == nil {
		return fmt.Errorf("failed to get a job: %v", err)
	}

	user, err := GetBasicUserInfo(c.Context(), job.OwnerId)
	if err != nil {
		return fmt.Errorf("failed to get job owner: %v", err)
	}

	if job.Type == "Package" {
		// Send the job status email to the requester
		if err = SendPackageJobStatusEmail(c, user, job.EntityId, job.Status); err != nil {
			return fmt.Errorf("failed to send a job status email: %v", err)
		}
	}

	// Notify the discord channel using the hook
	if job.Type == "Package" {
		if job.Package != nil {
			if err = st.SendTelegramMessage(fmt.Sprintf("Package %s %s-%s-%s-%s (%s) job status: %s", job.Package.Name, job.Package.Version, job.Configuration, job.Deployment, job.Platform, job.Package.Id, job.Status)); err != nil {
				logrus.Errorf("failed to send a telegram message: %v", err)
			}
		} else {
			if err = st.SendTelegramMessage(fmt.Sprintf("Package job %s status: %s", job.Id, job.Status)); err != nil {
				logrus.Errorf("failed to send a telegram message: %v", err)
			}
		}
	} else if job.Type == "Release" {
		if job.Release != nil {
			if err = st.SendTelegramMessage(fmt.Sprintf("App %s release %s-%s-%s-%s (%s) job status: %s", job.Release.AppName, job.Release.Version, job.Configuration, job.Deployment, job.Platform, job.Release.Id, job.Status)); err != nil {
				logrus.Errorf("failed to send a telegram message: %v", err)
			}
		} else {
			if err = st.SendTelegramMessage(fmt.Sprintf("App release job %s status: %s", job.Id, job.Status)); err != nil {
				logrus.Errorf("failed to send a telegram message: %v", err)
			}
		}
	} else if job.Type == "Launcher" {
		if job.App != nil {
			if err = st.SendTelegramMessage(fmt.Sprintf("Launcher %s %s job status: %s", job.App.Name, job.Platform, job.Status)); err != nil {
				logrus.Errorf("failed to send a telegram message: %v", err)
			}
		} else {
			if err = st.SendTelegramMessage(fmt.Sprintf("Launcher job %s status: %s", job.Id, job.Status)); err != nil {
				logrus.Errorf("failed to send a telegram message: %v", err)
			}
		}
	}

	return err
}

// ReportJobLogs Reports job logs to the user
func ReportJobLogs(c *fiber.Ctx, id uuid.UUID, warnings []string, errors []string) (err error) {
	job, err := GetJobForAdmin(c, id)
	if err != nil || job == nil {
		return fmt.Errorf("failed to get a job: %v", err)
	}

	user, err := GetBasicUserInfo(c.Context(), job.OwnerId)
	if err != nil {
		return fmt.Errorf("failed to get job owner: %v", err)
	}

	if job.Type == "Package" {
		// Limit the number of warnings and errors to 10
		if len(warnings) > 10 {
			moreWarningsNum := len(warnings) - 10
			warnings = warnings[:10]
			warnings = append(warnings, "... and "+strconv.Itoa(moreWarningsNum)+" more warnings")
		}

		if len(errors) > 10 {
			moreErrorsNum := len(errors) - 10
			errors = errors[:10]
			errors = append(errors, "... and "+strconv.Itoa(moreErrorsNum)+" more errors")
		}

		// Send the job status email to the requester
		if err = SendPackageJobLogEmail(c, user, job.EntityId, warnings, errors); err != nil {
			return fmt.Errorf("failed to send a job status email: %v", err)
		}

		// Notify telegram channel about the warnings and errors
		//if len(warnings) > 0 {
		//	for _, warning := range warnings {
		//		if err = SendTelegramMessage(fmt.Sprintf("Package %s %s-%s-%s-%s (%s) job warning: ```\n%s\n```", job.Package.Name, job.Package.Version, job.Configuration, job.Deployment, job.Platform, job.Package.Id, warning)); err != nil {
		//			logrus.Errorf("failed to send a telegram message: %v", err)
		//		}
		//	}
		//}
		//
		//if len(errors) > 0 {
		//	for _, e := range errors {
		//		if err = SendTelegramMessage(fmt.Sprintf("Package %s %s-%s-%s-%s (%s) job error: ```\n%s\n```", job.Package.Name, job.Package.Version, job.Configuration, job.Deployment, job.Platform, job.Package.Id, e)); err != nil {
		//			logrus.Errorf("failed to send a telegram message: %v", err)
		//		}
		//	}
		//}
	}

	// Notify the discord channel using the hook
	if job.Type == "Package" {
		if job.Package != nil {
			if err = st.SendTelegramMessage(fmt.Sprintf("Package %s %s-%s-%s-%s (%s) job status: %s", job.Package.Name, job.Package.Version, job.Configuration, job.Deployment, job.Platform, job.Package.Id, job.Status)); err != nil {
				logrus.Errorf("failed to send a telegram message: %v", err)
			}
		} else {
			if err = st.SendTelegramMessage(fmt.Sprintf("Package job %s status: %s", job.Id, job.Status)); err != nil {
				logrus.Errorf("failed to send a telegram message: %v", err)
			}
		}
	} else if job.Type == "Release" {
		if job.Release != nil {
			if err = st.SendTelegramMessage(fmt.Sprintf("App %s release %s-%s-%s-%s (%s) job status: %s", job.Release.AppName, job.Release.Version, job.Configuration, job.Deployment, job.Platform, job.Release.Id, job.Status)); err != nil {
				logrus.Errorf("failed to send a telegram message: %v", err)
			}
		} else {
			if err = st.SendTelegramMessage(fmt.Sprintf("App release job %s status: %s", job.Id, job.Status)); err != nil {
				logrus.Errorf("failed to send a telegram message: %v", err)
			}
		}
	} else if job.Type == "Launcher" {
		if job.App != nil {
			if err = st.SendTelegramMessage(fmt.Sprintf("Launcher %s %s job status: %s", job.App.Name, job.Platform, job.Status)); err != nil {
				logrus.Errorf("failed to send a telegram message: %v", err)
			}
		} else {
			if err = st.SendTelegramMessage(fmt.Sprintf("Launcher job %s status: %s", job.Id, job.Status)); err != nil {
				logrus.Errorf("failed to send a telegram message: %v", err)
			}
		}
	}

	return err
}

func CreatePackageJobs(c *fiber.Ctx, requester *sm.User, entityId uuid.UUID) (err error) {
	configuration := "Development"
	env := os.Getenv("ENVIRONMENT")
	if env == "test" {
		configuration = "Shipping"
	} else if env == "prod" {
		configuration = "Shipping"
	}

	//region Desktop Client Packages
	// Windows client
	if err = CreateJob(c, *requester, CreateJobRequestMetadata{
		Platform:      "Win64",
		Type:          "Package",
		Deployment:    "Client",
		Configuration: configuration,
		EntityId:      entityId,
	}); err != nil {
		logrus.Errorf("failed to create a job: %v", err)
	}

	// Mac client
	if err = CreateJob(c, *requester, CreateJobRequestMetadata{
		Platform:      "Mac",
		Type:          "Package",
		Deployment:    "Client",
		Configuration: configuration,
		EntityId:      entityId,
	}); err != nil {
		logrus.Errorf("failed to create a job: %v", err)
	}

	// Linux client
	//if err = CreateJob(c, *requester, CreateJobRequestMetadata{
	//	Platform:      "Mac",
	//	Type:          "Package",
	//	Deployment:    "Client",
	//	Configuration: configuration,
	//	EntityId:      entityId,
	//}); err != nil {
	//	logrus.Errorf("failed to create a job: %v", err)
	//}
	//endregion

	//region Mobile Client Packages
	// IOS client
	//if err = CreateJob(c, *requester, CreateJobRequestMetadata{
	//	Platform:      "IOS",
	//	Type:          "Package",
	//	Deployment:    "Client",
	//	Configuration: configuration,
	//	EntityId:      entityId,
	//}); err != nil {
	//	logrus.Errorf("failed to create a job: %v", err)
	//}

	// Android client
	//if err = CreateJob(c, *requester, CreateJobRequestMetadata{
	//	Platform:      "Android",
	//	Type:          "Package",
	//	Deployment:    "Client",
	//	Configuration: configuration,
	//	EntityId:      entityId,
	//}); err != nil {
	//	logrus.Errorf("failed to create a job: %v", err)
	//}
	//endregion

	//region Server Packages
	// Linux server
	if err = CreateJob(c, *requester, CreateJobRequestMetadata{
		Platform:      "Linux",
		Type:          "Package",
		Deployment:    "Server",
		Configuration: configuration,
		EntityId:      entityId,
	}); err != nil {
		logrus.Errorf("failed to create a job: %v", err)
	}

	// Windows server
	//if err = CreateJob(c, *requester, CreateJobRequestMetadata{
	//	Platform:      "Win64",
	//	Type:          "Package",
	//	Deployment:    "Server",
	//	Configuration: configuration,
	//	EntityId:      entityId,
	//}); err != nil {
	//	logrus.Errorf("failed to create a job: %v", err)
	//}
	//endregion

	return err
}

func PublishCodeReleaseForAllApps(c *fiber.Ctx, requester *sm.User, codeVersion string, contentVersion string) (err error) {
	db := database.DB
	ctx := c.UserContext()

	configuration := "Development"
	env := os.Getenv("ENVIRONMENT")
	if env == "test" {
		configuration = "Shipping"
	} else if env == "prod" {
		configuration = "Shipping"
	}

	var apps []App
	apps, _, err = IndexAppsForAdmin(c.Context(), 0, math.MaxInt)
	if err != nil {
		logrus.Errorf("failed to index apps: %v", err)
		return err
	}

	for _, app := range apps {
		if app.Id == nil {
			logrus.Errorf("no app id")
			continue
		}

		//region Create a release for an app.

		var newVer semver.Version

		release, err := getLatestReleaseInternal(c.Context(), *app.Id)
		if err != nil {
			logrus.Errorf("invalid release: %v", err)
			continue
		}
		if release != nil {
			ver, err := semver.NewVersion(release.Version)
			if err != nil {
				logrus.Errorf("invalid version: %v", err)
				continue
			}
			newVer = ver.IncMinor()
			if release.CodeVersion == codeVersion {
				logrus.Errorf("app %s %s already have release with this code version: %s", app.Id.String(), app.Name, codeVersion)
				continue
			}
		} else {
			newVer = semver.Version{}
			release = new(Release)
			release.AppId = app.Id
			release.Version = newVer.String()
			release.CodeVersion = codeVersion
			release.ContentVersion = contentVersion
		}

		var (
			q  string
			tx pgx.Tx
		)

		tx, err = db.Begin(ctx)
		if err != nil {
			logrus.Errorf("failed to begin tx: %v", err)
			continue
		}

		id, err1 := uuid.NewV4()
		if err1 != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "failed to generate uuid", "data": nil})
		}

		q = `INSERT INTO entities (id, created_at, updated_at, entity_type, public, views) VALUES ($1, now(), null, 'release', true, 0)`
		_, err = tx.Exec(ctx, q, id)
		if err != nil {
			err1 := tx.Rollback(ctx)
			logrus.Errorf("failed to rollback: %v, %v", err, err1)
			continue
		}

		q = `INSERT INTO releases (id, app_id, version, name, description, published, code_version, content_version) VALUES ($1, $2, $3, $4, $5, false, $6, $7)`
		_, err = tx.Exec(ctx, q, id, *app.Id, newVer.String(), release.Name, release.Description, codeVersion, contentVersion)
		if err != nil {
			err1 := tx.Rollback(ctx)
			logrus.Errorf("failed to rollback: %v, %v", err, err1)
			continue
		}
		err = tx.Commit(ctx)
		if err != nil {
			logrus.Errorf("failed to commit tx: %v", err)
			continue
		}

		//endregion

		//region Create jobs for the app release.

		//region Desktop Client Release

		// Windows client
		if err = CreateJob(c, *requester, CreateJobRequestMetadata{
			Platform:      "Win64",
			Type:          "Release",
			Deployment:    "Client",
			Configuration: configuration,
			EntityId:      id,
		}); err != nil {
			logrus.Errorf("failed to create a job: %v", err)
		}

		// Mac client
		if err = CreateJob(c, *requester, CreateJobRequestMetadata{
			Platform:      "Mac",
			Type:          "Release",
			Deployment:    "Client",
			Configuration: configuration,
			EntityId:      id,
		}); err != nil {
			logrus.Errorf("failed to create a job: %v", err)
		}

		// Linux client
		//if err = CreateJob(c, *requester, CreateJobRequestMetadata{
		//	Platform:      "Mac",
		//	Type:          "Release",
		//	Deployment:    "Client",
		//	Configuration: configuration,
		//	EntityId:      id,
		//}); err != nil {
		//	logrus.Errorf("failed to create a job: %v", err)
		//}

		//endregion

		//region Mobile Client Release'

		// IOS client
		//if err = CreateJob(c, *requester, CreateJobRequestMetadata{
		//	Platform:      "IOS",
		//	Type:          "Release",
		//	Deployment:    "Client",
		//	Configuration: configuration,
		//	EntityId:      id,
		//}); err != nil {
		//	logrus.Errorf("failed to create a job: %v", err)
		//}

		// Android client
		//if err = CreateJob(c, *requester, CreateJobRequestMetadata{
		//	Platform:      "Android",
		//	Type:          "Release",
		//	Deployment:    "Client",
		//	Configuration: configuration,
		//	EntityId:      id,
		//}); err != nil {
		//	logrus.Errorf("failed to create a job: %v", err)
		//}

		//endregion

		//region Server Release

		// Linux server
		if err = CreateJob(c, *requester, CreateJobRequestMetadata{
			Platform:      "Linux",
			Type:          "Release",
			Deployment:    "Server",
			Configuration: configuration,
			EntityId:      id,
		}); err != nil {
			logrus.Errorf("failed to create a job: %v", err)
		}

		// Windows server
		//if err = CreateJob(c, *requester, CreateJobRequestMetadata{
		//	Platform:      "Win64",
		//	Type:          "Release",
		//	Deployment:    "Server",
		//	Configuration: configuration,
		//	EntityId:      id,
		//}); err != nil {
		//	logrus.Errorf("failed to create a job: %v", err)
		//}
		//endregion

		//endregion
	}

	return err
}

func CreateJob(c *fiber.Ctx, requester sm.User, metadata CreateJobRequestMetadata) (err error) {
	db := database.DB
	ctx := c.UserContext()

	if metadata.Platform == "" {
		return fmt.Errorf("no job platform")
	}

	if metadata.Configuration == "" {
		return fmt.Errorf("no job configuration")
	}

	if metadata.Deployment == "" {
		return fmt.Errorf("no job deployment")
	}

	if metadata.Type == "" {
		return fmt.Errorf("no job type")
	}

	if metadata.EntityId.IsNil() {
		return fmt.Errorf("no job entity id")
	}

	var (
		q  string
		tx pgx.Tx
	)

	q = `SELECT j.id, j.version 
FROM jobs j 
WHERE j.entity_id = $1
AND j.platform = $2
AND j.deployment = $3
AND j.type = $4
AND j.configuration = $5`

	var (
		jobId      pgtypeuuid.UUID
		jobVersion int64
	)

	row := db.QueryRow(ctx, q, metadata.EntityId /*$1*/, metadata.Platform /*$2*/, metadata.Deployment /*$3*/, metadata.Type /*$4*/, metadata.Configuration /*$5*/)
	err = row.Scan(&jobId, &jobVersion)
	if err != nil && err.Error() == "no rows in result set" {
		tx, err = db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin tx: %v", err)
		}

		q = `INSERT INTO jobs (id, entity_id, created_at, updated_at, owner_id, worker_id, configuration, platform, type, deployment, status, message, version) VALUES (gen_random_uuid(), $1::uuid, now(), null, $2::uuid, null, $3::text, $4::text, $5::text, $6::text, 'unclaimed'::text, ''::text, 0)`
		_, err = tx.Exec(ctx, q, metadata.EntityId, requester.Id, metadata.Configuration, metadata.Platform, metadata.Type, metadata.Deployment)
		if err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("failed to rollback tx: %v", err)
		}

		err = tx.Commit(ctx)
		if err != nil {
			return fmt.Errorf("failed to commit tx: %v", err)
		}

		return nil
	} else if err == nil {
		tx, err = db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin tx: %v", err)
		}

		q = `DELETE FROM jobs j WHERE j.id = $1`
		_, err = tx.Exec(ctx, q, jobId.UUID)
		if err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("failed to rollback: %v", err)
		}

		jobVersion++

		q = `INSERT INTO jobs (id, entity_id, created_at, updated_at, owner_id, worker_id, configuration, platform, type, deployment, status, message, version) VALUES (gen_random_uuid(), $1::uuid, now(), null, $2::uuid, null, $3::text, $4::text, $5::text, $6::text, 'unclaimed'::text, ''::text, $7::bigint)`
		_, err = tx.Exec(ctx, q, metadata.EntityId, requester.Id, metadata.Configuration, metadata.Platform, metadata.Type, metadata.Deployment, jobVersion)
		if err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("failed to rollback tx: %v", err)
		}

		err = tx.Commit(ctx)
		if err != nil {
			return fmt.Errorf("failed to commit tx: %v", err)
		}
	}

	return nil
}
