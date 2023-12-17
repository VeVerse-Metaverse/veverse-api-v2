package model

import (
	"context"
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/jackc/pgtype"
	pgtypeuuid "github.com/jackc/pgtype/ext/gofrs-uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"veverse-api/database"
	"veverse-api/reflect"
)

type Object struct {
	Entity
	EntityId      *uuid.UUID  `json:"entityId,omitempty"`
	OffsetX       float64     `json:"pX,omitempty"`
	OffsetY       float64     `json:"pY,omitempty"`
	OffsetZ       float64     `json:"pZ,omitempty"`
	RotationX     float64     `json:"rX,omitempty"`
	RotationY     float64     `json:"rY,omitempty"`
	RotationZ     float64     `json:"rZ,omitempty"`
	ScaleX        float64     `json:"sX,omitempty"`
	ScaleY        float64     `json:"sY,omitempty"`
	ScaleZ        float64     `json:"sZ,omitempty"`
	WorldId       *uuid.UUID  `json:"spaceId,omitempty"`
	Class         ObjectClass `json:"placeableClass,omitempty"`
	Type          string      `json:"type,omitempty"`
	TotalLikes    *int32      `json:"totalLikes,omitempty"`
	TotalDislikes *int32      `json:"totalDislikes,omitempty"`
}

type ArtObject struct {
	Entity
	ObjectType      *string  `json:"objectType,omitempty"`
	Name            *string  `json:"name,omitempty"`
	Artist          *string  `json:"artist,omitempty"`
	Date            *string  `json:"date,omitempty"`
	Description     *string  `json:"description,omitempty"`
	Medium          *string  `json:"medium,omitempty"`
	Width           *float64 `json:"width,omitempty"`
	Height          *float64 `json:"height,omitempty"`
	ScaleMultiplier *float64 `json:"scaleMultiplier,omitempty"`
	Source          *string  `json:"source,omitempty"`
	SourceUrl       *string  `json:"sourceUrl,omitempty"`
	License         *string  `json:"license,omitempty"`
	Copyright       *string  `json:"copyright,omitempty"`
	Credit          *string  `json:"credit,omitempty"`
	Origin          *string  `json:"origin,omitempty"`
	Location        *string  `json:"location,omitempty"`
	Dimensions      *string  `json:"dimensions,omitempty"`
	Liked           *int32   `json:"liked,omitempty"`
	TotalLikes      *int32   `json:"totalLikes,omitempty"`
	TotalDislikes   *int32   `json:"totalDislikes,omitempty"`
}

type SearchArtObject struct {
	Name   *string `json:"name,omitempty"  validate:"required"`
	Type   *string `json:"type,omitempty" validate:"required"`
	Artist *string `json:"artist,omitempty" validate:"required"`
}

// ObjectBatchRequestMetadata Batch request metadata for requesting Object entities
type ObjectBatchRequestMetadata struct {
	BatchRequestMetadata
	WorldId string `json:"spaceId,omitempty"`
}

type ObjectRequestMetadata struct {
	IdRequestMetadata
}

var (
	objectSingular = "object"
	objectPlural   = "objects"
)

func findObject(h []Object, id *uuid.UUID) int {
	for i, v := range h {
		if v.Id.String() == id.String() {
			return i
		}
	}
	return -1
}

func findArtObject(h []ArtObject, id *uuid.UUID) int {
	for i, v := range h {
		if v.Id.String() == id.String() {
			return i
		}
	}
	return -1
}

