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

var (
	portalSingular = "portal"
	portalPlural   = "portals"
)

// Portal struct
type Portal struct {
	Entity

	Name        string  `json:"name,omitempty"`
	World       *World  `json:"space,omitempty"`
	Destination *Portal `json:"destination,omitempty"`
}

var SupportedPlatform = map[string]bool{
	"Win64":   true,
	"Linux":   true,
	"Mac":     true,
	"IOS":     true,
	"Android": true,
}

var SupportedDeployment = map[string]bool{
	"Client": true,
	"Server": true,
}

var SupportedConfiguration = map[string]bool{
	"Development": true,
	"Test":        true,
	"Shipping":    true,
}

// PortalBatchRequestMetadata Batch request metadata for requesting Portal entities
type PortalBatchRequestMetadata struct {
	BatchRequestMetadata
	WorldId    *uuid.UUID `json:"spaceId,omitempty"`    // Optional World where the portal is located to filter portals
	Platform   string     `json:"platform,omitempty"`   // SupportedPlatform (OS) of the destination pak file (Win64, Mac, Linux, IOS, Android)
	Deployment string     `json:"deployment,omitempty"` // SupportedDeployment for the destination pak file (Server or Client)
}

type PortalRequestMetadata struct {
	PackageRequestMetadata
}

type PortalCreateMetadata struct {
	Name          string     `json:"name,omitempty"`          // Name that used as portal identifier and part of the portal URL
	Public        *bool      `json:"public,omitempty"`        // Public or private
	WorldId       uuid.UUID  `json:"spaceId,omitempty"`       // Base WorldId
	DestinationId *uuid.UUID `json:"destinationId,omitempty"` // DestinationId target portal ID
}

type PortalUpdateMetadata struct {
	Name          *string    `json:"name,omitempty"`          // Name that used as portal identifier and part of the portal URL
	Public        *bool      `json:"public,omitempty"`        // Public or private
	WorldId       *uuid.UUID `json:"spaceId,omitempty"`       // Base WorldId
	DestinationId *uuid.UUID `json:"destinationId,omitempty"` // DestinationId target portal ID
}

func findPortal(h []Portal, id uuid.UUID) int {
	for i, v := range h {
		if *v.Id == id {
			return i
		}
	}
	return -1
}

// IndexPortalsForAdmin Index portals
func IndexPortalsForAdmin(ctx context.Context, offset int64, limit int64) (entities []Portal, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) FROM portals p`

	row := db.QueryRow(ctx, q)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	q = `SELECT
	p.id portalId,
	p.name portalName,
	e.public entityPublic,
	d.id destinationId,
	d.name destinationName,
	s.id spaceId,
	s.name spaceName,
	s.map spaceMap,
	m.id packageId,
	m.name packageName,
	m.title packageTitle,
	preview.id previewId,
	preview.url previewUrl,
	preview.type previewType,
	preview.mime previewMime,
	preview.size previewSize
