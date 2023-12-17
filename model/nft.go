package model

import (
	"context"
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"github.com/jackc/pgtype"
	pgtypeuuid "github.com/jackc/pgtype/ext/gofrs-uuid"
	"golang.org/x/exp/slices"
	"path/filepath"
	"strings"
	"veverse-api/database"
	"veverse-api/reflect"
)

type NFTAssets struct {
	Next     string     `json:"next,omitempty"`
	Previous int        `json:"previous,omitempty"`
	Assets   []NFTAsset `json:"assets"`
}

type NFTAsset struct {
	Id                   int    `json:"id"`
	ImageUrl             string `json:"image_url"`
	ImagePreviewUrl      string `json:"image_preview_url"`
	ImageThumbnailUrl    string `json:"image_thumbnail_url"`
	ImageOriginalUrl     string `json:"image_original_url"`
	AnimationUrl         string `json:"animation_url"`
	AnimationOriginalUrl string `json:"animation_original_url"`
	Name                 string `json:"name"`
	Description          string `json:"description"`
	ExternalLink         string `json:"external_link"`
	AssetContract        struct {
		Address    string `json:"address"`
		Owner      string `json:"owner,omitempty"`
		SchemaName string `json:"schema_name"`
	} `json:"asset_contract"`
	Permalink string `json:"permalink"`
}

type NFTMetadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Image       string `json:"image,omitempty"`
	ImageData   string `json:"image_data,omitempty"`
}

type NFTAssetsRequestMetadata struct {
	ChainId string `json:"chain_id"`
}

func CRUDNFTAssetsForRequester(c *fiber.Ctx, requester *sm.User, assets []NFTAsset, existedAssets map[int]map[string]interface{}) (err error) {
	db := database.DB

	for _, row := range assets {
		if _, ok := existedAssets[row.Id]; ok {
			// Update asset
		} else {
			id, err1 := uuid.NewV4()
			if err1 != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "failed to generate uuid", "data": nil})
			}

			tx, err1 := db.Begin(c.UserContext())
			if err1 != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err1.Error(), "data": nil})
			}

			//region Entity
			entityType := "object"
			q := `INSERT INTO entities (id, entity_type, public) VALUES ($1, $2, $3)`
			if _, err1 = tx.Exec(c.UserContext(), q, id /*1*/, entityType /*2*/, false /*3*/); err1 != nil {
				if err2 := tx.Rollback(c.UserContext()); err2 != nil {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": fmt.Sprintf("%s, %s", err.Error(), err2.Error()), "data": nil})
				}
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err1.Error(), "data": nil})
			}
			//endregion

			//region Accessible
			q = `INSERT INTO accessibles (user_id, entity_id, is_owner, can_view, can_edit, can_delete) VALUES ($1, $2, true, true, true, true)`
			if _, err1 = tx.Exec(c.UserContext(), q, requester.Id /*1*/, id /*2*/); err1 != nil {
				if err2 := tx.Rollback(c.UserContext()); err2 != nil {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": fmt.Sprintf("%s, %s", err.Error(), err2.Error()), "data": nil})
				}
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err1.Error(), "data": nil})
			}
			//endregion

			//region Objects
			q = `INSERT INTO objects (id, type, name, description, source_url) VALUES ($1, $2, $3, $4, $5)`
			if _, err = tx.Exec(c.UserContext(), q, id /*1*/, "NFT" /*2*/, row.Name /*3*/, row.Description /*4*/, row.Permalink /*5*/); err != nil {
				if err1 = tx.Rollback(c.UserContext()); err1 != nil {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": fmt.Sprintf("%s, %s", err.Error(), err1.Error()), "data": nil})
				}
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
			}
			//endregion

			//region Objects
			supportedExt := []string{".gltf", ".glb"}
			fileExt := filepath.Ext(row.AnimationUrl)
			if slices.Contains(supportedExt, fileExt) {
				q = `INSERT INTO files AS f (id, entity_id, url, type, mime, uploaded_by, created_at, updated_at) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, now(), null)`
				if _, err = tx.Exec(c.UserContext(), q, id /*1*/, row.AnimationUrl /*2*/, "model" /*3*/, "model/gltf-binary" /*$4*/, requester.Id /*5*/); err != nil {
					if err1 = tx.Rollback(c.UserContext()); err1 != nil {
						return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": fmt.Sprintf("%s, %s", err.Error(), err1.Error()), "data": nil})
					}
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
				}
			}
			//endregion

			//region nft_assets
			q = `INSERT INTO nft_assets (
                        id,
                        asset_id,
                        name,
                        description,
                        contract_address,
                        owner,
                        schema_name,
                        external_link,
                        permalink,
                        image_url,
                        image_preview_url,
                        image_thumbnail_url,
                        image_original_url,
                        animation_url,
                        animation_original_url
                    ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`

			if _, err = tx.Exec(
				c.UserContext(),
				q,
				id,                           /*1*/
				row.Id,                       /*2*/
				row.Name,                     /*3*/
				row.Description,              /*4*/
				row.AssetContract.Address,    /*5*/
				row.AssetContract.Owner,      /*6*/
				row.AssetContract.SchemaName, /*7*/
				row.ExternalLink,             /*8*/
				row.Permalink,                /*9*/
				row.ImageUrl,                 /*10*/
				row.ImagePreviewUrl,          /*11*/
				row.ImageThumbnailUrl,        /*12*/
				row.ImageOriginalUrl,         /*13*/
				row.AnimationUrl,             /*14*/
				row.AnimationOriginalUrl,     /*15*/

			); err != nil {
				if err1 = tx.Rollback(c.UserContext()); err1 != nil {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": fmt.Sprintf("%s, %s", err.Error(), err1.Error()), "data": nil})
				}
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
			}
			//endregion

			if err = tx.Commit(c.UserContext()); err != nil {
				if err1 = tx.Rollback(c.UserContext()); err1 != nil {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": fmt.Sprintf("%s, %s", err.Error(), err1.Error()), "data": nil})
				}
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
			}
		}
	}

	return err
}