// IndexObjectsForAdminForWorld Index packages for admin
func IndexObjectsForAdminForWorld(ctx context.Context, worldId uuid.UUID, offset int64, limit int64) (placeables []Object, total int64, err error) {
	db := database.DB

	q := `
SELECT COUNT(*) 
FROM placeables p
WHERE p.space_id = $1
`
	row := db.QueryRow(ctx, q, worldId)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}

	q = `SELECT
	p.id p_id,
	p.entity_id p_entity_id, -- Linked entity if any
	p.offset_x p_px,
	p.offset_y p_py,
	p.offset_z p_pz,
	p.rotation_x p_rx,
	p.rotation_y p_ry,
	p.rotation_z p_rz,
	p.scale_x p_sx,
	p.scale_y p_sy,
	p.scale_z p_sz,
	e.public pe_public,
	pc.cls pc_class,
	f.id mpf_id,
	f.type mpf_type,
	f.mime mpf_mime,
	f.url mpf_url,
	props.name prop_name,
	props.type prop_type,
	props.value prop_value
FROM placeables p
   	LEFT JOIN entities e ON e.id = p.id -- Entity (public flag)
   	LEFT JOIN files f ON f.entity_id = e.id
    LEFT JOIN properties props ON e.id = props.entity_id
	LEFT JOIN placeable_classes pc ON p.placeable_class_id = pc.id
WHERE p.space_id = $1 AND (f.type != 'image_full' OR f.type IS NULL) ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, worldId /*$1*/)

	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	var (
		ri        int64 = 0 // Current row index
		ei        int64 = 0 // Current entity index, if >= offset, append row to results, if >= limit, stop processing rows
		skipped         = false
		skippedId uuid.UUID
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexObjectsForAdminForWorld")
	}()
	for rows.Next() {
		var (
			id        pgtypeuuid.UUID
			entityId  pgtypeuuid.UUID
			offsetX   float64
			offsetY   float64
			offsetZ   float64
			rotationX float64
			rotationY float64
			rotationZ float64
			scaleX    float64
			scaleY    float64
			scaleZ    float64
			public    *bool
			class     *string
			fileId    pgtypeuuid.UUID
			fileType  *string
			fileMime  *string
			fileUrl   *string
			propName  *string
			propType  *string
			propValue *string
		)

		err = rows.Scan(
			&id,
			&entityId,
			&offsetX,
			&offsetY,
			&offsetZ,
			&rotationX,
			&rotationY,
			&rotationZ,
			&scaleX,
			&scaleY,
			&scaleZ,
			&public,
			&class,
			&fileId,
			&fileType,
			&fileMime,
			&fileUrl,
			&propName,
			&propType,
			&propValue)
		if err != nil {
			return nil, -1, err
		}

		ri++

		// Skip invalid entities if any
		if id.Status == pgtype.Null {
			continue
		}

		var prop *Property
		if propName != nil && propType != nil {
			prop = new(Property)
			prop.Name = *propName
			prop.Type = *propType
			prop.Value = *propValue
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
		}

		if i := findObject(placeables, &id.UUID); i >= 0 {
			if file != nil {
				placeables[i].Files = append(placeables[i].Files, *file)
			}
			if prop != nil {
				var duplicate bool
				if len(placeables[i].Properties) > 0 {
					for _, property := range placeables[i].Properties {
						if property.Type == prop.Type && property.Id == prop.Id && property.Name == prop.Name {
							duplicate = true
						}
					}
				}

				if !duplicate {
					placeables[i].Properties = append(placeables[i].Properties, *prop)
				}
			}
		} else {
			if skipped {
				if id.UUID == skippedId {
					continue // Continue to the next element without incrementing entity index
				}
			}

			// Skip first {offset} entities
			if ei < offset {
				ei++
				skipped = true
				skippedId = id.UUID
				continue
			}

			// Stop iteration after getting required number of entities
			if ei-offset >= limit {
				break
			}

			var e Object
			e.Id = &id.UUID
			e.EntityId = &entityId.UUID
			if public != nil {
				e.Public = public
			}
			e.OffsetX = offsetX
			e.OffsetY = offsetY
			e.OffsetZ = offsetZ
			e.RotationX = rotationX
			e.RotationY = rotationY
			e.RotationZ = rotationZ
			e.ScaleX = scaleX
			e.ScaleY = scaleY
			e.ScaleZ = scaleZ
			if class != nil {
				e.Class = ObjectClass{
					Class: *class,
				}
			}
			if file != nil {
				e.Files = append(e.Files, *file)
			}
			if prop != nil {
				e.Properties = append(e.Properties, *prop)
			}

			placeables = append(placeables, e)
			skipped = false
			ei++
		}
	}

	return placeables, total, err
}

// IndexObjectsForRequesterForWorld Index packages for a requester
func IndexObjectsForRequesterForWorld(ctx context.Context, requester *sm.User, worldId uuid.UUID, offset int64, limit int64) (entities []Object, total int64, err error) {
	db := database.DB

	//	q := `
	//SELECT COUNT(*)
	//FROM placeables p
	//    LEFT JOIN entities pe ON pe.id = p.id
	//	LEFT JOIN accessibles a ON pe.id = a.entity_id AND a.user_id = $1::uuid
	//WHERE p.space_id = $2 AND (pe.public OR a.can_view OR a.is_owner)`

	q := `
SELECT COUNT(*) 
FROM placeables p
    LEFT JOIN entities pe ON pe.id = p.id
WHERE p.space_id = $1 and pe.public`

	//row := db.QueryRow(ctx, q, requester.Id /*$1*/, worldId /*$2*/)
	row := db.QueryRow(ctx, q, worldId /*$1*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}

	//	q = `SELECT
	//	p.id p_id, p.entity_id p_entity_id,
	//	p.offset_x p_px, p.offset_y p_py, p.offset_z p_pz,
	//	p.rotation_x p_rx, p.rotation_y p_ry, p.rotation_z p_rz,
	//	p.scale_x p_sx, p.scale_y p_sy, p.scale_z p_sz,
	//	pe.public pe_public,
	//	pc.cls pc_class,
	//	pf.id mpf_id, pf.type mpf_type, pf.mime mpf_mime, pf.url mpf_url
	//FROM placeables p
	//   	LEFT JOIN entities pe ON pe.id = p.id
	//   	LEFT JOIN files pf ON pf.entity_id = pe.id
	//	LEFT JOIN placeable_classes pc ON p.placeable_class_id = pc.id
	//	LEFT JOIN accessibles a ON pe.id = a.entity_id AND a.user_id = $1::uuid
	//WHERE p.space_id = $2 AND (pf.type != 'image_full' OR pf.type IS NULL) AND (pe.public OR a.can_view OR a.is_owner)
	//ORDER BY p.id`

	q = `SELECT
	p.id p_id, p.entity_id p_entity_id,
	p.offset_x p_px, p.offset_y p_py, p.offset_z p_pz,
	p.rotation_x p_rx, p.rotation_y p_ry, p.rotation_z p_rz,
	p.scale_x p_sx, p.scale_y p_sy, p.scale_z p_sz,
	pe.public pe_public,
	pc.cls pc_class,
	pf.id mpf_id, pf.type mpf_type, pf.mime mpf_mime, pf.url mpf_url,
	props.name prop_name,
	props.type prop_type,
	props.value prop_value
FROM placeables p
	LEFT JOIN entities pe ON pe.id = p.id
	LEFT JOIN files pf ON pf.entity_id = pe.id
	LEFT JOIN properties props ON pe.id = props.entity_id
	LEFT JOIN placeable_classes pc ON p.placeable_class_id = pc.id
WHERE p.space_id = $1 AND (pf.type != 'image_full' OR pf.type IS NULL)
ORDER BY pe.updated_at DESC, pe.created_at DESC, p.id`

	var rows pgx.Rows
	//rows, err = db.Query(ctx, q, requester.Id /*$1*/, worldId /*$2*/)
	rows, err = db.Query(ctx, q, worldId /*$1*/)

	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	var (
		ei        int64 = 0 // Current entity index, if >= offset, append row to results, if >= limit, stop processing rows
		skipped         = false
		skippedId uuid.UUID
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexObjectsForRequesterForWorld")
	}()
	for rows.Next() {
		var (
			id        pgtypeuuid.UUID
			entityId  pgtypeuuid.UUID
			offsetX   float64
			offsetY   float64
			offsetZ   float64
			rotationX float64
			rotationY float64
			rotationZ float64
			scaleX    float64
			scaleY    float64
			scaleZ    float64
			public    *bool
			class     *string
			fileId    pgtypeuuid.UUID
			fileType  *string
			fileMime  *string
			fileUrl   *string
			propName  *string
			propType  *string
			propValue *string
		)

		err = rows.Scan(&id, &entityId,
			&offsetX, &offsetY, &offsetZ,
			&rotationX, &rotationY, &rotationZ,
			&scaleX, &scaleY, &scaleZ,
			&public, &class,
			&fileId, &fileType, &fileMime, &fileUrl,
			&propName,
			&propType,
			&propValue)
		if err != nil {
			return nil, -1, err
		}

		// Skip invalid entities if any
		if pgtype.Null == id.Status {
			continue
		}

		var prop *Property
		if propName != nil && propType != nil {
			prop = new(Property)
			prop.Name = *propName
			prop.Type = *propType
			prop.Value = *propValue
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
		}

		if i := findObject(entities, &id.UUID); i >= 0 {
			if file != nil && !containsFile(entities[i].Files, *file.Id) {
				entities[i].Files = append(entities[i].Files, *file)
			}
			if prop != nil {
				var duplicate bool
				if len(entities[i].Properties) > 0 {
					for _, property := range entities[i].Properties {
						if property.Type == prop.Type && property.Id == prop.Id && property.Name == prop.Name {
							duplicate = true
						}
					}
				}

				if !duplicate {
					entities[i].Properties = append(entities[i].Properties, *prop)
				}
			}
		} else {
			// new entity
			if skipped {
				if id.UUID == skippedId {
					continue // Continue to the next element without incrementing entity index
				}
			}

			// Skip first {offset} entities
			if ei < offset {
				ei++
				skipped = true
				skippedId = id.UUID
				continue
			}

			// Stop iteration after getting required number of entities
			if ei-offset >= limit {
				break
			}

			var e Object
			e.Id = &id.UUID
			e.EntityId = &entityId.UUID
			if public != nil {
				e.Public = public
			}
			e.OffsetX = offsetX
			e.OffsetY = offsetY
			e.OffsetZ = offsetZ
			e.RotationX = rotationX
			e.RotationY = rotationY
			e.RotationZ = rotationZ
			e.ScaleX = scaleX
			e.ScaleY = scaleY
			e.ScaleZ = scaleZ
			if class != nil {
				e.Class = ObjectClass{
					Class: *class,
				}
			}
			if file != nil {
				e.Files = append(e.Files, *file)
			}

			if prop != nil {
				e.Properties = append(e.Properties, *prop)
			}

			entities = append(entities, e)
			skipped = false
			ei++
		}
	}

	return entities, total, err
}

