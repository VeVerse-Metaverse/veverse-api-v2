package helper

import (
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"veverse-api/database"
	apiModel "veverse-api/model"
	"veverse-api/sessionStore"
)

func init() {
	// Register the sm.User type with gob so that it can be stored in the session, otherwise sess.Save() will produce an error
	gob.Register(sm.User{})
}

func GetRequesterId(c *fiber.Ctx) string {
	// Get the user from the context, which was set in the JWT middleware
	user := c.Locals("user")
	if user != nil {
		token := user.(*jwt.Token)
		claims := token.Claims.(jwt.MapClaims)
		id := claims["id"].(string)
		return id
	}
	return ""
}

func GetRequester(c *fiber.Ctx) (*sm.User, error) {
	db := database.DB

	sess, err := sessionStore.Session.Get(c)
	if err != nil {
		return nil, err
	}

	var v *sm.User = nil // sess.Get("requester")
	if v == nil {
		id := GetRequesterId(c)

		if id == "" {
			return nil, errors.New("no requester")
		}

		q := `SELECT u.id, u.is_admin, u.is_active, u.is_banned, u.is_internal, u.eth_address FROM users u WHERE u.id = $1`

		row := db.QueryRow(c.UserContext(), q, id)

		user := new(sm.User)
		err = row.Scan(&user.Id, &user.IsAdmin, &user.IsActive, &user.IsBanned, &user.IsInternal, &user.EthAddress)
		if err != nil {
			return nil, err
		}

		sess.Set("requester", user)
		if err = sess.Save(); err != nil {
			return nil, err
		}

		return user, nil
	}

	user := v //.(sm.User)

	return user, nil
	//return &user, nil
}

func GetRequesterTokenData(token string) (claims map[string]interface{}, err error) {

	var t *jwt.Token
	t, err = jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(apiModel.AUTH_SECRET), nil
	})

	if err != nil && err.Error() != "signature is invalid" {
		return nil, err
	}

	var ok bool
	claims, ok = t.Claims.(jwt.MapClaims)
	if ok {
		return claims, nil
	}

	return nil, err
}