func GetNFTAssets(ctx context.Context, requester *sm.User, offset int64, limit int64, query string) (err error, cAt pgtype.Timestamp, uAt pgtype.Timestamp, assetsMap map[int]map[string]interface{}, response []interface{}, total int) {
	q := `SELECT
    	na.id,
    	na.asset_id,
    	na.name asset_name,
		na.description asset_description,
		na.contract_address,
		na.owner,
		na.schema_name,
		na.permalink,
		na.image_url,
		na.image_preview_url,
		na.image_thumbnail_url,
		na.image_original_url,
		na.animation_url,
		na.animation_original_url,
		o.name object_name,
		o.type object_type,
		o.description object_description,
		e.created_at,
		e.updated_at,
		f.mime
FROM nft_assets AS na
	LEFT JOIN entities e on na.id = e.id
    LEFT JOIN objects o on e.id = o.id
	LEFT JOIN accessibles a on e.id = a.entity_id
	LEFT JOIN files f on f.entity_id = e.id
WHERE o.type = 'NFT' AND a.user_id = $1::uuid`

	db := database.DB
	rows, err := db.Query(ctx, q, requester.Id)

	if err != nil {
		return fmt.Errorf("failed to query %s @ %s: %v", reflect.FunctionName(), err), pgtype.Timestamp{}, pgtype.Timestamp{}, assetsMap, nil, -1
	}

	existedAssets := map[int]map[string]interface{}{}

	var (
		ei int64 = 0
	)

	var (
		id                   pgtypeuuid.UUID
		assetId              int
		name                 *string
		description          *string
		contractAddress      *string
		owner                *string
		schemaName           *string
		permalink            *string
		imageUrl             *string
		imagePreviewUrl      *string
		imageThumbnailUrl    *string
		imageOriginalUrl     *string
		animationUrl         *string
		animationOriginalUrl *string
		objectName           *string
		objectType           *string
		objectDescription    *string
		createdAt            pgtype.Timestamp
		updatedAt            pgtype.Timestamp
		fileMime             *string
	)

	defer func() {
		rows.Close()
		database.LogPgxStat("GetNFTAssets")
	}()
	for rows.Next() {
		err = rows.Scan(
			&id,
			&assetId,
			&name,
			&description,
			&contractAddress,
			&owner,
			&schemaName,
			&permalink,
			&imageUrl,
			&imagePreviewUrl,
			&imageThumbnailUrl,
			&imageOriginalUrl,
			&animationUrl,
			&animationOriginalUrl,
			&objectName,
			&objectType,
			&objectDescription,
			&createdAt,
			&updatedAt,
			&fileMime,
		)

		if err != nil {
			return err, cAt, uAt, nil, nil, -1
		}

		if cAt.Status == pgtype.Undefined && uAt.Status == pgtype.Undefined {
			cAt = createdAt
			uAt = updatedAt
		}

		existedAssets[assetId] = make(map[string]interface{})
		existedAssets[assetId]["asset_id"] = assetId
		existedAssets[assetId]["entity_id"] = id
		existedAssets[assetId]["createdAt"] = createdAt
		existedAssets[assetId]["updatedAt"] = updatedAt

		if query == "" || strings.Contains(strings.ToLower(*name), strings.ToLower(query)) {
			total++
		}

		if ei < offset {
			ei++
		} else if ei-offset < limit && (query == "" || strings.Contains(strings.ToLower(*name), strings.ToLower(query))) {
			response = append(response, map[string]interface{}{
				"id":                   id,
				"assetId":              assetId,
				"assetName":            name,
				"assetDescription":     description,
				"contractAddress":      contractAddress,
				"owner":                owner,
				"schemaName":           schemaName,
				"permalink":            permalink,
				"imageUrl":             imageUrl,
				"imagePreviewUrl":      imagePreviewUrl,
				"imageThumbnailUrl":    imageThumbnailUrl,
				"imageOriginalUrl":     imageOriginalUrl,
				"animationUrl":         animationUrl,
				"animationOriginalUrl": animationOriginalUrl,
				"mimeType":             fileMime,
				"objectName":           objectName,
				"objectType":           objectType,
				"objectDescription":    objectDescription,
				"createdAt":            createdAt,
				"updatedAt":            updatedAt,
			})

			ei++
		}
	}

	return nil, cAt, uAt, existedAssets, response, total
}