// GetObjectForAdmin Get portal
func GetObjectForAdmin(ctx context.Context, id uuid.UUID) (entity *Object, err error) {
	db := database.DB

	q := `SELECT
	o.id p_id,
	o.entity_id p_entity_id, -- Linked entity if any
	o.offset_x p_px,
	o.offset_y p_py,
	o.offset_z p_pz,
	o.rotation_x p_rx,
	o.rotation_y p_ry,
	o.rotation_z p_rz,
	o.scale_x p_sx,
	o.scale_y p_sy,
	o.scale_z p_sz,
	e.public pe_public,
	pc.cls pc_class,
	f.id mpf_id,
	f.type mpf_type,
	f.mime mpf_mime,
	f.url mpf_url,
	props.name prop_name,
	props.type prop_type,
	props.value prop_value
FROM placeables o
   	LEFT JOIN entities e ON e.id = o.id -- Entity (public flag)
   	LEFT JOIN files f ON f.entity_id = e.id
    LEFT JOIN properties props ON e.id = props.entity_id
	LEFT JOIN placeable_classes pc ON o.placeable_class_id = pc.id
WHERE e.id = $1 ORDER BY e.id`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, id /*$1*/)

	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	var props []Property

	defer func() {
		rows.Close()
		database.LogPgxStat("GetObjectForAdmin")
	}()
	for rows.Next() {
		var (
			objectId  pgtypeuuid.UUID
			entityId  pgtypeuuid.UUID
			offsetX   float64
			offsetY   float64
			offsetZ   float64
			rotationX float64
			rotationY float64
			rotationZ float64
			scaleX    float64
			scaleY    float64
			scaleZ    float64
			public    *bool
			class     *string
			fileId    pgtypeuuid.UUID
			fileType  *string
			fileMime  *string
			fileUrl   *string
			propName  *string
			propType  *string
			propValue *string
		)

		err = rows.Scan(
			&objectId,
			&entityId,
			&offsetX,
			&offsetY,
			&offsetZ,
			&rotationX,
			&rotationY,
			&rotationZ,
			&scaleX,
			&scaleY,
			&scaleZ,
			&public,
			&class,
			&fileId,
			&fileType,
			&fileMime,
			&fileUrl,
			&propName,
			&propType,
			&propValue)
		if err != nil {
			return nil, err
		}

		// Skip invalid entities if any
		if objectId.Status == pgtype.Null {
			continue
		}

		var prop *Property
		if propName != nil && propType != nil {
			prop = new(Property)
			prop.Name = *propName
			prop.Type = *propType
			prop.Value = *propValue
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
		}

		if entity != nil && file != nil {
			if file != nil && !containsFile(entity.Files, *file.Id) {
				entity.Files = append(entity.Files, *file)
			}
		} else {
			var e Object
			e.Id = &objectId.UUID
			e.EntityId = &entityId.UUID
			if public != nil {
				e.Public = public
			}
			e.OffsetX = offsetX
			e.OffsetY = offsetY
			e.OffsetZ = offsetZ
			e.RotationX = rotationX
			e.RotationY = rotationY
			e.RotationZ = rotationZ
			e.ScaleX = scaleX
			e.ScaleY = scaleY
			e.ScaleZ = scaleZ
			if class != nil {
				e.Class = ObjectClass{
					Class: *class,
				}
			}

			if file != nil {
				e.Files = append(e.Files, *file)
			}

			if prop != nil {
				var duplicate bool
				if len(props) > 0 {
					for _, property := range props {
						if property.Type == prop.Type && property.Id == prop.Id && property.Name == prop.Name {
							duplicate = true
						}
					}
				}

				if !duplicate {
					props = append(props, *prop)
				}
			}

			e.Properties = props
			entity = &e
		}
	}

	return entity, err
}

