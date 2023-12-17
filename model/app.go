package model

import (
	"context"
	"dev.hackerman.me/artheon/veverse-shared/model"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/gofrs/uuid"
	"github.com/jackc/pgtype"
	pgtypeuuid "github.com/jackc/pgtype/ext/gofrs-uuid"
	"github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"
	"veverse-api/database"
	"veverse-api/reflect"
)

type Link struct {
	Id   *uuid.UUID `json:"id"`
	Url  string     `json:"url"`
	Name *string    `json:"name"`
}

// Release struct
type Release struct {
	Entity

	AppId          *uuid.UUID `json:"appId,omitempty"`
	AppName        string     `json:"appName,omitempty"`
	AppTitle       string     `json:"appTitle,omitempty"`
	AppDescription *string    `json:"appDescription"`
	AppUrl         *string    `json:"appUrl"`
	AppExternal    *bool      `json:"appExternal"`
	Version        string     `json:"version,omitempty"`
	CodeVersion    string     `json:"codeVersion,omitempty"`
	ContentVersion string     `json:"contentVersion,omitempty"`
	Name           *string    `json:"name,omitempty"`
	Description    *string    `json:"description,omitempty"`
	Archive        *bool      `json:"archive"`
}

type ReleaseUpdateMetadata struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Version     *string `json:"version,omitempty"`
	Publish     *bool   `json:"publish,omitempty"`
}

type Launcher struct {
	Entity
	AppId   *uuid.UUID `json:"appId,omitempty"`
	Version string     `json:"version,omitempty"`
}

type LauncherRequestMetadata struct {
	PackageRequestMetadata
	Configuration string `json:"configuration,omitempty"` // see SupportedConfiguration
}

type ReleaseRequestMetadata struct {
	PackageRequestMetadata
	//BatchRequestMetadata
	Configuration string `json:"configuration,omitempty"` // see SupportedConfiguration
}

type ReleaseBatchRequestMetadata struct {
	ReleaseRequestMetadata
	BatchRequestMetadata
}

type App struct {
	Entity

	Name              string `json:"name,omitempty"`
	Description       string `json:"description,omitempty"`
	Url               string `json:"url,omitempty"`
	PixelStreamingUrl string `json:"pixelStreamingUrl,omitempty"`
	PrivacyPolicyURL  string `json:"privacyPolicyURL,omitempty"`
	External          bool   `json:"external,omitempty"`
	Title             string `json:"title,omitempty"` // Display name
	EnableSdk         bool   `json:"enableSdk,omitempty"`
}

type AppWithRelease struct {
	App
	Release          *Release `json:"release,omitempty"`
	PrivacyPolicyUrl string   `json:"privacyPolicyUrl,omitempty"`
}

var (
	ReleaseSingular = "release"
	ReleasePlural   = "releases"
	AppSingular     = "app"
	appPlural       = "apps"
)

func findRelease(h []Release, id uuid.UUID) int {
	for i, v := range h {
		if *v.Id == id {
			return i
		}
	}
	return -1
}

func getLatestReleaseInternal(ctx context.Context, id uuid.UUID) (entity *Release, err error) {
	db := database.DB

	q := `SELECT
	r.id            releaseId,
	r.version 		releaseVersion,
	r.code_version 	releaseCodeVersion,
	r.content_version 	releaseContentVersion,
	r.name          releaseName,
	r.description   releaseDescription,
	e.public        entityPublic,
	a.name 			appName
FROM releases r
	LEFT JOIN entities e ON r.id = e.id
    LEFT JOIN apps a ON a.id = r.app_id
WHERE r.app_id = $1::uuid
  AND r.version = (SELECT max(r1.version)
                   FROM releases r1
                   WHERE r1.app_id = $1::uuid AND r1.published = true)
  AND r.published = true
ORDER BY e.id`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, id /*$1*/)

	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("defer getLatestReleaseInternal")
	}()

	for rows.Next() {
		var (
			releaseId      pgtypeuuid.UUID
			version        *string
			codeVersion    *string
			contentVersion *string
			name           *string
			description    *string
			public         *bool
			appName        *string
		)

		err = rows.Scan(
			&releaseId,
			&version,
			&codeVersion,
			&contentVersion,
			&name,
			&description,
			&public,
			&appName,
		)
		if err != nil {
			return nil, err
		}

		if releaseId.Status == pgtype.Null {
			continue
		}

		var e Release
		e.Id = &releaseId.UUID
		if public != nil {
			e.Public = public
		}
		e.Name = name
		e.Description = description
		if version != nil {
			e.Version = *version
		}
		if codeVersion != nil {
			e.CodeVersion = *codeVersion
		}
		if contentVersion != nil {
			e.ContentVersion = *contentVersion
		}
		if appName != nil {
			e.AppName = *appName
		}

		entity = &e
	}

	return entity, err
}

