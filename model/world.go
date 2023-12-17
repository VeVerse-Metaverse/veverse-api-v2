package model

import (
	"context"
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/jackc/pgtype"
	pgtypeuuid "github.com/jackc/pgtype/ext/gofrs-uuid"
	"github.com/jackc/pgx/v5"
	"veverse-api/database"
	"veverse-api/reflect"
)

const (
	worldSingular = "world"
	worldPlural   = "worlds"
)

// World struct
type World struct {
	Entity
	Name          string   `json:"name,omitempty"`
	Description   string   `json:"description,omitempty"`
	Map           string   `json:"map,omitempty"`
	GameMode      string   `json:"gameMode,omitempty"`
	Package       *Package `json:"metaverse,omitempty"`
	Type          *string  `json:"type,omitempty"`
	Liked         *int32   `json:"liked,omitempty"`
	TotalLikes    *int32   `json:"totalLikes,omitempty"`
	TotalDislikes *int32   `json:"totalDislikes,omitempty"`
}

// WorldPlaceables struct
type WorldPlaceables struct {
	World
	Placeables []Object `json:"placeables,omitempty"`
}

// WorldBatchRequestMetadata Batch request metadata for requesting Portal entities
type WorldBatchRequestMetadata struct {
	BatchRequestMetadata
	PackageId  *string `json:"metaverseId,omitempty"` // Optional World where the portal is located to filter portals
	Platform   string  `json:"platform,omitempty"`    // SupportedPlatform (OS) of the pak file (Win64, Mac, Linux, IOS, Android)
	Deployment string  `json:"deployment,omitempty"`  // SupportedDeployment for the pak file (Server or Client)
}

type WorldRequestMetadata struct {
	PackageRequestMetadata
}

type WorldCreateMetadata struct {
	Name        string    `json:"name,omitempty"`        // Name that used as space identifier and part of the world URL
	Map         string    `json:"map,omitempty"`         // Map that is used to run the space in the game client
	Title       *string   `json:"title,omitempty"`       // Title visible to users
	Public      *bool     `json:"public,omitempty"`      // Public or private
	Description *string   `json:"description,omitempty"` // Description
	PackageId   uuid.UUID `json:"modId,omitempty"`       // Base PackageId
	Type        *string   `json:"type,omitempty"`        // Type of the space
	GameMode    *string   `json:"gameMode,omitempty"`    // Class of the GameMode blueprint used within the space to override default GameMode
}

type WorldUpdateMetadata struct {
	Name        *string    `json:"name,omitempty"`        // Name that used as space identifier and part of the world URL
	Map         *string    `json:"map,omitempty"`         // Map that is used to run the space in the game client
	Public      *bool      `json:"public,omitempty"`      // Public or private
	Description *string    `json:"description,omitempty"` // Description
	PackageId   *uuid.UUID `json:"modId,omitempty"`       // Base PackageId
	Type        *string    `json:"type,omitempty"`        // Type of the space
	GameMode    *string    `json:"gameMode,omitempty"`    // Class of the GameMode blueprint used within the space to override default GameMode
}

func findWorld(h []World, id uuid.UUID) int {
	for i, v := range h {
		if *v.Id == id {
			return i
		}
	}
	return -1
}