// GetObjectForRequester Get object
func GetObjectForRequester(ctx context.Context, requester *sm.User, id uuid.UUID) (entity *Object, err error) {
	db := database.DB

	q := `SELECT
	o.id p_id,
	o.entity_id p_entity_id, -- Linked entity if any
	o.offset_x p_px,
	o.offset_y p_py,
	o.offset_z p_pz,
	o.rotation_x p_rx,
	o.rotation_y p_ry,
	o.rotation_z p_rz,
	o.scale_x p_sx,
	o.scale_y p_sy,
	o.scale_z p_sz,
	e.public pe_public,
	pc.cls pc_class,
	f.id mpf_id,
	f.type mpf_type,
	f.mime mpf_mime,
	f.url mpf_url,
	props.name prop_name,
	props.type prop_type,
	props.value prop_value
FROM placeables o
   	LEFT JOIN entities e ON e.id = o.id -- Entity (public flag)
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
   	LEFT JOIN files f ON f.entity_id = e.id
    LEFT JOIN properties props ON e.id = props.entity_id
	LEFT JOIN placeable_classes pc ON o.placeable_class_id = pc.id
WHERE e.id = $2 ORDER BY e.id`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, requester.Id /*$1*/, id /*$2*/)

	if err != nil {
		return nil, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	var props []Property

	defer func() {
		rows.Close()
		database.LogPgxStat("GetObjectForRequester")
	}()
	for rows.Next() {
		var (
			objectId  pgtypeuuid.UUID
			entityId  pgtypeuuid.UUID
			offsetX   float64
			offsetY   float64
			offsetZ   float64
			rotationX float64
			rotationY float64
			rotationZ float64
			scaleX    float64
			scaleY    float64
			scaleZ    float64
			public    *bool
			class     *string
			fileId    pgtypeuuid.UUID
			fileType  *string
			fileMime  *string
			fileUrl   *string
			propName  *string
			propType  *string
			propValue *string
		)

		err = rows.Scan(
			&objectId,
			&entityId,
			&offsetX,
			&offsetY,
			&offsetZ,
			&rotationX,
			&rotationY,
			&rotationZ,
			&scaleX,
			&scaleY,
			&scaleZ,
			&public,
			&class,
			&fileId,
			&fileType,
			&fileMime,
			&fileUrl,
			&propName,
			&propType,
			&propValue)
		if err != nil {
			return nil, err
		}

		var prop *Property
		if propName != nil && propType != nil {
			prop = new(Property)
			prop.Name = *propName
			prop.Type = *propType
			prop.Value = *propValue
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
		}

		if entity != nil && file != nil {
			if file != nil && !containsFile(entity.Files, *file.Id) {
				entity.Files = append(entity.Files, *file)
			}
		} else {
			var e Object
			e.Id = &objectId.UUID
			e.EntityId = &entityId.UUID
			if public != nil {
				e.Public = public
			}
			e.OffsetX = offsetX
			e.OffsetY = offsetY
			e.OffsetZ = offsetZ
			e.RotationX = rotationX
			e.RotationY = rotationY
			e.RotationZ = rotationZ
			e.ScaleX = scaleX
			e.ScaleY = scaleY
			e.ScaleZ = scaleZ
			if class != nil {
				e.Class = ObjectClass{
					Class: *class,
				}
			}
			if file != nil {
				e.Files = append(e.Files, *file)
			}
			if prop != nil {
				var duplicate bool
				if len(props) > 0 {
					for _, property := range props {
						if property.Type == prop.Type && property.Id == prop.Id && property.Name == prop.Name {
							duplicate = true
						}
					}
				}

				if !duplicate {
					props = append(props, *prop)
				}
			}

			e.Properties = props
			entity = &e
		}
	}

	return entity, err
}

func GetObjectsForAdmin(ctx context.Context, offset int64, limit int64) (objects []Object, total int32, err error) {

	var (
		q    string
		row  pgx.Row
		rows pgx.Rows
		db   *pgxpool.Pool
	)

	db = database.DB
	q = `SELECT COUNT(*) FROM placeables p`

	row = db.QueryRow(ctx, q)
	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", objectSingular, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get %s", objectSingular)
	}

	q = `SELECT
	o.id p_id,
	o.entity_id p_entity_id, -- Linked entity if any
	o.offset_x p_px,
	o.offset_y p_py,
	o.offset_z p_pz,
	o.rotation_x p_rx,
	o.rotation_y p_ry,
	o.rotation_z p_rz,
	o.scale_x p_sx,
	o.scale_y p_sy,
	o.scale_z p_sz,
	e.public pe_public,
	pc.cls pc_class,
	f.id mpf_id,
	f.type mpf_type,
	f.mime mpf_mime,
	f.url mpf_url,
	props.name prop_name,
	props.type prop_type,
	props.value prop_value,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes
FROM placeables o
   	LEFT JOIN entities e ON e.id = o.id -- Entity (public flag)
	LEFT JOIN likables l ON l.entity_id = e.id
   	LEFT JOIN files f ON f.entity_id = e.id
    LEFT JOIN properties props ON e.id = props.entity_id
	LEFT JOIN placeable_classes pc ON o.placeable_class_id = pc.id
	GROUP BY o.id, e.public, pc.cls, f.id, f.type, f.mime, f.url, props.name, props.type, props.value
	OFFSET $1
	LIMIT $2`

	rows, err = db.Query(ctx, q, offset, limit)

	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", objectPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get %s", objectPlural)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetObjectsForAdmin")
	}()
	for rows.Next() {
		var props []Property
		var object *Object

		var (
			objectId      pgtypeuuid.UUID
			entityId      pgtypeuuid.UUID
			offsetX       float64
			offsetY       float64
			offsetZ       float64
			rotationX     float64
			rotationY     float64
			rotationZ     float64
			scaleX        float64
			scaleY        float64
			scaleZ        float64
			public        *bool
			class         *string
			fileId        pgtypeuuid.UUID
			fileType      *string
			fileMime      *string
			fileUrl       *string
			propName      *string
			propType      *string
			propValue     *string
			totalLikes    *int32
			totalDislikes *int32
		)

		err = rows.Scan(
			&objectId,
			&entityId,
			&offsetX,
			&offsetY,
			&offsetZ,
			&rotationX,
			&rotationY,
			&rotationZ,
			&scaleX,
			&scaleY,
			&scaleZ,
			&public,
			&class,
			&fileId,
			&fileType,
			&fileMime,
			&fileUrl,
			&propName,
			&propType,
			&propValue,
			&totalLikes,
			&totalDislikes,
		)

		if err != nil {
			logrus.Errorf("failed to scan %s @ %s: %v", objectPlural, reflect.FunctionName(), err)
			return nil, -1, fmt.Errorf("failed to get %s", objectPlural)
		}

		// Skip invalid entities if any
		if objectId.Status == pgtype.Null {
			continue
		}

		var prop *Property
		if propName != nil && propType != nil {
			prop = new(Property)
			prop.Name = *propName
			prop.Type = *propType
			prop.Value = *propValue
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
		}

		if object != nil && file != nil {
			if file != nil && !containsFile(object.Files, *file.Id) {
				object.Files = append(object.Files, *file)
			}
		} else {
			var e Object
			e.Id = &objectId.UUID
			e.EntityId = &entityId.UUID
			if public != nil {
				e.Public = public
			}
			e.OffsetX = offsetX
			e.OffsetY = offsetY
			e.OffsetZ = offsetZ
			e.RotationX = rotationX
			e.RotationY = rotationY
			e.RotationZ = rotationZ
			e.ScaleX = scaleX
			e.ScaleY = scaleY
			e.ScaleZ = scaleZ
			if class != nil {
				e.Class = ObjectClass{
					Class: *class,
				}
			}

			if file != nil {
				e.Files = append(e.Files, *file)
			}

			if prop != nil {
				var duplicate bool
				if len(props) > 0 {
					for _, property := range props {
						if property.Type == prop.Type && property.Id == prop.Id && property.Name == prop.Name {
							duplicate = true
						}
					}
				}

				if !duplicate {
					props = append(props, *prop)
				}
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

			e.Properties = props

			object = &e
		}

		objects = append(objects, *object)
	}

	return objects, total, nil
}