// GetLatestReleaseForAdmin Get release
func GetLatestReleaseForAdmin(ctx context.Context, id uuid.UUID, platform string, deployment string, configuration string) (entity *Release, err error) {
	db := database.DB

	q := `SELECT
	r.id            releaseId,
	r.version 		releaseVersion,
	r.code_version 	releaseCodeVersion,
	r.content_version releaseContentVersion,
	r.name          releaseName,
	r.description   releaseDescription,
	r.archive       releaseArchive,
	e.public        entityPublic,
	a.name 			appName,
	f.id            fId,
	f.url           fUrl,
	f.type			fType,
	f.mime			fMime,
	f.original_path fPath,
	f.size			fSize
FROM releases r
	LEFT JOIN entities e ON r.id = e.id
    LEFT JOIN apps a ON a.id = r.app_id
    LEFT JOIN files f ON e.id = f.entity_id AND f.platform = $2::text AND (f.deployment_type = $3::text)
WHERE r.app_id = $1::uuid
  AND r.version = (SELECT max(r1.version)
                   FROM releases r1
                   WHERE r1.app_id = $1::uuid AND r1.published = true)
  AND r.published = true
ORDER BY e.id`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, id /*$1*/, platform /*$2*/, deployment /*$3*/)

	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("defer GetLatestReleaseForAdmin")
	}()

	for rows.Next() {
		var (
			releaseId      pgtypeuuid.UUID
			version        string
			codeVersion    string
			contentVersion string
			name           *string
			description    *string
			archive        *bool
			public         *bool
			appName        string
			fileId         pgtypeuuid.UUID
			fileUrl        *string
			fileType       *string
			fileMime       *string
			filePath       *string
			fileSize       *int64
		)

		err = rows.Scan(
			&releaseId,
			&version,
			&codeVersion,
			&contentVersion,
			&name,
			&description,
			&archive,
			&public,
			&appName,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&filePath,
			&fileSize,
		)
		if err != nil {
			return nil, err
		}

		if releaseId.Status == pgtype.Null {
			continue
		}

		var file *File
		if fileId.Status != pgtype.Null {
			file = new(File)
			file.Id = &fileId.UUID
			file.Mime = fileMime
			file.OriginalPath = filePath
			file.Size = fileSize

			if fileUrl != nil {
				file.Url = *fileUrl
			}

			if fileType != nil {
				file.Type = *fileType
			}
		}

		if entity != nil {
			if file != nil && !containsFile(entity.Files, *file.Id) {
				entity.Files = append(entity.Files, *file)
			}
		} else {
			var e Release
			e.Id = &releaseId.UUID
			if public != nil {
				e.Public = public
			}
			e.Name = name
			e.Description = description
			e.Archive = archive
			e.Version = version
			e.CodeVersion = codeVersion
			e.ContentVersion = contentVersion
			e.AppName = appName
			if file != nil {
				e.Files = append(e.Files, *file)
			}

			entity = &e
		}
	}

	return entity, err
}

// GetLatestReleaseForRequester Get world
func GetLatestReleaseForRequester(ctx context.Context, requester *model.User, id uuid.UUID, platform string, deployment string, configuration string) (entity *Release, err error) {
	db := database.DB

	q := `SELECT
	r.id            releaseId,
	r.version 		releaseVersion,
	r.code_version 	releaseCodeVersion,
	r.content_version 	releaseContentVersion,
	r.name          releaseName,
	r.description   releaseDescription,
	r.archive       releaseArchive,
	e.public        entityPublic,
	a.name 			appName,
	f.id            fId,
	f.url           fUrl,
	f.type			fType,
	f.mime			fMime,
	f.original_path fPath,
	f.size 			fSize
FROM releases r
	LEFT JOIN entities e ON r.id = e.id
    LEFT JOIN apps a ON a.id = r.app_id
    LEFT JOIN files f ON r.id = f.entity_id AND f.platform = $3::text AND (f.deployment_type = $4::text)
	LEFT JOIN accessibles ac ON e.id = ac.entity_id AND ac.user_id = $1::uuid
WHERE r.app_id = $2::uuid
  AND r.version = (SELECT max(version)
                   FROM releases r1
                       	LEFT JOIN entities e1 ON r1.id = e1.id
						LEFT JOIN accessibles ac1 ON e1.id = ac1.entity_id AND ac1.user_id = $1::uuid
                   WHERE r1.app_id = $2::uuid AND r1.published = true
                     AND (e1.public OR (ac1.is_owner OR ac1.can_view))                   
                   )
  AND (e.public OR (ac.is_owner OR ac.can_view))
  AND r.published = true
ORDER BY e.id`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, requester.Id /*$1*/, id /*$2*/, platform /*$3*/, deployment /*$4*/)

	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("defer GetLatestReleaseForRequester")
	}()

	for rows.Next() {
		var (
			releaseId      pgtypeuuid.UUID
			version        *string
			codeVersion    *string
			contentVersion *string
			name           *string
			description    *string
			archive        *bool
			public         *bool
			appName        string
			fileId         pgtypeuuid.UUID
			fileUrl        *string
			fileType       *string
			fileMime       *string
			filePath       *string
			fileSize       *int64
		)

		err = rows.Scan(
			&releaseId,
			&version,
			&codeVersion,
			&contentVersion,
			&name,
			&description,
			&archive,
			&public,
			&appName,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&filePath,
			&fileSize,
		)
		if err != nil {
			return nil, err
		}

		if releaseId.Status == pgtype.Null {
			continue
		}

		var file *File
		if fileId.Status != pgtype.Null {
			file = new(File)
			file.Id = &fileId.UUID
			file.Mime = fileMime
			file.OriginalPath = filePath
			file.Size = fileSize

			if fileUrl != nil {
				file.Url = *fileUrl
			}

			if fileType != nil {
				file.Type = *fileType
			}
		}

		if entity != nil {
			if file != nil && !containsFile(entity.Files, *file.Id) {
				entity.Files = append(entity.Files, *file)
			}
		} else {
			var e Release
			e.Id = &releaseId.UUID
			if public != nil {
				e.Public = public
			}
			e.Name = name
			e.Description = description
			e.Archive = archive
			if version != nil {
				e.Version = *version
			}
			if codeVersion != nil {
				e.CodeVersion = *codeVersion
			}
			if contentVersion != nil {
				e.ContentVersion = *contentVersion
			}
			e.AppName = appName
			if file != nil {
				e.Files = append(e.Files, *file)
			}

			entity = &e
		}
	}

	return entity, err
}

