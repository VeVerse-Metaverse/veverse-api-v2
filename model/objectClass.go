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

// ObjectClass struct
type ObjectClass struct {
	Entity

	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
	Class       string `json:"class,omitempty"`
}

// ObjectClassBatchRequestMetadata Batch request metadata for requesting ObjectClass entities
type ObjectClassBatchRequestMetadata struct {
	BatchRequestMetadata

	Category string `json:"category,omitempty"` // Category to filter by
}

func findObjectClass(h []ObjectClass, id uuid.UUID) int {
	for i, v := range h {
		if *v.Id == id {
			return i
		}
	}
	return -1
}

func IndexObjectClassesForAdmin(ctx context.Context, offset int64, limit int64) (entities []ObjectClass, total int64, err error) {
	db := database.DB

	//region Count
	q := `SELECT COUNT(*) FROM placeable_classes pc`

	row := db.QueryRow(ctx, q)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to scan total @ %s: %v", reflect.FunctionName(), err)
	}
	//endregion

	q = `SELECT 
	pc.id                   placeableClassId,
	pc.name                 placeableClassName,
	pc.description          placeableClassDescription,
	pc.category             placeableClassCategory,
	e.public                entitypublic,
	preview.id              previewid,
	preview.url             previewurl,
	preview.type            previewtype,
	preview.mime        	previewmime
FROM placeable_classes pc
    LEFT JOIN entities e ON pc.id = e.id
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query object classes @ %s: %v", reflect.FunctionName(), err)
	}

	var (
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexObjectClassesForAdmin")
	}()
	for rows.Next() {
		var (
			id          pgtypeuuid.UUID
			name        *string
			description *string
			category    *string
			public      *bool
			fileId      pgtypeuuid.UUID
			fileUrl     *string
			fileType    *string
			fileMime    *string
		)

		err = rows.Scan(
			&id,
			&name,
			&description,
			&category,
			&public,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
		)
		if err != nil {
			return nil, -1, fmt.Errorf("failed to scan object class row @ %s: %v", reflect.FunctionName(), err)
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
		}

		if i := findObjectClass(entities, id.UUID); i >= 0 {
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

			var e ObjectClass
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
			if category != nil {
				e.Category = *category
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

func IndexObjectClassesForAdminForCategory(ctx context.Context, category string, offset int64, limit int64) (entities []ObjectClass, total int64, err error) {
	db := database.DB

	//region Count
	q := `SELECT COUNT(*) FROM placeable_classes pc WHERE pc.category = $1::text`

	row := db.QueryRow(ctx, q, category)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to scan total @ %s: %v", reflect.FunctionName(), err)
	}
	//endregion

	q = `SELECT 
	pc.id                   placeableClassId,
	pc.name                 placeableClassName,
	pc.description          placeableClassDescription,
	pc.category             placeableClassCategory,
	e.public                entitypublic,
	preview.id              previewid,
	preview.url             previewurl,
	preview.type            previewtype,
	preview.mime        	previewmime