FROM portals p
	LEFT JOIN entities e on p.id = e.id
	-- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id 
    LEFT JOIN entities de ON de.id = d.id
    LEFT JOIN files preview ON de.id = preview.entity_id AND preview.type = 'rendertarget_preview'
	-- destination space
    LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
 	-- destination package
    LEFT JOIN mods m ON m.id = s.mod_id
	LEFT JOIN entities me ON me.id = m.id
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows      pgx.Rows
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q)

	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPortalsForAdmin")
	}()
	for rows.Next() {
		var (
			id              pgtypeuuid.UUID
			name            *string
			public          *bool
			destinationId   pgtypeuuid.UUID
			destinationName *string
			spaceId         pgtypeuuid.UUID
			spaceName       *string
			spaceMap        *string
			packageId       pgtypeuuid.UUID
			packageName     *string
			packageTitle    *string
			previewId       pgtypeuuid.UUID
			previewUrl      *string
			previewType     *string
			previewMime     *string
			previewSize     *int64
		)

		err = rows.Scan(
			&id,
			&name,
			&public,
			&destinationId,
			&destinationName,
			&spaceId,
			&spaceName,
			&spaceMap,
			&packageId,
			&packageName,
			&packageTitle,
			&previewId,
			&previewUrl,
			&previewType,
			&previewMime,
			&previewSize,
		)
		if err != nil {
			return nil, -1, err
		}

		if id.Status == pgtype.Null {
			continue
		}

		var preview *File
		if previewId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &previewId.UUID
			if previewUrl != nil {
				preview.Url = *previewUrl
			}

			if previewType != nil {
				preview.Type = *previewType
			}

			if previewMime != nil {
				preview.Mime = previewMime
			}

			if previewSize != nil {
				preview.Size = previewSize
			}
		}

		if i := findPortal(entities, id.UUID); i >= 0 {
			if preview != nil && !containsFile(entities[i].Destination.Files, *preview.Id) {
				if entities[i].Destination != nil {
					entities[i].Destination.Files = append(entities[i].Destination.Files, *preview)
				}
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

			var e Portal
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if destinationId.Status != pgtype.Null {
				if e.Destination == nil {
					e.Destination = new(Portal)
				}
				e.Destination.Id = &destinationId.UUID
				if destinationName != nil {
					e.Destination.Name = *destinationName
				}
				if preview != nil {
					e.Destination.Files = append(e.Files, *preview)
				}
				if spaceId.Status != pgtype.Null {
					if e.Destination.World == nil {
						e.Destination.World = new(World)
					}
					e.Destination.World.Id = &spaceId.UUID
					if spaceName != nil {
						e.Destination.World.Name = *spaceName
					}
					if spaceMap != nil {
						e.Destination.World.Map = *spaceMap
					}
					if packageId.Status != pgtype.Null {
						if e.Destination.World.Package == nil {
							e.Destination.World.Package = new(Package)
						}
						e.Destination.World.Package.Id = &packageId.UUID
						if packageName != nil {
							e.Destination.World.Package.Name = *packageName
						}
						if packageTitle != nil {
							e.Destination.World.Package.Title = *packageTitle
						}
					}
				}
			}
			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexPortalsForAdminWithPak Index portals
func IndexPortalsForAdminWithPak(ctx context.Context, platform string, deployment string, offset int64, limit int64) (entities []Portal, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) FROM portals p`

	row := db.QueryRow(ctx, q)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	q = `SELECT
	p.id portalId,
	p.name portalName,
	e.public entityPublic,
	d.id destinationId,
	d.name destinationName,
	s.id spaceId,
	s.name spaceName,
	s.map spaceMap,
	m.id packageId,
	m.name packageName,
	m.title packageTitle,
	preview.id previewId,
	preview.url previewUrl,
	preview.type previewType,
	preview.mime previewMime,
    preview.size previewSize,
	pak.id pakId,
	pak.url pakUrl,
	pak.type pakType,
	pak.mime pakMime,
	pak.size pakSize
FROM portals p
	LEFT JOIN entities e on p.id = e.id
	-- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON de.id = d.id
    LEFT JOIN files preview ON de.id = preview.entity_id AND preview.type = 'rendertarget_preview'
	-- destination space
    LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
	-- destination package
    LEFT JOIN mods m ON m.id = s.mod_id 
	LEFT JOIN entities me ON me.id = m.id
	LEFT JOIN files pak ON me.id = pak.entity_id AND pak.type = 'pak' AND pak.platform = $1::text AND pak.deployment_type = $2::text
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows      pgx.Rows
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, platform /*$1*/, deployment /*$2*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPortalsForAdminWithPak")
	}()
	for rows.Next() {
		var (
			id              pgtypeuuid.UUID
			name            *string
			public          *bool
			destinationId   pgtypeuuid.UUID
			destinationName *string
			spaceId         pgtypeuuid.UUID
			spaceName       *string
			spaceMap        *string
			packageId       pgtypeuuid.UUID
			packageName     *string
			packageTitle    *string
			previewId       pgtypeuuid.UUID
			previewUrl      *string
			previewType     *string
			previewMime     *string
			previewSize     *int64
			pakId           pgtypeuuid.UUID
			pakUrl          *string
			pakType         *string
			pakMime         *string
			pakSize         *int64
		)

		err = rows.Scan(
			&id,
			&name,
			&public,
			&destinationId,
			&destinationName,
			&spaceId,
			&spaceName,
			&spaceMap,
			&packageId,
			&packageName,
			&packageTitle,
			&previewId,
			&previewUrl,
			&previewType,
			&previewMime,
			&previewSize,
			&pakId,
			&pakUrl,
			&pakType,
			&pakMime,
			&pakSize,
		)
		if err != nil {
			return nil, -1, err
		}

		if id.Status == pgtype.Null {
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
		}

		var preview *File
		if previewId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &previewId.UUID
			if previewUrl != nil {
				preview.Url = *previewUrl
			}

			if previewType != nil {
				preview.Type = *previewType
			}

			if previewMime != nil {
				preview.Mime = previewMime
			}

			if previewSize != nil {
				preview.Size = previewSize
			}
		}

		if i := findPortal(entities, id.UUID); i >= 0 {
			if preview != nil && !containsFile(entities[i].Destination.Files, *preview.Id) {
				if entities[i].Destination != nil {
					entities[i].Destination.Files = append(entities[i].Destination.Files, *preview)
				}
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

			var e Portal
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if destinationId.Status != pgtype.Null {
				if e.Destination == nil {
					e.Destination = new(Portal)
				}
				e.Destination.Id = &destinationId.UUID
				if destinationName != nil {
					e.Destination.Name = *destinationName
				}
				if preview != nil {
					e.Destination.Files = append(e.Files, *preview)
				}
				if spaceId.Status != pgtype.Null {
					if e.Destination.World == nil {
						e.Destination.World = new(World)
					}
					e.Destination.World.Id = &spaceId.UUID
					if spaceName != nil {
						e.Destination.World.Name = *spaceName
					}
					if spaceMap != nil {
						e.Destination.World.Map = *spaceMap
					}
					if packageId.Status != pgtype.Null {
						if e.Destination.World.Package == nil {
							e.Destination.World.Package = new(Package)
						}
						e.Destination.World.Package.Id = &packageId.UUID
						if packageName != nil {
							e.Destination.World.Package.Name = *packageName
						}
						if packageTitle != nil {
							e.Destination.World.Package.Title = *packageTitle
						}
						if pak != nil {
							if e.Destination.World.Package == nil {
								e.Destination.World.Package = new(Package)
							}
							e.Destination.World.Package.Files = append(e.Files, *pak)
						}
					}
				}
			}
			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexPortalsForAdminForWorldWithPak Index portals
func IndexPortalsForAdminForWorldWithPak(ctx context.Context, spaceId uuid.UUID, platform string, deployment string, offset int64, limit int64) (entities []Portal, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) FROM portals p WHERE p.space_id = $1`

	row := db.QueryRow(ctx, q, spaceId /*$1*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	q = `SELECT
	p.id portalId,
	p.name portalName,
	e.public entityPublic,
	d.id destinationId,
	d.name destinationName,
	s.id spaceId,
	s.name spaceName,
	s.map spaceMap,
	m.id packageId,
	m.name packageName,
	m.title packageTitle,
	preview.id previewId,
	preview.url previewUrl,
	preview.type previewType,
	preview.mime previewMime,
    preview.size previewSize,
	pak.id pakId,
	pak.url pakUrl,
	pak.type pakType,
	pak.mime pakMime,
	pak.size pakSize
FROM portals p
	LEFT JOIN entities e on p.id = e.id
    -- destination
    LEFT JOIN portals d ON d.id = p.destination_id 
    LEFT JOIN entities de ON de.id = d.id
    LEFT JOIN files preview ON de.id = preview.entity_id AND preview.type = 'rendertarget_preview'
    -- destination space
    LEFT JOIN spaces s ON s.id = d.space_id 
	LEFT JOIN entities se ON se.id = s.id
	-- destination package
    LEFT JOIN mods m ON m.id = s.mod_id 
	LEFT JOIN entities me ON me.id = m.id
	LEFT JOIN files pak ON me.id = pak.entity_id AND pak.type = 'pak' AND pak.platform = $1::text AND pak.deployment_type = $2::text
WHERE p.space_id = $3
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows      pgx.Rows
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, platform /*$1*/, deployment /*$2*/, spaceId /*$3*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPortalsForAdminForWorldWithPak")
	}()
	for rows.Next() {
		var (
			id                   pgtypeuuid.UUID
			name                 *string
			public               *bool
			destinationId        pgtypeuuid.UUID
			destinationName      *string
			destinationSpaceId   pgtypeuuid.UUID
			destinationSpaceName *string
			destinationSpaceMap  *string
			packageId            pgtypeuuid.UUID
			packageName          *string
			packageTitle         *string
			previewId            pgtypeuuid.UUID
			previewUrl           *string
			previewType          *string
			previewMime          *string
			previewSize          *int64
			pakId                pgtypeuuid.UUID
			pakUrl               *string
			pakType              *string
			pakMime              *string
			pakSize              *int64
		)

		err = rows.Scan(
			&id,
			&name,
			&public,
			&destinationId,
			&destinationName,
			&destinationSpaceId,
			&destinationSpaceName,
			&destinationSpaceMap,
			&packageId,
			&packageName,
			&packageTitle,
			&previewId,
			&previewUrl,
			&previewType,
			&previewMime,
			&previewSize,
			&pakId,
			&pakUrl,
			&pakType,
			&pakMime,
			&pakSize,
		)
		if err != nil {
			return nil, -1, err
		}

		if id.Status == pgtype.Null {
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
		}

		var preview *File
		if previewId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &previewId.UUID
			if previewUrl != nil {
				preview.Url = *previewUrl
			}

			if previewType != nil {
				preview.Type = *previewType
			}

			if previewMime != nil {
				preview.Mime = previewMime
			}

			if previewSize != nil {
				preview.Size = previewSize
			}
		}

		if i := findPortal(entities, id.UUID); i >= 0 {
			if preview != nil && !containsFile(entities[i].Destination.Files, *preview.Id) {
				if entities[i].Destination != nil {
					entities[i].Destination.Files = append(entities[i].Destination.Files, *preview)
				}
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

			var e Portal
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if destinationId.Status != pgtype.Null {
				if e.Destination == nil {
					e.Destination = new(Portal)
				}
				e.Destination.Id = &destinationId.UUID
				if destinationName != nil {
					e.Destination.Name = *destinationName
				}
				if preview != nil {
					e.Destination.Files = append(e.Files, *preview)
				}
				if destinationSpaceId.Status != pgtype.Null {
					if e.Destination.World == nil {
						e.Destination.World = new(World)
					}
					e.Destination.World.Id = &destinationSpaceId.UUID
					if destinationSpaceName != nil {
						e.Destination.World.Name = *destinationSpaceName
					}
					if destinationSpaceMap != nil {
						e.Destination.World.Map = *destinationSpaceMap
					}
					if packageId.Status != pgtype.Null {
						if e.Destination.World.Package == nil {
							e.Destination.World.Package = new(Package)
						}
						e.Destination.World.Package.Id = &packageId.UUID
						if packageName != nil {
							e.Destination.World.Package.Name = *packageName
						}
						if packageTitle != nil {
							e.Destination.World.Package.Title = *packageTitle
						}
						if pak != nil {
							if e.Destination.World.Package == nil {
								e.Destination.World.Package = new(Package)
							}
							e.Destination.World.Package.Files = append(e.Files, *pak)
						}
					}
				}
			}
			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexPortalsForAdminForWorld Index portals
func IndexPortalsForAdminForWorld(ctx context.Context, spaceId uuid.UUID, offset int64, limit int64) (entities []Portal, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) FROM portals p WHERE p.space_id = $1`

	row := db.QueryRow(ctx, q, spaceId /*$1*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	q = `SELECT
	p.id portalId,
	p.name portalName,
	e.public entityPublic,
	d.id destinationId,
	d.name destinationName,
	s.id spaceId,
	s.name spaceName,
	s.map spaceMap,
	m.id packageId,
	m.name packageName,
	m.title packageTitle,
	preview.id previewId,
	preview.url previewUrl,
	preview.type previewType,
	preview.mime previewMime,
	preview.size previewSize
FROM portals p
	LEFT JOIN entities e on p.id = e.id
	-- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON de.id = d.id
    LEFT JOIN files preview ON de.id = preview.entity_id AND preview.type = 'rendertarget_preview'
	-- destination space
    LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
	-- destination package
    LEFT JOIN mods m ON m.id = s.mod_id
	LEFT JOIN entities me ON me.id = m.id
WHERE p.space_id = $1
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows      pgx.Rows
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, spaceId /*$1*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPortalsForAdminForWorld")
	}()
	for rows.Next() {
		var (
			id                   pgtypeuuid.UUID
			name                 *string
			public               *bool
			destinationId        pgtypeuuid.UUID
			destinationName      *string
			destinationSpaceId   pgtypeuuid.UUID
			destinationSpaceName *string
			destinationSpaceMap  *string
			packageId            pgtypeuuid.UUID
			packageName          *string
			packageTitle         *string
			previewId            pgtypeuuid.UUID
			previewUrl           *string
			previewType          *string
			previewMime          *string
			previewSize          *int64
		)

		err = rows.Scan(
			&id,
			&name,
			&public,
			&destinationId,
			&destinationName,
			&destinationSpaceId,
			&destinationSpaceName,
			&destinationSpaceMap,
			&packageId,
			&packageName,
			&packageTitle,
			&previewId,
			&previewUrl,
			&previewType,
			&previewMime,
			&previewSize,
		)
		if err != nil {
			return nil, -1, err
		}

		if id.Status == pgtype.Null {
			continue
		}

		var preview *File
		if previewId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &previewId.UUID
			if previewUrl != nil {
				preview.Url = *previewUrl
			}

			if previewType != nil {
				preview.Type = *previewType
			}

			if previewMime != nil {
				preview.Mime = previewMime
			}

			if previewSize != nil {
				preview.Size = previewSize
			}
		}

		if i := findPortal(entities, id.UUID); i >= 0 {
			if preview != nil && !containsFile(entities[i].Destination.Files, *preview.Id) {
				if entities[i].Destination != nil {
					entities[i].Destination.Files = append(entities[i].Destination.Files, *preview)
				}
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

			var e Portal
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if destinationId.Status != pgtype.Null {
				if e.Destination == nil {
					e.Destination = new(Portal)
				}
				e.Destination.Id = &destinationId.UUID
				if destinationName != nil {
					e.Destination.Name = *destinationName
				}
				if preview != nil {
					e.Destination.Files = append(e.Files, *preview)
				}
				if destinationSpaceId.Status != pgtype.Null {
					if e.Destination.World == nil {
						e.Destination.World = new(World)
					}
					e.Destination.World.Id = &destinationSpaceId.UUID
					if destinationSpaceName != nil {
						e.Destination.World.Name = *destinationSpaceName
					}
					if destinationSpaceMap != nil {
						e.Destination.World.Map = *destinationSpaceMap
					}
					if packageId.Status != pgtype.Null {
						if e.Destination.World.Package == nil {
							e.Destination.World.Package = new(Package)
						}
						e.Destination.World.Package.Id = &packageId.UUID
						if packageName != nil {
							e.Destination.World.Package.Name = *packageName
						}
						if packageTitle != nil {
							e.Destination.World.Package.Title = *packageTitle
						}
					}
				}
			}
			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexPortalsForAdminWithQueryWithPak Index portals
func IndexPortalsForAdminWithQueryWithPak(ctx context.Context, platform string, deployment string, query string, offset int64, limit int64) (entities []Portal, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) 
FROM portals p 
    LEFT JOIN portals d ON p.destination_id = d.id
	LEFT JOIN spaces s ON p.space_id = s.id 
	LEFT JOIN mods m ON s.mod_id = m.id
WHERE p.name ILIKE $1::text OR
      d.name ILIKE $1::text OR
      s.name ILIKE $1::text OR
      m.name ILIKE $1::text`
	row := db.QueryRow(ctx, q, query /*$1*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	q = `SELECT
	p.id portalId,
	p.name portalName,
	e.public entityPublic,
	d.id destinationId,
	d.name destinationName,
	s.id spaceId,
	s.name spaceName,
	s.map spaceMap,
	m.id packageId,
	m.name packageName,
	m.title packageTitle,
	preview.id previewId,
	preview.url previewUrl,
	preview.type previewType,
	preview.mime previewMime,
    preview.size previewSize,
	pak.id pakId,
	pak.url pakUrl,
	pak.type pakType,
	pak.mime pakMime,
	pak.size pakSize
FROM portals p
	LEFT JOIN entities e on p.id = e.id
	-- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON de.id = d.id
    LEFT JOIN files preview ON de.id = preview.entity_id AND preview.type = 'rendertarget_preview'
	-- destination space
    LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
	-- destination package
    LEFT JOIN mods m ON m.id = s.mod_id
	LEFT JOIN entities me ON me.id = m.id
	LEFT JOIN files pak ON me.id = pak.entity_id AND pak.type = 'pak' AND pak.platform = $1::text AND pak.deployment_type = $2::text
WHERE p.name ILIKE $3::text OR
      d.name ILIKE $3::text OR
      s.name ILIKE $3::text OR
      m.name ILIKE $3::text
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows      pgx.Rows
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, platform /*$1*/, deployment /*$2*/, query /*$3*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPortalsForAdminWithQueryWithPak")
	}()
	for rows.Next() {
		var (
			id                   pgtypeuuid.UUID
			name                 *string
			public               *bool
			destinationId        pgtypeuuid.UUID
			destinationName      *string
			destinationSpaceId   pgtypeuuid.UUID
			destinationSpaceName *string
			destinationSpaceMap  *string
			packageId            pgtypeuuid.UUID
			packageName          *string
			packageTitle         *string
			previewId            pgtypeuuid.UUID
			previewUrl           *string
			previewType          *string
			previewMime          *string
			previewSize          *int64
			pakId                pgtypeuuid.UUID
			pakUrl               *string
			pakType              *string
			pakMime              *string
			pakSize              *int64
		)

		err = rows.Scan(
			&id,
			&name,
			&public,
			&destinationId,
			&destinationName,
			&destinationSpaceId,
			&destinationSpaceName,
			&destinationSpaceMap,
			&packageId,
			&packageName,
			&packageTitle,
			&previewId,
			&previewUrl,
			&previewType,
			&previewMime,
			&previewSize,
			&pakId,
			&pakUrl,
			&pakType,
			&pakMime,
			&pakSize,
		)
		if err != nil {
			return nil, -1, err
		}

		if id.Status == pgtype.Null {
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
		}

		var preview *File
		if previewId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &previewId.UUID
			if previewUrl != nil {
				preview.Url = *previewUrl
			}

			if previewType != nil {
				preview.Type = *previewType
			}

			if previewMime != nil {
				preview.Mime = previewMime
			}

			if previewSize != nil {
				preview.Size = previewSize
			}
		}

		if i := findPortal(entities, id.UUID); i >= 0 {
			if preview != nil && !containsFile(entities[i].Destination.Files, *preview.Id) {
				if entities[i].Destination != nil {
					entities[i].Destination.Files = append(entities[i].Destination.Files, *preview)
				}
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

			var e Portal
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if destinationId.Status != pgtype.Null {
				if e.Destination == nil {
					e.Destination = new(Portal)
				}
				e.Destination.Id = &destinationId.UUID
				if destinationName != nil {
					e.Destination.Name = *destinationName
				}
				if preview != nil {
					e.Destination.Files = append(e.Files, *preview)
				}
				if destinationSpaceId.Status != pgtype.Null {
					if e.Destination.World == nil {
						e.Destination.World = new(World)
					}
					e.Destination.World.Id = &destinationSpaceId.UUID
					if destinationSpaceName != nil {
						e.Destination.World.Name = *destinationSpaceName
					}
					if destinationSpaceMap != nil {
						e.Destination.World.Map = *destinationSpaceMap
					}
					if packageId.Status != pgtype.Null {
						if e.Destination.World.Package == nil {
							e.Destination.World.Package = new(Package)
						}
						e.Destination.World.Package.Id = &packageId.UUID
						if packageName != nil {
							e.Destination.World.Package.Name = *packageName
						}
						if packageTitle != nil {
							e.Destination.World.Package.Title = *packageTitle
						}
						if pak != nil {
							if e.Destination.World.Package == nil {
								e.Destination.World.Package = new(Package)
							}
							e.Destination.World.Package.Files = append(e.Files, *pak)
						}
					}
				}
			}
			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexPortalsForAdminForWorldWithQueryWithPak Index portals
func IndexPortalsForAdminForWorldWithQueryWithPak(ctx context.Context, spaceId uuid.UUID, platform string, deployment string, query string, offset int64, limit int64) (entities []Portal, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) 
FROM portals p
    LEFT JOIN portals d ON p.destination_id = d.id
	LEFT JOIN spaces s ON p.space_id = s.id
	LEFT JOIN mods m ON s.mod_id = m.id
WHERE p.space_id = $1 AND (
    p.name ILIKE $1::text OR
    d.name ILIKE $1::text OR
    s.name ILIKE $1::text OR
    m.name ILIKE $1::text)`

	row := db.QueryRow(ctx, q, spaceId /*$1*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	q = `SELECT
	p.id portalId,
	p.name portalName,
	e.public entityPublic,
	d.id destinationId,
	d.name destinationName,
	s.id spaceId,
	s.name spaceName,
	s.map spaceMap,
	m.id packageId,
	m.name packageName,
	m.title packageTitle,
	preview.id previewId,
	preview.url previewUrl,
	preview.type previewType,
	preview.mime previewMime,
    preview.size previewSize,
	pak.id pakId,
	pak.url pakUrl,
	pak.type pakType,
	pak.mime pakMime,
	pak.size pakSize
FROM portals p
	LEFT JOIN entities e on p.id = e.id
	-- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON de.id = d.id
    LEFT JOIN files preview ON de.id = preview.entity_id AND preview.type = 'rendertarget_preview'
	-- destination space
    LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
	-- destination package
    LEFT JOIN mods m ON m.id = s.mod_id
	LEFT JOIN entities me ON me.id = m.id
	LEFT JOIN files pak ON me.id = pak.entity_id AND pak.type = 'pak' AND pak.platform = $1::text AND pak.deployment_type = $2::text
WHERE p.space_id = $3 AND (
    p.name ILIKE $4::text OR
    d.name ILIKE $4::text OR
    s.name ILIKE $4::text OR
    m.name ILIKE $4::text)
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows      pgx.Rows
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, platform /*$1*/, deployment /*$2*/, spaceId /*$3*/, query /*$4*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPortalsForAdminForWorldWithQueryWithPak")
	}()
	for rows.Next() {
		var (
			id                   pgtypeuuid.UUID
			name                 *string
			public               *bool
			destinationId        pgtypeuuid.UUID
			destinationName      *string
			destinationSpaceId   pgtypeuuid.UUID
			destinationSpaceName *string
			destinationSpaceMap  *string
			packageId            pgtypeuuid.UUID
			packageName          *string
			packageTitle         *string
			previewId            pgtypeuuid.UUID
			previewUrl           *string
			previewType          *string
			previewMime          *string
			previewSize          *int64
			pakId                pgtypeuuid.UUID
			pakUrl               *string
			pakType              *string
			pakMime              *string
			pakSize              *int64
		)

		err = rows.Scan(
			&id,
			&name,
			&public,
			&destinationId,
			&destinationName,
			&destinationSpaceId,
			&destinationSpaceName,
			&destinationSpaceMap,
			&packageId,
			&packageName,
			&packageTitle,
			&previewId,
			&previewUrl,
			&previewType,
			&previewMime,
			&previewSize,
			&pakId,
			&pakUrl,
			&pakType,
			&pakMime,
			&pakSize,
		)
		if err != nil {
			return nil, -1, err
		}

		if id.Status == pgtype.Null {
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
		}

		var preview *File
		if previewId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &previewId.UUID
			if previewUrl != nil {
				preview.Url = *previewUrl
			}

			if previewType != nil {
				preview.Type = *previewType
			}

			if previewMime != nil {
				preview.Mime = previewMime
			}

			if previewSize != nil {
				preview.Size = previewSize
			}
		}

		if i := findPortal(entities, id.UUID); i >= 0 {
			if preview != nil && !containsFile(entities[i].Destination.Files, *preview.Id) {
				if entities[i].Destination != nil {
					entities[i].Destination.Files = append(entities[i].Destination.Files, *preview)
				}
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

			var e Portal
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if destinationId.Status != pgtype.Null {
				if e.Destination == nil {
					e.Destination = new(Portal)
				}
				e.Destination.Id = &destinationId.UUID
				if destinationName != nil {
					e.Destination.Name = *destinationName
				}
				if preview != nil {
					e.Destination.Files = append(e.Files, *preview)
				}
				if destinationSpaceId.Status != pgtype.Null {
					if e.Destination.World == nil {
						e.Destination.World = new(World)
					}
					e.Destination.World.Id = &destinationSpaceId.UUID
					if destinationSpaceName != nil {
						e.Destination.World.Name = *destinationSpaceName
					}
					if destinationSpaceMap != nil {
						e.Destination.World.Map = *destinationSpaceMap
					}
					if packageId.Status != pgtype.Null {
						if e.Destination.World.Package == nil {
							e.Destination.World.Package = new(Package)
						}
						e.Destination.World.Package.Id = &packageId.UUID
						if packageName != nil {
							e.Destination.World.Package.Name = *packageName
						}
						if packageTitle != nil {
							e.Destination.World.Package.Title = *packageTitle
						}
						if pak != nil {
							if e.Destination.World.Package == nil {
								e.Destination.World.Package = new(Package)
							}
							e.Destination.World.Package.Files = append(e.Files, *pak)
						}
					}
				}
			}
			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexPortalsForAdminWithQuery Index portals
func IndexPortalsForAdminWithQuery(ctx context.Context, query string, offset int64, limit int64) (entities []Portal, total int64, err error) {
	db := database.DB

	//region Count
	q := `SELECT COUNT(*) FROM portals p
    LEFT JOIN portals d ON p.destination_id = d.id
	LEFT JOIN spaces s ON p.space_id = s.id 
	LEFT JOIN mods m ON s.mod_id = m.id
WHERE p.name ILIKE $1::text OR
      d.name ILIKE $1::text OR
      s.name ILIKE $1::text OR
      m.name ILIKE $1::text`

	row := db.QueryRow(ctx, q, query /*$1*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	q = `SELECT
	p.id portalId,
	p.name portalName,
	e.public entityPublic,
	d.id destinationId,
	d.name destinationName,
	s.id spaceId,
	s.name spaceName,
	s.map spaceMap,
	m.id packageId,
	m.name packageName,
	m.title packageTitle,
	preview.id previewId,
	preview.url previewUrl,
	preview.type previewType,
	preview.mime previewMime,
	preview.size previewSize
FROM portals p
	LEFT JOIN entities e on p.id = e.id
	-- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON de.id = d.id
    LEFT JOIN files preview ON de.id = preview.entity_id AND preview.type = 'rendertarget_preview'
	-- destination space
    LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
	-- destination package
    LEFT JOIN mods m ON m.id = s.mod_id
	LEFT JOIN entities me ON me.id = m.id
WHERE p.name ILIKE $1::text OR
      d.name ILIKE $1::text OR
      s.name ILIKE $1::text OR
      m.name ILIKE $1::text
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows      pgx.Rows
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, query /*$1*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPortalsForAdminWithQuery")
	}()
	for rows.Next() {
		var (
			id                   pgtypeuuid.UUID
			name                 *string
			public               *bool
			destinationId        pgtypeuuid.UUID
			destinationName      *string
			destinationSpaceId   pgtypeuuid.UUID
			destinationSpaceName *string
			destinationSpaceMap  *string
			packageId            pgtypeuuid.UUID
			packageName          *string
			packageTitle         *string
			previewId            pgtypeuuid.UUID
			previewUrl           *string
			previewType          *string
			previewMime          *string
			previewSize          *int64
		)

		err = rows.Scan(
			&id,
			&name,
			&public,
			&destinationId,
			&destinationName,
			&destinationSpaceId,
			&destinationSpaceName,
			&destinationSpaceMap,
			&packageId,
			&packageName,
			&packageTitle,
			&previewId,
			&previewUrl,
			&previewType,
			&previewMime,
			&previewSize,
		)
		if err != nil {
			return nil, -1, err
		}

		if id.Status == pgtype.Null {
			continue
		}

		var preview *File
		if previewId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &previewId.UUID
			if previewUrl != nil {
				preview.Url = *previewUrl
			}

			if previewType != nil {
				preview.Type = *previewType
			}

			if previewMime != nil {
				preview.Mime = previewMime
			}

			if previewSize != nil {
				preview.Size = previewSize
			}
		}

		if i := findPortal(entities, id.UUID); i >= 0 {
			if preview != nil && !containsFile(entities[i].Destination.Files, *preview.Id) {
				if entities[i].Destination != nil {
					entities[i].Destination.Files = append(entities[i].Destination.Files, *preview)
				}
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

			var e Portal
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if destinationId.Status != pgtype.Null {
				if e.Destination == nil {
					e.Destination = new(Portal)
				}
				e.Destination.Id = &destinationId.UUID
				if destinationName != nil {
					e.Destination.Name = *destinationName
				}
				if preview != nil {
					e.Destination.Files = append(e.Files, *preview)
				}
				if destinationSpaceId.Status != pgtype.Null {
					if e.Destination.World == nil {
						e.Destination.World = new(World)
					}
					e.Destination.World.Id = &destinationSpaceId.UUID
					if destinationSpaceName != nil {
						e.Destination.World.Name = *destinationSpaceName
					}
					if destinationSpaceMap != nil {
						e.Destination.World.Map = *destinationSpaceMap
					}
					if packageId.Status != pgtype.Null {
						if e.Destination.World.Package == nil {
							e.Destination.World.Package = new(Package)
						}
						e.Destination.World.Package.Id = &packageId.UUID
						if packageName != nil {
							e.Destination.World.Package.Name = *packageName
						}
						if packageTitle != nil {
							e.Destination.World.Package.Title = *packageTitle
						}
					}
				}
			}
			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexPortalsForAdminForWorldWithQuery Index portals
func IndexPortalsForAdminForWorldWithQuery(ctx context.Context, spaceId uuid.UUID, query string, offset int64, limit int64) (entities []Portal, total int64, err error) {
	db := database.DB

	//region Count
	q := `SELECT COUNT(*) 
FROM portals p 
    LEFT JOIN portals d ON p.destination_id = d.id
	LEFT JOIN spaces s ON p.space_id = s.id 
	LEFT JOIN mods m ON s.mod_id = m.id
WHERE p.space_id = $1 AND (
    p.name ILIKE $2::text OR
    d.name ILIKE $2::text OR
    s.name ILIKE $2::text OR
    m.name ILIKE $2::text)`

	row := db.QueryRow(ctx, q, spaceId /*$1*/, query /*$2*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	q = `SELECT
	p.id portalId,
	p.name portalName,
	e.public entityPublic,
	d.id destinationId,
	d.name destinationName,
	s.id spaceId,
	s.name spaceName,
	s.map spaceMap,
	m.id packageId,
	m.name packageName,
	m.title packageTitle,
	preview.id previewId,
	preview.url previewUrl,
	preview.type previewType,
	preview.mime previewMime,
	preview.size previewSize
FROM portals p
	LEFT JOIN entities e on p.id = e.id
	-- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON de.id = d.id
    LEFT JOIN files preview ON de.id = preview.entity_id AND preview.type = 'rendertarget_preview'
	-- destination space
    LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
	-- destination package
    LEFT JOIN mods m ON m.id = s.mod_id
	LEFT JOIN entities me ON me.id = m.id
WHERE p.space_id = $1 AND (
      p.name ILIKE $2::text OR
      d.name ILIKE $2::text OR
      s.name ILIKE $2::text OR
      m.name ILIKE $2::text)
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows      pgx.Rows
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, spaceId /*$1*/, query /*$2*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPortalsForAdminForWorldWithQuery")
	}()
	for rows.Next() {
		var (
			id                   pgtypeuuid.UUID
			name                 *string
			public               *bool
			destinationId        pgtypeuuid.UUID
			destinationName      *string
			destinationSpaceId   pgtypeuuid.UUID
			destinationSpaceName *string
			destinationSpaceMap  *string
			packageId            pgtypeuuid.UUID
			packageName          *string
			packageTitle         *string
			previewId            pgtypeuuid.UUID
			previewUrl           *string
			previewType          *string
			previewMime          *string
			previewSize          *int64
		)

		err = rows.Scan(
			&id,
			&name,
			&public,
			&destinationId,
			&destinationName,
			&destinationSpaceId,
			&destinationSpaceName,
			&destinationSpaceMap,
			&packageId,
			&packageName,
			&packageTitle,
			&previewId,
			&previewUrl,
			&previewType,
			&previewMime,
			&previewSize,
		)
		if err != nil {
			return nil, -1, err
		}

		if id.Status == pgtype.Null {
			continue
		}

		var preview *File
		if previewId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &previewId.UUID
			if previewUrl != nil {
				preview.Url = *previewUrl
			}

			if previewType != nil {
				preview.Type = *previewType
			}

			if previewMime != nil {
				preview.Mime = previewMime
			}

			if previewSize != nil {
				preview.Size = previewSize
			}
		}

		if i := findPortal(entities, id.UUID); i >= 0 {
			if preview != nil && !containsFile(entities[i].Destination.Files, *preview.Id) {
				if entities[i].Destination != nil {
					entities[i].Destination.Files = append(entities[i].Destination.Files, *preview)
				}
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

			var e Portal
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if destinationId.Status != pgtype.Null {
				if e.Destination == nil {
					e.Destination = new(Portal)
				}
				e.Destination.Id = &destinationId.UUID
				if destinationName != nil {
					e.Destination.Name = *destinationName
				}
				if preview != nil {
					e.Destination.Files = append(e.Files, *preview)
				}
				if destinationSpaceId.Status != pgtype.Null {
					if e.Destination.World == nil {
						e.Destination.World = new(World)
					}
					e.Destination.World.Id = &destinationSpaceId.UUID
					if destinationSpaceName != nil {
						e.Destination.World.Name = *destinationSpaceName
					}
					if destinationSpaceMap != nil {
						e.Destination.World.Map = *destinationSpaceMap
					}
					if packageId.Status != pgtype.Null {
						if e.Destination.World.Package == nil {
							e.Destination.World.Package = new(Package)
						}
						e.Destination.World.Package.Id = &packageId.UUID
						if packageName != nil {
							e.Destination.World.Package.Name = *packageName
						}
						if packageTitle != nil {
							e.Destination.World.Package.Title = *packageTitle
						}
					}
				}
			}
			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexPortalsForRequesterWithPak Index portals
func IndexPortalsForRequesterWithPak(ctx context.Context, requester *sm.User, platform string, deployment string, offset int64, limit int64) (entities []Portal, total int64, err error) {
	db := database.DB

	//region Count
	q := `SELECT COUNT(*) 
FROM portals p
    LEFT JOIN entities e ON p.id = e.id
    LEFT JOIN accessibles a ON p.id = a.entity_id AND a.user_id = $1::uuid
    -- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON d.id = de.id
    LEFT JOIN accessibles da ON de.id = da.entity_id AND da.user_id = $1::uuid
    -- destination space
	LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
    LEFT JOIN accessibles sa ON se.id = sa.entity_id  AND sa.user_id = $1::uuid
    -- destination package
	LEFT JOIN mods m ON m.id = s.mod_id
	LEFT JOIN entities me ON me.id = m.id
    LEFT JOIN accessibles ma ON me.id = ma.entity_id AND ma.user_id = $1::uuid
WHERE (e.public OR a.can_view OR a.is_owner) AND
      (de.public OR da.can_view OR da.is_owner) AND
      (se.public OR sa.can_view OR sa.is_owner) AND
      (me.public OR ma.can_view OR ma.is_owner)`

	row := db.QueryRow(ctx, q, requester.Id /*$1*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	q = `SELECT
	p.id portalId,
	p.name portalName,
	e.public entityPublic,
	d.id destinationId,
	d.name destinationName,
	s.id spaceId,
	s.name spaceName,
	s.map spaceMap,
	m.id packageId,
	m.name packageName,
	m.title packageTitle,
	preview.id previewId,
	preview.url previewUrl,
	preview.type previewType,
	preview.mime previewMime,
    preview.size previewSize,
	pak.id pakId,
	pak.url pakUrl,
	pak.type pakType,
	pak.mime pakMime,
	pak.size pakSize
FROM portals p
	LEFT JOIN entities e on p.id = e.id
    LEFT JOIN accessibles a ON a.entity_id = p.id AND a.user_id = $1::uuid
	-- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON de.id = d.id
    LEFT JOIN accessibles da ON de.id = da.entity_id AND a.user_id = $1::uuid
    LEFT JOIN files preview ON de.id = preview.entity_id AND preview.type = 'rendertarget_preview'
	-- destination space
    LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
    LEFT JOIN accessibles sa ON se.id = sa.entity_id AND a.user_id = $1::uuid
	-- destination package
    LEFT JOIN mods m ON m.id = s.mod_id
	LEFT JOIN entities me ON me.id = m.id
    LEFT JOIN accessibles ma ON me.id = ma.entity_id AND a.user_id = $1::uuid
	LEFT JOIN files pak ON me.id = pak.entity_id AND pak.type = 'pak' AND pak.platform = $2::text AND pak.deployment_type = $3::text
WHERE (e.public OR a.can_view OR a.is_owner)
  AND (de.public OR da.can_view OR da.is_owner)
  AND (se.public OR sa.can_view OR sa.is_owner)
  AND (me.public OR ma.can_view OR ma.is_owner)
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows      pgx.Rows
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, requester.Id /*$1*/, platform /*$2*/, deployment /*$3*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPortalsForRequesterWithPak")
	}()
	for rows.Next() {
		var (
			id                   pgtypeuuid.UUID
			name                 *string
			public               *bool
			destinationId        pgtypeuuid.UUID
			destinationName      *string
			destinationSpaceId   pgtypeuuid.UUID
			destinationSpaceName *string
			destinationSpaceMap  *string
			packageId            pgtypeuuid.UUID
			packageName          *string
			packageTitle         *string
			previewId            pgtypeuuid.UUID
			previewUrl           *string
			previewType          *string
			previewMime          *string
			previewSize          *int64
			pakId                pgtypeuuid.UUID
			pakUrl               *string
			pakType              *string
			pakMime              *string
			pakSize              *int64
		)

		err = rows.Scan(
			&id,
			&name,
			&public,
			&destinationId,
			&destinationName,
			&destinationSpaceId,
			&destinationSpaceName,
			&destinationSpaceMap,
			&packageId,
			&packageName,
			&packageTitle,
			&previewId,
			&previewUrl,
			&previewType,
			&previewMime,
			&previewSize,
			&pakId,
			&pakUrl,
			&pakType,
			&pakMime,
			&pakSize,
		)
		if err != nil {
			return nil, -1, err
		}

		if id.Status == pgtype.Null {
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
		}

		var preview *File
		if previewId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &previewId.UUID
			if previewUrl != nil {
				preview.Url = *previewUrl
			}

			if previewType != nil {
				preview.Type = *previewType
			}

			if previewMime != nil {
				preview.Mime = previewMime
			}

			if previewSize != nil {
				preview.Size = previewSize
			}
		}

		if i := findPortal(entities, id.UUID); i >= 0 {
			if preview != nil && !containsFile(entities[i].Destination.Files, *preview.Id) {
				if entities[i].Destination != nil {
					entities[i].Destination.Files = append(entities[i].Destination.Files, *preview)
				}
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

			var e Portal
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if destinationId.Status != pgtype.Null {
				if e.Destination == nil {
					e.Destination = new(Portal)
				}
				e.Destination.Id = &destinationId.UUID
				if destinationName != nil {
					e.Destination.Name = *destinationName
				}
				if preview != nil {
					e.Destination.Files = append(e.Files, *preview)
				}
				if destinationSpaceId.Status != pgtype.Null {
					if e.Destination.World == nil {
						e.Destination.World = new(World)
					}
					e.Destination.World.Id = &destinationSpaceId.UUID
					if destinationSpaceName != nil {
						e.Destination.World.Name = *destinationSpaceName
					}
					if destinationSpaceMap != nil {
						e.Destination.World.Map = *destinationSpaceMap
					}
					if packageId.Status != pgtype.Null {
						if e.Destination.World.Package == nil {
							e.Destination.World.Package = new(Package)
						}
						e.Destination.World.Package.Id = &packageId.UUID
						if packageName != nil {
							e.Destination.World.Package.Name = *packageName
						}
						if packageTitle != nil {
							e.Destination.World.Package.Title = *packageTitle
						}
						if pak != nil {
							if e.Destination.World.Package == nil {
								e.Destination.World.Package = new(Package)
							}
							e.Destination.World.Package.Files = append(e.Files, *pak)
						}
					}
				}
			}
			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexPortalsForRequesterForWorldWithPak Index portals
func IndexPortalsForRequesterForWorldWithPak(ctx context.Context, requester *sm.User, spaceId uuid.UUID, platform string, deployment string, offset int64, limit int64) (entities []Portal, total int64, err error) {
	db := database.DB

	//region Count
	q := `SELECT COUNT(*)
FROM portals p
    LEFT JOIN entities e ON p.id = e.id
    LEFT JOIN accessibles a ON p.id = a.entity_id AND a.user_id = $1::uuid
    -- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON d.id = de.id
    LEFT JOIN accessibles da ON de.id = da.entity_id AND da.user_id = $1::uuid
    -- destination space
	LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
    LEFT JOIN accessibles sa ON se.id = sa.entity_id  AND sa.user_id = $1::uuid
    -- destination package
	LEFT JOIN mods m ON m.id = s.mod_id
	LEFT JOIN entities me ON me.id = m.id
    LEFT JOIN accessibles ma ON me.id = ma.entity_id AND ma.user_id = $1::uuid
WHERE p.space_id = $2 AND
      (e.public OR a.can_view OR a.is_owner) AND
      (de.public OR da.can_view OR da.is_owner) AND
      (se.public OR sa.can_view OR sa.is_owner) AND
      (me.public OR ma.can_view OR ma.is_owner)`

	row := db.QueryRow(ctx, q, requester.Id /*$1*/, spaceId /*$2*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	q = `SELECT
	p.id portalId,
	p.name portalName,
	e.public entityPublic,
	d.id destinationId,
	d.name destinationName,
	s.id spaceId,
	s.name spaceName,
	s.map spaceMap,
	m.id packageId,
	m.name packageName,
	m.title packageTitle,
	preview.id previewId,
	preview.url previewUrl,
	preview.type previewType,
	preview.mime previewMime,
    preview.size previewSize,
	pak.id pakId,
	pak.url pakUrl,
	pak.type pakType,
	pak.mime pakMime,
	pak.size pakSize
FROM portals p
	LEFT JOIN entities e on p.id = e.id
    LEFT JOIN accessibles a ON a.entity_id = p.id AND a.user_id = $1::uuid
	-- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON de.id = d.id
    LEFT JOIN accessibles da ON de.id = da.entity_id AND a.user_id = $1::uuid
    LEFT JOIN files preview ON de.id = preview.entity_id AND preview.type = 'rendertarget_preview'
	-- destination space
    LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
    LEFT JOIN accessibles sa ON se.id = sa.entity_id AND a.user_id = $1::uuid
	-- destination package
    LEFT JOIN mods m ON m.id = s.mod_id
	LEFT JOIN entities me ON me.id = m.id
    LEFT JOIN accessibles ma ON me.id = ma.entity_id AND a.user_id = $1::uuid
	LEFT JOIN files pak ON me.id = pak.entity_id AND pak.type = 'pak' AND pak.platform = $2::text AND pak.deployment_type = $3::text
WHERE p.space_id = $4 AND
      (e.public OR a.can_view OR a.is_owner) AND
      (de.public OR da.can_view OR da.is_owner) AND
      (se.public OR sa.can_view OR sa.is_owner) AND
      (me.public OR ma.can_view OR ma.is_owner)
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows      pgx.Rows
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, requester.Id /*$1*/, spaceId /*$2*/, platform /*$3*/, deployment /*$4*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPortalsForRequesterForWorldWithPak")
	}()
	for rows.Next() {
		var (
			id                   pgtypeuuid.UUID
			name                 *string
			public               *bool
			destinationId        pgtypeuuid.UUID
			destinationName      *string
			destinationSpaceId   pgtypeuuid.UUID
			destinationSpaceName *string
			destinationSpaceMap  *string
			packageId            pgtypeuuid.UUID
			packageName          *string
			packageTitle         *string
			previewId            pgtypeuuid.UUID
			previewUrl           *string
			previewType          *string
			previewMime          *string
			previewSize          *int64
			pakId                pgtypeuuid.UUID
			pakUrl               *string
			pakType              *string
			pakMime              *string
			pakSize              *int64
		)

		err = rows.Scan(
			&id,
			&name,
			&public,
			&destinationId,
			&destinationName,
			&destinationSpaceId,
			&destinationSpaceName,
			&destinationSpaceMap,
			&packageId,
			&packageName,
			&packageTitle,
			&previewId,
			&previewUrl,
			&previewType,
			&previewMime,
			&previewSize,
			&pakId,
			&pakUrl,
			&pakType,
			&pakMime,
			&pakSize,
		)
		if err != nil {
			return nil, -1, err
		}

		if id.Status == pgtype.Null {
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
		}

		var preview *File
		if previewId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &previewId.UUID
			if previewUrl != nil {
				preview.Url = *previewUrl
			}

			if previewType != nil {
				preview.Type = *previewType
			}

			if previewMime != nil {
				preview.Mime = previewMime
			}

			if previewSize != nil {
				preview.Size = previewSize
			}
		}

		if i := findPortal(entities, id.UUID); i >= 0 {
			if preview != nil && !containsFile(entities[i].Destination.Files, *preview.Id) {
				if entities[i].Destination != nil {
					entities[i].Destination.Files = append(entities[i].Destination.Files, *preview)
				}
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

			var e Portal
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if destinationId.Status != pgtype.Null {
				if e.Destination == nil {
					e.Destination = new(Portal)
				}
				e.Destination.Id = &destinationId.UUID
				if destinationName != nil {
					e.Destination.Name = *destinationName
				}
				if preview != nil {
					e.Destination.Files = append(e.Files, *preview)
				}
				if destinationSpaceId.Status != pgtype.Null {
					if e.Destination.World == nil {
						e.Destination.World = new(World)
					}
					e.Destination.World.Id = &destinationSpaceId.UUID
					if destinationSpaceName != nil {
						e.Destination.World.Name = *destinationSpaceName
					}
					if destinationSpaceMap != nil {
						e.Destination.World.Map = *destinationSpaceMap
					}
					if packageId.Status != pgtype.Null {
						if e.Destination.World.Package == nil {
							e.Destination.World.Package = new(Package)
						}
						e.Destination.World.Package.Id = &packageId.UUID
						if packageName != nil {
							e.Destination.World.Package.Name = *packageName
						}
						if packageTitle != nil {
							e.Destination.World.Package.Title = *packageTitle
						}
						if pak != nil {
							if e.Destination.World.Package == nil {
								e.Destination.World.Package = new(Package)
							}
							e.Destination.World.Package.Files = append(e.Files, *pak)
						}
					}
				}
			}
			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexPortalsForRequester Index portals
func IndexPortalsForRequester(ctx context.Context, requester *sm.User, offset int64, limit int64) (entities []Portal, total int64, err error) {
	db := database.DB

	//region Count
	q := `SELECT COUNT(*) 
FROM portals p
    LEFT JOIN entities e ON p.id = e.id
    LEFT JOIN accessibles a ON p.id = a.entity_id AND a.user_id = $1::uuid
    -- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON d.id = de.id
    LEFT JOIN accessibles da ON de.id = da.entity_id AND da.user_id = $1::uuid
    -- destination space
	LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
    LEFT JOIN accessibles sa ON se.id = sa.entity_id  AND sa.user_id = $1::uuid
    -- destination package
	LEFT JOIN mods m ON m.id = s.mod_id
	LEFT JOIN entities me ON me.id = m.id
    LEFT JOIN accessibles ma ON me.id = ma.entity_id AND ma.user_id = $1::uuid
WHERE (e.public OR a.can_view OR a.is_owner) AND
      (de.public OR da.can_view OR da.is_owner) AND
      (se.public OR sa.can_view OR sa.is_owner) AND
      (me.public OR ma.can_view OR ma.is_owner)`

	row := db.QueryRow(ctx, q, requester.Id /*$1*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	q = `SELECT
	p.id portalId,
	p.name portalName,
	e.public entityPublic,
	d.id destinationId,
	d.name destinationName,
	s.id spaceId,
	s.name spaceName,
	s.map spaceMap,
	m.id packageId,
	m.name packageName,
	m.title packageTitle,
	preview.id previewId,
	preview.url previewUrl,
	preview.type previewType,
	preview.mime previewMime,
	preview.size previewSize
FROM portals p
	LEFT JOIN entities e on p.id = e.id
    LEFT JOIN accessibles a ON a.entity_id = p.id AND a.user_id = $1::uuid
	-- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON de.id = d.id
    LEFT JOIN accessibles da ON de.id = da.entity_id AND a.user_id = $1::uuid
    LEFT JOIN files preview ON de.id = preview.entity_id AND preview.type = 'rendertarget_preview'
	-- destination space
    LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
    LEFT JOIN accessibles sa ON se.id = sa.entity_id AND a.user_id = $1::uuid
	-- destination mod
    LEFT JOIN mods m ON m.id = s.mod_id
	LEFT JOIN entities me ON me.id = m.id
    LEFT JOIN accessibles ma ON me.id = ma.entity_id AND a.user_id = $1::uuid
WHERE (e.public OR a.can_view OR a.is_owner) AND
      (de.public OR da.can_view OR da.is_owner) AND
      (se.public OR sa.can_view OR sa.is_owner) AND
      (me.public OR ma.can_view OR ma.is_owner)
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows      pgx.Rows
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, requester.Id /*$1*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPortalsForRequester")
	}()
	for rows.Next() {
		var (
			id                   pgtypeuuid.UUID
			name                 *string
			public               *bool
			destinationId        pgtypeuuid.UUID
			destinationName      *string
			destinationSpaceId   pgtypeuuid.UUID
			destinationSpaceName *string
			destinationSpaceMap  *string
			packageId            pgtypeuuid.UUID
			packageName          *string
			packageTitle         *string
			previewId            pgtypeuuid.UUID
			previewUrl           *string
			previewType          *string
			previewMime          *string
			previewSize          *int64
		)

		err = rows.Scan(
			&id,
			&name,
			&public,
			&destinationId,
			&destinationName,
			&destinationSpaceId,
			&destinationSpaceName,
			&destinationSpaceMap,
			&packageId,
			&packageName,
			&packageTitle,
			&previewId,
			&previewUrl,
			&previewType,
			&previewMime,
			&previewSize,
		)
		if err != nil {
			return nil, -1, err
		}

		if id.Status == pgtype.Null {
			continue
		}

		var preview *File
		if previewId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &previewId.UUID
			if previewUrl != nil {
				preview.Url = *previewUrl
			}

			if previewType != nil {
				preview.Type = *previewType
			}

			if previewMime != nil {
				preview.Mime = previewMime
			}

			if previewSize != nil {
				preview.Size = previewSize
			}
		}

		if i := findPortal(entities, id.UUID); i >= 0 {
			if preview != nil && !containsFile(entities[i].Destination.Files, *preview.Id) {
				if entities[i].Destination != nil {
					entities[i].Destination.Files = append(entities[i].Destination.Files, *preview)
				}
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

			var e Portal
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if destinationId.Status != pgtype.Null {
				if e.Destination == nil {
					e.Destination = new(Portal)
				}
				e.Destination.Id = &destinationId.UUID
				if destinationName != nil {
					e.Destination.Name = *destinationName
				}
				if preview != nil {
					e.Destination.Files = append(e.Files, *preview)
				}
				if destinationSpaceId.Status != pgtype.Null {
					if e.Destination.World == nil {
						e.Destination.World = new(World)
					}
					e.Destination.World.Id = &destinationSpaceId.UUID
					if destinationSpaceName != nil {
						e.Destination.World.Name = *destinationSpaceName
					}
					if destinationSpaceMap != nil {
						e.Destination.World.Map = *destinationSpaceMap
					}
					if packageId.Status != pgtype.Null {
						if e.Destination.World.Package == nil {
							e.Destination.World.Package = new(Package)
						}
						e.Destination.World.Package.Id = &packageId.UUID
						if packageName != nil {
							e.Destination.World.Package.Name = *packageName
						}
						if packageTitle != nil {
							e.Destination.World.Package.Title = *packageTitle
						}
					}
				}
			}
			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexPortalsForRequesterForWorld Index portals
func IndexPortalsForRequesterForWorld(ctx context.Context, requester *sm.User, spaceId uuid.UUID, offset int64, limit int64) (entities []Portal, total int64, err error) {
	db := database.DB

	//region Count
	q := `SELECT COUNT(*) 
FROM portals p
    LEFT JOIN entities e ON p.id = e.id
    LEFT JOIN accessibles a ON p.id = a.entity_id AND a.user_id = $1::uuid
    -- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON d.id = de.id
    LEFT JOIN accessibles da ON de.id = da.entity_id AND da.user_id = $1::uuid
    -- destination space
	LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
    LEFT JOIN accessibles sa ON se.id = sa.entity_id  AND sa.user_id = $1::uuid
    -- destination package
	LEFT JOIN mods m ON m.id = s.mod_id
	LEFT JOIN entities me ON me.id = m.id
    LEFT JOIN accessibles ma ON me.id = ma.entity_id AND ma.user_id = $1::uuid
WHERE p.space_id = $2 AND
      (e.public OR a.can_view OR a.is_owner) AND
      (de.public OR da.can_view OR da.is_owner) AND
      (se.public OR sa.can_view OR sa.is_owner) AND
      (me.public OR ma.can_view OR ma.is_owner)`

	row := db.QueryRow(ctx, q, requester.Id /*$1*/, spaceId /*$2*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	q = `SELECT
	p.id portalId,
	p.name portalName,
	e.public entityPublic,
	d.id destinationId,
	d.name destinationName,
	s.id spaceId,
	s.name spaceName,
	s.map spaceMap,
	m.id packageId,
	m.name packageName,
	m.title packageTitle,
	preview.id previewId,
	preview.url previewUrl,
	preview.type previewType,
	preview.mime previewMime,
	preview.size previewSize
FROM portals p
	LEFT JOIN entities e on p.id = e.id
    LEFT JOIN accessibles a ON a.entity_id = p.id AND a.user_id = $1::uuid
	-- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON de.id = d.id
    LEFT JOIN accessibles da ON de.id = da.entity_id AND a.user_id = $1::uuid
    LEFT JOIN files preview ON de.id = preview.entity_id AND preview.type = 'rendertarget_preview'
	-- destination space
    LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
    LEFT JOIN accessibles sa ON se.id = sa.entity_id AND a.user_id = $1::uuid
	-- destination package
    LEFT JOIN mods m ON m.id = s.mod_id
	LEFT JOIN entities me ON me.id = m.id
    LEFT JOIN accessibles ma ON me.id = ma.entity_id AND a.user_id = $1::uuid
WHERE p.space_id = $2
  AND (e.public OR a.can_view OR a.is_owner)
  AND (de.public OR da.can_view OR da.is_owner)
  AND (se.public OR sa.can_view OR sa.is_owner)
  AND (me.public OR ma.can_view OR ma.is_owner)
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows      pgx.Rows
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, requester.Id /*$1*/, spaceId /*$2*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPortalsForRequesterForWorld")
	}()
	for rows.Next() {
		var (
			id                   pgtypeuuid.UUID
			name                 *string
			public               *bool
			destinationId        pgtypeuuid.UUID
			destinationName      *string
			destinationSpaceId   pgtypeuuid.UUID
			destinationSpaceName *string
			destinationSpaceMap  *string
			packageId            pgtypeuuid.UUID
			packageName          *string
			packageTitle         *string
			previewId            pgtypeuuid.UUID
			previewUrl           *string
			previewType          *string
			previewMime          *string
			previewSize          *int64
		)

		err = rows.Scan(
			&id,
			&name,
			&public,
			&destinationId,
			&destinationName,
			&destinationSpaceId,
			&destinationSpaceName,
			&destinationSpaceMap,
			&packageId,
			&packageName,
			&packageTitle,
			&previewId,
			&previewUrl,
			&previewType,
			&previewMime,
			&previewSize,
		)
		if err != nil {
			return nil, -1, err
		}

		if id.Status == pgtype.Null {
			continue
		}

		var preview *File
		if previewId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &previewId.UUID
			if previewUrl != nil {
				preview.Url = *previewUrl
			}

			if previewType != nil {
				preview.Type = *previewType
			}

			if previewMime != nil {
				preview.Mime = previewMime
			}

			if previewSize != nil {
				preview.Size = previewSize
			}
		}

		if i := findPortal(entities, id.UUID); i >= 0 {
			if preview != nil && !containsFile(entities[i].Destination.Files, *preview.Id) {
				if entities[i].Destination != nil {
					entities[i].Destination.Files = append(entities[i].Destination.Files, *preview)
				}
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

			var e Portal
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if destinationId.Status != pgtype.Null {
				if e.Destination == nil {
					e.Destination = new(Portal)
				}
				e.Destination.Id = &destinationId.UUID
				if destinationName != nil {
					e.Destination.Name = *destinationName
				}
				if preview != nil {
					e.Destination.Files = append(e.Files, *preview)
				}
				if destinationSpaceId.Status != pgtype.Null {
					if e.Destination.World == nil {
						e.Destination.World = new(World)
					}
					e.Destination.World.Id = &destinationSpaceId.UUID
					if destinationSpaceName != nil {
						e.Destination.World.Name = *destinationSpaceName
					}
					if destinationSpaceMap != nil {
						e.Destination.World.Map = *destinationSpaceMap
					}
					if packageId.Status != pgtype.Null {
						if e.Destination.World.Package == nil {
							e.Destination.World.Package = new(Package)
						}
						e.Destination.World.Package.Id = &packageId.UUID
						if packageName != nil {
							e.Destination.World.Package.Name = *packageName
						}
						if packageTitle != nil {
							e.Destination.World.Package.Title = *packageTitle
						}
					}
				}
			}
			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexPortalsForRequesterWithQueryWithPak Index portals
func IndexPortalsForRequesterWithQueryWithPak(ctx context.Context, requester *sm.User, platform string, deployment string, query string, offset int64, limit int64) (entities []Portal, total int64, err error) {
	db := database.DB

	//region Count
	q := `SELECT COUNT(*)
FROM portals p
    LEFT JOIN entities e ON p.id = e.id
    LEFT JOIN accessibles a ON p.id = a.entity_id AND a.user_id = $1::uuid
    -- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON d.id = de.id
    LEFT JOIN accessibles da ON de.id = da.entity_id AND da.user_id = $1::uuid
    -- destination space
	LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
    LEFT JOIN accessibles sa ON se.id = sa.entity_id  AND sa.user_id = $1::uuid
    -- destination package
	LEFT JOIN mods m ON m.id = s.mod_id
	LEFT JOIN entities me ON me.id = m.id
    LEFT JOIN accessibles ma ON me.id = ma.entity_id AND ma.user_id = $1::uuid
WHERE (e.public OR a.can_view OR a.is_owner) AND
      (de.public OR da.can_view OR da.is_owner) AND
      (se.public OR sa.can_view OR sa.is_owner) AND
      (me.public OR ma.can_view OR ma.is_owner) AND 
      (p.name ILIKE $2::text OR
       d.name ILIKE $2::text OR
       s.name ILIKE $2::text OR
       m.name ILIKE $2::text)`

	row := db.QueryRow(ctx, q, requester.Id /*$1*/, query /*$2*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	q = `SELECT
	p.id portalId,
	p.name portalName,
	e.public entityPublic,
	d.id destinationId,
	d.name destinationName,
	s.id spaceId,
	s.name spaceName,
	s.map spaceMap,
	m.id packageId,
	m.name packageName,
	m.title packageTitle,
	preview.id previewId,
	preview.url previewUrl,
	preview.type previewType,
	preview.mime previewMime,
    preview.size previewSize,
	pak.id pakId,
	pak.url pakUrl,
	pak.type pakType,
	pak.mime pakMime,
	pak.size pakSize
FROM portals p
	LEFT JOIN entities e on p.id = e.id
    LEFT JOIN accessibles a ON a.entity_id = p.id AND a.user_id = $1::uuid
	-- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON de.id = d.id
    LEFT JOIN accessibles da ON de.id = da.entity_id 
    LEFT JOIN files preview ON de.id = preview.entity_id AND preview.type = 'rendertarget_preview'
	-- destination space
    LEFT JOIN spaces s ON s.id = d.space_id 
	LEFT JOIN entities se ON se.id = s.id
    LEFT JOIN accessibles sa ON se.id = sa.entity_id 
	-- destination mod
    LEFT JOIN mods m ON m.id = s.mod_id
	LEFT JOIN entities me ON me.id = m.id
    LEFT JOIN accessibles ma ON me.id = ma.entity_id
	LEFT JOIN files pak ON me.id = pak.entity_id AND pak.type = 'pak' AND pak.platform = $2::text AND pak.deployment_type = $3::text
WHERE (e.public OR a.can_view OR a.is_owner)
  AND (de.public OR da.can_view OR da.is_owner)
  AND (se.public OR sa.can_view OR sa.is_owner)
  AND (me.public OR ma.can_view OR ma.is_owner)
  AND (p.name ILIKE $4::text
   OR d.name ILIKE $4::text
   OR s.name ILIKE $4::text
   OR m.name ILIKE $4::text)
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows      pgx.Rows
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, requester.Id /*$1*/, platform /*$2*/, deployment /*$3*/, query /*$4*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPortalsForRequesterWithQueryWithPak")
	}()
	for rows.Next() {
		var (
			id                   pgtypeuuid.UUID
			name                 *string
			public               *bool
			destinationId        pgtypeuuid.UUID
			destinationName      *string
			destinationSpaceId   pgtypeuuid.UUID
			destinationSpaceName *string
			destinationSpaceMap  *string
			packageId            pgtypeuuid.UUID
			packageName          *string
			packageTitle         *string
			previewId            pgtypeuuid.UUID
			previewUrl           *string
			previewType          *string
			previewMime          *string
			previewSize          *int64
			pakId                pgtypeuuid.UUID
			pakUrl               *string
			pakType              *string
			pakMime              *string
			pakSize              *int64
		)

		err = rows.Scan(
			&id,
			&name,
			&public,
			&destinationId,
			&destinationName,
			&destinationSpaceId,
			&destinationSpaceName,
			&destinationSpaceMap,
			&packageId,
			&packageName,
			&packageTitle,
			&previewId,
			&previewUrl,
			&previewType,
			&previewMime,
			&previewSize,
			&pakId,
			&pakUrl,
			&pakType,
			&pakMime,
			&pakSize,
		)
		if err != nil {
			return nil, -1, err
		}

		if id.Status == pgtype.Null {
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
		}

		var preview *File
		if previewId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &previewId.UUID
			if previewUrl != nil {
				preview.Url = *previewUrl
			}

			if previewType != nil {
				preview.Type = *previewType
			}

			if previewMime != nil {
				preview.Mime = previewMime
			}

			if previewSize != nil {
				preview.Size = previewSize
			}
		}

		if i := findPortal(entities, id.UUID); i >= 0 {
			if preview != nil && !containsFile(entities[i].Destination.Files, *preview.Id) {
				if entities[i].Destination != nil {
					entities[i].Destination.Files = append(entities[i].Destination.Files, *preview)
				}
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

			var e Portal
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if destinationId.Status != pgtype.Null {
				if e.Destination == nil {
					e.Destination = new(Portal)
				}
				e.Destination.Id = &destinationId.UUID
				if destinationName != nil {
					e.Destination.Name = *destinationName
				}
				if preview != nil {
					e.Destination.Files = append(e.Files, *preview)
				}
				if destinationSpaceId.Status != pgtype.Null {
					if e.Destination.World == nil {
						e.Destination.World = new(World)
					}
					e.Destination.World.Id = &destinationSpaceId.UUID
					if destinationSpaceName != nil {
						e.Destination.World.Name = *destinationSpaceName
					}
					if destinationSpaceMap != nil {
						e.Destination.World.Map = *destinationSpaceMap
					}
					if packageId.Status != pgtype.Null {
						if e.Destination.World.Package == nil {
							e.Destination.World.Package = new(Package)
						}
						e.Destination.World.Package.Id = &packageId.UUID
						if packageName != nil {
							e.Destination.World.Package.Name = *packageName
						}
						if packageTitle != nil {
							e.Destination.World.Package.Title = *packageTitle
						}
						if pak != nil {
							if e.Destination.World.Package == nil {
								e.Destination.World.Package = new(Package)
							}
							e.Destination.World.Package.Files = append(e.Files, *pak)
						}
					}
				}
			}
			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexPortalsForRequesterForWorldWithQueryWithPak Index portals
func IndexPortalsForRequesterForWorldWithQueryWithPak(ctx context.Context, requester *sm.User, spaceId uuid.UUID, platform string, deployment string, query string, offset int64, limit int64) (entities []Portal, total int64, err error) {
	db := database.DB

	//region Count
	q := `SELECT COUNT(*)
FROM portals p
    LEFT JOIN entities e ON p.id = e.id
    LEFT JOIN accessibles a ON p.id = a.entity_id AND a.user_id = $1::uuid
    -- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON d.id = de.id
    LEFT JOIN accessibles da ON de.id = da.entity_id AND da.user_id = $1::uuid
    -- destination space
	LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
    LEFT JOIN accessibles sa ON se.id = sa.entity_id  AND sa.user_id = $1::uuid
    -- destination package
	LEFT JOIN mods m ON m.id = s.mod_id
	LEFT JOIN entities me ON me.id = m.id
    LEFT JOIN accessibles ma ON me.id = ma.entity_id AND ma.user_id = $1::uuid
WHERE p.space_id = $2 AND
      (e.public OR a.can_view OR a.is_owner) AND
      (de.public OR da.can_view OR da.is_owner) AND
      (se.public OR sa.can_view OR sa.is_owner) AND
      (me.public OR ma.can_view OR ma.is_owner) AND 
      (p.name ILIKE $3::text OR
       d.name ILIKE $3::text OR
       s.name ILIKE $3::text OR
       m.name ILIKE $3::text)`

	row := db.QueryRow(ctx, q, requester.Id /*$1*/, spaceId /*$2*/, query /*$3*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	q = `SELECT
	p.id portalId,
	p.name portalName,
	e.public entityPublic,
	d.id destinationId,
	d.name destinationName,
	s.id spaceId,
	s.name spaceName,
	s.map spaceMap,
	m.id packageId,
	m.name packageName,
	m.title packageTitle,
	preview.id previewId,
	preview.url previewUrl,
	preview.type previewType,
	preview.mime previewMime,
    preview.size previewSize,
	pak.id pakId,
	pak.url pakUrl,
	pak.type pakType,
	pak.mime pakMime,
	pak.size pakSize
FROM portals p
	LEFT JOIN entities e on p.id = e.id
    LEFT JOIN accessibles a ON a.entity_id = p.id AND a.user_id = $1::uuid
	-- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON de.id = d.id
    LEFT JOIN accessibles da ON de.id = da.entity_id 
    LEFT JOIN files preview ON de.id = preview.entity_id AND preview.type = 'rendertarget_preview'
	-- destination space
    LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
    LEFT JOIN accessibles sa ON se.id = sa.entity_id 
	-- destination mod
    LEFT JOIN mods m ON m.id = s.mod_id
	LEFT JOIN entities me ON me.id = m.id
    LEFT JOIN accessibles ma ON me.id = ma.entity_id
	LEFT JOIN files pak ON me.id = pak.entity_id AND pak.type = 'pak' AND pak.platform = $2::text AND pak.deployment_type = $3::text
WHERE p.space_id = $4 
  AND (e.public OR a.can_view OR a.is_owner)
  AND (de.public OR da.can_view OR da.is_owner)
  AND (se.public OR sa.can_view OR sa.is_owner)
  AND (me.public OR ma.can_view OR ma.is_owner)
  AND (p.name ILIKE $5::text
   OR d.name ILIKE $5::text
   OR s.name ILIKE $5::text
   OR m.name ILIKE $5::text)
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows      pgx.Rows
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, requester.Id /*$1*/, platform /*$2*/, deployment /*$3*/, spaceId /*$4*/, query /*$5*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPortalsForRequesterForWorldWithQueryWithPak")
	}()
	for rows.Next() {
		var (
			id                   pgtypeuuid.UUID
			name                 *string
			public               *bool
			destinationId        pgtypeuuid.UUID
			destinationName      *string
			destinationSpaceId   pgtypeuuid.UUID
			destinationSpaceName *string
			destinationSpaceMap  *string
			packageId            pgtypeuuid.UUID
			packageName          *string
			packageTitle         *string
			previewId            pgtypeuuid.UUID
			previewUrl           *string
			previewType          *string
			previewMime          *string
			previewSize          *int64
			pakId                pgtypeuuid.UUID
			pakUrl               *string
			pakType              *string
			pakMime              *string
			pakSize              *int64
		)

		err = rows.Scan(
			&id,
			&name,
			&public,
			&destinationId,
			&destinationName,
			&destinationSpaceId,
			&destinationSpaceName,
			&destinationSpaceMap,
			&packageId,
			&packageName,
			&packageTitle,
			&previewId,
			&previewUrl,
			&previewType,
			&previewMime,
			&previewSize,
			&pakId,
			&pakUrl,
			&pakType,
			&pakMime,
			&pakSize,
		)
		if err != nil {
			return nil, -1, err
		}

		if id.Status == pgtype.Null {
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
		}

		var preview *File
		if previewId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &previewId.UUID
			if previewUrl != nil {
				preview.Url = *previewUrl
			}

			if previewType != nil {
				preview.Type = *previewType
			}

			if previewMime != nil {
				preview.Mime = previewMime
			}

			if previewSize != nil {
				preview.Size = previewSize
			}
		}

		if i := findPortal(entities, id.UUID); i >= 0 {
			if preview != nil && !containsFile(entities[i].Destination.Files, *preview.Id) {
				if entities[i].Destination != nil {
					entities[i].Destination.Files = append(entities[i].Destination.Files, *preview)
				}
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

			var e Portal
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if destinationId.Status != pgtype.Null {
				if e.Destination == nil {
					e.Destination = new(Portal)
				}
				e.Destination.Id = &destinationId.UUID
				if destinationName != nil {
					e.Destination.Name = *destinationName
				}
				if preview != nil {
					e.Destination.Files = append(e.Files, *preview)
				}
				if destinationSpaceId.Status != pgtype.Null {
					if e.Destination.World == nil {
						e.Destination.World = new(World)
					}
					e.Destination.World.Id = &destinationSpaceId.UUID
					if destinationSpaceName != nil {
						e.Destination.World.Name = *destinationSpaceName
					}
					if destinationSpaceMap != nil {
						e.Destination.World.Map = *destinationSpaceMap
					}
					if packageId.Status != pgtype.Null {
						if e.Destination.World.Package == nil {
							e.Destination.World.Package = new(Package)
						}
						e.Destination.World.Package.Id = &packageId.UUID
						if packageName != nil {
							e.Destination.World.Package.Name = *packageName
						}
						if packageTitle != nil {
							e.Destination.World.Package.Title = *packageTitle
						}
						if pak != nil {
							if e.Destination.World.Package == nil {
								e.Destination.World.Package = new(Package)
							}
							e.Destination.World.Package.Files = append(e.Files, *pak)
						}
					}
				}
			}
			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexPortalsForRequesterWithQuery Index portals
func IndexPortalsForRequesterWithQuery(ctx context.Context, requester *sm.User, query string, offset int64, limit int64) (entities []Portal, total int64, err error) {
	db := database.DB

	//region Count
	q := `SELECT COUNT(*)
FROM portals p
    LEFT JOIN entities e ON p.id = e.id
    LEFT JOIN accessibles a ON p.id = a.entity_id AND a.user_id = $1::uuid
    -- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON d.id = de.id
    LEFT JOIN accessibles da ON d.id = da.entity_id AND da.user_id = $1::uuid
    -- destination space
	LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
    LEFT JOIN accessibles sa ON s.id = sa.entity_id  AND sa.user_id = $1::uuid
    -- destination package
	LEFT JOIN mods m ON m.id = s.mod_id
	LEFT JOIN entities me ON me.id = m.id
    LEFT JOIN accessibles ma ON m.id = ma.entity_id AND ma.user_id = $1::uuid
WHERE (e.public OR a.can_view OR a.is_owner) AND
      (de.public OR da.can_view OR da.is_owner) AND
      (se.public OR sa.can_view OR sa.is_owner) AND
      (me.public OR ma.can_view OR ma.is_owner) AND 
      (p.name ILIKE $2::text OR
       d.name ILIKE $2::text OR
       s.name ILIKE $2::text OR
       m.name ILIKE $2::text)`

	row := db.QueryRow(ctx, q, requester.Id /*$1*/, query /*$2*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	q = `SELECT
	p.id portalId,
	p.name portalName,
	e.public entityPublic,
	d.id destinationId,
	d.name destinationName,
	s.id spaceId,
	s.name spaceName,
	s.map spaceMap,
	m.id packageId,
	m.name packageName,
	m.title packageTitle,
	preview.id previewId,
	preview.url previewUrl,
	preview.type previewType,
	preview.mime previewMime,
	preview.size previewSize
FROM portals p
	LEFT JOIN entities e on p.id = e.id
    LEFT JOIN accessibles a ON a.entity_id = p.id AND a.user_id = $1::uuid
	-- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON de.id = d.id
    LEFT JOIN accessibles da ON da.entity_id = d.id AND a.user_id = $1::uuid
    LEFT JOIN files preview ON de.id = preview.entity_id AND preview.type = 'rendertarget_preview'
	-- destination space
    LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
    LEFT JOIN accessibles sa ON sa.entity_id = s.id AND a.user_id = $1::uuid
	-- destination mod
    LEFT JOIN mods m ON m.id = s.mod_id
	LEFT JOIN entities me ON me.id = m.id
    LEFT JOIN accessibles ma ON me.id = ma.entity_id AND a.user_id = $1::uuid
WHERE (e.public OR a.can_view OR a.is_owner)
  AND (de.public OR da.can_view OR da.is_owner)
  AND (se.public OR sa.can_view OR sa.is_owner)
  AND (me.public OR ma.can_view OR ma.is_owner)
  AND (p.name ILIKE $2::text
   OR d.name ILIKE $2::text
   OR s.name ILIKE $2::text
   OR m.name ILIKE $2::text)
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows      pgx.Rows
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, requester.Id /*$1*/, query /*$2*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPortalsForRequesterWithQuery")
	}()
	for rows.Next() {
		var (
			id                   pgtypeuuid.UUID
			name                 *string
			public               *bool
			destinationId        pgtypeuuid.UUID
			destinationName      *string
			destinationSpaceId   pgtypeuuid.UUID
			destinationSpaceName *string
			destinationSpaceMap  *string
			packageId            pgtypeuuid.UUID
			packageName          *string
			packageTitle         *string
			previewId            pgtypeuuid.UUID
			previewUrl           *string
			previewType          *string
			previewMime          *string
			previewSize          *int64
		)

		err = rows.Scan(
			&id,
			&name,
			&public,
			&destinationId,
			&destinationName,
			&destinationSpaceId,
			&destinationSpaceName,
			&destinationSpaceMap,
			&packageId,
			&packageName,
			&packageTitle,
			&previewId,
			&previewUrl,
			&previewType,
			&previewMime,
			&previewSize,
		)
		if err != nil {
			return nil, -1, err
		}

		if id.Status == pgtype.Null {
			continue
		}

		var preview *File
		if previewId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &previewId.UUID
			if previewUrl != nil {
				preview.Url = *previewUrl
			}

			if previewType != nil {
				preview.Type = *previewType
			}

			if previewMime != nil {
				preview.Mime = previewMime
			}

			if previewSize != nil {
				preview.Size = previewSize
			}
		}

		if i := findPortal(entities, id.UUID); i >= 0 {
			if preview != nil && !containsFile(entities[i].Destination.Files, *preview.Id) {
				if entities[i].Destination != nil {
					entities[i].Destination.Files = append(entities[i].Destination.Files, *preview)
				}
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

			var e Portal
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if destinationId.Status != pgtype.Null {
				if e.Destination == nil {
					e.Destination = new(Portal)
				}
				e.Destination.Id = &destinationId.UUID
				if destinationName != nil {
					e.Destination.Name = *destinationName
				}
				if preview != nil {
					e.Destination.Files = append(e.Files, *preview)
				}
				if destinationSpaceId.Status != pgtype.Null {
					if e.Destination.World == nil {
						e.Destination.World = new(World)
					}
					e.Destination.World.Id = &destinationSpaceId.UUID
					if destinationSpaceName != nil {
						e.Destination.World.Name = *destinationSpaceName
					}
					if destinationSpaceMap != nil {
						e.Destination.World.Map = *destinationSpaceMap
					}
					if packageId.Status != pgtype.Null {
						if e.Destination.World.Package == nil {
							e.Destination.World.Package = new(Package)
						}
						e.Destination.World.Package.Id = &packageId.UUID
						if packageName != nil {
							e.Destination.World.Package.Name = *packageName
						}
						if packageTitle != nil {
							e.Destination.World.Package.Title = *packageTitle
						}
					}
				}
			}
			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// IndexPortalsForRequesterForWorldWithQuery Index portals
func IndexPortalsForRequesterForWorldWithQuery(ctx context.Context, requester *sm.User, spaceId uuid.UUID, query string, offset int64, limit int64) (entities []Portal, total int64, err error) {
	db := database.DB

	//region Count
	q := `SELECT COUNT(*)
FROM portals p
    LEFT JOIN entities e ON p.id = e.id
    LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
    -- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON d.id = de.id
    LEFT JOIN accessibles da ON de.id = da.entity_id AND da.user_id = $1::uuid
    -- destination space
	LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
    LEFT JOIN accessibles sa ON se.id = sa.entity_id AND sa.user_id = $1::uuid
    -- destination package
	LEFT JOIN mods m ON m.id = s.mod_id
	LEFT JOIN entities me ON me.id = m.id
    LEFT JOIN accessibles ma ON me.id = ma.entity_id AND ma.user_id = $1::uuid
WHERE p.space_id = $2 AND 
      (e.public OR a.can_view OR a.is_owner) AND
      (de.public OR da.can_view OR da.is_owner) AND
      (se.public OR sa.can_view OR sa.is_owner) AND
      (me.public OR ma.can_view OR ma.is_owner) AND 
      (p.name ILIKE $3::text OR
       d.name ILIKE $3::text OR
       s.name ILIKE $3::text OR
       m.name ILIKE $3::text)`

	row := db.QueryRow(ctx, q, requester.Id /*$1*/, spaceId /*$2*/, query /*$3*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	q = `SELECT
	p.id portalId,
	p.name portalName,
	e.public entityPublic,
	d.id destinationId,
	d.name destinationName,
	s.id spaceId,
	s.name spaceName,
	s.map spaceMap,
	m.id packageId,
	m.name packageName,
	m.title packageTitle,
	preview.id previewId,
	preview.url previewUrl,
	preview.type previewType,
	preview.mime previewMime,
	preview.size previewSize
FROM portals p
	LEFT JOIN entities e on p.id = e.id
    LEFT JOIN accessibles a ON a.entity_id = p.id AND a.user_id = $1::uuid
	-- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON de.id = d.id
    LEFT JOIN accessibles da ON da.entity_id = d.id
    LEFT JOIN files preview ON de.id = preview.entity_id AND preview.type = 'rendertarget_preview'
	-- destination space
    LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
    LEFT JOIN accessibles sa ON sa.entity_id = s.id
	-- destination mod
    LEFT JOIN mods m ON m.id = s.mod_id
	LEFT JOIN entities me ON me.id = m.id
    LEFT JOIN accessibles ma ON me.id = ma.entity_id
WHERE p.space_id = $2 AND
      (e.public OR a.can_view OR a.is_owner) AND
      (de.public OR da.can_view OR da.is_owner) AND
      (se.public OR sa.can_view OR sa.is_owner) AND
      (me.public OR ma.can_view OR ma.is_owner) AND
      (p.name ILIKE $3::text OR
       d.name ILIKE $3::text OR
       s.name ILIKE $3::text OR
       m.name ILIKE $3::text)
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows      pgx.Rows
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q, requester.Id /*$1*/, spaceId /*$2*/, query /*$3*/)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexPortalsForRequesterForWorldWithQuery")
	}()
	for rows.Next() {
		var (
			id                   pgtypeuuid.UUID
			name                 *string
			public               *bool
			destinationId        pgtypeuuid.UUID
			destinationName      *string
			destinationSpaceId   pgtypeuuid.UUID
			destinationSpaceName *string
			destinationSpaceMap  *string
			packageId            pgtypeuuid.UUID
			packageName          *string
			packageTitle         *string
			previewId            pgtypeuuid.UUID
			previewUrl           *string
			previewType          *string
			previewMime          *string
			previewSize          *int64
		)

		err = rows.Scan(
			&id,
			&name,
			&public,
			&destinationId,
			&destinationName,
			&destinationSpaceId,
			&destinationSpaceName,
			&destinationSpaceMap,
			&packageId,
			&packageName,
			&packageTitle,
			&previewId,
			&previewUrl,
			&previewType,
			&previewMime,
			&previewSize,
		)
		if err != nil {
			return nil, -1, err
		}

		if id.Status == pgtype.Null {
			continue
		}

		var preview *File
		if previewId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &previewId.UUID
			if previewUrl != nil {
				preview.Url = *previewUrl
			}

			if previewType != nil {
				preview.Type = *previewType
			}

			if previewMime != nil {
				preview.Mime = previewMime
			}

			if previewSize != nil {
				preview.Size = previewSize
			}
		}

		if i := findPortal(entities, id.UUID); i >= 0 {
			if preview != nil && !containsFile(entities[i].Destination.Files, *preview.Id) {
				if entities[i].Destination != nil {
					entities[i].Destination.Files = append(entities[i].Destination.Files, *preview)
				}
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

			var e Portal
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if destinationId.Status != pgtype.Null {
				if e.Destination == nil {
					e.Destination = new(Portal)
				}
				e.Destination.Id = &destinationId.UUID
				if destinationName != nil {
					e.Destination.Name = *destinationName
				}
				if preview != nil {
					e.Destination.Files = append(e.Files, *preview)
				}
				if destinationSpaceId.Status != pgtype.Null {
					if e.Destination.World == nil {
						e.Destination.World = new(World)
					}
					e.Destination.World.Id = &destinationSpaceId.UUID
					if destinationSpaceName != nil {
						e.Destination.World.Name = *destinationSpaceName
					}
					if destinationSpaceMap != nil {
						e.Destination.World.Map = *destinationSpaceMap
					}
					if packageId.Status != pgtype.Null {
						if e.Destination.World.Package == nil {
							e.Destination.World.Package = new(Package)
						}
						e.Destination.World.Package.Id = &packageId.UUID
						if packageName != nil {
							e.Destination.World.Package.Name = *packageName
						}
						if packageTitle != nil {
							e.Destination.World.Package.Title = *packageTitle
						}
					}
				}
			}
			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// GetPortalForAdminWithPak Get portal
func GetPortalForAdminWithPak(ctx context.Context, id uuid.UUID, platform string, deployment string) (entity *Portal, err error) {
	db := database.DB

	q := `SELECT
	p.id portalId,
	p.name portalName,
	e.public entityPublic,
	d.id destinationId,
	d.name destinationName,
	s.id spaceId,
	s.name spaceName,
	s.map spaceMap,
	m.id packageId,
	m.name packageName,
	m.title packageTitle,
	preview.id previewId,
	preview.url previewUrl,
	preview.type previewType,
	preview.mime previewMime,
    preview.size previewSize,
	pak.id pakId,
	pak.url pakUrl,
	pak.type pakType,
	pak.mime pakMime,
	pak.size pakSize
FROM portals p
	LEFT JOIN entities e on p.id = e.id
	-- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON de.id = d.id
    LEFT JOIN files preview ON de.id = preview.entity_id AND preview.type = 'rendertarget_preview'
	-- destination space
    LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
	-- destination package
    LEFT JOIN mods m ON m.id = s.mod_id 
	LEFT JOIN entities me ON me.id = m.id
	LEFT JOIN files pak ON me.id = pak.entity_id AND pak.type = 'pak' AND pak.platform = $1::text AND pak.deployment_type = $2::text
WHERE p.id = $3
ORDER BY e.id`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, platform /*$1*/, deployment /*$2*/, id /*$3*/)
	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetPortalForAdminWithPak")
	}()
	for rows.Next() {
		var (
			portalId        pgtypeuuid.UUID
			name            *string
			public          *bool
			destinationId   pgtypeuuid.UUID
			destinationName *string
			spaceId         pgtypeuuid.UUID
			spaceName       *string
			spaceMap        *string
			packageId       pgtypeuuid.UUID
			packageName     *string
			packageTitle    *string
			previewId       pgtypeuuid.UUID
			previewUrl      *string
			previewType     *string
			previewMime     *string
			previewSize     *int64
			pakId           pgtypeuuid.UUID
			pakUrl          *string
			pakType         *string
			pakMime         *string
			pakSize         *int64
		)

		err = rows.Scan(
			&portalId,
			&name,
			&public,
			&destinationId,
			&destinationName,
			&spaceId,
			&spaceName,
			&spaceMap,
			&packageId,
			&packageName,
			&packageTitle,
			&previewId,
			&previewUrl,
			&previewType,
			&previewMime,
			&previewSize,
			&pakId,
			&pakUrl,
			&pakType,
			&pakMime,
			&pakSize,
		)
		if err != nil {
			return nil, err
		}

		if portalId.Status == pgtype.Null {
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
		}

		var preview *File
		if previewId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &previewId.UUID
			if previewUrl != nil {
				preview.Url = *previewUrl
			}

			if previewType != nil {
				preview.Type = *previewType
			}

			if previewMime != nil {
				preview.Mime = previewMime
			}

			if previewSize != nil {
				preview.Size = previewSize
			}
		}

		if entity != nil {
			if preview != nil && !containsFile(entity.Destination.Files, *preview.Id) {
				if entity.Destination != nil {
					entity.Destination.Files = append(entity.Destination.Files, *preview)
				}
			}
		} else {
			var e Portal
			e.Id = &portalId.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if destinationId.Status != pgtype.Null {
				if e.Destination == nil {
					e.Destination = new(Portal)
				}
				e.Destination.Id = &destinationId.UUID
				if destinationName != nil {
					e.Destination.Name = *destinationName
				}
				if preview != nil {
					e.Destination.Files = append(e.Files, *preview)
				}
				if spaceId.Status != pgtype.Null {
					if e.Destination.World == nil {
						e.Destination.World = new(World)
					}
					e.Destination.World.Id = &spaceId.UUID
					if spaceName != nil {
						e.Destination.World.Name = *spaceName
					}
					if spaceMap != nil {
						e.Destination.World.Map = *spaceMap
					}
					if packageId.Status != pgtype.Null {
						if e.Destination.World.Package == nil {
							e.Destination.World.Package = new(Package)
						}
						e.Destination.World.Package.Id = &packageId.UUID
						if packageName != nil {
							e.Destination.World.Package.Name = *packageName
						}
						if packageTitle != nil {
							e.Destination.World.Package.Title = *packageTitle
						}
						if pak != nil {
							if e.Destination.World.Package == nil {
								e.Destination.World.Package = new(Package)
							}
							e.Destination.World.Package.Files = append(e.Files, *pak)
						}
					}
				}
			}
			entity = &e
		}
	}

	return entity, err
}