func GetLatestLauncherForAdmin(ctx context.Context, id uuid.UUID, platform string, deployment string, configuration string) (entity *Launcher, err error) {
	db := database.DB

	q := `SELECT
	l.id            launcherId,
	l.version 		launcherVersion,
	f.id            fId,
	f.url           fUrl,
	f.type			fType,
	f.mime			fMime,
	f.original_path fPath,
	f.size 			fSize
FROM launchers l
	LEFT JOIN entities e ON l.id = e.id
	LEFT JOIN apps a ON a.id = l.app_id
	LEFT JOIN files f ON l.id = f.entity_id AND f.platform = $2::text AND f.deployment_type = $3::text
WHERE l.app_id = $1::uuid AND l.version = (SELECT max(version) FROM launchers l1 WHERE l1.app_id = $1::uuid)
ORDER BY e.id`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, id /*$1*/, platform /*$2*/, deployment /*$3*/)
	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("defer GetLatestLauncherForAdmin")
	}()

	for rows.Next() {
		var (
			launcherId      pgtypeuuid.UUID
			launcherVersion string
			fileId          pgtypeuuid.UUID
			fileUrl         *string
			fileType        *string
			fileMime        *string
			filePath        *string
			fileSize        *int64
		)

		err = rows.Scan(
			&launcherId,
			&launcherVersion,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&filePath,
			&fileSize,
		)
		if err != nil {
			return nil, err
		}

		if launcherId.Status == pgtype.Null {
			continue
		}

		var file *File
		if fileId.Status != pgtype.Null {
			file = new(File)
			file.Id = &fileId.UUID
			file.Mime = fileMime
			file.OriginalPath = filePath
			file.Size = fileSize

			if fileUrl != nil {
				file.Url = *fileUrl
			}

			if fileType != nil {
				file.Type = *fileType
			}
		}

		if entity != nil {
			if file != nil && !containsFile(entity.Files, *file.Id) {
				entity.Files = append(entity.Files, *file)
			}
		} else {
			var e Launcher
			e.Id = &launcherId.UUID
			e.Version = launcherVersion
			if file != nil {
				e.Files = append(e.Files, *file)
			}

			entity = &e
		}
	}

	return entity, err
}

func GetLatestLauncherForRequester(ctx context.Context, requester *model.User, id uuid.UUID, platform string, deployment string, configuration string) (entity *Launcher, err error) {
	db := database.DB

	q := `SELECT
	l.id            launcherId,
	l.version 		launcherVersion,
	f.id            fId,
	f.url           fUrl,
	f.type			fType,
	f.mime			fMime,
	f.original_path fPath,
	f.size 			fSize
FROM launchers l
	LEFT JOIN entities e ON l.id = e.id
	LEFT JOIN apps a ON a.id = l.app_id
	LEFT JOIN files f ON l.id = f.entity_id AND f.platform = $3::text AND f.deployment_type = $4::text
	LEFT JOIN accessibles ac ON e.id = ac.entity_id AND ac.user_id = $1::uuid
WHERE l.app_id = $2::uuid
  AND l.version = (SELECT max(version) 
					FROM launchers l1
						LEFT JOIN entities e1 ON l1.id = e1.id
						LEFT JOIN accessibles ac1 ON l1.id = ac1.entity_id AND ac1.user_id = $1::uuid
					WHERE l1.app_id = $2::uuid
					AND (e1.public = true OR (ac1.is_owner OR ac1.can_view))
				  )
AND (e.public OR (ac.is_owner OR ac.can_view))
ORDER BY e.id`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, requester.Id /*$1*/, id /*$2*/, platform /*$3*/, deployment /*$4*/)
	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("defer GetLatestLauncherForRequester")
	}()

	for rows.Next() {
		var (
			launcherId      pgtypeuuid.UUID
			launcherVersion string
			fileId          pgtypeuuid.UUID
			fileUrl         *string
			fileType        *string
			fileMime        *string
			filePath        *string
			fileSize        *int64
		)

		err = rows.Scan(
			&launcherId,
			&launcherVersion,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&filePath,
			&fileSize,
		)
		if err != nil {
			return nil, err
		}

		if launcherId.Status == pgtype.Null {
			continue
		}

		var file *File
		if fileId.Status != pgtype.Null {
			file = new(File)
			file.Id = &fileId.UUID
			file.Mime = fileMime
			file.OriginalPath = filePath
			file.Size = fileSize

			if fileUrl != nil {
				file.Url = *fileUrl
			}

			if fileType != nil {
				file.Type = *fileType
			}
		}

		if entity != nil {
			if file != nil && !containsFile(entity.Files, *file.Id) {
				entity.Files = append(entity.Files, *file)
			}
		} else {
			var e Launcher
			e.Id = &launcherId.UUID
			e.Version = launcherVersion
			if file != nil {
				e.Files = append(e.Files, *file)
			}

			entity = &e
		}
	}

	return entity, err
}

func IndexAppsForAdmin(ctx context.Context, offset int64, limit int64) (apps []App, total int32, err error) {
	db := database.DB

	var row pgx.Row
	q := `SELECT COUNT(id) FROM apps`
	row = db.QueryRow(ctx, q)
	err = row.Scan(&total)

	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", appPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get apps")
	}

	var rows pgx.Rows
	q = `SELECT id, name, description FROM apps LIMIT $1 OFFSET $2`
	rows, err = db.Query(ctx, q, limit, offset)

	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", appPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get apps")
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("defer IndexAppsForAdmin")
	}()

	for rows.Next() {
		var app App

		err = rows.Scan(&app.Id, &app.Name, &app.Description)
		if err != nil {
			logrus.Errorf("failed to query %s @ %s: %v", appPlural, reflect.FunctionName(), err)
			return nil, -1, fmt.Errorf("failed to get apps")
		}

		apps = append(apps, app)
	}

	return apps, total, err
}

func IndexAppsForRequester(ctx context.Context, user *model.User, offset int64, limit int64) (apps []App, total int32, err error) {
	db := database.DB

	var row pgx.Row
	q := `SELECT COUNT(app.id) FROM apps app INNER JOIN accessibles acc ON app.id = acc.entity_id AND acc.user_id = $1`
	row = db.QueryRow(ctx, q, user.Id)
	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", appPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get user apps")
	}

	q = `SELECT app.id, app.name, description
FROM apps app 
    INNER JOIN accessibles acc ON app.id = acc.entity_id AND acc.user_id = $1 
LIMIT $2 OFFSET $3`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, user.Id, limit, offset)

	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", appPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get user apps")
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("defer IndexAppsForRequester")
	}()

	for rows.Next() {
		var app App

		err = rows.Scan(&app.Id, &app.Name, &app.Description)
		if err != nil {
			logrus.Errorf("failed to scan %s @ %s: %v", appPlural, reflect.FunctionName(), err)
			return nil, -1, fmt.Errorf("failed to get user apps")
		}

		apps = append(apps, app)
	}

	return apps, total, err
}

