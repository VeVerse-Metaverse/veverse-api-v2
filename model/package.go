package model

import (
	"context"
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"fmt"
	"github.com/gofrs/uuid"
	uuid2 "github.com/google/uuid"
	"github.com/jackc/pgtype"
	pgtypeuuid "github.com/jackc/pgtype/ext/gofrs-uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lithammer/shortuuid/v4"
	"time"
	"veverse-api/database"
	"veverse-api/reflect"
)

const (
	packageSingular = "package"
	packagePlural   = "packages"
)

// Package entity model
type Package struct {
	Entity

	Name          string     `json:"name,omitempty"`
	Title         string     `json:"title,omitempty"`
	Summary       string     `json:"summary,omitempty"`
	Description   string     `json:"description,omitempty"`
	Map           string     `json:"map,omitempty"`
	Release       string     `json:"release,omitempty"`
	Price         *float64   `json:"price,omitempty"`
	Version       string     `json:"version,omitempty"`
	ReleasedAt    *time.Time `json:"releasedAt,omitempty"`
	Downloads     *int32     `json:"downloads,omitempty"`
	Liked         *int32     `json:"liked,omitempty"`
	TotalLikes    *int32     `json:"totalLikes,omitempty"`
	TotalDislikes *int32     `json:"totalDislikes,omitempty"`
}

// PackageBatchRequestMetadata Batch request metadata for requesting Package entities
type PackageBatchRequestMetadata struct {
	BatchRequestMetadata
	Platform   string `json:"platform,omitempty"`   // SupportedPlatform (OS) of the pak file (Win64, Mac, Linux, IOS, Android)
	Deployment string `json:"deployment,omitempty"` // SupportedDeployment for the pak file (Server or Client)
}

type PackageRequestMetadata struct {
	IdRequestMetadata
	Platform   string `json:"platform,omitempty"`   // SupportedPlatform (OS) of the destination pak file (Win64, Mac, Linux, IOS, Android)
	Deployment string `json:"deployment,omitempty"` // SupportedDeployment for the destination pak file (Server or Client)
}

type PackageCreateMetadata struct {
	Name        string  `json:"name,omitempty"`        // Name that used as package identifier
	Title       *string `json:"title,omitempty"`       // Title visible to users
	Public      *bool   `json:"public,omitempty"`      // Public or private
	Summary     *string `json:"summary,omitempty"`     // Short Summary
	Description *string `json:"description,omitempty"` // Full Description
	Release     string  `json:"releaseName,omitempty"` // Release
	Map         *string `json:"map,omitempty"`         // Map (list of maps included into the package)
	Version     *string `json:"version,omitempty"`     // Version of the package
}

type PackageUpdateMetadata struct {
	Name        *string `json:"name,omitempty"`        // Name that used as package identifier
	Title       *string `json:"title,omitempty"`       // Title visible to users
	Public      *bool   `json:"public,omitempty"`      // Public or private
	Summary     *string `json:"summary,omitempty"`     // Short Summary
	Description *string `json:"description,omitempty"` // Full Description
	Release     *string `json:"releaseName,omitempty"` // Release
	Map         *string `json:"map,omitempty"`         // Map (list of maps included into the package)
	Version     *string `json:"version,omitempty"`     // Version of the package
}

func findPackage(h []Package, id uuid.UUID) int {
	for i, v := range h {
		if *v.Id == id {
			return i
		}
	}
	return -1
}

// IndexPackagesForAdmin Index packages for admin
func IndexPackagesForAdmin(ctx context.Context, requester *sm.User, offset int64, limit int64) (entities []Package, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) FROM mods m`

	row := db.QueryRow(ctx, q)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	if total == 0 {
		return []Package{}, total, nil
	}

	q = `SELECT
	m.id 					packageId,
	m.name 					packageName,
	m.title 				packageTitle,
	m.description 			packageDescription,
	m.price					packagePrice,
	m.version 				packageVersion,
	m.released_at			packageReleasedAt,
	m.downloads 			packageDownloads,
	e.public                entityPublic,
	e.views					entityViews,
	preview.id 				previewId,
	preview.url 			previewUrl,
	preview.type 			previewType,
	preview.mime 			previewMime,
	preview.size 			previewSize,
	preview.platform 		previewPlatform,
	preview.original_path 	previewOriginalPath,
	preview.hash			previewHash,
	preview.created_at		previewCreatedAt,
	u.id 					ownerId,
	u.name 					ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes
FROM mods m
	LEFT JOIN entities e on m.id = e.id
	LEFT JOIN files preview ON e.id = preview.entity_id
	LEFT JOIN accessibles aa on e.id = aa.entity_id
	LEFT JOIN users u ON aa.user_id = u.id AND aa.is_owner
	LEFT JOIN likables l ON l.entity_id = e.id
	LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
GROUP BY m.id,
		e.id,
		u.id,
		e.public,
		e.views,
		preview.id,
		preview.url,
		preview.type,
		preview.mime,
		preview.size,
		preview.platform,
		preview.original_path,
		preview.hash,
		preview.created_at,
		l2.value,
		e.updated_at,
		e.created_at,
		aa.created_at
ORDER BY e.updated_at DESC, e.created_at DESC, aa.created_at, e.id`

	var (
		rows      pgx.Rows
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, requester.Id)
	if err != nil {
		return []Package{}, total, err
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPackagesForAdmin")
	}()
	for rows.Next() {
		var (
			id               pgtypeuuid.UUID
			name             string
			title            string
			description      *string
			price            *float64
			version          string
			releasedAt       *time.Time
			downloads        *int32
			public           *bool
			views            *int32
			fileId           pgtypeuuid.UUID
			fileUrl          *string
			fileType         *string
			fileMime         *string
			fileSize         *int64
			filePlatform     *string
			fileOriginalPath *string
			fileHash         *string
			fileCreatedAt    *time.Time
			ownerId          pgtypeuuid.UUID
			ownerName        *string
			liked            *int32
			totalLikes       *int32
			totalDislikes    *int32
		)

		err = rows.Scan(
			&id,
			&name,
			&title,
			&description,
			&price,
			&version,
			&releasedAt,
			&downloads,
			&public,
			&views,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileSize,
			&filePlatform,
			&fileOriginalPath,
			&fileHash,
			&fileCreatedAt,
			&ownerId,
			&ownerName,
			&liked,
			&totalLikes,
			&totalDislikes,
		)
		if err != nil {
			return nil, -1, err
		}

		if id.Status == pgtype.Null {
			continue
		}

		var file *File = nil
		if fileId.Status != pgtype.Null {
			file = new(File)
			file.Id = &fileId.UUID
			if fileType != nil {
				file.Type = *fileType
			}
			if fileMime != nil {
				file.Mime = fileMime
			}
			if fileSize != nil {
				file.Size = fileSize
			}
			if fileUrl != nil {
				file.Url = *fileUrl
			}
			if filePlatform != nil {
				file.Platform = *filePlatform
			}
			if fileOriginalPath != nil {
				file.OriginalPath = fileOriginalPath
			}
			if fileHash != nil {
				file.Hash = fileHash
			}
			if fileCreatedAt != nil {
				file.CreatedAt = *fileCreatedAt
			}
		}

		var owner *User = nil
		if ownerId.Status != pgtype.Null {
			owner = new(User)
			owner.Id = &ownerId.UUID
			if ownerName != nil {
				owner.Name = ownerName
			}
		}

		if i := findPackage(entities, id.UUID); i >= 0 {
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

			var e Package
			e.Id = &id.UUID
			e.Name = name
			e.Title = title
			if description != nil {
				e.Description = *description
			}
			e.Price = price
			e.Version = version
			e.ReleasedAt = releasedAt
			e.Downloads = downloads
			e.Public = public
			e.Views = views
			e.Owner = owner
			e.Liked = liked
			e.TotalLikes = totalLikes
			e.TotalDislikes = totalDislikes

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

// IndexPackagesForAdminWithPak Index packages for admin with pak file
func IndexPackagesForAdminWithPak(ctx context.Context, requester *sm.User, offset int64, limit int64, platform string, deployment string) (entities []Package, total int64, err error) {
	db := database.DB

	q := `SELECT COUNT(*) FROM mods m`

	row := db.QueryRow(ctx, q)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}

	q = `SELECT 
	m.id                    packageId,
	m.name                  packageName,
	m.title                 packageMap,
	m.description           packageDescription,
	m.price					packagePrice,
	m.version 				packageVersion,
	m.released_at			packageReleasedAt,
	m.downloads 			packageDownloads,
	e.public                entityPublic,
	e.views					entityViews,
	pak.id                  pakId,
	pak.url                 pakUrl,
	pak.type                pakType,
	pak.mime            	pakMime,
	pak.size				pakSize,
	pak.platform 			pakPlatform,
	pak.original_path 		pakOriginalPath,
	pak.hash				pakHash,
	pak.created_at			pakCreatedAt,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size			previewSize,
	preview.platform 		previewPlatform,
	preview.original_path 	previewOriginalPath,
	preview.hash			previewHash,
	preview.created_at		previewCreatedAt,
	u.id 					ownerId,
	u.name 					ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes
FROM mods m
    LEFT JOIN entities e ON m.id = e.id
	LEFT JOIN files pak ON pak.entity_id = m.id AND pak.type = 'pak' AND pak.platform = $1::text AND pak.deployment_type = $2::text
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
	LEFT JOIN accessibles aa on e.id = aa.entity_id
	LEFT JOIN users u ON aa.user_id = u.id AND aa.is_owner
	LEFT JOIN likables l ON l.entity_id = e.id
	LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $3
GROUP BY m.id,
	 	e.id,
		u.id,
		e.public,
		e.views,
		pak.id,
		pak.url,
		pak.type,
		pak.mime,
		pak.size,
		pak.platform,
		pak.original_path,
		pak.hash,
		pak.created_at,
		preview.id,
		preview.url,
		preview.type,
		preview.mime,
		preview.size,
		preview.platform,
		preview.original_path,
		preview.hash,
		preview.created_at,
		l2.value,
		e.updated_at,
		e.created_at,
		aa.created_at