// GetPortalForAdmin Get portal
func GetPortalForAdmin(ctx context.Context, id uuid.UUID) (entity *Portal, err error) {
	db := database.DB

	q := `SELECT
	p.id portalId,
	p.name portalName,
	e.public entityPublic,
	d.id destinationId,
	d.name destinationName,
	s.id spaceId,
	s.name spaceName,
	s.map spaceMap,
	m.id packageId,
	m.name packageName,
	m.title packageTitle,
	preview.id previewId,
	preview.url previewUrl,
	preview.type previewType,
	preview.mime previewMime,
	preview.size previewSize
FROM portals p
	LEFT JOIN entities e on p.id = e.id
	-- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON de.id = d.id
    LEFT JOIN files preview ON de.id = preview.entity_id AND preview.type = 'rendertarget_preview'
	-- destination space
    LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
	-- destination package
    LEFT JOIN mods m ON m.id = s.mod_id 
	LEFT JOIN entities me ON me.id = m.id
WHERE p.id = $1
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
		database.LogPgxStat("GetPortalForAdmin")
	}()
	for rows.Next() {
		var (
			portalId        pgtypeuuid.UUID
			name            *string
			public          *bool
			destinationId   pgtypeuuid.UUID
			destinationName *string
			spaceId         pgtypeuuid.UUID
			spaceName       *string
			spaceMap        *string
			packageId       pgtypeuuid.UUID
			packageName     *string
			packageTitle    *string
			previewId       pgtypeuuid.UUID
			previewUrl      *string
			previewType     *string
			previewMime     *string
			previewSize     *int64
		)

		err = rows.Scan(
			&portalId,
			&name,
			&public,
			&destinationId,
			&destinationName,
			&spaceId,
			&spaceName,
			&spaceMap,
			&packageId,
			&packageName,
			&packageTitle,
			&previewId,
			&previewUrl,
			&previewType,
			&previewMime,
			&previewSize,
		)
		if err != nil {
			return nil, err
		}

		if portalId.Status == pgtype.Null {
			continue
		}

		var preview *File
		if previewId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &previewId.UUID
			if previewUrl != nil {
				preview.Url = *previewUrl
			}

			if previewType != nil {
				preview.Type = *previewType
			}

			if previewMime != nil {
				preview.Mime = previewMime
			}

			if previewSize != nil {
				preview.Size = previewSize
			}
		}

		if entity != nil {
			if preview != nil && !containsFile(entity.Destination.Files, *preview.Id) {
				if entity.Destination != nil {
					entity.Destination.Files = append(entity.Destination.Files, *preview)
				}
			}
		} else {
			var e Portal
			e.Id = &portalId.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if destinationId.Status != pgtype.Null {
				if e.Destination == nil {
					e.Destination = new(Portal)
				}
				e.Destination.Id = &destinationId.UUID
				if destinationName != nil {
					e.Destination.Name = *destinationName
				}
				if preview != nil {
					e.Destination.Files = append(e.Files, *preview)
				}
				if spaceId.Status != pgtype.Null {
					if e.Destination.World == nil {
						e.Destination.World = new(World)
					}
					e.Destination.World.Id = &spaceId.UUID
					if spaceName != nil {
						e.Destination.World.Name = *spaceName
					}
					if spaceMap != nil {
						e.Destination.World.Map = *spaceMap
					}
					if packageId.Status != pgtype.Null {
						if e.Destination.World.Package == nil {
							e.Destination.World.Package = new(Package)
						}
						e.Destination.World.Package.Id = &packageId.UUID
						if packageName != nil {
							e.Destination.World.Package.Name = *packageName
						}
						if packageTitle != nil {
							e.Destination.World.Package.Title = *packageTitle
						}
					}
				}
			}
			entity = &e
		}
	}

	return entity, err
}

// GetPortalForRequesterWithPak Get portal
func GetPortalForRequesterWithPak(ctx context.Context, requester *sm.User, id uuid.UUID, platform string, deployment string) (entity *Portal, err error) {
	db := database.DB

	q := `SELECT
	p.id portalId,
	p.name portalName,
	e.public entityPublic,
	d.id destinationId,
	d.name destinationName,
	s.id spaceId,
	s.name spaceName,
	s.map spaceMap,
	m.id packageId,
	m.name packageName,
	m.title packageTitle,
	preview.id previewId,
	preview.url previewUrl,
	preview.type previewType,
	preview.mime previewMime,
    preview.size previewSize,
	pak.id pakId,
	pak.url pakUrl,
	pak.type pakType,
	pak.mime pakMime,
	pak.size pakSize
FROM portals p
	LEFT JOIN entities e on p.id = e.id
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
	-- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON de.id = d.id
    LEFT JOIN accessibles da ON de.id = da.entity_id AND da.user_id = $1::uuid
    LEFT JOIN files preview ON de.id = preview.entity_id AND preview.type = 'rendertarget_preview'
	-- destination space
    LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
	LEFT JOIN accessibles sa ON se.id = sa.entity_id AND sa.user_id = $1::uuid
	-- destination package
    LEFT JOIN mods m ON m.id = s.mod_id 
	LEFT JOIN entities me ON me.id = m.id
    LEFT JOIN accessibles ma ON me.id = ma.entity_id AND ma.user_id = $1::uuid
	LEFT JOIN files pak ON me.id = pak.entity_id AND pak.type = 'pak' AND pak.platform = $2::text AND pak.deployment_type = $3::text
WHERE p.id = $4
ORDER BY e.id`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, requester.Id /*$1*/, platform /*$2*/, deployment /*$3*/, id /*$4*/)
	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetPortalForRequesterWithPak")
	}()
	for rows.Next() {
		var (
			portalId        pgtypeuuid.UUID
			name            *string
			public          *bool
			destinationId   pgtypeuuid.UUID
			destinationName *string
			spaceId         pgtypeuuid.UUID
			spaceName       *string
			spaceMap        *string
			packageId       pgtypeuuid.UUID
			packageName     *string
			packageTitle    *string
			previewId       pgtypeuuid.UUID
			previewUrl      *string
			previewType     *string
			previewMime     *string
			previewSize     *int64
			pakId           pgtypeuuid.UUID
			pakUrl          *string
			pakType         *string
			pakMime         *string
			pakSize         *int64
		)

		err = rows.Scan(
			&portalId,
			&name,
			&public,
			&destinationId,
			&destinationName,
			&spaceId,
			&spaceName,
			&spaceMap,
			&packageId,
			&packageName,
			&packageTitle,
			&previewId,
			&previewUrl,
			&previewType,
			&previewMime,
			&previewSize,
			&pakId,
			&pakUrl,
			&pakType,
			&pakMime,
			&pakSize,
		)
		if err != nil {
			return nil, err
		}

		if portalId.Status == pgtype.Null {
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
		}

		var preview *File
		if previewId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &previewId.UUID
			if previewUrl != nil {
				preview.Url = *previewUrl
			}

			if previewType != nil {
				preview.Type = *previewType
			}

			if previewMime != nil {
				preview.Mime = previewMime
			}

			if previewSize != nil {
				preview.Size = previewSize
			}
		}

		if entity != nil {
			if preview != nil && !containsFile(entity.Destination.Files, *preview.Id) {
				if entity.Destination != nil {
					entity.Destination.Files = append(entity.Destination.Files, *preview)
				}
			}
		} else {
			var e Portal
			e.Id = &portalId.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if destinationId.Status != pgtype.Null {
				if e.Destination == nil {
					e.Destination = new(Portal)
				}
				e.Destination.Id = &destinationId.UUID
				if destinationName != nil {
					e.Destination.Name = *destinationName
				}
				if preview != nil {
					e.Destination.Files = append(e.Files, *preview)
				}
				if spaceId.Status != pgtype.Null {
					if e.Destination.World == nil {
						e.Destination.World = new(World)
					}
					e.Destination.World.Id = &spaceId.UUID
					if spaceName != nil {
						e.Destination.World.Name = *spaceName
					}
					if spaceMap != nil {
						e.Destination.World.Map = *spaceMap
					}
					if packageId.Status != pgtype.Null {
						if e.Destination.World.Package == nil {
							e.Destination.World.Package = new(Package)
						}
						e.Destination.World.Package.Id = &packageId.UUID
						if packageName != nil {
							e.Destination.World.Package.Name = *packageName
						}
						if packageTitle != nil {
							e.Destination.World.Package.Title = *packageTitle
						}
						if pak != nil {
							if e.Destination.World.Package == nil {
								e.Destination.World.Package = new(Package)
							}
							e.Destination.World.Package.Files = append(e.Files, *pak)
						}
					}
				}
			}
			entity = &e
		}
	}

	return entity, err
}

// GetPortalForRequester Get portal
func GetPortalForRequester(ctx context.Context, requester *sm.User, id uuid.UUID) (entity *Portal, err error) {
	db := database.DB

	q := `SELECT
	p.id portalId,
	p.name portalName,
	e.public entityPublic,
	d.id destinationId,
	d.name destinationName,
	s.id spaceId,
	s.name spaceName,
	s.map spaceMap,
	m.id packageId,
	m.name packageName,
	m.title packageTitle,
	preview.id previewId,
	preview.url previewUrl,
	preview.type previewType,
	preview.mime previewMime,
	preview.size previewSize
FROM portals p
	LEFT JOIN entities e on p.id = e.id
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
	-- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON de.id = d.id
    LEFT JOIN accessibles da ON de.id = da.entity_id AND da.user_id = $1::uuid
    LEFT JOIN files preview ON de.id = preview.entity_id AND preview.type = 'rendertarget_preview'
	-- destination space
    LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
	LEFT JOIN accessibles sa ON se.id = sa.entity_id AND sa.user_id = $1::uuid
	-- destination package
    LEFT JOIN mods m ON m.id = s.mod_id 
	LEFT JOIN entities me ON me.id = m.id
    LEFT JOIN accessibles ma ON me.id = ma.entity_id AND ma.user_id = $1::uuid
WHERE p.id = $2
ORDER BY e.id`

	var (
		rows pgx.Rows
	)

	rows, err = db.Query(ctx, q, requester.Id /*$1*/, id /*$2*/)

	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetPortalForRequester")
	}()
	for rows.Next() {
		var (
			portalId        pgtypeuuid.UUID
			name            *string
			public          *bool
			destinationId   pgtypeuuid.UUID
			destinationName *string
			spaceId         pgtypeuuid.UUID
			spaceName       *string
			spaceMap        *string
			packageId       pgtypeuuid.UUID
			packageName     *string
			packageTitle    *string
			previewId       pgtypeuuid.UUID
			previewUrl      *string
			previewType     *string
			previewMime     *string
			previewSize     *int64
		)

		err = rows.Scan(
			&portalId,
			&name,
			&public,
			&destinationId,
			&destinationName,
			&spaceId,
			&spaceName,
			&spaceMap,
			&packageId,
			&packageName,
			&packageTitle,
			&previewId,
			&previewUrl,
			&previewType,
			&previewMime,
			&previewSize,
		)
		if err != nil {
			return nil, err
		}

		if portalId.Status == pgtype.Null {
			continue
		}

		var preview *File
		if previewId.Status != pgtype.Null {
			preview = new(File)
			preview.Id = &previewId.UUID
			if previewUrl != nil {
				preview.Url = *previewUrl
			}

			if previewType != nil {
				preview.Type = *previewType
			}

			if previewMime != nil {
				preview.Mime = previewMime
			}

			if previewSize != nil {
				preview.Size = previewSize
			}
		}

		if entity != nil {
			if preview != nil && !containsFile(entity.Destination.Files, *preview.Id) {
				if entity.Destination != nil {
					entity.Destination.Files = append(entity.Destination.Files, *preview)
				}
			}
		} else {
			var e Portal
			e.Id = &portalId.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = *name
			}
			if destinationId.Status != pgtype.Null {
				if e.Destination == nil {
					e.Destination = new(Portal)
				}
				e.Destination.Id = &destinationId.UUID
				if destinationName != nil {
					e.Destination.Name = *destinationName
				}
				if preview != nil {
					e.Destination.Files = append(e.Files, *preview)
				}
				if spaceId.Status != pgtype.Null {
					if e.Destination.World == nil {
						e.Destination.World = new(World)
					}
					e.Destination.World.Id = &spaceId.UUID
					if spaceName != nil {
						e.Destination.World.Name = *spaceName
					}
					if spaceMap != nil {
						e.Destination.World.Map = *spaceMap
					}
					if packageId.Status != pgtype.Null {
						if e.Destination.World.Package == nil {
							e.Destination.World.Package = new(Package)
						}
						e.Destination.World.Package.Id = &packageId.UUID
						if packageName != nil {
							e.Destination.World.Package.Name = *packageName
						}
						if packageTitle != nil {
							e.Destination.World.Package.Title = *packageTitle
						}
					}
				}
			}
			entity = &e
		}
	}

	return entity, err
}

// CreatePortalForRequester Creates a new world
func CreatePortalForRequester(ctx context.Context, requester *sm.User, m PortalCreateMetadata) (entity *Portal, err error) {
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
	entityType := "portal"
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
	q = `INSERT INTO portals (id, name, space_id, destination_id) VALUES ($1, $2, $3, $4)`

	if _, err1 = tx.Exec(ctx, q, id /*1*/, m.Name /*2*/, m.WorldId /*3*/, m.DestinationId /*4*/); err1 != nil {
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

	if entity, err1 = GetPortalForRequester(ctx, requester, id); err1 != nil {
		return nil, fmt.Errorf("failed to get the entity: %v", err1)
	}

	return entity, nil
}

func UpdatePortalForRequester(ctx context.Context, requester *sm.User, id uuid.UUID, m PortalUpdateMetadata) (entity *Portal, err error) {
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

	e, err1 := GetPortalForRequester(ctx, requester, id)
	if err1 != nil {
		return nil, fmt.Errorf("failed to get the source world: %v", err1)
	}

	if m.Name != nil {
		e.Name = *m.Name
	}

	if m.WorldId != nil {
		if e.World != nil {
			e.World.Id = m.WorldId
		} else {
			e.World = new(World)
			e.World.Id = m.WorldId
		}
	}

	if m.DestinationId != nil {
		if e.Destination != nil {
			e.Destination.Id = m.DestinationId
		} else {
			e.Destination = new(Portal)
			e.Destination.Id = m.DestinationId
		}
	}

	q := `UPDATE portals SET name=$1, destination_id=$2, space_id=$3 WHERE id = $2`
	if _, err1 = tx.Exec(ctx, q, e.Name /*$1*/, e.Destination.Id /*$2*/, e.World.Id /*$3*/); err1 != nil {
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

	e, err1 = GetPortalForRequester(ctx, requester, id)
	if err1 != nil {
		return nil, fmt.Errorf("failed to get the updated world: %v", err1)
	}

	return e, nil
}