func GetObjectsForRequester(ctx context.Context, requester *sm.User, offset int64, limit int64) (objects []Object, total int32, err error) {

	var (
		q    string
		row  pgx.Row
		rows pgx.Rows
		db   *pgxpool.Pool
	)

	db = database.DB
	q = `SELECT
    COUNT(*)
FROM placeables p
    LEFT JOIN entities pe ON p.id = pe.id
    LEFT JOIN accessibles a on pe.id = a.entity_id
WHERE a.user_id = $1 AND (pe.public OR a.can_view OR a.is_owner)`

	row = db.QueryRow(ctx, q, "00000000-0000-4000-a000-00000000000b")
	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", objectSingular, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get %s", objectSingular)
	}

	q = `SELECT
	p.id p_id,
	p.entity_id p_entity_id, -- Linked entity if any
	p.offset_x p_px,
	p.offset_y p_py,
	p.offset_z p_pz,
	p.rotation_x p_rx,
	p.rotation_y p_ry,
	p.rotation_z p_rz,
	p.scale_x p_sx,
	p.scale_y p_sy,
	p.scale_z p_sz,
	pe.public pe_public,
	pc.cls pc_class,
	f.id mpf_id,
	f.type mpf_type,
	f.mime mpf_mime,
	f.url mpf_url,
	props.name prop_name,
	props.type prop_type,
	props.value prop_value,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes
FROM placeables p
   	LEFT JOIN entities pe ON pe.id = p.id -- Entity (public flag)
	LEFT JOIN accessibles a on pe.id = a.entity_id
	LEFT JOIN likables l ON l.entity_id = pe.id
   	LEFT JOIN files f ON f.entity_id = pe.id
    LEFT JOIN properties props ON pe.id = props.entity_id
	LEFT JOIN placeable_classes pc ON p.placeable_class_id = pc.id
WHERE a.user_id = $1 AND (pe.public OR a.can_view OR a.is_owner)
GROUP BY p.id, pe.public, pc.cls, f.id, f.type, f.mime, f.url, props.name, props.type, props.value
OFFSET $2
LIMIT $3`

	rows, err = db.Query(ctx, q, "00000000-0000-4000-a000-00000000000b", offset, limit)

	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", objectPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get %s", objectPlural)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetObjectsForRequester")
	}()
	for rows.Next() {
		var props []Property
		var object *Object

		var (
			objectId      pgtypeuuid.UUID
			entityId      pgtypeuuid.UUID
			offsetX       float64
			offsetY       float64
			offsetZ       float64
			rotationX     float64
			rotationY     float64
			rotationZ     float64
			scaleX        float64
			scaleY        float64
			scaleZ        float64
			public        *bool
			class         *string
			fileId        pgtypeuuid.UUID
			fileType      *string
			fileMime      *string
			fileUrl       *string
			propName      *string
			propType      *string
			propValue     *string
			totalLikes    *int32
			totalDislikes *int32
		)

		err = rows.Scan(
			&objectId,
			&entityId,
			&offsetX,
			&offsetY,
			&offsetZ,
			&rotationX,
			&rotationY,
			&rotationZ,
			&scaleX,
			&scaleY,
			&scaleZ,
			&public,
			&class,
			&fileId,
			&fileType,
			&fileMime,
			&fileUrl,
			&propName,
			&propType,
			&propValue,
			&totalLikes,
			&totalDislikes,
		)

		if err != nil {
			logrus.Errorf("failed to scan %s @ %s: %v", objectPlural, reflect.FunctionName(), err)
			return nil, -1, fmt.Errorf("failed to get %s", objectPlural)
		}

		// Skip invalid entities if any
		if objectId.Status == pgtype.Null {
			continue
		}

		var prop *Property
		if propName != nil && propType != nil {
			prop = new(Property)
			prop.Name = *propName
			prop.Type = *propType
			prop.Value = *propValue
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
		}

		if object != nil && file != nil {
			if file != nil && !containsFile(object.Files, *file.Id) {
				object.Files = append(object.Files, *file)
			}
		} else {
			var e Object
			e.Id = &objectId.UUID
			e.EntityId = &entityId.UUID
			if public != nil {
				e.Public = public
			}
			e.OffsetX = offsetX
			e.OffsetY = offsetY
			e.OffsetZ = offsetZ
			e.RotationX = rotationX
			e.RotationY = rotationY
			e.RotationZ = rotationZ
			e.ScaleX = scaleX
			e.ScaleY = scaleY
			e.ScaleZ = scaleZ
			if class != nil {
				e.Class = ObjectClass{
					Class: *class,
				}
			}

			if file != nil {
				e.Files = append(e.Files, *file)
			}

			if prop != nil {
				var duplicate bool
				if len(props) > 0 {
					for _, property := range props {
						if property.Type == prop.Type && property.Id == prop.Id && property.Name == prop.Name {
							duplicate = true
						}
					}
				}

				if !duplicate {
					props = append(props, *prop)
				}
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

			e.Properties = props

			object = &e
		}

		objects = append(objects, *object)
	}

	return objects, total, nil
}