// IndexWorldsForAdmin Index packages for admin
func IndexWorldsForAdmin(ctx context.Context, requester *User, offset int64, limit int64) (entities []World, total int64, err error) {
	db := database.DB

	//region Count
	q := `SELECT COUNT(*) FROM spaces w`

	row := db.QueryRow(ctx, q)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	q = `SELECT 
	w.id                    worldId,
	w.name                  worldName,
	w.description           worldDescription,
	w.map                   worldMap,
	w.game_mode             worldGameMode,
	e.public                entityPublic,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size			previewSize,
	owner.id 				ownerId,
	owner.name 				ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM spaces w
    LEFT JOIN entities e ON w.id = e.id
	LEFT JOIN likables l ON l.entity_id = e.id
    LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
	LEFT JOIN accessibles a ON a.entity_id = e.id
	LEFT JOIN users owner ON owner.id = a.user_id
	LEFT JOIN mods m ON w.mod_id = m.id
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
GROUP BY w.id, e.public, preview.id, preview.url, preview.type, preview.mime, preview.size, owner.id, e.updated_at, e.created_at, e.id, l2.value, e.views
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, requester.Id)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	var (
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexWorldsForAdmin")
	}()
	for rows.Next() {
		var (
			id            pgtypeuuid.UUID
			name          *string
			description   *string
			mapName       *string
			gameMode      *string
			public        *bool
			fileId        pgtypeuuid.UUID
			fileUrl       *string
			fileType      *string
			fileMime      *string
			fileSize      *int64
			ownerId       *uuid.UUID
			ownerName     *string
			liked         *int32
			totalLikes    *int32
			totalDislikes *int32
			views         *int32
		)

		err = rows.Scan(
			&id,
			&name,
			&description,
			&mapName,
			&gameMode,
			&public,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileSize,
			&ownerId,
			&ownerName,
			&liked,
			&totalLikes,
			&totalDislikes,
			&views,
		)
		if err != nil {
			return nil, -1, err
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

			if fileUrl != nil {
				file.Url = *fileUrl
			}

			if fileSize != nil {
				file.Size = fileSize
			}
		}

		if i := findWorld(entities, id.UUID); i >= 0 {
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

			var e World
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if description != nil {
				e.Description = *description
			}
			if gameMode != nil {
				e.GameMode = *gameMode
			}
			if mapName != nil {
				e.Map = *mapName
			}
			if file != nil {
				e.Files = append(e.Files, *file)
			}

			e.Owner = new(User)
			if ownerId != nil {
				e.Owner.Id = ownerId
			}

			if ownerName != nil {
				e.Owner.Name = ownerName
			}

			if liked != nil {
				e.Liked = liked
			}

			if totalLikes != nil {
				e.TotalLikes = totalLikes
			} else {
				e.TotalLikes = new(int32)
				*e.TotalLikes = 0
			}

			if totalDislikes != nil {
				e.TotalDislikes = totalDislikes
			} else {
				e.TotalDislikes = new(int32)
				*e.TotalDislikes = 0
			}

			e.Views = views

			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexWorldsForAdminWithPak Index packages for admin with pak file
func IndexWorldsForAdminWithPak(ctx context.Context, requester *sm.User, offset int64, limit int64, platform string, deployment string) (entities []World, total int64, err error) {
	db := database.DB

	q := `SELECT COUNT(*) FROM spaces w`

	row := db.QueryRow(ctx, q)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}

	q = `SELECT 
	w.id                    worldId,
	w.name                  worldName,
	w.description           worldDescription,
	w.map                   worldMap,
	w.game_mode             worldGameMode,
	m.id					modId,
	m.name					modName,
	m.title					modTitle,
	e.public                entityPublic,
	pak.id                  pakId,
	pak.url                 pakUrl,
	pak.type                pakType,
	pak.mime            	pakMime,
	pak.size				pakSize,
	pak.original_path		pakOriginalPath,
	pak.hash				pakHash,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size			previewSize,
	preview.original_path	previewOriginalPath,
	preview.hash			previewHash,
	owner.id 				ownerId,
	owner.name 				ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM spaces w
    LEFT JOIN entities e ON w.id = e.id
	LEFT JOIN likables l ON l.entity_id = e.id
    LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
	LEFT JOIN mods m ON w.mod_id = m.id
	LEFT JOIN files pak ON pak.entity_id = m.id AND ((pak.platform = $2::text AND pak.deployment_type = $3::text) OR (pak.platform = '' AND pak.deployment_type = ''))
    LEFT JOIN accessibles a ON a.entity_id = e.id
	LEFT JOIN users owner ON owner.id = a.user_id
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
GROUP BY w.id, m.id, e.updated_at, e.created_at, e.id, pak.id, pak.url, pak.type, pak.mime, pak.size, pak.original_path, pak.hash, preview.id,
preview.url, preview.type, preview.mime, preview.size, preview.original_path, preview.hash, owner.id, l2.value, e.views
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows      pgx.Rows
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, requester.Id, platform /*$1*/, deployment /*$2*/)

	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexWorldsForAdminWithPak")
	}()
	for rows.Next() {
		var (
			id               pgtypeuuid.UUID
			name             *string
			description      *string
			mapName          *string
			gameMode         *string
			modId            *uuid.UUID
			modName          *string
			modTitle         *string
			public           *bool
			pakId            pgtypeuuid.UUID
			pakUrl           *string
			pakType          *string
			pakMime          *string
			pakSize          *int64
			pakOriginalPath  *string
			pakHash          *string
			fileId           pgtypeuuid.UUID
			fileUrl          *string
			fileType         *string
			fileMime         *string
			fileSize         *int64
			fileOriginalPath *string
			fileHash         *string
			ownerId          *uuid.UUID
			ownerName        *string
			liked            *int32
			totalLikes       *int32
			totalDislikes    *int32
			views            *int32
		)

		err = rows.Scan(
			&id,
			&name,
			&description,
			&mapName,
			&gameMode,
			&modId,
			&modName,
			&modTitle,
			&public,
			&pakId,
			&pakUrl,
			&pakType,
			&pakMime,
			&pakSize,
			&pakOriginalPath,
			&pakHash,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileSize,
			&fileOriginalPath,
			&fileHash,
			&ownerId,
			&ownerName,
			&liked,
			&totalLikes,
			&totalDislikes,
			&views,
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

			if pakUrl != nil {
				pak.Url = *pakUrl
			}

			if pakSize != nil {
				pak.Size = pakSize
			}

			if pakOriginalPath != nil && *pakOriginalPath != "" {
				pak.OriginalPath = pakOriginalPath
			}

			if pakHash != nil && *pakHash != "" {
				pak.Hash = pakHash
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

			if fileOriginalPath != nil && *fileOriginalPath != "" {
				file.OriginalPath = fileOriginalPath
			}

			if fileHash != nil && *fileHash != "" {
				file.Hash = fileHash
			}
		}
		//endregion

		if i := findWorld(entities, id.UUID); i >= 0 {
			if file != nil && !containsFile(entities[i].Files, *file.Id) {
				entities[i].Files = append(entities[i].Files, *file)
			}

			if pak != nil && !containsFile(entities[i].Package.Files, *pak.Id) {
				entities[i].Package.Files = append(entities[i].Package.Files, *pak)
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

			var e World
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if description != nil {
				e.Description = *description
			}
			if gameMode != nil {
				e.GameMode = *gameMode
			}
			if mapName != nil {
				e.Map = *mapName
			}
			if file != nil {
				e.Files = append(e.Files, *file)
			}

			if e.Package == nil && modId != nil {
				e.Package = new(Package)
				e.Package.Id = modId

				if modName != nil {
					e.Package.Name = *modName
				}

				if modTitle != nil {
					e.Package.Title = *modTitle
				}

				if pak != nil {
					e.Package.Files = append(e.Files, *pak)
				}
			}

			e.Owner = new(User)
			if ownerId != nil {
				e.Owner.Id = ownerId
			}

			if ownerName != nil {
				e.Owner.Name = ownerName
			}

			if liked != nil {
				e.Liked = liked
			}

			if totalLikes != nil {
				e.TotalLikes = totalLikes
			} else {
				e.TotalLikes = new(int32)
				*e.TotalLikes = 0
			}

			if totalDislikes != nil {
				e.TotalDislikes = totalDislikes
			} else {
				e.TotalDislikes = new(int32)
				*e.TotalDislikes = 0
			}

			e.Views = views

			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexWorldsForAdminForPackage Index packages for admin
func IndexWorldsForAdminForPackage(ctx context.Context, requester *User, packageId uuid.UUID, offset int64, limit int64) (entities []World, total int64, err error) {
	db := database.DB

	q := `SELECT COUNT(*)
FROM spaces w
WHERE w.mod_id = $1`

	row := db.QueryRow(ctx, q, packageId)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}

	q = `SELECT 
	w.id                    worldId,
	w.name                  worldName,
	w.description           worldDescription,
	w.map                   worldMap,
	w.game_mode             worldGameMode,
	e.public                entityPublic,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size			previewSize,
	owner.id 				ownerId,
	owner.name 				ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM spaces w
    LEFT JOIN entities e ON w.id = e.id
    LEFT JOIN likables l ON l.entity_id = e.id
	LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
	LEFT JOIN accessibles a ON a.entity_id = e.id
	LEFT JOIN users owner ON owner.id = a.user_id
	LEFT JOIN mods m ON w.mod_id = m.id
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
WHERE w.mod_id = $2
GROUP BY e.id, w.id, e.updated_at, e.created_at, preview.id, preview.url, preview.type, preview.mime, preview.size, owner.id, l2.value, e.views
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, requester.Id /*$1*/, packageId /*$2*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	var (
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexWorldsForAdminForPackage")
	}()
	for rows.Next() {
		var (
			id            pgtypeuuid.UUID
			name          *string
			description   *string
			mapName       *string
			gameMode      *string
			public        *bool
			fileId        pgtypeuuid.UUID
			fileUrl       *string
			fileType      *string
			fileMime      *string
			fileSize      *int64
			ownerId       *uuid.UUID
			ownerName     *string
			liked         *int32
			totalLikes    *int32
			totalDislikes *int32
			views         *int32
		)

		err = rows.Scan(
			&id,
			&name,
			&description,
			&mapName,
			&gameMode,
			&public,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileSize,
			&ownerId,
			&ownerName,
			&liked,
			&totalLikes,
			&totalDislikes,
			&views,
		)
		if err != nil {
			return nil, -1, err
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

			if fileUrl != nil {
				file.Url = *fileUrl
			}

			if fileSize != nil {
				file.Size = fileSize
			}
		}

		if i := findWorld(entities, id.UUID); i >= 0 {
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

			var e World
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if description != nil {
				e.Description = *description
			}
			if gameMode != nil {
				e.GameMode = *gameMode
			}
			if mapName != nil {
				e.Map = *mapName
			}
			if file != nil {
				e.Files = append(e.Files, *file)
			}

			e.Owner = new(User)
			if ownerId != nil {
				e.Owner.Id = ownerId
			}

			if ownerName != nil {
				e.Owner.Name = ownerName
			}

			if liked != nil {
				e.Liked = liked
			}

			if totalLikes != nil {
				e.TotalLikes = totalLikes
			} else {
				e.TotalLikes = new(int32)
				*e.TotalLikes = 0
			}

			if totalDislikes != nil {
				e.TotalDislikes = totalDislikes
			} else {
				e.TotalDislikes = new(int32)
				*e.TotalDislikes = 0
			}

			e.Views = views

			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexWorldsForAdminForPackageWithPak Index packages for admin with pak file
func IndexWorldsForAdminForPackageWithPak(ctx context.Context, requester *sm.User, packageId uuid.UUID, offset int64, limit int64, platform string, deployment string) (entities []World, total int64, err error) {
	db := database.DB

	q := `SELECT COUNT(*)
FROM spaces w
WHERE w.mod_id = $1`

	row := db.QueryRow(ctx, q, packageId /*1*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}

	q = `SELECT 
	w.id                    worldId,
	w.name                  worldName,
	w.description           worldDescription,
	w.map                   worldMap,
	w.game_mode             worldGameMode,
	m.id					modId,
	m.name					modName,
	m.title					modTitle,
	e.public                entityPublic,
	pak.id                  pakId,
	pak.url                 pakUrl,
	pak.type                pakType,
	pak.mime            	pakMime,
	pak.size				pakSize,
	pak.original_path		pakOriginalPath,
	pak.hash				pakHash,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size			previewSize,
	preview.original_path	previewOriginalPath,
	preview.hash			previewHash,
	owner.id 				ownerId,
	owner.name 				ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM spaces w
    LEFT JOIN entities e ON w.id = e.id
	LEFT JOIN likables l ON l.entity_id = e.id
    LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
	LEFT JOIN accessibles a ON a.entity_id = e.id
	LEFT JOIN users owner ON owner.id = a.user_id
	LEFT JOIN mods m ON w.mod_id = m.id
	LEFT JOIN files pak ON pak.entity_id = m.id AND ((pak.platform = $2::text AND pak.deployment_type = $3::text) OR (pak.platform = '' AND pak.deployment_type = ''))
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
WHERE w.mod_id = $4
GROUP BY e.id, w.id, m.id, e.public, pak.id, pak.url, pak.type, pak.mime, pak.size, pak.original_path, pak.hash, preview.id, preview.url, preview.type, preview.mime, preview.size, preview.original_path, preview.hash, owner.id, l2.value, e.updated_at, e.created_at, e.views
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, requester.Id /*$1*/, platform /*$2*/, deployment /*$3*/, packageId /*$4*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	var (
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexWorldsForAdminForPackageWithPak")
	}()
	for rows.Next() {
		var (
			id               pgtypeuuid.UUID
			name             *string
			description      *string
			mapName          *string
			gameMode         *string
			modId            *uuid.UUID
			modName          *string
			modTitle         *string
			public           *bool
			pakId            pgtypeuuid.UUID
			pakUrl           *string
			pakType          *string
			pakMime          *string
			pakSize          *int64
			pakOriginalPath  *string
			pakHash          *string
			fileId           pgtypeuuid.UUID
			fileUrl          *string
			fileType         *string
			fileMime         *string
			fileSize         *int64
			fileOriginalPath *string
			fileHash         *string
			ownerId          *uuid.UUID
			ownerName        *string
			liked            *int32
			totalLikes       *int32
			totalDislikes    *int32
			views            *int32
		)

		err = rows.Scan(&id, &name, &description, &mapName, &gameMode, &modId, &modName, &modTitle, &public,
			&pakId, &pakUrl, &pakType, &pakMime, &pakSize, &pakOriginalPath, &pakHash,
			&fileId, &fileUrl, &fileType, &fileMime, &fileSize, &fileOriginalPath, &fileHash,
			&ownerId, &ownerName, &liked, &totalLikes, &totalDislikes, &views)
		if err != nil {
			return nil, -1, err
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

			if fileUrl != nil {
				file.Url = *fileUrl
			}

			if fileSize != nil {
				file.Size = fileSize
			}

			if fileOriginalPath != nil && *fileOriginalPath != "" {
				file.OriginalPath = fileOriginalPath
			}

			if fileHash != nil && *fileHash != "" {
				file.Hash = fileHash
			}
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

			if pakUrl != nil {
				pak.Url = *pakUrl
			}

			if pakSize != nil {
				pak.Size = pakSize
			}

			if pakOriginalPath != nil && *pakOriginalPath != "" {
				pak.OriginalPath = pakOriginalPath
			}

			if pakHash != nil && *pakHash != "" {
				pak.Hash = pakHash
			}
		}

		if i := findWorld(entities, id.UUID); i >= 0 {
			if file != nil && !containsFile(entities[i].Files, *file.Id) {
				entities[i].Files = append(entities[i].Files, *file)
			}

			if pak != nil && !containsFile(entities[i].Package.Files, *pak.Id) {
				entities[i].Package.Files = append(entities[i].Package.Files, *pak)
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

			var e World
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if description != nil {
				e.Description = *description
			}
			if gameMode != nil {
				e.GameMode = *gameMode
			}
			if mapName != nil {
				e.Map = *mapName
			}
			if file != nil {
				e.Files = append(e.Files, *file)
			}

			if e.Package == nil && modId != nil {
				e.Package = new(Package)
				e.Package.Id = modId

				if modName != nil {
					e.Package.Name = *modName
				}

				if modTitle != nil {
					e.Package.Title = *modTitle
				}

				if pak != nil {
					e.Package.Files = append(e.Files, *pak)
				}
			}

			e.Owner = new(User)
			if ownerId != nil {
				e.Owner.Id = ownerId
			}

			if ownerName != nil {
				e.Owner.Name = ownerName
			}

			if liked != nil {
				e.Liked = liked
			}

			if totalLikes != nil {
				e.TotalLikes = totalLikes
			} else {
				e.TotalLikes = new(int32)
				*e.TotalLikes = 0
			}

			if totalDislikes != nil {
				e.TotalDislikes = totalDislikes
			} else {
				e.TotalDislikes = new(int32)
				*e.TotalDislikes = 0
			}

			e.Views = views

			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexWorldsForAdminWithQuery Index packages for admin with query and pak file
func IndexWorldsForAdminWithQuery(ctx context.Context, requester *User, offset int64, limit int64, query string) (entities []World, total int64, err error) {
	db := database.DB

	q := `SELECT COUNT(*) FROM spaces w LEFT JOIN mods m ON w.mod_id = m.id WHERE w.name ILIKE $1::text OR m.name ILIKE $1::text`

	row := db.QueryRow(ctx, q, query /*1*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}

	q = `SELECT 
	w.id                    worldId,
	w.name                  worldName,
	w.description           worldDescription,
	w.map                   worldMap,
	w.game_mode             worldGameMode,
	e.public                entityPublic,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size			previewSize,
	owner.id 				ownerId,
	owner.name 				ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM spaces w
    LEFT JOIN entities e ON w.id = e.id
    LEFT JOIN likables l ON l.entity_id = e.id
    LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
    LEFT JOIN accessibles a ON a.entity_id = e.id
	LEFT JOIN users owner ON owner.id = a.user_id
	LEFT JOIN mods m ON w.mod_id = m.id
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
WHERE w.name ILIKE $2::text OR m.name ILIKE $2::text
GROUP BY e.id, w.id, e.public, preview.id, preview.url, preview.type, preview.mime, preview.size, owner.id, l2.value, e.updated_at, e.created_at, e.views
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows      pgx.Rows
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, requester.Id, query /*$2*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexWorldsForAdminWithQuery")
	}()
	for rows.Next() {
		var (
			id            pgtypeuuid.UUID
			name          *string
			description   *string
			mapName       *string
			gameMode      *string
			public        *bool
			fileId        pgtypeuuid.UUID
			fileUrl       *string
			fileType      *string
			fileMime      *string
			fileSize      *int64
			ownerId       *uuid.UUID
			ownerName     *string
			liked         *int32
			totalLikes    *int32
			totalDislikes *int32
			views         *int32
		)

		err = rows.Scan(
			&id,
			&name,
			&description,
			&mapName,
			&gameMode,
			&public,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileSize,
			&ownerId,
			&ownerName,
			&liked,
			&totalLikes,
			&totalDislikes,
			&views,
		)
		if err != nil {
			return nil, -1, err
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

			if fileUrl != nil {
				file.Url = *fileUrl
			}

			if fileSize != nil {
				file.Url = *fileUrl
			}
		}

		if i := findWorld(entities, id.UUID); i >= 0 {
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

			var e World
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if description != nil {
				e.Description = *description
			}
			if gameMode != nil {
				e.GameMode = *gameMode
			}
			if mapName != nil {
				e.Map = *mapName
			}
			if file != nil {
				e.Files = append(e.Files, *file)
			}

			e.Owner = new(User)
			if ownerId != nil {
				e.Owner.Id = ownerId
			}

			if ownerName != nil {
				e.Owner.Name = ownerName
			}

			if liked != nil {
				e.Liked = liked
			}

			if totalLikes != nil {
				e.TotalLikes = totalLikes
			} else {
				e.TotalLikes = new(int32)
				*e.TotalLikes = 0
			}

			if totalDislikes != nil {
				e.TotalDislikes = totalDislikes
			} else {
				e.TotalDislikes = new(int32)
				*e.TotalDislikes = 0
			}

			e.Views = views

			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexWorldsForAdminWithQueryWithPak Index packages for admin with query and pak file
func IndexWorldsForAdminWithQueryWithPak(ctx context.Context, requester *sm.User, offset int64, limit int64, query string, platform string, deployment string) (entities []World, total int64, err error) {
	db := database.DB

	q := `SELECT COUNT(*) FROM spaces w LEFT JOIN mods m on w.mod_id = m.id WHERE w.name ILIKE $1::text OR m.name ILIKE $1::text`

	row := db.QueryRow(ctx, q, query /*$1*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}

	q = `SELECT 
	w.id                    worldId,
	w.name                  worldName,
	w.description           worldDescription,
	w.map                   worldMap,
	w.game_mode             worldGameMode,
	m.id					modId,
	m.name					modName,
	m.title					modTitle,
	e.public                entityPublic,
	pak.id                  pakId,
	pak.url                 pakUrl,
	pak.type                pakType,
	pak.mime            	pakMime,
	pak.size				pakSize,
	pak.original_path		pakOriginalPath,
	pak.hash				pakHash,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size			previewSize,
	preview.original_path	previewOriginalPath,
	preview.hash			previewHash,
	owner.id 				ownerId,
	owner.name 				ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM spaces w
    LEFT JOIN entities e ON w.id = e.id
    LEFT JOIN likables l ON l.entity_id = e.id
    LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
    LEFT JOIN accessibles a ON a.entity_id = e.id
	LEFT JOIN users owner ON owner.id = a.user_id
	LEFT JOIN mods m ON w.mod_id = m.id
	LEFT JOIN files pak ON pak.entity_id = m.id AND ((pak.platform = $2::text AND pak.deployment_type = $3::text) OR (pak.platform = '' AND pak.deployment_type = ''))
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
WHERE w.name ILIKE $4::text OR m.name ILIKE $4::text
GROUP BY e.id, m.id, w.id, e.public, pak.id, pak.url, pak.type, pak.mime, pak.size, pak.original_path, pak.hash, preview.id, preview.url, preview.type, preview.mime, preview.size, preview.original_path, preview.hash, owner.id, l2.value, e.updated_at, e.created_at, e.views
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows      pgx.Rows
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)
	rows, err = db.Query(ctx, q, requester.Id, platform /*$1*/, deployment /*$2*/, query /*$3*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexWorldsForAdminWithQueryWithPak")
	}()
	for rows.Next() {
		var (
			id               pgtypeuuid.UUID
			name             *string
			description      *string
			mapName          *string
			gameMode         *string
			modId            *uuid.UUID
			modName          *string
			modTitle         *string
			public           *bool
			pakId            pgtypeuuid.UUID
			pakUrl           *string
			pakType          *string
			pakMime          *string
			pakSize          *int64
			pakOriginalPath  *string
			pakHash          *string
			fileId           pgtypeuuid.UUID
			fileUrl          *string
			fileType         *string
			fileMime         *string
			fileSize         *int64
			fileOriginalPath *string
			fileHash         *string
			ownerId          *uuid.UUID
			ownerName        *string
			liked            *int32
			totalLikes       *int32
			totalDislikes    *int32
			views            *int32
		)

		err = rows.Scan(
			&id,
			&name,
			&description,
			&mapName,
			&gameMode,
			&modId,
			&modName,
			&modTitle,
			&public,
			&pakId,
			&pakUrl,
			&pakType,
			&pakMime,
			&pakSize,
			&pakOriginalPath,
			&pakHash,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileSize,
			&fileOriginalPath,
			&fileHash,
			&ownerId,
			&ownerName,
			&liked,
			&totalLikes,
			&totalDislikes,
			&views,
		)
		if err != nil {
			return nil, -1, err
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

			if pakUrl != nil {
				pak.Url = *pakUrl
			}

			if pakSize != nil {
				pak.Size = pakSize
			}

			if pakOriginalPath != nil && *pakOriginalPath != "" {
				pak.OriginalPath = pakOriginalPath
			}

			if pakHash != nil && *pakHash != "" {
				pak.Hash = pakHash
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

			if fileOriginalPath != nil && *fileOriginalPath != "" {
				file.OriginalPath = fileOriginalPath
			}

			if fileHash != nil && *fileHash != "" {
				file.Hash = fileHash
			}
		}

		if i := findWorld(entities, id.UUID); i >= 0 {
			if file != nil && !containsFile(entities[i].Files, *file.Id) {
				entities[i].Files = append(entities[i].Files, *file)
			}

			if pak != nil && !containsFile(entities[i].Package.Files, *pak.Id) {
				entities[i].Package.Files = append(entities[i].Package.Files, *pak)
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

			var e World
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if description != nil {
				e.Description = *description
			}
			if gameMode != nil {
				e.GameMode = *gameMode
			}
			if mapName != nil {
				e.Map = *mapName
			}
			if file != nil {
				e.Files = append(e.Files, *file)
			}

			if e.Package == nil && modId != nil {
				e.Package = new(Package)
				e.Package.Id = modId

				if modName != nil {
					e.Package.Name = *modName
				}

				if modTitle != nil {
					e.Package.Title = *modTitle
				}

				if pak != nil {
					e.Package.Files = append(e.Files, *pak)
				}
			}

			e.Owner = new(User)
			if ownerId != nil {
				e.Owner.Id = ownerId
			}

			if ownerName != nil {
				e.Owner.Name = ownerName
			}

			if liked != nil {
				e.Liked = liked
			}

			if totalLikes != nil {
				e.TotalLikes = totalLikes
			} else {
				e.TotalLikes = new(int32)
				*e.TotalLikes = 0
			}

			if totalDislikes != nil {
				e.TotalDislikes = totalDislikes
			} else {
				e.TotalDislikes = new(int32)
				*e.TotalDislikes = 0
			}

			e.Views = views

			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexWorldsForAdminForPackageWithQuery Index packages for admin with query and pak file
func IndexWorldsForAdminForPackageWithQuery(ctx context.Context, requester *User, packageId uuid.UUID, offset int64, limit int64, query string) (entities []World, total int64, err error) {
	db := database.DB

	q := `SELECT COUNT(*)
FROM spaces w
	LEFT JOIN mods m on w.mod_id = m.id
WHERE w.mod_id = $1 AND (w.name ILIKE $2::text OR m.name ILIKE $2::text)`

	row := db.QueryRow(ctx, q, packageId /*$1*/, query /*$2*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}

	q = `SELECT 
	w.id                    worldId,
	w.name                  worldName,
	w.description           worldDescription,
	w.map                   worldMap,
	w.game_mode             worldGameMode,
	e.public                entityPublic,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size			previewSize,
	owner.id 				ownerId,
	owner.name 				ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM spaces w
    LEFT JOIN entities e ON w.id = e.id
	LEFT JOIN likables l ON l.entity_id = e.id
    LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
	LEFT JOIN accessibles a ON a.entity_id = e.id
	LEFT JOIN users owner ON owner.id = a.user_id
	LEFT JOIN mods m ON w.mod_id = m.id
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
WHERE w.mod_id = $2 AND (w.name ILIKE $3::text OR m.name ILIKE $3::text) 
GROUP BY e.id, w.id, e.public, preview.id, preview.url, preview.type, preview.mime, preview.size, owner.id, l2.value, e.updated_at, e.created_at, e.views
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, requester.Id, packageId /*$1*/, query /*$2*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	var (
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexWorldsForAdminForPackageWithQuery")
	}()
	for rows.Next() {
		var (
			id            pgtypeuuid.UUID
			name          *string
			description   *string
			mapName       *string
			gameMode      *string
			public        *bool
			fileId        pgtypeuuid.UUID
			fileUrl       *string
			fileType      *string
			fileMime      *string
			fileSize      *int64
			ownerId       *uuid.UUID
			ownerName     *string
			liked         *int32
			totalLikes    *int32
			totalDislikes *int32
			views         *int32
		)

		err = rows.Scan(&id, &name, &description, &mapName, &gameMode, &public,
			&fileId, &fileUrl, &fileType, &fileMime, &fileSize, &ownerId, &ownerName, &liked, &totalLikes, &totalDislikes, &views)
		if err != nil {
			return nil, -1, err
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

			if fileUrl != nil {
				file.Url = *fileUrl
			}

			if fileSize != nil {
				file.Size = fileSize
			}
		}

		if i := findWorld(entities, id.UUID); i >= 0 {
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

			var e World
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if description != nil {
				e.Description = *description
			}
			if gameMode != nil {
				e.GameMode = *gameMode
			}
			if mapName != nil {
				e.Map = *mapName
			}
			if file != nil {
				e.Files = append(e.Files, *file)
			}

			e.Owner = new(User)
			if ownerId != nil {
				e.Owner.Id = ownerId
			}

			if ownerName != nil {
				e.Owner.Name = ownerName
			}

			if liked != nil {
				e.Liked = liked
			}

			if totalLikes != nil {
				e.TotalLikes = totalLikes
			} else {
				e.TotalLikes = new(int32)
				*e.TotalLikes = 0
			}

			if totalDislikes != nil {
				e.TotalDislikes = totalDislikes
			} else {
				e.TotalDislikes = new(int32)
				*e.TotalDislikes = 0
			}

			e.Views = views

			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexWorldsForAdminForPackageWithQueryWithPak Index packages for admin with query and pak file
func IndexWorldsForAdminForPackageWithQueryWithPak(ctx context.Context, requester *sm.User, packageId uuid.UUID, offset int64, limit int64, query string, platform string, deployment string) (entities []World, total int64, err error) {
	db := database.DB

	q := `SELECT COUNT(*)
FROM spaces w
	LEFT JOIN mods m on w.mod_id = m.id
WHERE w.mod_id = $1 AND (w.name ILIKE $2::text OR m.name ILIKE $2::text)`

	row := db.QueryRow(ctx, q, packageId /*$1*/, query /*$2*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}

	q = `SELECT 
	w.id                    worldId,
	w.name                  worldName,
	w.description           worldDescription,
	w.map                   worldMap,
	w.game_mode             worldGameMode,
	m.id					modId,
	m.name					modName,
	m.title					modTitle,
	e.public                entityPublic,
	pak.id                  pakId,
	pak.url                 pakUrl,
	pak.type                pakType,
	pak.mime            	pakMime,
	pak.size				pakSize,
	pak.original_path		pakOriginalPath,
	pak.hash				pakHash,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size			previewSize,
	preview.original_path	previewOriginalPath,
	preview.hash			previewHash,
	owner.id 				ownerId,
	owner.name 				ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM spaces w
    LEFT JOIN entities e ON w.id = e.id
	LEFT JOIN likables l ON l.entity_id = e.id
    LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
    LEFT JOIN accessibles a ON a.entity_id = e.id
	LEFT JOIN users owner ON owner.id = a.user_id
	LEFT JOIN mods m ON w.mod_id = m.id
	LEFT JOIN files pak ON pak.entity_id = m.id AND ((pak.platform = $2::text AND pak.deployment_type = $3::text) OR (pak.platform = '' AND pak.deployment_type = ''))
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
WHERE w.mod_id = $4 AND (w.name ILIKE $5::text OR m.name ILIKE $5::text)
GROUP BY e.id, m.id, e.public, pak.id, pak.url, pak.type, pak.mime, pak.size, pak.original_path, pak.hash, preview.id, preview.url, preview.type, preview.mime, preview.size, preview.original_path, preview.hash, owner.id, l2.value, w.id, e.updated_at, e.created_at, e.views
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, requester.Id /*$1*/, platform /*$2*/, deployment /*$3*/, packageId /*$4*/, query /*$5*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	var (
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexWorldsForAdminForPackageWithQueryWithPak")
	}()
	for rows.Next() {
		var (
			id               pgtypeuuid.UUID
			name             *string
			description      *string
			mapName          *string
			gameMode         *string
			modId            *uuid.UUID
			modName          *string
			modTitle         *string
			public           *bool
			pakId            pgtypeuuid.UUID
			pakUrl           *string
			pakType          *string
			pakMime          *string
			pakSize          *int64
			pakOriginalPath  *string
			pakHash          *string
			fileId           pgtypeuuid.UUID
			fileUrl          *string
			fileType         *string
			fileMime         *string
			fileSize         *int64
			fileOriginalPath *string
			fileHash         *string
			ownerId          *uuid.UUID
			ownerName        *string
			liked            *int32
			totalLikes       *int32
			totalDislikes    *int32
			views            *int32
		)

		err = rows.Scan(&id, &name, &description, &mapName, &gameMode, &modId, &modName, &modTitle, &public,
			&pakId, &pakUrl, &pakType, &pakMime, &pakSize, &pakOriginalPath, &pakHash,
			&fileId, &fileUrl, &fileType, &fileMime, &fileSize, &fileOriginalPath, &fileHash,
			&ownerId, &ownerName, &liked, &totalLikes, &totalDislikes, &views)
		if err != nil {
			return nil, -1, err
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

			if fileUrl != nil {
				file.Url = *fileUrl
			}

			if fileSize != nil {
				file.Size = fileSize
			}

			if fileOriginalPath != nil && *fileOriginalPath != "" {
				file.OriginalPath = fileOriginalPath
			}

			if fileHash != nil && *fileHash != "" {
				file.Hash = fileHash
			}
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

			if pakUrl != nil {
				pak.Url = *pakUrl
			}

			if pakSize != nil {
				pak.Size = pakSize
			}

			if pakOriginalPath != nil && *pakOriginalPath != "" {
				pak.OriginalPath = pakOriginalPath
			}

			if pakHash != nil && *pakHash != "" {
				pak.Hash = pakHash
			}
		}

		if i := findWorld(entities, id.UUID); i >= 0 {
			if file != nil && !containsFile(entities[i].Files, *file.Id) {
				entities[i].Files = append(entities[i].Files, *file)
			}

			if pak != nil && !containsFile(entities[i].Package.Files, *pak.Id) {
				entities[i].Package.Files = append(entities[i].Package.Files, *pak)
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

			var e World
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if description != nil {
				e.Description = *description
			}
			if gameMode != nil {
				e.GameMode = *gameMode
			}
			if mapName != nil {
				e.Map = *mapName
			}
			if file != nil {
				e.Files = append(e.Files, *file)
			}

			if e.Package == nil && modId != nil {
				e.Package = new(Package)
				e.Package.Id = modId

				if modName != nil {
					e.Package.Name = *modName
				}

				if modTitle != nil {
					e.Package.Title = *modTitle
				}

				if pak != nil {
					e.Package.Files = append(e.Files, *pak)
				}
			}

			e.Owner = new(User)
			if ownerId != nil {
				e.Owner.Id = ownerId
			}

			if ownerName != nil {
				e.Owner.Name = ownerName
			}

			if liked != nil {
				e.Liked = liked
			}

			if totalLikes != nil {
				e.TotalLikes = totalLikes
			} else {
				e.TotalLikes = new(int32)
				*e.TotalLikes = 0
			}

			if totalDislikes != nil {
				e.TotalDislikes = totalDislikes
			} else {
				e.TotalDislikes = new(int32)
				*e.TotalDislikes = 0
			}

			e.Views = views

			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexWorldsForRequester Index packages for requester
func IndexWorldsForRequester(ctx context.Context, requester *User, offset int64, limit int64) (entities []World, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*)
FROM spaces w
    LEFT JOIN entities e ON e.id = w.id
	LEFT JOIN accessibles a ON e.id = a.entity_id
WHERE (e.public OR a.can_view OR a.is_owner)`

	row := db.QueryRow(ctx, q)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	q = `SELECT 
	w.id                    worldId,
	w.name                  worldName,
	w.description           worldDescription,
	w.map                   worldMap,
	w.game_mode             worldGameMode,
	e.public                entityPublic,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size			previewSize,
	owner.name 				ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM spaces w
    LEFT JOIN entities e ON w.id = e.id
	LEFT JOIN likables l ON l.entity_id = e.id
    LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
	LEFT JOIN mods m ON w.mod_id = m.id
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
	LEFT JOIN accessibles a ON e.id = a.entity_id
	LEFT JOIN users owner ON owner.id = a.user_id
WHERE (e.public OR a.can_view OR a.is_owner)
GROUP BY e.id, w.id, e.public, preview.id, preview.url, preview.type, preview.mime, preview.size, owner.name, l2.value, e.updated_at, e.created_at, e.views
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows      pgx.Rows
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, requester.Id)

	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexWorldsForRequester")
	}()
	for rows.Next() {
		var (
			id            pgtypeuuid.UUID
			name          *string
			description   *string
			mapName       *string
			gameMode      *string
			public        *bool
			fileId        pgtypeuuid.UUID
			fileUrl       *string
			fileType      *string
			fileMime      *string
			fileSize      *int64
			ownerName     *string
			liked         *int32
			totalLikes    *int32
			totalDislikes *int32
			views         *int32
		)

		err = rows.Scan(
			&id,
			&name,
			&description,
			&mapName,
			&gameMode,
			&public,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileSize,
			&ownerName,
			&liked,
			&totalLikes,
			&totalDislikes,
			&views,
		)
		if err != nil {
			return nil, -1, err
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

			if fileUrl != nil {
				file.Url = *fileUrl
			}

			if fileSize != nil {
				file.Size = fileSize
			}
		}

		if i := findWorld(entities, id.UUID); i >= 0 {
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

			var e World
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if description != nil {
				e.Description = *description
			}
			if gameMode != nil {
				e.GameMode = *gameMode
			}
			if mapName != nil {
				e.Map = *mapName
			}
			if file != nil {
				e.Files = append(e.Files, *file)
			}

			e.Owner = new(User)
			if ownerName != nil {
				e.Owner.Name = ownerName
			}

			if liked != nil {
				e.Liked = liked
			}

			if totalLikes != nil {
				e.TotalLikes = totalLikes
			} else {
				e.TotalLikes = new(int32)
				*e.TotalLikes = 0
			}

			if totalDislikes != nil {
				e.TotalDislikes = totalDislikes
			} else {
				e.TotalDislikes = new(int32)
				*e.TotalDislikes = 0
			}

			e.Views = views

			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexWorldsForRequesterWithPak Index packages for requester with pak file
func IndexWorldsForRequesterWithPak(ctx context.Context, requester *sm.User, offset int64, limit int64, platform string, deployment string) (entities []World, total int64, err error) {
	db := database.DB

	q := `SELECT COUNT(*)
FROM spaces w
    LEFT JOIN entities e ON e.id = w.id
    LEFT JOIN accessibles a ON e.id = a.entity_id
WHERE (e.public OR a.can_view OR a.is_owner)`

	row := db.QueryRow(ctx, q)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}

	q = `SELECT 
	w.id                    worldId,
	w.name                  worldName,
	w.description           worldDescription,
	w.map                   worldMap,
	w.game_mode             worldGameMode,
	m.id					modId,
	m.name					modName,
	m.title					modTitle,
	e.public                entityPublic,
	pak.id                  pakId,
	pak.url                 pakUrl,
	pak.type                pakType,
	pak.mime            	pakMime,
	pak.size				pakSize,
	pak.original_path		pakOriginalPath,
	pak.hash				pakHash,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size			previewSize,
	preview.original_path	previewOriginalPath,
	preview.hash			previewHash,
	owner.name 				ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM spaces w
    LEFT JOIN entities e ON w.id = e.id
	LEFT JOIN mods m ON w.mod_id = m.id
	LEFT JOIN likables l ON l.entity_id = e.id
    LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
	LEFT JOIN files pak ON pak.entity_id = m.id AND ((pak.platform = $2::text AND pak.deployment_type = $3::text) OR (pak.platform = '' AND pak.deployment_type = ''))
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
	LEFT JOIN accessibles a ON e.id = a.entity_id
	LEFT JOIN users owner ON owner.id = a.user_id
WHERE (e.public OR a.can_view OR a.is_owner)
GROUP BY e.id, w.id, m.id, e.public, pak.id, pak.url, pak.type, pak.mime, pak.size, pak.original_path, pak.hash, preview.id, preview.url, preview.type, preview.mime, preview.size, preview.original_path, preview.hash, owner.name, l2.value, e.updated_at, e.created_at, e.views
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, requester.Id, platform /*$1*/, deployment /*$2*/)

	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	var (
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexWorldsForRequesterWithPak")
	}()
	for rows.Next() {
		var (
			id               pgtypeuuid.UUID
			name             *string
			description      *string
			mapName          *string
			gameMode         *string
			modId            *uuid.UUID
			modName          *string
			modTitle         *string
			public           *bool
			pakId            pgtypeuuid.UUID
			pakUrl           *string
			pakType          *string
			pakMime          *string
			pakSize          *int64
			pakOriginalPath  *string
			pakHash          *string
			fileId           pgtypeuuid.UUID
			fileUrl          *string
			fileType         *string
			fileMime         *string
			fileSize         *int64
			fileOriginalPath *string
			fileHash         *string
			ownerName        *string
			liked            *int32
			totalLikes       *int32
			totalDislikes    *int32
			views            *int32
		)

		err = rows.Scan(&id, &name, &description, &mapName, &gameMode, &modId, &modName, &modTitle, &public,
			&pakId, &pakUrl, &pakType, &pakMime, &pakSize, &pakOriginalPath, &pakHash,
			&fileId, &fileUrl, &fileType, &fileMime, &fileSize, &fileOriginalPath, &fileHash,
			&ownerName, &liked, &totalLikes, &totalDislikes, &views)
		if err != nil {
			return nil, -1, err
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

			if fileUrl != nil {
				file.Url = *fileUrl
			}

			if fileSize != nil {
				file.Size = fileSize
			}

			if fileOriginalPath != nil && *fileOriginalPath != "" {
				file.OriginalPath = fileOriginalPath
			}

			if fileHash != nil && *fileHash != "" {
				file.Hash = fileHash
			}
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

			if pakUrl != nil {
				pak.Url = *pakUrl
			}

			if pakSize != nil {
				pak.Size = pakSize
			}

			if pakOriginalPath != nil && *pakOriginalPath != "" {
				pak.OriginalPath = pakOriginalPath
			}

			if pakHash != nil && *pakHash != "" {
				pak.Hash = pakHash
			}
		}

		if i := findWorld(entities, id.UUID); i >= 0 {
			if file != nil && !containsFile(entities[i].Files, *file.Id) {
				entities[i].Files = append(entities[i].Files, *file)
			}

			if pak != nil && !containsFile(entities[i].Package.Files, *pak.Id) {
				entities[i].Package.Files = append(entities[i].Package.Files, *pak)
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

			var e World
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if description != nil {
				e.Description = *description
			}
			if gameMode != nil {
				e.GameMode = *gameMode
			}
			if mapName != nil {
				e.Map = *mapName
			}
			if file != nil {
				e.Files = append(e.Files, *file)
			}

			if e.Package == nil && modId != nil {
				e.Package = new(Package)
				e.Package.Id = modId

				if modName != nil {
					e.Package.Name = *modName
				}

				if modTitle != nil {
					e.Package.Title = *modTitle
				}

				if pak != nil {
					e.Package.Files = append(e.Files, *pak)
				}
			}

			e.Owner = new(User)
			if ownerName != nil {
				e.Owner.Name = ownerName
			}

			if liked != nil {
				e.Liked = liked
			}

			if totalLikes != nil {
				e.TotalLikes = totalLikes
			} else {
				e.TotalLikes = new(int32)
				*e.TotalLikes = 0
			}

			if totalDislikes != nil {
				e.TotalDislikes = totalDislikes
			} else {
				e.TotalDislikes = new(int32)
				*e.TotalDislikes = 0
			}

			e.Views = views

			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexWorldsForRequesterForPackage Index packages for requester
func IndexWorldsForRequesterForPackage(ctx context.Context, requester *User, packageId uuid.UUID, offset int64, limit int64) (entities []World, total int64, err error) {
	db := database.DB

	q := `SELECT COUNT(*)
FROM spaces w
    LEFT JOIN entities e ON e.id = w.id
    LEFT JOIN accessibles a ON e.id = a.entity_id  AND a.user_id = $1::uuid
WHERE w.mod_id = $2 AND (e.public OR a.can_view OR a.is_owner)`

	row := db.QueryRow(ctx, q, requester.Id /*$1*/, packageId /*$2*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}

	q = `SELECT 
	w.id                    worldId,
	w.name                  worldName,
	w.description           worldDescription,
	w.map                   worldMap,
	w.game_mode             worldGameMode,
	e.public                entityPublic,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size			previewSize,
	owner.name 				ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM spaces w
    LEFT JOIN entities e ON w.id = e.id
	LEFT JOIN likables l ON l.entity_id = e.id
    LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
	LEFT JOIN mods m ON w.mod_id = m.id
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
	LEFT JOIN accessibles a on e.id = a.entity_id
	LEFT JOIN users owner ON owner.id = a.user_id
WHERE w.mod_id = $2
GROUP BY e.id, w.id, e.public, preview.id, preview.url, preview.type, preview.mime, preview.size, owner.name, l2.value, e.updated_at, e.created_at, e.views
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, requester.Id, packageId /*$1*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	var (
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexWorldsForRequesterForPackage")
	}()
	for rows.Next() {
		var (
			id            pgtypeuuid.UUID
			name          *string
			description   *string
			mapName       *string
			gameMode      *string
			public        *bool
			fileId        pgtypeuuid.UUID
			fileUrl       *string
			fileType      *string
			fileMime      *string
			fileSize      *int64
			ownerName     *string
			liked         *int32
			totalLikes    *int32
			totalDislikes *int32
			views         *int32
		)

		err = rows.Scan(&id, &name, &description, &mapName, &gameMode, &public,
			&fileId, &fileUrl, &fileType, &fileMime, &fileSize, &ownerName, &liked, &totalLikes, &totalDislikes, &views)
		if err != nil {
			return nil, -1, err
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

			if fileUrl != nil {
				file.Url = *fileUrl
			}

			if fileSize != nil {
				file.Size = fileSize
			}
		}

		if i := findWorld(entities, id.UUID); i >= 0 {
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

			var e World
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if description != nil {
				e.Description = *description
			}
			if gameMode != nil {
				e.GameMode = *gameMode
			}
			if mapName != nil {
				e.Map = *mapName
			}
			if file != nil {
				e.Files = append(e.Files, *file)
			}

			e.Owner = new(User)
			if ownerName != nil {
				e.Owner.Name = ownerName
			}

			if liked != nil {
				e.Liked = liked
			}

			if totalLikes != nil {
				e.TotalLikes = totalLikes
			} else {
				e.TotalLikes = new(int32)
				*e.TotalLikes = 0
			}

			if totalDislikes != nil {
				e.TotalDislikes = totalDislikes
			} else {
				e.TotalDislikes = new(int32)
				*e.TotalDislikes = 0
			}

			e.Views = views

			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexWorldsForRequesterForPackageWithPak Index packages for requester with pak file
func IndexWorldsForRequesterForPackageWithPak(ctx context.Context, requester *sm.User, packageId uuid.UUID, offset int64, limit int64, platform string, deployment string) (entities []World, total int64, err error) {
	db := database.DB

	q := `SELECT COUNT(*)
FROM spaces w
    LEFT JOIN entities e ON e.id = w.id
	LEFT JOIN accessibles a ON e.id = a.entity_id
WHERE w.mod_id = $1 AND (e.public OR a.can_view OR a.is_owner)`

	row := db.QueryRow(ctx, q, packageId /*$1*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}

	q = `SELECT 
	w.id                    worldId,
	w.name                  worldName,
	w.description           worldDescription,
	w.map                   worldMap,
	w.game_mode             worldGameMode,
	m.id					modId,
	m.name					modName,
	m.title					modTitle,
	e.public                entityPublic,
	pak.id                  pakId,
	pak.url                 pakUrl,
	pak.type                pakType,
	pak.mime            	pakMime,
	pak.size     			int64,
	pak.original_path		pakOriginalPath,
	pak.hash				pakHash,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size			previewSize,
	preview.original_path	previewOriginalPath,
	preview.hash			previewHash,	
	owner.name 				ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM spaces w
    LEFT JOIN entities e ON w.id = e.id
	LEFT JOIN likables l ON l.entity_id = e.id
    LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
	LEFT JOIN mods m ON w.mod_id = m.id
	LEFT JOIN files pak ON pak.entity_id = m.id AND ((pak.platform = $2::text AND pak.deployment_type = $3::text) OR (pak.platform = '' AND pak.deployment_type = ''))
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
	LEFT JOIN accessibles a ON e.id = a.entity_id
	LEFT JOIN users owner ON owner.id = a.user_id
WHERE w.mod_id = $4 AND (e.public OR a.can_view OR a.is_owner)
GROUP BY e.id, w.id, m.id, e.public, pak.id, pak.url, pak.type, pak.mime, pak.size, pak.original_path, pak.hash, preview.id, preview.url, preview.type, preview.mime, preview.size, preview.original_path, preview.hash, owner.name, l2.value, e.updated_at, e.created_at, e.views
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, requester.Id, platform /*$1*/, deployment /*$2*/, packageId /*$3*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	var (
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexWorldsForRequesterForPackageWithPak")
	}()
	for rows.Next() {
		var (
			id               pgtypeuuid.UUID
			name             *string
			description      *string
			mapName          *string
			gameMode         *string
			modId            *uuid.UUID
			modName          *string
			modTitle         *string
			public           *bool
			pakId            pgtypeuuid.UUID
			pakUrl           *string
			pakType          *string
			pakMime          *string
			pakSize          *int64
			pakOriginalPath  *string
			pakHash          *string
			fileId           pgtypeuuid.UUID
			fileUrl          *string
			fileType         *string
			fileMime         *string
			fileSize         *int64
			fileOriginalPath *string
			fileHash         *string
			ownerName        *string
			liked            *int32
			totalLikes       *int32
			totalDislikes    *int32
			views            *int32
		)

		err = rows.Scan(&id, &name, &description, &mapName, &gameMode, &modId, &modName, &modTitle, &public,
			&pakId, &pakUrl, &pakType, &pakMime, &pakSize, &pakOriginalPath, &pakHash,
			&fileId, &fileUrl, &fileType, &fileMime, &fileSize, &fileOriginalPath, &fileHash,
			&ownerName, &liked, &totalLikes, &totalDislikes, &views)
		if err != nil {
			return nil, -1, err
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

			if fileUrl != nil {
				file.Url = *fileUrl
			}

			if fileSize != nil {
				file.Size = fileSize
			}

			if fileOriginalPath != nil && *fileOriginalPath != "" {
				file.OriginalPath = fileOriginalPath
			}

			if fileHash != nil && *fileHash != "" {
				file.Hash = fileHash
			}
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

			if pakUrl != nil {
				pak.Url = *pakUrl
			}

			if pakSize != nil {
				pak.Size = pakSize
			}

			if pakOriginalPath != nil && *pakOriginalPath != "" {
				pak.OriginalPath = pakOriginalPath
			}

			if pakHash != nil && *pakHash != "" {
				pak.Hash = pakHash
			}
		}

		if i := findWorld(entities, id.UUID); i >= 0 {
			if file != nil && !containsFile(entities[i].Files, *file.Id) {
				entities[i].Files = append(entities[i].Files, *file)
			}

			if pak != nil && !containsFile(entities[i].Package.Files, *pak.Id) {
				entities[i].Package.Files = append(entities[i].Package.Files, *pak)
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

			var e World
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if description != nil {
				e.Description = *description
			}
			if gameMode != nil {
				e.GameMode = *gameMode
			}
			if mapName != nil {
				e.Map = *mapName
			}
			if file != nil {
				e.Files = append(e.Files, *file)
			}

			if e.Package == nil && modId != nil {
				e.Package = new(Package)
				e.Package.Id = modId

				if modName != nil {
					e.Package.Name = *modName
				}

				if modTitle != nil {
					e.Package.Title = *modTitle
				}

				if pak != nil {
					e.Package.Files = append(e.Files, *pak)
				}
			}

			e.Owner = new(User)
			if ownerName != nil {
				e.Owner.Name = ownerName
			}

			if liked != nil {
				e.Liked = liked
			}

			if totalLikes != nil {
				e.TotalLikes = totalLikes
			} else {
				e.TotalLikes = new(int32)
				*e.TotalLikes = 0
			}

			if totalDislikes != nil {
				e.TotalDislikes = totalDislikes
			} else {
				e.TotalDislikes = new(int32)
				*e.TotalDislikes = 0
			}

			e.Views = views

			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexWorldsForRequesterWithQuery Index packages for requester with query and pak file
func IndexWorldsForRequesterWithQuery(ctx context.Context, requester *User, offset int64, limit int64, query string) (entities []World, total int64, err error) {
	db := database.DB

	q := `SELECT COUNT(*)
FROM spaces w
    LEFT JOIN mods m ON w.mod_id = m.id
	LEFT JOIN entities e on w.id = e.id
	LEFT JOIN accessibles a on e.id = a.entity_id
WHERE (w.name ILIKE $1::text OR m.name ILIKE $1::text) AND (e.public OR a.can_view OR a.is_owner)`

	row := db.QueryRow(ctx, q, query /*$1*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}

	q = `SELECT 
	w.id                    worldId,
	w.name                  worldName,
	w.description           worldDescription,
	w.map                   worldMap,
	w.game_mode             worldGameMode,
	e.public                entityPublic,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size			previewSize,
	owner.name 				ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM spaces w
    LEFT JOIN entities e ON w.id = e.id
	LEFT JOIN likables l ON l.entity_id = e.id
    LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
	LEFT JOIN mods m ON w.mod_id = m.id
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
	LEFT JOIN accessibles a ON e.id = a.entity_id
	LEFT JOIN users owner ON owner.id = a.user_id
WHERE (w.name ILIKE $2::text OR m.name ILIKE $2::text) AND (e.public OR a.can_view OR a.is_owner)
GROUP BY e.id, w.id, e.public, preview.id, preview.url, preview.type, preview.mime, preview.size, owner.name, l2.value, e.updated_at, e.created_at, e.views
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, requester.Id, query /*$1*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	var (
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexWorldsForRequesterWithQuery")
	}()
	for rows.Next() {
		var (
			id            pgtypeuuid.UUID
			name          *string
			description   *string
			mapName       *string
			gameMode      *string
			public        *bool
			fileId        pgtypeuuid.UUID
			fileUrl       *string
			fileType      *string
			fileMime      *string
			fileSize      *int64
			ownerName     *string
			liked         *int32
			totalLikes    *int32
			totalDislikes *int32
			views         *int32
		)

		err = rows.Scan(&id, &name, &description, &mapName, &gameMode, &public,
			&fileId, &fileUrl, &fileType, &fileMime, &fileSize, &ownerName, &liked, &totalLikes, &totalDislikes, &views)
		if err != nil {
			return nil, -1, err
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

			if fileUrl != nil {
				file.Url = *fileUrl
			}

			if fileSize != nil {
				file.Size = fileSize
			}
		}

		if i := findWorld(entities, id.UUID); i >= 0 {
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

			var e World
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if description != nil {
				e.Description = *description
			}
			if gameMode != nil {
				e.GameMode = *gameMode
			}
			if mapName != nil {
				e.Map = *mapName
			}
			if file != nil {
				e.Files = append(e.Files, *file)
			}

			e.Owner = new(User)
			if ownerName != nil {
				e.Owner.Name = ownerName
			}

			if liked != nil {
				e.Liked = liked
			}

			if totalLikes != nil {
				e.TotalLikes = totalLikes
			} else {
				e.TotalLikes = new(int32)
				*e.TotalLikes = 0
			}

			if totalDislikes != nil {
				e.TotalDislikes = totalDislikes
			} else {
				e.TotalDislikes = new(int32)
				*e.TotalDislikes = 0
			}

			e.Views = views

			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexWorldsForRequesterWithQueryWithPak Index packages for requester with query and pak file
func IndexWorldsForRequesterWithQueryWithPak(ctx context.Context, requester *sm.User, offset int64, limit int64, query string, platform string, deployment string) (entities []World, total int64, err error) {
	db := database.DB

	q := `SELECT COUNT(*)
FROM spaces w
    LEFT JOIN mods m ON w.mod_id = m.id
   	LEFT JOIN entities e on w.id = e.id
	LEFT JOIN accessibles a on e.id = a.entity_id
WHERE (w.name ILIKE $1::text OR m.name ILIKE $1::text) AND (e.public OR a.can_view OR a.is_owner)`

	row := db.QueryRow(ctx, q, query /*$1*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}

	q = `SELECT 
	w.id                    worldId,
	w.name                  worldName,
	w.description           worldDescription,
	w.map                   worldMap,
	w.game_mode             worldGameMode,
	m.id					modId,
	m.name					modName,
	m.title					modTitle,
	e.public                entityPublic,
	pak.id                  pakId,
	pak.url                 pakUrl,
	pak.type                pakType,
	pak.mime            	pakMime,
	pak.size				pakSize,
	pak.original_path		pakOriginalPath,
	pak.hash				pakHash,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size			previewSize,
	preview.original_path	previewOriginalPath,
	preview.hash			previewHash,
	owner.name 				ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM spaces w
    LEFT JOIN entities e ON w.id = e.id
	LEFT JOIN likables l ON l.entity_id = e.id
    LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
	LEFT JOIN mods m ON w.mod_id = m.id
	LEFT JOIN files pak ON pak.entity_id = m.id AND ((pak.platform = $2::text AND pak.deployment_type = $3::text) OR (pak.platform = '' AND pak.deployment_type = ''))
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
	LEFT JOIN accessibles a ON e.id = a.entity_id
	LEFT JOIN users owner ON owner.id = a.user_id
WHERE (w.name ILIKE $4::text OR m.name ILIKE $4::text)
GROUP BY e.id, w.id, m.id, e.public, pak.id, pak.url, pak.type, pak.mime, pak.size, pak.original_path, pak.hash, preview.id, preview.url, preview.type, preview.mime, preview.size, preview.original_path, preview.hash, owner.name, l2.value, e.updated_at, e.created_at, e.views
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, requester.Id, platform /*$1*/, deployment /*$2*/, query /*$3*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	var (
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexWorldsForRequesterWithQueryWithPak")
	}()
	for rows.Next() {
		var (
			id               pgtypeuuid.UUID
			name             *string
			description      *string
			mapName          *string
			gameMode         *string
			modId            *uuid.UUID
			modName          *string
			modTitle         *string
			public           *bool
			pakId            pgtypeuuid.UUID
			pakUrl           *string
			pakType          *string
			pakMime          *string
			pakSize          *int64
			pakOriginalPath  *string
			pakHash          *string
			fileId           pgtypeuuid.UUID
			fileUrl          *string
			fileType         *string
			fileMime         *string
			fileSize         *int64
			fileOriginalPath *string
			fileHash         *string
			ownerName        *string
			liked            *int32
			totalLikes       *int32
			totalDislikes    *int32
			views            *int32
		)

		err = rows.Scan(&id, &name, &description, &mapName, &gameMode, &modId, &modName, &modTitle, &public,
			&pakId, &pakUrl, &pakType, &pakMime, &pakSize, &pakOriginalPath, &pakHash,
			&fileId, &fileUrl, &fileType, &fileMime, &fileSize, &fileOriginalPath, &fileHash,
			&ownerName, &liked, &totalLikes, &totalDislikes, &views)
		if err != nil {
			return nil, -1, err
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

			if fileUrl != nil {
				file.Url = *fileUrl
			}

			if fileSize != nil {
				file.Size = fileSize
			}

			if fileOriginalPath != nil && *fileOriginalPath != "" {
				file.OriginalPath = fileOriginalPath
			}

			if fileHash != nil && *fileHash != "" {
				file.Hash = fileHash
			}
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

			if pakUrl != nil {
				pak.Url = *pakUrl
			}

			if pakSize != nil {
				pak.Size = pakSize
			}

			if pakOriginalPath != nil && *pakOriginalPath != "" {
				pak.OriginalPath = pakOriginalPath
			}

			if pakHash != nil && *pakHash != "" {
				pak.Hash = pakHash
			}
		}

		if i := findWorld(entities, id.UUID); i >= 0 {
			if file != nil && !containsFile(entities[i].Files, *file.Id) {
				entities[i].Files = append(entities[i].Files, *file)
			}

			if pak != nil && !containsFile(entities[i].Package.Files, *pak.Id) {
				entities[i].Package.Files = append(entities[i].Package.Files, *pak)
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

			var e World
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if description != nil {
				e.Description = *description
			}
			if gameMode != nil {
				e.GameMode = *gameMode
			}
			if mapName != nil {
				e.Map = *mapName
			}
			if file != nil {
				e.Files = append(e.Files, *file)
			}

			if e.Package == nil && modId != nil {
				e.Package = new(Package)
				e.Package.Id = modId

				if modName != nil {
					e.Package.Name = *modName
				}

				if modTitle != nil {
					e.Package.Title = *modTitle
				}

				if pak != nil {
					e.Package.Files = append(e.Files, *pak)
				}
			}

			e.Owner = new(User)
			if ownerName != nil {
				e.Owner.Name = ownerName
			}

			if liked != nil {
				e.Liked = liked
			}

			if totalLikes != nil {
				e.TotalLikes = totalLikes
			} else {
				e.TotalLikes = new(int32)
				*e.TotalLikes = 0
			}

			if totalDislikes != nil {
				e.TotalDislikes = totalDislikes
			} else {
				e.TotalDislikes = new(int32)
				*e.TotalDislikes = 0
			}

			e.Views = views

			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexWorldsForRequesterForPackageWithQuery Index packages for requester with query and pak file
func IndexWorldsForRequesterForPackageWithQuery(ctx context.Context, requester *User, packageId uuid.UUID, offset int64, limit int64, query string) (entities []World, total int64, err error) {
	db := database.DB

	q := `SELECT COUNT(*)
FROM spaces w
    LEFT JOIN mods m ON w.mod_id = m.id
	LEFT JOIN entities e on w.id = e.id
	LEFT JOIN accessibles a on e.id = a.entity_id
WHERE w.mod_id = $1 AND (w.name ILIKE $2::text OR m.name ILIKE $2::text) AND (e.public OR a.can_view OR a.is_owner)`

	row := db.QueryRow(ctx, q, packageId /*$1*/, query /*$2*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}

	q = `SELECT 
	w.id                    worldId,
	w.name                  worldName,
	w.description           worldDescription,
	w.map                   worldMap,
	w.game_mode             worldGameMode,
	e.public                entityPublic,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size			previewSize,
	owner.name 				ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM spaces w
    LEFT JOIN entities e ON w.id = e.id
	LEFT JOIN likables l ON l.entity_id = e.id
    LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
	LEFT JOIN mods m ON w.mod_id = m.id
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
	LEFT JOIN accessibles a ON e.id = a.entity_id
	LEFT JOIN users owner ON owner.id = a.user_id
WHERE w.mod_id = $2 AND (w.name ILIKE $3::text OR m.name ILIKE $3::text)
GROUP BY e.id, w.id, e.public, preview.id, preview.url, preview.type, preview.mime, preview.size, owner.name, l2.value, e.updated_at, e.created_at, e.views
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, requester.Id, packageId /*$1*/, query /*$2*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	var (
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexWorldsForRequesterForPackageWithQuery")
	}()
	for rows.Next() {
		var (
			id            pgtypeuuid.UUID
			name          *string
			description   *string
			mapName       *string
			gameMode      *string
			public        *bool
			fileId        pgtypeuuid.UUID
			fileUrl       *string
			fileType      *string
			fileMime      *string
			fileSize      *int64
			ownerName     *string
			liked         *int32
			totalLikes    *int32
			totalDislikes *int32
			views         *int32
		)

		err = rows.Scan(&id, &name, &description, &mapName, &gameMode, &public,
			&fileId, &fileUrl, &fileType, &fileMime, &fileSize, &ownerName, &liked, &totalLikes, &totalDislikes, &views)
		if err != nil {
			return nil, -1, err
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

			if fileUrl != nil {
				file.Url = *fileUrl
			}

			if fileSize != nil {
				file.Size = fileSize
			}
		}

		if i := findWorld(entities, id.UUID); i >= 0 {
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

			var e World
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if description != nil {
				e.Description = *description
			}
			if gameMode != nil {
				e.GameMode = *gameMode
			}
			if mapName != nil {
				e.Map = *mapName
			}
			if file != nil {
				e.Files = append(e.Files, *file)
			}

			e.Owner = new(User)
			if ownerName != nil {
				e.Owner.Name = ownerName
			}

			if liked != nil {
				e.Liked = liked
			}

			if totalLikes != nil {
				e.TotalLikes = totalLikes
			} else {
				e.TotalLikes = new(int32)
				*e.TotalLikes = 0
			}

			if totalDislikes != nil {
				e.TotalDislikes = totalDislikes
			} else {
				e.TotalDislikes = new(int32)
				*e.TotalDislikes = 0
			}

			e.Views = views

			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexWorldsForRequesterForPackageWithQueryWithPak Index packages for requester with query and pak file
func IndexWorldsForRequesterForPackageWithQueryWithPak(ctx context.Context, requester *sm.User, packageId uuid.UUID, offset int64, limit int64, query string, platform string, deployment string) (entities []World, total int64, err error) {
	db := database.DB

	q := `SELECT COUNT(*)
FROM spaces w
    LEFT JOIN mods m ON w.mod_id = m.id
	LEFT JOIN entities e on w.id = e.id
	LEFT JOIN accessibles a on e.id = a.entity_id
WHERE w.mod_id = $1 AND (w.name ILIKE $2::text OR m.name ILIKE $2::text) AND (e.public OR a.can_view OR a.is_owner)`

	row := db.QueryRow(ctx, q, packageId /*$1*/, query /*$2*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}

	q = `SELECT 
	w.id                    worldId,
	w.name                  worldName,
	w.description           worldDescription,
	w.map                   worldMap,
	w.game_mode             worldGameMode,
	m.id					modId,
	m.name					modName,
	m.title					modTitle,
	e.public                entityPublic,
	pak.id                  pakId,
	pak.url                 pakUrl,
	pak.type                pakType,
	pak.mime            	pakMime,
	pak.size				pakSize,
	pak.original_path		pakOriginalPath,
	pak.hash				pakHash,
	preview.id              previewId,
	preview.url             previewUrl,
	preview.type            previewType,
	preview.mime        	previewMime,
	preview.size			previewSize,
	preview.original_path	previewOriginalPath,
	preview.hash			previewHash,
	owner.name 				ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM spaces w
    LEFT JOIN entities e ON w.id = e.id
	LEFT JOIN likables l ON l.entity_id = e.id
    LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
	LEFT JOIN mods m ON w.mod_id = m.id
	LEFT JOIN files pak ON pak.entity_id = m.id AND ((pak.platform = $2::text AND pak.deployment_type = $3::text) OR (pak.platform = '' AND pak.deployment_type = ''))
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
	LEFT JOIN accessibles a ON e.id = a.entity_id
	LEFT JOIN users owner ON owner.id = a.user_id
WHERE w.mod_id = $4 AND (w.name ILIKE $5::text OR m.name ILIKE $5::text) AND (e.public OR a.can_view OR a.is_owner)
GROUP BY e.id, w.id, m.id, e.public, pak.id, pak.url, pak.type, pak.mime, pak.size, pak.original_path, pak.hash, preview.id, preview.url, preview.type, preview.mime, preview.size, preview.original_path, preview.hash, owner.name, l2.value, e.updated_at, e.created_at, e.views
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, requester.Id, platform /*$1*/, deployment /*$2*/, packageId /*$3*/, query /*$3*/)

	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	var (
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexWorldsForRequesterForPackageWithQueryWithPak")
	}()
	for rows.Next() {
		var (
			id               pgtypeuuid.UUID
			name             *string
			description      *string
			mapName          *string
			gameMode         *string
			modId            *uuid.UUID
			modName          *string
			modTitle         *string
			public           *bool
			pakId            pgtypeuuid.UUID
			pakUrl           *string
			pakType          *string
			pakMime          *string
			pakSize          *int64
			pakOriginalPath  *string
			pakHash          *string
			fileId           pgtypeuuid.UUID
			fileUrl          *string
			fileType         *string
			fileMime         *string
			fileSize         *int64
			fileOriginalPath *string
			fileHash         *string
			ownerName        *string
			liked            *int32
			totalLikes       *int32
			totalDislikes    *int32
			views            *int32
		)

		err = rows.Scan(&id, &name, &description, &mapName, &gameMode, &modId, &modName, &modTitle, &public,
			&pakId, &pakUrl, &pakType, &pakMime, &pakSize, &pakOriginalPath, &pakHash,
			&fileId, &fileUrl, &fileType, &fileMime, &fileSize, &fileOriginalPath, &fileHash,
			&ownerName, &liked, &totalLikes, &totalDislikes, &views)
		if err != nil {
			return nil, -1, err
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

			if fileUrl != nil {
				file.Url = *fileUrl
			}

			if fileSize != nil {
				file.Size = fileSize
			}

			if fileOriginalPath != nil && *fileOriginalPath != "" {
				file.OriginalPath = fileOriginalPath
			}

			if fileHash != nil && *fileHash != "" {
				file.Hash = fileHash
			}
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

			if pakUrl != nil {
				pak.Url = *pakUrl
			}

			if pakSize != nil {
				pak.Size = pakSize
			}

			if pakOriginalPath != nil && *pakOriginalPath != "" {
				pak.OriginalPath = pakOriginalPath
			}

			if pakHash != nil && *pakHash != "" {
				pak.Hash = pakHash
			}
		}

		if i := findWorld(entities, id.UUID); i >= 0 {
			if file != nil && !containsFile(entities[i].Files, *file.Id) {
				entities[i].Files = append(entities[i].Files, *file)
			}

			if pak != nil && !containsFile(entities[i].Package.Files, *pak.Id) {
				entities[i].Package.Files = append(entities[i].Package.Files, *pak)
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

			var e World
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if description != nil {
				e.Description = *description
			}
			if gameMode != nil {
				e.GameMode = *gameMode
			}
			if mapName != nil {
				e.Map = *mapName
			}
			if file != nil {
				e.Files = append(e.Files, *file)
			}

			if e.Package == nil && modId != nil {
				e.Package = new(Package)
				e.Package.Id = modId

				if modName != nil {
					e.Package.Name = *modName
				}

				if modTitle != nil {
					e.Package.Title = *modTitle
				}

				if pak != nil {
					e.Package.Files = append(e.Files, *pak)
				}
			}

			e.Owner = new(User)
			if ownerName != nil {
				e.Owner.Name = ownerName
			}

			if liked != nil {
				e.Liked = liked
			}

			if totalLikes != nil {
				e.TotalLikes = totalLikes
			} else {
				e.TotalLikes = new(int32)
				*e.TotalLikes = 0
			}

			if totalDislikes != nil {
				e.TotalDislikes = totalDislikes
			} else {
				e.TotalDislikes = new(int32)
				*e.TotalDislikes = 0
			}

			e.Views = views

			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// GetWorldForAdminWithPak Get world for admin
func GetWorldForAdminWithPak(ctx context.Context, requester *sm.User, id uuid.UUID, platform string, deployment string) (entity *World, err error) {
	db := database.DB

	q := `SELECT
	w.id                    worldId,
	w.name                  worldName,
	w.description           worldDescription,
	w.map                   worldMap,
	w.game_mode             worldGameMode,
	m.id					modId,
	m.name					modName,
	m.title					modTitle,
	e.public                entityPublic,
	pak.id                  pakId,
	pak.url                 pakUrl,
	pak.type                pakType,
	pak.mime            	pakMime,
	pak.size				pakSize,
	pak.original_path		pakOriginalPath,
	pak.hash				pakHash,
	f.id              previewId,
	f.url             previewUrl,
	f.type            previewType,
	f.mime        	  previewMime,
	f.size			  previewSize,
	f.original_path	previewOriginalPath,
	f.hash			previewHash,
	owner.id 			ownerId,
	owner.name 			ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM spaces w
	LEFT JOIN entities e on w.id = e.id
    LEFT JOIN likables l ON l.entity_id = e.id
    LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
	LEFT JOIN accessibles a ON a.entity_id = e.id
	LEFT JOIN users owner ON owner.id = a.user_id
	LEFT JOIN files f ON e.id = f.entity_id 
    LEFT JOIN mods m ON m.id = w.mod_id 
	LEFT JOIN entities me ON me.id = m.id
	LEFT JOIN files pak ON me.id = pak.entity_id AND ((pak.platform = $2::text AND pak.deployment_type = $3::text) OR (pak.platform = '' AND pak.deployment_type = ''))
WHERE w.id = $4
GROUP BY e.id, w.id, m.id, f.id, f.url, f.type, f.mime, f.size, f.original_path, f.hash, e.public, pak.id, pak.url, pak.type, pak.mime, pak.size, pak.original_path, pak.hash, owner.id, l2.value, e.views
ORDER BY e.id`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, requester.Id, platform /*$1*/, deployment /*$2*/, id /*$3*/)

	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetWorldForAdminWithPak")
	}()
	for rows.Next() {
		var (
			worldId          pgtypeuuid.UUID
			name             *string
			description      *string
			mapName          *string
			gameMode         *string
			modId            *uuid.UUID
			modName          *string
			modTitle         *string
			public           *bool
			pakId            pgtypeuuid.UUID
			pakUrl           *string
			pakType          *string
			pakMime          *string
			pakSize          *int64
			pakOriginalPath  *string
			pakHash          *string
			fileId           pgtypeuuid.UUID
			fileUrl          *string
			fileType         *string
			fileMime         *string
			fileSize         *int64
			fileOriginalPath *string
			fileHash         *string
			ownerId          *uuid.UUID
			ownerName        *string
			liked            *int32
			totalLikes       *int32
			totalDislikes    *int32
			views            *int32
		)

		err = rows.Scan(
			&worldId,
			&name,
			&description,
			&mapName,
			&gameMode,
			&modId,
			&modName,
			&modTitle,
			&public,
			&pakId,
			&pakUrl,
			&pakType,
			&pakMime,
			&pakSize,
			&pakOriginalPath,
			&pakHash,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileSize,
			&fileOriginalPath,
			&fileHash,
			&ownerId,
			&ownerName,
			&liked,
			&totalLikes,
			&totalDislikes,
			&views,
		)
		if err != nil {
			return nil, err
		}

		if worldId.Status == pgtype.Null {
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

			if pakOriginalPath != nil && *pakOriginalPath != "" {
				pak.OriginalPath = pakOriginalPath
			}

			if pakHash != nil && *pakHash != "" {
				pak.Hash = pakHash
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

			if fileOriginalPath != nil && *fileOriginalPath != "" {
				file.OriginalPath = fileOriginalPath
			}

			if fileHash != nil && *fileHash != "" {
				file.Hash = fileHash
			}
		}

		if entity != nil {
			if file != nil && !containsFile(entity.Files, *file.Id) {
				entity.Files = append(entity.Files, *file)
			}

			if pak != nil && !containsFile(entity.Package.Files, *pak.Id) {
				entity.Package.Files = append(entity.Package.Files, *pak)
			}
		} else {
			var e World
			e.Id = &worldId.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if description != nil {
				e.Description = *description
			}
			if gameMode != nil {
				e.GameMode = *gameMode
			}
			if mapName != nil {
				e.Map = *mapName
			}

			e.Owner = new(User)
			if ownerId != nil {
				e.Owner.Id = ownerId
			}

			if ownerName != nil {
				e.Owner.Name = ownerName
			}

			if liked != nil {
				e.Liked = liked
			}

			if file != nil {
				e.Files = append(e.Files, *file)
			}

			if totalLikes != nil {
				e.TotalLikes = totalLikes
			} else {
				e.TotalLikes = new(int32)
				*e.TotalLikes = 0
			}

			if totalDislikes != nil {
				e.TotalDislikes = totalDislikes
			} else {
				e.TotalDislikes = new(int32)
				*e.TotalDislikes = 0
			}

			e.Views = views

			if e.Package == nil && modId != nil {
				e.Package = new(Package)
				e.Package.Id = modId

				if modName != nil {
					e.Package.Name = *modName
				}

				if modTitle != nil {
					e.Package.Title = *modTitle
				}

				if pak != nil {
					e.Package.Files = append(e.Files, *pak)
				}
			}

			entity = &e
		}
	}

	return entity, err
}

// GetWorldForAdmin Get portal
func GetWorldForAdmin(ctx context.Context, requester *sm.User, id uuid.UUID) (entity *World, err error) {
	db := database.DB

	q := `SELECT
	w.id                    worldId,
	w.name                  worldName,
	w.description           worldDescription,
	w.map                   worldMap,
	w.game_mode             worldGameMode,
	e.public                entityPublic,
	f.id              previewId,
	f.url             previewUrl,
	f.type            previewType,
	f.mime        	  previewMime,
	f.size			  previewSize,
	owner.id 			ownerId,
	owner.name 			ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM spaces w
	LEFT JOIN entities e on w.id = e.id
    LEFT JOIN likables l ON l.entity_id = e.id
    LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
    LEFT JOIN accessibles a ON a.entity_id = e.id
	LEFT JOIN users owner ON owner.id = a.user_id
    LEFT JOIN files f ON e.id = f.entity_id
WHERE w.id = $2
GROUP BY e.id, w.id, e.public, f.id, f.url, f.type, f.mime, f.size, owner.id, l2.value, e.views
ORDER BY e.id`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, requester.Id, id /*$1*/)

	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetWorldForAdmin")
	}()
	for rows.Next() {
		var (
			worldId       pgtypeuuid.UUID
			name          *string
			description   *string
			mapName       *string
			gameMode      *string
			public        *bool
			fileId        pgtypeuuid.UUID
			fileUrl       *string
			fileType      *string
			fileMime      *string
			fileSize      *int64
			ownerId       *uuid.UUID
			ownerName     *string
			liked         *int32
			totalLikes    *int32
			totalDislikes *int32
			views         *int32
		)

		err = rows.Scan(
			&worldId,
			&name,
			&description,
			&mapName,
			&gameMode,
			&public,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileSize,
			&ownerId,
			&ownerName,
			&liked,
			&totalLikes,
			&totalDislikes,
			&views,
		)
		if err != nil {
			return nil, err
		}

		if worldId.Status == pgtype.Null {
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

			if fileSize != nil {
				file.Size = fileSize
			}
		}

		if entity != nil {
			if file != nil && !containsFile(entity.Files, *file.Id) {
				entity.Files = append(entity.Files, *file)
			}
		} else {
			var e World
			e.Id = &worldId.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if description != nil {
				e.Description = *description
			}
			if mapName != nil {
				e.Map = *mapName
			}
			if gameMode != nil {
				e.GameMode = *gameMode
			}

			e.Owner = new(User)
			if ownerId != nil {
				e.Owner.Id = ownerId
			}

			if ownerName != nil {
				e.Owner.Name = ownerName
			}

			if liked != nil {
				e.Liked = liked
			}

			if file != nil {
				e.Files = append(e.Files, *file)
			}

			if totalLikes != nil {
				e.TotalLikes = totalLikes
			} else {
				e.TotalLikes = new(int32)
				*e.TotalLikes = 0
			}

			if totalDislikes != nil {
				e.TotalDislikes = totalDislikes
			} else {
				e.TotalDislikes = new(int32)
				*e.TotalDislikes = 0
			}

			e.Views = views

			entity = &e
		}
	}

	return entity, err
}

// GetWorldForRequesterWithPak Get world with pack
func GetWorldForRequesterWithPak(ctx context.Context, requester *sm.User, id uuid.UUID, platform string, deployment string) (entity *World, err error) {
	db := database.DB

	q := `SELECT
	w.id                    worldId,
	w.name                  worldName,
	w.description           worldDescription,
	w.map                   worldMap,
	w.game_mode             worldGameMode,
	m.id					modId,
	m.name					modName,
	m.title					modTitle,
	e.public                entityPublic,
	pak.id                  pakId,
	pak.url                 pakUrl,
	pak.type                pakType,
	pak.mime            	pakMime,
	pak.size				pakSize,
	pak.original_path		pakOriginalPath,
	pak.hash				pakHash,
	f.id              previewId,
	f.url             previewUrl,
	f.type            previewType,
	f.mime        	previewMime,
	f.size			previewSize,
	f.original_path	previewOriginalPath,
	f.hash			previewHash,
	owner.name 		ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM spaces w
	LEFT JOIN entities e on w.id = e.id
	LEFT JOIN likables l ON l.entity_id = e.id
    LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
	LEFT JOIN accessibles a ON e.id = a.entity_id
	LEFT JOIN users owner ON owner.id = a.user_id
    LEFT JOIN files f ON w.id = f.entity_id
    LEFT JOIN mods m ON m.id = w.mod_id 
	LEFT JOIN entities me ON me.id = m.id
	LEFT JOIN files pak ON me.id = pak.entity_id AND ((pak.platform = $2::text AND pak.deployment_type = $3::text) OR (pak.platform = '' AND pak.deployment_type = ''))
WHERE w.id = $4
GROUP BY e.id, w.id, m.id, e.public, pak.id, pak.url, pak.type, pak.mime, pak.size, pak.original_path, pak.hash, f.id, f.url, f.type, f.mime, f.size, f.original_path, f.hash, owner.name, l2.value, e.views
ORDER BY e.id`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, requester.Id, platform /*$1*/, deployment /*$2*/, id /*$3*/)

	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetWorldForRequesterWithPak")
	}()
	for rows.Next() {
		var (
			worldId          pgtypeuuid.UUID
			name             *string
			description      *string
			mapName          *string
			gameMode         *string
			modId            *uuid.UUID
			modName          *string
			modTitle         *string
			public           *bool
			pakId            pgtypeuuid.UUID
			pakUrl           *string
			pakType          *string
			pakMime          *string
			pakSize          *int64
			pakOriginalPath  *string
			pakHash          *string
			fileId           pgtypeuuid.UUID
			fileUrl          *string
			fileType         *string
			fileMime         *string
			fileSize         *int64
			fileOriginalPath *string
			fileHash         *string
			ownerName        *string
			liked            *int32
			totalLikes       *int32
			totalDislikes    *int32
			views            *int32
		)

		err = rows.Scan(
			&worldId,
			&name,
			&description,
			&mapName,
			&gameMode,
			&modId,
			&modName,
			&modTitle,
			&public,
			&pakId,
			&pakUrl,
			&pakType,
			&pakMime,
			&pakSize,
			&pakOriginalPath,
			&pakHash,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileSize,
			&fileOriginalPath,
			&fileHash,
			&ownerName,
			&liked,
			&totalLikes,
			&totalDislikes,
			&views,
		)
		if err != nil {
			return nil, err
		}

		if worldId.Status == pgtype.Null {
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

			if pakOriginalPath != nil && *pakOriginalPath != "" {
				pak.OriginalPath = pakOriginalPath
			}

			if pakHash != nil && *pakHash != "" {
				pak.Hash = pakHash
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

			if fileOriginalPath != nil && *fileOriginalPath != "" {
				file.OriginalPath = fileOriginalPath
			}

			if fileHash != nil && *fileHash != "" {
				file.Hash = fileHash
			}
		}

		if entity != nil {
			if file != nil && !containsFile(entity.Files, *file.Id) {
				entity.Files = append(entity.Files, *file)
			}

			if pak != nil && !containsFile(entity.Package.Files, *pak.Id) {
				entity.Package.Files = append(entity.Package.Files, *pak)
			}
		} else {
			var e World
			e.Id = &worldId.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if description != nil {
				e.Description = *description
			}
			if gameMode != nil {
				e.GameMode = *gameMode
			}
			if mapName != nil {
				e.Map = *mapName
			}

			if e.Package == nil && modId != nil {
				e.Package = new(Package)
				e.Package.Id = modId

				if modName != nil {
					e.Package.Name = *modName
				}

				if modTitle != nil {
					e.Package.Title = *modTitle
				}

				if pak != nil {
					e.Package.Files = append(e.Files, *pak)
				}
			}

			e.Owner = new(User)
			if ownerName != nil {
				e.Owner.Name = ownerName
			}

			if liked != nil {
				e.Liked = liked
			}

			if file != nil {
				e.Files = append(e.Files, *file)
			}

			if totalLikes != nil {
				e.TotalLikes = totalLikes
			} else {
				e.TotalLikes = new(int32)
				*e.TotalLikes = 0
			}

			if totalDislikes != nil {
				e.TotalDislikes = totalDislikes
			} else {
				e.TotalDislikes = new(int32)
				*e.TotalDislikes = 0
			}

			e.Views = views

			entity = &e
		}
	}

	return entity, err
}

// GetWorldForRequester Get world
func GetWorldForRequester(ctx context.Context, requester *sm.User, id uuid.UUID) (entity *World, err error) {
	db := database.DB

	q := `SELECT
	w.id                    worldId,
	w.name                  worldName,
	w.description           worldDescription,
	w.map                   worldMap,
	w.game_mode             worldGameMode,
	w.mod_id				modId,
	e.public                entityPublic,
	f.id              		fId,
	f.url             		fUrl,
	f.type            		fType,
	f.mime        			fMime,
	f.size					fSize,
	owner.name 				ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM spaces w
	LEFT JOIN entities e on w.id = e.id
	LEFT JOIN likables l ON l.entity_id = e.id
    LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
    LEFT JOIN files f ON w.id = f.entity_id
	LEFT JOIN accessibles a ON e.id = a.entity_id
	LEFT JOIN users owner ON owner.id = a.user_id
WHERE w.id = $2
GROUP BY e.id, w.id, e.public, f.id, f.url, f.type, f.mime, f.size, owner.name, l2.value, e.views
ORDER BY e.id`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, requester.Id, id /*$1*/)

	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetWorldForRequester")
	}()
	for rows.Next() {
		var (
			worldId       pgtypeuuid.UUID
			name          *string
			description   *string
			mapName       *string
			gameMode      *string
			packageId     *uuid.UUID
			public        *bool
			fileId        pgtypeuuid.UUID
			fileUrl       *string
			fileType      *string
			fileMime      *string
			fileSize      *int64
			ownerName     *string
			liked         *int32
			totalLikes    *int32
			totalDislikes *int32
			views         *int32
		)

		err = rows.Scan(
			&worldId,
			&name,
			&description,
			&mapName,
			&gameMode,
			&packageId,
			&public,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileSize,
			&ownerName,
			&liked,
			&totalLikes,
			&totalDislikes,
			&views,
		)
		if err != nil {
			return nil, err
		}

		if worldId.Status == pgtype.Null {
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

			if fileSize != nil {
				file.Size = fileSize
			}
		}

		if entity != nil {
			if file != nil && !containsFile(entity.Files, *file.Id) {
				entity.Files = append(entity.Files, *file)
			}
		} else {
			var e World
			e.Id = &worldId.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if description != nil {
				e.Description = *description
			}
			if mapName != nil {
				e.Map = *mapName
			}
			if gameMode != nil {
				e.GameMode = *gameMode
			}

			if e.Package == nil && packageId != nil {
				e.Package = new(Package)
				e.Package.Id = packageId
			}

			e.Owner = new(User)
			if ownerName != nil {
				e.Owner.Name = ownerName
			}

			if liked != nil {
				e.Liked = liked
			}

			if file != nil {
				e.Files = append(e.Files, *file)
			}

			if totalLikes != nil {
				e.TotalLikes = totalLikes
			} else {
				e.TotalLikes = new(int32)
				*e.TotalLikes = 0
			}

			if totalDislikes != nil {
				e.TotalDislikes = totalDislikes
			} else {
				e.TotalDislikes = new(int32)
				*e.TotalDislikes = 0
			}

			e.Views = views

			entity = &e
		}
	}

	return entity, err
}

// GetLatestWorldForAdminWithPak Get the latest world for admin
func GetLatestWorldForAdminWithPak(ctx context.Context, requester *sm.User, platform string, deployment string) (entity *World, err error) {
	db := database.DB

	q := `SELECT
	w.id                    worldId,
	w.name                  worldName,
	w.description           worldDescription,
	w.map                   worldMap,
	w.game_mode             worldGameMode,
	m.id					modId,
	m.name					modName,
	m.title					modTitle,
	e.public                entityPublic,
	pak.id                  pakId,
	pak.url                 pakUrl,
	pak.type                pakType,
	pak.mime            	pakMime,
	pak.size				pakSize,
	pak.original_path		pakOriginalPath,
	pak.hash				pakHash,
	f.id					previewId,
	f.url             		previewUrl,
	f.type            		previewType,
	f.mime        	  		previewMime,
	f.size			  		previewSize,
	f.original_path			previewOriginalPath,
	f.hash					previewHash,
	owner.id 				ownerId,
	owner.name 				ownerName,
	l2.value				liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM spaces w
	LEFT JOIN entities e on w.id = e.id
	LEFT JOIN likables l ON l.entity_id = e.id
    LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
	LEFT JOIN accessibles a ON a.entity_id = e.id
	LEFT JOIN users owner ON owner.id = a.user_id
	LEFT JOIN files f ON e.id = f.entity_id 
    LEFT JOIN mods m ON m.id = w.mod_id 
	LEFT JOIN entities me ON me.id = m.id
	LEFT JOIN files pak ON me.id = pak.entity_id AND ((pak.platform = $2::text AND pak.deployment_type = $3::text) OR (pak.platform = '' AND pak.deployment_type = ''))
GROUP BY w.id, w.name, w.description, w.map, w.game_mode, m.id, m.name, m.title, e.public, pak.id, pak.url, pak.type, pak.mime, pak.size, pak.original_path, pak.hash, f.id, f.url, f.type, f.mime, f.size, f.original_path, f.hash, owner.id, owner.name, l2.value, e.created_at, e.views
ORDER BY e.created_at DESC`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, requester.Id, platform /*$1*/, deployment /*$2*/)

	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	var latestId uuid.UUID

	defer func() {
		rows.Close()
		database.LogPgxStat("GetLatestWorldForAdminWithPak")
	}()
	for rows.Next() {
		var (
			worldId          pgtypeuuid.UUID
			name             *string
			description      *string
			mapName          *string
			gameMode         *string
			modId            *uuid.UUID
			modName          *string
			modTitle         *string
			public           *bool
			pakId            pgtypeuuid.UUID
			pakUrl           *string
			pakType          *string
			pakMime          *string
			pakSize          *int64
			pakOriginalPath  *string
			pakHash          *string
			fileId           pgtypeuuid.UUID
			fileUrl          *string
			fileType         *string
			fileMime         *string
			fileSize         *int64
			fileOriginalPath *string
			fileHash         *string
			ownerId          *uuid.UUID
			ownerName        *string
			liked            *int32
			totalLikes       *int32
			totalDislikes    *int32
			views            *int32
		)

		err = rows.Scan(
			&worldId,
			&name,
			&description,
			&mapName,
			&gameMode,
			&modId,
			&modName,
			&modTitle,
			&public,
			&pakId,
			&pakUrl,
			&pakType,
			&pakMime,
			&pakSize,
			&pakOriginalPath,
			&pakHash,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
			&fileSize,
			&fileOriginalPath,
			&fileHash,
			&ownerId,
			&ownerName,
			&liked,
			&totalLikes,
			&totalDislikes,
			&views,
		)
		if err != nil {
			return nil, err
		}

		if worldId.Status == pgtype.Null {
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

			if pakOriginalPath != nil && *pakOriginalPath != "" {
				pak.OriginalPath = pakOriginalPath
			}

			if pakHash != nil && *pakHash != "" {
				pak.Hash = pakHash
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

			if fileOriginalPath != nil && *fileOriginalPath != "" {
				file.OriginalPath = fileOriginalPath
			}

			if fileHash != nil && *fileHash != "" {
				file.Hash = fileHash
			}
		}

		if entity != nil {
			if latestId != *entity.Id {
				// Next entity, stop iteration
				rows.Close()
				break
			}

			if file != nil && !containsFile(entity.Files, *file.Id) {
				entity.Files = append(entity.Files, *file)
			}

			if pak != nil && !containsFile(entity.Package.Files, *pak.Id) {
				entity.Package.Files = append(entity.Package.Files, *pak)
			}
		} else {
			var e World
			e.Id = &worldId.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if description != nil {
				e.Description = *description
			}
			if gameMode != nil {
				e.GameMode = *gameMode
			}
			if mapName != nil {
				e.Map = *mapName
			}

			e.Owner = new(User)
			if ownerId != nil {
				e.Owner.Id = ownerId
			}

			if ownerName != nil {
				e.Owner.Name = ownerName
			}

			if liked != nil {
				e.Liked = liked
			}

			if file != nil {
				e.Files = append(e.Files, *file)
			}

			if totalLikes != nil {
				e.TotalLikes = totalLikes
			} else {
				e.TotalLikes = new(int32)
				*e.TotalLikes = 0
			}

			if totalDislikes != nil {
				e.TotalDislikes = totalDislikes
			} else {
				e.TotalDislikes = new(int32)
				*e.TotalDislikes = 0
			}

			e.Views = views

			if e.Package == nil && modId != nil {
				e.Package = new(Package)
				e.Package.Id = modId

				if modName != nil {
					e.Package.Name = *modName
				}

				if modTitle != nil {
					e.Package.Title = *modTitle
				}

				if pak != nil {
					e.Package.Files = append(e.Files, *pak)
				}
			}

			entity = &e
		}

		latestId = *entity.Id
	}

	return entity, err
}

// CreateWorldForRequester Creates a new world
func CreateWorldForRequester(ctx context.Context, requester *sm.User, m WorldCreateMetadata) (entity *World, err error) {
	db := database.DB

	id, err1 := uuid.NewV4()
	if err1 != nil {
		return nil, fmt.Errorf("failed to generate uuid: %v", err1)
	}

	tx, err1 := db.Begin(ctx)
	if err1 != nil {
		return nil, fmt.Errorf("failed to begin tx: %v", err1)
	}

	//region Entity
	entityType := "space"
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
	q = `INSERT INTO spaces (id,
                    name,
                    description,
					map,
					mod_id,
					type,
					game_mode) VALUES ($1, $2, $3, $4, $5, $6, $7)`

	if _, err1 = tx.Exec(ctx, q, id /*1*/, m.Name /*2*/, m.Description /*3*/, m.Map /*4*/, m.PackageId /*5*/, m.Type /*6*/, m.GameMode /*7*/); err1 != nil {
		if err2 := tx.Rollback(ctx); err2 != nil {
			return nil, fmt.Errorf("failed to rollback failed tx: %v, %v", err1, err2)
		}
		return nil, fmt.Errorf("failed to exec tx: %v", err1)
	}
	//endregion

	// Create a default portal with no destination that will work as a default entrypoint for this world
	portalId, err1 := uuid.NewV4()
	if err1 != nil {
		return nil, fmt.Errorf("failed to generate uuid for the portal: %v", err1)
	}

	//region Portal Entity
	entityType = "portal"
	q = `INSERT INTO entities (id, entity_type, public) VALUES ($1, $2, $3)`
	if _, err1 = tx.Exec(ctx, q, portalId /*1*/, entityType /*2*/, m.Public /*3*/); err1 != nil {
		if err2 := tx.Rollback(ctx); err2 != nil {
			return nil, fmt.Errorf("failed to rollback failed portal tx: %v, %v", err1, err2)
		}
		return nil, fmt.Errorf("failed to exec tx: %v", err1)
	}
	//endregion

	q = `INSERT INTO portals (id, name, space_id, destination_id) VALUES ($1, $2, $3, NULL)`
	if _, err1 = tx.Exec(ctx, q, portalId /*$1*/, "Default" /*$2*/, id /*$3*/); err1 != nil {
		if err2 := tx.Rollback(ctx); err2 != nil {
			return nil, fmt.Errorf("failed to rollback failed portal tx: %v, %v", err1, err2)
		}
		return nil, fmt.Errorf("failed to exec portal tx: %v", err1)
	}

	if err1 = tx.Commit(ctx); err1 != nil {
		if err2 := tx.Rollback(ctx); err2 != nil {
			return nil, fmt.Errorf("failed to rollback failed tx: %v, %v", err1, err2)
		}
		return nil, fmt.Errorf("failed to commit tx: %v", err1)
	}

	if entity, err1 = GetWorldForRequester(ctx, requester, id); err1 != nil {
		return nil, fmt.Errorf("failed to get the entity: %v", err1)
	}

	return entity, nil
}

func DeleteWorldForAdmin(ctx context.Context, id uuid.UUID) error {
	db := database.DB

	tx, err := db.Begin(ctx)
	if err != nil {
		_ = tx.Rollback(ctx)
		return err // failed to initiate tx
	}

	q := `DELETE FROM portals WHERE space_id = $1`
	_, err = tx.Exec(ctx, q, id)
	if err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("failed to delete %s @ %s: %v", portalSingular, reflect.FunctionName(), err)
	}

	q = `DELETE FROM entities WHERE id = $1`
	_, err = tx.Exec(ctx, q, id)

	if err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("failed to delete %s @ %s: %v", worldSingular, reflect.FunctionName(), err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("failed to commit tx: %v", err)
	}

	return nil
}

func DeleteWorldForRequester(ctx context.Context, requester *sm.User, id uuid.UUID) (err error) {
	db := database.DB

	q := `DELETE FROM entities e USING accessibles a WHERE e.id = $1 AND e.id = a.entity_id AND a.user_id = $2 AND a.can_delete = true`
	_, err = db.Exec(ctx, q, id, requester.Id /*$1*/)

	if err != nil {
		return err
	}

	return nil
}

func UpdateWorldForAdmin(ctx context.Context, requester *sm.User, id uuid.UUID, m WorldUpdateMetadata) (entity *World, err error) {
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

	var e *World
	e, err = GetWorldForAdmin(ctx, requester, id)

	if err != nil {
		return nil, fmt.Errorf("failed to get the source world: %v", err1)
	}

	if m.Name != nil {
		e.Name = *m.Name
	}

	if m.Map != nil {
		e.Map = *m.Map
	}

	if m.Description != nil {
		e.Description = *m.Description
	}

	if m.GameMode != nil {
		e.GameMode = *m.GameMode
	}

	if m.Type != nil {
		e.Type = m.Type
	}

	q := `UPDATE spaces SET name=$1, description=$2, game_mode=$3, map=$4, type=$5`
	spaceVals := []interface{}{e.Name, e.Description, e.GameMode, e.Map, e.Type}
	if m.PackageId != nil {
		if e.Package != nil {
			e.Package.Id = m.PackageId
		} else {
			e.Package = new(Package)
			e.Package.Id = m.PackageId
		}

		q += `, mod_id=$6 WHERE id = $7`
		spaceVals = append(spaceVals, e.Package.Id, id)
	} else {
		q += ` WHERE id = $6`
		spaceVals = append(spaceVals, id)
	}

	if _, err1 = tx.Exec(ctx, q, spaceVals...); err1 != nil {
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

	e, err1 = GetWorldForAdmin(ctx, requester, id)
	if err1 != nil {
		return nil, fmt.Errorf("failed to get the updated world: %v", err1)
	}

	return e, nil
}

func UpdateWorldForRequester(ctx context.Context, requester *sm.User, id uuid.UUID, m WorldUpdateMetadata) (entity *World, err error) {
	db := database.DB

	tx, err1 := db.Begin(ctx)
	if err1 != nil {
		return nil, fmt.Errorf("failed to begin tx: %v", err1)
	}

	if m.Public != nil {
		q := `UPDATE entities AS e SET public = $1 FROM accessibles AS a WHERE e.id = $2 AND a.entity_id = e.id AND a.user_id = $3`
		if _, err1 = tx.Exec(ctx, q, m.Public /*$1*/, id /*$2*/); err1 != nil {
			if err2 := tx.Rollback(ctx); err2 != nil {
				return nil, fmt.Errorf("failed to rollback failed tx: %v, %v", err1, err2)
			}
			return nil, fmt.Errorf("failed to exec entity update tx: %v", err1)
		}
	}

	var e *World
	e, err = GetWorldForRequester(ctx, requester, id)

	if err != nil {
		return nil, fmt.Errorf("failed to get the source world: %v", err1)
	}

	if m.Name != nil {
		e.Name = *m.Name
	}

	if m.Map != nil {
		e.Map = *m.Map
	}

	if m.Description != nil {
		e.Description = *m.Description
	}

	if m.GameMode != nil {
		e.GameMode = *m.GameMode
	}

	if m.Type != nil {
		e.Type = m.Type
	}

	var q string
	spaceVals := []interface{}{e.Name, e.Description, e.GameMode, e.Map, e.Type}
	if m.PackageId != nil {
		if e.Package != nil {
			e.Package.Id = m.PackageId
		} else {
			e.Package = new(Package)
			e.Package.Id = m.PackageId
		}

		q = `UPDATE spaces AS s SET name = $1, description=$2, game_mode=$3, map=$4, type=$5, mod_id=$6  FROM accessibles a WHERE s.id = $7 AND s.id = a.entity_id AND a.user_id = $8`
		spaceVals = append(spaceVals, e.Package.Id, id, requester.Id)
	} else {
		q = `UPDATE spaces AS s SET name=$1, description=$2, game_mode=$3, map=$4, type=$5 FROM accessibles a WHERE s.id = $6 AND s.id = a.entity_id AND a.user_id = $7`
		spaceVals = append(spaceVals, id, requester.Id)
	}

	if _, err1 = tx.Exec(ctx, q, spaceVals...); err1 != nil {
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

	e, err1 = GetWorldForRequester(ctx, requester, id)
	if err1 != nil {
		return nil, fmt.Errorf("failed to get the updated world: %v", err1)
	}

	return e, nil
}
