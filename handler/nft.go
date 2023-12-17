package handler

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgtype"
	pgtypeuuid "github.com/jackc/pgtype/ext/gofrs-uuid"
	"github.com/jackc/pgx/v5"
	"io/ioutil"
	"net/http"
	"time"
	"veverse-api/database"
	"veverse-api/helper"
	"veverse-api/model"
)

//var DefaultChainId = os.Getenv("DEFAULT_CHAIN_ID")

func GetRequesterNFTAssets(c *fiber.Ctx) error {

	//m := model.NFTAssetsRequestMetadata{}
	//if err := c.QueryParser(&m); err != nil {
	//	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	//}
	//
	//if m.ChainId == "" {
	//	m.ChainId = DefaultChainId
	//}

	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	m := model.BatchRequestMetadata{}
	err = c.QueryParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	var (
		offset int64 = 0
		limit  int64 = 100
		query        = ""
		total  int
	)

	if m.Offset > 0 {
		offset = m.Offset
	}

	if m.Limit > 0 && m.Limit < 100 {
		limit = m.Limit
	}

	if m.Query != "" {
		query = m.Query
	}

	err, cAt, uAt, assetsMap, response, total := model.GetNFTAssets(c.UserContext(), requester, offset, limit, query)

	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "something went wrong", "data": nil})
	}

	if (uAt.Status == pgtype.Null && time.Now().UnixMilli()-time.Hour.Milliseconds() > cAt.Time.UnixMilli()) ||
		(uAt.Status != pgtype.Null && time.Now().UnixMilli()-time.Hour.Milliseconds() > uAt.Time.UnixMilli()) {

		if requester.EthAddress == nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "requester didn't link account with wallet", "data": nil})
		}

		//0x7661f05aa6042043ca691f3704ec6abafeef4595
		ethAddress := *requester.EthAddress
		cursor := ""
		var assets []model.NFTAsset
		for {
			url := "https://api.opensea.io/api/v1/assets?owner=" + ethAddress + "&order_direction=asc&include_orders=false&cursor=" + cursor
			req, _ := http.NewRequest("GET", url, nil)

			req.Header.Add("Accept", "application/json")
			req.Header.Add("X-API-Key", model.OPENSEA_API_KEY)

			res, _ := http.DefaultClient.Do(req)

			defer res.Body.Close()
			body, err1 := ioutil.ReadAll(res.Body)
			if err1 != nil {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "internal err", "data": nil})
			}

			var nftAssets model.NFTAssets
			json.Unmarshal(body, &nftAssets)

			assets = append(assets, nftAssets.Assets...)

			if nftAssets.Next != "" {
				cursor = nftAssets.Next
			} else {
				break
			}
		}

		_ = model.CRUDNFTAssetsForRequester(c, requester, assets, assetsMap)

		err, _, _, assetsMap, response, total = model.GetNFTAssets(c.UserContext(), requester, offset, limit, query)

		if err != nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "something went wrong", "data": nil})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fiber.Map{"entities": response, "limit": limit, "offset": offset, "total": total}})
	} else {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": fiber.Map{"entities": response, "limit": limit, "offset": offset, "total": total}})
	}
}

func GetRequesterNFTAsset(c *fiber.Ctx) error {
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	//region Request metadata
	m := model.IdRequestMetadata{}
	if err = c.ParamsParser(&m); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

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
	INNER JOIN entities e on na.id = e.id
    INNER JOIN objects o on e.id = o.id
	INNER JOIN accessibles a on e.id = a.entity_id
	LEFT JOIN files f on f.entity_id = e.id
WHERE o.type = 'NFT' AND a.user_id = $1::uuid AND na.id::uuid = $2`

	var (
		row pgx.Row

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

	db := database.DB
	row = db.QueryRow(c.UserContext(), q, requester.Id, m.Id)

	err = row.Scan(
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
		if err.Error() == "no rows in result set" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "not found", "data": nil})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	asset := map[string]interface{}{
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
		"objectName":           objectName,
		"mimeType":             fileMime,
		"objectType":           objectType,
		"objectDescription":    objectDescription,
		"createdAt":            createdAt,
		"updatedAt":            updatedAt,
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": asset})
}