func IndexOwnedAppsForRequester(ctx context.Context, user *model.User, offset int64, limit int64) (apps []App, total int32, err error) {
	db := database.DB

	var row pgx.Row
	q := `SELECT COUNT(app.id) FROM apps app INNER JOIN accessibles acc ON app.id = acc.entity_id AND acc.user_id = $1 AND acc.is_owner = true`
	row = db.QueryRow(ctx, q, user.Id)
	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", appPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get user apps")
	}

	q = `SELECT app.id, app.name, description
FROM apps app 
    INNER JOIN accessibles acc ON app.id = acc.entity_id AND acc.user_id = $1 AND acc.is_owner = true
LIMIT $2 OFFSET $3`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, user.Id, limit, offset)

	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", appPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get user apps")
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("defer IndexOwnedAppsForRequester")
	}()

	for rows.Next() {
		var app App

		err = rows.Scan(&app.Id, &app.Name, &app.Description)
		if err != nil {
			logrus.Errorf("failed to scan %s @ %s: %v", appPlural, reflect.FunctionName(), err)
			return nil, -1, fmt.Errorf("failed to get user apps")
		}

		apps = append(apps, app)
	}

	return apps, total, err
}

func GetAppForAdmin(ctx context.Context, id uuid.UUID) (entity *AppWithRelease, err error) {
	db := database.DB

	q := `SELECT a.id,
       a.name,
       a.description,
	   a.url,
	   a.pixel_streaming_url,
	   a.privacy_policy_url,
	   a.title,
	   a.enable_sdk,
	   l.id,
	   l.url,
	   l.name,
       r.id,
       r.version,
       r.code_version,
	   r.content_version,
       r.name,
       r.description,
       r.archive,
       f.id,
       f.url,
       f.type,
       f.mime,
       f.version,
       f.original_path,
       f.size
FROM apps a
    	LEFT JOIN entities e ON a.id = e.id
    	LEFT JOIN releases r ON a.id = r.app_id AND r.version = (SELECT max(r1.version) FROM releases r1 WHERE r1.app_id = $1::uuid AND r1.published)
        LEFT JOIN files f ON f.entity_id = a.id
		LEFT JOIN links l ON l.entity_id = e.id
	WHERE a.id = $1`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, id)

	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", appPlural, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("defer GetAppForAdmin")
	}()

	for rows.Next() {
		var (
			appId                 pgtypeuuid.UUID
			appName               *string
			appDescription        *string
			appUrl                *string
			appPixelStreamingUrl  *string
			appPrivacyPolicyUrl   *string
			appTitle              *string
			appEnableSdk          *bool
			linkId                pgtypeuuid.UUID
			linkUrl               *string
			linkName              *string
			releaseId             pgtypeuuid.UUID
			releaseVersion        *string
			releaseCodeVersion    *string
			releaseContentVersion *string
			releaseName           *string
			releaseDescription    *string
			releaseArchive        *bool
			fileId                pgtypeuuid.UUID
			fileUrl               *string
			fileType              *string
			fileMime              *string
			fileVersion           *int32
			fileOriginalPath      *string
			fileSize              *int64
		)

		err = rows.Scan(
			&appId,
			&appName,
			&appDescription,
			&appUrl,
			&appPixelStreamingUrl,
			&appPrivacyPolicyUrl,
			&appTitle,
			&appEnableSdk,
			&linkId,
			&linkUrl,
			&linkName,
			&releaseId,
			&releaseVersion,
			&releaseCodeVersion,
			&releaseContentVersion,
			&releaseName,
			&releaseDescription,
			&releaseArchive,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileVersion,
			&fileOriginalPath,
			&fileSize,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan %s @ %s: %v", appPlural, reflect.FunctionName(), err)
		}

		if appId.Status == pgtype.Null {
			continue
		}

		var file *File
		if fileId.Status != pgtype.Null {
			file = new(File)
			file.Id = &fileId.UUID
			if fileUrl != nil {
				file.Url = *fileUrl
			}
			if fileType != nil {
				file.Type = *fileType
			}
			if fileMime != nil {
				file.Mime = fileMime
			}
			if fileOriginalPath != nil {
				file.OriginalPath = fileOriginalPath
			}
			if fileSize != nil {
				file.Size = fileSize
			}
		}

		var link *Link
		if linkId.Status != pgtype.Null {
			link = new(Link)
			link.Id = &linkId.UUID
			if linkUrl != nil {
				link.Url = *linkUrl
			}
			if linkName != nil {
				link.Name = linkName
			}
		}

		var release *Release
		if releaseId.Status != pgtype.Null {
			release = new(Release)
			release.Id = &releaseId.UUID
			if releaseName != nil {
				release.Name = releaseName
			}
			if releaseVersion != nil {
				release.Version = *releaseVersion
			}
			if releaseDescription != nil {
				release.Description = releaseDescription
			}
			if releaseCodeVersion != nil {
				release.CodeVersion = *releaseCodeVersion
			}
			if releaseContentVersion != nil {
				release.ContentVersion = *releaseContentVersion
			}
			release.Archive = releaseArchive
		}

		if entity != nil {
			if file != nil && !containsFile(entity.Files, *file.Id) {
				entity.Files = append(entity.Files, *file)
			}
			if link != nil && !containsLink(entity.Links, *link.Id) {
				entity.Links = append(entity.Links, *link)
			}
		} else {
			var e AppWithRelease
			e.Id = &appId.UUID
			if appName != nil {
				e.Name = *appName
			}
			if appDescription != nil {
				e.Description = *appDescription
			}

			if appUrl != nil {
				e.App.Url = *appUrl
			}

			if appPixelStreamingUrl != nil {
				e.App.PixelStreamingUrl = *appPixelStreamingUrl
			}

			if appPrivacyPolicyUrl != nil {
				e.App.PrivacyPolicyURL = *appPrivacyPolicyUrl
			}

			if appTitle != nil {
				e.App.Title = *appTitle
			}

			if appEnableSdk != nil {
				e.App.EnableSdk = *appEnableSdk
			}

			if release != nil {
				e.Release = release
			}

			if file != nil && !containsFile(e.Files, *file.Id) {
				e.Files = append(e.Files, *file)
			}
			entity = &e
		}
	}

	return entity, err
}

