package model

import (
	"context"
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"encoding/gob"
	"fmt"
	"github.com/gofrs/uuid"
	"github.com/jackc/pgtype"
	pgtypeuuid "github.com/jackc/pgtype/ext/gofrs-uuid"
	"github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"
	"math"
	"time"
	"veverse-api/database"
	"veverse-api/reflect"
)

type User struct {
	Entity

	Email          *string    `json:"email,omitempty"`
	PasswordHash   *string    `json:"-"`
	ApiKey         *string    `json:"apiKey,omitempty"`
	Name           *string    `json:"name"`
	Description    *string    `json:"description,omitempty"`
	IsActive       bool       `json:"isActive,omitempty"`
	IsAdmin        bool       `json:"isAdmin,omitempty"`
	IsMuted        bool       `json:"isMuted,omitempty"`
	IsBanned       bool       `json:"isBanned,omitempty"`
	IsInternal     bool       `json:"isInternal,omitempty"`
	LastSeenAt     *time.Time `json:"lastSeenAt,omitempty"`
	ActivatedAt    *time.Time `json:"activatedAt,omitempty"`
	AllowEmails    bool       `json:"allowEmails,omitempty"`
	Experience     int32      `json:"experience,omitempty"`
	Level          int32      `json:"level,omitempty"`
	Rank           string     `json:"rank,omitempty"`
	EthAddress     *string    `json:"ethAddress,omitempty"`
	Address        *string    `json:"address,omitempty"`
	DefaultPersona *Persona   `json:"defaultPersona,omitempty"`
	Presence       *Presence  `json:"presence,omitempty"`
}

type NonceRequestMetadata struct {
	Address string `query:"address,required"`
}

const (
	UserSingular     = "user"
	userPlural       = "users"
	FollowerSingular = "follower"
	followerPlural   = "followers"
	FriendPlural     = "friends"
	AvatarPlural     = "avatars"
)

func init() {
	// register for session storage
	gob.Register(User{})
}

var expBase = 10.0
var expExponent = 1.5
var ranks = map[int32]string{
	0:   "newcomer",
	10:  "beginner",
	20:  "amateur",
	30:  "apprentice",
	40:  "accustomed",
	50:  "skillful",
	60:  "expert",
	70:  "prodigy",
	80:  "professional",
	90:  "legendary",
	100: "epic",
}

// Updates user model computed properties such as rank and level based on experience
func (u *User) UpdateComputedProperties() {
	if u == nil {
		return
	}

	u.Rank = u.GetRank()
	u.Level = u.GetLevel()
}

func (u User) GetLevel() int32 {
	return int32(math.Floor(math.Pow(float64(u.Experience)/expBase, 1.0/expExponent)))
}

func (u User) GetRank() string {
	l := u.GetLevel()
	i := int32(math.Round(float64(l)/10)) * 10

	if r, ok := ranks[i]; ok {
		return r
	}

	return "unknown"
}

func findUser(h []User, id uuid.UUID) int {
	for i, v := range h {
		if *v.Id == id {
			return i
		}
	}
	return -1
}

// IndexUsersForAdmin Index users for admin
func IndexUsersForAdmin(ctx context.Context, offset int64, limit int64) (entities []User, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) FROM users u`

	row := db.QueryRow(ctx, q)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}
	//endregion

	if total == 0 {
		return []User{}, total, nil
	}

	q = `SELECT