ORDER BY e.updated_at DESC, e.created_at DESC, aa.created_at, e.id`

	var (
		rows      pgx.Rows
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, platform /*$1*/, deployment /*$2*/, requester.Id /*$3*/)
	if err != nil {
		return nil, -1, err
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPackagesForAdminWithPak")
	}()
	for rows.Next() {
		var (
			id               pgtypeuuid.UUID
			name             string
			title            string
			description      *string
			price            *float64
			version          string
			releasedAt       *time.Time
			downloads        *int32
			public           *bool
			views            *int32
			pakId            pgtypeuuid.UUID
			pakUrl           *string
			pakType          *string
			pakMime          *string
			pakSize          *int64
			pakPlatform      *string
			pakOriginalPath  *string
			pakHash          *string
			pakCreatedAt     *time.Time
			fileId           pgtypeuuid.UUID
			fileUrl          *string
			fileType         *string
			fileMime         *string
			fileSize         *int64
			filePlatform     *string
			fileOriginalPath *string
			fileHash         *string
			fileCreatedAt    *time.Time
			ownerId          pgtypeuuid.UUID
			ownerName        *string
			liked            *int32
			totalLikes       *int32
			totalDislikes    *int32
		)

		err = rows.Scan(
			&id,
			&name,
			&title,
			&description,
			&price,
			&version,
			&releasedAt,
			&downloads,
			&public,
			&views,
			&pakId,
			&pakUrl,
			&pakType,
			&pakMime,
			&pakSize,
			&pakPlatform,
			&pakOriginalPath,
			&pakHash,
			&pakCreatedAt,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileSize,
			&filePlatform,
			&fileOriginalPath,
			&fileHash,
			&fileCreatedAt,
			&ownerId,
			&ownerName,
			&liked,
			&totalLikes,
			&totalDislikes,
		)
		if err != nil {
			return nil, -1, err
		}

		ri++

		if id.Status == pgtype.Null {
			continue
		}

		//region Pak
		var pak *File
		if pakId.Status != pgtype.Null {
			pak = new(File)
			pak.Id = &pakId.UUID
			if pakType != nil {
				pak.Type = *pakType
			}
			if pakMime != nil {
				pak.Mime = pakMime
			}
			if pakSize != nil {
				pak.Size = pakSize
			}
			if pakUrl != nil {
				pak.Url = *pakUrl
			}
			if pakPlatform != nil {
				pak.Platform = *pakPlatform
			}
			if pakOriginalPath != nil {
				pak.OriginalPath = pakOriginalPath
			}
			if pakHash != nil {
				pak.Hash = pakHash
			}
			if pakCreatedAt != nil {
				pak.CreatedAt = *pakCreatedAt
			}
		}
		//endregion

		//region File
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
			if fileSize != nil {
				file.Size = fileSize
			}
			if filePlatform != nil {
				file.Platform = *filePlatform
			}
			if fileOriginalPath != nil {
				file.OriginalPath = fileOriginalPath
			}
			if fileHash != nil {
				file.Hash = fileHash
			}
			if fileCreatedAt != nil {
				file.CreatedAt = *fileCreatedAt
			}
		}
		//endregion

		var owner *User = nil
		if ownerId.Status != pgtype.Null {
			owner = new(User)
			owner.Id = &ownerId.UUID
			if ownerName != nil {
				owner.Name = ownerName
			}
		}

		if i := findPackage(entities, id.UUID); i >= 0 {
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

			var e Package
			e.Id = &id.UUID
			e.Name = name
			e.Title = title
			if description != nil {
				e.Description = *description
			}
			e.Price = price
			e.Version = version
			e.ReleasedAt = releasedAt
			e.Downloads = downloads
			e.Public = public
			e.Views = views
			e.Owner = owner
			e.Liked = liked
			e.TotalLikes = totalLikes
			e.TotalDislikes = totalDislikes

			if pak != nil {
				e.Files = append(e.Files, *pak)
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

// IndexPackagesForAdminWithQuery Index packages for admin with query and pak file
func IndexPackagesForAdminWithQuery(ctx context.Context, requester *sm.User, offset int64, limit int64, query string) (entities []Package, total int64, err error) {
	db := database.DB

	q := `SELECT COUNT(*) FROM mods m WHERE m.name ILIKE $1::text OR m.title ILIKE $1::text`

	row := db.QueryRow(ctx, q, query /*$1*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to scan total @ %s: %v", reflect.FunctionName(), err)
	}

	q = `SELECT 
	m.id                    packageId,
	m.name                  packageName,
	m.title                 packageTitle,
	m.description           packageDescription,
	m.price					packagePrice,
	m.version 				packageVersion,
	m.released_at			packageReleasedAt,
	m.downloads 			packageDownloads,
	e.public                entityPublic,
	e.views					entityViews,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size			previewSize,
	preview.platform 		previewPlatform,
	preview.original_path 	previewOriginalPath,
	preview.hash			previewHash,
	preview.created_at		previewCreatedAt,
	u.id 					ownerId,
	u.name 					ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes
FROM mods m
    LEFT JOIN entities e ON m.id = e.id
	LEFT JOIN files preview ON e.id = preview.entity_id
	LEFT JOIN accessibles a on e.id = a.entity_id
	LEFT JOIN users u ON a.user_id = u.id AND a.is_owner
	LEFT JOIN likables l ON l.entity_id = e.id
	LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
WHERE m.name ILIKE $2::text
GROUP BY m.id,
		u.id,
		e.id,
		e.public,
		e.views,
		preview.id,
		preview.url,
		preview.type,
		preview.mime,
		preview.size,
		preview.platform,
		preview.original_path,
		preview.hash,
		preview.created_at,
		l2.value,
		e.updated_at,
		e.created_at,
		a.created_at
ORDER BY e.updated_at DESC, e.created_at DESC, a.created_at, e.id`

	var (
		rows      pgx.Rows
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, requester.Id /*$1*/, query /*$2*/)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to query %s @ %s: %v", packagePlural, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPackagesForAdminWithQuery")
	}()
	for rows.Next() {
		var (
			id               pgtypeuuid.UUID
			name             string
			title            string
			description      string
			price            *float64
			version          string
			releasedAt       *time.Time
			downloads        *int32
			public           *bool
			views            *int32
			fileId           pgtypeuuid.UUID
			fileUrl          *string
			fileType         *string
			fileMime         *string
			fileSize         *int64
			filePlatform     *string
			fileOriginalPath *string
			fileHash         *string
			fileCreatedAt    *time.Time
			ownerId          pgtypeuuid.UUID
			ownerName        *string
			liked            *int32
			totalLikes       *int32
			totalDislikes    *int32
		)

		err = rows.Scan(
			&id,
			&name,
			&title,
			&description,
			&price,
			&version,
			&releasedAt,
			&downloads,
			&public,
			&views,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileSize,
			&filePlatform,
			&fileOriginalPath,
			&fileHash,
			&fileCreatedAt,
			&ownerId,
			&ownerName,
			&liked,
			&totalLikes,
			&totalDislikes,
		)
		if err != nil {
			return nil, -1, fmt.Errorf("failed to index %s @ %s: %v", packagePlural, reflect.FunctionName(), err)
		}

		ri++

		if id.Status == pgtype.Null {
			continue
		}

		var file *File
		if fileId.Status != pgtype.Null {
			file = new(File)
			file.Id = &fileId.UUID
			if fileType != nil {
				file.Type = *fileType
			}
			if fileMime != nil {
				file.Mime = fileMime
			}
			if fileSize != nil {
				file.Size = fileSize
			}
			if fileUrl != nil {
				file.Url = *fileUrl
			}
			if filePlatform != nil {
				file.Platform = *filePlatform
			}
			if fileOriginalPath != nil {
				file.OriginalPath = fileOriginalPath
			}
			if fileHash != nil {
				file.Hash = fileHash
			}
			if fileCreatedAt != nil {
				file.CreatedAt = *fileCreatedAt
			}
		}

		var owner *User = nil
		if ownerId.Status != pgtype.Null {
			owner = new(User)
			owner.Id = &ownerId.UUID
			if ownerName != nil {
				owner.Name = ownerName
			}
		}

		if i := findPackage(entities, id.UUID); i >= 0 {
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

			var e Package
			e.Id = &id.UUID
			e.Name = name
			e.Title = title
			e.Description = description
			e.Price = price
			e.Version = version
			e.ReleasedAt = releasedAt
			e.Downloads = downloads
			e.Public = public
			e.Views = views
			e.Owner = owner
			e.Liked = liked
			e.TotalLikes = totalLikes
			e.TotalDislikes = totalDislikes

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

// IndexPackagesForAdminWithQueryWithPak Index packages for admin with query and pak file
func IndexPackagesForAdminWithQueryWithPak(ctx context.Context, requester *sm.User, offset int64, limit int64, query string, platform string, deployment string) (entities []Package, total int64, err error) {
	db := database.DB

	q := `SELECT COUNT(*) FROM mods m WHERE m.name ILIKE $1::text OR m.title ILIKE $1::text`

	row := db.QueryRow(ctx, q, query)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to scan total @ %s: %v", reflect.FunctionName(), err)
	}

	q = `SELECT 
	m.id                	spaceId,
	m.name              	spaceName,
	m.title             	spaceTitle,
	m.description       	spaceDescription,
	m.price					packagePrice,
	m.version 				packageVersion,
	m.released_at			packageReleasedAt,
	m.downloads 			packageDownloads,
	e.public            	entityPublic,
	e.views					entityViews,
	pak.id              	pakId,
	pak.url             	pakUrl,
	pak.type            	pakType,
	pak.mime            	pakMime,
	pak.size            	pakSize,
	pak.platform 			pakPlatform,
	pak.original_path 		pakOriginalPath,
	pak.hash				pakHash,
	pak.created_at			pakCreatedAt,
	preview.id          	previewId,
	preview.url         	previewUrl,
	preview.type        	previewType,
	preview.mime        	previewMime,
	preview.size			previewSize,
	preview.platform 		previewPlatform,
	preview.original_path 	previewOriginalPath,
	preview.hash			previewHash,
	preview.created_at		previewCreatedAt,
	u.id 					ownerId,
	u.name 					ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes
FROM mods m
    LEFT JOIN entities e ON m.id = e.id
	LEFT JOIN files pak ON pak.entity_id = m.id AND pak.type = 'pak' AND pak.platform = $1::text AND pak.deployment_type = $2::text
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
	LEFT JOIN accessibles a on e.id = a.entity_id
	LEFT JOIN users u ON a.user_id = u.id AND a.is_owner
	LEFT JOIN likables l ON l.entity_id = e.id
	LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $4
WHERE m.name ILIKE $3::text
GROUP BY m.id,
		u.id,
		e.id,
		e.public,
		e.views,
		pak.id,
		pak.url,
		pak.type,
		pak.mime,
		pak.size,
		pak.platform,
		pak.original_path,
		pak.hash,
		pak.created_at,
		preview.id,
		preview.url,
		preview.type,
		preview.mime,
		preview.size,
		preview.platform,
		preview.original_path,
		preview.hash,
		preview.created_at,
		l2.value,
		e.updated_at,
		e.created_at,
		a.created_at
ORDER BY e.updated_at DESC, e.created_at DESC, a.created_at, e.id`

	var (
		rows      pgx.Rows
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)
	rows, err = db.Query(ctx, q, platform /*$1*/, deployment /*$2*/, query /*$3*/, requester.Id /*$4*/)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to query %s @ %s: %v", packagePlural, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPackagesForAdminWithQueryWithPak")
	}()
	for rows.Next() {
		var (
			id               pgtypeuuid.UUID
			name             string
			title            string
			description      string
			price            *float64
			version          string
			releasedAt       *time.Time
			downloads        *int32
			public           *bool
			views            *int32
			pakId            pgtypeuuid.UUID
			pakUrl           *string
			pakType          *string
			pakMime          *string
			pakSize          *int64
			pakPlatform      *string
			pakOriginalPath  *string
			pakHash          *string
			pakCreatedAt     *time.Time
			fileId           pgtypeuuid.UUID
			fileUrl          *string
			fileType         *string
			fileMime         *string
			fileSize         *int64
			filePlatform     *string
			fileOriginalPath *string
			fileHash         *string
			fileCreatedAt    *time.Time
			ownerId          pgtypeuuid.UUID
			ownerName        *string
			liked            *int32
			totalLikes       *int32
			totalDislikes    *int32
		)

		err = rows.Scan(
			&id,
			&name,
			&title,
			&description,
			&price,
			&version,
			&releasedAt,
			&downloads,
			&public,
			&views,
			&pakId,
			&pakUrl,
			&pakType,
			&pakMime,
			&pakSize,
			&pakPlatform,
			&pakOriginalPath,
			&pakHash,
			&pakCreatedAt,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileSize,
			&filePlatform,
			&fileOriginalPath,
			&fileHash,
			&fileCreatedAt,
			&ownerId,
			&ownerName,
			&liked,
			&totalLikes,
			&totalDislikes,
		)
		if err != nil {
			return nil, -1, fmt.Errorf("failed to scan %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
		}

		ri++

		if id.Status == pgtype.Null {
			continue
		}

		var pak *File
		if pakId.Status != pgtype.Null {
			pak = new(File)
			pak.Id = &pakId.UUID
			if pakType != nil {
				pak.Type = *pakType
			}
			if pakMime != nil {
				pak.Mime = pakMime
			}
			if pakSize != nil {
				pak.Size = pakSize
			}
			if pakUrl != nil {
				pak.Url = *pakUrl
			}
			if pakPlatform != nil {
				pak.Platform = *pakPlatform
			}
			if pakOriginalPath != nil {
				pak.OriginalPath = pakOriginalPath
			}
			if pakHash != nil {
				pak.Hash = pakHash
			}
			if pakCreatedAt != nil {
				pak.CreatedAt = *pakCreatedAt
			}
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
			if fileSize != nil {
				file.Size = fileSize
			}
			if filePlatform != nil {
				file.Platform = *filePlatform
			}
			if fileOriginalPath != nil {
				file.OriginalPath = fileOriginalPath
			}
			if fileHash != nil {
				file.Hash = fileHash
			}
			if fileCreatedAt != nil {
				file.CreatedAt = *fileCreatedAt
			}
		}

		var owner *User = nil
		if ownerId.Status != pgtype.Null {
			owner = new(User)
			owner.Id = &ownerId.UUID
			if ownerName != nil {
				owner.Name = ownerName
			}
		}

		if i := findPackage(entities, id.UUID); i >= 0 {
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

			var e Package
			e.Id = &id.UUID
			e.Name = name
			e.Title = title
			e.Description = description
			e.Price = price
			e.Version = version
			e.ReleasedAt = releasedAt
			e.Downloads = downloads
			e.Public = public
			e.Views = views
			e.Owner = owner
			e.Liked = liked
			e.TotalLikes = totalLikes
			e.TotalDislikes = totalDislikes

			if file != nil {
				e.Files = append(e.Files, *file)
			}
			if pak != nil {
				e.Files = append(e.Files, *pak)
			}

			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexPackagesForRequester Index packages for requester
func IndexPackagesForRequester(ctx context.Context, requester *sm.User, offset int64, limit int64) (entities []Package, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*)
FROM spaces s
    LEFT JOIN entities e ON e.id = s.id
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
WHERE e.public OR a.can_view OR a.is_owner`

	row := db.QueryRow(ctx, q, requester.Id)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to scan total @ %s: %v", reflect.FunctionName(), err)
	}
	//endregion

	q = `SELECT
	m.id                    packageId,
	m.name                  packageName,
	m.title                 packageTitle,
	m.description           packageDescription,
	m.price					packagePrice,
	m.version 				packageVersion,
	m.released_at			packageReleasedAt,
	m.downloads 			packageDownloads,
	m.map                   packageMap,
	e.public                entityPublic,
	e.views					entityViews,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size        	previewSize,
	preview.platform 		previewPlatform,
	preview.original_path 	previewOriginalPath,
	preview.hash			previewHash,
	preview.created_at		previewCreatedAt,
	u.id					ownerId,
	u.name					ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes
FROM mods m
    LEFT JOIN entities e ON m.id = e.id
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
	LEFT JOIN accessibles aa on e.id = aa.entity_id
	LEFT JOIN users u ON aa.user_id = u.id AND aa.is_owner
	LEFT JOIN likables l ON l.entity_id = e.id
	LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1::uuid
WHERE e.public OR a.can_view OR a.is_owner
GROUP BY u.id,
		e.id,
		m.id,
		m.name,
		m.title,
		m.description,
		m.price,
		m.version,
		m.released_at,
		m.downloads,
		m.map,
		e.public,
		e.views,
		preview.id,
		preview.url,
		preview.type,
		preview.mime,
		preview.size,
		preview.platform,
		preview.original_path,
		preview.hash,
		preview.created_at,
		u.name,
		l2.value,
		e.updated_at,
		e.created_at,
		aa.created_at
ORDER BY e.updated_at DESC, e.created_at DESC, aa.created_at, e.id`
	var (
		rows      pgx.Rows
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, requester.Id)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to query %s @ %s: %v", packagePlural, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPackagesForRequester")
	}()
	for rows.Next() {
		var (
			id               pgtypeuuid.UUID
			name             string
			title            string
			description      *string
			price            *float64
			version          string
			releasedAt       *time.Time
			downloads        *int32
			mapName          string
			public           *bool
			views            *int32
			fileId           pgtypeuuid.UUID
			fileUrl          *string
			fileType         *string
			fileMime         *string
			fileSize         *int64
			filePlatform     *string
			fileOriginalPath *string
			fileHash         *string
			fileCreatedAt    *time.Time
			ownerId          pgtypeuuid.UUID
			ownerName        *string
			liked            *int32
			totalLikes       *int32
			totalDislikes    *int32
		)
		err = rows.Scan(
			&id,
			&name,
			&title,
			&description,
			&price,
			&version,
			&releasedAt,
			&downloads,
			&mapName,
			&public,
			&views,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileSize,
			&filePlatform,
			&fileOriginalPath,
			&fileHash,
			&fileCreatedAt,
			&ownerId,
			&ownerName,
			&liked,
			&totalLikes,
			&totalDislikes,
		)
		if err != nil {
			return nil, -1, fmt.Errorf("failed to scan %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
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
			if fileUrl != nil {
				file.Url = *fileUrl
			}
			if fileType != nil {
				file.Type = *fileType
			}
			if fileMime != nil {
				file.Mime = fileMime
			}
			if fileSize != nil {
				file.Size = fileSize
			}
			if filePlatform != nil {
				file.Platform = *filePlatform
			}
			if fileOriginalPath != nil {
				file.OriginalPath = fileOriginalPath
			}
			if fileHash != nil {
				file.Hash = fileHash
			}
			if fileCreatedAt != nil {
				file.CreatedAt = *fileCreatedAt
			}
		}
		//endregion

		var owner *User = nil
		if ownerId.Status != pgtype.Null {
			owner = new(User)
			owner.Id = &ownerId.UUID
			if ownerName != nil {
				owner.Name = ownerName
			}
		}

		if i := findPackage(entities, id.UUID); i >= 0 {
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

			var e Package
			e.Id = &id.UUID
			e.Name = name
			e.Title = title
			if description != nil {
				e.Description = *description
			}
			e.Price = price
			e.Version = version
			e.ReleasedAt = releasedAt
			e.Downloads = downloads
			e.Map = mapName
			e.Public = public
			e.Views = views
			e.Owner = owner
			e.Liked = liked
			e.TotalLikes = totalLikes
			e.TotalDislikes = totalDislikes

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

// IndexPackagesForRequesterWithPak Index packages for requester with pak file
func IndexPackagesForRequesterWithPak(ctx context.Context, requester *sm.User, offset int64, limit int64, platform string, deployment string) (entities []Package, total int64, err error) {
	db := database.DB

	q := `SELECT COUNT(*) FROM mods m
    LEFT JOIN entities e ON e.id = m.id
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
WHERE /*e.public OR a.can_view*/ a.can_edit OR a.is_owner`

	row := db.QueryRow(ctx, q, requester.Id)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to scan total @ %s: %v", reflect.FunctionName(), err)
	}

	q = `SELECT 
	m.id                    packageId,
	m.name                  packageName,
	m.title                 packageMap,
	m.description           packageDescription,
	m.price					packagePrice,
	m.version 				packageVersion,
	m.released_at			packageReleasedAt,
	m.downloads 			packageDownloads,
	e.public                entityPublic,
	e.views					entityViews,
	pak.id                  pakId,
	pak.url                 pakUrl,
	pak.type                pakType,
	pak.mime            	pakMime,
	pak.size            	pakSize,
	pak.platform 			pakPlatform,
	pak.original_path 		pakOriginalPath,
	pak.hash				pakHash,
	pak.created_at			pakCreatedAt,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size			previewSize,
	preview.platform 		previewPlatform,
	preview.original_path 	previewOriginalPath,
	preview.hash			previewHash,
	preview.created_at		previewCreatedAt,
	u.id 					ownerId,
	u.name 					ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes
FROM mods m
    LEFT JOIN entities e ON m.id = e.id
	LEFT JOIN files pak ON pak.entity_id = m.id AND pak.type = 'pak' AND pak.platform = $1::text AND pak.deployment_type = $2::text
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $3::uuid
	LEFT JOIN accessibles aa on e.id = aa.entity_id
	LEFT JOIN users u ON aa.user_id = u.id AND aa.is_owner
	LEFT JOIN likables l ON l.entity_id = e.id
	LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $3
WHERE /*e.public OR a.can_view*/ a.can_edit OR a.is_owner
GROUP BY m.id,
	 	e.id,
		u.id,
		e.public,
		e.views,
		pak.id,
		pak.url,
		pak.type,
		pak.mime,
		pak.size,
		pak.platform,
		pak.original_path,
		pak.hash,
		pak.created_at,
		preview.id,
		preview.url,
		preview.type,
		preview.mime,
		preview.size,
		preview.platform,
		preview.original_path,
		preview.hash,
		preview.created_at,
		l2.value,
		e.updated_at,
		e.created_at,
		aa.created_at
ORDER BY e.updated_at DESC, e.created_at DESC, aa.created_at, e.id`

	var (
		rows      pgx.Rows
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, platform /*$1*/, deployment /*$2*/, requester.Id /*$3*/)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to query %s @ %s: %v", packagePlural, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPackagesForRequesterWithPak")
	}()
	for rows.Next() {
		var (
			id               pgtypeuuid.UUID
			name             string
			title            string
			description      string
			price            *float64
			version          string
			releasedAt       *time.Time
			downloads        *int32
			public           *bool
			views            *int32
			pakId            pgtypeuuid.UUID
			pakUrl           *string
			pakType          *string
			pakMime          *string
			pakSize          *int64
			pakPlatform      *string
			pakOriginalPath  *string
			pakHash          *string
			pakCreatedAt     *time.Time
			fileId           pgtypeuuid.UUID
			fileUrl          *string
			fileType         *string
			fileMime         *string
			fileSize         *int64
			filePlatform     *string
			fileOriginalPath *string
			fileHash         *string
			fileCreatedAt    *time.Time
			ownerId          pgtypeuuid.UUID
			ownerName        *string
			liked            *int32
			totalLikes       *int32
			totalDislikes    *int32
		)

		err = rows.Scan(
			&id,
			&name,
			&title,
			&description,
			&price,
			&version,
			&releasedAt,
			&downloads,
			&public,
			&views,
			&pakId,
			&pakUrl,
			&pakType,
			&pakMime,
			&pakSize,
			&pakPlatform,
			&pakOriginalPath,
			&pakHash,
			&pakCreatedAt,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileSize,
			&filePlatform,
			&fileOriginalPath,
			&fileHash,
			&fileCreatedAt,
			&ownerId,
			&ownerName,
			&liked,
			&totalLikes,
			&totalDislikes,
		)
		if err != nil {
			return nil, -1, fmt.Errorf("failed to scan %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
		}

		ri++

		if id.Status == pgtype.Null {
			continue
		}

		//region Pak
		var pak *File
		if pakId.Status != pgtype.Null {
			pak = new(File)
			pak.Id = &pakId.UUID
			if pakType != nil {
				pak.Type = *pakType
			}
			if pakMime != nil {
				pak.Mime = pakMime
			}
			if pakSize != nil {
				pak.Size = pakSize
			}
			if pakUrl != nil {
				pak.Url = *pakUrl
			}
			if pakPlatform != nil {
				pak.Platform = *pakPlatform
			}
			if pakOriginalPath != nil {
				pak.OriginalPath = pakOriginalPath
			}
			if pakHash != nil {
				pak.Hash = pakHash
			}
			if pakCreatedAt != nil {
				pak.CreatedAt = *pakCreatedAt
			}
		}
		//endregion

		//region File
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
			if filePlatform != nil {
				file.Platform = *filePlatform
			}
			if fileSize != nil {
				file.Size = fileSize
			}
			if fileOriginalPath != nil {
				file.OriginalPath = fileOriginalPath
			}
			if fileHash != nil {
				file.Hash = fileHash
			}
			if fileCreatedAt != nil {
				file.CreatedAt = *fileCreatedAt
			}
		}
		//endregion

		var owner *User = nil
		if ownerId.Status != pgtype.Null {
			owner = new(User)
			owner.Id = &ownerId.UUID
			if ownerName != nil {
				owner.Name = ownerName
			}
		}

		if i := findPackage(entities, id.UUID); i >= 0 {
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

			var e Package
			e.Id = &id.UUID
			e.Name = name
			e.Title = title
			e.Description = description
			e.Price = price
			e.Version = version
			e.ReleasedAt = releasedAt
			e.Downloads = downloads
			e.Public = public
			e.Views = views
			e.Owner = owner
			e.Liked = liked
			e.TotalLikes = totalLikes
			e.TotalDislikes = totalDislikes

			if pak != nil {
				e.Files = append(e.Files, *pak)
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

// IndexPackagesForRequesterWithQuery Index packages for requester with query and pak file
func IndexPackagesForRequesterWithQuery(ctx context.Context, requester *sm.User, offset int64, limit int64, query string) (entities []Package, total int64, err error) {
	db := database.DB

	q := `SELECT COUNT(*) FROM mods m
	LEFT JOIN entities e ON e.id = m.id
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
WHERE m.name ILIKE $2::text OR m.title ILIKE $2::text AND (/*e.public OR a.can_view*/ a.can_edit OR a.is_owner)`

	row := db.QueryRow(ctx, q, requester.Id, query)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to scan total @ %s: %v", reflect.FunctionName(), err)
	}

	q = `SELECT 
	m.id                    packageId,
	m.name                  packageName,
	m.title                 packageTitle,
	m.description           packageDescription,
	m.price					packagePrice,
	m.version 				packageVersion,
	m.released_at			packageReleasedAt,
	m.downloads 			packageDownloads,
	e.public                entityPublic,
	e.views					entityViews,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size        	previewSize,
	preview.platform 		previewPlatform,
	preview.original_path 	previewOriginalPath,
	preview.hash			previewHash,
	preview.created_at		previewCreatedAt,
	u.id					ownerId,
	u.name					ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes
FROM mods m
    LEFT JOIN entities e ON m.id = e.id
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
	LEFT JOIN accessibles aa on e.id = aa.entity_id
	LEFT JOIN users u ON aa.user_id = u.id AND aa.is_owner
	LEFT JOIN likables l ON l.entity_id = e.id
	LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
WHERE m.name ILIKE $2::text OR m.title ILIKE $2::text AND (/*e.public OR a.can_view*/ a.can_edit OR a.is_owner)
GROUP BY m.id,
		u.id,
		e.id,
		e.public,
		e.views,
		preview.id,
		preview.url,
		preview.type,
		preview.mime,
		preview.size,
		preview.platform,
		preview.original_path,
		preview.hash,
		preview.created_at,
		l2.value,
		e.updated_at,
		e.created_at,
		a.created_at
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows      pgx.Rows
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, requester.Id /*$1*/, query /*$2*/)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to query %s @ %s: %v", packagePlural, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPackagesForRequesterWithQuery")
	}()
	for rows.Next() {
		var (
			id               pgtypeuuid.UUID
			name             string
			title            string
			description      string
			price            *float64
			version          string
			releasedAt       *time.Time
			downloads        *int32
			public           *bool
			views            *int32
			fileId           pgtypeuuid.UUID
			fileUrl          *string
			fileType         *string
			fileMime         *string
			fileSize         *int64
			filePlatform     *string
			fileOriginalPath *string
			fileHash         *string
			fileCreatedAt    *time.Time
			ownerId          pgtypeuuid.UUID
			ownerName        *string
			liked            *int32
			totalLikes       *int32
			totalDislikes    *int32
		)

		err = rows.Scan(
			&id,
			&name,
			&title,
			&description,
			&price,
			&version,
			&releasedAt,
			&downloads,
			&public,
			&views,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileSize,
			&filePlatform,
			&fileOriginalPath,
			&fileHash,
			&fileCreatedAt,
			&ownerId,
			&ownerName,
			&liked,
			&totalLikes,
			&totalDislikes,
		)

		if err != nil {
			return nil, -1, fmt.Errorf("failed to scan %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
		}

		ri++

		if id.Status == pgtype.Null {
			continue
		}

		var file *File
		if fileId.Status != pgtype.Null {
			file = new(File)
			file.Id = &fileId.UUID
			if fileType != nil {
				file.Type = *fileType
			}
			if fileMime != nil {
				file.Mime = fileMime
			}
			if fileSize != nil {
				file.Size = fileSize
			}
			if fileUrl != nil {
				file.Url = *fileUrl
			}
			if filePlatform != nil {
				file.Platform = *filePlatform
			}
			if fileOriginalPath != nil {
				file.OriginalPath = fileOriginalPath
			}
			if fileHash != nil {
				file.Hash = fileHash
			}
			if fileCreatedAt != nil {
				file.CreatedAt = *fileCreatedAt
			}
		}

		var owner *User = nil
		if ownerId.Status != pgtype.Null {
			owner = new(User)
			owner.Id = &ownerId.UUID
			if ownerName != nil {
				owner.Name = ownerName
			}
		}

		if i := findPackage(entities, id.UUID); i >= 0 {
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

			var e Package
			e.Id = &id.UUID
			e.Name = name
			e.Title = title
			e.Description = description
			e.Price = price
			e.Version = version
			e.ReleasedAt = releasedAt
			e.Downloads = downloads
			e.Public = public
			e.Views = views
			e.Owner = owner
			e.Liked = liked
			e.TotalLikes = totalLikes
			e.TotalDislikes = totalDislikes

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

// IndexPackagesForRequesterWithQueryWithPak Index packages for admin with query and pak file
func IndexPackagesForRequesterWithQueryWithPak(ctx context.Context, requester *sm.User, offset int64, limit int64, query string, platform string, deployment string) (entities []Package, total int64, err error) {
	db := database.DB

	q := `SELECT COUNT(*) FROM mods m 
	LEFT JOIN entities e on m.id = e.id
	LEFT JOIN accessibles a on e.id = a.entity_id AND a.user_id = $1::uuid 
WHERE m.name ILIKE $2::text OR m.title ILIKE $2::text AND (/*e.public OR a.can_view*/ a.can_edit OR a.is_owner)`

	row := db.QueryRow(ctx, q, requester.Id /*$1*/, query /*$2*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to scan total @ %s: %v", reflect.FunctionName(), err)
	}

	q = `SELECT 
	m.id                	spaceId,
	m.name              	spaceName,
	m.title             	spaceTitle,
	m.description       	spaceDescription,
	m.price					packagePrice,
	m.version 				packageVersion,
	m.released_at			packageReleasedAt,
	m.downloads 			packageDownloads,
	e.public            	entityPublic,
	e.views					entityViews,
	pak.id              	pakId,
	pak.url             	pakUrl,
	pak.type            	pakType,
	pak.mime            	pakMime,
	pak.size				pakSize,
	pak.platform 			pakPlatform,
	pak.original_path 		pakOriginalPath,
	pak.hash				pakHash,
	pak.created_at			pakCreatedAt,
	preview.id          	previewId,
	preview.url         	previewUrl,
	preview.type        	previewType,
	preview.mime        	previewMime,
	preview.size			previewSize,
	preview.platform 		previewPlatform,
	preview.original_path 	previewOriginalPath,
	preview.hash			previewHash,
	preview.created_at		previewCreatedAt,
	u.id 					ownerId,
	u.name 					ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes
FROM mods m
    LEFT JOIN entities e ON m.id = e.id
	LEFT JOIN files pak ON pak.entity_id = m.id AND pak.type = 'pak' AND pak.platform = $1::text AND pak.deployment_type = $2::text
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $3 
	LEFT JOIN accessibles aa on e.id = aa.entity_id
	LEFT JOIN users u ON aa.user_id = u.id AND aa.is_owner
	LEFT JOIN likables l ON l.entity_id = e.id
	LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $3
WHERE (/*e.public OR a.can_view*/ a.can_edit OR a.is_owner) AND m.name ILIKE $4::text OR m.title ILIKE $4::text
GROUP BY u.id,
        m.id,
		e.id,
		m.name,
		m.title,
		m.description,
		m.price,
		m.version,
		m.released_at,
		m.downloads,
		e.public,
		e.views,
		pak.id,
		pak.url,
		pak.type,
		pak.mime,
		pak.size,
		pak.platform,
		pak.original_path,
		pak.hash,
		pak.created_at,
		preview.id,
		preview.url,
		preview.type,
		preview.mime,
		preview.size,
		preview.platform,
		preview.original_path,
		preview.hash,
		preview.created_at,
		u.name,
		l2.value,
		e.updated_at,
		e.created_at,
		aa.created_at
ORDER BY e.updated_at DESC, e.created_at DESC, aa.created_at, e.id`

	var (
		rows      pgx.Rows
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)
	rows, err = db.Query(ctx, q, platform /*$1*/, deployment /*$2*/, requester.Id /*$3*/, query /*$4*/)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to query %s @ %s: %v", packagePlural, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPackagesForRequesterWithQueryWithPak")
	}()
	for rows.Next() {
		var (
			id               pgtypeuuid.UUID
			name             string
			title            string
			description      string
			price            *float64
			version          string
			releasedAt       *time.Time
			downloads        *int32
			public           *bool
			views            *int32
			pakId            pgtypeuuid.UUID
			pakUrl           *string
			pakType          *string
			pakMime          *string
			pakSize          *int64
			pakPlatform      *string
			pakOriginalPath  *string
			pakHash          *string
			pakCreatedAt     *time.Time
			fileId           pgtypeuuid.UUID
			fileUrl          *string
			fileType         *string
			fileMime         *string
			fileSize         *int64
			filePlatform     *string
			fileOriginalPath *string
			fileHash         *string
			fileCreatedAt    *time.Time
			ownerId          pgtypeuuid.UUID
			ownerName        *string
			liked            *int32
			totalLikes       *int32
			totalDislikes    *int32
		)

		err = rows.Scan(
			&id,
			&name,
			&title,
			&description,
			&price,
			&version,
			&releasedAt,
			&downloads,
			&public,
			&views,
			&pakId,
			&pakUrl,
			&pakType,
			&pakMime,
			&pakSize,
			&pakPlatform,
			&pakOriginalPath,
			&pakHash,
			&pakCreatedAt,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileSize,
			&filePlatform,
			&fileOriginalPath,
			&fileHash,
			&fileCreatedAt,
			&ownerId,
			&ownerName,
			&liked,
			&totalLikes,
			&totalDislikes,
		)
		if err != nil {
			return nil, -1, fmt.Errorf("failed to scan %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
		}

		ri++

		if id.Status == pgtype.Null {
			continue
		}

		var pak *File
		if pakId.Status != pgtype.Null {
			pak = new(File)
			pak.Id = &pakId.UUID
			if pakType != nil {
				pak.Type = *pakType
			}
			if pakMime != nil {
				pak.Mime = pakMime
			}
			if pakSize != nil {
				pak.Size = pakSize
			}
			if pakUrl != nil {
				pak.Url = *pakUrl
			}
			if pakPlatform != nil {
				pak.Platform = *pakPlatform
			}
			if pakOriginalPath != nil {
				pak.OriginalPath = pakOriginalPath
			}
			if pakHash != nil {
				pak.Hash = pakHash
			}
			if pakCreatedAt != nil {
				pak.CreatedAt = *pakCreatedAt
			}
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
			if fileSize != nil {
				file.Size = fileSize
			}
			if filePlatform != nil {
				file.Platform = *filePlatform
			}
			if fileOriginalPath != nil {
				file.OriginalPath = fileOriginalPath
			}
			if fileHash != nil {
				file.Hash = fileHash
			}
			if fileCreatedAt != nil {
				file.CreatedAt = *fileCreatedAt
			}
		}

		var owner *User = nil
		if ownerId.Status != pgtype.Null {
			owner = new(User)
			owner.Id = &ownerId.UUID
			if ownerName != nil {
				owner.Name = ownerName
			}
		}

		if i := findPackage(entities, id.UUID); i >= 0 {
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

			var e Package
			e.Id = &id.UUID
			e.Name = name
			e.Title = title
			e.Description = description
			e.Price = price
			e.Version = version
			e.ReleasedAt = releasedAt
			e.Downloads = downloads
			e.Public = public
			e.Views = views
			e.Owner = owner
			e.Liked = liked
			e.TotalLikes = totalLikes
			e.TotalDislikes = totalDislikes

			if file != nil {
				e.Files = append(e.Files, *file)
			}
			if pak != nil {
				e.Files = append(e.Files, *pak)
			}

			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// GetPackageForAdminWithPak Get package
func GetPackageForAdminWithPak(ctx context.Context, requester *sm.User, id uuid.UUID, platform string, deployment string) (entity *Package, err error) {
	db := database.DB

	q := `SELECT 
	m.id                    packageId,
	m.name                  packageName,
	m.title                 packageMap,
	m.description           packageDescription,
	m.price					packagePrice,
	m.version 				packageVersion,
	m.released_at			packageReleasedAt,
	m.downloads 			packageDownloads,
	e.public                entityPublic,
	e.views					entityViews,
	pak.id                  pakId,
	pak.url                 pakUrl,
	pak.type                pakType,
	pak.mime            	pakMime,
	pak.size            	pakSize,
	pak.platform 			pakPlatform,
	pak.original_path 		pakOriginalPath,
	pak.hash				pakHash,
	pak.created_at			pakCreatedAt,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size        	previewSize,
	preview.platform 		previewPlatform,
	preview.original_path 	previewOriginalPath,
	preview.hash			previewHash,
	preview.created_at		previewCreatedAt,
	u.id 					ownerId,
	u.name 					ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes
FROM mods m
    LEFT JOIN entities e ON m.id = e.id
	LEFT JOIN files pak ON pak.entity_id = m.id AND pak.type = 'pak' AND pak.platform = $1::text AND pak.deployment_type = $2::text
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
	LEFT JOIN accessibles aa on e.id = aa.entity_id
	LEFT JOIN users u ON aa.user_id = u.id AND aa.is_owner
	LEFT JOIN likables l ON l.entity_id = e.id
	LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $3
WHERE m.id = $4
GROUP BY m.id,
    e.public,
    e.views,
    pak.id,
    pak.url,
    pak.type,
    pak.mime,
    pak.size,
    pak.platform,
    pak.original_path,
    pak.hash,
    pak.created_at,
	preview.id,
    preview.url,
    preview.type,
    preview.mime,
    preview.size,
	preview.platform,
	preview.original_path,
	preview.hash,
	preview.created_at,
	u.id,
	u.name,
    l2.value`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, platform /*$1*/, deployment /*$2*/, requester.Id /*$3*/, id /*$4*/)
	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetPackageForAdminWithPak")
	}()
	for rows.Next() {
		var (
			oId              pgtypeuuid.UUID
			name             string
			title            string
			description      string
			price            *float64
			version          string
			releasedAt       *time.Time
			downloads        *int32
			public           *bool
			views            *int32
			pakId            pgtypeuuid.UUID
			pakUrl           *string
			pakType          *string
			pakMime          *string
			pakSize          *int64
			pakPlatform      *string
			pakOriginalPath  *string
			pakHash          *string
			pakCreatedAt     *time.Time
			fileId           pgtypeuuid.UUID
			fileUrl          *string
			fileType         *string
			fileMime         *string
			fileSize         *int64
			filePlatform     *string
			fileOriginalPath *string
			fileHash         *string
			fileCreatedAt    *time.Time
			ownerId          pgtypeuuid.UUID
			ownerName        *string
			liked            *int32
			totalLikes       *int32
			totalDislikes    *int32
		)
		err = rows.Scan(
			&oId,
			&name,
			&title,
			&description,
			&price,
			&version,
			&releasedAt,
			&downloads,
			&public,
			&views,
			&pakId,
			&pakUrl,
			&pakType,
			&pakMime,
			&pakSize,
			&pakPlatform,
			&pakOriginalPath,
			&pakHash,
			&pakCreatedAt,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileSize,
			&filePlatform,
			&fileOriginalPath,
			&fileHash,
			&fileCreatedAt,
			&ownerId,
			&ownerName,
			&liked,
			&totalLikes,
			&totalDislikes,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
		}

		if oId.Status == pgtype.Null {
			continue
		}

		var pak *File
		if pakId.Status != pgtype.Null {
			pak = new(File)
			pak.Id = &pakId.UUID
			if pakUrl != nil {
				pak.Url = *pakUrl
			}
			if pakType != nil {
				pak.Type = *pakType
			}
			if pakSize != nil {
				pak.Size = pakSize
			}
			if pakMime != nil {
				pak.Size = pakSize
			}
			if pakPlatform != nil {
				pak.Platform = *pakPlatform
			}
			if pakOriginalPath != nil {
				pak.OriginalPath = pakOriginalPath
			}
			if pakHash != nil {
				pak.Hash = pakHash
			}
			if pakCreatedAt != nil {
				pak.CreatedAt = *pakCreatedAt
			}
		}

		var preview *File
		if fileId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &fileId.UUID
			if fileUrl != nil {
				preview.Url = *fileUrl
			}
			if fileType != nil {
				preview.Type = *fileType
			}
			if fileMime != nil {
				preview.Mime = fileMime
			}
			if fileSize != nil {
				preview.Size = fileSize
			}
			if filePlatform != nil {
				preview.Platform = *filePlatform
			}
			if fileOriginalPath != nil {
				preview.OriginalPath = fileOriginalPath
			}
			if fileHash != nil {
				preview.Hash = fileHash
			}
			if fileCreatedAt != nil {
				preview.CreatedAt = *fileCreatedAt
			}
		}

		var owner *User = nil
		if ownerId.Status != pgtype.Null {
			owner = new(User)
			owner.Id = &ownerId.UUID
			if ownerName != nil {
				owner.Name = ownerName
			}
		}

		if entity != nil {
			if preview != nil && !containsFile(entity.Files, *preview.Id) {
				entity.Files = append(entity.Files, *preview)
			}

			if pak != nil && !containsFile(entity.Files, *pak.Id) {
				entity.Files = append(entity.Files, *pak)
			}
		} else {
			var e Package
			e.Id = &oId.UUID
			e.Name = name
			e.Title = title
			e.Description = description
			e.Price = price
			e.Version = version
			e.ReleasedAt = releasedAt
			e.Downloads = downloads
			e.Public = public
			e.Views = views
			e.Owner = owner
			e.Liked = liked
			e.TotalLikes = totalLikes
			e.TotalDislikes = totalDislikes

			if pak != nil {
				e.Files = append(e.Files, *pak)
			}
			if preview != nil {
				e.Files = append(e.Files, *preview)
			}

			entity = &e
		}
	}

	return entity, err
}

// GetPackageForAdmin Get package
func GetPackageForAdmin(ctx context.Context, requester *sm.User, id uuid.UUID) (entity *Package, err error) {
	db := database.DB

	q := `SELECT 
	m.id                    packageId,
	m.name                  packageName,
	m.title                 packageMap,
	m.description           packageDescription,
	m.price					packagePrice,
	m.version 				packageVersion,
	m.released_at			packageReleasedAt,
	m.downloads 			packageDownloads,
	e.public                entityPublic,
	e.views					entityViews,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size        	previewSize,
	preview.platform 		previewPlatform,
	preview.original_path 	previewOriginalPath,
	preview.hash			previewHash,
	preview.created_at		previewCreatedAt,
	u.id					ownerId,
	u.name					ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes
FROM mods m
    LEFT JOIN entities e ON m.id = e.id
	LEFT JOIN files preview ON e.id = preview.entity_id
	LEFT JOIN accessibles aa on e.id = aa.entity_id
	LEFT JOIN users u ON aa.user_id = u.id AND aa.is_owner
	LEFT JOIN likables l ON l.entity_id = e.id
	LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
WHERE m.id = $2
GROUP BY m.id,
         e.public,
         e.views,
         preview.id,
         preview.url,
         preview.type,
         preview.mime,
         preview.size,
         preview.platform,
         preview.original_path,
         preview.hash,
         preview.created_at,
         u.id,
         l2.value`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, requester.Id /*1*/, id /*$2*/)
	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetPackageForAdmin")
	}()
	for rows.Next() {
		var (
			oId              pgtypeuuid.UUID
			name             string
			title            string
			description      string
			price            *float64
			version          string
			releasedAt       *time.Time
			downloads        *int32
			public           *bool
			views            *int32
			fileId           pgtypeuuid.UUID
			fileUrl          *string
			fileType         *string
			fileMime         *string
			fileSize         *int64
			filePlatform     *string
			fileOriginalPath *string
			fileHash         *string
			fileCreatedAt    *time.Time
			ownerId          pgtypeuuid.UUID
			ownerName        *string
			liked            *int32
			totalLikes       *int32
			totalDislikes    *int32
		)
		err = rows.Scan(
			&oId,
			&name,
			&title,
			&description,
			&price,
			&version,
			&releasedAt,
			&downloads,
			&public,
			&views,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileSize,
			&filePlatform,
			&fileOriginalPath,
			&fileHash,
			&fileCreatedAt,
			&ownerId,
			&ownerName,
			&liked,
			&totalLikes,
			&totalDislikes,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
		}

		if oId.Status == pgtype.Null {
			continue
		}

		var preview *File
		if fileId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &fileId.UUID
			if fileUrl != nil {
				preview.Url = *fileUrl
			}
			if fileType != nil {
				preview.Type = *fileType
			}
			if fileMime != nil {
				preview.Mime = fileMime
			}
			if fileSize != nil {
				preview.Size = fileSize
			}
			if filePlatform != nil {
				preview.Platform = *filePlatform
			}
			if fileOriginalPath != nil {
				preview.OriginalPath = fileOriginalPath
			}
			if fileHash != nil {
				preview.Hash = fileHash
			}
			if fileCreatedAt != nil {
				preview.CreatedAt = *fileCreatedAt
			}
		}

		var owner *User = nil
		if ownerId.Status != pgtype.Null {
			owner = new(User)
			owner.Id = &ownerId.UUID
			if ownerName != nil {
				owner.Name = ownerName
			}
		}

		if entity != nil {
			if preview != nil && !containsFile(entity.Files, *preview.Id) {
				entity.Files = append(entity.Files, *preview)
			}
		} else {
			var e Package
			e.Id = &oId.UUID
			e.Name = name
			e.Title = title
			e.Description = description
			e.Price = price
			e.Version = version
			e.ReleasedAt = releasedAt
			e.Downloads = downloads
			e.Public = public
			e.Views = views
			e.Owner = owner
			e.Liked = liked
			e.TotalLikes = totalLikes
			e.TotalDislikes = totalDislikes

			if preview != nil {
				e.Files = append(e.Files, *preview)
			}

			entity = &e
		}
	}

	return entity, err
}

// GetPackageForRequesterWithPak Get package
func GetPackageForRequesterWithPak(ctx context.Context, requester *sm.User, id uuid.UUID, platform string, deployment string) (entity *Package, err error) {
	db := database.DB

	q := `SELECT 
	m.id                    packageId,
	m.name                  packageName,
	m.title                 packageMap,
	m.description           packageDescription,
	m.price					packagePrice,
	m.version 				packageVersion,
	m.released_at			packageReleasedAt,
	m.downloads 			packageDownloads,
	e.public                entityPublic,
	e.views					entityViews,
	pak.id                  pakId,
	pak.url                 pakUrl,
	pak.type                pakType,
	pak.mime            	pakMime,
	pak.size            	pakSize,
	pak.platform 			pakPlatform,
	pak.original_path 		pakOriginalPath,
	pak.hash				pakHash,
	pak.created_at			pakCreatedAt,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size        	previewSize,
	preview.platform 		previewPlatform,
	preview.original_path 	previewOriginalPath,
	preview.hash			previewHash,
	preview.created_at		previewCreatedAt,
	u.id 					ownerId,
	u.name 					ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes
FROM mods m
    LEFT JOIN entities e ON m.id = e.id
    LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $4::uuid
	LEFT JOIN files pak ON pak.entity_id = m.id AND pak.type = 'pak' AND pak.platform = $1::text AND pak.deployment_type = $2::text
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
	LEFT JOIN accessibles aa on e.id = aa.entity_id
	LEFT JOIN users u ON aa.user_id = u.id AND aa.is_owner
	LEFT JOIN likables l ON l.entity_id = e.id
	LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $4
WHERE m.id = $3 AND (e.public OR a.can_view OR a.is_owner)
GROUP BY m.id,
        u.id,
        e.public,
        e.views,
        pak.id,
        pak.url,
        pak.type,
        pak.mime,
        pak.size,
        pak.platform,
		pak.original_path,
		pak.hash,
        pak.created_at,
		preview.id,
		preview.url,
		preview.type,
		preview.mime,
		preview.size,
		preview.platform,
		preview.original_path,
		preview.hash,
		preview.created_at,
		l2.value`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, platform /*$1*/, deployment /*$2*/, id /*$3*/, requester.Id /*$4*/)
	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetPackageForRequesterWithPak")
	}()
	for rows.Next() {
		var (
			oId              pgtypeuuid.UUID
			name             string
			title            string
			description      string
			price            *float64
			version          string
			releasedAt       *time.Time
			downloads        *int32
			public           *bool
			views            *int32
			pakId            pgtypeuuid.UUID
			pakUrl           *string
			pakType          *string
			pakMime          *string
			pakSize          *int64
			pakPlatform      *string
			pakOriginalPath  *string
			pakHash          *string
			pakCreatedAt     *time.Time
			fileId           pgtypeuuid.UUID
			fileUrl          *string
			fileType         *string
			fileMime         *string
			fileSize         *int64
			filePlatform     *string
			fileOriginalPath *string
			fileHash         *string
			fileCreatedAt    *time.Time
			ownerId          pgtypeuuid.UUID
			ownerName        *string
			liked            *int32
			totalLikes       *int32
			totalDislikes    *int32
		)
		err = rows.Scan(
			&oId,
			&name,
			&title,
			&description,
			&price,
			&version,
			&releasedAt,
			&downloads,
			&public,
			&views,
			&pakId,
			&pakUrl,
			&pakType,
			&pakMime,
			&pakSize,
			&pakPlatform,
			&pakOriginalPath,
			&pakHash,
			&pakCreatedAt,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileSize,
			&filePlatform,
			&fileOriginalPath,
			&fileHash,
			&fileCreatedAt,
			&ownerId,
			&ownerName,
			&liked,
			&totalLikes,
			&totalDislikes,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
		}

		if oId.Status == pgtype.Null {
			continue
		}

		var pak *File
		if pakId.Status != pgtype.Null {
			pak = new(File)
			pak.Id = &pakId.UUID
			if pakUrl != nil {
				pak.Url = *pakUrl
			}
			if pakType != nil {
				pak.Type = *pakType
			}
			if pakMime != nil {
				pak.Mime = pakMime
			}
			if pakSize != nil {
				pak.Size = pakSize
			}
			if pakPlatform != nil {
				pak.Platform = *pakPlatform
			}
			if pakOriginalPath != nil {
				pak.OriginalPath = pakOriginalPath
			}
			if pakHash != nil {
				pak.Hash = pakHash
			}
			if pakCreatedAt != nil {
				pak.CreatedAt = *pakCreatedAt
			}
		}

		var preview *File
		if fileId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &fileId.UUID
			if fileUrl != nil {
				preview.Url = *fileUrl
			}
			if fileType != nil {
				preview.Type = *fileType
			}
			if fileMime != nil {
				preview.Mime = fileMime
			}
			if fileSize != nil {
				preview.Size = fileSize
			}
			if filePlatform != nil {
				preview.Platform = *filePlatform
			}
			if fileOriginalPath != nil {
				preview.OriginalPath = fileOriginalPath
			}
			if fileHash != nil {
				preview.Hash = fileHash
			}
			if fileCreatedAt != nil {
				preview.CreatedAt = *fileCreatedAt
			}
		}

		var owner *User = nil
		if ownerId.Status != pgtype.Null {
			owner = new(User)
			owner.Id = &ownerId.UUID
			if ownerName != nil {
				owner.Name = ownerName
			}
		}

		if entity != nil {
			if preview != nil && !containsFile(entity.Files, *preview.Id) {
				entity.Files = append(entity.Files, *preview)
			}
		} else {
			var e Package
			e.Id = &oId.UUID
			e.Name = name
			e.Title = title
			e.Description = description
			e.Price = price
			e.Version = version
			e.ReleasedAt = releasedAt
			e.Downloads = downloads
			e.Public = public
			e.Views = views
			e.Owner = owner
			e.Liked = liked
			e.TotalLikes = totalLikes
			e.TotalDislikes = totalDislikes

			if pak != nil {
				e.Files = append(e.Files, *pak)
			}
			if preview != nil {
				e.Files = append(e.Files, *preview)
			}

			entity = &e
		}
	}

	return entity, err
}

// GetPackageForRequester Get package
func GetPackageForRequester(ctx context.Context, requester *sm.User, id uuid.UUID) (entity *Package, err error) {
	db := database.DB

	q := `SELECT 
	m.id                    packageId,
	m.name                  packageName,
	m.title                 packageMap,
	m.description           packageDescription,
	m.price					packagePrice,
	m.version 				packageVersion,
	m.released_at			packageReleasedAt,
	m.downloads 			packageDownloads,
	m.release_name         	packageRelease,
	e.public                entityPublic,
	e.views					entityViews,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size        	previewSize,
	preview.platform 		previewPlatform,
	preview.original_path 	previewOriginalPath,
	preview.hash			previewHash,
	preview.created_at		previewCreatedAt,
	u.id    				ownerId,
	u.name 					ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes
FROM mods m
    LEFT JOIN entities e ON m.id = e.id
    LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $2::uuid
	LEFT JOIN files preview ON e.id = preview.entity_id AND (preview.type = 'image_preview' or preview.type = 'pak-extra-content')
	LEFT JOIN accessibles aa on e.id = aa.entity_id
	LEFT JOIN users u ON aa.user_id = u.id AND aa.is_owner
	LEFT JOIN likables l ON l.entity_id = e.id
	LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $2::uuid
WHERE m.id = $1 AND (e.public OR a.can_view OR a.is_owner)
GROUP BY m.id,
        u.id,
        e.public,
        e.views,
        e.updated_at,
        e.created_at,
        aa.created_at,
		preview.id,
		preview.url,
		preview.type,
		preview.mime,
		preview.size,
		preview.platform,
		preview.original_path,
		preview.hash,
		preview.created_at,
		l2.value
ORDER BY e.updated_at DESC, e.created_at DESC, aa.created_at`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, id /*$1*/, requester.Id /*$2*/)
	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetPackageForRequester")
	}()
	for rows.Next() {
		var (
			oId              pgtypeuuid.UUID
			name             string
			title            string
			description      string
			price            *float64
			version          string
			releasedAt       *time.Time
			downloads        *int32
			release          string
			public           *bool
			views            *int32
			fileId           pgtypeuuid.UUID
			fileUrl          *string
			fileType         *string
			fileMime         *string
			fileSize         *int64
			filePlatform     *string
			fileOriginalPath *string
			fileHash         *string
			fileCreatedAt    *time.Time
			ownerId          pgtypeuuid.UUID
			ownerName        *string
			liked            *int32
			totalLikes       *int32
			totalDislikes    *int32
		)
		err = rows.Scan(
			&oId,
			&name,
			&title,
			&description,
			&price,
			&version,
			&releasedAt,
			&downloads,
			&release,
			&public,
			&views,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileSize,
			&filePlatform,
			&fileOriginalPath,
			&fileHash,
			&fileCreatedAt,
			&ownerId,
			&ownerName,
			&liked,
			&totalLikes,
			&totalDislikes,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
		}

		if oId.Status == pgtype.Null {
			continue
		}

		var preview *File
		if fileId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &fileId.UUID
			if fileUrl != nil {
				preview.Url = *fileUrl
			}
			if fileType != nil {
				preview.Type = *fileType
			}
			if fileMime != nil {
				preview.Mime = fileMime
			}
			if fileSize != nil {
				preview.Size = fileSize
			}
			if filePlatform != nil {
				preview.Platform = *filePlatform
			}
			if fileOriginalPath != nil {
				preview.OriginalPath = fileOriginalPath
			}
			if fileHash != nil {
				preview.Hash = fileHash
			}
			if fileCreatedAt != nil {
				preview.CreatedAt = *fileCreatedAt
			}
		}

		var owner *User = nil
		if ownerId.Status != pgtype.Null {
			owner = new(User)
			owner.Id = &ownerId.UUID
			if ownerName != nil {
				owner.Name = ownerName
			}
		}

		if entity != nil {
			if preview != nil && !containsFile(entity.Files, *preview.Id) {
				entity.Files = append(entity.Files, *preview)
			}
		} else {
			var e Package
			e.Id = &oId.UUID
			e.Name = name
			e.Title = title
			e.Description = description
			e.Price = price
			e.Version = version
			e.ReleasedAt = releasedAt
			e.Downloads = downloads
			e.Release = release
			e.Public = public
			e.Views = views
			e.Owner = owner
			e.Liked = liked
			e.TotalLikes = totalLikes
			e.TotalDislikes = totalDislikes

			if preview != nil {
				e.Files = append(e.Files, *preview)
			}

			entity = &e
		}
	}

	return entity, err
}

// CreatePackageForRequester Creates a new world
func CreatePackageForRequester(ctx context.Context, requester *sm.User, m PackageCreateMetadata) (entity *Package, err error) {
	db := database.DB

	id, err1 := uuid.NewV4()
	if err1 != nil {
		return nil, fmt.Errorf("failed to generate uuid: %v", err1)
	}

	m.Name = string(StripNonAscii([]byte(*m.Title)))
	m.Name = string(ReplaceSpaces([]byte(m.Name), '_'))
	m.Name = RemoveDuplicatedRunes(m.Name, '_')
	shortId := shortuuid.DefaultEncoder.Encode(uuid2.UUID(id))
	if len(m.Name) > 31 {
		m.Name = m.Name[:31]
	}
	m.Name = fmt.Sprintf("%s_%s", m.Name, shortId)

	tx, err1 := db.Begin(ctx)
	if err1 != nil {
		return nil, fmt.Errorf("failed to begin tx: %v", err1)
	}

	//region Entity
	entityType := "mod"
	q := `INSERT INTO entities (id, entity_type, public) VALUES ($1, $2, $3)`
	if _, err1 = tx.Exec(ctx, q, id /*1*/, entityType /*2*/, m.Public /*3*/); err1 != nil {
		if err2 := tx.Rollback(ctx); err2 != nil {
			return nil, fmt.Errorf("failed to rollback failed tx: %v, %v", err1, err2)
		}
		return nil, fmt.Errorf("failed to exec tx: %v", err1)
	}
	//endregion

	//region Accessible
	q = `INSERT INTO accessibles (user_id, entity_id, is_owner, can_view, can_edit, can_delete) VALUES ($1, $2, true, true, true, true)`
	if _, err1 = tx.Exec(ctx, q, requester.Id /*1*/, id /*2*/); err1 != nil {
		if err2 := tx.Rollback(ctx); err2 != nil {
			return nil, fmt.Errorf("failed to rollback failed tx: %v, %v", err1, err2)
		}
		return nil, fmt.Errorf("failed to exec tx: %v", err1)
	}
	//endregion

	//region World
	q = `INSERT INTO mods (id,
                    name,
                    title,
					summary,
					description,
					release_name,
					map,
                    version) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	if _, err1 = tx.Exec(ctx, q, id /*1*/, m.Name /*2*/, m.Title /*3*/, m.Summary /*4*/, m.Description /*5*/, m.Release /*6*/, m.Map /*7*/, m.Version /*8*/); err1 != nil {
		if err2 := tx.Rollback(ctx); err2 != nil {
			return nil, fmt.Errorf("failed to rollback failed tx: %v, %v", err1, err2)
		}
		return nil, fmt.Errorf("failed to exec tx: %v", err1)
	}
	//endregion

	if err1 = tx.Commit(ctx); err1 != nil {
		if err2 := tx.Rollback(ctx); err2 != nil {
			return nil, fmt.Errorf("failed to rollback failed tx: %v, %v", err1, err2)
		}
		return nil, fmt.Errorf("failed to commit tx: %v", err1)
	}

	if entity, err1 = GetPackageForRequester(ctx, requester, id); err1 != nil {
		return nil, fmt.Errorf("failed to get the entity: %v", err1)
	}

	if m.Title != nil {
		entity.Title = *m.Title
	}

	return entity, nil
}

func UpdatePackageForRequester(ctx context.Context, requester *sm.User, id uuid.UUID, m PackageUpdateMetadata) (entity *Package, err error) {
	db := database.DB

	tx, err1 := db.Begin(ctx)
	if err1 != nil {
		return nil, fmt.Errorf("failed to begin tx: %v", err1)
	}

	if m.Public != nil {
		q := `UPDATE entities SET public = $1 WHERE id = $2`
		if _, err1 = tx.Exec(ctx, q, m.Public /*$1*/, id /*$2*/); err1 != nil {
			if err2 := tx.Rollback(ctx); err2 != nil {
				return nil, fmt.Errorf("failed to rollback failed tx: %v, %v", err1, err2)
			}
			return nil, fmt.Errorf("failed to exec entity update tx: %v", err1)
		}
	}

	e, err1 := GetPackageForRequester(ctx, requester, id)
	if err1 != nil {
		return nil, fmt.Errorf("failed to get the source world: %v", err1)
	}

	if m.Name != nil {
		e.Name = *m.Name
	}

	if m.Map != nil {
		e.Map = *m.Map
	}

	if m.Summary != nil {
		e.Summary = *m.Summary
	}

	if m.Description != nil {
		e.Description = *m.Description
	}

	if m.Release != nil && *m.Release != "" {
		e.Release = *m.Release
	}

	if m.Version != nil {
		e.Version = *m.Version
	}

	if m.Title != nil {
		e.Title = *m.Title
	}

	q := `UPDATE mods SET name=$1, description=$2, summary=$3, map=$4, release_name=$5, version=$6, title=$7 WHERE id = $8`
	if _, err1 = tx.Exec(ctx, q, e.Name /*$1*/, e.Description /*$2*/, e.Summary /*$3*/, e.Map /*$4*/, e.Release /*$5*/, e.Version /*$6*/, e.Title /*$7*/, id); err1 != nil {
		if err2 := tx.Rollback(ctx); err2 != nil {
			return nil, fmt.Errorf("failed to rollback failed tx: %v, %v", err1, err2)
		}
		return nil, fmt.Errorf("failed to exec world update tx: %v", err1)
	}

	if err1 = tx.Commit(ctx); err1 != nil {
		if err2 := tx.Rollback(ctx); err2 != nil {
			return nil, fmt.Errorf("failed to rollback failed tx: %v, %v", err1, err2)
		}
		return nil, fmt.Errorf("failed to commit tx: %v", err1)
	}

	e, err1 = GetPackageForRequester(ctx, requester, id)
	if err1 != nil {
		return nil, fmt.Errorf("failed to get the updated world: %v", err1)
	}

	return e, nil
}

// GetLatestPackageForAdminWithPak Get latest package
func GetLatestPackageForAdminWithPak(ctx context.Context, platform string, deployment string) (entity *Package, err error) {
	db := database.DB

	q := `SELECT 
	m.id                    packageId,
	m.name                  packageName,
	m.title                 packageMap,
	m.description           packageDescription,
	e.public                entityPublic,
	pak.id                  pakId,
	pak.url                 pakUrl,
	pak.type                pakType,
	pak.mime            	pakMime,
	pak.size            	pakSize,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size        	previewSize,
	u.id ownerId,
	u.name ownerName
FROM mods m
    LEFT JOIN entities e ON m.id = e.id
	LEFT JOIN files pak ON pak.entity_id = m.id AND pak.type = 'pak' AND pak.platform = $1::text AND pak.deployment_type = $2::text
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
	LEFT JOIN accessibles aa on e.id = aa.entity_id
	LEFT JOIN users u ON aa.user_id = u.id AND aa.is_owner
ORDER BY e.created_at`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, platform /*$1*/, deployment /*$2*/)
	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	var latestId uuid.UUID
	defer func() {
		rows.Close()
		database.LogPgxStat("GetLatestPackageForAdminWithPak")
	}()
	for rows.Next() {
		var (
			oId         pgtypeuuid.UUID
			name        *string
			title       *string
			description *string
			public      *bool
			pakId       pgtypeuuid.UUID
			pakUrl      *string
			pakType     *string
			pakMime     *string
			pakSize     *int64
			fileId      pgtypeuuid.UUID
			fileUrl     *string
			fileType    *string
			fileMime    *string
			fileSize    *int64
			ownerId     pgtypeuuid.UUID
			ownerName   *string
		)
		err = rows.Scan(
			&oId,
			&name,
			&title,
			&description,
			&public,
			&pakId,
			&pakUrl,
			&pakType,
			&pakMime,
			&pakSize,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileSize,
			&ownerId,
			&ownerName,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
		}

		if oId.Status == pgtype.Null {
			continue
		}

		var pak *File
		if pakId.Status != pgtype.Null {
			pak = new(File)
			pak.Id = &pakId.UUID
			if pakUrl != nil {
				pak.Url = *pakUrl
			}

			if pakType != nil {
				pak.Type = *pakType
			}

			if pakSize != nil {
				pak.Size = pakSize
			}

			if pakMime != nil {
				pak.Size = pakSize
			}
		}

		var preview *File
		if fileId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &fileId.UUID
			if fileUrl != nil {
				preview.Url = *fileUrl
			}

			if fileType != nil {
				preview.Type = *fileType
			}

			if fileMime != nil {
				preview.Mime = fileMime
			}

			if fileSize != nil {
				preview.Size = fileSize
			}
		}

		var owner *User = nil
		if ownerId.Status != pgtype.Null {
			owner = new(User)
			owner.Id = &ownerId.UUID
			if ownerName != nil {
				owner.Name = ownerName
			}
		}

		if entity != nil {
			if latestId != *entity.Id {
				// Next entity, stop iteration
				rows.Close()
				break
			}

			if preview != nil && !containsFile(entity.Files, *preview.Id) {
				entity.Files = append(entity.Files, *preview)
			}
		} else {
			var e Package
			e.Id = &oId.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if preview != nil {
				e.Files = append(e.Files, *preview)
			}
			e.Owner = owner

			entity = &e
		}

		latestId = *entity.Id
	}

	return entity, err
}

func IndexPackageMapsForRequester(ctx context.Context, requester *sm.User, id uuid.UUID, offset int64, limit int64) ([]string, error) {
	db := database.DB

	q := `SELECT DISTINCT w.map
FROM mods m 
    LEFT JOIN entities e ON m.id = e.id
    LEFT JOIN spaces w ON m.id = w.mod_id
    LEFT JOIN entities we ON w.id = we.id
    LEFT JOIN accessibles a ON m.id = a.entity_id AND a.user_id = $1::uuid
	LEFT JOIN accessibles aa ON w.id = aa.entity_id AND aa.user_id = $1::uuid
WHERE m.id = $2::uuid AND (a.is_owner OR a.can_view OR e.public) AND (aa.is_owner OR a.can_view OR we.public)
OFFSET $3::int LIMIT $4::int`

	var (
		rows pgx.Rows
		err  error
	)

	rows, err = db.Query(ctx, q, requester.Id, id, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	var maps []string
	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPackageMapsForRequester")
	}()
	for rows.Next() {
		var (
			wMap *string
		)
		err = rows.Scan(&wMap)
		if err != nil {
			return nil, fmt.Errorf("failed to scan %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
		}

		var el string
		if wMap != nil {
			el = *wMap
		}

		maps = append(maps, el)
	}

	return maps, nil
}