func GetArtObjectsForAdmin(ctx context.Context, requester *sm.User, offset int64, limit int64, query string) (objects []ArtObject, total int32, err error) {

	var (
		q    string
		row  pgx.Row
		rows pgx.Rows
		db   *pgxpool.Pool
	)

	db = database.DB
	q = `SELECT COUNT(*) FROM objects o WHERE o.type <> 'NFT' AND o.name ILIKE $1::text`

	row = db.QueryRow(ctx, q, query)
	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", objectSingular, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get %s", objectSingular)
	}

	q = `SELECT
	o.id,
	o.type,
	o.name,
	o.artist,
	o.date,
	o.description,
	o.medium,
	o.width,
	o.height,
	o.scale_multiplier,
	o.source,
	o.source_url,
	o.license,
	o.copyright,
	o.credit,
	o.origin,
	o.location,
	o.dimensions,
	owner.id ownerId,
	owner.name ownerName,
	f.id fId,
	f.type fType,
	f.mime fMime,
	f.url fUrl,
	l2.value liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM objects o
   	LEFT JOIN entities e ON e.id = o.id
	LEFT JOIN files f ON f.entity_id = e.id
	LEFT JOIN likables l ON l.entity_id = e.id
	LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
	LEFT JOIN accessibles a ON a.entity_id = e.id
	LEFT JOIN users owner ON owner.id = a.user_id
WHERE o.type <> 'NFT' AND o.name ILIKE $2::text
	GROUP BY o.id, owner.id, f.id, f.type, f.mime, f.url, l2.value, e.views
	ORDER BY o.id`

	rows, err = db.Query(ctx, q, requester.Id, query)

	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", objectPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get %s", objectPlural)
	}

	var (
		ri        int64 = 0 // Current row index
		ei        int64 = 0 // Current entity index, if >= offset, append row to results, if >= limit, stop processing rows
		skipped         = false
		skippedId uuid.UUID
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("GetArtObjectsForAdmin")
	}()
	for rows.Next() {
		var (
			o         ArtObject
			fileId    pgtypeuuid.UUID
			fileType  *string
			fileMime  *string
			fileUrl   *string
			ownerId   *uuid.UUID
			ownerName *string
		)

		err = rows.Scan(
			&o.Id,
			&o.ObjectType,
			&o.Name,
			&o.Artist,
			&o.Date,
			&o.Description,
			&o.Medium,
			&o.Width,
			&o.Height,
			&o.ScaleMultiplier,
			&o.Source,
			&o.SourceUrl,
			&o.License,
			&o.Copyright,
			&o.Credit,
			&o.Origin,
			&o.Location,
			&o.Dimensions,
			&ownerId,
			&ownerName,
			&fileId,
			&fileType,
			&fileMime,
			&fileUrl,
			&o.Liked,
			&o.TotalLikes,
			&o.TotalDislikes,
			&o.Views,
		)

		if err != nil {
			logrus.Errorf("failed to scan %s @ %s: %v", objectPlural, reflect.FunctionName(), err)
			return nil, -1, fmt.Errorf("failed to get %s", objectPlural)
		}

		ri++

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
		}

		if i := findArtObject(objects, o.Id); i >= 0 {
			if file != nil {
				objects[i].Files = append(objects[i].Files, *file)
			}
		} else {
			if skipped {
				if *o.Id == skippedId {
					continue // Continue to the next element without incrementing entity index
				}
			}

			if ei < offset {
				ei++
				skipped = true
				skippedId = *o.Id
				continue
			}

			// Stop iteration after getting required number of entities
			if ei-offset >= limit {
				break
			}

			if file != nil {
				o.Files = append(o.Files, *file)
			}

			if o.TotalLikes == nil {
				o.TotalLikes = new(int32)
				*o.TotalLikes = 0
			}

			if o.TotalDislikes == nil {
				o.TotalDislikes = new(int32)
				*o.TotalDislikes = 0
			}

			o.Owner = new(User)
			if ownerId != nil {
				o.Owner.Id = ownerId
			}

			if ownerName != nil {
				o.Owner.Name = ownerName
			}

			objects = append(objects, o)
			skipped = false
			ei++
		}
	}

	return objects, total, nil
}