FROM placeable_classes pc
    LEFT JOIN entities e ON pc.id = e.id
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
WHERE pc.category = $1::text
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, category)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to query object classes @ %s: %v", reflect.FunctionName(), err)
	}

	var (
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexObjectClassesForAdminForCategory")
	}()
	for rows.Next() {
		var (
			id                     pgtypeuuid.UUID
			name                   *string
			description            *string
			placeableClassCategory *string
			public                 *bool
			fileId                 pgtypeuuid.UUID
			fileUrl                *string
			fileType               *string
			fileMime               *string
		)

		err = rows.Scan(
			&id,
			&name,
			&description,
			&placeableClassCategory,
			&public,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
		)
		if err != nil {
			return nil, -1, fmt.Errorf("failed to scan object class row @ %s: %v", reflect.FunctionName(), err)
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
		}

		if i := findObjectClass(entities, id.UUID); i >= 0 {
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

			var e ObjectClass
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
			if placeableClassCategory != nil {
				e.Category = *placeableClassCategory
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

func IndexObjectClassesForAdminWithQuery(ctx context.Context, offset int64, limit int64, query string) (entities []ObjectClass, total int64, err error) {
	db := database.DB

	//region Count
	q := `SELECT COUNT(*) FROM placeable_classes pc WHERE pc.name ILIKE $1::text`

	row := db.QueryRow(ctx, q, query)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to scan total @ %s: %v", reflect.FunctionName(), err)
	}
	//endregion

	q = `SELECT 
	pc.id                   placeableClassId,
	pc.name                 placeableClassName,
	pc.description          placeableClassDescription,
	pc.category             placeableClassCategory,
	e.public                entitypublic,
	preview.id              previewid,
	preview.url             previewurl,
	preview.type            previewtype,
	preview.mime        	previewmime
FROM placeable_classes pc
    LEFT JOIN entities e ON pc.id = e.id
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
WHERE pc.name ILIKE $1::text
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, query)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to scan total @ %s: %v", reflect.FunctionName(), err)
	}

	var (
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexObjectClassesForAdminWithQuery")
	}()
	for rows.Next() {
		var (
			id                     pgtypeuuid.UUID
			name                   *string
			description            *string
			placeableClassCategory *string
			public                 *bool
			fileId                 pgtypeuuid.UUID
			fileUrl                *string
			fileType               *string
			fileMime               *string
		)

		err = rows.Scan(
			&id,
			&name,
			&description,
			&placeableClassCategory,
			&public,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
		)
		if err != nil {
			return nil, -1, fmt.Errorf("failed to scan object class @ %s: %v", reflect.FunctionName(), err)
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
		}

		if i := findObjectClass(entities, id.UUID); i >= 0 {
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

			var e ObjectClass
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
			if placeableClassCategory != nil {
				e.Category = *placeableClassCategory
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

func IndexObjectClassesForAdminForCategoryWithQuery(ctx context.Context, category string, offset int64, limit int64, query string) (entities []ObjectClass, total int64, err error) {
	db := database.DB

	//region Count
	q := `SELECT COUNT(*) FROM placeable_classes pc WHERE pc.category = $1::text AND pc.name ILIKE $2::text`

	row := db.QueryRow(ctx, q, category, query)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to scan total @ %s: %v", reflect.FunctionName(), err)
	}
	//endregion

	q = `SELECT 
	pc.id                   placeableClassId,
	pc.name                 placeableClassName,
	pc.description          placeableClassDescription,
	pc.category             placeableClassCategory,
	e.public                entitypublic,
	preview.id              previewid,
	preview.url             previewurl,
	preview.type            previewtype,
	preview.mime        	previewmime
FROM placeable_classes pc
    LEFT JOIN entities e ON pc.id = e.id
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
WHERE pc.category = $1::text AND pc.name ILIKE $2::text
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, category, query)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to query object classes @ %s: %v", reflect.FunctionName(), err)
	}

	var (
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexObjectClassesForAdminForCategoryWithQuery")
	}()
	for rows.Next() {
		var (
			id                     pgtypeuuid.UUID
			name                   *string
			description            *string
			placeableClassCategory *string
			public                 *bool
			fileId                 pgtypeuuid.UUID
			fileUrl                *string
			fileType               *string
			fileMime               *string
		)

		err = rows.Scan(
			&id,
			&name,
			&description,
			&placeableClassCategory,
			&public,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
		)
		if err != nil {
			return nil, -1, fmt.Errorf("failed to scan object class @ %s: %v", reflect.FunctionName(), err)
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
		}

		if i := findObjectClass(entities, id.UUID); i >= 0 {
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

			var e ObjectClass
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
			if placeableClassCategory != nil {
				e.Category = *placeableClassCategory
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

func IndexObjectClassesForRequester(ctx context.Context, requester *sm.User, offset int64, limit int64) (entities []ObjectClass, total int64, err error) {
	db := database.DB

	//region Count
	q := `SELECT COUNT(*)
FROM placeable_classes pc 
    LEFT JOIN entities e ON e.id = pc.id
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
WHERE e.public OR a.can_view OR a.is_owner`

	row := db.QueryRow(ctx, q, requester.Id)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to scan total @ %s: %v", reflect.FunctionName(), err)
	}
	//endregion

	q = `SELECT 
	pc.id                   placeableClassId,
	pc.name                 placeableClassName,
	pc.description          placeableClassDescription,
	pc.category             placeableClassCategory,
	e.public                entitypublic,
	preview.id              previewid,
	preview.url             previewurl,
	preview.type            previewtype,
	preview.mime        	previewmime
FROM placeable_classes pc
    LEFT JOIN entities e ON pc.id = e.id
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
WHERE e.public OR a.can_view OR a.is_owner
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, requester.Id)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to query object classes @ %s: %v", reflect.FunctionName(), err)
	}

	var (
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexObjectClassesForRequester")
	}()
	for rows.Next() {
		var (
			id          pgtypeuuid.UUID
			name        *string
			description *string
			category    *string
			public      *bool
			fileId      pgtypeuuid.UUID
			fileUrl     *string
			fileType    *string
			fileMime    *string
		)

		err = rows.Scan(
			&id,
			&name,
			&description,
			&category,
			&public,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
		)
		if err != nil {
			return nil, -1, fmt.Errorf("failed to scan object class @ %s: %v", reflect.FunctionName(), err)
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
		}

		if i := findObjectClass(entities, id.UUID); i >= 0 {
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

			var e ObjectClass
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
			if category != nil {
				e.Category = *category
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

func IndexObjectClassesForRequesterForCategory(ctx context.Context, requester *sm.User, category string, offset int64, limit int64) (entities []ObjectClass, total int64, err error) {
	db := database.DB

	//region Count
	q := `SELECT COUNT(*)
FROM placeable_classes pc
	LEFT JOIN entities e ON e.id = pc.id
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
WHERE pc.category = $2::text AND (e.public OR a.can_view OR a.is_owner)`

	row := db.QueryRow(ctx, q, requester.Id /*$1*/, category /*$2*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to scan total @ %s: %v", reflect.FunctionName(), err)
	}
	//endregion

	q = `SELECT 
	pc.id                   placeableClassId,
	pc.name                 placeableClassName,
	pc.description          placeableClassDescription,
	pc.category             placeableClassCategory,
	e.public                entitypublic,
	preview.id              previewid,
	preview.url             previewurl,
	preview.type            previewtype,
	preview.mime        	previewmime
FROM placeable_classes pc
    LEFT JOIN entities e ON pc.id = e.id
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
WHERE pc.category = $2::text AND (e.public OR a.can_view OR a.is_owner)
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, requester.Id /*$1*/, category /*$2*/)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to query object classes @ %s: %v", reflect.FunctionName(), err)
	}

	var (
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexObjectClassesForRequesterForCategory")
	}()
	for rows.Next() {
		var (
			id                     pgtypeuuid.UUID
			name                   *string
			description            *string
			placeableClassCategory *string
			public                 *bool
			fileId                 pgtypeuuid.UUID
			fileUrl                *string
			fileType               *string
			fileMime               *string
		)

		err = rows.Scan(
			&id,
			&name,
			&description,
			&placeableClassCategory,
			&public,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
		)
		if err != nil {
			return nil, -1, fmt.Errorf("failed to scan object class @ %s: %v", reflect.FunctionName(), err)
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
		}

		if i := findObjectClass(entities, id.UUID); i >= 0 {
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

			var e ObjectClass
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
			if placeableClassCategory != nil {
				e.Category = *placeableClassCategory
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

func IndexObjectClassesForRequesterWithQuery(ctx context.Context, requester *sm.User, offset int64, limit int64, query string) (entities []ObjectClass, total int64, err error) {
	db := database.DB

	//region Count
	q := `SELECT COUNT(*) 
FROM placeable_classes pc
	LEFT JOIN entities e on pc.id = e.id
	LEFT JOIN accessibles a on e.id = a.entity_id AND a.user_id = $1::uuid 
WHERE pc.name ILIKE $2::text AND (e.public OR a.can_view OR a.is_owner)`

	row := db.QueryRow(ctx, q, requester.Id /*$1*/, query /*$2*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to scan total @ %s: %v", reflect.FunctionName(), err)
	}
	//endregion

	q = `SELECT 
	pc.id                   placeableClassId,
	pc.name                 placeableClassName,
	pc.description          placeableClassDescription,
	pc.category             placeableClassCategory,
	e.public                entitypublic,
	preview.id              previewid,
	preview.url             previewurl,
	preview.type            previewtype,
	preview.mime        	previewmime
FROM placeable_classes pc
    LEFT JOIN entities e ON pc.id = e.id
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1 
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
WHERE pc.name ILIKE $2::text AND (e.public OR a.can_view OR a.is_owner)
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, requester.Id /*$1*/, query /*$2*/)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to query rows @ %s: %v", reflect.FunctionName(), err)
	}

	var (
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexObjectClassesForRequesterWithQuery")
	}()
	for rows.Next() {
		var (
			id                     pgtypeuuid.UUID
			name                   *string
			description            *string
			placeableClassCategory *string
			public                 *bool
			fileId                 pgtypeuuid.UUID
			fileUrl                *string
			fileType               *string
			fileMime               *string
		)

		err = rows.Scan(
			&id,
			&name,
			&description,
			&placeableClassCategory,
			&public,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
		)
		if err != nil {
			return nil, -1, fmt.Errorf("failed to scan object class @ %s: %v", reflect.FunctionName(), err)
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
		}

		if i := findObjectClass(entities, id.UUID); i >= 0 {
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

			var e ObjectClass
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
			if placeableClassCategory != nil {
				e.Category = *placeableClassCategory
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

func IndexObjectClassesForRequesterForCategoryWithQuery(ctx context.Context, requester *sm.User, category string, offset int64, limit int64, query string) (entities []ObjectClass, total int64, err error) {
	db := database.DB

	//region Count
	q := `SELECT COUNT(*) 
FROM placeable_classes pc
	LEFT JOIN entities e on pc.id = e.id
	LEFT JOIN accessibles a on e.id = a.entity_id AND a.user_id = $1::uuid
WHERE pc.category = $2::text AND pc.name ILIKE $3::text AND (e.public OR a.can_view OR a.is_owner)`

	row := db.QueryRow(ctx, q, requester.Id /*$1*/, category /*$2*/, query /*$3*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to scan total @ %s: %v", reflect.FunctionName(), err)
	}
	//endregion

	q = `SELECT 
	pc.id                   placeableClassId,
	pc.name                 placeableClassName,
	pc.description          placeableClassDescription,
	pc.category             placeableClassCategory,
	e.public                entitypublic,
	preview.id              previewid,
	preview.url             previewurl,
	preview.type            previewtype,
	preview.mime        	previewmime
FROM placeable_classes pc
    LEFT JOIN entities e ON pc.id = e.id
   	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1 
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
WHERE pc.category = $2::text AND pc.name ILIKE $3::text AND (e.public OR a.can_view OR a.is_owner)
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, requester.Id /*$1*/, category /*$2*/, query /*$3*/)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to query object classes @ %s: %v", reflect.FunctionName(), err)
	}

	var (
		ri        int64 = 0
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexObjectClassesForRequesterWithQuery")
	}()
	for rows.Next() {
		var (
			id                     pgtypeuuid.UUID
			name                   *string
			description            *string
			placeableClassCategory *string
			public                 *bool
			fileId                 pgtypeuuid.UUID
			fileUrl                *string
			fileType               *string
			fileMime               *string
		)

		err = rows.Scan(
			&id,
			&name,
			&description,
			&placeableClassCategory,
			&public,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
		)
		if err != nil {
			return nil, -1, fmt.Errorf("failed to scan object class @ %s: %v", reflect.FunctionName(), err)
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
		}

		if i := findObjectClass(entities, id.UUID); i >= 0 {
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

			var e ObjectClass
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
			if placeableClassCategory != nil {
				e.Category = *placeableClassCategory
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

func IndexObjectClassCategoriesForAdmin(ctx context.Context, offset int64, limit int64) (entities []string, total int64, err error) {
	db := database.DB

	//region Count
	q := `SELECT count(*) FROM (SELECT DISTINCT category FROM placeable_classes) AS q`

	row := db.QueryRow(ctx, q)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to scan total @ %s: %v", reflect.FunctionName(), err)
	}
	//endregion

	q = `SELECT DISTINCT pc.category FROM placeable_classes pc WHERE pc.category IS NOT NULL ORDER BY pc.category`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to query object class categories @ %s: %v", reflect.FunctionName(), err)
	}

	var (
		ri int64 = 0
		ei int64 = 0
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexObjectClassCategoriesForAdmin")
	}()
	for rows.Next() {
		var (
			category string
		)

		err = rows.Scan(
			&category,
		)
		if err != nil {
			return nil, -1, err
		}

		ri++

		if ei < offset {
			ei++
			continue
		}

		if ei-offset >= limit {
			break
		}

		entities = append(entities, category)
		ei++
	}

	return entities, total, err
}

func IndexObjectClassCategoriesForAdminWithQuery(ctx context.Context, offset int64, limit int64, query string) (entities []string, total int64, err error) {
	db := database.DB

	//region Count
	q := `SELECT count(*) FROM (SELECT DISTINCT pc.category FROM placeable_classes pc WHERE pc.category ILIKE $1::text) AS q`

	row := db.QueryRow(ctx, q, query)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to scan total @ %s: %v", reflect.FunctionName(), err)
	}
	//endregion

	q = `SELECT DISTINCT pc.category FROM placeable_classes pc WHERE pc.category ILIKE $1::text ORDER BY pc.category`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, query)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to query object class categories @ %s: %v", reflect.FunctionName(), err)
	}

	var (
		ri int64 = 0
		ei int64 = 0
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexObjectClassCategoriesForAdminWithQuery")
	}()
	for rows.Next() {
		var (
			category string
		)

		err = rows.Scan(
			&category,
		)
		if err != nil {
			return nil, -1, fmt.Errorf("failed to scan object class category @ %s: %v", reflect.FunctionName(), err)
		}

		ri++

		if ei < offset {
			ei++
			continue
		}

		if ei-offset >= limit {
			break
		}

		entities = append(entities, category)
		ei++
	}

	return entities, total, err
}

func IndexObjectClassCategoriesForRequester(ctx context.Context, requester *sm.User, offset int64, limit int64) (entities []string, total int64, err error) {
	db := database.DB

	//region Count
	q := `SELECT count(*) FROM (
SELECT DISTINCT category 
	FROM placeable_classes pc 
		LEFT JOIN entities e ON pc.id = e.id
		LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
	WHERE (e.public OR a.can_view OR a.is_owner)
) AS q`

	row := db.QueryRow(ctx, q, requester.Id /*$1*/)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to scan total @ %s: %v", reflect.FunctionName(), err)
	}
	//endregion

	q = `SELECT DISTINCT pc.category 
FROM placeable_classes pc 
    LEFT JOIN entities e ON pc.id = e.id
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
WHERE pc.category IS NOT NULL AND (e.public OR a.can_view OR a.is_owner)
ORDER BY pc.category`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, requester.Id /*$1*/)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to query object class categories @ %s: %v", reflect.FunctionName(), err)
	}

	var (
		ri int64 = 0
		ei int64 = 0
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexObjectClassCategoriesForRequester")
	}()
	for rows.Next() {
		var (
			category string
		)

		err = rows.Scan(
			&category,
		)
		if err != nil {
			return nil, -1, fmt.Errorf("failed to scan object class category @ %s: %v", reflect.FunctionName(), err)
		}

		ri++

		if ei < offset {
			ei++
			continue
		}

		if ei-offset >= limit {
			break
		}

		entities = append(entities, category)
		ei++
	}

	return entities, total, err
}

func IndexObjectClassCategoriesForRequesterWithQuery(ctx context.Context, requester *sm.User, offset int64, limit int64, query string) (entities []string, total int64, err error) {
	db := database.DB

	//region Count
	q := `SELECT count(*) FROM (
SELECT DISTINCT category 
	FROM placeable_classes pc 
		LEFT JOIN entities e ON pc.id = e.id
		LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
	WHERE (e.public OR a.can_view OR a.is_owner) AND category ILIKE $2::text 
) AS q`

	row := db.QueryRow(ctx, q, requester.Id, query)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, fmt.Errorf("failed to scan total @ %s: %v", reflect.FunctionName(), err)
	}
	//endregion

	q = `SELECT DISTINCT pc.category 
FROM placeable_classes pc 
    LEFT JOIN entities e ON pc.id = e.id
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
WHERE (e.public OR a.can_view OR a.is_owner) AND category ILIKE $2::text 
ORDER BY pc.category`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, requester.Id, query)

	if err != nil {
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	var (
		ri int64 = 0
		ei int64 = 0
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexObjectClassCategoriesForRequesterWithQuery")
	}()
	for rows.Next() {
		var (
			category string
		)

		err = rows.Scan(
			&category,
		)
		if err != nil {
			return nil, -1, err
		}

		ri++

		if ei < offset {
			ei++
			continue
		}

		if ei-offset >= limit {
			break
		}

		entities = append(entities, category)
		ei++
	}

	return entities, total, err
}