func GetAppSdkLinkForAdmin(ctx context.Context, id uuid.UUID) (url *string, err error) {
	db := database.DB

	q := `SELECT f.url
FROM apps a
    	LEFT JOIN entities e ON a.id = e.id
        LEFT JOIN files f ON f.entity_id = a.id
	WHERE a.id = $1 AND f.type = 'app-sdk'`

	var rows pgx.Row
	rows = db.QueryRow(ctx, q, id)

	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", appPlural, reflect.FunctionName(), err)
	}

	err = rows.Scan(&url)
	if err != nil {
		return nil, fmt.Errorf("failed to scan %s @ %s: %v", appPlural, reflect.FunctionName(), err)
	}

	return url, nil
}

func containsRelease(h []Release, id uuid.UUID) bool {
	for _, v := range h {
		if *v.Id == id {
			return true
		}
	}
	return false
}

func GetAppForRequester(ctx context.Context, requester *model.User, id uuid.UUID) (entity *AppWithRelease, err error) {
	db := database.DB

	q := `SELECT a.id,
	   a.name,
	   a.description,
	   a.url,
	   a.pixel_streaming_url,
	   a.privacy_policy_url,
	   a.title,
	   a.enable_sdk,
	   l.id,
	   l.url,
	   l.name,
	   r.id,
	   r.version,
	   r.code_version,
	   r.content_version,
	   r.name,
	   r.description,
	   r.archive,
	   f.id,
	   f.url,
	   f.type,
	   f.mime,
	   f.original_path,
	   f.size
FROM apps a
	LEFT JOIN entities e ON a.id = e.id
	LEFT JOIN releases r ON a.id = r.app_id AND r.version = (SELECT max(version)
	  	FROM releases r1 
	      LEFT JOIN entities e2 on r1.id = e2.id
	      LEFT JOIN accessibles ac1 ON e2.id = ac1.entity_id AND ac1.user_id=$1::uuid 
	  	WHERE r1.app_id = $2::uuid AND (e2.public OR (ac1.is_owner OR ac1.can_view)))
	LEFT JOIN files f ON f.entity_id = a.id
	LEFT JOIN links l ON l.entity_id = e.id
	LEFT JOIN accessibles ac ON e.id = ac.entity_id AND ac.user_id=$1::uuid
WHERE a.id = $2::uuid
	AND (e.public OR (ac.is_owner OR ac.can_view))
	ORDER BY e.id`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, requester.Id, id)

	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", appPlural, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetAppForRequester")
	}()

	for rows.Next() {
		var (
			appId                 pgtypeuuid.UUID
			appName               *string
			appDescription        *string
			appUrl                *string
			appPixelStreamingUrl  *string
			appPrivacyPolicyUrl   *string
			appTitle              *string
			appEnableSdk          *bool
			linkId                pgtypeuuid.UUID
			linkUrl               *string
			linkName              *string
			releaseId             pgtypeuuid.UUID
			releaseVersion        *string
			releaseCodeVersion    *string
			releaseContentVersion *string
			releaseName           *string
			releaseDescription    *string
			releaseArchive        *bool
			fileId                pgtypeuuid.UUID
			fileUrl               *string
			fileType              *string
			fileMime              *string
			fileOriginalPath      *string
			fileSize              *int64
		)

		err = rows.Scan(
			&appId,
			&appName,
			&appDescription,
			&appUrl,
			&appPixelStreamingUrl,
			&appPrivacyPolicyUrl,
			&appTitle,
			&appEnableSdk,
			&linkId,
			&linkUrl,
			&linkName,
			&releaseId,
			&releaseVersion,
			&releaseCodeVersion,
			&releaseContentVersion,
			&releaseName,
			&releaseDescription,
			&releaseArchive,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileOriginalPath,
			&fileSize,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan %s @ %s: %v", appPlural, reflect.FunctionName(), err)
		}

		if appId.Status == pgtype.Null {
			continue
		}

		var file *File
		if fileId.Status != pgtype.Null {
			file = new(File)
			file.Id = &fileId.UUID
			if fileUrl != nil {
				file.Url = *fileUrl
			}
			if fileType != nil {
				file.Type = *fileType
			}
			if fileMime != nil {
				file.Mime = fileMime
			}
			if fileOriginalPath != nil {
				file.OriginalPath = fileOriginalPath
			}
			if fileSize != nil {
				file.Size = fileSize
			}
		}

		var link *Link
		if linkId.Status != pgtype.Null {
			link = new(Link)
			link.Id = &linkId.UUID
			if linkUrl != nil {
				link.Url = *linkUrl
			}
			if linkName != nil {
				link.Name = linkName
			}
		}

		var release *Release
		if releaseId.Status != pgtype.Null {
			release = new(Release)
			release.Id = &releaseId.UUID
			if releaseName != nil {
				release.Name = releaseName
			}
			if releaseVersion != nil {
				release.Version = *releaseVersion
			}
			if releaseDescription != nil {
				release.Description = releaseDescription
			}
			if releaseCodeVersion != nil {
				release.CodeVersion = *releaseCodeVersion
			}
			if releaseContentVersion != nil {
				release.ContentVersion = *releaseContentVersion
			}
			release.Archive = releaseArchive
		}

		if entity != nil {
			if file != nil && !containsFile(entity.Files, *file.Id) {
				entity.Files = append(entity.Files, *file)
			}
			if link != nil && !containsLink(entity.Links, *link.Id) {
				entity.Links = append(entity.Links, *link)
			}
		} else {
			var e AppWithRelease
			e.Id = &appId.UUID
			if appName != nil {
				e.Name = *appName
			}
			if appDescription != nil {
				e.Description = *appDescription
			}
			if appUrl != nil {
				e.Url = *appUrl
			}
			if appPixelStreamingUrl != nil {
				e.PixelStreamingUrl = *appPixelStreamingUrl
			}
			if appPrivacyPolicyUrl != nil {
				e.PrivacyPolicyUrl = *appPrivacyPolicyUrl
			}
			if appTitle != nil {
				e.Title = *appTitle
			}
			if appEnableSdk != nil {
				e.EnableSdk = *appEnableSdk
			}
			if release != nil {
				e.Release = release
			}
			if file != nil {
				e.Files = append(e.Files, *file)
			}

			entity = &e
		}
	}

	return entity, err
}