func GetArtObjectsForRequester(ctx context.Context, requester *sm.User, offset int64, limit int64, query string) (objects []ArtObject, total int32, err error) {

	var (
		q    string
		row  pgx.Row
		rows pgx.Rows
		db   *pgxpool.Pool
	)

	db = database.DB
	q = `SELECT COUNT(*) FROM objects o
	LEFT JOIN entities e ON o.id = e.id
	LEFT JOIN accessibles a on e.id = a.entity_id
	WHERE e.public AND o.type <> 'NFT' AND o.name ILIKE $1::text`

	row = db.QueryRow(ctx, q, query)
	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", objectSingular, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get %s", objectSingular)
	}

	q = `SELECT
	o.id,
	o.type,
	o.name,
	o.artist,
	o.date,
	o.description,
	o.medium,
	o.width,
	o.height,
	o.scale_multiplier,
	o.source,
	o.source_url,
	o.license,
	o.copyright,
	o.credit,
	o.origin,
	o.location,
	o.dimensions,
	owner.name ownerName,
	f.id fId,
	f.type fType,
	f.mime fMime,
	f.url fUrl,
	l2.value liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM objects o
   	LEFT JOIN entities e ON e.id = o.id
	LEFT JOIN files f ON f.entity_id = e.id
    LEFT JOIN accessibles a on e.id = a.entity_id
	LEFT JOIN users owner ON owner.id = a.user_id
	LEFT JOIN likables l ON l.entity_id = e.id
	LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
WHERE e.public AND o.type <> 'NFT' AND o.name ILIKE $2::text
	GROUP BY o.id, owner.name, f.id, f.type, f.mime, f.url, l2.value, e.views
	ORDER BY o.id`

	rows, err = db.Query(ctx, q, requester.Id, query)

	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", objectPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get %s", objectPlural)
	}

	var (
		ri        int64 = 0 // Current row index
		ei        int64 = 0 // Current entity index, if >= offset, append row to results, if >= limit, stop processing rows
		skipped         = false
		skippedId uuid.UUID
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("GetArtObjectsForRequester")
	}()
	for rows.Next() {
		var (
			o         ArtObject
			fileId    pgtypeuuid.UUID
			fileType  *string
			fileMime  *string
			fileUrl   *string
			ownerName *string
		)

		err = rows.Scan(
			&o.Id,
			&o.ObjectType,
			&o.Name,
			&o.Artist,
			&o.Date,
			&o.Description,
			&o.Medium,
			&o.Width,
			&o.Height,
			&o.ScaleMultiplier,
			&o.Source,
			&o.SourceUrl,
			&o.License,
			&o.Copyright,
			&o.Credit,
			&o.Origin,
			&o.Location,
			&o.Dimensions,
			&ownerName,
			&fileId,
			&fileType,
			&fileMime,
			&fileUrl,
			&o.Liked,
			&o.TotalLikes,
			&o.TotalDislikes,
			&o.Views,
		)

		if err != nil {
			logrus.Errorf("failed to scan %s @ %s: %v", objectPlural, reflect.FunctionName(), err)
			return nil, -1, fmt.Errorf("failed to get %s", objectPlural)
		}

		ri++

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
		}

		if i := findArtObject(objects, o.Id); i >= 0 {
			if file != nil {
				objects[i].Files = append(objects[i].Files, *file)
			}
		} else {
			if skipped {
				if *o.Id == skippedId {
					continue // Continue to the next element without incrementing entity index
				}
			}

			if ei < offset {
				ei++
				skipped = true
				skippedId = *o.Id
				continue
			}

			// Stop iteration after getting required number of entities
			if ei-offset >= limit {
				break
			}

			if file != nil {
				o.Files = append(o.Files, *file)
			}

			if o.TotalLikes == nil {
				o.TotalLikes = new(int32)
				*o.TotalLikes = 0
			}

			if o.TotalDislikes == nil {
				o.TotalDislikes = new(int32)
				*o.TotalDislikes = 0
			}

			o.Owner = new(User)
			if ownerName != nil {
				o.Owner.Name = ownerName
			}

			objects = append(objects, o)
			skipped = false
			ei++
		}
	}

	return objects, total, nil
}

func GetArtObjectForAdmin(ctx context.Context, requester *sm.User, entityId uuid.UUID) (object *ArtObject, err error) {

	var (
		q    string
		rows pgx.Rows
		db   *pgxpool.Pool
	)

	db = database.DB

	q = `SELECT
	o.id,
	o.type,
	o.name,
	o.artist,
	o.date,
	o.description,
	o.medium,
	o.width,
	o.height,
	o.scale_multiplier,
	o.source,
	o.source_url,
	o.license,
	o.copyright,
	o.credit,
	o.origin,
	o.location,
	o.dimensions,
	owner.id ownerId,
	owner.name ownerName,
	f.id fId,
	f.type fType,
	f.mime fMime,
	f.url fUrl,
	l2.value liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM objects o
   	LEFT JOIN entities e ON e.id = o.id
	LEFT JOIN files f ON f.entity_id = e.id
	LEFT JOIN likables l ON l.entity_id = e.id
	LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
	LEFT JOIN accessibles a ON a.entity_id = e.id
	LEFT JOIN users owner ON owner.id = a.user_id
WHERE o.type <> 'NFT' AND o.id = $2
	GROUP BY o.id, owner.id, f.id, f.type, f.mime, f.url, l2.value, e.views
	ORDER BY o.id`

	rows, err = db.Query(ctx, q, requester.Id, entityId)

	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", objectSingular, reflect.FunctionName(), err)
		return nil, fmt.Errorf("failed to get %s", objectSingular)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetArtObjectForAdmin")
	}()
	for rows.Next() {
		var (
			fileId    pgtypeuuid.UUID
			fileType  *string
			fileMime  *string
			fileUrl   *string
			ownerId   *uuid.UUID
			ownerName *string

			// object
			objId              *uuid.UUID
			objType            *string
			objName            *string
			objArtist          *string
			objDate            *string
			objDescription     *string
			objMedium          *string
			objWidth           *float64
			objHeight          *float64
			objScaleMultiplier *float64
			objSource          *string
			objSourceUrl       *string
			objLicense         *string
			objCopyright       *string
			objCredit          *string
			objOrigin          *string
			objLocation        *string
			objDimensions      *string
			objLiked           *int32
			objTotalLikes      *int32
			objTotalDislikes   *int32
			objViews           *int32
		)

		err = rows.Scan(
			&objId,
			&objType,
			&objName,
			&objArtist,
			&objDate,
			&objDescription,
			&objMedium,
			&objWidth,
			&objHeight,
			&objScaleMultiplier,
			&objSource,
			&objSourceUrl,
			&objLicense,
			&objCopyright,
			&objCredit,
			&objOrigin,
			&objLocation,
			&objDimensions,
			&ownerId,
			&ownerName,
			&fileId,
			&fileType,
			&fileMime,
			&fileUrl,
			&objLiked,
			&objTotalLikes,
			&objTotalDislikes,
			&objViews,
		)

		if err != nil {
			logrus.Errorf("failed to scan %s @ %s: %v", objectSingular, reflect.FunctionName(), err)
			return nil, fmt.Errorf("failed to get %s", objectSingular)
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
		}

		if object != nil && file != nil {
			if file != nil && !containsFile(object.Files, *file.Id) {
				object.Files = append(object.Files, *file)
			}
		} else {
			var o ArtObject
			o.Id = objId
			o.ObjectType = objType
			o.Name = objName
			o.Artist = objArtist
			o.Date = objDate
			o.Description = objDescription
			o.Medium = objMedium
			o.Width = objWidth
			o.Height = objHeight
			o.ScaleMultiplier = objScaleMultiplier
			o.Source = objSource
			o.SourceUrl = objSourceUrl
			o.License = objLicense
			o.Copyright = objCopyright
			o.Credit = objCredit
			o.Origin = objOrigin
			o.Location = objLocation
			o.Dimensions = objDimensions
			o.Liked = objLiked
			o.TotalLikes = objTotalLikes
			o.TotalDislikes = objTotalDislikes
			o.Views = objViews

			if file != nil {
				o.Files = append(o.Files, *file)
			}

			o.Owner = new(User)
			if ownerId != nil {
				o.Owner.Id = ownerId
			}

			if ownerName != nil {
				o.Owner.Name = ownerName
			}

			object = &o
		}
	}

	return object, nil
}