u.id userId,
u.name userName,
u.description userDescription,
u.is_active userIsActive,
u.is_admin userIsAdmin,
u.is_muted userIsMuted,
u.is_banned userIsBanned,
u.experience userExperience,
u.default_persona_id userDefaultPersonaId,
p.space_id presenceSpaceId,
p.server_id presenceServerId,
p.status presenceStatus,
p.updated_at presenceUpdatedAt,
e.public entityPublic,
avatar.id previewId,
avatar.url previewUrl,
avatar.type previewType,
avatar.mime previewMime
FROM users u
	LEFT JOIN entities e on u.id = e.id
	LEFT JOIN files avatar ON e.id = avatar.entity_id AND avatar.type = 'image_avatar'
	LEFT JOIN presence p on u.id = p.user_id AND p.updated_at > now() - interval '1m'
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows      pgx.Rows
		ei        int64 = 0
		skipped         = false
		skippedId uuid.UUID
	)

	rows, err = db.Query(ctx, q)

	if err != nil {
		return []User{}, total, err
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexUsersForAdmin")
	}()
	for rows.Next() {
		var (
			id          pgtypeuuid.UUID
			name        *string
			title       *string
			description *string
			public      *bool
			fileId      pgtypeuuid.UUID
			fileUrl     *string
			fileType    *string
			fileMime    *string
		)

		err = rows.Scan(
			&id,
			&name,
			&title,
			&description,
			&public,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
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

			if fileUrl != nil {
				file.Url = *fileUrl
			}
		}

		if i := findUser(entities, id.UUID); i >= 0 {
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

			var e User
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = name
			}
			if description != nil {
				e.Description = description
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

// IndexUsersForAdminWithQuery Index users for admin with query and pak file
func IndexUsersForAdminWithQuery(ctx context.Context, offset int64, limit int64, query string) (entities []User, total int64, err error) {
	db := database.DB

	q := `SELECT COUNT(*) FROM users u WHERE u.name ILIKE $1::text`

	row := db.QueryRow(ctx, q, query)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}

	q = `SELECT 
	m.id                    userid,
	m.name                  username,
	m.title                 usertitle,
	m.description           userdescription,
	e.public                entitypublic,
	preview.id              previewid,
	preview.url             previewurl,
	preview.type            previewtype,
	preview.mime        	previewmime
FROM mods m
    LEFT JOIN entities e ON m.id = e.id
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
WHERE m.name ILIKE $1::text
ORDER BY e.updated_at DESC, e.created_at DESC, e.id`

	var (
		rows      pgx.Rows
		ri        int64 = 0
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
		database.LogPgxStat("IndexUsersForAdminWithQuery")
	}()
	for rows.Next() {
		var (
			id          pgtypeuuid.UUID
			name        *string
			title       *string
			description *string
			public      *bool
			fileId      pgtypeuuid.UUID
			fileUrl     *string
			fileType    *string
			fileMime    *string
		)

		err = rows.Scan(
			&id,
			&name,
			&title,
			&description,
			&public,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
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
		}

		if i := findUser(entities, id.UUID); i >= 0 {
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

			var e User
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = name
			}
			if description != nil {
				e.Description = description
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

// IndexUsersForRequester Index users for requester
func IndexUsersForRequester(ctx context.Context, requester *User, offset int64, limit int64) (entities []User, total int64, err error) {
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
		return nil, -1, err
	}
	//endregion

	q = `SELECT
	m.id                    userid,
	m.name                  username,
	m.title                 usertitle,
	m.description           userdescription,
	m.map                   usermap,
	e.public                entitypublic,
	preview.id              previewid,
	preview.url             previewurl,
	preview.type            previewtype,
	preview.mime        	previewmime
FROM mods m
    LEFT JOIN entities e ON m.id = e.id
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
WHERE e.public OR a.can_view OR a.is_owner
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
		database.LogPgxStat("IndexUsersForRequester")
	}()
	for rows.Next() {
		var (
			id          pgtypeuuid.UUID
			name        *string
			title       *string
			description *string
			public      *bool
			fileId      pgtypeuuid.UUID
			fileUrl     *string
			fileType    *string
			fileMime    *string
		)
		err = rows.Scan(
			&id,
			&name,
			&title,
			&description,
			&public,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
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
			if fileUrl != nil {
				file.Url = *fileUrl
			}

			if fileType != nil {
				file.Type = *fileType
			}

			if fileMime != nil {
				file.Mime = fileMime
			}
		}
		//endregion

		if i := findUser(entities, id.UUID); i >= 0 {
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

			var e User
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = name
			}
			if description != nil {
				e.Description = description
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

// IndexUsersForRequesterWithQuery Index users for requester with query and pak file
func IndexUsersForRequesterWithQuery(ctx context.Context, requester *User, offset int64, limit int64, query string) (entities []User, total int64, err error) {
	db := database.DB

	q := `SELECT COUNT(*) FROM mods m
	LEFT JOIN entities e ON e.id = m.id
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
WHERE m.name ILIKE $2::text OR m.title ILIKE $2::text AND (e.public OR a.can_view OR a.is_owner)`

	row := db.QueryRow(ctx, q, requester.Id, query)

	err = row.Scan(&total)
	if err != nil {
		return nil, -1, err
	}

	q = `SELECT 
	m.id                    userid,
	m.name                  username,
	m.title                 usertitle,
	m.description           userdescription,
	e.public                entitypublic,
	preview.id              previewid,
	preview.url             previewurl,
	preview.type            previewtype,
	preview.mime        	previewmime
FROM mods m
    LEFT JOIN entities e ON m.id = e.id
	LEFT JOIN files preview ON e.id = preview.entity_id AND preview.type = 'image_preview'
	LEFT JOIN accessibles a ON e.id = a.entity_id AND a.user_id = $1::uuid
WHERE m.name ILIKE $2::text OR m.title ILIKE $2::text AND (e.public OR a.can_view OR a.is_owner)
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
		return nil, 0, fmt.Errorf("failed to query %s @ %s: %v", packageSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexUsersForRequesterWithQuery")
	}()
	for rows.Next() {
		var (
			id          pgtypeuuid.UUID
			name        *string
			title       *string
			description *string
			public      *bool
			fileId      pgtypeuuid.UUID
			fileUrl     *string
			fileType    *string
			fileMime    *string
		)

		err = rows.Scan(
			&id,
			&name,
			&title,
			&description,
			&public,
			&fileId,
			&fileUrl,
			&fileType,
			&fileMime,
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
		}

		if i := findUser(entities, id.UUID); i >= 0 {
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

			var e User
			e.Id = &id.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = name
			}
			if description != nil {
				e.Description = description
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

// GetUserForAdmin Get user
func GetUserForAdmin(ctx context.Context, id uuid.UUID) (entity *User, err error) {
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
	m.id userId,
	m.name userName,
	m.title userTitle,
	preview.id previewId,
	preview.url previewUrl,
	preview.type previewType,
	preview.mime previewMime
FROM portals p
	LEFT JOIN entities e on p.id = e.id
	-- destination portal
    LEFT JOIN portals d ON d.id = p.destination_id
    LEFT JOIN entities de ON de.id = d.id
    LEFT JOIN files preview ON de.id = preview.entity_id AND preview.type = 'rendertarget_preview'
	-- destination space
    LEFT JOIN spaces s ON s.id = d.space_id
	LEFT JOIN entities se ON se.id = s.id
	-- destination user
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
		database.LogPgxStat("GetUserForAdmin")
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
			userId          pgtypeuuid.UUID
			userName        *string
			userTitle       *string
			previewId       pgtypeuuid.UUID
			previewUrl      *string
			previewType     *string
			previewMime     *string
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
			&userId,
			&userName,
			&userTitle,
			&previewId,
			&previewUrl,
			&previewType,
			&previewMime,
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
		}

		if entity != nil {
			if preview != nil && !containsFile(entity.Files, *preview.Id) {
				entity.Files = append(entity.Files, *preview)
			}
		} else {
			var e User
			e.Id = &portalId.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = name
			}
			if preview != nil {
				e.Files = append(e.Files, *preview)
			}

			entity = &e
		}
	}

	return entity, err
}

// GetUserForRequester Get user
func GetUserForRequester(ctx context.Context, requester *User, id uuid.UUID) (entity *User, err error) {
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
	m.id userId,
	m.name userName,
	m.title userTitle,
	preview.id previewId,
	preview.url previewUrl,
	preview.type previewType,
	preview.mime previewMime
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
	-- destination user
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
		database.LogPgxStat("GetUserForRequester")
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
			userId          pgtypeuuid.UUID
			userName        *string
			userTitle       *string
			previewId       pgtypeuuid.UUID
			previewUrl      *string
			previewType     *string
			previewMime     *string
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
			&userId,
			&userName,
			&userTitle,
			&previewId,
			&previewUrl,
			&previewType,
			&previewMime,
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
		}

		if entity != nil {
			if preview != nil && !containsFile(entity.Files, *preview.Id) {
				entity.Files = append(entity.Files, *preview)
			}
		} else {
			var e User
			e.Id = &portalId.UUID
			if public != nil {
				e.Public = public
			}
			if name != nil {
				e.Name = name
			}
			if preview != nil {
				e.Files = append(e.Files, *preview)
			}
			entity = &e
		}
	}

	return entity, err
}

// GetUserNonce Get user nonce for web3 secure signature
func GetUserNonce(ctx context.Context, address string) (err error, nonce *int, user User) {
	db := database.DB
	q := `SELECT u.id, u.hash, u.is_admin, u.nonce FROM users u where u.eth_address = $1`

	var row pgx.Row
	row = db.QueryRow(ctx, q, address)

	err = row.Scan(&user.Id, &user.PasswordHash, &user.IsAdmin, &nonce)

	return err, nonce, user
}

// IndexFollowers Index friends for requester
func IndexFollowers(ctx context.Context, userId uuid.UUID, offset int64, limit int64) (followers []User, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) FROM followers WHERE leader_id = $1`

	row := db.QueryRow(ctx, q, userId)

	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", followerPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get %s", followerPlural)
	}
	//endregion

	q = `SELECT
    	u.id,
    	u.name,
    	u.description,
    	e.views,
    	u.last_seen_at,
		u.is_active,
		u.is_muted,
		u.is_admin,
		u.experience
	FROM followers fw
	LEFT JOIN users u ON fw.follower_id = u.id
	LEFT JOIN entities e ON e.id = u.id
	LEFT JOIN files f ON f.entity_id = e.id
	WHERE fw.leader_id = $1
	OFFSET $2 LIMIT $3`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, userId, offset, limit)
	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", followerPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get %s", followerPlural)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexFollowers")
	}()
	for rows.Next() {
		var user User
		err = rows.Scan(
			&user.Id,
			&user.Name,
			&user.Description,
			&user.Views,
			&user.LastSeenAt,
			&user.IsActive,
			&user.IsMuted,
			&user.IsAdmin,
			&user.Experience,
		)

		if err != nil {
			logrus.Errorf("failed to scan %s @ %s: %v", followerPlural, reflect.FunctionName(), err)
			return nil, -1, fmt.Errorf("failed to get %s", followerPlural)
		}

		user.Level = user.GetLevel()
		user.Rank = user.GetRank()

		followers = append(followers, user)
	}

	return followers, total, err
}

func IndexFriends(ctx context.Context, userId uuid.UUID, offset int64, limit int64) (friends []User, total int64, err error) {
	db := database.DB

	//region Total
	q := `SELECT COUNT(*) FROM followers f
	INNER JOIN followers f2 ON f.leader_id = f2.follower_id AND f.follower_id = f2.leader_id
	WHERE f.leader_id = $1`

	row := db.QueryRow(ctx, q, userId)

	err = row.Scan(&total)
	if err != nil {
		logrus.Errorf("failed to scan %s @ %s: %v", FriendPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get %s", FriendPlural)
	}
	//endregion

	q = `SELECT
    	u.id,
    	u.name,
    	u.description,
    	e.views,
    	u.last_seen_at,
		u.is_active,
		u.is_muted,
		u.is_admin,
		u.experience
	FROM followers f
	INNER JOIN followers f2 ON f.leader_id = f2.follower_id AND f.follower_id = f2.leader_id
	INNER JOIN users u ON u.id = f.follower_id
	INNER JOIN entities e ON e.id = u.id
	WHERE f.leader_id = $1
	OFFSET $2 LIMIT $3`

	var rows pgx.Rows
	rows, err = db.Query(ctx, q, userId, offset, limit)
	if err != nil {
		logrus.Errorf("failed to query %s @ %s: %v", FriendPlural, reflect.FunctionName(), err)
		return nil, -1, fmt.Errorf("failed to get %s", FriendPlural)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("IndexFriends")
	}()
	for rows.Next() {
		var user User
		err = rows.Scan(
			&user.Id,
			&user.Name,
			&user.Description,
			&user.Views,
			&user.LastSeenAt,
			&user.IsActive,
			&user.IsMuted,
			&user.IsAdmin,
			&user.Experience,
		)

		if err != nil {
			logrus.Errorf("failed to scan %s @ %s: %v", FriendPlural, reflect.FunctionName(), err)
			return nil, -1, fmt.Errorf("failed to get %s", FriendPlural)
		}

		user.Level = user.GetLevel()
		user.Rank = user.GetRank()

		friends = append(friends, user)
	}

	return friends, total, err
}

// GetBasicUserInfo Get basic user info
func GetBasicUserInfo(ctx context.Context, id uuid.UUID) (entity *sm.User, err error) {
	db := database.DB

	q := `SELECT
	u.id userId,
	u.name userName,
	u.email userEmail,
	u.allow_emails userAllowEmails,
	e.public entityPublic
FROM users u
	LEFT JOIN entities e on u.id = e.id
WHERE u.id = $1
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
		database.LogPgxStat("GetBasicUserInfo")
	}()
	for rows.Next() {
		var (
			userId          pgtypeuuid.UUID
			userName        *string
			userEmail       *string
			userAllowEmails *bool
			public          bool
		)

		err = rows.Scan(
			&userId,
			&userName,
			&userEmail,
			&userAllowEmails,
			&public,
		)
		if err != nil {
			return nil, err
		}

		if userId.Status == pgtype.Null {
			continue
		}

		var e sm.User
		e.Id = userId.UUID
		e.Name = userName
		e.Email = userEmail
		if userAllowEmails != nil {
			e.AllowEmails = *userAllowEmails
		} else {
			e.AllowEmails = true
		}
		e.Public = public

		entity = &e
	}

	return entity, err
}