func IndexReleasesForAdmin1(ctx context.Context, offset int64, limit int64) (releases []Release, err error) {
	db := database.DB

	q := `SELECT
    r.id,
    r.app_id,
    r.version,
    r.name,
    r.description,
	a.name AS app_name,
	a.description AS app_description 
FROM releases r
	INNER JOIN apps a on a.id = r.app_id
	LIMIT $1 OFFSET $2`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", ReleasePlural, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexReleasesForAdmin1")
	}()

	for rows.Next() {
		var release Release

		err = rows.Scan(
			&release.Id,
			&release.AppId,
			&release.Version,
			&release.Name,
			&release.Description,
			&release.AppName,
			&release.AppDescription,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan %s @ %s: %v", ReleasePlural, reflect.FunctionName(), err)
		}

		releases = append(releases, release)
	}

	return releases, nil
}

// IndexReleasesForAdmin Index packages for admin with pak file
func IndexReleasesForAdmin(ctx context.Context, id uuid.UUID, offset int64, limit int64) (entities []Release, total int64, err error) {
	db := database.DB

	q := `SELECT COUNT(*) FROM releases r`

	row := db.QueryRow(ctx, q)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}

	q = `SELECT 
	r.id                    releaseId,
	r.version               releaseVersion,
	r.code_version          releaseCodeVersion,
	r.name           		releaseName,
	r.description           releaseDescription,
	r.archive               releaseArchive,
	r.app_id                releaseAppId,
	a.name 					appName,
	a.title 				appTitle,
	e.public                entityPublic,
	f.id                  	fileId,
	f.url                 	fileUrl,
	f.type                	fileType,
	f.mime            		fileMime,
	f.deployment_type		fileDeployment,
	f.platform				filePlatform,
	f.size					fileSize,
	f.original_path			filePath,
	f.version				fileVersion
FROM releases r
    LEFT JOIN entities e ON r.id = e.id
    LEFT JOIN apps a ON r.app_id = a.id
	LEFT JOIN files f ON f.entity_id = r.id AND f.type LIKE '%release%archive%'
	WHERE a.id = $1
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows      pgx.Rows
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, id)
	if err != nil {
		return nil, -1, err
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexReleasesForAdmin")
	}()

	for rows.Next() {
		var (
			id             pgtypeuuid.UUID
			version        *string
			codeVersion    *string
			name           *string
			description    *string
			archive        *bool
			appId          pgtypeuuid.UUID
			appName        *string
			appTitle       *string
			public         *bool
			fileId         pgtypeuuid.UUID
			fileUrl        *string
			fileType       *string
			fileMime       *string
			fileDeployment *string
			filePlatform   *string
			fileSize       *int64
			filePath       *string
			fileVersion    *int
		)

		err = rows.Scan(
			&id,
			&version,
			&codeVersion,
			&name,
			&description,
			&archive,
			&appId,
			&appName,
			&appTitle,
			&public,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileDeployment,
			&filePlatform,
			&fileSize,
			&filePath,
			&fileVersion,
		)
		if err != nil {
			return nil, -1, err
		}

		ri++

		if id.Status == pgtype.Null {
			continue
		}

		//region File
		var file *File
		if fileId.Status != pgtype.Null {
			file = new(File)
			file.Id = &fileId.UUID
			file.OriginalPath = filePath
			file.Size = fileSize
			if fileUrl != nil {
				file.Url = *fileUrl
			}
			if fileType != nil {
				file.Type = *fileType
			}
			if fileMime != nil {
				file.Mime = fileMime
			}
			if fileDeployment != nil {
				file.Deployment = *fileDeployment
			}
			if filePlatform != nil {
				file.Platform = *filePlatform
			}
			if fileVersion != nil {
				file.Version = *fileVersion
			}
		}
		//endregion

		if i := findRelease(entities, id.UUID); i >= 0 {
			if file != nil && !containsFile(entities[i].Files, *file.Id) {
				entities[i].Files = append(entities[i].Files, *file)
			}
		} else {
			if skipped {
				if id.UUID == skippedId {
					continue
				}
			}

			if ei < offset {
				ei++
				skipped = true
				skippedId = id.UUID
				continue
			}

			if ei-offset >= limit {
				break
			}

			var e Release
			e.Id = &id.UUID
			e.Name = name
			e.Description = description
			e.Archive = archive
			if appId.Status == pgtype.Present {
				e.AppId = &appId.UUID
			}
			if appName != nil {
				e.AppName = *appName
			}
			if appTitle != nil {
				e.AppTitle = *appTitle
			}
			if public != nil {
				e.Public = public
			}
			if version != nil {
				e.Version = *version
			}
			if file != nil {
				e.Files = append(e.Files, *file)
			}
			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

func IndexReleasesForRequester(ctx context.Context, offset int64, limit int64) (releases []Release, total int64, err error) {
	db := database.DB

	q := `SELECT
    r.id,
    r.app_id,
    r.version,
    r.code_version,
    r.content_version,
    r.name,
    r.description,
	a.name AS app_name,
	a.title AS app_title,
	a.description AS app_description 
FROM releases r
	INNER JOIN apps a on a.id = r.app_id
	LIMIT $1 OFFSET $2`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", ReleasePlural, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexReleasesForRequester")
	}()

	for rows.Next() {
		var release Release

		err = rows.Scan(
			&release.Id,
			&release.AppId,
			&release.Version,
			&release.CodeVersion,
			&release.ContentVersion,
			&release.Name,
			&release.Description,
			&release.AppName,
			&release.AppTitle,
			&release.AppDescription,
		)

		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan %s @ %s: %v", ReleasePlural, reflect.FunctionName(), err)
		}

		releases = append(releases, release)
	}

	return releases, 0, nil
}

func UpdateAppForAdmin(ctx context.Context) (err error) {
	return err
}

func UpdateAppForRequester(ctx context.Context) (err error) {
	return err
}

func AddReleaseForAdmin(ctx context.Context, requester *model.User, id uuid.UUID, metadata ReleaseUpdateMetadata) (err error) {
	db := database.DB

	var (
		q             string
		row           pgx.Row
		count         *int8
		version       *string
		constraintStr string
		c             *semver.Constraints
		v             *semver.Version
	)

	q = `SELECT 
    count(a.id), r.version