func GetArtObjectForRequester(ctx context.Context, requester *sm.User, entityId uuid.UUID) (object *ArtObject, err error) {

	var (
		q    string
		rows pgx.Rows
		db   *pgxpool.Pool
	)

	db = database.DB

	q = `SELECT
	o.id,
	o.type,
	o.name,
	o.artist,
	o.date,
	o.description,
	o.medium,
	o.width,
	o.height,
	o.scale_multiplier,
	o.source,
	o.source_url,
	o.license,
	o.copyright,
	o.credit,
	o.origin,
	o.location,
	o.dimensions,
	owner.id ownerId,
	owner.name ownerName,
	f.id fId,
	f.type fType,
	f.mime fMime,
	f.url fUrl,
	l2.value liked,
	sum(case when l.value >= 0 then l.value end) as total_likes,
	sum(case when l.value < 0 then l.value end) as total_dislikes,
	e.views
FROM objects o
   	LEFT JOIN entities e ON e.id = o.id
	LEFT JOIN accessibles a ON a.entity_id = e.id AND a.user_id = $1::uuid
	LEFT JOIN users owner ON owner.id = a.user_id
	LEFT JOIN files f ON f.entity_id = e.id
	LEFT JOIN likables l ON l.entity_id = e.id
	LEFT JOIN likables l2 ON l2.entity_id = e.id AND l2.user_id = $1
WHERE o.type <> 'NFT' AND o.id = $2 AND (e.public OR a.can_view OR a.is_owner)
	GROUP BY o.id,e.created_at, owner.id, owner.name, f.id, f.type, f.mime, f.url, l2.value, e.views
	ORDER BY e.created_at DESC, o.id`

	rows, err = db.Query(ctx, q, requester.Id, entityId)

	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", objectSingular, reflect.FunctionName(), err)
		return nil, fmt.Errorf("failed to get %s", objectSingular)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("GetArtObjectForRequester")
	}()
	for rows.Next() {
		var (
			fileId    pgtypeuuid.UUID
			fileType  *string
			fileMime  *string
			fileUrl   *string
			ownerId   *uuid.UUID
			ownerName *string

			// object
			objId              *uuid.UUID
			objType            *string
			objName            *string
			objArtist          *string
			objDate            *string
			objDescription     *string
			objMedium          *string
			objWidth           *float64
			objHeight          *float64
			objScaleMultiplier *float64
			objSource          *string
			objSourceUrl       *string
			objLicense         *string
			objCopyright       *string
			objCredit          *string
			objOrigin          *string
			objLocation        *string
			objDimensions      *string
			objLiked           *int32
			objTotalLikes      *int32
			objTotalDislikes   *int32
			objViews           *int32
		)

		err = rows.Scan(
			&objId,
			&objType,
			&objName,
			&objArtist,
			&objDate,
			&objDescription,
			&objMedium,
			&objWidth,
			&objHeight,
			&objScaleMultiplier,
			&objSource,
			&objSourceUrl,
			&objLicense,
			&objCopyright,
			&objCredit,
			&objOrigin,
			&objLocation,
			&objDimensions,
			&ownerId,
			&ownerName,
			&fileId,
			&fileType,
			&fileMime,
			&fileUrl,
			&objLiked,
			&objTotalLikes,
			&objTotalDislikes,
			&objViews,
		)

		if err != nil {
			logrus.Errorf("failed to scan %s @ %s: %v", objectSingular, reflect.FunctionName(), err)
			return nil, fmt.Errorf("failed to get %s", objectSingular)
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
		}

		if object != nil && file != nil {
			if file != nil && !containsFile(object.Files, *file.Id) {
				object.Files = append(object.Files, *file)
			}
		} else {
			var o ArtObject
			o.Id = objId
			o.ObjectType = objType
			o.Name = objName
			o.Artist = objArtist
			o.Date = objDate
			o.Description = objDescription
			o.Medium = objMedium
			o.Width = objWidth
			o.Height = objHeight
			o.ScaleMultiplier = objScaleMultiplier
			o.Source = objSource
			o.SourceUrl = objSourceUrl
			o.License = objLicense
			o.Copyright = objCopyright
			o.Credit = objCredit
			o.Origin = objOrigin
			o.Location = objLocation
			o.Dimensions = objDimensions
			o.Liked = objLiked
			o.TotalLikes = objTotalLikes
			o.TotalDislikes = objTotalDislikes
			o.Views = objViews

			if file != nil {
				o.Files = append(o.Files, *file)
			}

			o.Owner = new(User)
			if ownerId != nil {
				o.Owner.Id = ownerId
			}

			if ownerName != nil {
				o.Owner.Name = ownerName
			}

			object = &o
		}
	}

	return object, nil
}
