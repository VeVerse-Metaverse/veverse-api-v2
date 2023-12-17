package handler

import (
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"github.com/dgrijalva/jwt-go"
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"github.com/markbates/goth"
	gf "github.com/shareed2k/goth_fiber"
	"github.com/sirupsen/logrus"
	"strconv"
	"time"
	"veverse-api/model"
)

type OAuthHelperRequest struct {
	Scope   string `json:"scope"`
	Code    string `json:"code"`
	Session string `json:"session"`
}

func OAuthHelperCallback(ctx *fiber.Ctx) error {
	// Get the provider name from the path
	provider := ctx.Params("provider")
	if provider == "" {
		logrus.Errorf("provider is empty")
		return ctx.SendString("error: no provider")
	}

	// Get the provider
	p, err := goth.GetProvider(provider)
	if err != nil {
		logrus.Errorf("failed to get provider: %v", err)
		return ctx.Status(fiber.StatusInternalServerError).SendString("error: unknown or unsupported provider")
	}

	// Parse the request body
	var req OAuthHelperRequest
	if err := ctx.BodyParser(&req); err != nil {
		logrus.Errorf("failed to parse request body: %v", err)
		return ctx.Status(fiber.StatusUnprocessableEntity).SendString("error: failed to process request body")
	}

	// Unmarshal the session
	sess, err := p.UnmarshalSession(req.Session)
	if err != nil {
		logrus.Errorf("failed to unmarshal session: %v", err)
		return ctx.Status(fiber.StatusUnprocessableEntity).SendString("error: failed to parse session")
	}

	// Get the user from the provider
	user, err := p.FetchUser(sess)
	if err != nil {
		logrus.Errorf("failed to fetch user: %v", err)
		return ctx.Status(fiber.StatusInternalServerError).SendString("error: failed to fetch user")
	}

	var t string

	if user.UserID != "" {
		//region Authenticate user with user id
		id := uuid.FromStringOrNil(user.UserID)
		if id != uuid.Nil {
			u, err := sm.GetUserById(ctx.UserContext(), sm.GetUserByIdRequest{Id: id})
			if err != nil {
				if err.Error() == "no rows in result set" {
					// User not found error, register user with user id
					u, err = sm.RegisterUserFromOAuthWithId(ctx.UserContext(), sm.RegisterUserRequestFromOAuthWithId{Id: id.String()})
					if err != nil {
						logrus.Errorf("failed to register user: %v", err)
						return ctx.Status(fiber.StatusInternalServerError).SendString("error: failed to register user")
					}
				} else {
					// Some other error
					logrus.Errorf("failed to get user by id: %v", err)
					return ctx.Status(fiber.StatusInternalServerError).SendString("error: failed to authenticate user with user id")
				}
			}

			// User found, login user
			t, err = getSignedJwt(u)
			if err != nil {
				logrus.Errorf("failed to get jwt token: %v", err)
				return ctx.SendStatus(fiber.StatusInternalServerError)
			}

			// Send jwt token to client
			return ctx.JSON(fiber.Map{"status": "ok", "message": "ok", "data": t})
		}
		//endregion
	}

	if user.Email != "" {
		//region Authenticate user with email

		// Get user from db by email
		u, err := sm.GetUserByEmail(ctx.UserContext(), sm.GetUserByEmailRequest{Email: user.Email})
		if err != nil {
			if err.Error() == "no rows in result set" {
				// User not found error, register user with email
				u, err = sm.RegisterUserFromOAuthWithEmail(ctx.UserContext(), sm.RegisterUserRequestFromOAuthWithEmail{Email: user.Email})
				if err != nil {
					logrus.Errorf("failed to register user: %v", err)
					return ctx.Status(fiber.StatusInternalServerError).SendString("error: failed to register user")
				}
			} else {
				// Some other error
				logrus.Errorf("failed to get user by email: %v", err)
				return ctx.Status(fiber.StatusInternalServerError).SendString("error: failed to authenticate user with email")
			}
		}

		// User found, login user
		t, err = getSignedJwt(u)
		if err != nil {
			logrus.Errorf("failed to get jwt token: %v", err)
			return ctx.SendStatus(fiber.StatusInternalServerError)
		}

		// Send jwt token to client
		return ctx.JSON(fiber.Map{"status": "ok", "message": "ok", "data": t})

		//endregion
	} else {
		// region Authenticate user with ethereum address

		userRawData, ok := user.RawData["data"].(map[string]interface{})
		if ok && userRawData != nil {
			userEthAddress, ok := userRawData["eth_wallet"].(string)
			if ok && userEthAddress != "" {
				// Get user from db by ethereum address
				u, err := sm.GetUserByEthAddress(ctx.UserContext(), sm.GetUserByEthAddressRequest{EthAddress: userEthAddress})
				if err != nil {
					if err.Error() == "no rows in result set" {
						// User not found error, register user with ethereum address
						u, err = sm.RegisterUserFromOAuthWithEthAddress(ctx.UserContext(), sm.RegisterUserRequestFromOAuthWithEthAddress{EthAddress: userEthAddress})
						if err != nil {
							logrus.Errorf("failed to register user: %v", err)
							return ctx.Status(fiber.StatusInternalServerError).SendString("error: failed to register user")
						}
					} else {
						// Some other error
						logrus.Errorf("failed to get user by eth address: %v", err)
						return ctx.Status(fiber.StatusInternalServerError).SendString("error: failed to authenticate user with ethereum address")
					}
				}

				// User found, login user
				t, err = getSignedJwt(u)
				if err != nil {
					logrus.Errorf("failed to get jwt token: %v", err)
					return ctx.SendStatus(fiber.StatusInternalServerError)
				}

				// Send jwt token to client
				return ctx.JSON(fiber.Map{"status": "ok", "message": "ok", "data": t})
			} else {
				logrus.Errorf("failed to get user by eth address: %v", err)
				return ctx.Status(fiber.StatusBadRequest).SendString("error: failed to authenticate user with ethereum address")
			}
		} else {
			logrus.Errorf("failed to get user by eth address: %v", err)
			return ctx.Status(fiber.StatusBadRequest).SendString("error: failed to authenticate user with ethereum address")
		}

		//endregion
	}
}

func getSignedJwt(u *sm.User) (t string, err error) {
	// Login user
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["id"] = u.Id
	claims["is_admin"] = u.IsAdmin

	var exp int
	exp, err = strconv.Atoi(model.AUTH_EXPIRATION)
	if err != nil {
		exp = 72 // Default to 3 days
	}
	claims["exp"] = time.Now().Add(time.Duration(exp) * time.Hour).Unix()

	t, err = token.SignedString([]byte(model.AUTH_SECRET))
	if err != nil {
		return "", err
	}

	return t, nil
}

func OAuthCallback(ctx *fiber.Ctx) error {
	_, err := gf.CompleteUserAuth(ctx)
	if err != nil {
		logrus.Errorf("failed to complete user auth: %v", err)
		return err
	}
	return ctx.SendString("authenticated")
}

func OAuthLogout(ctx *fiber.Ctx) error {
	if err := gf.Logout(ctx); err != nil {
		logrus.Errorf("failed to logout: %v", err)
		return err
	}

	return ctx.SendString("logout")
}