FROM apps a
LEFT JOIN releases r ON a.id = r.app_id AND r.version = (SELECT MAX(r1.version) FROM releases r1 WHERE r1.app_id = $1) 
WHERE a.id = $1
GROUP BY r.version`

	row = db.QueryRow(ctx, q, id)
	err = row.Scan(&count, &version)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", ReleaseSingular, reflect.FunctionName(), err)
		return fmt.Errorf("failed to add new release")
	}

	if version == nil {
		constraintStr = fmt.Sprintf("> %s", "0.0.0")
	} else {
		constraintStr = fmt.Sprintf("> %s", *version)
	}

	c, err = semver.NewConstraint(constraintStr)
	if err != nil {
		logrus.Errorf("failed to parse release version %s @ %s: %v", ReleaseSingular, reflect.FunctionName(), err)
		return fmt.Errorf("failed to check release version")
	}

	v, err = semver.NewVersion(*metadata.Version)
	if err != nil {
		logrus.Errorf("failed to parse release version %s @ %s: %v", ReleaseSingular, reflect.FunctionName(), err)
		return fmt.Errorf("failed to check release version")
	}

	constraintIsValid, _ := c.Validate(v)
	if constraintIsValid {
		var (
			entityId uuid.UUID
			tx       pgx.Tx
		)

		entityId, err = uuid.NewV4()
		if err != nil {
			logrus.Errorf("failed to generate uuid %s @ %s: %v", ReleaseSingular, reflect.FunctionName(), err)
			return fmt.Errorf("failed to set %s", ReleaseSingular)
		}

		tx, err = db.Begin(ctx)
		if err != nil {
			logrus.Errorf("failed to begin tx: %s @ %s: %v", ReleaseSingular, reflect.FunctionName(), err)
			return fmt.Errorf("failed to add new release")
		}

		q = `INSERT INTO entities (id, created_at, updated_at, entity_type, public, views) VALUES ($1, now(), null, 'release', true, 0)`
		_, err = tx.Exec(ctx, q, entityId)
		if err != nil {
			err1 := tx.Rollback(ctx)
			logrus.Errorf("failed to rollback: %s @ %s: %v, %v", ReleaseSingular, reflect.FunctionName(), err, err1)
			return fmt.Errorf("failed to add new release")
		}

		//region Accessible
		q = `INSERT INTO accessibles (user_id, entity_id, is_owner, can_view, can_edit, can_delete) VALUES ($1, $2, true, true, true, true)`
		if _, err = tx.Exec(ctx, q, requester.Id /*1*/, entityId /*2*/); err != nil {
			if err2 := tx.Rollback(ctx); err2 != nil {
				logrus.Errorf("failed to rollback failed tx: %s @ %s: %v, %v", ReleaseSingular, reflect.FunctionName(), err, err2)
				return fmt.Errorf("failed to add new release")
			}

			logrus.Errorf("failed to exec tx: %s @ %s: %v", ReleaseSingular, reflect.FunctionName(), err)
			return fmt.Errorf("failed to add new release")
		}
		//endregion

		q = `INSERT INTO releases (id, app_id, version, name, description, published, archive) VALUES ($1, $2, $3, $4, $5, false, true)`
		_, err = tx.Exec(ctx, q, entityId, id, metadata.Version, metadata.Name, metadata.Description)
		if err != nil {
			err1 := tx.Rollback(ctx)
			logrus.Errorf("failed to exec tx: %s @ %s: %v, %v", ReleaseSingular, reflect.FunctionName(), err, err1)
			return fmt.Errorf("failed to add new release")
		}

		err = tx.Commit(ctx)
		if err != nil {
			logrus.Errorf("failed to commit tx: %v", err)
			return fmt.Errorf("failed to add new release")
		}
	} else {
		logrus.Errorf("release version is lower than latest: %s", *version)
		return fmt.Errorf("release version is lower than latest version: %s", *version)
	}

	return nil
}

func AddReleaseForRequester(ctx context.Context, requester *model.User, id uuid.UUID, metadata ReleaseUpdateMetadata) (err error) {
	db := database.DB

	var (
		q             string
		row           pgx.Row
		count         *int8
		version       *string
		constraintStr string
		c             *semver.Constraints
		v             *semver.Version
	)

	q = `SELECT 
    count(a.id), r.version
FROM apps a
LEFT JOIN entities e ON a.id = e.id
LEFT JOIN releases r ON a.id = r.app_id AND r.version = (SELECT max(version)
	  	FROM releases r1 
	      LEFT JOIN entities e2 on r1.id = e2.id
	      LEFT JOIN accessibles ac1 ON e2.id = ac1.entity_id AND ac1.user_id=$1::uuid 
	  	WHERE r1.app_id = $2::uuid AND (e2.public OR (ac1.is_owner OR ac1.can_view)))
LEFT JOIN accessibles ac ON e.id = ac.entity_id AND ac.user_id=$1::uuid
WHERE a.id = $2::uuid
	AND (e.public OR (ac.is_owner OR ac.can_view))
GROUP BY r.version`

	row = db.QueryRow(ctx, q, requester.Id, id)
	err = row.Scan(&count, &version)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", ReleaseSingular, reflect.FunctionName(), err)
		return fmt.Errorf("failed to add new release")
	}

	if version == nil {
		constraintStr = fmt.Sprintf("> %s", "0.0.0")
	} else {
		constraintStr = fmt.Sprintf("> %s", *version)
	}

	c, err = semver.NewConstraint(constraintStr)
	if err != nil {
		logrus.Errorf("failed to parse release version %s @ %s: %v", ReleaseSingular, reflect.FunctionName(), err)
		return fmt.Errorf("failed to check release version")
	}

	v, err = semver.NewVersion(*metadata.Version)
	if err != nil {
		logrus.Errorf("failed to parse release version %s @ %s: %v", ReleaseSingular, reflect.FunctionName(), err)
		return fmt.Errorf("failed to check release version")
	}

	constraintIsValid, _ := c.Validate(v)
	if constraintIsValid {
		var (
			entityId uuid.UUID
			tx       pgx.Tx
		)

		entityId, err = uuid.NewV4()
		if err != nil {
			logrus.Errorf("failed to generate uuid %s @ %s: %v", ReleaseSingular, reflect.FunctionName(), err)
			return fmt.Errorf("failed to set %s", ReleaseSingular)
		}

		tx, err = db.Begin(ctx)
		if err != nil {
			logrus.Errorf("failed to begin tx: %s @ %s: %v", ReleaseSingular, reflect.FunctionName(), err)
			return fmt.Errorf("failed to add new release")
		}

		q = `INSERT INTO entities (id, created_at, updated_at, entity_type, public, views) VALUES ($1, now(), null, 'release', true, 0)`
		_, err = tx.Exec(ctx, q, entityId)
		if err != nil {
			err1 := tx.Rollback(ctx)
			logrus.Errorf("failed to rollback: %s @ %s: %v, %v", ReleaseSingular, reflect.FunctionName(), err, err1)
			return fmt.Errorf("failed to add new release")
		}

		//region Accessible
		q = `INSERT INTO accessibles (user_id, entity_id, is_owner, can_view, can_edit, can_delete) VALUES ($1, $2, true, true, true, true)`
		if _, err = tx.Exec(ctx, q, requester.Id /*1*/, entityId /*2*/); err != nil {
			if err2 := tx.Rollback(ctx); err2 != nil {
				logrus.Errorf("failed to rollback failed tx: %s @ %s: %v, %v", ReleaseSingular, reflect.FunctionName(), err, err2)
				return fmt.Errorf("failed to add new release")
			}

			logrus.Errorf("failed to exec tx: %s @ %s: %v", ReleaseSingular, reflect.FunctionName(), err)
			return fmt.Errorf("failed to add new release")
		}
		//endregion

		q = `INSERT INTO releases (id, app_id, version, name, description, published, archive) VALUES ($1, $2, $3, $4, $5, false, true)`
		_, err = tx.Exec(ctx, q, entityId, id, metadata.Version, metadata.Name, metadata.Description)
		if err != nil {
			err1 := tx.Rollback(ctx)
			logrus.Errorf("failed to exec tx: %s @ %s: %v, %v", ReleaseSingular, reflect.FunctionName(), err, err1)
			return fmt.Errorf("failed to add new release")
		}

		err = tx.Commit(ctx)
		if err != nil {
			logrus.Errorf("failed to commit tx: %v", err)
			return fmt.Errorf("failed to add new release")
		}
	} else {
		logrus.Errorf("release version is lower than latest: %s", *version)
		return fmt.Errorf("release version is lower than latest version: %s", *version)
	}

	return nil
}

func UpdateReleaseForAdmin(ctx context.Context, id uuid.UUID, metadata ReleaseUpdateMetadata) (err error) {
	db := database.DB

	var (
		q   string
		row pgx.Row
	)

	q = `UPDATE releases SET`

	isUpdate := false
	if metadata.Name != nil {
		isUpdate = true
		q += fmt.Sprintf(" name = %s", *metadata.Name)
	}

	if metadata.Description != nil {
		if isUpdate {
			q += ","
		} else {
			isUpdate = true
		}

		q += fmt.Sprintf(" description = %s", *metadata.Description)
	}

	if metadata.Version != nil {
		if isUpdate {
			q += ","
		} else {
			isUpdate = true
		}

		q += fmt.Sprintf(" publish = %v", *metadata.Version)
	}

	if metadata.Publish != nil {
		if isUpdate {
			q += ","
		} else {
			isUpdate = true
		}

		q += fmt.Sprintf(" published = %t", *metadata.Publish)
	}

	if isUpdate {
		q += ` WHERE id = $1::uuid`
		row = db.QueryRow(ctx, q, id)

		if err = row.Scan(); err != nil {
			if err.Error() != "no rows in result set" {
				logrus.Errorf("failed to query %s @ %s: %v", ReleaseSingular, reflect.FunctionName(), err)
				return fmt.Errorf("failed to update release data")
			}
		}
	}

	return nil
}

// UpdateReleaseForRequester TODO: Fix for requester
func UpdateReleaseForRequester(ctx context.Context, requester *model.User, id uuid.UUID, metadata ReleaseUpdateMetadata) (err error) {
	db := database.DB

	var (
		q     string
		row   pgx.Row
		total *int32
	)

	q = `SELECT COUNT(r.id)
FROM releases r
    LEFT JOIN entities e ON r.id = e.id
	LEFT JOIN accessibles a on e.id = a.entity_id
WHERE r.id = $1::uuid AND a.user_id = $2::uuid AND (e.public OR (a.is_owner OR a.can_view))`

	row = db.QueryRow(ctx, q, id, requester.Id)
	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", appPlural, reflect.FunctionName(), err)
		return fmt.Errorf("failed to update release data")
	}

	if *total == 0 {
		logrus.Errorf("failed to scan %s @ %s: %v", appPlural, reflect.FunctionName(), err)
		return fmt.Errorf("failed to update release data")
	}

	q = `UPDATE releases SET`

	isUpdate := false
	if metadata.Name != nil {
		isUpdate = true
		q += fmt.Sprintf(" name = %s", *metadata.Name)
	}

	if metadata.Description != nil {
		if isUpdate {
			q += ","
		} else {
			isUpdate = true
		}

		q += fmt.Sprintf(" description = %s", *metadata.Description)
	}

	if metadata.Version != nil {
		if isUpdate {
			q += ","
		} else {
			isUpdate = true
		}

		q += fmt.Sprintf(" publish = %v", *metadata.Version)
	}

	if metadata.Publish != nil {
		if isUpdate {
			q += ","
		} else {
			isUpdate = true
		}

		q += fmt.Sprintf(" published = %t", *metadata.Publish)
	}

	if isUpdate {
		q += ` WHERE id = $1::uuid`
		row = db.QueryRow(ctx, q, id)

		if err = row.Scan(); err != nil {
			if err.Error() != "no rows in result set" {
				logrus.Errorf("failed to query %s @ %s: %v", ReleaseSingular, reflect.FunctionName(), err)
				return fmt.Errorf("failed to update release data")
			}
		}
	}

	return nil
}
